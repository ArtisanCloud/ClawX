package unit

import (
	"testing"
	"time"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
)

func TestSkillContextDigestProjectionAndRebuild(t *testing.T) {
	projector := skillorchestrator.NewContextDigestProjector(50)
	now := time.Now().UTC()
	records := []skillorchestrator.AuditRecord{
		{
			TraceID:        "cfm-1",
			EventType:      skilldomain.AuditEventExec,
			ConversationID: "conv-1",
			Actor:          "u1",
			SkillID:        "bid.collect",
			Source:         "command",
			Intent:         "run_skill",
			Arguments:      map[string]any{"risk": "high"},
			Result:         "confirm_required",
			OccurredAt:     now.Add(-2 * time.Second),
		},
		{
			TraceID:        "cfm-1",
			EventType:      skilldomain.AuditEventExec,
			ConversationID: "conv-1",
			Actor:          "u1",
			SkillID:        "bid.collect",
			Source:         "command",
			Intent:         "run_skill",
			Arguments:      map[string]any{"risk": "high"},
			Result:         "success",
			OccurredAt:     now.Add(-1 * time.Second),
		},
	}

	digest := projector.Project("conv-1", records)
	if err := digest.Validate(); err != nil {
		t.Fatalf("digest should be valid: %v", err)
	}
	if len(digest.Entries) != 2 {
		t.Fatalf("expected 2 digest entries, got %d", len(digest.Entries))
	}

	tampered := digest
	tampered.Hash = "tampered"
	rebuilt := skillorchestrator.RebuildContextDigestIfStale(tampered, "conv-1", records, projector)
	if !rebuilt.Rebuilt || rebuilt.Reason != "validation_failed" {
		t.Fatalf("expected rebuild for tampered digest, got rebuilt=%v reason=%s", rebuilt.Rebuilt, rebuilt.Reason)
	}
	if rebuilt.Digest.Hash == "tampered" || rebuilt.Digest.Hash == "" {
		t.Fatalf("rebuilt digest hash is invalid")
	}

	latest := skillorchestrator.RebuildContextDigestIfStale(digest, "conv-1", records, projector)
	if latest.Rebuilt {
		t.Fatalf("expected digest already up-to-date")
	}
}
