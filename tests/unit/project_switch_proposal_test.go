package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"clawx/internal/application/intent"
	skilldomain "clawx/internal/domain/skill"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectSwitchProposalRules(t *testing.T) {
	pipeline := intent.NewPipeline(projectProposalTestRegistry{}, nil, intent.DefaultLLMFallback{}, 0.72)

	cases := []struct {
		name             string
		text             string
		wantProjectID    string
		wantReasonPrefix string
	}{
		{
			name:             "english project hint",
			text:             "please continue project:nba implementation",
			wantProjectID:    "nba",
			wantReasonPrefix: "text_project_hint",
		},
		{
			name:             "chinese switch hint",
			text:             "切换到 bid 项目继续开发",
			wantProjectID:    "bid",
			wantReasonPrefix: "text_switch_intent",
		},
		{
			name:             "no project hint",
			text:             "do something unrelated",
			wantProjectID:    "",
			wantReasonPrefix: "",
		},
		{
			name:             "control command should not emit proposal",
			text:             "/project use bid",
			wantProjectID:    "",
			wantReasonPrefix: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := pipeline.Decide(context.Background(), chatiface.Message{
				ConversationID: "discord:-:-:proposal-user",
				UserID:         "proposal-user",
				Text:           tc.text,
				Channel:        "discord",
				ContextFlags: chatiface.ContextFlags{
					IsAllowed: true,
				},
			})
			if err != nil {
				t.Fatalf("pipeline decide: %v", err)
			}

			if result.ProposalProjectID != tc.wantProjectID {
				t.Fatalf("unexpected proposal project id: got=%q want=%q", result.ProposalProjectID, tc.wantProjectID)
			}
			if tc.wantReasonPrefix == "" {
				if result.ProposalReason != "" || result.ProposalConfidence != 0 {
					t.Fatalf("unexpected proposal metadata: reason=%q confidence=%.2f", result.ProposalReason, result.ProposalConfidence)
				}
				return
			}
			if result.ProposalReason != tc.wantReasonPrefix {
				t.Fatalf("unexpected proposal reason: got=%q want=%q", result.ProposalReason, tc.wantReasonPrefix)
			}
			if result.ProposalConfidence <= 0 {
				t.Fatalf("proposal confidence should be positive: %.2f", result.ProposalConfidence)
			}
		})
	}
}

type projectProposalTestRegistry struct{}

func (projectProposalTestRegistry) Snapshot() skilldomain.RegistrySnapshot {
	return skilldomain.RegistrySnapshot{
		Version:     1,
		Entries:     nil,
		GeneratedAt: time.Now().UTC(),
	}
}

func (projectProposalTestRegistry) FindEntry(_ string) (skilldomain.CatalogEntry, bool) {
	return skilldomain.CatalogEntry{}, false
}

func (projectProposalTestRegistry) FindActiveDefinition(_ string) (skilldomain.Definition, error) {
	return skilldomain.Definition{}, fmt.Errorf("not found")
}
