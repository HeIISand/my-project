package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

func isValidRepo(s string) bool {
	if strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != ""
}

func parseArgs(args []string, getenv func(string) string) (Config, error) {
	var cfg Config
	fs := flag.NewFlagSet("ghreport", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var reposStr string
	var limit int
	var sinceStr string
	var out string
	var trace string
	var cacheDir string

	fs.StringVar(&reposStr, "repos", "", "")
	fs.IntVar(&limit, "limit", 100, "")
	fs.StringVar(&sinceStr, "since", "", "")
	fs.StringVar(&out, "out", "-", "")
	fs.StringVar(&trace, "trace", "", "")
	fs.StringVar(&cacheDir, "cache-dir", "./.ghreport-cache", "")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() > 0 {
		return cfg, errors.New("unexpected positional arguments")
	}

	if reposStr == "" {
		return cfg, errors.New("--repos is required")
	}

	rawRepos := strings.Split(reposStr, ",")
	seen := make(map[string]bool)
	for _, r := range rawRepos {
		r = strings.TrimSpace(r)
		if !isValidRepo(r) {
			return cfg, fmt.Errorf("invalid repo: %s", r)
		}
		if !seen[r] {
			seen[r] = true
			cfg.Repos = append(cfg.Repos, r)
		}
	}

	if limit < 1 {
		return cfg, errors.New("--limit must be >= 1")
	}
	cfg.Limit = limit

	if sinceStr != "" {
		if len(sinceStr) != 10 {
			return cfg, errors.New("invalid --since format")
		}
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return cfg, err
		}
		cfg.Since = t.UTC()
		cfg.HasSince = true
	}

	if out == "" {
		return cfg, errors.New("invalid --out")
	}
	cfg.Out = out
	cfg.Trace = trace

	if cacheDir == "" {
		return cfg, errors.New("invalid --cache-dir")
	}
	cfg.CacheDir = cacheDir

	baseURL := getenv("GITHUB_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	cfg.BaseURL = strings.TrimRight(baseURL, "/")
	cfg.Token = getenv("GITHUB_TOKEN")

	return cfg, nil
}
