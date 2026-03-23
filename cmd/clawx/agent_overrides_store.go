package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type conversationAgentOverridesSnapshot struct {
	Version int               `json:"version"`
	Items   map[string]string `json:"items"`
}

func (o *conversationAgentOverrides) EnablePersistence(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.persistencePath = filepath.Clean(path)
	return o.loadLocked()
}

func (o *conversationAgentOverrides) loadLocked() error {
	if strings.TrimSpace(o.persistencePath) == "" {
		return nil
	}
	body, err := os.ReadFile(o.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var snapshot conversationAgentOverridesSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return err
	}
	if len(snapshot.Items) == 0 {
		return nil
	}
	if o.byScope == nil {
		o.byScope = make(map[string]string, len(snapshot.Items))
	}
	for rawScope, rawAgent := range snapshot.Items {
		scope := strings.TrimSpace(rawScope)
		agent := strings.TrimSpace(rawAgent)
		if scope == "" || agent == "" {
			continue
		}
		o.byScope[scope] = agent
	}
	return nil
}

func (o *conversationAgentOverrides) persistLocked() error {
	if strings.TrimSpace(o.persistencePath) == "" {
		return nil
	}
	dir := filepath.Dir(o.persistencePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	items := make(map[string]string, len(o.byScope))
	for scope, agent := range o.byScope {
		scope = strings.TrimSpace(scope)
		agent = strings.TrimSpace(agent)
		if scope == "" || agent == "" {
			continue
		}
		items[scope] = agent
	}
	snapshot := conversationAgentOverridesSnapshot{
		Version: 1,
		Items:   items,
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	tmp, err := os.CreateTemp(dir, ".agent-overrides-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp overrides file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, o.persistencePath)
}
