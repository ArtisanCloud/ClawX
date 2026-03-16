package memory

import (
	memorydomain "clawx/internal/domain/memory"
)

func ApplyMinimalPrivilege(profile memorydomain.MemoryProfile) memorydomain.MemoryProfile {
	profile.AllowMainPrivate = false
	profile.ACLMode = memorydomain.ACLModeDegraded
	if len(profile.LoadOrder) == 0 {
		profile.LoadOrder = []memorydomain.Layer{memorydomain.LayerAgentPrivate, memorydomain.LayerProjectShare}
	}
	return profile
}
