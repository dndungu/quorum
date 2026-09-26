package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReviewContextFlags(t *testing.T) {
	o, err := parseArgs([]string{"--audience", "engineering leads", "--decision", "approve rollout", "--evidence", "a.md", "--evidence", "b.txt", "proposal.md"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Audience != "engineering leads" || o.Decision != "approve rollout" || len(o.Evidence) != 2 {
		t.Fatalf("lost review context: %+v", o)
	}
}

func TestInvalidExecutionLimits(t *testing.T) {
	for _, args := range [][]string{{"--parallel", "0"}, {"--parallel", "-1"}, {"--timeout", "0s"}, {"--timeout", "-1s"}, {"--max-tokens", "0"}} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestDecisionPreview(t *testing.T) {
	tests := []struct{ input, want string }{
		{"Preamble\n## Decision\nTrial first.\nPreserve the qualification.\n\n## Next edits\nDetails", "Trial first.\nPreserve the qualification."},
		{"## decision\r\nAdopt.\r\n", "Adopt."},
		{"Nonconforming response", ""},
	}
	for _, tt := range tests {
		if got := decisionText(tt.input); got != tt.want {
			t.Errorf("decisionText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBriefEvidenceAndTemplateErrors(t *testing.T) {
	dir := t.TempDir()
	evidence := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(evidence, []byte("Observed result\nImportant qualification"), 0o600); err != nil {
		t.Fatal(err)
	}
	brief, err := buildBrief("Proposed rule\nSecond paragraph", &Options{Audience: "authors", Decision: "adopt rule", Evidence: []string{evidence}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"authors", "adopt rule", "D:L1", "E1:L2", "Important qualification"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	for _, o := range []*Options{{Template: filepath.Join(dir, "missing")}, {Evidence: []string{filepath.Join(dir, "missing")}}} {
		if _, err := buildBrief("document", o); err == nil {
			t.Errorf("missing input must fail: %+v", o)
		}
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, o := range []*Options{{Template: empty}, {Evidence: []string{empty}}} {
		if _, err := buildBrief("document", o); err == nil {
			t.Errorf("empty input must fail: %+v", o)
		}
	}
	custom := filepath.Join(dir, "template.md")
	if err := os.WriteFile(custom, []byte("Return only a checklist."), 0o600); err != nil {
		t.Fatal(err)
	}
	brief, err = buildBrief("document", &Options{Template: custom, Evidence: []string{evidence}})
	if err != nil || !strings.Contains(brief, "Return only a checklist.") || !strings.Contains(brief, "E1:L2") || strings.Contains(brief, defaultTemplate) {
		t.Fatalf("custom template must replace instructions while retaining evidence: %v", err)
	}
}

func TestCrossExaminationRetainsEvidenceAndDistinctPeers(t *testing.T) {
	t.Setenv("QUORUM_TEST_KEY", "test-key")
	var mu sync.Mutex
	var prompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		prompts = append(prompts, req.Messages[0].Content)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "revised review"}}}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	var seats []Seat
	var reviews []Review
	for _, name := range []string{"alpha", "beta", "gamma"} {
		s := Seat{Name: name, Model: "test", BaseURL: server.URL, APIKeyEnv: "QUORUM_TEST_KEY"}
		seats = append(seats, s)
		reviews = append(reviews, Review{Seat: s, Text: "initial " + name})
	}
	const brief = "Original document and supporting evidence: uniquely identifiable context"
	got, err := crossExamine(seats, reviews, brief, 2, &Options{Parallel: 2, Timeout: time.Second})
	if err != nil || len(got) != 3 {
		t.Fatalf("cross-examination: %v, %v", got, err)
	}
	for _, prompt := range prompts {
		if !strings.Contains(prompt, brief) {
			t.Error("cross-examination lost original source and evidence")
		}
		labels := 0
		for _, label := range []string{"--- Reviewer 1 ---", "--- Reviewer 2 ---", "--- Reviewer 3 ---"} {
			if strings.Contains(prompt, label) {
				labels++
			}
		}
		if labels != 2 {
			t.Errorf("need two distinct peer labels, got %d", labels)
		}
	}
}

func TestRunPreservesSourcesAndAccountsForEveryRound(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("QUORUM_TEST_KEY", "test-key")
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		prompt := req.Messages[0].Content
		for _, want := range []string{"[D:L1] Draft claim", "[E1:L2] Only one show was sampled", "skill authors", "adopt a procedure"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("request lost %q", want)
			}
		}
		mu.Lock()
		calls++
		mu.Unlock()
		content := "VERDICT: qualify the claim."
		if strings.Contains(prompt, "=== CHAIR INSTRUCTIONS ===") {
			content = "## Decision\nQualify the claim before adoption.\n\n## Next edits\nLimit the claim to one show (E1:L2)."
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
			"usage":   map[string]any{"cost": 0.01, "prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	files := map[string]string{
		"config.yaml": "defaults:\n  base_url: " + server.URL + "\n  api_key_env: QUORUM_TEST_KEY\nseats:\n  - name: one\n    model: test\n  - name: two\n    model: test\n",
		"draft.md":    "Draft claim\nMore context",
		"source.txt":  "Observation\nOnly one show was sampled",
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{"--config", "config.yaml", "--out", "runs", "--rounds", "2", "--audience", "skill authors", "--decision", "adopt a procedure", "--evidence", "source.txt", "draft.md"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("runs")
	if err != nil || len(entries) != 1 {
		t.Fatalf("run directories: %v, %v", entries, err)
	}
	dir := filepath.Join("runs", entries[0].Name())
	for _, name := range []string{"source.md", "brief.md", "reviews/one.md", "reviews/two.md", "reviews/one-r2.md", "reviews/two-r2.md", "synthesis.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Cost        CostSummary `json:"cost"`
		Audience    string      `json:"audience"`
		ReviewCalls []any       `json:"review_calls"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if calls != 5 || summary.Cost.Calls != 5 || summary.Cost.ReportedUSD != 0.05 || summary.Cost.TotalTokens != 75 || len(summary.ReviewCalls) != 4 || summary.Audience != "skill authors" {
		t.Fatalf("lost calls or context: calls=%d summary=%+v", calls, summary)
	}
}

func TestFailedCrossExaminationRetainsTextWithoutDoubleCounting(t *testing.T) {
	t.Setenv("QUORUM_TEST_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"temporarily unavailable"}}`))
	}))
	defer server.Close()
	s := Seat{Name: "one", Model: "test", BaseURL: server.URL, APIKeyEnv: "QUORUM_TEST_KEY"}
	cost := 0.01
	previous := Review{Seat: s, Round: 1, Text: "Earlier supported finding", Usage: LLMUsage{CostUSD: &cost}}
	next, err := crossExamine([]Seat{s}, []Review{previous}, "source", 2, &Options{Parallel: 1, Timeout: time.Second})
	if err != nil || len(next) != 1 {
		t.Fatalf("cross-examination: %v %v", next, err)
	}
	if next[0].Text != previous.Text || next[0].Err == "" || next[0].Round != 2 || next[0].Usage.CostUSD != nil {
		t.Fatalf("failed round must retain text, record failure, and not reuse usage: %+v", next[0])
	}
	summary := summarizeCost(append([]Review{previous}, next...), LLMUsage{}, false)
	if summary.Calls != 2 || summary.ReportedUSD != cost || summary.UnreportedCalls != 1 {
		t.Fatalf("wrong fallback cost: %+v", summary)
	}
}

func TestTokenCapRejectsIncompleteResponseButPreservesUsage(t *testing.T) {
	t.Setenv("QUORUM_TEST_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.MaxTokens != 128 {
			t.Errorf("max_tokens = %d, want 128", req.MaxTokens)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "length", "message": map[string]string{"content": "Half a review"}}},
			"usage":   map[string]any{"cost": 0.01, "total_tokens": 150},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	result, err := callLLM(Seat{Name: "one", Model: "test", BaseURL: server.URL, APIKeyEnv: "QUORUM_TEST_KEY"}, "source", time.Second, 0, 128)
	if err == nil || !strings.Contains(err.Error(), "incomplete") || result.Text != "" || result.Usage.CostUSD == nil || *result.Usage.CostUSD != 0.01 || result.Usage.TotalTokens != 150 {
		t.Fatalf("incomplete review accepted or usage lost: %+v %v", result, err)
	}
}

func TestFailedSynthesisStillWritesCostSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("QUORUM_TEST_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		reason := "stop"
		if strings.Contains(req.Messages[0].Content, "=== CHAIR INSTRUCTIONS ===") {
			reason = "length"
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": reason, "message": map[string]string{"content": "review"}}},
			"usage":   map[string]any{"cost": 0.01},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	for name, content := range map[string]string{
		"config.yaml": "defaults:\n  base_url: " + server.URL + "\n  api_key_env: QUORUM_TEST_KEY\nseats:\n  - name: one\n    model: test\n",
		"draft.md":    "A document",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := run([]string{"--config", "config.yaml", "--out", "runs", "draft.md"})
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete synthesis: %v", err)
	}
	paths, err := filepath.Glob("runs/*/summary.json")
	if err != nil || len(paths) != 1 {
		t.Fatalf("missing failure summary: %v %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Cost      CostSummary `json:"cost"`
		Synthesis bool        `json:"synthesis"`
		Error     string      `json:"synthesis_error"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Cost.Calls != 2 || summary.Cost.ReportedUSD != 0.02 || summary.Synthesis || summary.Error == "" {
		t.Fatalf("invalid failure summary: %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(paths[0]), "synthesis.md")); !os.IsNotExist(err) {
		t.Fatal("incomplete synthesis must not be published as finished")
	}
}
