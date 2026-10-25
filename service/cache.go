package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

func writeTrace(w io.Writer, entry TraceEntry) {
	if w == nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = w.Write(append(data, '\n'))
}

func cacheFilePath(cacheDir string, reqURL string) string {
	sum := sha256.Sum256([]byte(reqURL))
	return filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json")
}

func loadCache(cacheDir string, reqURL string) (CacheEntry, bool) {
	var entry CacheEntry
	path := cacheFilePath(cacheDir, reqURL)
	data, err := os.ReadFile(path)
	if err != nil {
		return entry, false
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return entry, false
	}
	return entry, true
}

func saveCache(cacheDir string, reqURL string, entry CacheEntry) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := cacheFilePath(cacheDir, reqURL)
	_ = os.WriteFile(path, data, 0644)
}
