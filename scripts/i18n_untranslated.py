#!/usr/bin/env python3
"""Report i18n values that still hold the source language instead of a translation.

Key parity is already enforced by the test suite
(web/i18n/message-integrity.test.mjs and
internal/pkg/i18nx/locale_parity_test.go), so this exists for the gap those
tests cannot see: a key can be present in every locale and still carry the
Chinese string, which is how Chinese kept showing up in an otherwise translated
UI.

Usage:
    python scripts/i18n_untranslated.py <base> <child>

<base> is the source language, <child> the one being audited. Values in <child>
containing CJK are listed. Those byte-identical to <base> are reported as
untranslated; the rest are left to judgement, since product names and sample
data legitimately keep CJK.

Exit code: 0 when the child has no untranslated values, 1 when it does,
2 on usage errors.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

# Hangul and Kana are deliberately excluded: this looks for Chinese script,
# which is what leaked into the UI, not for non-ASCII text in general.
CJK = re.compile(r"[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]")

USAGE = "Usage: python scripts/i18n_untranslated.py <base> <child>\n"


def flatten_json(value: Any, prefix: str = "", out: dict[str, Any] | None = None) -> dict[str, Any]:
    if out is None:
        out = {}
    if isinstance(value, dict):
        for key, child in value.items():
            flatten_json(child, f"{prefix}.{key}" if prefix else str(key), out)
        return out
    out[prefix] = value
    return out


def _indent_of(line: str) -> int:
    return len(line) - len(line.lstrip())


def parse_flat_yaml(text: str) -> dict[str, Any]:
    """Parse the locale YAML dialect: flat top-level keys, one nested level, block scalars."""
    lines = text.splitlines()
    out: dict[str, Any] = {}
    index = 0

    while index < len(lines):
        raw = lines[index]
        index += 1

        if not raw.strip() or raw.lstrip().startswith("#"):
            continue

        indent = _indent_of(raw)
        if indent != 0:
            continue

        head = raw.strip()
        if ":" not in head:
            continue
        key, _, rest = head.partition(":")
        key = key.strip().strip("\"'")
        value = rest.strip()

        if value in ("|", ">", "|-", ">-", "|+", ">+"):
            block: list[str] = []
            block_indent: int | None = None
            while index < len(lines):
                line = lines[index]
                if line.strip() and _indent_of(line) <= indent:
                    break
                line_indent = _indent_of(line)
                block_indent = line_indent if block_indent is None else block_indent
                block.append(line[min(block_indent, line_indent):])
                index += 1
            out[key] = "\n".join(block).strip()
        elif not value:
            # A key with no inline value opens one nested level.
            while index < len(lines):
                line = lines[index]
                if line.strip() and _indent_of(line) <= indent:
                    break
                stripped = line.strip()
                index += 1
                if ":" in stripped:
                    nested_key, _, nested_value = stripped.partition(":")
                    out[f"{key}.{nested_key.strip()}"] = nested_value.strip().strip("\"'")
            continue
        else:
            out[key] = value.strip().strip("\"'")

    return out


def load_values(file: str) -> dict[str, Any]:
    suffix = Path(file).suffix.lower()
    # utf-8-sig tolerates a BOM and a plain utf-8 file equally; a BOM is common
    # in locale files authored on Windows and would otherwise abort the run.
    text = Path(file).read_text(encoding="utf-8-sig")
    if suffix == ".json":
        return flatten_json(json.loads(text))
    if suffix in (".yml", ".yaml"):
        return parse_flat_yaml(text)
    raise ValueError(f"Unsupported file format: {file}")


def main(argv: list[str]) -> int:
    args = [arg for arg in argv if not arg.startswith("-")]
    if len(args) != 2:
        sys.stderr.write(USAGE)
        return 2

    base_file, child_file = args
    try:
        base = load_values(base_file)
        child = load_values(child_file)
    except (OSError, ValueError, json.JSONDecodeError) as error:
        sys.stderr.write(f"Failed to read bundle: {error}\n")
        return 2

    untranslated: list[tuple[str, str]] = []
    review: list[tuple[str, str]] = []
    for key, value in child.items():
        if not isinstance(value, str) or not CJK.search(value):
            continue
        base_value = base.get(key)
        if isinstance(base_value, str) and base_value.strip() == value.strip():
            untranslated.append((key, value))
        else:
            review.append((key, value))

    print(
        f"child {child_file}: {len(child)} values, "
        f"{len(untranslated) + len(review)} with CJK "
        f"({len(untranslated)} untranslated, {len(review)} to review)"
    )

    if untranslated:
        print(f"\nuntranslated, identical to base ({len(untranslated)}):")
        for key, value in untranslated:
            print(f"  - {key} = {value}")

    if review:
        print(f"\nCJK but differs from base ({len(review)}):")
        for key, value in review:
            print(f"  ? {key} = {value}")

    if not untranslated and not review:
        print("OK: no CJK left in child values")
        return 0
    return 1


if __name__ == "__main__":
    # Do not litter the repo with __pycache__ when this is run from tooling.
    sys.dont_write_bytecode = True
    raise SystemExit(main(sys.argv[1:]))