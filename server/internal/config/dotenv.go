package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE files. Process environment variables always
// win. Among files, later paths override earlier ones:
//
//	../.env, server/.env, .env
func LoadDotEnv() ([]string, error) {
	preset := map[string]struct{}{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			preset[kv[:i]] = struct{}{}
		}
	}
	candidates := []string{
		"../.env",
		"server/.env",
		".env",
	}
	var loaded []string
	for _, path := range candidates {
		applied, err := loadDotEnvFile(path, preset)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return loaded, fmt.Errorf("%s: %w", path, err)
		}
		if applied {
			loaded = append(loaded, path)
		}
	}
	return loaded, nil
}

func loadDotEnvFile(path string, preset map[string]struct{}) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	any := false
	for sc.Scan() {
		lineNo++
		key, val, ok, err := parseDotEnvLine(sc.Text())
		if err != nil {
			return any, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if !ok {
			continue
		}
		any = true
		if _, locked := preset[key]; locked {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return any, err
		}
	}
	return any, sc.Err()
}

func parseDotEnvLine(line string) (key, val string, ok bool, err error) {
	s := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if s == "" || strings.HasPrefix(s, "#") {
		return "", "", false, nil
	}
	s = strings.TrimPrefix(s, "export ")
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return "", "", false, fmt.Errorf("expected KEY=VALUE")
	}
	key = strings.TrimSpace(s[:eq])
	raw := strings.TrimSpace(s[eq+1:])
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false, fmt.Errorf("invalid key")
	}
	if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
		return key, strings.TrimSpace(raw[1 : len(raw)-1]), true, nil
	}
	if i := strings.Index(raw, " #"); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	return key, strings.TrimSpace(raw), true, nil
}
