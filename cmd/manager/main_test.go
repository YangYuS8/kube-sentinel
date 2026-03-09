package main

import "testing"

func TestEnvOrDefaultFallsBackOnEmptyValue(t *testing.T) {
	t.Setenv("KUBE_SENTINEL_TEST_ENV", "")
	if got := envOrDefault("KUBE_SENTINEL_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestEnvOrDefaultUsesConfiguredValue(t *testing.T) {
	t.Setenv("KUBE_SENTINEL_TEST_ENV", ":9999")
	if got := envOrDefault("KUBE_SENTINEL_TEST_ENV", "fallback"); got != ":9999" {
		t.Fatalf("expected configured value, got %q", got)
	}
}

func TestEnvBoolOrDefault(t *testing.T) {
	t.Setenv("KUBE_SENTINEL_BOOL_ENV", "true")
	if !envBoolOrDefault("KUBE_SENTINEL_BOOL_ENV", false) {
		t.Fatalf("expected true value to parse")
	}
	t.Setenv("KUBE_SENTINEL_BOOL_ENV", "")
	if !envBoolOrDefault("KUBE_SENTINEL_BOOL_ENV", true) {
		t.Fatalf("expected fallback when empty")
	}
}

func TestParseRuntimeModeDefaultsToLegacyForUnknownValue(t *testing.T) {
	if got := parseRuntimeMode("minimal"); got != "minimal" {
		t.Fatalf("expected minimal mode, got %q", got)
	}
	if got := parseRuntimeMode("unexpected"); got != "" {
		t.Fatalf("expected legacy mode for unknown values, got %q", got)
	}
}
