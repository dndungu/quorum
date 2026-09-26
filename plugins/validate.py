#!/usr/bin/env python3
"""Check Quorum's host package metadata and shared workflow contract."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLUGINS = ROOT / "plugins"


def load_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def skill_body(path):
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        return text.lstrip()
    _, _, body = text.split("---\n", 2)
    return body.lstrip()


def main():
    codex = PLUGINS / "codex"
    claude = PLUGINS / "claude-code"
    cursor_command = PLUGINS / "cursor/.cursor/commands/quorum-review.md"

    codex_manifest = load_json(codex / "plugin.json")
    claude_manifest = load_json(claude / ".claude-plugin/plugin.json")
    codex_market = load_json(ROOT / ".agents/plugins/marketplace.json")
    claude_market = load_json(ROOT / ".claude-plugin/marketplace.json")

    if codex_manifest["name"] != "quorum-review" or codex_manifest["version"] != "0.1.0":
        raise ValueError("Codex plugin identity/version mismatch")
    if claude_manifest["name"] != "quorum-review":
        raise ValueError("Claude plugin identity mismatch")
    if codex_market["plugins"][0]["name"] != codex_manifest["name"]:
        raise ValueError("Codex marketplace entry does not match its plugin")
    if claude_market["plugins"][0]["name"] != claude_manifest["name"]:
        raise ValueError("Claude marketplace entry does not match its plugin")
    if not (ROOT / codex_market["plugins"][0]["source"]["path"].removeprefix("./")).is_dir():
        raise ValueError("Codex marketplace source path does not exist")
    if not (ROOT / claude_market["plugins"][0]["source"].removeprefix("./")).is_dir():
        raise ValueError("Claude marketplace source path does not exist")

    expected = skill_body(PLUGINS / "shared/review-workflow.md")
    bodies = [
        skill_body(codex / "skills/quorum-review/SKILL.md"),
        skill_body(claude / "skills/quorum-review/SKILL.md"),
        cursor_command.read_text(encoding="utf-8").lstrip(),
    ]
    if any(body != expected for body in bodies):
        raise ValueError("host instructions drifted from plugins/shared/review-workflow.md")
    if "## Before the review" not in expected or "## Run the review" not in expected or "## Communicate the result" not in expected:
        raise ValueError("workflow is missing a required contract section")
    print("Validated Codex, Claude Code, and Cursor Quorum packages.")


if __name__ == "__main__":
    main()
