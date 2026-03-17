package main

import (
	"strings"
	"testing"
)

func TestNormalizeServiceName(t *testing.T) {
	if got := normalizeServiceName(" ClawX-Prod "); got != "clawx-prod" {
		t.Fatalf("unexpected normalized name: %q", got)
	}
	if got := normalizeServiceName("$$$"); got != "clawx" {
		t.Fatalf("fallback name mismatch: %q", got)
	}
}

func TestBuildSystemdUserUnitContainsServeExec(t *testing.T) {
	unit := buildSystemdUserUnit(
		"clawx",
		"/usr/local/bin/clawx",
		"/home/ubuntu/.clawx/config.json",
		"/home/ubuntu",
	)
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/clawx run") {
		t.Fatalf("missing ExecStart run: %s", unit)
	}
	if !strings.Contains(unit, "Environment=CLAWX_CONFIG=/home/ubuntu/.clawx/config.json") {
		t.Fatalf("missing CLAWX_CONFIG env")
	}
}

func TestParseInstallServiceArgs(t *testing.T) {
	opts, err := parseInstallServiceArgs([]string{"--name", "clawx-bot", "--start", "--binary", "/usr/local/bin/clawx", "--force"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Name != "clawx-bot" || !opts.Start || !opts.Force {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if opts.Binary != "/usr/local/bin/clawx" {
		t.Fatalf("unexpected binary: %q", opts.Binary)
	}
}
