# Quorum for coding agents

Each package in this directory provides a native entry point into the installed `quorum` CLI. The CLI remains the canonical review engine and owns configuration, model endpoints, API credentials, source snapshots, review rounds, synthesis, and cost reporting. The host packages contain instructions only: they do not bundle binaries, make provider calls themselves, install credentials, or add background hooks.

## Requirements and behavior

- Install Quorum and configure `.quorum/config.yaml` and the environment variables it names. See the repository's [setup guide](../README.md#get-started).
- Provide one text or Markdown file. Quorum currently reviews one file per invocation.
- Invoking `/quorum-review` asks the CLI to contact the configured model endpoints. These calls can incur provider charges.
- The agent may include supporting files only when the user supplied or approved them. Quorum sends every selected source to each selected endpoint.
- The skill does not enable extra rounds or append a reviewed copy by default. It links the saved output and leaves source edits to a separate user request.

## Install in Codex

1. Add this GitHub repository as a plugin marketplace: `codex plugin marketplace add dndungu/quorum`.
2. Install the plugin: `codex plugin add quorum-review@quorum`.
3. Start a new session and invoke `$quorum-review` with a document and the decision you need to make.

For a local checkout, the root [`plugin.json`](codex/plugin.json) and [`marketplace.json`](../.agents/plugins/marketplace.json) expose the Codex package. Install/testing is subject to current Codex local-marketplace support; the package does not auto-enable itself.

## Install in Claude Code

```sh
claude plugin marketplace add dndungu/quorum
claude plugin install quorum-review@quorum
```

Restart or reload plugins, then invoke `/quorum-review:quorum-review` with the document and decision. During local development, load just the plugin with `claude --plugin-dir ./plugins/claude-code`.

## Install in Cursor

Cursor project commands are Markdown files. Copy the command into the project you want to use:

```sh
mkdir -p .cursor/commands
cp /path/to/quorum/plugins/cursor/.cursor/commands/quorum-review.md .cursor/commands/
```

Open or reload the project, then invoke `/quorum-review` in Cursor chat.

## Verify package files

Run `python3 plugins/validate.py` from the repository root to check manifests, marketplace paths, command files, and that all three hosts carry the same workflow instructions. This checks repository packaging; an actual host installation and review are still required before claiming consumer verification.
