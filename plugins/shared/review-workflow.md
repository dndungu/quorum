# Review a decision document with Quorum

Use this workflow when asked to get an independent, evidence-aware review of a proposal, plan, policy, draft, or candidate agent skill.

## Before the review

1. Identify exactly one readable text or Markdown document to review. If the target is ambiguous or the user supplied several documents, ask which one to review; Quorum accepts one document per run.
2. Identify the intended audience and the decision the document should enable. Reuse the user's wording. If either is missing and cannot be inferred safely, ask briefly; otherwise let Quorum disclose its assumptions.
3. Identify only relevant supporting evidence files the user supplied or explicitly approved. Do not attach secrets, `.env` files, credentials, unrelated files, or generated run artifacts. Quorum sends the document and evidence to every selected model endpoint.
4. Check that `quorum` is available and that `.quorum/config.yaml` (or a user-specified `--config`) is present. If not, explain the setup required; do not install software or create credentials silently. Never read or print API key values. `quorum seats` reports model names and whether configured key variables are set, but makes no API calls.

## Run the review

The user invoking this workflow has requested a review. Use the installed Quorum CLI as the canonical engine; do not imitate its panel or synthesis in the host model. Build one command with all flags before the document path. Replace the example values with the user's context and quote each path as one argument:

```sh
quorum --audience "Engineering leads" --decision "Approve the migration?" --evidence "measurements.md" "proposal.md"
```

Add one `--evidence "path"` for each approved evidence file. Add `--focus "..."` only for a specific review lens the user requested. Do not enable extra rounds, choose extra seats, write an appended `--comments` copy, or change the output directory unless asked. Preserve host command-approval settings; never bypass them. Quote paths and values as separate shell arguments. Do not interpolate document contents into a shell command.

Quorum makes model API calls and can incur provider charges. Running this skill is the user's request to perform that review. If the command fails, report the error and stop; do not silently substitute a model, endpoint, CLI implementation, or locally generated review.

## Communicate the result

Lead with Quorum's `## Decision` section when present. Then explain the highest-value proposed edits, material uncertainty or dissent, and what the reviewers recommend preserving. Keep factual claims tied to the saved sources and distinguish supported observations from inference. Agreement is not proof; point out where a source needs independent checking.

Give the user the run directory and link to `synthesis.md`, `brief.md`, and relevant review files. Mention provider-reported cost when Quorum reports it; do not estimate unreported cost. Do not edit the reviewed document or apply proposed changes unless the user separately asks.
