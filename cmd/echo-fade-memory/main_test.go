package main

import (
	"errors"
	"os"
	"testing"
)

func TestApplyServeRuntimeOverridesSetsRuntimeHomeAndWorkspace(t *testing.T) {
	t.Setenv("ECHO_FADE_MEMORY_HOME", "")
	t.Setenv("ECHO_FADE_MEMORY_WORKSPACE", "")
	t.Setenv("PORT", "")

	err := applyServeRuntimeOverrides([]string{
		"--workdir", "/Users/system/.echo-fade-memory",
		"--workspace=debug-local",
		"--port", "9090",
	})
	if err != nil {
		t.Fatalf("applyServeRuntimeOverrides() error = %v", err)
	}

	if got := os.Getenv("ECHO_FADE_MEMORY_HOME"); got != "/Users/system/.echo-fade-memory" {
		t.Fatalf("ECHO_FADE_MEMORY_HOME = %q, want %q", got, "/Users/system/.echo-fade-memory")
	}
	if got := os.Getenv("ECHO_FADE_MEMORY_WORKSPACE"); got != "debug-local" {
		t.Fatalf("ECHO_FADE_MEMORY_WORKSPACE = %q, want %q", got, "debug-local")
	}
	if got := os.Getenv("PORT"); got != "9090" {
		t.Fatalf("PORT = %q, want %q", got, "9090")
	}
}

func TestApplyServeRuntimeOverridesHelp(t *testing.T) {
	err := applyServeRuntimeOverrides([]string{"--help"})
	if !errors.Is(err, errServeHelp) {
		t.Fatalf("error = %v, want errServeHelp", err)
	}
}

func TestApplyServeRuntimeOverridesRejectsUnknownOption(t *testing.T) {
	err := applyServeRuntimeOverrides([]string{"--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown option")
	}
}

func TestApplyServeRuntimeOverridesRejectsMissingValue(t *testing.T) {
	err := applyServeRuntimeOverrides([]string{"--workdir"})
	if err == nil {
		t.Fatal("expected error for missing --workdir value")
	}
}

func TestApplyServeRuntimeOverridesRejectsInvalidPort(t *testing.T) {
	err := applyServeRuntimeOverrides([]string{"--port", "0"})
	if err == nil {
		t.Fatal("expected error for invalid --port value")
	}
}
