package memory

import (
	memorydomain "clawx/internal/domain/memory"
)

type LoaderACLInput struct {
	Profile         memorydomain.MemoryProfile
	RouteKey        string
	UserID          string
	IsDirectMessage bool
	OwnerAllowlist  []string
}

type LoaderACLResult struct {
	Profile        memorydomain.MemoryProfile
	Classification SessionClassResult
}

func ApplyLoaderACL(input LoaderACLInput) LoaderACLResult {
	profile := input.Profile
	classification := ClassifySession(SessionClassInput{
		RouteKey:        input.RouteKey,
		UserID:          input.UserID,
		IsDirectMessage: input.IsDirectMessage,
		OwnerAllowlist:  input.OwnerAllowlist,
	})
	profile.ScopeKey.ChatMode = classification.ChatMode
	if classification.Degraded {
		profile = ApplyMinimalPrivilege(profile)
	} else {
		profile.AllowMainPrivate = classification.AllowMainPrivate
		profile.ACLMode = memorydomain.ACLModeStrict
	}
	if profile.ACLMode == "" {
		profile.ACLMode = memorydomain.ACLModeStrict
	}
	return LoaderACLResult{
		Profile:        profile,
		Classification: classification,
	}
}
