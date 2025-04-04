package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/adamhf/rss_gitlab_sync/storage"
	"github.com/mmcdole/gofeed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var lastRunGauge prometheus.Gauge
var issuesCreatedCounter prometheus.Counter
var issueCreationErrorCounter prometheus.Counter

type Config struct {
	Feeds    []Feed
	Interval int
}

type Feed struct {
	ID              string
	FeedURL         string `yaml:"feed_url"`
	Name            string
	GitlabProjectID int `yaml:"gitlab_project_id"`
	Labels          []string
	AddedSince      time.Time `yaml:"added_since"`
	Retroactive     bool
}

type EnvValues struct {
	DBPath           string
	ConfDir          string
	GitlabAPIKey     string
	GitlabAPIBaseUrl string
	S3Enabled        bool
	S3Endpoint       string
	S3Region         string
	S3BucketName     string
	S3KeyPrefix      string
	S3AccessKey      string
	S3SecretKey      string
	S3BackupInterval time.Duration
}

func hasExistingGitlabIssue(guid string, projectID int, gitlabClient *gitlab.Client) bool {
	searchOpts := &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 10,
		},
	}
	issues, _, err := gitlabClient.Search.IssuesByProject(projectID, guid, searchOpts)
	if err != nil {
		log.Printf("Unable to query Gitlab for existing issues for GUID %s: %v\n", guid, err)
	}
	retVal := false
	if len(issues) == 1 {
		retVal = true
		log.Printf("Found existing issues for %s in project (%s). Marking as syncronised.\n", guid, issues[0].WebURL)

	} else if len(issues) > 1 {
		retVal = true
		var urls []string
		for _, issue := range issues {
			urls = append(urls, issue.WebURL)
		}
		log.Printf("Found multiple existing issues for %s in project (%s)\n", guid, strings.Join(urls, ", "))
	}

	return retVal
}

func (feed Feed) checkFeed(redisClient storage.RedisInterface, gitlabClient *gitlab.Client) {
	fp := gofeed.NewParser()
	rss, err := fp.ParseURL(feed.FeedURL)

	if err != nil {
		log.Printf("Unable to parse feed %s: \n %s", feed.Name, err)
		return
	}

	var newArticle []*gofeed.Item
	var oldArticle []*gofeed.Item
	for _, item := range rss.Items {
		// Use our Redis interface with context
		ctx := context.Background()
		found, err := redisClient.SIsMember(ctx, feed.ID, item.GUID).Result()
		if err != nil {
			log.Printf("Error checking database for GUID %s in feed %s: %v", item.GUID, feed.Name, err)
			continue // Skip this item if check fails
		}
		if found {
			oldArticle = append(oldArticle, item)
		} else {
			newArticle = append(newArticle, item)
		}
	}

	log.Printf("Checked feed: %s, New articles: %d, Old articles: %d", feed.Name, len(newArticle), len(oldArticle))

	for _, item := range newArticle {
		var itemTime *time.Time
		// Prefer updated itemTime to published
		if item.UpdatedParsed != nil {
			itemTime = item.UpdatedParsed
		} else {
			itemTime = item.PublishedParsed
		}

		// Check if itemTime is nil before comparing
		if itemTime == nil {
			log.Printf("Skipping item '%s' due to nil date", item.Title)
			continue
		}

		if itemTime.Before(feed.AddedSince) {
			log.Printf("Ignoring '%s' as its date is before the specified AddedSince (Item: %s vs AddedSince: %s)\n",
				item.Title, itemTime, feed.AddedSince)
			// Add using our Redis interface
			ctx := context.Background()
			err := redisClient.SAdd(ctx, feed.ID, item.GUID).Err()
			if err != nil {
				log.Printf("Error adding old GUID %s to database for feed %s: %v", item.GUID, feed.Name, err)
			}
			continue
		}

		// Check Gitlab to see if we already have a matching issue there
		if hasExistingGitlabIssue(item.GUID, feed.GitlabProjectID, gitlabClient) {
			// We think its new but there is already a matching GUID in Gitlab. Mark as Sync'd
			ctx := context.Background()
			err := redisClient.SAdd(ctx, feed.ID, item.GUID).Err()
			if err != nil {
				log.Printf("Error adding existing GUID %s to database for feed %s: %v", item.GUID, feed.Name, err)
			}
			continue
		}

		// Prefer description over content
		var body string
		if item.Description != "" {
			body = item.Description
		} else {
			body = item.Content
		}

		now := time.Now()
		issueTime := &now
		if feed.Retroactive {
			issueTime = itemTime
		}

		// Correctly pass the address of the LabelOptions slice
		labels := gitlab.LabelOptions(feed.Labels) // Create the slice first
		issueOptions := &gitlab.CreateIssueOptions{
			Title:       gitlab.String(item.Title),
			Description: gitlab.String(body + "<br>" + item.Link + "<br>" + item.GUID),
			Labels:      &labels, // Pass the address of the slice
			CreatedAt:   issueTime,
		}

		ctx := context.Background()
		_, _, err := gitlabClient.Issues.CreateIssue(feed.GitlabProjectID, issueOptions, gitlab.WithContext(ctx))
		if err != nil {
			log.Printf("Unable to create Gitlab issue for %s: %v\n", item.Title, err)
			issueCreationErrorCounter.Inc()
			continue
		}

		err = redisClient.SAdd(ctx, feed.ID, item.GUID).Err()
		if err != nil {
			log.Printf("Unable to persist item %s in database: %s \n", item.Title, err)
			continue
		}
		issuesCreatedCounter.Inc()
		if feed.Retroactive {
			log.Printf("Retroactively issue setting date to %s", itemTime)
		}
		log.Printf("Created Gitlab Issue '%s' in project: %d' \n", item.Title, feed.GitlabProjectID)
	}
}

