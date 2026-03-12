package persistence

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"clawx/internal/domain/session"
)

type sessionFileSnapshot struct {
	Version int              `json:"version"`
	Records []session.Record `json:"records"`
}

type windowBindingFileSnapshot struct {
	Version  int                     `json:"version"`
	Bindings []session.WindowBinding `json:"bindings"`
}

type sessionTranscriptEvent struct {
	Timestamp time.Time      `json:"timestamp"`
	Event     string         `json:"event"`
	Session   session.Record `json:"session"`
}

type SessionFileRepository struct {
	mu             sync.RWMutex
	byID           map[string]session.Record
	byConversation map[string][]string
	byWindow       map[string]session.WindowBinding
	bindingByConv  map[string][]string
	lockCounter    uint64
	stateDir       string
	knownAgents    map[string]struct{}
}

func NewSessionFileRepository(stateDir string) (*SessionFileRepository, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return nil, fmt.Errorf("state directory is required")
	}

	repo := &SessionFileRepository{
		byID:           make(map[string]session.Record),
		byConversation: make(map[string][]string),
		byWindow:       make(map[string]session.WindowBinding),
		bindingByConv:  make(map[string][]string),
		stateDir:       stateDir,
		knownAgents:    make(map[string]struct{}),
	}
	if err := repo.loadFromDisk(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *SessionFileRepository) Create(_ context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[record.ID]; exists {
		return fmt.Errorf("create session %s: %w", record.ID, session.ErrInvalidSession)
	}

	record = cloneRecord(record)
	record.AgentID = normalizeAgentID(record.AgentID)
	r.byID[record.ID] = record
	r.rebuildConversationIndexLocked()

	if err := r.persistIndexLocked(); err != nil {
		return err
	}
	if err := r.appendTranscriptLocked("create", record); err != nil {
		return err
	}
	return nil
}

func (r *SessionFileRepository) Save(_ context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[record.ID]; !exists {
		return session.ErrSessionNotFound
	}
	record = cloneRecord(record)
	record.AgentID = normalizeAgentID(record.AgentID)
	r.byID[record.ID] = record
	r.rebuildConversationIndexLocked()

	if err := r.persistIndexLocked(); err != nil {
		return err
	}
	if err := r.appendTranscriptLocked("save", record); err != nil {
		return err
	}
	return nil
}

func (r *SessionFileRepository) GetByID(_ context.Context, sessionID string) (session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return session.Record{}, session.ErrSessionNotFound
	}
	return cloneRecord(record), nil
}

func (r *SessionFileRepository) GetLatestByConversation(_ context.Context, conversationID string) (session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byConversation[conversationID]
	if len(ids) == 0 {
		return session.Record{}, session.ErrSessionNotFound
	}

	var latest session.Record
	var found bool
	for _, id := range ids {
		record, exists := r.byID[id]
		if !exists {
			continue
		}
		if !found || record.LastUsedAt.After(latest.LastUsedAt) {
			latest = record
			found = true
		}
	}
	if !found {
		return session.Record{}, session.ErrSessionNotFound
	}
	return cloneRecord(latest), nil
}

func (r *SessionFileRepository) ListByConversation(_ context.Context, conversationID string) ([]session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byConversation[conversationID]
	if len(ids) == 0 {
		return nil, nil
	}

	records := make([]session.Record, 0, len(ids))
	for _, id := range ids {
		record, exists := r.byID[id]
		if !exists {
			continue
		}
		records = append(records, cloneRecord(record))
	}
	return records, nil
}

func (r *SessionFileRepository) GetWindowBinding(_ context.Context, windowID string) (session.WindowBinding, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return session.WindowBinding{}, session.ErrWindowBindingNotFound
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	binding, exists := r.byWindow[windowID]
	if !exists {
		return session.WindowBinding{}, session.ErrWindowBindingNotFound
	}
	return cloneWindowBinding(binding), nil
}

func (r *SessionFileRepository) SetWindowBinding(_ context.Context, binding session.WindowBinding) error {
	binding.WindowID = strings.TrimSpace(binding.WindowID)
	binding.CurrentSessionID = strings.TrimSpace(binding.CurrentSessionID)
	binding.ConversationID = strings.TrimSpace(binding.ConversationID)
	if err := binding.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = now
	}
	if binding.LastUsedAt.IsZero() {
		binding.LastUsedAt = binding.UpdatedAt
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.byWindow[binding.WindowID] = cloneWindowBinding(binding)
	r.rebuildWindowBindingIndexLocked()
	if err := r.persistWindowBindingsLocked(); err != nil {
		return err
	}
	return nil
}

