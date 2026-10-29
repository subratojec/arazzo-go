package commands

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shaunhoulihan/arazzo-go/internal/executor"
	"github.com/shaunhoulihan/arazzo-go/internal/runner"
	"github.com/shaunhoulihan/arazzo-go/internal/ui"
	"go.yaml.in/yaml/v4"
)

// JSONWorkflowResult represents the structured JSON output for a workflow execution.
type JSONWorkflowResult struct {
	WorkflowID string           `json:"workflowId"`
	Success    bool             `json:"success"`
	DurationMS float64          `json:"durationMs"`
	Error      *string          `json:"error"`
	Outputs    map[string]any   `json:"outputs,omitempty"`
	Steps      []JSONStepResult `json:"steps"`
}

// JSONStepResult represents a single executed step inside the JSON workflow result.
type JSONStepResult struct {
	StepID     string         `json:"stepId"`
	Success    bool           `json:"success"`
	DurationMS float64        `json:"durationMs"`
	StatusCode int            `json:"statusCode,omitempty"`
	Retries    int            `json:"retries,omitempty"`
	Error      *string        `json:"error"`
	Outputs    map[string]any `json:"outputs,omitempty"`
}

// JUnitTestSuites is the root XML container for JUnit test reports.
type JUnitTestSuites struct {
	XMLName   xml.Name         `xml:"testsuites"`
	Name      string           `xml:"name,attr,omitempty"`
	Tests     int              `xml:"tests,attr"`
	Failures  int              `xml:"failures,attr"`
	Errors    int              `xml:"errors,attr"`
	Time      string           `xml:"time,attr"`
	TestSuite []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a single testsuite mapping to an Arazzo workflow.
type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      string          `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents an executed step (or workflow-level check) inside a suite.
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

// JUnitFailure describes a failed assertion or execution error.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr,omitempty"`
	Content string `xml:",chardata"`
}

func formatWorkflowList(workflows []runner.WorkflowInfo) string {
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle("Workflows"))
	if len(workflows) == 0 {
		fmt.Fprintln(&b, "  "+ui.KeyHints("(none)"))
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, ui.KeyHints("0 workflows"))
		return b.String()
	}
	idWidth := 0
	for _, wf := range workflows {
		if n := len(wf.WorkflowID); n > idWidth {
			idWidth = n
		}
	}
	for _, wf := range workflows {
		id := ui.Apply(ui.Emphasis, fmt.Sprintf("%-*s", idWidth, wf.WorkflowID))
		if wf.Summary != "" {
			fmt.Fprintf(&b, "  %s  %s\n", id, wf.Summary)
			continue
		}
		fmt.Fprintf(&b, "  %s\n", id)
	}
	fmt.Fprintln(&b)
	label := "workflow"
	if len(workflows) != 1 {
		label = "workflows"
	}
	fmt.Fprintln(&b, ui.KeyHints(fmt.Sprintf("%d %s", len(workflows), label)))
	return b.String()
}

func formatWorkflowInfo(info *runner.WorkflowInfo) string {
	if info == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle(info.WorkflowID))
	if info.Summary != "" {
		fmt.Fprintln(&b, "  "+ui.Field("summary", info.Summary))
	}
	if desc := strings.TrimSpace(info.Description); desc != "" {
		fmt.Fprintln(&b, "  "+ui.Section("description"))
		for _, line := range strings.Split(desc, "\n") {
			fmt.Fprintln(&b, "    "+line)
		}
	}
	if len(info.OutputNames) > 0 {
		fmt.Fprintln(&b, "  "+ui.Field("outputs", strings.Join(info.OutputNames, ", ")))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, ui.Section("steps"))
	if len(info.Steps) == 0 {
		fmt.Fprintln(&b, "  "+ui.KeyHints("(none)"))
	}
	numWidth := len(fmt.Sprintf("%d", len(info.Steps)))
	for i, step := range info.Steps {
		if i > 0 {
			fmt.Fprintln(&b)
		}
		target := step.OperationID
		if target == "" {
			target = step.OperationPath
		}
		if target == "" {
			target = step.WorkflowID
		}
		fmt.Fprintf(&b, "  %s  %s\n", ui.KeyHints(fmt.Sprintf("%*d", numWidth, i+1)), ui.Apply(ui.Emphasis, step.StepID))
		if target != "" {
			fmt.Fprintf(&b, "     %s\n", ui.KeyHints(target))
		}
		if desc := strings.TrimSpace(step.Description); desc != "" {
			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "     %s\n", line)
			}
		}
	}
	return b.String()
}

func formatWorkflowResult(result *runner.Result, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return formatResult(result), nil
	case "json":
		return formatJSONResult(result)
	case "junit":
		return formatJUnitResult(result)
	default:
		return "", fmt.Errorf("unsupported output format %q: expected text, json, or junit", format)
	}
}

func formatResult(result *runner.Result) string {
	if result == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle("Workflow "+result.WorkflowId))
	status := ui.Apply(ui.Success, "success")
	if !result.Success {
		status = ui.Apply(ui.Error, "failed")
	}
	fmt.Fprintln(&b, "  "+ui.Field("status", status))
	fmt.Fprintln(&b, "  "+ui.Field("duration", result.Duration.String()))
	fmt.Fprintln(&b, "  "+ui.Field("steps", fmt.Sprintf("%d", len(result.Steps))))
	if len(result.Outputs) > 0 {
		fmt.Fprintln(&b, "  "+ui.Field("outputs", formatJSON(result.Outputs)))
	}
	if result.Error != nil {
		fmt.Fprintln(&b, "  "+ui.Field("error", ui.Apply(ui.Error, result.Error.Error())))
	}
	if len(result.Steps) == 0 {
		return b.String()
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, ui.Section("steps"))
	for _, step := range result.Steps {
		if step == nil {
			continue
		}
		mark := ui.Apply(ui.Success, "✓")
		if !step.Success {
			mark = ui.Apply(ui.Error, "✗")
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n", mark, ui.Apply(ui.Emphasis, step.StepId), ui.KeyHints(step.Duration.String()))
		if !step.Success && step.Error != nil {
			fmt.Fprintf(&b, "     %s\n", ui.Apply(ui.Error, step.Error.Error()))
		}
		if len(step.Outputs) > 0 {
			fmt.Fprintf(&b, "     %s %s\n", ui.KeyHints("outputs:"), formatJSON(step.Outputs))
		}
	}
	return b.String()
}

