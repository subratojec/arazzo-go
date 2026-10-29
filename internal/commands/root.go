package commands

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shaunhoulihan/arazzo-go/internal"
	"github.com/shaunhoulihan/arazzo-go/internal/runner"
	"github.com/shaunhoulihan/arazzo-go/internal/ui"
)

var (
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "arazzo",
	Short: "Run Arazzo 1.0.1 workflows",
	Long: fmt.Sprintf(`
arazzo loads an Arazzo document, resolves the referenced OpenAPI sources,
and executes HTTP steps with a cookie jar so session cookies persist.

Typical flow: list-workflows → describe-workflow → execute-workflow.

Build Version: %s
Git Commit: %s
Build Time: %s`,
		internal.BuildVersion, internal.GitCommit, internal.BuildTime),
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func printError(err error) {
	fmtMode := strings.ToLower(strings.TrimSpace(executeFormat))
	if fmtMode == "json" || fmtMode == "junit" {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Fprintln(os.Stderr, ui.Apply(ui.Error, fmt.Sprintf("Error: %v", err)))
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "debug, info, warn, or error")
}

func loadRunner(path string, verbose bool) (*runner.Runner, error) {
	return runner.FromFile(path, runner.WithLogger(newCLILogger(verbose)))
}

func newCLILogger(verbose bool) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: effectiveLogLevel(logLevel, verbose)}))
}

func effectiveLogLevel(raw string, verbose bool) slog.Level {
	if verbose {
		return slog.LevelDebug
	}
	return parseLogLevel(raw)
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
