package scraper

import (
	"fmt"
	"log"
	"time"

	"github.com/jctanner/go-jira-scraper/pkg/cache"
	"github.com/jctanner/go-jira-scraper/pkg/jira"
)

// Scraper orchestrates the scraping process
type Scraper struct {
	client *jira.Client
	cache  *cache.DiskCache
	config Config
}

// Config holds scraper configuration
type Config struct {
	Workers   int
	FullSync  bool
	BatchSize int
	Limit     int
}

// ScrapeResult contains the results of a scrape operation
type ScrapeResult struct {
	IssuesProcessed int
	APICalls        int
	CacheHits       int
	Errors          int
	Duration        time.Duration
}

// New creates a new Scraper instance
func New(client *jira.Client, cache *cache.DiskCache, config Config) *Scraper {
	// Set defaults
	if config.Workers == 0 {
		config.Workers = 4
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	return &Scraper{
		client: client,
		cache:  cache,
		config: config,
	}
}

// ScrapeProject fetches all issues from a project
func (s *Scraper) ScrapeProject(project string) (*ScrapeResult, error) {
	start := time.Now()
	result := &ScrapeResult{}

	log.Printf("Starting scrape of project: %s", project)

	// Get all issue keys from JIRA
	log.Printf("Searching for issues in project %s...", project)
	issueKeys, err := s.client.GetAllIssuesInProject(project, "updated DESC", s.config.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	log.Printf("Found %d issues in project %s", len(issueKeys), project)
	result.IssuesProcessed = len(issueKeys)

	// Determine which issues need fetching
	toFetch := []string{}
	for _, key := range issueKeys {
		if s.config.FullSync {
			// Full sync: fetch everything
			toFetch = append(toFetch, key)
		} else {
			// Incremental: only fetch if not in cache or outdated
			if !s.cache.Exists(key) {
				toFetch = append(toFetch, key)
			} else {
				result.CacheHits++
			}
		}
	}

	log.Printf("Need to fetch %d issues (%d cache hits)", len(toFetch), result.CacheHits)

	// Fetch issues (for now, sequentially - we'll add concurrency later)
	for i, key := range toFetch {
		log.Printf("Fetching %d/%d: %s", i+1, len(toFetch), key)
		
		issue, duration, err := s.client.GetIssueWithHistory(key)
		if err != nil {
			log.Printf("Error fetching %s: %v", key, err)
			result.Errors++
			continue
		}
		result.APICalls++

		// Store in cache
		_, err = s.cache.WriteIssue(issue, duration)
		if err != nil {
			log.Printf("Error caching %s: %v", key, err)
			result.Errors++
			continue
		}

		// Delay to avoid hitting rate limits (be polite to the API)
		time.Sleep(500 * time.Millisecond)
	}

	result.Duration = time.Since(start)
	log.Printf("Scrape complete: %d issues, %d API calls, %d cache hits, %d errors in %s",
		result.IssuesProcessed, result.APICalls, result.CacheHits, result.Errors, result.Duration)

	return result, nil
}

// ScrapeIssue fetches a single issue
func (s *Scraper) ScrapeIssue(key string) error {
	log.Printf("Fetching issue: %s", key)

	issue, duration, err := s.client.GetIssueWithHistory(key)
	if err != nil {
		return fmt.Errorf("failed to fetch issue: %w", err)
	}

	_, err = s.cache.WriteIssue(issue, duration)
	if err != nil {
		return fmt.Errorf("failed to cache issue: %w", err)
	}

	log.Printf("Successfully fetched and cached %s", key)
	return nil
}

// ValidateCache checks cache integrity
func (s *Scraper) ValidateCache() error {
	log.Println("Validating cache...")
	
	keys, err := s.cache.ListIssues()
	if err != nil {
		return fmt.Errorf("failed to list cached issues: %w", err)
	}

	log.Printf("Found %d cached issues", len(keys))
	
	errors := 0
	for _, key := range keys {
		_, err := s.cache.GetIssue(key)
		if err != nil {
			log.Printf("Error reading %s: %v", key, err)
			errors++
		}
	}

	if errors > 0 {
		log.Printf("Cache validation found %d errors", errors)
	} else {
		log.Println("Cache validation passed")
	}

	return nil
}