func (r *SessionFileRepository) ListWindowBindingsByConversation(_ context.Context, conversationID string) ([]session.WindowBinding, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	windowIDs := r.bindingByConv[conversationID]
	if len(windowIDs) == 0 {
		return nil, nil
	}

	bindings := make([]session.WindowBinding, 0, len(windowIDs))
	for _, windowID := range windowIDs {
		binding, exists := r.byWindow[windowID]
		if !exists {
			continue
		}
		bindings = append(bindings, cloneWindowBinding(binding))
	}
	return bindings, nil
}

func (r *SessionFileRepository) Acquire(_ context.Context, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return "", session.ErrSessionNotFound
	}
	if record.LockToken != "" {
		return "", ErrLockHeldByAnotherProcess
	}

	r.lockCounter++
	lockToken := fmt.Sprintf("%s-%d-%d", sessionID, time.Now().UTC().UnixNano(), r.lockCounter)
	record.LockToken = lockToken
	r.byID[sessionID] = record
	return lockToken, nil
}

func (r *SessionFileRepository) Release(_ context.Context, sessionID, lockToken string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return session.ErrSessionNotFound
	}
	if record.LockToken == "" {
		return nil
	}
	if record.LockToken != lockToken {
		return session.ErrInvalidLock
	}
	record.LockToken = ""
	r.byID[sessionID] = record
	return nil
}

func (r *SessionFileRepository) loadFromDisk() error {
	agentsDir := filepath.Join(r.stateDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read agents dir: %w", err)
		}
		entries = nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := normalizeAgentID(entry.Name())
		storePath := filepath.Join(agentsDir, agentID, "sessions", "sessions.json")
		snapshot, readErr := readSessionFileSnapshot(storePath)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				records, rebuildErr := r.loadRecordsFromTranscripts(agentID)
				if rebuildErr != nil {
					return rebuildErr
				}
				snapshot = sessionFileSnapshot{Version: 1, Records: records}
			} else {
				records, rebuildErr := r.loadRecordsFromTranscripts(agentID)
				if rebuildErr != nil {
					return readErr
				}
				snapshot = sessionFileSnapshot{Version: 1, Records: records}
			}
		}

		r.knownAgents[agentID] = struct{}{}
		for _, raw := range snapshot.Records {
			record := cloneRecord(raw)
			record.AgentID = normalizeAgentID(firstNonEmpty(record.AgentID, agentID))
			record.LockToken = ""
			if record.Status == session.StatusRunning {
				record.Status = session.StatusIdle
			}
			if err := record.Validate(); err != nil {
				continue
			}

			existing, exists := r.byID[record.ID]
			if exists && existing.LastUsedAt.After(record.LastUsedAt) {
				continue
			}
			r.byID[record.ID] = record
		}
	}

	r.rebuildConversationIndexLocked()
	if err := r.loadWindowBindingsFromDisk(); err != nil {
		return err
	}
	r.rebuildWindowBindingIndexLocked()
	return nil
}

func readSessionFileSnapshot(path string) (sessionFileSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return sessionFileSnapshot{}, err
	}
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 {
		return sessionFileSnapshot{}, nil
	}

	var snapshot sessionFileSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return sessionFileSnapshot{}, fmt.Errorf("parse session store %q: %w", path, err)
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	return snapshot, nil
}

func readWindowBindingFileSnapshot(path string) (windowBindingFileSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return windowBindingFileSnapshot{}, err
	}
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 {
		return windowBindingFileSnapshot{}, nil
	}

	var snapshot windowBindingFileSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return windowBindingFileSnapshot{}, fmt.Errorf("parse window binding store %q: %w", path, err)
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	return snapshot, nil
}

func (r *SessionFileRepository) loadRecordsFromTranscripts(agentID string) ([]session.Record, error) {
	sessionsDir := r.agentSessionsDir(agentID)
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session transcript dir %q: %w", sessionsDir, err)
	}

	byID := make(map[string]session.Record)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		filePath := filepath.Join(sessionsDir, name)
		records, err := readTranscriptRecords(filePath, agentID)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			existing, exists := byID[record.ID]
			if exists && existing.LastUsedAt.After(record.LastUsedAt) {
				continue
			}
			byID[record.ID] = record
		}
	}

	result := make([]session.Record, 0, len(byID))
	for _, record := range byID {
		result = append(result, record)
	}
	return result, nil
}

func readTranscriptRecords(path string, defaultAgent string) ([]session.Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	recordsByID := make(map[string]session.Record)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event sessionTranscriptEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		record := event.Session
		record.AgentID = normalizeAgentID(firstNonEmpty(record.AgentID, defaultAgent))
		record.LockToken = ""
		if record.Status == session.StatusRunning {
			record.Status = session.StatusIdle
		}
		if err := record.Validate(); err != nil {
			continue
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = record.LastUsedAt
		}
		if record.LastUsedAt.IsZero() {
			record.LastUsedAt = event.Timestamp
		}

		existing, exists := recordsByID[record.ID]
		if exists && existing.LastUsedAt.After(record.LastUsedAt) {
			continue
		}
		recordsByID[record.ID] = record
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan transcript %q: %w", path, err)
	}

	records := make([]session.Record, 0, len(recordsByID))
	for _, record := range recordsByID {
		records = append(records, record)
	}
	return records, nil
}