func formatJSONResult(result *runner.Result) (string, error) {
	if result == nil {
		return "{}\n", nil
	}

	res := JSONWorkflowResult{
		WorkflowID: result.WorkflowId,
		Success:    result.Success,
		DurationMS: durationToMS(result.Duration),
		Outputs:    cleanOutputs(result.Outputs),
		Steps:      make([]JSONStepResult, 0, len(result.Steps)),
	}

	if result.Error != nil {
		errStr := result.Error.Error()
		res.Error = &errStr
	}

	for _, step := range result.Steps {
		if step == nil {
			continue
		}
		sr := JSONStepResult{
			StepID:     step.StepId,
			Success:    step.Success,
			DurationMS: durationToMS(step.Duration),
			StatusCode: step.StatusCode,
			Retries:    step.Retries,
			Outputs:    cleanOutputs(step.Outputs),
		}
		if step.Error != nil {
			errStr := step.Error.Error()
			sr.Error = &errStr
		}
		res.Steps = append(res.Steps, sr)
	}

	bytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json result: %w", err)
	}
	return string(bytes) + "\n", nil
}

func formatJUnitResult(result *runner.Result) (string, error) {
	if result == nil {
		result = &runner.Result{WorkflowId: "unknown", Success: false}
	}

	suite := JUnitTestSuite{
		Name:      result.WorkflowId,
		Time:      formatSeconds(result.Duration),
		TestCases: make([]JUnitTestCase, 0, len(result.Steps)),
	}

	if len(result.Steps) == 0 {
		// When no steps ran (e.g. workflow-level error), emit a synthetic testcase
		// so CI still gets a valid testsuite with a failure rather than an empty report.
		suite.Tests = 1
		tc := JUnitTestCase{
			Name:      "workflow",
			Classname: result.WorkflowId,
			Time:      formatSeconds(result.Duration),
		}
		if !result.Success || result.Error != nil {
			suite.Failures = 1
			errMsg := "workflow failed"
			if result.Error != nil {
				errMsg = result.Error.Error()
			}
			tc.Failure = &JUnitFailure{
				Message: errMsg,
				Type:    "WorkflowFailure",
				Content: errMsg,
			}
		}
		suite.TestCases = append(suite.TestCases, tc)
	} else {
		suite.Tests = len(result.Steps)
		for _, step := range result.Steps {
			if step == nil {
				continue
			}
			tc := JUnitTestCase{
				Name:      step.StepId,
				Classname: result.WorkflowId,
				Time:      formatSeconds(step.Duration),
			}
			if !step.Success {
				suite.Failures++
				errMsg := "step failed"
				if step.Error != nil {
					errMsg = step.Error.Error()
				}
				tc.Failure = &JUnitFailure{
					Message: errMsg,
					Type:    "StepFailure",
					Content: errMsg,
				}
			}
			suite.TestCases = append(suite.TestCases, tc)
		}
	}

	suites := JUnitTestSuites{
		Name:      "arazzo",
		Tests:     suite.Tests,
		Failures:  suite.Failures,
		Errors:    0,
		Time:      suite.Time,
		TestSuite: []JUnitTestSuite{suite},
	}

	data, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal junit xml: %w", err)
	}
	return xml.Header + string(data) + "\n", nil
}

func durationToMS(d time.Duration) float64 {
	// Microsecond precision: e.g. 1250 µs -> 1.25 ms
	return float64(d.Microseconds()) / 1000.0
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

func cleanOutputs(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	j := jsonable(raw)
	m, ok := j.(map[string]any)
	if !ok {
		return nil
	}
	redacted := executor.RedactValue(m)
	res, _ := redacted.(map[string]any)
	return res
}

func formatJSON(v any) string {
	b, err := json.Marshal(jsonable(v))
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func jsonable(v any) any {
	switch t := v.(type) {
	case *yaml.Node:
		return yamlNodeJSON(t)
	case yaml.Node:
		return yamlNodeJSON(&t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = jsonable(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = jsonable(val)
		}
		return out
	default:
		return v
	}
}

func yamlNodeJSON(node *yaml.Node) any {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return yamlNodeJSON(node.Content[0])
	}
	switch node.Kind {
	case yaml.MappingNode:
		out := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			out[node.Content[i].Value] = yamlNodeJSON(node.Content[i+1])
		}
		return out
	case yaml.SequenceNode:
		out := make([]any, len(node.Content))
		for i, child := range node.Content {
			out[i] = yamlNodeJSON(child)
		}
		return out
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!int":
			if v, err := strconv.ParseInt(node.Value, 10, 64); err == nil {
				return v
			}
		case "!!float":
			if v, err := strconv.ParseFloat(node.Value, 64); err == nil {
				return v
			}
		case "!!bool":
			if v, err := strconv.ParseBool(node.Value); err == nil {
				return v
			}
		case "!!null":
			return nil
		}
		return node.Value
	default:
		return node.Value
	}
}
