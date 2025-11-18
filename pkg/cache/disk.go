package cache

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jctanner/go-jira-scraper/pkg/models"
)

// DiskCache manages the local disk cache for JIRA issues
type DiskCache struct {
	baseDir  string
	jiraHost string // Hostname of JIRA instance for namespacing
}

// New creates a new DiskCache instance
func New(baseDir string) *DiskCache {
	return &DiskCache{
		baseDir:  baseDir,
		jiraHost: "", // Will be set when Initialize is called with jiraURL
	}
}

// NewWithHost creates a new DiskCache instance with a specific JIRA host
func NewWithHost(baseDir string, jiraURL string) *DiskCache {
	host := extractHostname(jiraURL)
	return &DiskCache{
		baseDir:  baseDir,
		jiraHost: host,
	}
}

// extractHostname extracts the hostname from a JIRA URL
func extractHostname(jiraURL string) string {
	parsed, err := url.Parse(jiraURL)
	if err != nil {
		// Fallback to a safe default
		return "unknown"
	}
	return parsed.Host
}

// getDataPath returns the base path for data storage
// Format: .data/jira/<hostname>/
func (d *DiskCache) getDataPath() string {
	if d.jiraHost != "" {
		return filepath.Join(d.baseDir, "jira", d.jiraHost)
	}
	// Backward compatibility: if no host set, use old structure
	return d.baseDir
}

// Initialize creates the cache directory structure
func (d *DiskCache) Initialize() error {
	dataPath := d.getDataPath()
	dirs := []string{
		filepath.Join(dataPath, "by_id"),
		filepath.Join(dataPath, "by_key"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// WriteIssue stores an issue to disk with fetch metadata
func (d *DiskCache) WriteIssue(issue *models.IssueWithHistory, duration time.Duration) (string, error) {
	dataPath := d.getDataPath()
	
	// Wrap with cache metadata
	cached := &models.CachedIssue{
		CacheMetadata: models.CacheMetadata{
			FetchedAt:         time.Now().UTC(),
			FetchedBy:         "go-jira-scraper/0.1.0",
			APICallDurationMS: duration.Milliseconds(),
		},
		JiraData: issue,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal issue: %w", err)
	}

	// Write to by_id directory
	idPath := filepath.Join(dataPath, "by_id", issue.ID+".json")
	if err := os.WriteFile(idPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write issue file: %w", err)
	}

	// Create symlink in by_key directory
	keyPath := filepath.Join(dataPath, "by_key", issue.Key+".json")
	relPath := filepath.Join("..", "by_id", issue.ID+".json")

	// Remove existing symlink if it exists
	os.Remove(keyPath)

	// Create new symlink
	if err := os.Symlink(relPath, keyPath); err != nil {
		// Not fatal if symlink creation fails (e.g., on Windows without permissions)
		// The file is still accessible via by_id
		fmt.Fprintf(os.Stderr, "Warning: failed to create symlink %s: %v\n", keyPath, err)
	}

	return idPath, nil
}

// GetIssue retrieves an issue from disk by key
func (d *DiskCache) GetIssue(key string) (*models.CachedIssue, error) {
	dataPath := d.getDataPath()
	keyPath := filepath.Join(dataPath, "by_key", key+".json")
	return d.readIssueFile(keyPath)
}

// GetIssueByID retrieves an issue from disk by ID
func (d *DiskCache) GetIssueByID(id string) (*models.CachedIssue, error) {
	dataPath := d.getDataPath()
	idPath := filepath.Join(dataPath, "by_id", id+".json")
	return d.readIssueFile(idPath)
}

// readIssueFile reads and unmarshals an issue file
func (d *DiskCache) readIssueFile(path string) (*models.CachedIssue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("issue not found in cache")
		}
		return nil, fmt.Errorf("failed to read issue file: %w", err)
	}

	var cached models.CachedIssue
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue: %w", err)
	}

	return &cached, nil
}

// GetLastFetched returns when an issue was last fetched (uses file mtime as fallback)
func (d *DiskCache) GetLastFetched(key string) (time.Time, error) {
	// Try embedded metadata first
	cached, err := d.GetIssue(key)
	if err == nil && !cached.CacheMetadata.FetchedAt.IsZero() {
		return cached.CacheMetadata.FetchedAt, nil
	}

	// Fall back to file modification time
	dataPath := d.getDataPath()
	keyPath := filepath.Join(dataPath, "by_key", key+".json")
	info, err := os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, fmt.Errorf("issue not found in cache")
		}
		return time.Time{}, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.ModTime(), nil
}

// Exists checks if an issue exists in the cache
func (d *DiskCache) Exists(key string) bool {
	dataPath := d.getDataPath()
	keyPath := filepath.Join(dataPath, "by_key", key+".json")
	_, err := os.Stat(keyPath)
	return err == nil
}

// ListIssues returns all cached issue keys
func (d *DiskCache) ListIssues() ([]string, error) {
	dataPath := d.getDataPath()
	keyDir := filepath.Join(dataPath, "by_key")
	entries, err := os.ReadDir(keyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			key := strings.TrimSuffix(entry.Name(), ".json")
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// ListIssuesForProject returns all cached issue keys for a specific project
func (d *DiskCache) ListIssuesForProject(project string) ([]string, error) {
	allKeys, err := d.ListIssues()
	if err != nil {
		return nil, err
	}

	var projectKeys []string
	for _, key := range allKeys {
		if strings.HasPrefix(key, project+"-") {
			projectKeys = append(projectKeys, key)
		}
	}

	return projectKeys, nil
}


