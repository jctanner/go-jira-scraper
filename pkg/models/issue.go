package models

import "time"

// Issue represents a JIRA issue without history
type Issue struct {
	ID     string       `json:"id"`
	Key    string       `json:"key"`
	Self   string       `json:"self"`
	Fields *IssueFields `json:"fields"`
}

// IssueWithHistory includes the changelog
type IssueWithHistory struct {
	Issue
	Changelog *Changelog `json:"changelog,omitempty"`
}

// IssueFields contains all JIRA fields
type IssueFields struct {
	Summary        string     `json:"summary"`
	Description    string     `json:"description"`
	IssueType      *IssueType `json:"issuetype"`
	Status         *Status    `json:"status"`
	Priority       *Priority  `json:"priority,omitempty"`
	Assignee       *User      `json:"assignee,omitempty"`
	Creator        *User      `json:"creator"`
	Created        string     `json:"created"`
	Updated        string     `json:"updated"`
	ResolutionDate *string    `json:"resolutiondate,omitempty"`
}

// Changelog contains issue history
type Changelog struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Histories  []History `json:"histories"`
}

// History represents a single change event
type History struct {
	ID      string        `json:"id"`
	Author  *User         `json:"author"`
	Created string        `json:"created"`
	Items   []HistoryItem `json:"items"`
}

// HistoryItem represents a single field change
type HistoryItem struct {
	Field      string  `json:"field"`
	FieldType  string  `json:"fieldtype"`
	From       *string `json:"from"`
	FromString *string `json:"fromString"`
	To         *string `json:"to"`
	ToString   *string `json:"toString"`
}

// User represents a JIRA user
type User struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

// Status represents an issue status
type Status struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Priority represents an issue priority
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IssueType represents an issue type
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CachedIssue wraps the JIRA issue with cache metadata
type CachedIssue struct {
	CacheMetadata CacheMetadata     `json:"_cache_metadata"`
	JiraData      *IssueWithHistory `json:"jira_data"`
}

// CacheMetadata contains information about when and how the issue was cached
type CacheMetadata struct {
	FetchedAt         time.Time `json:"fetched_at"`
	FetchedBy         string    `json:"fetched_by"`
	APICallDurationMS int64     `json:"api_call_duration_ms"`
}

// SearchResult represents the result of a JIRA search
type SearchResult struct {
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Total      int      `json:"total"`
	Issues     []*Issue `json:"issues"`
}



