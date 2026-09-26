# Communication research → Quorum changes

Date: 2026-09-26. Decision: make the default review help an identified reader act,
with specific edits and traceable support. Preserve independent reviews as the
underlying record. This is an implemented design choice, not evidence that the
new prompts improve all review tasks.

## Sources and selection

The user's Gamma talk in `tmp/talk.txt` motivated the question. Five additional
YouTube talks were selected for relevance, substantial reach, and coverage of
explanation, attention, and evidence. This is a curated shortlist, not an
exhaustive or statistically ranked list of the best videos. View counts establish
reach, not truth or effectiveness.

| Talk | Channel | YouTube views at retrieval | Useful contribution |
|---|---|---:|---|
| [How to Speak — Patrick Winston](https://www.youtube.com/watch?v=Unzc731iCUY) | MIT OpenCourseWare | 23,189,309 | Start with value; use recognizable structure; end on contributions. |
| [How to avoid death By PowerPoint — David JP Phillips](https://www.youtube.com/watch?v=Iwpi1Lm6dFo) | TEDx Talks | 5,895,437 | Give one focal message appropriate prominence. |
| [The best stats you've ever seen — Hans Rosling](https://www.youtube.com/watch?v=hVimVzgtD6w) | TED | 4,394,750 | Use relevant comparisons; aggregates can obscure consequential differences. |
| [Talk nerdy to me — Melissa Marshall](https://www.youtube.com/watch?v=y66YKWz_sf0) | TED | 619,681 | Explain relevance; clarify unfamiliar terms without sacrificing precision. |
| [Storytelling with Data — Cole Nussbaumer Knaflic](https://www.youtube.com/watch?v=8EMW7io4rSI) | Talks at Google | 461,582 | Separate exploration from explanation; focus attention on the actionable finding. |

Discovery used web search after Composio's YouTube search returned a daily quota
error. Composio video-details calls supplied titles, channels, and view counts.
Captions were fetched with the existing `yt-transcript` skill's `yt-dlp` workflow
and cleaned with its `vtt_to_text.py`. The cleaner globally deduplicates identical
caption lines, so these transcripts cannot establish repetition or pacing.
The timestamped captions are retained in the research archive for checking.

Extraction used the installed `/distill` extractor with
[the communication lens](communication-lens.md). Its original first-choice model
was no longer in the live catalog. A free-model attempt did not finish promptly
and was stopped. A small adapter supplied current inexpensive models, bounded
output tokens, and saved usage receipts; it did not reimplement corpus handling.
Five completed extractions used `google/gemini-2.5-flash-lite`; Qwen was configured
as fallback. Cost reported by the five completed responses: **$0.00502210**.
The abandoned free attempt supplied no receipt and is excluded from this total.

Results: **5 files; 0 empty extractions; 25 ideas; 21 passed applicability >= 3**.
This targeted corpus returning ideas for every talk is not a general success-rate
claim. A normalized substring check located 24 of 25 model-generated anchors.
Winston's “simplify slides” locator was inaccurate. Some anchors also exceeded
the requested length; generated notes are provisional, not verified quotations.
Selected mechanisms were checked against the transcripts before adoption.

## Adopted and declined

| Design choice | Source basis | Implementation |
|---|---|---|
| Explicit audience and intended action | Marshall's relevance question; Knaflic's explanatory purpose | `--audience`, `--decision`; record context in brief and summary. |
| Decision before supporting detail | Winston's opening value and final contribution; Knaflic's selective emphasis | Chair outputs Decision → Next edits → Uncertainty and dissent → Keep → Supporting detail. |
| Focused, concrete changes | Phillips's focal-message advice, adapted cautiously to text; Marshall's concrete explanations | Up to three edits with location, proposed wording/action, consequence, and uncertainty. Zero edits is valid. |
| Preserve precision and exceptions | Marshall's distinction between accessibility and oversimplification; Rosling's disaggregated examples | Prompts preserve qualifications and consequential minority findings; no vote-count ranking. |
| Inspectable evidence | Our engineering application of the talks' examples and the podcast pipeline's provenance failures | `--evidence` snapshots, stable `D:L#` / `E#:L#` references, original brief in every round. |
| Concise explanation with an audit trail | Knaflic's exploration/explanation distinction | 500/600-word prompt targets while retaining full review artifacts. |

Declined: a universal six-item ceiling, banning bullet lists, mandatory suspense,
color prescriptions, animated charts of review agreement, or treating popularity
as validation. A transcript cannot establish that a slide's visual design worked.
The talks do not prove that source IDs, three edits, or multi-model review improve
outcomes; these are explicit product hypotheses. No skill was automatically
installed from these assertions.

Cross-examination fixes necessary to make the evidence workflow usable: keep the
original document and sources, give peers distinct stable labels, record failed
rounds while retaining their last useful text, and total usage across all rounds.
Invalid concurrency/timeouts, unknown selected chairs, and missing/empty supporting
inputs fail before model calls. Custom reviewer templates remain supported.

## Validation

- Existing tests passed before changes.
- Isolated baseline regressions reproduced acceptance of zero concurrency and
  silent reuse of old usage when cross-examination failed.
- New tests exercise source/context propagation through real HTTP requests to a
  local test server, distinct labels, run artifacts, all-round accounting, and
  failed-round fallback. `go test -race ./...` and `go vet ./...` passed.
- A live two-model, two-round test used Gemini Flash-Lite and Qwen 3.5 Flash on
  [a synthetic skill proposal](../../testdata/communication/draft.md) and
  [its evidence](../../testdata/communication/evidence.md). All five calls reported
  cost: **$0.00331167**, 14,292 prompt tokens and 6,743 completion tokens.
- Its 575-word synthesis rejected immediate adoption, distinguished frequency
  from effectiveness, recognized the single-show limitation, and identified an
  implementation rule unsupported by the trial.
- Limitations observed: Qwen emitted a long planning preamble; the chair's table
  under-attributed shared findings to Qwen and one document reference was too
  loose (the accompanying evidence reference supported the point). Output
  instructions were tightened afterward. This is a smoke test, not a blinded
  comparative evaluation or proof of citation reliability. Prompt length targets
  and reference correctness are not mechanically enforced.
- A follow-up check exposed pathological table padding (19,107 completion tokens
  across three calls, $0.007946745). This directly motivated two further changes:
  supporting details now use short bullets, and `--max-tokens` defaults to 4096
  per call. API-reported truncation is rejected as incomplete while retaining
  usage, including a failed synthesis summary. No partial report is published
  as a successful synthesis. A token cap bounds output, not factual error.
- The bounded follow-up completed all three calls for **$0.002084705** and produced
  5,507 bytes of synthesis, with shared findings attributed to both reviewers.
  It still used 796 words despite the 600-word target, and Qwen still included a
  planning preamble in its raw review. The terminal therefore leads with the
  chair's Decision; individual reviews remain in their files. Total reported
  cost of extraction and all three live checks: **$0.01836522** across 16 completed
  calls. Final race tests and vet passed after adding the cap and failure tests.

## Reproducible artifacts

Raw captions, cleaned transcripts, hashes, metadata, extraction JSON, rank output,
anchor checks, adapter, and receipts live in the sibling podcasts repository:
`research/quorum-communication-2026-09-26/`. Wisdom contains the curated takeaways
and source record; raw material and scripts stay out of wisdom.

To repeat extraction, set `OPENROUTER_API_KEY` and run that archive's
`run_distill.py --corpus <archive>/transcripts --lens <archive>/lens.md --out <new-notes>`.
It uses the installed distill skill and skips existing results, so use a new output
directory for an actual rerun. Prices/model availability can change.
Live review artifacts remain locally in `tmp/quorum-research/evaluation/`.
