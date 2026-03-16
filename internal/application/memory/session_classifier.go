package memory

import (
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

type SessionClassResult struct {
	ChatMode         memorydomain.ChatMode
	AllowMainPrivate bool
	OwnerMatched     bool
	Degraded         bool
	Reason           string
}

type SessionClassInput struct {
	RouteKey        string
	UserID          string
	IsDirectMessage bool
	OwnerAllowlist  []string
}

func ClassifySession(input SessionClassInput) SessionClassResult {
	owners := normalizeOwners(input.OwnerAllowlist)
	routeDirect, routeUser := parseDirectRoutePeer(input.RouteKey)
	userID := strings.ToLower(strings.TrimSpace(input.UserID))
	direct := input.IsDirectMessage || routeDirect

	result := SessionClassResult{
		ChatMode:         memorydomain.ChatModeShared,
		AllowMainPrivate: false,
		OwnerMatched:     false,
		Degraded:         false,
		Reason:           "shared_by_default",
	}
	if !direct {
		result.Reason = "non_direct_session"
		return result
	}
	if len(owners) == 0 {
		result.Reason = "owner_allowlist_empty"
		return result
	}

	if userID != "" && routeUser != "" && userID != routeUser {
		result.Degraded = true
		result.Reason = "route_user_conflict"
		return result
	}
	identity := userID
	if identity == "" {
		identity = routeUser
	}
	if identity == "" {
		result.Degraded = true
		result.Reason = "missing_direct_identity"
		return result
	}
	if _, ok := owners[identity]; !ok {
		result.Reason = "owner_not_allowed"
		return result
	}

	result.ChatMode = memorydomain.ChatModeMain
	result.AllowMainPrivate = true
	result.OwnerMatched = true
	result.Reason = "direct_owner_allowed"
	return result
}

func parseDirectRoutePeer(routeKey string) (bool, string) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(routeKey)), ":")
	if len(parts) < 4 {
		return false, ""
	}
	for idx := 0; idx < len(parts)-1; idx++ {
		if parts[idx] != "direct" {
			continue
		}
		peer := strings.TrimSpace(parts[idx+1])
		if peer == "" || peer == "-" {
			return true, ""
		}
		return true, peer
	}
	return false, ""
}

func normalizeOwners(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
