package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	sleepFunc = func(d time.Duration) {}
}

func makeEnv(baseURL string, token string) func(string) string {
	return func(k string) string {
		switch k {
		case "GITHUB_API_BASE_URL":
			return baseURL
		case "GITHUB_TOKEN":
			return token
		default:
			return ""
		}
	}
}

func TestParseArgs(t *testing.T) {
	cfg, err := parseArgs([]string{"--repos", "cli/cli,golang/go,cli/cli", "--limit", "10", "--since", "2026-01-05"}, makeEnv("https://api.github.com/", "sec"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 2 || cfg.Repos[0] != "cli/cli" || cfg.Repos[1] != "golang/go" {
		t.Fatalf("unexpected repos: %v", cfg.Repos)
	}
	if cfg.Limit != 10 {
		t.Fatalf("unexpected limit: %d", cfg.Limit)
	}
	if !cfg.HasSince || cfg.Since.Format("2006-01-02") != "2026-01-05" {
		t.Fatalf("unexpected since: %v", cfg.Since)
	}
	if cfg.BaseURL != "https://api.github.com" {
		t.Fatalf("unexpected base url: %s", cfg.BaseURL)
	}

	invalidCases := [][]string{
		{},
		{"--repos", ""},
		{"--repos", "invalid"},
		{"--repos", "a/b/c"},
		{"--repos", "cli/cli", "--limit", "0"},
		{"--repos", "cli/cli", "--limit", "-1"},
		{"--repos", "cli/cli", "--since", "2026-13-45"},
		{"--repos", "cli/cli", "extra"},
	}
	for _, tc := range invalidCases {
		if _, err := parseArgs(tc, makeEnv("", "")); err == nil {
			t.Fatalf("expected error for args: %v", tc)
		}
	}
}

func TestPaginationLimitAndDeduplication(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/o/r/issues?page=2>; rel="next"`, srv.URL))
			_, _ = w.Write([]byte(`[
				{"number": 1, "title": "Issue 1", "user": {"login": "u1"}, "labels": [{"name": "bug"}], "created_at": "2026-01-01T10:00:00Z", "html_url": "http://example.com/1"},
				{"number": 2, "title": "PR 2", "user": {"login": "u2"}, "labels": [], "created_at": "2026-01-02T10:00:00Z", "html_url": "http://example.com/2", "pull_request": {"url": "http://example.com/pr/2"}},
				{"number": 3, "title": "Issue 3", "user": {"login": "u3"}, "labels": [], "created_at": "2026-01-03T10:00:00Z", "html_url": "http://example.com/3"}
			]`))
			return
		}
		_, _ = w.Write([]byte(`[
			{"number": 3, "title": "Issue 3 dup", "user": {"login": "u3"}, "labels": [], "created_at": "2026-01-03T10:00:00Z", "html_url": "http://example.com/3"},
			{"number": 4, "title": "Issue 4", "user": {"login": "u4"}, "labels": [], "created_at": "2026-01-04T10:00:00Z", "html_url": "http://example.com/4"},
			{"number": 5, "title": "Issue 5", "user": {"login": "u5"}, "labels": [], "created_at": "2026-01-05T10:00:00Z", "html_url": "http://example.com/5"}
		]`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	var outBuf, errBuf bytes.Buffer
	code := run([]string{"--repos", "o/r", "--limit", "3", "--cache-dir", filepath.Join(tmp, "cache")}, makeEnv(srv.URL, ""), &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr: %s", code, errBuf.String())
	}

	var rep Report
	if err := json.Unmarshal(outBuf.Bytes(), &rep); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(rep.Repos) != 1 || rep.Repos[0].Count != 3 {
		t.Fatalf("expected 3 issues, got: %+v", rep.Repos)
	}
	nums := []int{rep.Repos[0].Issues[0].Number, rep.Repos[0].Issues[1].Number, rep.Repos[0].Issues[2].Number}
	if nums[0] != 1 || nums[1] != 3 || nums[2] != 4 {
		t.Fatalf("unexpected issue numbers: %v", nums)
	}
}

func TestCachingAndTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"etag-1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"etag-1"`)
		_, _ = w.Write([]byte(`[
			{"number": 10, "title": "Cached", "user": {"login": "alice"}, "labels": [], "created_at": "2026-02-01T00:00:00Z", "html_url": "http://example.com/10"}
		]`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	trace1 := filepath.Join(tmp, "trace1.jsonl")
	trace2 := filepath.Join(tmp, "trace2.jsonl")

	var out1, err1 bytes.Buffer
	code1 := run([]string{"--repos", "o/r", "--cache-dir", cacheDir, "--trace", trace1}, makeEnv(srv.URL, "secret-token-123"), &out1, &err1)
	if code1 != 0 {
		t.Fatalf("run 1 failed: %d", code1)
	}

	var out2, err2 bytes.Buffer
	code2 := run([]string{"--repos", "o/r", "--cache-dir", cacheDir, "--trace", trace2}, makeEnv(srv.URL, "secret-token-123"), &out2, &err2)
	if code2 != 0 {
		t.Fatalf("run 2 failed: %d", code2)
	}

	var r1, r2 Report
	_ = json.Unmarshal(out1.Bytes(), &r1)
	_ = json.Unmarshal(out2.Bytes(), &r2)
	if len(r1.Repos[0].Issues) != 1 || len(r2.Repos[0].Issues) != 1 || r1.Repos[0].Issues[0].Number != r2.Repos[0].Issues[0].Number {
		t.Fatalf("reports mismatch: %+v vs %+v", r1, r2)
	}

	t2Data, err := os.ReadFile(trace2)
	if err != nil {
		t.Fatalf("failed reading trace2: %v", err)
	}
	if strings.Contains(string(t2Data), "secret-token-123") {
		t.Fatalf("token leaked into trace")
	}
	var te TraceEntry
	if err := json.Unmarshal(bytes.TrimSpace(t2Data), &te); err != nil {
		t.Fatalf("invalid trace json: %v", err)
	}
	if !te.FromCache || te.Status != 304 {
		t.Fatalf("expected from_cache=true and status=304, got %+v", te)
	}
}

func TestRateLimitAndErrors(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/o/unauth/") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/repos/o/missing/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "/repos/o/ratelimited/") {
			n := atomic.AddInt32(&attempts, 1)
			if n == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte(`[{"number": 1, "title": "OK", "user": {"login": "u"}, "labels": [], "created_at": "2026-01-01T00:00:00Z", "html_url": "http://example.com/1"}]`))
			return
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()

	var outBuf, errBuf bytes.Buffer
	code := run([]string{"--repos", "o/ratelimited,o/missing", "--cache-dir", filepath.Join(tmp, "c1")}, makeEnv(srv.URL, ""), &outBuf, &errBuf)
	if code != 1 {
		t.Fatalf("expected exit 1 for partial failure, got %d", code)
	}
	var rep Report
	_ = json.Unmarshal(outBuf.Bytes(), &rep)
	if rep.Repos[0].Error != nil || rep.Repos[0].Count != 1 {
		t.Fatalf("expected repo 0 success, got %+v", rep.Repos[0])
	}
	if rep.Repos[1].Error == nil || *rep.Repos[1].Error != "not_found" {
		t.Fatalf("expected repo 1 not_found, got %+v", rep.Repos[1])
	}

	outBuf.Reset()
	code = run([]string{"--repos", "o/unauth", "--cache-dir", filepath.Join(tmp, "c2")}, makeEnv(srv.URL, ""), &outBuf, &errBuf)
	if code != 3 || !strings.Contains(outBuf.String(), `"unauthorized"`) {
		t.Fatalf("expected exit 3 unauthorized, got %d: %s", code, outBuf.String())
	}

	outBuf.Reset()
	code = run([]string{"--repos", "o/missing", "--cache-dir", filepath.Join(tmp, "c3")}, makeEnv(srv.URL, ""), &outBuf, &errBuf)
	if code != 3 || !strings.Contains(outBuf.String(), `"all_failed"`) {
		t.Fatalf("expected exit 3 all_failed, got %d: %s", code, outBuf.String())
	}
}
