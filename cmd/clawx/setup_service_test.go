package main

import "testing"

func TestParseSetupServiceArgs(t *testing.T) {
	opts, err := parseSetupServiceArgs([]string{
		"--target", "discord",
		"--binary", "/tmp/clawx-bin",
		"--name", "clawx-discord",
		"--start=false",
		"--force=false",
		"--skip-build",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Target != "discord" {
		t.Fatalf("unexpected target: %q", opts.Target)
	}
	if opts.Binary != "/tmp/clawx-bin" {
		t.Fatalf("unexpected binary: %q", opts.Binary)
	}
	if opts.Name != "clawx-discord" {
		t.Fatalf("unexpected name: %q", opts.Name)
	}
	if opts.Start || opts.Force == true || !opts.SkipBuild {
		t.Fatalf("unexpected flags: %+v", opts)
	}
}

func TestParseSetupServiceArgsRejectsInvalidTarget(t *testing.T) {
	if _, err := parseSetupServiceArgs([]string{"--target", "invalid", "--binary", "/tmp/clawx-bin"}); err == nil {
		t.Fatalf("expected parse error")
	}
}
