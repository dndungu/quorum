package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Review struct {
	Seat    Seat
	Model   string
	Round   int
	Text    string
	Err     string
	Seconds float64
	Usage   LLMUsage
}

func convene(seats []Seat, brief string, round int, o *Options) ([]Review, error) {
	sem := make(chan struct{}, o.Parallel)
	var mu sync.Mutex
	reviews := make([]Review, len(seats))
	var wg sync.WaitGroup
	for i, s := range seats {
		wg.Add(1)
		go func(i int, s Seat) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			o.logf("asking %s (model %s), round %d\n", s.Name, s.Model, round)
			result, err := callLLM(s, brief, o.Timeout, cfgTemp(o))
			r := Review{Seat: s, Model: s.Model, Round: round, Text: result.Text, Usage: result.Usage, Seconds: 0}
			if err != nil {
				r.Err = err.Error()
				o.logf("%s failed: %v\n", s.Name, err)
			}
			mu.Lock()
			reviews[i] = r
			mu.Unlock()
		}(i, s)
	}
	wg.Wait()
	var kept []Review
	for _, r := range reviews {
		if r.Err == "" {
			kept = append(kept, r)
		} else {
			fmt.Printf("  ABSTAINED %-10s %s\n", r.Seat.Name, r.Err)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("all seats failed")
	}
	return kept, nil
}

// crossExamine shows each reviewer its peers' anonymized round-1 reviews
// and asks it to defend or concede.
func crossExamine(seats []Seat, reviews []Review, round int, o *Options) ([]Review, error) {
	byName := map[string]Review{}
	for _, r := range reviews {
		byName[r.Seat.Name] = r
	}
	// Anonymized peer digest.
	var b strings.Builder
	for _, r := range reviews {
		if r.Seat.Name == "" {
			continue
		}
		_ = r
		break
	}
	_ = b
	prompts := make(map[string]string, len(seats))
	for _, r := range reviews {
		var peers strings.Builder
		for _, p := range reviews {
			if p.Seat.Name == r.Seat.Name {
				continue
			}
			peers.WriteString(fmt.Sprintf("\n--- Reviewer %s ---\n%s\n", anonName(p.Seat.Name), p.Text))
		}
		prompts[r.Seat.Name] = fmt.Sprintf(
			"Below are the anonymized reviews of the same document by other expert reviewers. "+
				"Where they disagree with you, either defend your position with evidence or concede and revise it. "+
				"Where they raise points you missed, incorporate the good ones. "+
				"Output your FULL revised review in the same required structure.\n%s", peers.String())
	}
	var mu sync.Mutex
	var next []Review
	sem := make(chan struct{}, o.Parallel)
	var wg sync.WaitGroup
	start := time.Now()
	for _, s := range seats {
		r, ok := byName[s.Name]
		if !ok {
			continue // abstained earlier rounds stay abstained
		}
		wg.Add(1)
		go func(s Seat, r Review) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, err := callLLM(s, r.Text+"\n\n"+prompts[s.Name], o.Timeout, cfgTemp(o))
			nr := Review{Seat: s, Model: s.Model, Round: round, Text: result.Text, Usage: result.Usage}
			if err != nil {
				nr.Err = err.Error()
				nr.Text = r.Text // fall back to previous round
			}
			mu.Lock()
			if nr.Err == "" {
				next = append(next, nr)
			} else {
				next = append(next, r)
			}
			mu.Unlock()
		}(s, r)
	}
	wg.Wait()
	_ = start
	sort.Slice(next, func(i, j int) bool { return next[i].Seat.Name < next[j].Seat.Name })
	return next, nil
}

var anonLetters = "ABCDEFGH"

func anonName(seat string) string {
	for i, c := range strings.Split(seat, "") {
		_ = c
		if i < len(anonLetters) {
			return "Reviewer " + string(anonLetters[i])
		}
	}
	return "Reviewer X"
}

func synthesize(chair Seat, reviews []Review, brief string, o *Options) (LLMResult, error) {
	var b strings.Builder
	b.WriteString("You are the CHAIR of a review council. Below is the document under review, ")
	b.WriteString("followed by every reviewer's response verbatim.\n\n")
	b.WriteString("=== DOCUMENT UNDER REVIEW ===\n" + brief + "\n\n")
	b.WriteString("=== COUNCIL REVIEWS ===\n")
	for _, r := range reviews {
		fmt.Fprintf(&b, "\n--- %s (%s) ---\n%s\n", r.Seat.Name, r.Model, r.Text)
	}
	b.WriteString(`
As chair, write a synthesis in exactly this structure:
1. CONSENSUS MATRIX: one row per distinct issue, columns per seat: RAISED / AGREED / SILENT / DISPUTED.
2. RANKED FINDINGS: order by (seats raising it) x severity. Quote the strongest articulation. Discard generic filler ("add more tests") and say why.
3. REJECTED FEEDBACK: points declined, each with a one-line reason.
4. STRONGEST-PART CONSENSUS: what multiple reviewers said to keep.
5. TOP 3 ACTIONS: the concrete edits with the highest expected value.
Be the chair, not a stenographer.`)
	return callLLM(chair, b.String(), o.Timeout, cfgTemp(o))
}

const defaultTemplate = `You are one of several independent expert reviewers. Critique the
document above. Be direct and specific; do not flatter. Respond in
exactly this structure:
1. VERDICT: one sentence overall assessment.
2. TOP RISKS: the 3-5 most serious flaws or blind spots, ranked.
3. CHALLENGES: assumptions you would push back on, with reasoning.
4. CONCRETE CHANGES: specific edits you would make.
5. MISSING: anything absent that should exist.
6. STRONGEST PART: what should NOT change.`

