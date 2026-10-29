package commands

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	libarazzo "github.com/pb33f/libopenapi/arazzo"
	"go.yaml.in/yaml/v4"

	"github.com/shaunhoulihan/arazzo-go/internal/runner"
)

func TestFormatWorkflowList(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatWorkflowList([]runner.WorkflowInfo{
		{WorkflowID: "login-with-password", Summary: "Log in with email and password"},
		{WorkflowID: "other"},
	})
	if !strings.Contains(got, "arazzo") || !strings.Contains(got, "Workflows") {
		t.Fatalf("missing title: %q", got)
	}
	if !strings.Contains(got, "login-with-password") || !strings.Contains(got, "Log in with email and password") {
		t.Fatalf("missing row: %q", got)
	}
	if !strings.Contains(got, "2 workflows") {
		t.Fatalf("missing summary: %q", got)
	}
}

func TestFormatWorkflowInfo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatWorkflowInfo(&runner.WorkflowInfo{
		WorkflowID:  "login-with-password",
		Summary:     "Log in",
		Description: "Drives POST /auth/login.",
		OutputNames: []string{"session"},
		Steps: []runner.StepInfo{
			{StepID: "submit-credentials", OperationID: "$sourceDescriptions.userSessionApi.passwordLogin"},
		},
	})
	if !strings.Contains(got, "login-with-password") {
		t.Fatalf("missing workflow id: %q", got)
	}
	if !strings.Contains(got, "summary:") || !strings.Contains(got, "Log in") {
		t.Fatalf("missing summary: %q", got)
	}
	if !strings.Contains(got, "submit-credentials") {
		t.Fatalf("missing step: %q", got)
	}
	if !strings.Contains(got, "outputs: session") {
		t.Fatalf("missing outputs: %q", got)
	}
}

func TestFormatResult(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatResult(&libarazzo.WorkflowResult{
		WorkflowId: "login-with-password",
		Success:    false,
		Duration:   12 * time.Millisecond,
		Error:      errors.New("unauthorized"),
		Steps: []*libarazzo.StepResult{
			{
				StepId:   "submit-credentials",
				Success:  false,
				Duration: 5 * time.Millisecond,
				Error:    errors.New("401"),
			},
		},
	})
	if !strings.Contains(got, "status: failed") {
		t.Fatalf("missing status: %q", got)
	}
	if !strings.Contains(got, "submit-credentials") || !strings.Contains(got, "401") {
		t.Fatalf("missing step: %q", got)
	}
	if !strings.Contains(got, "unauthorized") {
		t.Fatalf("missing workflow error: %q", got)
	}
}

func TestFormatJSONResult(t *testing.T) {
	result := &libarazzo.WorkflowResult{
		WorkflowId: "test-workflow",
		Success:    false,
		Duration:   15 * time.Millisecond,
		Error:      errors.New("workflow error"),
		Outputs: map[string]any{
			"sessionToken": "secret-xyz",
			"publicData":   "hello",
		},
		Steps: []*libarazzo.StepResult{
			{
				StepId:     "step-1",
				Success:    true,
				Duration:   5 * time.Millisecond,
				StatusCode: 200,
				Outputs: map[string]any{
					"passwordHash": "secret-hash",
				},
			},
			{
				StepId:     "step-2",
				Success:    false,
				Duration:   10 * time.Millisecond,
				StatusCode: 401,
				Error:      errors.New("unauthorized"),
			},
		},
	}

	got, err := formatJSONResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed JSONWorkflowResult
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if parsed.WorkflowID != "test-workflow" || parsed.Success {
		t.Errorf("unexpected parsed workflow header: %+v", parsed)
	}
	if parsed.Error == nil || *parsed.Error != "workflow error" {
		t.Errorf("unexpected error: %v", parsed.Error)
	}
	if parsed.DurationMS != 15.0 {
		t.Errorf("got duration %v, want 15.0", parsed.DurationMS)
	}
	if len(parsed.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(parsed.Steps))
	}

	// Verify secrets are redacted in outputs
	if parsed.Outputs["sessionToken"] != "***" {
		t.Errorf("sessionToken not redacted: %v", parsed.Outputs["sessionToken"])
	}
	if parsed.Outputs["publicData"] != "hello" {
		t.Errorf("publicData changed: %v", parsed.Outputs["publicData"])
	}
	if parsed.Steps[0].Outputs["passwordHash"] != "***" {
		t.Errorf("passwordHash not redacted: %v", parsed.Steps[0].Outputs["passwordHash"])
	}
}

