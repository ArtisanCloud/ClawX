package contract

import (
	"testing"

	"clawx/internal/application/autonomy"
)

func TestRoutingIronLawContract(t *testing.T) {
	if err := autonomy.EnforceRoutingIronLaw("/agent list", "control"); err != nil {
		t.Fatalf("slash->control should be allowed: %v", err)
	}
	if err := autonomy.EnforceRoutingIronLaw("我想看一下当前进度", "execute"); err != nil {
		t.Fatalf("non-slash->execute should be allowed: %v", err)
	}
	if err := autonomy.EnforceRoutingIronLaw("/agent list", "execute"); err == nil {
		t.Fatalf("slash->execute must be rejected")
	}
	if err := autonomy.EnforceRoutingIronLaw("切换到 bid-all", "control"); err == nil {
		t.Fatalf("non-slash->control must be rejected")
	}
}
