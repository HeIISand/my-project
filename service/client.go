package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var sleepFunc = time.Sleep

func parseNextLink(linkHeader string, currentURL string) string {
	if linkHeader == "" {
		return ""
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		sections := strings.Split(part, ";")
		if len(sections) < 2 {
			continue
		}
		rawURL := strings.TrimSpace(sections[0])
		if !strings.HasPrefix(rawURL, "<") || !strings.HasSuffix(rawURL, ">") {
			continue
		}
		rawURL = rawURL[1 : len(rawURL)-1]

		isNext := false
		for _, sec := range sections[1:] {
			sec = strings.TrimSpace(sec)
			if sec == `rel="next"` || sec == "rel=next" {
				isNext = true
				break
			}
		}
		if isNext {
			base, err := url.Parse(currentURL)
			if err != nil {
				return rawURL
			}
			ref, err := url.Parse(rawURL)
			if err != nil {
				return rawURL
			}
			return base.ResolveReference(ref).String()
		}
	}
	return ""
}

func isRateLimited(resp *http.Response, body []byte) bool {
	if resp.StatusCode != 403 && resp.StatusCode != 429 {
		return false
	}
	if resp.StatusCode == 429 {
		return true
	}
	if resp.Header.Get("Retry-After") != "" {
		return true
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return true
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "rate limit") || strings.Contains(lower, "secondary rate") || strings.Contains(lower, "abuse")
}

func rateLimitDelay(h http.Header) time.Duration {
	if ra := strings.TrimSpace(h.Get("Retry-After")); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			if secs < 0 {
				secs = 0
			}
			return time.Duration(secs) * time.Second
		}
		if t, err := http.ParseTime(ra); err == nil {
			d := time.Until(t)
			if d < 0 {
				d = 0
			}
			return d
		}
	}
	if resetStr := strings.TrimSpace(h.Get("X-RateLimit-Reset")); resetStr != "" {
		if resetEpoch, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			wait := resetEpoch - time.Now().UTC().Unix()
			if wait < 0 {
				wait = 0
			}
			return time.Duration(wait) * time.Second
		}
	}
	return 60 * time.Second
}

func classifyNetError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline") {
		return "timeout"
	}
	return "network"
}

func retryBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Second
	case 2:
		return 2 * time.Second
	default:
		return 4 * time.Second
	}
}

func fetchPage(client *http.Client, reqURL string, cfg Config, traceWriter io.Writer) ([]byte, string, string, bool) {
	cached, hasCache := loadCache(cfg.CacheDir, reqURL)

	for attempt := 1; attempt <= 4; attempt++ {
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			errStr := "network"
			writeTrace(traceWriter, TraceEntry{
				TS:        time.Now().UTC().Format(time.RFC3339),
				Method:    "GET",
				URL:       reqURL,
				Status:    0,
				Attempt:   attempt,
				FromCache: false,
				Error:     &errStr,
			})
			return nil, "", "request_failed", false
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "ghreport/1.0")
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		if cfg.Token != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
		}
		if hasCache && cached.ETag != "" {
			req.Header.Set("If-None-Match", cached.ETag)
		}

		resp, err := client.Do(req)
		if err != nil {
			errStr := classifyNetError(err)
			writeTrace(traceWriter, TraceEntry{
				TS:        time.Now().UTC().Format(time.RFC3339),
				Method:    "GET",
				URL:       reqURL,
				Status:    0,
				Attempt:   attempt,
				FromCache: false,
				Error:     &errStr,
			})
			if attempt <= 3 {
				sleepFunc(retryBackoff(attempt))
				continue
			}
			return nil, "", "request_failed", false
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			errStr := "network"
			writeTrace(traceWriter, TraceEntry{
				TS:        time.Now().UTC().Format(time.RFC3339),
				Method:    "GET",
				URL:       reqURL,
				Status:    resp.StatusCode,
				Attempt:   attempt,
				FromCache: false,
				Error:     &errStr,
			})
			if attempt <= 3 {
				sleepFunc(retryBackoff(attempt))
				continue
			}
			return nil, "", "request_failed", false
		}

		if resp.StatusCode == http.StatusNotModified {
			if !hasCache || cached.ETag == "" {
				writeTrace(traceWriter, TraceEntry{
					TS:        time.Now().UTC().Format(time.RFC3339),
					Method:    "GET",
					URL:       reqURL,
					Status:    resp.StatusCode,
					Attempt:   attempt,
					FromCache: false,
					Error:     nil,
				})
				return nil, "", "invalid_response", false
			}
			writeTrace(traceWriter, TraceEntry{
				TS:        time.Now().UTC().Format(time.RFC3339),
				Method:    "GET",
				URL:       reqURL,
				Status:    resp.StatusCode,
				Attempt:   attempt,
				FromCache: true,
				Error:     nil,
			})
			linkVal := strings.Join(resp.Header.Values("Link"), ", ")
			if linkVal == "" {
				linkVal = cached.Link
			}
			return cached.Body, linkVal, "", false
		}

		writeTrace(traceWriter, TraceEntry{
			TS:        time.Now().UTC().Format(time.RFC3339),
			Method:    "GET",
			URL:       reqURL,
			Status:    resp.StatusCode,
			Attempt:   attempt,
			FromCache: false,
			Error:     nil,
		})

		if resp.StatusCode == http.StatusOK {
			linkVal := strings.Join(resp.Header.Values("Link"), ", ")
			etag := resp.Header.Get("ETag")
			if etag != "" {
				saveCache(cfg.CacheDir, reqURL, CacheEntry{
					ETag: etag,
					Link: linkVal,
					Body: bodyBytes,
				})
			}
			return bodyBytes, linkVal, "", false
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, "", "", true
		}

		if resp.StatusCode == http.StatusNotFound {
			return nil, "", "not_found", false
		}

		if isRateLimited(resp, bodyBytes) {
			if attempt <= 3 {
				sleepFunc(rateLimitDelay(resp.Header))
				continue
			}
			return nil, "", "rate_limited", false
		}

		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			if attempt <= 3 {
				sleepFunc(retryBackoff(attempt))
				continue
			}
			return nil, "", "request_failed", false
		}

		return nil, "", "request_failed", false
	}

	return nil, "", "request_failed", false
}