func TestFormatJUnitResult(t *testing.T) {
	result := &libarazzo.WorkflowResult{
		WorkflowId: "checkout-flow",
		Success:    false,
		Duration:   2500 * time.Millisecond,
		Steps: []*libarazzo.StepResult{
			{
				StepId:   "login",
				Success:  true,
				Duration: 500 * time.Millisecond,
			},
			{
				StepId:   "pay",
				Success:  false,
				Duration: 2000 * time.Millisecond,
				Error:    errors.New("insufficient funds"),
			},
		},
	}

	got, err := formatJUnitResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(got, xml.Header) {
		t.Errorf("missing xml header: %q", got)
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal([]byte(got), &suites); err != nil {
		t.Fatalf("invalid xml: %v", err)
	}

	if suites.Tests != 2 || suites.Failures != 1 {
		t.Errorf("expected tests=2, failures=1; got tests=%d failures=%d", suites.Tests, suites.Failures)
	}
	if len(suites.TestSuite) != 1 {
		t.Fatalf("expected 1 testsuite, got %d", len(suites.TestSuite))
	}

	suite := suites.TestSuite[0]
	if suite.Name != "checkout-flow" || suite.Tests != 2 || suite.Failures != 1 {
		t.Errorf("unexpected testsuite attributes: %+v", suite)
	}
	if len(suite.TestCases) != 2 {
		t.Fatalf("expected 2 testcases, got %d", len(suite.TestCases))
	}
	if suite.TestCases[0].Failure != nil {
		t.Errorf("login should not have failure")
	}
	if suite.TestCases[1].Failure == nil || suite.TestCases[1].Failure.Message != "insufficient funds" {
		t.Errorf("pay should have failure with 'insufficient funds'")
	}
}

func TestWorkflowLevelErrorOutputs(t *testing.T) {
	result := &libarazzo.WorkflowResult{
		WorkflowId: "empty-flow",
		Success:    false,
		Duration:   0,
		Error:      errors.New("failed before starting"),
		Steps:      nil,
	}

	// 1. JSON should have steps: [] and error set
	jsonOut, err := formatJSONResult(result)
	if err != nil {
		t.Fatal(err)
	}
	var parsed JSONWorkflowResult
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Error == nil || *parsed.Error != "failed before starting" {
		t.Errorf("expected error string, got %v", parsed.Error)
	}
	if len(parsed.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(parsed.Steps))
	}

	// 2. JUnit should emit 1 synthetic testcase with failure
	junitOut, err := formatJUnitResult(result)
	if err != nil {
		t.Fatal(err)
	}
	var suites JUnitTestSuites
	if err := xml.Unmarshal([]byte(junitOut), &suites); err != nil {
		t.Fatal(err)
	}
	if suites.Tests != 1 || suites.Failures != 1 {
		t.Errorf("expected tests=1, failures=1; got tests=%d failures=%d", suites.Tests, suites.Failures)
	}
	if len(suites.TestSuite) != 1 || len(suites.TestSuite[0].TestCases) != 1 {
		t.Fatalf("expected 1 testcase, got %v", suites.TestSuite)
	}
	tc := suites.TestSuite[0].TestCases[0]
	if tc.Name != "workflow" || tc.Failure == nil || tc.Failure.Message != "failed before starting" {
		t.Errorf("unexpected synthetic testcase: %+v", tc)
	}
}

func TestFormatJSONRendersYAMLMappingNodes(t *testing.T) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	node.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "high"},
		{Kind: yaml.ScalarNode, Tag: "!!float", Value: "1"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "critical"},
		{Kind: yaml.ScalarNode, Tag: "!!float", Value: "0"},
	}
	got := formatJSON(map[string]any{"issueCounts": node})
	if !strings.Contains(got, `"high":1`) || !strings.Contains(got, `"critical":0`) {
		t.Fatalf("expected readable counts, got %s", got)
	}
	if strings.Contains(got, `"Kind"`) {
		t.Fatalf("raw yaml.Node leaked: %s", got)
	}
}

func TestGoldenFilesWithLoginWorkflow(t *testing.T) {
	// Set up mock auth server matching testdata/login.arazzo.yaml
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "ifc_session", Value: "id-token", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "ifc_csrf", Value: "csrf-token", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Load workflow from testdata/login.arazzo.yaml
	specPath := filepath.Join("..", "..", "testdata", "login.arazzo.yaml")
	r, err := runner.FromFile(specPath)
	if err != nil {
		t.Fatalf("load runner: %v", err)
	}

	result, err := r.ExecuteWorkflow(context.Background(), "login-with-password", map[string]any{
		"email":    "user@example.com",
		"password": "secret",
		"baseUrl":  srv.URL,
	})
	if err != nil {
		t.Fatalf("execute login: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %v", result.Error)
	}

	// Normalize durations to ensure deterministic golden file output
	result.Duration = 10 * time.Millisecond
	for _, step := range result.Steps {
		step.Duration = 10 * time.Millisecond
	}

	// 1. Check JSON Golden File
	jsonOut, err := formatJSONResult(result)
	if err != nil {
		t.Fatalf("format json: %v", err)
	}
	compareOrUpdateGolden(t, filepath.Join("..", "..", "testdata", "login_result.json.golden"), jsonOut)

	// 2. Check JUnit Golden File
	junitOut, err := formatJUnitResult(result)
	if err != nil {
		t.Fatalf("format junit: %v", err)
	}
	compareOrUpdateGolden(t, filepath.Join("..", "..", "testdata", "login_result.junit.golden"), junitOut)
}

func compareOrUpdateGolden(t *testing.T, path, actual string) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("failed to update golden file %s: %v", path, err)
		}
		return
	}

	expectedBytes, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist yet, write it
		if os.IsNotExist(err) {
			if writeErr := os.WriteFile(path, []byte(actual), 0o644); writeErr != nil {
				t.Fatalf("failed to create golden file %s: %v", path, writeErr)
			}
			return
		}
		t.Fatalf("failed to read golden file %s: %v", path, err)
	}

	expected := string(expectedBytes)
	if strings.TrimSpace(actual) != strings.TrimSpace(expected) {
		t.Errorf("golden mismatch for %s:\n--- Expected ---\n%s\n--- Actual ---\n%s", path, expected, actual)
	}
}
