package skillorchestrator

import (
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type DigestRebuildResult struct {
	Digest   skilldomain.ContextDigest
	Rebuilt  bool
	Reason   string
	Expected string
	Current  string
}

func RebuildContextDigestIfStale(
	existing skilldomain.ContextDigest,
	conversationID string,
	records []AuditRecord,
	projector *ContextDigestProjector,
) DigestRebuildResult {
	if projector == nil {
		projector = NewContextDigestProjector(50)
	}
	projected := projector.Project(conversationID, records)
	normalized := existing.Normalize()
	if strings.TrimSpace(normalized.ConversationID) == "" {
		return DigestRebuildResult{
			Digest:   projected,
			Rebuilt:  true,
			Reason:   "empty_conversation_id",
			Expected: projected.Hash,
			Current:  normalized.Hash,
		}
	}
	if err := normalized.Validate(); err != nil {
		return DigestRebuildResult{
			Digest:   projected,
			Rebuilt:  true,
			Reason:   "validation_failed",
			Expected: projected.Hash,
			Current:  normalized.Hash,
		}
	}
	if normalized.Hash != projected.Hash {
		return DigestRebuildResult{
			Digest:   projected,
			Rebuilt:  true,
			Reason:   "digest_mismatch",
			Expected: projected.Hash,
			Current:  normalized.Hash,
		}
	}
	return DigestRebuildResult{
		Digest:   normalized,
		Rebuilt:  false,
		Reason:   "up_to_date",
		Expected: projected.Hash,
		Current:  normalized.Hash,
	}
}
