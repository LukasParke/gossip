#!/usr/bin/env python3
"""
Normalize Go benchmark output so alert comparisons are polarity-safe.

- Keeps lower-is-better metrics as-is (`ns/op`, `B/op`, `allocs/op`).
- Converts higher-is-better throughput (`MB/s`) into lower-is-better `ns/MB`.
  This preserves throughput signal while aligning alert polarity.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

MBS_RE = re.compile(r"(?P<val>\d+(?:\.\d+)?)\s+MB/s")


def convert_mbs_to_ns_per_mb(line: str) -> str:
    def repl(match: re.Match[str]) -> str:
        mbs = float(match.group("val"))
        if mbs <= 0:
            return "inf ns/MB"
        ns_per_mb = 1_000_000_000.0 / mbs
        return f"{ns_per_mb:.2f} ns/MB"

    return MBS_RE.sub(repl, line)


def normalize_line(line: str) -> str:
    if not line.startswith("Benchmark"):
        return line
    return convert_mbs_to_ns_per_mb(line)


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: normalize_bench_metrics.py <input> <output>", file=sys.stderr)
        return 2

    in_path = Path(sys.argv[1])
    out_path = Path(sys.argv[2])

    src = in_path.read_text(encoding="utf-8")
    normalized = "".join(normalize_line(line) for line in src.splitlines(keepends=True))
    out_path.write_text(normalized, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
