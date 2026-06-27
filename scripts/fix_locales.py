#!/usr/bin/env python3
"""Repair UTF-8 mojibake in locale JSON files.

Fixes text that was UTF-8 decoded as Windows-1252/Latin-1 and saved again
(e.g. "Sveiki atvykÄ™" -> "Sveiki atvykę", "ÐžÐ±..." -> "Об...").
"""

from __future__ import annotations

import json
import pathlib
import sys

# Unicode characters produced when UTF-8 bytes are read as Windows-1252.
CP1252_UNICODE_TO_BYTE: dict[int, int] = {
    0x20AC: 0x80,
    0x201A: 0x82,
    0x0192: 0x83,
    0x201E: 0x84,
    0x2026: 0x85,
    0x2020: 0x86,
    0x2021: 0x87,
    0x02C6: 0x88,
    0x2030: 0x89,
    0x0160: 0x8A,
    0x2039: 0x8B,
    0x0152: 0x8C,
    0x017D: 0x8E,
    0x2018: 0x91,
    0x2019: 0x92,
    0x201C: 0x93,
    0x201D: 0x94,
    0x2022: 0x95,
    0x2013: 0x96,
    0x2014: 0x97,
    0x02DC: 0x98,
    0x2122: 0x99,
    0x0161: 0x9A,
    0x203A: 0x9B,
    0x0153: 0x9C,
    0x017E: 0x9E,
    0x0178: 0x9F,
}


def to_cp1252_bytes(text: str) -> bytes:
    out = bytearray()
    for ch in text:
        code = ord(ch)
        if code <= 0xFF:
            out.append(code)
            continue
        mapped = CP1252_UNICODE_TO_BYTE.get(code)
        if mapped is None:
            raise ValueError(code)
        out.append(mapped)
    return bytes(out)


def fix_mojibake(text: str) -> str:
    if not text:
        return text

    current = text
    for _ in range(6):
        try:
            raw = to_cp1252_bytes(current)
        except ValueError:
            return current
        try:
            decoded = raw.decode("utf-8")
        except UnicodeDecodeError:
            return current
        if decoded == current:
            return current
        current = decoded
    return current


def fix_file(path: pathlib.Path) -> int:
    with path.open("r", encoding="utf-8") as handle:
        data = json.load(handle)

    changed = 0
    fixed: dict[str, str] = {}
    for key, value in data.items():
        new_value = fix_mojibake(value)
        if new_value != value:
            changed += 1
        fixed[key] = new_value

    with path.open("w", encoding="utf-8", newline="\n") as handle:
        json.dump(fixed, handle, ensure_ascii=False, indent=2)
        handle.write("\n")

    return changed


def main() -> int:
    root = pathlib.Path(__file__).resolve().parents[1] / "server" / "internal" / "webui" / "locales"
    for path in sorted(root.glob("*.json")):
        if path.name == "en.json":
            continue
        count = fix_file(path)
        print(f"{path.name}: {count} values fixed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
