package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clawx/internal/infrastructure/config"
)

type conversationProgressNotifier func(ctx context.Context, message string) error
type conversationProgressDispatcher func(ctx context.Context, targetRaw json.RawMessage, message string) error

type conversationProgressState struct {
	LastDigest string
	LastSentAt time.Time
}

type conversationProgressRoute struct {
	ConversationID string          `json:"conversation_id"`
	Channel        string          `json:"channel"`
	Instance       string          `json:"instance"`
	Target         json.RawMessage `json:"target"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type conversationProgressRouteSnapshot struct {
	Version int                         `json:"version"`
	Routes  []conversationProgressRoute `json:"routes"`
}

type conversationProgressDeliveryPlan struct {
	Notify   conversationProgressNotifier
	Source   string
	Channel  string
	Instance string
}

type conversationProgressRegistry struct {
	mu           sync.Mutex
	byConv       map[string]conversationProgressNotifier
	dispatchers  map[string]conversationProgressDispatcher
	routes       map[string]conversationProgressRoute
	state        map[string]conversationProgressState
	routesLoaded bool
}

var globalConversationProgressNotifiers = &conversationProgressRegistry{
	byConv:      make(map[string]conversationProgressNotifier),
	dispatchers: make(map[string]conversationProgressDispatcher),
	routes:      make(map[string]conversationProgressRoute),
	state:       make(map[string]conversationProgressState),
}

func registerConversationProgressNotifier(conversationID string, notifier conversationProgressNotifier) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || notifier == nil {
		return
	}
	globalConversationProgressNotifiers.mu.Lock()
	defer globalConversationProgressNotifiers.mu.Unlock()
	globalConversationProgressNotifiers.byConv[conversationID] = notifier
}

func registerConversationProgressDispatcher(channel string, instanceID string, dispatcher conversationProgressDispatcher) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	instanceID = normalizeConversationProgressDispatcherInstance(instanceID)
	if channel == "" || dispatcher == nil {
		return
	}

	globalConversationProgressNotifiers.mu.Lock()
	defer globalConversationProgressNotifiers.mu.Unlock()
	globalConversationProgressNotifiers.dispatchers[conversationProgressDispatcherKey(channel, instanceID)] = dispatcher
}

func bindConversationProgressRoute(conversationID string, channel string, instanceID string, target any) {
	conversationID = strings.TrimSpace(conversationID)
	channel = strings.ToLower(strings.TrimSpace(channel))
	instanceID = normalizeConversationProgressDispatcherInstance(instanceID)
	if conversationID == "" || channel == "" || target == nil {
		return
	}
	targetRaw, err := json.Marshal(target)
	if err != nil {
		log.Printf("conversation progress route marshal failed: conversation_id=%s channel=%s instance=%s err=%v", conversationID, channel, instanceID, err)
		return
	}
	route := conversationProgressRoute{
		ConversationID: conversationID,
		Channel:        channel,
		Instance:       instanceID,
		Target:         append(json.RawMessage(nil), bytes.TrimSpace(targetRaw)...),
		UpdatedAt:      time.Now().UTC(),
	}
	if len(route.Target) == 0 {
		return
	}

	globalConversationProgressNotifiers.mu.Lock()
	defer globalConversationProgressNotifiers.mu.Unlock()
	globalConversationProgressNotifiers.ensureRoutesLoadedLocked()
	prev, exists := globalConversationProgressNotifiers.routes[conversationID]
	if exists &&
		prev.Channel == route.Channel &&
		prev.Instance == route.Instance &&
		bytes.Equal(bytes.TrimSpace(prev.Target), route.Target) {
		return
	}
	globalConversationProgressNotifiers.routes[conversationID] = route
	if err := globalConversationProgressNotifiers.persistRoutesLocked(); err != nil {
		log.Printf("conversation progress route persist failed: conversation_id=%s channel=%s instance=%s err=%v", conversationID, channel, instanceID, err)
	}
}

func emitConversationProgressNotification(conversationID string, message string) {
	conversationID = strings.TrimSpace(conversationID)
	message = strings.TrimSpace(message)
	if conversationID == "" || message == "" {
		return
	}
	if isConversationProgressNotifyDisabled() {
		return
	}

	plan, shouldSend, digest := resolveConversationProgressNotification(conversationID, message)
	if !shouldSend || plan.Notify == nil {
		return
	}

	go func(convID string, payload string, payloadDigest string, delivery conversationProgressDeliveryPlan) {
		ctx, cancel := context.WithTimeout(context.Background(), resolveConversationProgressNotifyTimeout())
		defer cancel()
		if err := delivery.Notify(ctx, payload); err != nil {
			log.Printf(
				"conversation progress notify failed: conversation_id=%s digest=%s source=%s channel=%s instance=%s err=%v",
				convID,
				payloadDigest,
				delivery.Source,
				delivery.Channel,
				delivery.Instance,
				err,
			)
		}
	}(conversationID, message, digest, plan)
}

func resolveConversationProgressNotification(conversationID string, message string) (conversationProgressDeliveryPlan, bool, string) {
	digest := digestConversationProgressMessage(message)
	now := time.Now().UTC()
	minInterval := resolveConversationProgressNotifyMinInterval()

	globalConversationProgressNotifiers.mu.Lock()
	defer globalConversationProgressNotifiers.mu.Unlock()
	globalConversationProgressNotifiers.ensureRoutesLoadedLocked()

	plan := conversationProgressDeliveryPlan{}
	if notifier, ok := globalConversationProgressNotifiers.byConv[conversationID]; ok && notifier != nil {
		plan = conversationProgressDeliveryPlan{
			Notify: notifier,
			Source: "live_notifier",
		}
	} else if route, ok := globalConversationProgressNotifiers.routes[conversationID]; ok {
		if dispatcher, channel, instance := globalConversationProgressNotifiers.resolveDispatcherForRouteLocked(route); dispatcher != nil {
			targetCopy := append(json.RawMessage(nil), route.Target...)
			plan = conversationProgressDeliveryPlan{
				Notify: func(ctx context.Context, body string) error {
					return dispatcher(ctx, targetCopy, body)
				},
				Source:   "persisted_route",
				Channel:  channel,
				Instance: instance,
			}
		}
	}
	if plan.Notify == nil {
		return conversationProgressDeliveryPlan{}, false, digest
	}
	prev := globalConversationProgressNotifiers.state[conversationID]
	if strings.TrimSpace(prev.LastDigest) == digest {
		return plan, false, digest
	}
	if minInterval > 0 && !prev.LastSentAt.IsZero() && now.Sub(prev.LastSentAt) < minInterval {
		return plan, false, digest
	}
	globalConversationProgressNotifiers.state[conversationID] = conversationProgressState{
		LastDigest: digest,
		LastSentAt: now,
	}
	return plan, true, digest
}

func digestConversationProgressMessage(message string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(message)))
	return hex.EncodeToString(sum[:])
}

func resolveConversationProgressNotifyMinInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL"))
	if raw == "" {
		return 8 * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return 8 * time.Second
}

func resolveConversationProgressNotifyTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT"))
	if raw == "" {
		return 5 * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 5 * time.Second
}

func isConversationProgressNotifyDisabled() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("CLAWX_PROGRESS_NOTIFY_DISABLED")))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *conversationProgressRegistry) ensureRoutesLoadedLocked() {
	if s.routesLoaded {
		return
	}
	s.routesLoaded = true
	path := conversationProgressRoutePath()
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var snapshot conversationProgressRouteSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return
	}
	if s.routes == nil {
		s.routes = make(map[string]conversationProgressRoute, len(snapshot.Routes))
	}
	for _, raw := range snapshot.Routes {
		route, ok := normalizeConversationProgressRoute(raw)
		if !ok {
			continue
		}
		s.routes[route.ConversationID] = route
	}
}

func (s *conversationProgressRegistry) persistRoutesLocked() error {
	path := conversationProgressRoutePath()
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(s.routes))
	for conversationID := range s.routes {
		conversationID = strings.TrimSpace(conversationID)
		if conversationID == "" {
			continue
		}
		ids = append(ids, conversationID)
	}
	sort.Strings(ids)
	routes := make([]conversationProgressRoute, 0, len(ids))
	for _, conversationID := range ids {
		route, ok := normalizeConversationProgressRoute(s.routes[conversationID])
		if !ok {
			continue
		}
		routes = append(routes, route)
	}
	snapshot := conversationProgressRouteSnapshot{
		Version: 1,
		Routes:  routes,
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	tmp, err := os.CreateTemp(dir, ".conversation-progress-routes-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp route file: %w", err)
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
	return os.Rename(tmpPath, path)
}

func (s *conversationProgressRegistry) resolveDispatcherForRouteLocked(route conversationProgressRoute) (conversationProgressDispatcher, string, string) {
	route, ok := normalizeConversationProgressRoute(route)
	if !ok {
		return nil, "", ""
	}
	key := conversationProgressDispatcherKey(route.Channel, route.Instance)
	if dispatcher, exists := s.dispatchers[key]; exists && dispatcher != nil {
		return dispatcher, route.Channel, route.Instance
	}
	if route.Instance != "default" {
		defaultKey := conversationProgressDispatcherKey(route.Channel, "default")
		if dispatcher, exists := s.dispatchers[defaultKey]; exists && dispatcher != nil {
			return dispatcher, route.Channel, "default"
		}
	}
	return nil, "", ""
}

func normalizeConversationProgressRoute(route conversationProgressRoute) (conversationProgressRoute, bool) {
	route.ConversationID = strings.TrimSpace(route.ConversationID)
	route.Channel = strings.ToLower(strings.TrimSpace(route.Channel))
	route.Instance = normalizeConversationProgressDispatcherInstance(route.Instance)
	route.Target = append(json.RawMessage(nil), bytes.TrimSpace(route.Target)...)
	if route.ConversationID == "" || route.Channel == "" || len(route.Target) == 0 {
		return conversationProgressRoute{}, false
	}
	if route.UpdatedAt.IsZero() {
		route.UpdatedAt = time.Now().UTC()
	}
	return route, true
}

func conversationProgressDispatcherKey(channel string, instanceID string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	instanceID = normalizeConversationProgressDispatcherInstance(instanceID)
	if channel == "" {
		return ""
	}
	return channel + "::" + instanceID
}

func normalizeConversationProgressDispatcherInstance(instanceID string) string {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return "default"
	}
	return instanceID
}

func conversationProgressRoutePath() string {
	if explicit := strings.TrimSpace(os.Getenv("CLAWX_PROGRESS_NOTIFY_ROUTE_FILE")); explicit != "" {
		return filepath.Clean(explicit)
	}
	return filepath.Join(config.StateDir(), "state", "conversation_progress_routes.json")
}
