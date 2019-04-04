package main

import (
	"flag"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/mmcdole/gofeed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var lastRunGauge prometheus.Gauge
var issuesCreatedCounter prometheus.Counter

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
}

type EnvValues struct {
	RedisURL      string
	RedisPassword string
	ConfDir       string
	GitlabAPIKey  string
}

func hasExistingGitlabIssue(guid string, projectID int, gitlabClient *gitlab.Client) bool {
	searchOptions := &gitlab.SearchOptions{
		Page:    1,
		PerPage: 10,
	}
	issues, _, err := gitlabClient.Search.IssuesByProject(projectID, guid, searchOptions)
	if err != nil {
		log.Printf("Unable to query Gitlab for existing issues\n")
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
		found := redisClient.SIsMember(feed.ID, item.GUID).Val()
		if found == true {
			oldArticle = append(oldArticle, item)
		} else {
			newArticle = append(newArticle, item)
		}
	}

	log.Printf("Checked feed: %s, New articles: %d, Old articles: %d", feed.Name, len(newArticle), len(oldArticle))

	for _, item := range newArticle {
		var time *time.Time
		// Prefer updated time to published
		if item.UpdatedParsed != nil {
			time = item.UpdatedParsed
		} else {
			time = item.PublishedParsed
		}

		if time.Before(feed.AddedSince) {
			log.Printf("Ignoring '%s' as its date is before the specified AddedSince (Item: %s vs AddedSince: %s)\n",
				item.Title, time, feed.AddedSince)
			redisClient.SAdd(feed.ID, item.GUID)
			continue
		}

		// Check Gitlab to see if we already have a matching issue there
		if hasExistingGitlabIssue(item.GUID, feed.GitlabProjectID, gitlabClient) {
			// We think its new but there is already a matching GUID in Gitlab.  Mark as Sync'd
			redisClient.SAdd(feed.ID, item.GUID)
			continue
		}

		// Prefer description over content
		var body string
		if item.Description != "" {
			body = item.Description
		} else {
			body = item.Content
		}

		issueOptions := &gitlab.CreateIssueOptions{
			Title:       gitlab.String(item.Title),
			Description: gitlab.String(body + "\n" + item.GUID),
			Labels:      feed.Labels,
			CreatedAt:   time,
		}

		if _, _, err := gitlabClient.Issues.CreateIssue(feed.GitlabProjectID, issueOptions); err != nil {
			log.Printf("Unable to create Gitlab issue for %s \n %s \n", feed.Name, err)
			continue
		}
		if err := redisClient.SAdd(feed.ID, item.GUID).Err(); err != nil {
			log.Printf("Unable to persist in %s Redis: %s \n", item.Title, err)
			continue
		}
		issuesCreatedCounter.Inc()
		log.Printf("Created Gitlab Issue '%s' in project: %d' \n", item.Title, feed.GitlabProjectID)
	}
}

func readConfig(path string) *Config {
	config := &Config{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
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
		Name: "issues_created",
		Help: "Number of issues created in Gitlab",
	}
	issuesCreatedCounter = prometheus.NewCounter(issuesCreatedCounterOpts)
	prometheus.MustRegister(issuesCreatedCounter)

	client = gitlab.NewClient(nil, env.GitlabAPIKey)
	config = readConfig(path.Join(env.ConfDir, "config.yaml"))

	redisClient = redis.NewClient(&redis.Options{
		Addr:     env.RedisURL,
		Password: env.RedisPassword,
		DB:       0, // use default DB
	})

	if err := redisClient.Ping().Err(); err != nil {
		panic(fmt.Sprintf("Unable to connect to Redis @ %s", env.RedisURL))
	} else {
		log.Printf("Connected to Redis @ %s", env.RedisURL)
	}

	return
}

func main() {
	env := readEnv()
	redisClient, gitlabClient, config := initialise(env)

	go func() {
		for {
			log.Printf("Running checks at %s\n", time.Now().Format(time.RFC850))
			for _, configEntry := range config.Feeds {
				configEntry.checkFeed(redisClient, gitlabClient)
			}
			lastRunGauge.SetToCurrentTime()
			time.Sleep(time.Duration(config.Interval) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))

}

func readEnv() EnvValues {
	var gitlabPAToken, configDir, redisURL, redisPassword string
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

	return EnvValues{
		RedisURL:      redisURL,
		RedisPassword: redisPassword,
		ConfDir:       configDir,
		GitlabAPIKey:  gitlabPAToken,
	}
}
