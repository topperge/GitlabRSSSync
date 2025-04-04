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

	"github.com/go-redis/redis/v9" // Updated to v9
	"github.com/mmcdole/gofeed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3" // Updated to v3
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
	RedisURL         string
	RedisPassword    string
	ConfDir          string
	GitlabAPIKey     string
	GitlabAPIBaseUrl string
	UseSentinel      bool
}

func hasExistingGitlabIssue(guid string, projectID int, gitlabClient *gitlab.Client) bool {
	// Updated for gitlab.com/gitlab-org/api/client-go
	// Pagination is now typically handled by ListOptions embedded or passed separately.
	// Assuming SearchIssuesByProject still takes ListOptions directly or within SearchOptions.
	// We need to pass *SearchOptions, which embeds ListOptions.
	searchOpts := &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{ // Embed ListOptions
			Page:    1,
			PerPage: 10,
		},
		// Search query (guid) is passed as the second argument to the function
	}
	issues, _, err := gitlabClient.Search.IssuesByProject(projectID, guid, searchOpts) // Pass projectID, guid, and searchOpts
	if err != nil {
		log.Printf("Unable to query Gitlab for existing issues for GUID %s: %v\n", guid, err) // Log the error with GUID
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

func (feed Feed) checkFeed(redisClient *redis.Client, gitlabClient *gitlab.Client) {
	fp := gofeed.NewParser()
	rss, err := fp.ParseURL(feed.FeedURL)

	if err != nil {
		log.Printf("Unable to parse feed %s: \n %s", feed.Name, err)
		return
	}

	var newArticle []*gofeed.Item
	var oldArticle []*gofeed.Item
	for _, item := range rss.Items {
		// Add context.Background() to SIsMember call
		found, err := redisClient.SIsMember(context.Background(), feed.ID, item.GUID).Result()
		if err != nil {
			log.Printf("Error checking Redis for GUID %s in feed %s: %v", item.GUID, feed.Name, err)
			continue // Skip this item if Redis check fails
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
			// Add context.Background() to SAdd call
			err := redisClient.SAdd(context.Background(), feed.ID, item.GUID).Err()
			if err != nil {
				log.Printf("Error adding old GUID %s to Redis for feed %s: %v", item.GUID, feed.Name, err)
			}
			continue
		}

		// Check Gitlab to see if we already have a matching issue there
		if hasExistingGitlabIssue(item.GUID, feed.GitlabProjectID, gitlabClient) {
			// We think its new but there is already a matching GUID in Gitlab.  Mark as Sync'd
			// Add context.Background() to SAdd call
			err := redisClient.SAdd(context.Background(), feed.ID, item.GUID).Err()
			if err != nil {
				log.Printf("Error adding existing GUID %s to Redis for feed %s: %v", item.GUID, feed.Name, err)
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

		// Add context.Background() to CreateIssue call using gitlab.WithContext
		_, _, err := gitlabClient.Issues.CreateIssue(feed.GitlabProjectID, issueOptions, gitlab.WithContext(context.Background()))
		if err != nil {
			log.Printf("Unable to create Gitlab issue for %s: %v\n", item.Title, err) // Log error with item title
			issueCreationErrorCounter.Inc()
			continue
		}
		// Add context.Background() to SAdd call
		err = redisClient.SAdd(context.Background(), feed.ID, item.GUID).Err()
		if err != nil {
			log.Printf("Unable to persist in %s Redis: %s \n", item.Title, err)
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

	data, err := os.ReadFile(path) // Use os.ReadFile instead of ioutil.ReadFile
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", path, err) // Log the error properly
	}

	if err = yaml.Unmarshal(data, config); err != nil {
		log.Printf("Unable to parse config YAML \n %s \n", err)
		panic(err)
	}

	return config
}

func initialise(env EnvValues) (redisClient *redis.Client, client *gitlab.Client, config *Config) {
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
	// Updated for gitlab.com/gitlab-org/api/client-go
	var err error // Declare err variable
	client, err = gitlab.NewClient(env.GitlabAPIKey, gitlab.WithBaseURL(env.GitlabAPIBaseUrl))
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err) // Handle error
	}
	config = readConfig(path.Join(env.ConfDir, "config.yaml"))

	if !env.UseSentinel {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     env.RedisURL,
			Password: env.RedisPassword,
			DB:       0, // use default DB
		})
	} else {
		redisClient = redis.NewFailoverClient(&redis.FailoverOptions{
			SentinelAddrs: []string{env.RedisURL},
			Password:      env.RedisPassword,
			MasterName:    "mymaster", // Ensure this matches your Sentinel config
			DB:            0,          // use default DB
		})
	}

	// Add context.Background() to Ping call
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Sprintf("Unable to connect to Redis @ %s: %v", env.RedisURL, err)) // Log the error
	} else {
		log.Printf("Connected to Redis @ %s", env.RedisURL)
	}

	return
}

func main() {
	env := readEnv()
	redisClient, gitlabClient, config := initialise(env)
	go checkLiveliness(redisClient)
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

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting web server on port %s", *addr) // Log server start
	log.Fatal(http.ListenAndServe(*addr, nil))

}

func readEnv() EnvValues {
	var gitlabAPIBaseUrl, gitlabPAToken, configDir, redisURL, redisPassword string
	useSentinel := false

	if envGitlabAPIBaseUrl := os.Getenv("GITLAB_API_BASE_URL"); envGitlabAPIBaseUrl == "" {
		panic("Could not find GITLAB_API_BASE_URL specified as an environment variable")
	} else {
		gitlabAPIBaseUrl = envGitlabAPIBaseUrl
	}
	if envGitlabAPIToken := os.Getenv("GITLAB_API_TOKEN"); envGitlabAPIToken == "" {
		panic("Could not find GITLAB_API_TOKEN specified as an environment variable")
	} else {
		gitlabPAToken = envGitlabAPIToken
	}
	if envConfigDir := os.Getenv("CONFIG_DIR"); envConfigDir == "" {
		panic("Could not find CONFIG_DIR specified as an environment variable")
	} else {
		configDir = envConfigDir
	}
	if envRedisURL := os.Getenv("REDIS_URL"); envRedisURL == "" {
		panic("Could not find REDIS_URL specified as an environment variable")
	} else {
		redisURL = envRedisURL
	}

	envRedisPassword, hasRedisPasswordEnv := os.LookupEnv("REDIS_PASSWORD")
	if !hasRedisPasswordEnv {
		panic("Could not find REDIS_PASSWORD specified as an environment variable, it may be empty but it must exist")
	} else {
		redisPassword = envRedisPassword
	}

	_, hasRedisSentinel := os.LookupEnv("USE_SENTINEL")
	if hasRedisSentinel {
		log.Printf("Running in sentinel aware mode")
		useSentinel = true
	}

	return EnvValues{
		RedisURL:         redisURL,
		RedisPassword:    redisPassword,
		ConfDir:          configDir,
		GitlabAPIKey:     gitlabPAToken,
		GitlabAPIBaseUrl: gitlabAPIBaseUrl,
		UseSentinel:      useSentinel,
	}
}

func checkLiveliness(client *redis.Client) {
	// Register health check handler on the main HTTP server
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		if err := client.Ping(ctx).Err(); err != nil {
			log.Printf("Health check failed: %v", err)
			http.Error(w, "Unable to connect to the redis master", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "All is well!")
	})
}
