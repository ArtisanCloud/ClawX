package autonomy

import "testing"

func TestEnforceRoutingIronLaw(t *testing.T) {
	if err := EnforceRoutingIronLaw("/agent use bid-all", "control"); err != nil {
		t.Fatalf("slash control should pass: %v", err)
	}
	if err := EnforceRoutingIronLaw("帮我切换到 bid-all", "execute"); err != nil {
		t.Fatalf("non-slash execute should pass: %v", err)
	}
	if err := EnforceRoutingIronLaw("/agent use bid-all", "execute"); err == nil {
		t.Fatalf("slash execute should violate iron law")
	}
	if err := EnforceRoutingIronLaw("现在有多少个智能体", "control"); err == nil {
		t.Fatalf("non-slash control should violate iron law")
	}
}