func processRepo(client *http.Client, repo string, cfg Config, traceWriter io.Writer) (RepoReport, bool) {
	report := RepoReport{
		Repo:   repo,
		Error:  nil,
		Count:  0,
		Issues: []Issue{},
	}

	nextURL := fmt.Sprintf("%s/repos/%s/issues?state=open&sort=created&direction=asc&per_page=100", cfg.BaseURL, repo)
	seenNumbers := make(map[int]bool)

	for nextURL != "" && len(report.Issues) < cfg.Limit {
		bodyBytes, linkHeader, repoErr, unauthorized := fetchPage(client, nextURL, cfg, traceWriter)
		if unauthorized {
			return report, true
		}
		if repoErr != "" {
			errCopy := repoErr
			report.Error = &errCopy
			report.Issues = []Issue{}
			report.Count = 0
			return report, false
		}

		var rawIssues []apiIssue
		if err := json.Unmarshal(bodyBytes, &rawIssues); err != nil {
			errCopy := "invalid_response"
			report.Error = &errCopy
			report.Issues = []Issue{}
			report.Count = 0
			return report, false
		}

		for _, item := range rawIssues {
			if item.Number == nil || item.Title == nil || item.User == nil || item.CreatedAt == nil || item.HTMLURL == nil {
				errCopy := "invalid_response"
				report.Error = &errCopy
				report.Issues = []Issue{}
				report.Count = 0
				return report, false
			}

			createdTime, err := time.Parse(time.RFC3339, *item.CreatedAt)
			if err != nil {
				errCopy := "invalid_response"
				report.Error = &errCopy
				report.Issues = []Issue{}
				report.Count = 0
				return report, false
			}

			if len(item.PullRequest) > 0 && string(item.PullRequest) != "null" {
				continue
			}

			if cfg.HasSince && createdTime.Before(cfg.Since) {
				continue
			}

			num := *item.Number
			if seenNumbers[num] {
				continue
			}
			seenNumbers[num] = true

			labels := make([]string, 0, len(item.Labels))
			for _, l := range item.Labels {
				labels = append(labels, l.Name)
			}

			report.Issues = append(report.Issues, Issue{
				Number:    num,
				Title:     *item.Title,
				Author:    item.User.Login,
				Labels:    labels,
				CreatedAt: *item.CreatedAt,
				URL:       *item.HTMLURL,
			})

			if len(report.Issues) >= cfg.Limit {
				break
			}
		}

		if len(report.Issues) >= cfg.Limit {
			break
		}

		nextURL = parseNextLink(linkHeader, nextURL)
	}

	sort.Slice(report.Issues, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, report.Issues[i].CreatedAt)
		tj, _ := time.Parse(time.RFC3339, report.Issues[j].CreatedAt)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return report.Issues[i].Number < report.Issues[j].Number
	})

	report.Count = len(report.Issues)
	return report, false
}