func readConfig(path string) *Config {
	config := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", path, err)
	}

	if err = yaml.Unmarshal(data, config); err != nil {
		log.Printf("Unable to parse config YAML \n %s \n", err)
		panic(err)
	}

	return config
}

func initialise(env EnvValues) (redisClient storage.RedisInterface, client *gitlab.Client, config *Config) {
	// Initialize Prometheus metrics
	gaugeOpts := prometheus.GaugeOpts{
		Name: "last_run_time",
		Help: "Last Run Time in Unix Seconds",
	}
	lastRunGauge = prometheus.NewGauge(gaugeOpts)
	prometheus.MustRegister(lastRunGauge)

	issuesCreatedCounterOpts := prometheus.CounterOpts{
		Name: "issue_creation_total",
		Help: "The total number of issues created in Gitlab since start-up",
	}
	issuesCreatedCounter = prometheus.NewCounter(issuesCreatedCounterOpts)
	prometheus.MustRegister(issuesCreatedCounter)

	issueCreationErrorCountOpts := prometheus.CounterOpts{
		Name: "issue_creation_error_total",
		Help: "The total of failures in creating Gitlab issues since start-up",
	}
	issueCreationErrorCounter = prometheus.NewCounter(issueCreationErrorCountOpts)
	prometheus.MustRegister(issueCreationErrorCounter)

	// Initialize GitLab client
	var err error
	client, err = gitlab.NewClient(env.GitlabAPIKey, gitlab.WithBaseURL(env.GitlabAPIBaseUrl))
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}

	// Read configuration file
	config = readConfig(path.Join(env.ConfDir, "config.yaml"))

	// Create S3 backup configuration
	s3Config := storage.S3BackupConfig{
		Enabled:    env.S3Enabled,
		Endpoint:   env.S3Endpoint,
		Region:     env.S3Region,
		BucketName: env.S3BucketName,
		KeyPrefix:  env.S3KeyPrefix,
		AccessKey:  env.S3AccessKey,
		SecretKey:  env.S3SecretKey,
		Frequency:  env.S3BackupInterval,
	}

	// Initialize SQLite-based Redis store
	redisStore, err := storage.NewRedisStore(env.DBPath, s3Config)
	if err != nil {
		log.Fatalf("Failed to create SQLite database: %v", err)
	}
	redisClient = redisStore.GetClient()

	// Ping to verify connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v", err))
	} else {
		log.Printf("Connected to SQLite database at %s", env.DBPath)
	}

	return
}

