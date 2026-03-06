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
