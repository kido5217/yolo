#!/usr/bin/env python3
"""sweep.py — the S8.3 parity diff sweep (spec §7.3, D7).

ON-DEMAND, user-run, NEVER CI (the root e2e-live.sh pattern — the entry
is `just parity-sweep`):
  1. renders the yolo side: YOLO_PARITY_DUMP=<tmp> go test -count=1
     -run ^TestParityDump$ ./internal/tui/ (the D6 dump test writes the
     17 raw streams to <tmp>/yolo/<name>.raw),
  2. normalizes BOTH sides with the shared normalize.py (the yolo raw
     streams; the upstream side is the pinned normalized fixture
     upstream/<name>.screen.json — D4),
  3. per-surface cell diff (t/fg/bg/b on the union of cell keys),
  4. writes docs/superpowers/plans/2026-08-24-opencode-tui-parity/
     parity-sweep-report.md (the MATCH / GAPS(n) table + the mismatch
     detail + the environment: the yolo HEAD sha, the manifest sha256,
     the npm version) and prints the summary.

Exit 0 on a COMPLETED sweep — the GAPS lines are INFORMATIONAL (S8.4
consumes the report and closes or logs every gap, D7); exit 1 on a
mechanical failure (the go test failure, a missing fixture, a crash).
"""

import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
import normalize  # noqa: E402

TESTDATA = os.path.join(REPO, "internal", "tui", "testdata", "parity")
UPSTREAM = os.path.join(TESTDATA, "upstream")
MANIFEST = os.path.join(UPSTREAM, "MANIFEST.json")
REPORT = os.path.join(
    REPO,
    "docs",
    "superpowers",
    "plans",
    "2026-08-24-opencode-tui-parity",
    "parity-sweep-report.md",
)


def fail(msg):
    print("FAIL: %s" % msg)
    sys.exit(1)


def main():
    if not os.path.exists(MANIFEST):
        fail("the upstream MANIFEST.json is missing (run `just parity-capture` first)")
    with open(MANIFEST) as fh:
        man = json.load(fh)
    dump = tempfile.mkdtemp(prefix="yolo-parity-")
    env = dict(os.environ, YOLO_PARITY_DUMP=dump)
    r = subprocess.run(
        ["go", "test", "-count=1", "-run", "^TestParityDump$", "./internal/tui/"],
        cwd=REPO,
        env=env,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print(r.stdout)
        print(r.stderr)
        fail("the yolo dump test failed (exit %d)" % r.returncode)
    head = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=REPO, capture_output=True, text=True
    ).stdout.strip()
    with open(MANIFEST, "rb") as fh:
        man_sha = hashlib.sha256(fh.read()).hexdigest()
    rows = []
    for s in man["surfaces"]:
        name = s["name"]
        yolo_path = os.path.join(dump, "yolo", name + ".raw")
        up_path = os.path.join(UPSTREAM, name + ".screen.json")
        if not os.path.exists(yolo_path):
            fail(
                "the yolo dump is missing %s (TestParityDump did not render it)" % name
            )
        if not os.path.exists(up_path):
            fail(
                "the upstream fixture is missing %s (run `just parity-capture` first)"
                % name
            )
        with open(yolo_path, "rb") as fh:
            yolo = normalize.screen(fh.read(), s["cols"], s["rows"])
        with open(up_path) as fh:
            upstream = json.load(fh)
        rows.append((name, s["cols"], s["rows"], diff_screens(upstream, yolo)))
    # the report
    lines = [
        "# Parity sweep report (S8.3)",
        "",
        "- yolo HEAD: `%s`" % head,
        "- fixture manifest sha256: `%s`" % man_sha,
        "- npm opencode-ai: %s" % man["npm_version"],
        "",
        "| surface | size | result |",
        "|---|---|---|",
    ]
    for name, cols, rows_, gaps in rows:
        lines.append(
            "| %s | %dx%d | %s |"
            % (name, cols, rows_, "MATCH" if not gaps else "GAPS(%d)" % len(gaps))
        )
    lines.append("")
    for name, cols, rows_, gaps in rows:
        if not gaps:
            continue
        lines.append("## %s — %d gaps" % (name, len(gaps)))
        lines.append("")
        for g in gaps[:20]:
            lines.append(
                "- cell %s %s: upstream=%r yolo=%r"
                % (g["cell"], g["field"], g["upstream"], g["yolo"])
            )
        if len(gaps) > 20:
            lines.append("- … %d more" % (len(gaps) - 20))
        lines.append("")
    os.makedirs(os.path.dirname(REPORT), exist_ok=True)
    with open(REPORT, "w") as fh:
        fh.write("\n".join(lines) + "\n")
    n_match = sum(1 for _, _, _, g in rows if not g)
    print(
        "PASS: sweep complete — %d/%d surfaces MATCH, report at %s"
        % (n_match, len(rows), os.path.relpath(REPORT, REPO))
    )


def cellkey(k):
    r, c = k.split(":")
    return (int(r), int(c))


def diff_screens(a, b):
    """The per-cell diff (D7): t/fg/bg/b on the union of cell keys."""
    out = []
    keys = set(a.get("cells", {})) | set(b.get("cells", {}))
    for k in sorted(keys, key=cellkey):
        ca = a.get("cells", {}).get(k, {})
        cb = b.get("cells", {}).get(k, {})
        for f in ("t", "fg", "bg", "b"):
            if ca.get(f) != cb.get(f):
                out.append(
                    {"cell": k, "field": f, "upstream": ca.get(f), "yolo": cb.get(f)}
                )
    return out


if __name__ == "__main__":
    main()
