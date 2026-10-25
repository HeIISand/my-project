package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func run(args []string, getenv func(string) string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := parseArgs(args, getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ghreport: %v\n", err)
		return 2
	}

	if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
		_, _ = fmt.Fprintf(stderr, "ghreport: %v\n", err)
		return 2
	}

	var traceWriter io.Writer
	if cfg.Trace != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Trace), 0755); err == nil {
			tf, err := os.Create(cfg.Trace)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "ghreport: %v\n", err)
				return 2
			}
			defer tf.Close()
			traceWriter = tf
		}
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			for k, v := range via[0].Header {
				if _, ok := req.Header[k]; !ok {
					req.Header[k] = v
				}
			}
			return nil
		},
	}

	var repoReports []RepoReport
	successCount := 0
	failCount := 0

	for _, repo := range cfg.Repos {
		rep, unauthorized := processRepo(client, repo, cfg, traceWriter)
		if unauthorized {
			_, _ = fmt.Fprintln(stdout, `{"error": "unauthorized"}`)
			return 3
		}
		if rep.Error != nil {
			failCount++
		} else {
			successCount++
		}
		repoReports = append(repoReports, rep)
	}

	if successCount == 0 && failCount > 0 {
		_, _ = fmt.Fprintln(stdout, `{"error": "all_failed"}`)
		return 3
	}

	finalReport := Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Repos:       repoReports,
	}

	outBytes, err := json.MarshalIndent(finalReport, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintln(stdout, `{"error": "all_failed"}`)
		return 3
	}
	outBytes = append(outBytes, '\n')

	if cfg.Out == "-" {
		_, _ = stdout.Write(outBytes)
	} else {
		if err := os.WriteFile(cfg.Out, outBytes, 0644); err != nil {
			_, _ = fmt.Fprintf(stderr, "ghreport: %v\n", err)
			return 2
		}
	}

	if failCount > 0 {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
