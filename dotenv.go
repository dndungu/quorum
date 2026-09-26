package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv loads KEY=VALUE entries from path without replacing variables
// already present in the process environment.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !validEnvName(key) {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}

		value, err = parseDotEnvValue(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: set %s: %w", path, lineNo, key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func validEnvName(key string) bool {
	for i, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return key != ""
}

func parseDotEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value[0] == '\'' || value[0] == '"' {
		quote := value[0]
		end := -1
		escaped := false
		for i := 1; i < len(value); i++ {
			if quote == '"' && value[i] == '\\' && !escaped {
				escaped = true
				continue
			}
			if value[i] == quote && !escaped {
				end = i
				break
			}
			escaped = false
		}
		if end < 0 {
			return "", fmt.Errorf("unterminated quoted value")
		}
		suffix := strings.TrimSpace(value[end+1:])
		if suffix != "" && !strings.HasPrefix(suffix, "#") {
			return "", fmt.Errorf("unexpected text after quoted value")
		}
		quoted := value[:end+1]
		if quote == '\'' {
			return quoted[1 : len(quoted)-1], nil
		}
		unquoted, err := strconv.Unquote(quoted)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value: %w", err)
		}
		return unquoted, nil
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = value[:i]
	}
	if i := strings.Index(value, "\t#"); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value), nil
}
