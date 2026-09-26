# Quorum

Quorum runs an independent panel of language models against a Markdown document, then asks a chair model to synthesize the reviews. It records each review, the synthesis, token usage, and API-reported cost in a timestamped run directory.

## Requirements

- Go 1.27 or newer
- One or more API keys for an OpenAI-compatible chat completions endpoint
- A YAML configuration file describing the reviewers and their models

## Quick start

```sh
git clone https://github.com/dndungu/quorum.git
cd quorum
cp .quorum/config.example.yaml .quorum/config.yaml
cp .env.example .env
```

Edit `.quorum/config.yaml` to use model identifiers supported by your API endpoint, then set the matching endpoint and API key in `.env`. Quorum loads `.env` from the current working directory and does not replace variables already present in the process environment. Keep your real `.env` and `.quorum/config.yaml` private; both are excluded by `.gitignore`.

Run a review:

```sh
go run . path/to/document.md
```

Or install the CLI:

```sh
go install github.com/dndungu/quorum@latest
quorum path/to/document.md
```

## Configuration

The example config uses shared endpoint and key environment variables. Each seat inherits these defaults; a seat may instead define its own `base_url`, `base_url_env`, or `api_key_env`.

```yaml
defaults:
  base_url_env: QUORUM_BASE_URL
  api_key_env: QUORUM_API_KEY

seats:
  - name: reviewer-one
    model: provider/model-id
  - name: reviewer-two
    model: provider/model-id
    enabled: false
```

`base_url` is the API base URL before `/chat/completions` (for example, `https://openrouter.ai/api/v1`). Use either `base_url` for a literal value or `base_url_env` for an environment variable. Put API keys in environment variables and refer to them by name with `api_key_env`. Model IDs depend on your endpoint and account; replace the sample values with supported IDs. Seats are enabled by default, and `enabled: false` excludes a seat unless it is explicitly named with `--seats`.

## CLI

```text
quorum [flags] [file.md]
```

Useful flags:

- `--seats name1,name2` selects reviewers; by default all enabled seats run.
- `--skip name1,name2` excludes reviewers.
- `--chair name` selects the reviewer that writes the synthesis; defaults to the first selected seat.
- `--rounds n` runs cross-examination after the first round when `n` is greater than 1.
- `--focus "text"` adds an instruction to the review brief.
- `--template file` supplies a custom response-format preamble.
- `--out dir` or `--output-dir dir` selects the base output directory. By default, runs are saved under `<input-file-directory>/.quorum/runs/`.
- `--no-synthesis` skips chair synthesis.
- `--comments` writes a `*.reviewed.md` copy beside the input document with council comments appended.
- `--config path` selects a YAML config file (default `.quorum/config.yaml`).
- `--dry-run` displays the brief and selected seats without calling APIs.
- `--timeout 5m` sets the timeout per seat; `--parallel 4` sets the maximum concurrent seats.

For example:

```sh
quorum --seats reviewer-one,reviewer-two --rounds 2 --focus "Check the migration plan" --out ./review-output path/to/document.md
```

Each run creates a timestamped directory containing `source.md`, `brief.md`, `reviews/`, `synthesis.md` (unless disabled), and `summary.json`. The summary includes provider-reported usage and cost when the endpoint returns those fields. Quorum does not estimate missing costs.

## Development

```sh
go test ./...
go run . --dry-run path/to/document.md
```

## License

Copyright 2026 David Ndungu. Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