func main() {
	flag.Parse()
	env := readEnv()
	redisClient, gitlabClient, config := initialise(env)

	// Register health check
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			ctx := context.Background()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("Health check failed: %v", err)
				http.Error(w, "Unable to connect to the database", http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "All is well!")
		})
	}()

	// Start RSS feed checker
	go func() {
		for {
			log.Printf("Running checks at %s\n", time.Now().Format(time.RFC850))
			for _, configEntry := range config.Feeds {
				configEntry.checkFeed(redisClient, gitlabClient)
			}
			lastRunGauge.SetToCurrentTime()

			// Use config.Interval for sleep duration
			sleepDuration := time.Duration(config.Interval) * time.Second
			if sleepDuration <= 0 {
				sleepDuration = 10 * time.Minute // Default if interval is invalid
				log.Printf("Invalid interval in config, using default: %v", sleepDuration)
			}
			time.Sleep(sleepDuration)
		}
	}()

	// HTTP server for Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting web server on port %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func readEnv() EnvValues {
	var gitlabAPIBaseUrl, gitlabAPIToken, configDir, dbPath string
	var s3Enabled bool
	var s3Endpoint, s3Region, s3BucketName, s3KeyPrefix, s3AccessKey, s3SecretKey string
	var s3BackupInterval time.Duration

	// Required environment variables
	if envGitlabAPIBaseUrl := os.Getenv("GITLAB_API_BASE_URL"); envGitlabAPIBaseUrl == "" {
		panic("Could not find GITLAB_API_BASE_URL specified as an environment variable")
	} else {
		gitlabAPIBaseUrl = envGitlabAPIBaseUrl
	}

	if envGitlabAPIToken := os.Getenv("GITLAB_API_TOKEN"); envGitlabAPIToken == "" {
		panic("Could not find GITLAB_API_TOKEN specified as an environment variable")
	} else {
		gitlabAPIToken = envGitlabAPIToken
	}

	if envConfigDir := os.Getenv("CONFIG_DIR"); envConfigDir == "" {
		panic("Could not find CONFIG_DIR specified as an environment variable")
	} else {
		configDir = envConfigDir
	}

	// SQLite path
	if envDBPath := os.Getenv("DB_PATH"); envDBPath == "" {
		// Default to config dir if not specified
		dbPath = path.Join(configDir, "gitlabrsssync.db")
		log.Printf("Using default database path: %s", dbPath)
	} else {
		dbPath = envDBPath
	}

	// S3 backup configuration (optional)
	if os.Getenv("S3_ENABLED") == "true" {
		s3Enabled = true

		if envS3Endpoint := os.Getenv("S3_ENDPOINT"); envS3Endpoint != "" {
			s3Endpoint = envS3Endpoint
		}

		if envS3Region := os.Getenv("S3_REGION"); envS3Region == "" {
			if s3Enabled {
				log.Printf("S3_REGION not specified, using default: us-east-1")
			}
			s3Region = "us-east-1"
		} else {
			s3Region = envS3Region
		}

		if envS3BucketName := os.Getenv("S3_BUCKET_NAME"); envS3BucketName == "" {
			if s3Enabled {
				panic("S3_BUCKET_NAME is required when S3_ENABLED=true")
			}
		} else {
			s3BucketName = envS3BucketName
		}

		if envS3KeyPrefix := os.Getenv("S3_KEY_PREFIX"); envS3KeyPrefix == "" {
			s3KeyPrefix = "gitlabrsssync"
			if s3Enabled {
				log.Printf("S3_KEY_PREFIX not specified, using default: %s", s3KeyPrefix)
			}
		} else {
			s3KeyPrefix = envS3KeyPrefix
		}

		s3AccessKey = os.Getenv("S3_ACCESS_KEY")
		s3SecretKey = os.Getenv("S3_SECRET_KEY")

		if envS3BackupInterval := os.Getenv("S3_BACKUP_INTERVAL"); envS3BackupInterval == "" {
			s3BackupInterval = 6 * time.Hour
			if s3Enabled {
				log.Printf("S3_BACKUP_INTERVAL not specified, using default: %v", s3BackupInterval)
			}
		} else {
			interval, err := time.ParseDuration(envS3BackupInterval)
			if err != nil {
				log.Printf("Invalid S3_BACKUP_INTERVAL: %v, using default: 6h", err)
				s3BackupInterval = 6 * time.Hour
			} else {
				s3BackupInterval = interval
			}
		}
	}

	return EnvValues{
		DBPath:           dbPath,
		ConfDir:          configDir,
		GitlabAPIKey:     gitlabAPIToken,
		GitlabAPIBaseUrl: gitlabAPIBaseUrl,
		S3Enabled:        s3Enabled,
		S3Endpoint:       s3Endpoint,
		S3Region:         s3Region,
		S3BucketName:     s3BucketName,
		S3KeyPrefix:      s3KeyPrefix,
		S3AccessKey:      s3AccessKey,
		S3SecretKey:      s3SecretKey,
		S3BackupInterval: s3BackupInterval,
	}
}
