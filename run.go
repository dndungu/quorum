package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func newRunDir(base string) (string, error) {
	dir := filepath.Join(base, time.Now().Format("2006-01-02T15-04-05"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "reviews"), 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func writeFileString(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

var _ = fmt.Sprintf
