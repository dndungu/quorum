# Quorum

**Turn independent model reviews into a clear decision and concrete edits.**

Quorum is a Go CLI for reviewing proposals, technical plans, and candidate agent
skills. Give it a document, tell it who needs to act, and optionally attach the
supporting evidence. A panel of language models reviews the material; a chair
synthesizes what to change, what remains uncertain, and what to keep.

You get the recommendation first, with the full reviews, source snapshots, token
usage, and provider-reported cost available behind it. Agreement is context—not
proof that a finding is correct.

```sh
quorum --audience "Engineering leads" \
  --decision "Approve this migration or run a pilot?" \
  --evidence measurements.md proposal.md
```

The default synthesis asks for five sections:

| Read in this order | What it answers |
|---|---|
| **Decision** | What should we do, and what qualification changes that answer? |
| **Next edits** | Which up to three concrete changes matter most? |
| **Uncertainty and dissent** | What remains disputed or unverified, and what would resolve it? |
| **Keep** | Which strong examples, ideas, and original voice should survive? |
| **Supporting detail** | What supports each finding, and which reviewers raised it? |

These are model instructions, not a guarantee of correct reasoning or citations.
Check consequential claims against the saved sources.

[Get started](#get-started) · [Examples](#choose-the-review-you-need) ·
[CLI reference](#cli-reference) · [Configuration](#configure-the-panel) ·
[Artifacts and costs](#inspect-the-result-and-its-cost) ·
[Agent plugins](#use-quorum-from-coding-agents) ·
[Failures and limits](#understand-failures-and-limits) · [Development](#development)

## Get started

You need Go 1.27 or newer, access to an OpenAI-compatible Chat Completions
endpoint, and model identifiers supported by that endpoint. A single reviewer
works; configure multiple seats for independent first-pass opinions.

```sh
git clone https://github.com/dndungu/quorum.git
cd quorum
cp .quorum/config.example.yaml .quorum/config.yaml
cp .env.example .env
```

Set the endpoint and key in `.env`:

```dotenv
QUORUM_BASE_URL=https://openrouter.ai/api/v1
QUORUM_API_KEY=replace-with-your-api-key
```

In `.quorum/config.yaml`, replace the sample model IDs with models available to
your account. Names such as `reviewer-one` identify seats; they are not model IDs.

```yaml
defaults:
  base_url_env: QUORUM_BASE_URL
  api_key_env: QUORUM_API_KEY

seats:
  - name: reviewer-one
    model: provider/first-model-id
  - name: reviewer-two
    model: provider/second-model-id
```

Preview the supplied synthetic example without calling a model:

```sh
go run . --dry-run \
  --audience "Agent skill authors" \
  --decision "Adopt this procedure or run a limited trial?" \
  --evidence testdata/communication/evidence.md \
  testdata/communication/draft.md
```

The preview prints the assembled brief and selected seats. It also writes
`source.md` and `brief.md` into a run directory. It resolves the configured base
URLs but does not require valid API keys or verify that model IDs exist.

Remove `--dry-run` to run the review. The terminal prints the chair's Decision
section when present, the run location, and reported cost. Open `synthesis.md`
for the full result.

Install the current checkout as a CLI:

```sh
go install .
quorum --audience "Engineering leads" proposal.md
```

Ensure Go's binary installation directory is on your `PATH`. To install the
version published on GitHub instead, use
`go install github.com/dndungu/quorum@latest`. That does not include unpublished
local changes. Installed binaries still look for `.env` and `.quorum/config.yaml`
in the current working directory; use `--config` for a different YAML file.

## Choose the review you need

### Check a proposal against its evidence

```sh
quorum --audience "Engineering leads" \
  --decision "Approve the migration?" \
  --focus "Check rollback assumptions and operational risk" \
  --evidence measurements.md --evidence incident-notes.txt \
  proposal.md
```

The audience describes who must understand the document. The decision describes
what it should enable. The focus adds a specific review instruction. If audience
or decision is omitted, reviewers are asked to infer cautiously and disclose
material assumptions.

### Let reviewers challenge one another

```sh
quorum --seats reviewer-one,reviewer-two \
  --chair reviewer-two --rounds 2 proposal.md
```

Round one is independent: every selected seat receives the same brief. In each
later round, every participating seat receives the original brief, its previous
review, and the other seats' previous reviews. It is asked to defend or revise
its findings using evidence. Peer labels are distinct and stable; the prose can
still reveal a reviewer or model's identity.

The chair then receives the brief and the latest available reviews, identified
by seat and model. Later-round reviews are influenced by their peers; they are
not additional independent votes. More rounds also resend the document and
reviews, increasing input-token usage.

### Review a distilled takeaway or candidate skill

```sh
quorum --audience "Agent skill authors" \
  --decision "Adopt this procedure or test it first?" \
  --evidence observations.md --evidence trial-results.txt \
  candidate-skill.md
```

The default instructions distinguish assertions, observed results, and inference.
For reusable procedures, they ask about triggers, steps, exceptions, source
support, and how effectiveness could be tested. Repeated appearances in a corpus
do not establish that a technique transfers to every setting.

Quorum can review the output of a transcript-distillation workflow. It does not
fetch videos, extract transcripts, install skills, or automatically integrate
with `/distill`. Those remain separate tools and workflows.

## Use Quorum from coding agents

Quorum provides native review workflows for Codex, Claude Code, and Cursor in
[`plugins/`](plugins/README.md). Each host invokes the same installed Quorum CLI;
the plugins do not duplicate the review engine or manage API credentials.

Quorum must already be installed and configured in the project. Invoking a plugin
review sends the selected document and any approved evidence to every selected
model endpoint and may incur provider charges. The plugin preserves each host's
command approval settings, does not attach `.env` or unrelated files, and leaves
the input document unchanged. Read the host-specific installation instructions
and artifact behavior before using it.

### Supply your own review instructions

Create `review-template.md`, for example:

```markdown
Review this migration plan for rollback readiness.
Return the blocking risks, concrete edits, and evidence needed to resolve them.
Cite document and evidence line references. State when support is missing.
```

```sh
quorum --template review-template.md \
  --evidence rollback-test.md migration.md
```

The file replaces the default reviewer instructions, including their requested
structure and word target. Audience, decision, evidence, and `--focus` still go
into the brief. The chair retains its standard synthesis instructions. Template
paths must exist and contain non-whitespace text.

### Keep individual reviews or append them to a document copy

```sh
quorum --no-synthesis --comments --out ./review-output proposal.md
```

This skips the chair call and writes `proposal.reviewed.md` beside the input.
The copy contains the original document followed by the latest review from each
participating seat under Council Comments. It is an appendix, not inline edits
or an automatically rewritten document. `--comments` also works with synthesis
enabled. An existing reviewed copy is overwritten.

## CLI reference

```text
quorum [flags] [file.md]
quorum seats
```

Place all flags **before** the document path. Exactly one document can be given;
there is no stdin or batch-input mode. The default path, when omitted, is
`docs/plan.md`. Markdown is the intended format, but the CLI reads plain text
without requiring a `.md` extension. It does not convert PDF, Word, or slide files.

### Select reviewers

| Option | Default | Behavior |
|---|---|---|
| `--seats name1,name2` | All enabled seats | Explicitly selects seats, including disabled ones. |
| `--skip name1,name2` | None | Excludes seats; takes precedence over `--seats`. Unknown skip names are ignored. |
| `--chair name` | First selected seat | Must name a selected reviewer. The chair also participates in reviews. |
| `seats` | — | Lists configured names, models, enabled status, and whether each key environment variable is set. No API calls. |

Selection preserves YAML order, not the order in `--seats`. Unknown explicit
seat names, duplicate configured names during selection, and an empty selection
are errors. The `seats` command currently uses the default configuration path;
arguments following `seats` are not parsed, including `--config`.

### Define the task

| Option | Default | Behavior |
|---|---|---|
| `--audience "text"` | Unspecified | Names the intended reader. |
| `--decision "text"` | Unspecified | Names the intended decision or action. |
| `--focus "text"` | None | Appends a special instruction to the brief. |
| `--evidence path` | None | Adds a supporting text file; repeat for more sources. |
| `--template path` | Built-in instructions | Replaces reviewer instructions with a file. |
| `--rounds n` | `1` | Total review rounds. Values below one are normalized to one. |
| `--no-synthesis` | `false` | Skips the chair call. |

### Control output and execution

| Option | Default | Behavior |
|---|---|---|
| `--out dir`, `--output-dir dir` | `<input-dir>/.quorum/runs` | Base directory for timestamped runs. |
| `--comments` | `false` | Writes a reviewed copy beside the input document. |
| `--config path` | `.quorum/config.yaml` | YAML configuration file. |
| `--dry-run` | `false` | Writes source/brief snapshots and prints the brief and seats; no API calls. |
| `--timeout duration` | `5m` | Positive timeout per HTTP request, not for the entire run. |
| `--parallel n` | `4` | Maximum concurrent review calls per round; must be positive. |
| `--max-tokens n` | `4096` | Positive output-token cap sent with each call, including the chair. |
| `-v` | `false` | Logs progress and reviewer failures to stderr. |

## Configure the panel

A **seat** is a named reviewer with a model, endpoint, and key environment
variable. Seats may share a provider or use different endpoints.

```yaml
defaults:
  base_url_env: QUORUM_BASE_URL
  api_key_env: QUORUM_API_KEY

seats:
  - name: primary
    model: provider/first-model-id

  - name: alternate
    model: another-model-id
    base_url: https://example.com/v1
    api_key_env: ALTERNATE_API_KEY
    enabled: false
```

The example endpoint and model IDs above are placeholders. Replace them before
making real calls.

| Field | Scope | Meaning |
|---|---|---|
| `name` | Seat | Required unique label. Used in selection, reports, and filenames. Use simple filename-safe names. |
| `model` | Seat | Required model identifier accepted by the endpoint. |
| `base_url` | Defaults or seat | Literal API base URL, before `/chat/completions`. |
| `base_url_env` | Defaults or seat | Environment variable containing the base URL. Must resolve to a nonempty value. |
| `api_key_env` | Defaults or seat | Environment variable containing the API key; its value is required when making a call. |
| `enabled` | Seat | Defaults to `true`; explicit `--seats` can include a disabled seat. |

A seat inherits the default endpoint only if it defines neither `base_url` nor
`base_url_env`. If both are present, `base_url_env` wins. A missing seat-level
`api_key_env` inherits the default. Configuration loading validates the required
name, model, and resolved endpoint for **every** seat, including disabled or
unselected seats.

The YAML parser also accepts `defaults.timeout`, `defaults.parallel`, and
`defaults.temperature`, but those values currently have **no effect**. Use the
CLI flags for timeout and parallelism. Requests currently use temperature `0`;
there is no temperature flag.

### Environment and endpoint behavior

Quorum loads `.env` from the current directory without replacing existing process
environment variables. A missing file is allowed. Supported forms include
`KEY=value`, optional `export`, single- or double-quoted values, blank lines,
and comments. Double-quoted escapes are decoded; shell expansion and command
substitution are not performed. Invalid entries fail the run.

The repository ignores `.env` and `.quorum/config.yaml`. Keep keys in environment
variables rather than YAML values. Source and evidence text are sent to each
selected endpoint; later rounds also send peer reviews, and the chair receives
the collected reviews.

Each request is a non-streaming `POST` to `<base_url>/chat/completions` with bearer
authentication, one user message, `model`, `temperature: 0`, `max_tokens`, and
`usage: {include: true}`. Quorum reads the first choice's text. Endpoint and model
support for these fields can vary; there is no provider-specific adapter,
automatic retry, or automatic model fallback.

## Follow a claim back to its source

The brief labels the document `D` and evidence files `E1`, `E2`, and so on, in
`--evidence` order. Each original line, including blank lines, gets a reference:

```text
[D:L12] The proposal's claim.
[E1:L7] The supporting observation or qualification.
```

The exact source document is saved separately as `source.md`. The assembled
`brief.md` snapshots the numbered document, numbered evidence, audience, decision,
and review instructions. Every round and the chair receive that same brief.
Evidence files are embedded there rather than copied into separate output files.

A reference means the text was supplied—not that it was independently verified.
Quorum does not open URLs, fetch citations, check line references automatically,
or establish that a source's assertions are true. With no evidence files, reviewers
are instructed to state verification limits. With long inputs, check that the
full brief and peer reviews fit your model's context window; automatic chunking
and input-token budgeting are not implemented.

## Inspect the result and its cost

A successful two-round run with two seats produces:

```text
<input-dir>/.quorum/runs/2026-09-26T12-00-00/
├── source.md
├── brief.md
├── reviews/
│   ├── reviewer-one.md
│   ├── reviewer-two.md
│   ├── reviewer-one-r2.md
│   └── reviewer-two-r2.md
├── synthesis.md
└── summary.json
```

Review files include seat, model, and round metadata. Failed attempts are marked;
failed later rounds can contain retained text from an earlier successful round.
The optional reviewed copy lives beside the input, even when `--out` points
elsewhere. A `.txt` input produces a filename such as `notes.txt.reviewed.md`.

Run names use local time with second precision. Runs sharing an output base and
starting in the same second can reuse a directory and overwrite artifacts. Use
distinct `--out` directories for concurrent runs. The root repository ignore rule
covers `.quorum/runs/`; choose appropriate ignore rules for outputs elsewhere.

### What summary.json records

| Field | Contents |
|---|---|
| `file`, `chair`, `rounds` | Input path, selected chair, and normalized requested round count. |
| `focus`, `audience`, `decision`, `evidence` | Supplied context and evidence paths; omitted when empty. |
| `max_tokens` | Per-call output cap. |
| `synthesis` | Whether synthesis completed successfully. |
| `synthesis_error` | Chair failure, when present. |
| `synthesis_usage` | Reported chair token counts and cost. |
| `seats` | Latest results for participating seats: seat, model, round, failure/error, elapsed seconds, and usage. |
| `review_calls` | Recorded review attempts across all rounds, including failures. |
| `cost` | Aggregate call counts, reported/unreported cost counts, reported USD, and token counts. |

Usage entries contain `prompt_tokens`, `completion_tokens`, `total_tokens`, and
optional `cost_usd`. Aggregate cost fields are `calls`, `reported_calls`,
`unreported_calls`, `reported_usd`, `prompt_tokens`, `completion_tokens`, and
`total_tokens`. Zero-valued per-call token fields can be omitted.

Quorum reads cost from the provider's `usage.cost` field. It does not estimate
missing prices or treat missing cost as zero. `reported_usd` is a partial total
when some calls have no reported cost. Recorded attempts can include local
failures before an HTTP request, such as a missing key; call counts are not a
billing ledger. HTTP/API failures may have no recoverable usage information.

With `N` reviewers and `R` rounds, a fully successful run makes `N × R + 1` calls,
or `N × R` with `--no-synthesis`. Two reviewers and two rounds make five calls.
Each call has its own output cap; this is not a total-dollar or input-token budget.

## Understand failures and limits

| Situation | Result |
|---|---|
| Invalid config, input, template, evidence, chair, or execution limit | Fails before model calls. Missing/whitespace-only templates and evidence are rejected. |
| Some seats fail in round one | Their attempts are recorded; successful seats continue. Failed first-round seats do not rejoin later. |
| All seats fail in round one | Failed review files and a summary are written; the command fails. |
| A later-round request fails | Previous text is retained, the new failure is recorded, and old usage is not counted again. The seat can be tried in a subsequent round. |
| API reports `finish_reason: length` | Partial text is rejected as incomplete; reported usage is retained. Raise `--max-tokens` if appropriate. |
| API returns empty content | The call fails. |
| Chair fails | A summary records its error and available usage; no completed synthesis is written for that attempt, and the command fails. |
| Filesystem write fails | The command fails; already-written artifacts may remain and a summary may be absent. |

Ordinary run failures exit with status `1`; argument parsing/validation errors
exit with status `2`. Successful runs, including runs with some reviewer failures,
exit `0`. Help currently also exits `2`. For automation, inspect the summary's
failure fields as well as the process exit status.

The default reviewer prompt requests a verdict, up to three priority changes,
open questions, and content to keep, targeting 500 words. The chair targets 600
words. Those targets, reviewer attribution, citation accuracy, and preservation
of minority findings depend on model compliance. They are not mechanically
enforced. Models can emit verbose preambles or exceed the word targets even
within the output-token cap. Prompt instructions to treat source text as untrusted
are not a security boundary.

The source document is not edited in place. Quorum does not apply suggested
changes, prove factual correctness, guarantee independent model reasoning, or
validate a skill's effectiveness through execution. It supplies a review record
for the author to assess.

## What the communication work added

The current implementation adds explicit audience/decision context, attached
line-referenced evidence, decision-first synthesis, and a terminal decision
preview. Default prompts prioritize consequential supported findings, concrete
edits, uncertainty, and content worth preserving. Supporting detail uses short
bullets instead of large padded consensus tables.

Cross-examination now retains the original brief and uses stable distinct peer
labels. Usage accounting covers all recorded rounds, preserves reported usage
on truncation and synthesis failure, and avoids counting retained reviews twice.
Execution validation and a configurable output-token cap make failure behavior
explicit.

These changes were informed by five communication talks and tested with local
HTTP fixtures, race checks, and inexpensive live model calls. The live checks
found the planted issues in a synthetic skill proposal, but also exposed citation,
attribution, and output-length limitations. They do not establish that the new
prompts outperform the old ones on real documents. The next evaluation is a
matched comparison on real proposals, plans, and candidate skills.

See [research, sources, costs, and validation](docs/research/communication.md)
and the [extraction lens](docs/research/communication-lens.md).

## Development

Tests use local HTTP servers and require no real API keys or paid model calls.

```sh
go test ./...
go test -race ./...
go vet ./...
```

| File | Responsibility |
|---|---|
| `main.go` | Process entry and error exit. |
| `cli.go` | Flags, seat selection, workflow, and artifact orchestration. |
| `prompts.go` | Review/synthesis instructions and numbered evidence snapshots. |
| `convene.go` | Concurrent reviews, cross-examination, synthesis, reports, and accounting. |
| `llm.go` | Chat Completions requests, response validation, and usage parsing. |
| `config.go`, `dotenv.go` | YAML configuration and environment loading. |
| `run.go` | Run-directory creation and file helpers. |
| `cli_test.go`, `review_workflow_test.go` | CLI behavior, source propagation, failure handling, accounting, and token-cap tests. |
| `testdata/communication/` | Synthetic proposal and evidence for repeatable smoke checks. |

## License

Copyright 2026 David Ndungu. Licensed under the Apache License, Version 2.0.
See [LICENSE](LICENSE).
