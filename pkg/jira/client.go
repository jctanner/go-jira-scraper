package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jctanner/go-jira-scraper/pkg/models"
)

// Client handles interactions with the JIRA API
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	batchSize  int
}

// New creates a new JIRA client
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		batchSize: 10, // Default to 10 for JIRA rate limit compatibility
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBatchSize sets the batch size for search queries
func (c *Client) SetBatchSize(size int) {
	if size > 0 && size <= 100 {
		c.batchSize = size
	}
}

// doRequest performs an HTTP request with authentication and retry logic
func (c *Client) doRequest(method, path string, query url.Values) ([]byte, error) {
	return c.doRequestWithRetry(method, path, query, 3)
}

// doRequestWithRetry performs an HTTP request with retry logic for rate limits
func (c *Client) doRequestWithRetry(method, path string, query url.Values, maxRetries int) ([]byte, error) {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	log.Printf("Request: %s %s", method, reqURL)

	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d/%d", attempt, maxRetries)
		}

		req, err := http.NewRequest(method, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < maxRetries {
				waitTime := time.Duration(1<<uint(attempt+1)) * time.Second
				log.Printf("Request error. Waiting %v before retry...", waitTime)
				time.Sleep(waitTime)
			}
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			if attempt < maxRetries {
				waitTime := time.Duration(1<<uint(attempt+1)) * time.Second
				log.Printf("Read error. Waiting %v before retry...", waitTime)
				time.Sleep(waitTime)
			}
			continue
		}

		// Success!
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Request successful (status %d)", resp.StatusCode)
			return body, nil
		}

		// Handle rate limiting (429)
		if resp.StatusCode == 429 {
			if attempt >= maxRetries {
				log.Printf("Rate limit exceeded and max retries (%d) reached. Giving up.", maxRetries)
				return nil, fmt.Errorf("rate limit max retries exceeded after %d attempts: %s", maxRetries, string(body))
			}

			// Check for Retry-After header
			var waitTime time.Duration
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					waitTime = time.Duration(seconds) * time.Second
				}
			}
			
			// If no valid Retry-After, use exponential backoff
			if waitTime == 0 {
				waitTime = time.Duration(1<<uint(attempt+1)) * time.Second
			}
			
			log.Printf("Rate limited (429). Waiting %v before retry...", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Other errors (don't retry)
		log.Printf("API error: status %d", resp.StatusCode)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Max retries (%d) exceeded", maxRetries)
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Search executes a JQL query and returns issue keys
func (c *Client) Search(jql string, maxResults int, startAt int) (*models.SearchResult, error) {
	query := url.Values{}
	query.Set("jql", jql)
	query.Set("maxResults", fmt.Sprintf("%d", maxResults))
	query.Set("startAt", fmt.Sprintf("%d", startAt))
	query.Set("fields", "id,key,summary,updated")

	body, err := c.doRequest("GET", "/rest/api/2/search", query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var result models.SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return &result, nil
}

// GetIssue fetches a single issue without history
func (c *Client) GetIssue(key string) (*models.Issue, error) {
	path := fmt.Sprintf("/rest/api/2/issue/%s", key)
	
	body, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	var issue models.Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to parse issue: %w", err)
	}

	return &issue, nil
}

// GetIssueWithHistory fetches issue with complete changelog
func (c *Client) GetIssueWithHistory(key string) (*models.IssueWithHistory, time.Duration, error) {
	start := time.Now()
	
	path := fmt.Sprintf("/rest/api/2/issue/%s", key)
	query := url.Values{}
	query.Set("expand", "changelog")
	
	body, err := c.doRequest("GET", path, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get issue with history: %w", err)
	}

	var issue models.IssueWithHistory
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, 0, fmt.Errorf("failed to parse issue: %w", err)
	}

	duration := time.Since(start)
	return &issue, duration, nil
}

// GetAllIssuesInProject fetches all issue keys for a project
func (c *Client) GetAllIssuesInProject(project string, orderBy string, limit int) ([]string, error) {
	if orderBy == "" {
		orderBy = "updated DESC"
	}
	
	jql := fmt.Sprintf("project = %s ORDER BY %s", project, orderBy)
	
	var allKeys []string
	startAt := 0

	log.Printf("Searching with batch size: %d", c.batchSize)
	if limit > 0 {
		log.Printf("Limiting search to %d issues", limit)
	}

	for {
		result, err := c.Search(jql, c.batchSize, startAt)
		if err != nil {
			return nil, err
		}

		for _, issue := range result.Issues {
			allKeys = append(allKeys, issue.Key)
			
			// Check if we've hit the limit
			if limit > 0 && len(allKeys) >= limit {
				log.Printf("Reached limit of %d issues, stopping search", limit)
				return allKeys, nil
			}
		}

		// Check if we've fetched all issues
		if startAt+len(result.Issues) >= result.Total {
			break
		}

		startAt += len(result.Issues)
		
		// Small delay between pagination requests to avoid rate limits
		time.Sleep(500 * time.Millisecond)
	}

	return allKeys, nil
}

// TestConnection verifies the JIRA connection and authentication
func (c *Client) TestConnection() error {
	_, err := c.doRequest("GET", "/rest/api/2/myself", nil)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

