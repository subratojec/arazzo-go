package commands

import (
	"log/slog"
	"testing"
)

func TestEffectiveLogLevel(t *testing.T) {
	t.Parallel()

	if got := effectiveLogLevel("info", false); got != slog.LevelInfo {
		t.Fatalf("default info: got %v", got)
	}
	if got := effectiveLogLevel("warn", false); got != slog.LevelWarn {
		t.Fatalf("warn: got %v", got)
	}
	if got := effectiveLogLevel("error", true); got != slog.LevelDebug {
		t.Fatalf("verbose overrides log-level: got %v", got)
	}
	if got := effectiveLogLevel("info", true); got != slog.LevelDebug {
		t.Fatalf("verbose: got %v", got)
	}
}

func TestExecuteWorkflowVerboseFlag(t *testing.T) {
	t.Parallel()
	flag := executeCmd.Flags().Lookup("verbose")
	if flag == nil {
		t.Fatal("missing --verbose")
	}
	if flag.Shorthand != "v" {
		t.Fatalf("shorthand: got %q", flag.Shorthand)
	}
	if flag.DefValue != "false" {
		t.Fatalf("default: got %q", flag.DefValue)
	}
}

func TestExecuteWorkflowFormatFlag(t *testing.T) {
	t.Parallel()
	flag := executeCmd.Flags().Lookup("format")
	if flag == nil {
		t.Fatal("missing --format")
	}
	if flag.DefValue != "text" {
		t.Fatalf("default: got %q, want text", flag.DefValue)
	}
}

func TestExecuteWorkflowOutputFlag(t *testing.T) {
	t.Parallel()
	flag := executeCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("missing --output")
	}
	if flag.Shorthand != "o" {
		t.Fatalf("shorthand: got %q, want o", flag.Shorthand)
	}
	if flag.DefValue != "" {
		t.Fatalf("default: got %q, want empty", flag.DefValue)
	}
}
