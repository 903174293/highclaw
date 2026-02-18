package agent

import (
	"testing"

	"github.com/highclaw/highclaw/internal/config"
)

func testPolicy(level string) *SecurityPolicy {
	cfg := config.Default()
	cfg.Autonomy.Level = level
	return NewSecurityPolicy(cfg)
}

func TestPolicyAllowsLowRisk(t *testing.T) {
	p := testPolicy("supervised")
	err := p.ValidateBashInput(`{"command":"ls -la"}`)
	if err != nil {
		t.Fatalf("expected low-risk command to be allowed, got error: %v", err)
	}
}

func TestPolicyBlocksHighRiskNetworkByDefault(t *testing.T) {
	p := testPolicy("supervised")
	err := p.ValidateBashInput(`{"command":"curl -s wttr.in/Singapore?format=v2"}`)
	if err == nil {
		t.Fatal("expected curl to be blocked by default high-risk policy")
	}
}

func TestPolicyRequiresApprovalForMediumRisk(t *testing.T) {
	p := testPolicy("supervised")
	err := p.ValidateBashInput(`{"command":"git commit -m test"}`)
	if err == nil {
		t.Fatal("expected medium-risk command to require approval")
	}

	err = p.ValidateBashInput(`{"command":"git commit -m test","approved":true}`)
	if err != nil {
		t.Fatalf("expected approved medium-risk command to pass, got error: %v", err)
	}
}
