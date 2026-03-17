package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

type attachmentContextStore struct {
	workspaceRoot string
	httpClient    *http.Client
}

type attachmentContextSnapshot struct {
	RouteKey    string                 `json:"route_key"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Attachments []chatiface.Attachment `json:"attachments"`
}

func newAttachmentContextStore(cfg config.Snapshot) (*attachmentContextStore, error) {
	root := strings.TrimSpace(cfg.Projects.WorkspaceRoot)
	if root == "" {
		root = config.WorkspaceRoot()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create attachment context workspace root %q: %w", root, err)
	}
	return &attachmentContextStore{workspaceRoot: filepath.Clean(root)}, nil
}

func (s *attachmentContextStore) Resolve(projectID, agentID, routeKey string, current []chatiface.Attachment) ([]chatiface.Attachment, error) {
	projectID = normalizeExecutionDirSegment(projectID)
	agentID = normalizeExecutionDirSegment(agentID)
	routeKey = strings.TrimSpace(routeKey)
	if projectID == "" || agentID == "" || routeKey == "" {
		return append([]chatiface.Attachment(nil), current...), nil
	}
	current = normalizeAttachments(current)
	if len(current) > 0 {
		current = s.materialize(projectID, agentID, routeKey, current)
		if err := s.save(projectID, agentID, routeKey, current); err != nil {
			return current, err
		}
		return current, nil
	}
	cached, err := s.load(projectID, agentID, routeKey)
	if err != nil {
		return nil, err
	}
	return s.materialize(projectID, agentID, routeKey, cached), nil
}

func (s *attachmentContextStore) save(projectID, agentID, routeKey string, attachments []chatiface.Attachment) error {
	path := s.snapshotPath(projectID, agentID, routeKey)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	snapshot := attachmentContextSnapshot{
		RouteKey:    routeKey,
		UpdatedAt:   time.Now().UTC(),
		Attachments: append([]chatiface.Attachment(nil), attachments...),
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".attachments-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmpFile.Write(payload); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (s *attachmentContextStore) load(projectID, agentID, routeKey string) ([]chatiface.Attachment, error) {
	path := s.snapshotPath(projectID, agentID, routeKey)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snapshot attachmentContextSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, err
	}
	return normalizeAttachments(snapshot.Attachments), nil
}

func (s *attachmentContextStore) snapshotPath(projectID, agentID, routeKey string) string {
	return filepath.Join(
		s.workspaceRoot,
		projectID,
		".agents",
		agentID,
		"context",
		"attachments",
		hashRouteKey(routeKey)+".json",
	)
}

func (s *attachmentContextStore) localAttachmentPath(projectID, agentID, routeKey string, attachment chatiface.Attachment) string {
	name := sanitizeAttachmentName(attachment.Name)
	if name == "" {
		name = "attachment.bin"
	}
	routeHash := hashRouteKey(routeKey)
	fileHash := hashRouteKey(strings.ToLower(strings.TrimSpace(attachment.URL)) + "|" + strings.ToLower(name))
	return filepath.Join(
		s.workspaceRoot,
		projectID,
		".agents",
		agentID,
		"context",
		"attachments",
		"files",
		routeHash,
		fileHash+"-"+name,
	)
}

func (s *attachmentContextStore) materialize(projectID, agentID, routeKey string, attachments []chatiface.Attachment) []chatiface.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]chatiface.Attachment, 0, len(attachments))
	for _, item := range attachments {
		normalized := item
		localPath := strings.TrimSpace(normalized.LocalPath)
		if localPath != "" {
			if stat, err := os.Stat(localPath); err == nil && !stat.IsDir() {
				result = append(result, normalized)
				continue
			}
			normalized.LocalPath = ""
		}
		if strings.TrimSpace(normalized.URL) == "" {
			result = append(result, normalized)
			continue
		}
		targetPath := s.localAttachmentPath(projectID, agentID, routeKey, normalized)
		if stat, err := os.Stat(targetPath); err == nil && !stat.IsDir() {
			normalized.LocalPath = targetPath
			result = append(result, normalized)
			continue
		}
		if err := s.downloadAttachment(normalized.URL, targetPath); err != nil {
			log.Printf("attachment download skipped: project=%s agent=%s route=%s url=%s err=%v", projectID, agentID, routeKey, normalized.URL, err)
			result = append(result, normalized)
			continue
		}
		normalized.LocalPath = targetPath
		result = append(result, normalized)
	}
	return result
}

func (s *attachmentContextStore) downloadAttachment(rawURL, targetPath string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported attachment url scheme: %s", scheme)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("attachment download status: %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".attachment-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	const maxSize = int64(200 * 1024 * 1024)
	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return err
	}
	if written > maxSize {
		return fmt.Errorf("attachment too large: %d", written)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func hashRouteKey(routeKey string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(routeKey)))
	return hex.EncodeToString(sum[:])
}

func normalizeAttachments(items []chatiface.Attachment) []chatiface.Attachment {
	if len(items) == 0 {
		return nil
	}
	result := make([]chatiface.Attachment, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		url := strings.TrimSpace(item.URL)
		ct := strings.TrimSpace(item.ContentType)
		size := item.SizeBytes
		if name == "" && url == "" {
			continue
		}
		key := strings.ToLower(name) + "|" + strings.ToLower(url)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, chatiface.Attachment{
			Name:        name,
			URL:         url,
			LocalPath:   strings.TrimSpace(item.LocalPath),
			ContentType: ct,
			SizeBytes:   size,
		})
	}
	return result
}

func sanitizeAttachmentName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "._")
}

func mergeDecisionAttachments(runtime agentRuntime, decision service.Decision) service.Decision {
	if runtime.attachmentStore == nil {
		return decision
	}
	attachments, err := runtime.attachmentStore.Resolve(
		decision.ProjectID,
		runtime.agentID,
		decision.RouteKey,
		decision.Message.Attachments,
	)
	if err != nil {
		log.Printf("attachment context resolve failed: agent=%s project=%s route=%s err=%v", runtime.agentID, decision.ProjectID, decision.RouteKey, err)
		return decision
	}
	if len(attachments) == 0 {
		return decision
	}
	message := decision.Message
	message.Attachments = attachments
	decision.Message = message
	return decision
}
