package main

import "testing"

func TestResolveTaskControlMetricsRouteEnabled(t *testing.T) {
	if !resolveTaskControlMetricsRouteEnabled("") {
		t.Fatalf("expected default enabled")
	}
	for _, raw := range []string{"0", "false", "off", "no", "disabled", " FALSE "} {
		if resolveTaskControlMetricsRouteEnabled(raw) {
			t.Fatalf("expected disabled for %q", raw)
		}
	}
	for _, raw := range []string{"1", "true", "on", "yes", "custom"} {
		if !resolveTaskControlMetricsRouteEnabled(raw) {
			t.Fatalf("expected enabled for %q", raw)
		}
	}
}

func TestResolveTaskControlMetricsRoutePath(t *testing.T) {
	cases := map[string]string{
		"":                       "/metrics/task-control",
		"metrics/task-control":   "/metrics/task-control",
		"/metrics/task-control/": "/metrics/task-control",
		"/":                      "/",
	}
	for in, expected := range cases {
		got := resolveTaskControlMetricsRoutePath(in)
		if got != expected {
			t.Fatalf("path normalize mismatch input=%q expected=%q got=%q", in, expected, got)
		}
	}
}

func TestResolveTaskControlPrometheusRouteEnabled(t *testing.T) {
	if !resolveTaskControlPrometheusRouteEnabled("") {
		t.Fatalf("expected default enabled")
	}
	for _, raw := range []string{"0", "false", "off", "no", "disabled", " FALSE "} {
		if resolveTaskControlPrometheusRouteEnabled(raw) {
			t.Fatalf("expected disabled for %q", raw)
		}
	}
	for _, raw := range []string{"1", "true", "on", "yes", "custom"} {
		if !resolveTaskControlPrometheusRouteEnabled(raw) {
			t.Fatalf("expected enabled for %q", raw)
		}
	}
}

func TestResolveTaskControlPrometheusRoutePath(t *testing.T) {
	cases := map[string]string{
		"":                                  "/metrics/task-control/prometheus",
		"metrics/task-control/prometheus":   "/metrics/task-control/prometheus",
		"/metrics/task-control/prometheus/": "/metrics/task-control/prometheus",
		"/":                                 "/",
	}
	for in, expected := range cases {
		got := resolveTaskControlPrometheusRoutePath(in)
		if got != expected {
			t.Fatalf("path normalize mismatch input=%q expected=%q got=%q", in, expected, got)
		}
	}
}