func buildBrief(source, template, focus string) string {
	if template == "" {
		template = defaultTemplate
	} else if data, err := os.ReadFile(template); err == nil {
		template = string(data)
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(source))
	b.WriteString("\n\n---\n\n")
	b.WriteString(template)
	if focus != "" {
		b.WriteString("\n\nSPECIAL FOCUS: " + focus)
	}
	return b.String()
}

func buildCommentedDoc(source string, reviews []Review) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(source, "\n"))
	b.WriteString("\n\n---\n\n# Council Comments\n\n")
	for _, r := range reviews {
		fmt.Fprintf(&b, "## Comment: %s (%s, round %d)\n\n%s\n\n", r.Seat.Name, r.Model, r.Round, strings.TrimSpace(r.Text))
	}
	return b.String()
}

func writeReview(runDir string, r Review, round int) error {
	dir := filepath.Join(runDir, "reviews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := r.Seat.Name
	if round > 1 {
		name = fmt.Sprintf("%s-r%d", r.Seat.Name, round)
	}
	header := fmt.Sprintf("---\nseat: %s\nmodel: %s\nround: %d\n---\n\n", r.Seat.Name, r.Model, round)
	return os.WriteFile(filepath.Join(dir, name+".md"), []byte(header+r.Text), 0o644)
}

type CostSummary struct {
	Calls            int     `json:"calls"`
	ReportedCalls    int     `json:"reported_calls"`
	UnreportedCalls  int     `json:"unreported_calls"`
	ReportedUSD      float64 `json:"reported_usd"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
}

func summarizeCost(reviews []Review, synthesis LLMUsage, includeSynthesis bool) CostSummary {
	summary := CostSummary{Calls: len(reviews)}
	if includeSynthesis {
		summary.Calls++
	}
	add := func(usage LLMUsage) {
		summary.PromptTokens += usage.PromptTokens
		summary.CompletionTokens += usage.CompletionTokens
		summary.TotalTokens += usage.TotalTokens
		if usage.CostUSD == nil {
			summary.UnreportedCalls++
			return
		}
		summary.ReportedCalls++
		summary.ReportedUSD += *usage.CostUSD
	}
	for _, review := range reviews {
		add(review.Usage)
	}
	if includeSynthesis {
		add(synthesis)
	}
	return summary
}

func writeSummary(runDir string, reviews []Review, chair Seat, o *Options, synthesis LLMUsage, includeSynthesis bool) error {
	type seatSum struct {
		Seat    string   `json:"seat"`
		Model   string   `json:"model"`
		Round   int      `json:"round"`
		Failed  bool     `json:"failed"`
		Error   string   `json:"error,omitempty"`
		Seconds float64  `json:"seconds"`
		Usage   LLMUsage `json:"usage"`
	}
	type summary struct {
		File           string      `json:"file"`
		Chair          string      `json:"chair"`
		Rounds         int         `json:"rounds"`
		Focus          string      `json:"focus,omitempty"`
		Synthesis      bool        `json:"synthesis"`
		SynthesisUsage LLMUsage    `json:"synthesis_usage,omitempty"`
		Cost           CostSummary `json:"cost"`
		Seats          []seatSum   `json:"seats"`
	}
	s := summary{File: o.File, Chair: chair.Name, Rounds: o.Rounds, Focus: o.Focus, Synthesis: includeSynthesis, SynthesisUsage: synthesis, Cost: summarizeCost(reviews, synthesis, includeSynthesis)}
	for _, r := range reviews {
		s.Seats = append(s.Seats, seatSum{
			Seat:    r.Seat.Name,
			Model:   r.Model,
			Round:   r.Round,
			Failed:  r.Err != "",
			Error:   r.Err,
			Seconds: r.Seconds,
			Usage:   r.Usage,
		})
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "summary.json"), append(data, '\n'), 0o644)
}

func printSummary(reviews []Review, chair Seat, runDir string, synthesis LLMUsage, includeSynthesis bool) {
	fmt.Printf("\n╔══════════════════════════════════════╗\n")
	fmt.Printf("║  COUNCIL VERDICTS                    ║\n")
	fmt.Printf("╚══════════════════════════════════════╝\n")
	for _, r := range reviews {
		verdict := firstLine(r.Text)
		if len(verdict) > 90 {
			verdict = verdict[:90] + "…"
		}
		fmt.Printf("  %-10s %s\n", r.Seat.Name, verdict)
	}
	fmt.Printf("\nchair: %s | run: %s\n", chair.Name, runDir)
	cost := summarizeCost(reviews, synthesis, includeSynthesis)
	if cost.UnreportedCalls == 0 && cost.Calls > 0 {
		fmt.Printf("API-reported cost: $%.8f USD across %d calls (%d prompt, %d completion tokens)\n", cost.ReportedUSD, cost.Calls, cost.PromptTokens, cost.CompletionTokens)
	} else if cost.ReportedCalls > 0 {
		fmt.Printf("API-reported cost: $%.8f USD across %d of %d calls; cost unavailable for %d calls\n", cost.ReportedUSD, cost.ReportedCalls, cost.Calls, cost.UnreportedCalls)
	} else {
		fmt.Printf("API-reported cost: unavailable for %d calls\n", cost.Calls)
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func cfgTemp(o *Options) float64 {
	return 0 // wired through Config.Defaults in loadConfig caller; kept simple
}
