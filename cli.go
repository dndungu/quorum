package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	File        string
	Seats       string
	Skip        string
	Chair       string
	Rounds      int
	Focus       string
	Template    string
	Out         string
	NoSynthesis bool
	Comments    bool
	Config      string
	DryRun      bool
	Timeout     time.Duration
	Parallel    int
	Verbose     bool
}

func usage() {
	fmt.Fprint(os.Stderr, `quorum - convene a council of LLMs to review a markdown document

usage:
  quorum [flags] [file.md]

selection:
  --seats list          comma-separated reviewers to convene (default: all enabled)
  --skip list           reviewers to exclude
  --chair name          reviewer who synthesizes (default: first seat)

behavior:
  --rounds n            1 = single pass; 2+ = cross-examination rounds
  --focus "text"        extra instruction appended to the brief
  --template file       custom response-format preamble

output:
  --out dir             base output dir (default <input-dir>/.quorum/runs)
  --output-dir dir       alias for --out
  --no-synthesis        skip chair synthesis
  --comments            write source.reviewed.md with council comments appended

plumbing:
  --config path         config file (default .quorum/config.yaml)
  --dry-run             print brief and seats, call no APIs
  --timeout dur         per-seat timeout (default 5m)
  --parallel n          max concurrent seats (default 4)
  -v                    verbose
  seats                 list configured reviewers
`)
	os.Exit(2)
}

func parseArgs(args []string) (*Options, error) {
	fs := flag.NewFlagSet("quorum", flag.ContinueOnError)
	o := &Options{}
	fs.StringVar(&o.Seats, "seats", "", "reviewers to convene")
	fs.StringVar(&o.Skip, "skip", "", "reviewers to skip")
	fs.StringVar(&o.Chair, "chair", "", "synthesizing reviewer")
	fs.IntVar(&o.Rounds, "rounds", 1, "review rounds (2+ enables cross-exam)")
	fs.StringVar(&o.Focus, "focus", "", "extra review instruction")
	fs.StringVar(&o.Template, "template", "", "custom response format file")
	fs.StringVar(&o.Out, "out", "", "output base dir (default beside input file)")
	fs.StringVar(&o.Out, "output-dir", "", "output base dir (alias for --out)")
	fs.BoolVar(&o.NoSynthesis, "no-synthesis", false, "skip synthesis")
	fs.BoolVar(&o.Comments, "comments", false, "write source.reviewed.md")
	fs.StringVar(&o.Config, "config", ".quorum/config.yaml", "config file")
	fs.BoolVar(&o.DryRun, "dry-run", false, "no API calls")
	timeout := fs.String("timeout", "5m", "per-seat timeout")
	fs.IntVar(&o.Parallel, "parallel", 4, "max concurrent seats")
	fs.BoolVar(&o.Verbose, "v", false, "verbose")

	if len(args) > 0 && args[0] == "seats" {
		o.File = "seats-subcommand" // sentinel
		return o, nil
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	d, err := time.ParseDuration(*timeout)
	if err != nil {
		return nil, fmt.Errorf("bad --timeout: %w", err)
	}
	o.Timeout = d
	rest := fs.Args()
	switch len(rest) {
	case 0:
		o.File = "docs/plan.md"
	case 1:
		o.File = rest[0]
	default:
		return nil, errors.New("only one file allowed")
	}
	if o.Out == "" {
		o.Out = filepath.Join(filepath.Dir(o.File), ".quorum", "runs")
	}
	return o, nil
}

func run(args []string) error {
	if err := loadDotEnv(".env"); err != nil {
		return err
	}

	o, err := parseArgs(args)
	if err != nil {
		usage()
		return err
	}
	cfg, err := loadConfig(o.Config)
	if err != nil {
		return err
	}

	if o.File == "seats-subcommand" {
		return listSeats(cfg)
	}

	if o.Rounds < 1 {
		o.Rounds = 1
	}
	seats, err := selectSeats(cfg, o)
	if err != nil {
		return err
	}
	if len(seats) == 0 {
		return errors.New("no seats selected; check config and --seats/--skip")
	}

	source, err := os.ReadFile(o.File)
	if err != nil {
		return fmt.Errorf("read %s: %w", o.File, err)
	}

	runDir, err := newRunDir(o.Out)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "source.md"), source, 0o644); err != nil {
		return err
	}

	brief := buildBrief(string(source), o.Template, o.Focus)
	if err := os.WriteFile(filepath.Join(runDir, "brief.md"), []byte(brief), 0o644); err != nil {
		return err
	}

	if o.DryRun {
		fmt.Printf("== DRY RUN ==\nrun dir: %s\nbrief (%d words):\n\n%s\n\nseats:\n",
			runDir, countWords(brief), brief)
		for _, s := range seats {
			fmt.Printf("  %-10s model=%s key=%s\n", s.Name, s.Model, s.APIKeyEnv)
		}
		return nil
	}

	// Fan out round 1, gather.
	reviews, err := convene(seats, brief, 1, o)
	if err != nil {
		return err
	}
	for _, r := range reviews {
		if err := writeReview(runDir, r, 1); err != nil {
			return err
		}
	}

	// Cross-examination rounds: show each reviewer the anonymized peers.
	for round := 2; round <= o.Rounds; round++ {
		o.logf("round %d: cross-examining %d seats\n", round, len(reviews))
		reviews, err = crossExamine(seats, reviews, round, o)
		if err != nil {
			return err
		}
		for _, r := range reviews {
			if err := writeReview(runDir, r, round); err != nil {
				return err
			}
		}
	}

	// Chair synthesis.
	chair := seats[0]
	if o.Chair != "" {
		for _, s := range seats {
			if s.Name == o.Chair {
				chair = s
			}
		}
	}
	var synthesisResult LLMResult
	if !o.NoSynthesis {
		synthesisResult, err = synthesize(chair, reviews, brief, o)
		if err != nil {
			return fmt.Errorf("synthesis (%s): %w", chair.Name, err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "synthesis.md"), []byte(synthesisResult.Text), 0o644); err != nil {
			return err
		}
		fmt.Printf("synthesis written by %s -> %s\n", chair.Name, filepath.Join(runDir, "synthesis.md"))
	}

	if err := writeSummary(runDir, reviews, chair, o, synthesisResult.Usage, !o.NoSynthesis); err != nil {
		return err
	}

	if o.Comments {
		commented := buildCommentedDoc(string(source), reviews)
		out := strings.TrimSuffix(o.File, ".md") + ".reviewed.md"
		if err := os.WriteFile(out, []byte(commented), 0o644); err != nil {
			return err
		}
		fmt.Printf("comments written -> %s\n", out)
	}

	printSummary(reviews, chair, runDir, synthesisResult.Usage, !o.NoSynthesis)
	return nil
}

