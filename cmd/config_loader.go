package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadConfigPayload(path string) (string, error) {
	if !isYAMLPath(path) {
		return "", fmt.Errorf("config file must be .yaml or .yml: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", fmt.Errorf("config file %s is empty", path)
	}
	var payload interface{}
	if err := yaml.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", fmt.Errorf("parse config file: %w", err)
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	result := strings.TrimSpace(string(normalized))
	if result == "" || result == "null" {
		return "", fmt.Errorf("config file %s produced empty payload", path)
	}
	return result, nil
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	return ext == ".yaml" || ext == ".yml"
}