func (r *SessionFileRepository) persistIndexLocked() error {
	grouped := make(map[string][]session.Record)
	for _, record := range r.byID {
		agentID := normalizeAgentID(record.AgentID)
		record.AgentID = agentID
		grouped[agentID] = append(grouped[agentID], sanitizePersistentRecord(record))
		r.knownAgents[agentID] = struct{}{}
	}

	agents := make([]string, 0, len(r.knownAgents))
	for agentID := range r.knownAgents {
		agents = append(agents, agentID)
	}
	sort.Strings(agents)

	for _, agentID := range agents {
		records := grouped[agentID]
		sort.Slice(records, func(i, j int) bool {
			return records[i].LastUsedAt.After(records[j].LastUsedAt)
		})

		snapshot := sessionFileSnapshot{
			Version: 1,
			Records: records,
		}
		storePath := filepath.Join(r.agentSessionsDir(agentID), "sessions.json")
		if err := writeJSONAtomic(storePath, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func (r *SessionFileRepository) persistWindowBindingsLocked() error {
	bindings := make([]session.WindowBinding, 0, len(r.byWindow))
	for _, binding := range r.byWindow {
		bindings = append(bindings, cloneWindowBinding(binding))
	}
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].WindowID < bindings[j].WindowID
	})

	snapshot := windowBindingFileSnapshot{
		Version:  1,
		Bindings: bindings,
	}
	return writeJSONAtomic(r.windowBindingStorePath(), snapshot)
}

func (r *SessionFileRepository) appendTranscriptLocked(event string, record session.Record) error {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	event = strings.TrimSpace(event)
	if event == "" {
		event = "save"
	}

	payload := sessionTranscriptEvent{
		Timestamp: time.Now().UTC(),
		Event:     event,
		Session:   sanitizePersistentRecord(record),
	}
	line, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	path := filepath.Join(r.agentSessionsDir(record.AgentID), record.ID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session transcript dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open transcript file %q: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("append transcript %q: %w", path, err)
	}
	return nil
}

func (r *SessionFileRepository) rebuildConversationIndexLocked() {
	index := make(map[string][]string)
	for _, record := range r.byID {
		index[record.ConversationID] = append(index[record.ConversationID], record.ID)
	}
	r.byConversation = index
}

func (r *SessionFileRepository) rebuildWindowBindingIndexLocked() {
	index := make(map[string][]string)
	for windowID, binding := range r.byWindow {
		conversationID := strings.TrimSpace(binding.ConversationID)
		if conversationID == "" {
			continue
		}
		index[conversationID] = append(index[conversationID], windowID)
	}
	r.bindingByConv = index
}

func (r *SessionFileRepository) agentSessionsDir(agentID string) string {
	return filepath.Join(r.stateDir, "agents", normalizeAgentID(agentID), "sessions")
}

func (r *SessionFileRepository) windowBindingStorePath() string {
	return filepath.Join(r.stateDir, "window_bindings.json")
}

func (r *SessionFileRepository) loadWindowBindingsFromDisk() error {
	snapshot, err := readWindowBindingFileSnapshot(r.windowBindingStorePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, raw := range snapshot.Bindings {
		binding := cloneWindowBinding(raw)
		binding.WindowID = strings.TrimSpace(binding.WindowID)
		binding.CurrentSessionID = strings.TrimSpace(binding.CurrentSessionID)
		binding.ConversationID = strings.TrimSpace(binding.ConversationID)
		if err := binding.Validate(); err != nil {
			continue
		}
		r.byWindow[binding.WindowID] = binding
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %q: %w", path, err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}

func sanitizePersistentRecord(record session.Record) session.Record {
	clean := cloneRecord(record)
	clean.LockToken = ""
	if clean.Status == session.StatusRunning {
		clean.Status = session.StatusIdle
	}
	return clean
}

func normalizeAgentID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "main"
	}

	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	segment := strings.Trim(strings.ReplaceAll(b.String(), "--", "-"), "-")
	if segment == "" {
		return "main"
	}
	return segment
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func bytesTrimSpace(data []byte) []byte {
	start := 0
	for start < len(data) && isWhitespace(data[start]) {
		start++
	}
	end := len(data) - 1
	for end >= start && isWhitespace(data[end]) {
		end--
	}
	if start > end {
		return nil
	}
	return data[start : end+1]
}

func isWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}