// selectSeats applies enabled/default + --seats + --skip.
func selectSeats(cfg *Config, o *Options) ([]Seat, error) {
	var want map[string]bool
	explicit := o.Seats != ""
	if explicit {
		want = map[string]bool{}
		for _, n := range splitList(o.Seats) {
			want[n] = true
		}
	}
	skip := map[string]bool{}
	for _, n := range splitList(o.Skip) {
		skip[n] = true
	}

	var out []Seat
	seen := map[string]bool{}
	for _, s := range cfg.Seats {
		if seen[s.Name] {
			return nil, fmt.Errorf("duplicate seat %q in config", s.Name)
		}
		seen[s.Name] = true
		include := s.Enabled
		if explicit {
			enabled := want[s.Name] // explicit --seats overrides enabled:false
			include = &enabled
		}
		if skip[s.Name] {
			disabled := false
			include = &disabled
		}
		if include != nil && *include {
			out = append(out, s)
		}
	}
	if explicit {
		for n := range want {
			if !seen[n] {
				return nil, fmt.Errorf("--seats: unknown reviewer %q (see: quorum seats)", n)
			}
		}
	}
	return out, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func listSeats(cfg *Config) error {
	fmt.Printf("%-12s %-28s %-10s %s\n", "SEAT", "MODEL", "ENABLED", "API KEY")
	for _, s := range cfg.Seats {
		key := "missing"
		if os.Getenv(s.APIKeyEnv) != "" {
			key = "ok"
		}
		fmt.Printf("%-12s %-28s %-10t %s (%s)\n", s.Name, s.Model, s.isEnabled(), key, s.APIKeyEnv)
	}
	return nil
}

func (o *Options) logf(format string, a ...any) {
	if o.Verbose {
		fmt.Fprintf(os.Stderr, "[quorum] "+format, a...)
	}
}
