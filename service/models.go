package main

import (
	"encoding/json"
	"time"
)

type Config struct {
	Repos    []string
	Limit    int
	Since    time.Time
	HasSince bool
	Out      string
	Trace    string
	CacheDir string
	BaseURL  string
	Token    string
}

type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"created_at"`
	URL       string   `json:"url"`
}

type RepoReport struct {
	Repo   string  `json:"repo"`
	Error  *string `json:"error"`
	Count  int     `json:"count"`
	Issues []Issue `json:"issues"`
}

type Report struct {
	GeneratedAt string       `json:"generated_at"`
	Repos       []RepoReport `json:"repos"`
}

type TraceEntry struct {
	TS        string  `json:"ts"`
	Method    string  `json:"method"`
	URL       string  `json:"url"`
	Status    int     `json:"status"`
	Attempt   int     `json:"attempt"`
	FromCache bool    `json:"from_cache"`
	Error     *string `json:"error"`
}

type CacheEntry struct {
	ETag string `json:"etag"`
	Link string `json:"link"`
	Body []byte `json:"body"`
}

type apiLabel struct {
	Name string `json:"name"`
}

type apiUser struct {
	Login string `json:"login"`
}

type apiIssue struct {
	Number      *int            `json:"number"`
	Title       *string         `json:"title"`
	User        *apiUser        `json:"user"`
	Labels      []apiLabel      `json:"labels"`
	CreatedAt   *string         `json:"created_at"`
	HTMLURL     *string         `json:"html_url"`
	PullRequest json.RawMessage `json:"pull_request"`
}
