#!/usr/bin/env python3
"""normalize.py — the S8.2/S8.3 shared normalizer (spec §7.3, D3).

Replays a raw terminal byte stream (the upstream pty capture or the yolo
teatest dump) into ONE canonical screen JSON:

    {"cols": W, "rows": H,
     "cells": {"<row>:<col>": {"t": char, "fg": color, "bg": color,
                                "b": true}}}

Replay model: cursor positioning (CUP H/F), SGR (0, 1, 30-37/90-97,
40-47/100-107, 38;2;r;g;b / 48;2;r;g;b, 38;5;n / 48;5;n), printable
characters (UTF-8, wide runes take 2 columns, autowrap at the right
edge), CR/LF/VT/FF, TAB. The stream is replayed IN ORDER so the last
repaint of a cell wins — both TUIs (opentui, bubbletea) repaint with
absolute cursor positioning, so the result is the true final screen.

Volatile bits are masked to fixed tokens BEFORE replay (D5): OSC 12/0/22
(cursor color / window+icon titles / OSC 22), synchronized output
([>4;...m / [<4m), ses_ session ids, ISO-8601 timestamps, and the fixed
scratch path prefix /tmp/opencode-parity. Truecolor and 256-color values
are kept VERBATIM — the color-space difference (upstream 24-bit vs the
yolo ANSI256 teatest pin, deviation 125) is a parity finding for S8.4,
not normalizer noise.
"""

import re
import unicodedata

_MASKS = [
    (re.compile(rb"\x1b\]12;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\]0;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\]22;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\[>4;[0-9]*m"), b""),
    (re.compile(rb"\x1b\[<4m"), b""),
    (re.compile(rb"ses_[A-Za-z0-9]{10,}"), b"<SES>"),
    (re.compile(rb"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z"), b"<TS>"),
    # S8.2 first-live-run additions (deviation 255): the sidebar
    # session-title line ("New session - <ISO timestamp>") WRAPS — the
    # time part and the ms fragment (" 710Z") are drawn as separate
    # styled runs on different lines (not split across ESC chunks), so
    # the contiguous TS mask above cannot see them. The local fragment
    # masks normalize the volatile parts (date / time / milliseconds);
    # with all three applied the line is run-independent.
    (re.compile(rb"\d{4}-\d{2}-\d{2}"), b"<D>"),
    (re.compile(rb"\d{2}:\d{2}:\d{2}"), b"<T>"),
    (re.compile(rb"[ ]?\d{3}Z\b"), b""),
    (re.compile(rb"/tmp/opencode-parity"), b"<SCRATCH>"),
    # S8.2 first-live-run additions (deviation 255): the message-meta
    # duration (Locale.duration "NNNms" / "N.Ns" — pure jitter over the
    # unpaced mock stream) and the home prompt placeholder line. The
    # line is drawn as a gray text run ("Ask anything... "<example>"" —
    # example = one of 3 boot-randomized strings, randomIndex over the
    # home.tsx list) + a white space FILL from the raw text end to the
    # input right edge (the fill's start column is computed from the
    # raw example length). The whole line is canonicalized to the
    # fixed-length form ("<EX>" + 35 spaces, fill at col 64, 15 spaces)
    # so the replay is identical across boots; the SGR values are the
    # default-theme constants of the hermetic capture env.
    (re.compile(rb"(?<!\d)\d+ms\b"), b"<DUR>"),
    (re.compile(rb"(?<!\d)\d+\.\ds\b"), b"<DUR>"),
    (
        re.compile(
            rb"\x1b\[13;7H\x1b\[38;2;128;128;128m\x1b\[48;2;30;30;30m"
            rb"Ask anything... \"[^\"]*\"\x1b\[0m\x1b\[13;\d+H"
            rb"\x1b\[38;2;255;255;255m\x1b\[48;2;30;30;30m +"
        ),
        b"\x1b[13;7H\x1b[38;2;128;128;128m\x1b[48;2;30;30;30m"
        b'Ask anything... "<EX>' + b" " * 35 + b'"\x1b[0m'
        b"\x1b[13;64H\x1b[38;2;255;255;255m\x1b[48;2;30;30;30m" + b" " * 15,
    ),
]

# The param class admits '>' and '$' (the intermediate bytes of the
# private sequences bubbletea v2 emits at boot/exit: the DECRQM queries
# \x1b[?2026$p / \x1b[?2027$p and the secondary-DA \x1b[>4m / \x1b[>1u).
# They are consumed-and-ignored below (S8.3, deviation 256): without the
# two extra bytes the ESC is skipped alone and the leftover "[>4m"-class
# text lands in the replayed screen (row 0 of every yolo surface). The
# upstream capture never emits them (its sync output is masked above),
# so this is a no-op for the pinned upstream fixtures.
_CSI = re.compile(rb"\x1b\[([0-9;?<>$]*)([A-Za-z])")

_FG_BASIC = {c: "ansi:%d" % (30 + c) for c in range(8)}
_BG_BASIC = {c: "ansi:%d" % (40 + c) for c in range(8)}
_FG_BRIGHT = {c: "ansi:%d" % (90 + c) for c in range(8)}
_BG_BRIGHT = {c: "ansi:%d" % (100 + c) for c in range(8)}


def mask(data: bytes) -> bytes:
    """Apply the volatile-bit masks (D3)."""
    for rx, sub in _MASKS:
        data = rx.sub(sub, data)
    return data


def screen(data: bytes, cols: int, rows: int) -> dict:
    """Replay the raw stream into the final cols x rows cell grid."""
    data = mask(data)
    grid = [[None] * cols for _ in range(rows)]
    r = c = 0
    fg = bg = None
    bold = False
    i, n = 0, len(data)
    while i < n:
        b = data[i]
        if b == 0x1B:
            m = _CSI.match(data, i)
            if m:
                params, fin = m.group(1), m.group(2)
                i = m.end()
                if fin in (b"H", b"F"):
                    ps = [p for p in params.split(b";") if p != b""]
                    if len(ps) >= 2:
                        r = max(0, min(rows - 1, int(ps[0]) - 1))
                        c = max(0, min(cols - 1, int(ps[1]) - 1))
                elif fin == b"m":
                    ps = [p for p in params.split(b";") if p != b""]
                    if not ps:
                        ps = [b"0"]
                    if not all(p.isdigit() for p in ps):
                        # a non-numeric-param "m" sequence (the private
                        # \x1b[>4m secondary-DA query) is not SGR:
                        # consumed-and-ignored (deviation 256).
                        continue
                    j = 0
                    while j < len(ps):
                        code = int(ps[j])
                        if code == 0:
                            fg = bg = None
                            bold = False
                            j += 1
                        elif code == 1:
                            bold = True
                            j += 1
                        elif code in (38, 48) and j + 1 < len(ps):
                            kind, adv = None, 1
                            if ps[j + 1] == b"2" and j + 4 < len(ps):
                                kind = "rgb:#%02x%02x%02x" % (
                                    int(ps[j + 2]),
                                    int(ps[j + 3]),
                                    int(ps[j + 4]),
                                )
                                adv = 5
                            elif ps[j + 1] == b"5" and j + 2 < len(ps):
                                kind = "256:" + ps[j + 2].decode("ascii")
                                adv = 3
                            if kind is not None:
                                if code == 38:
                                    fg = kind
                                else:
                                    bg = kind
                            j += adv
                        elif 30 <= code <= 37:
                            fg = _FG_BASIC[code - 30]
                            j += 1
                        elif 90 <= code <= 97:
                            fg = _FG_BRIGHT[code - 90]
                            j += 1
                        elif 40 <= code <= 47:
                            bg = _BG_BASIC[code - 40]
                            j += 1
                        elif 100 <= code <= 107:
                            bg = _BG_BRIGHT[code - 100]
                            j += 1
                        else:
                            j += 1
                continue
            if data[i : i + 2] == b"\x1b]":
                # an OSC the masks did not claim: skip to the terminator.
                j = i + 2
                while j < n and data[j] not in (0x07, 0x1B):
                    j += 1
                i = (j + 1) if j < n else i + 1
                continue
            i += 1
            continue
        if b == 0x0D:
            c = 0
        elif b in (0x0A, 0x0B, 0x0C):
            r = min(rows - 1, r + 1)
        elif b == 0x09:
            c = min(cols - 1, (c // 8 + 1) * 8)
        elif 0x20 <= b <= 0x7E:
            grid[r][c] = {
                "t": bytes([b]).decode("ascii"),
                "fg": fg,
                "bg": bg,
                "bold": bold,
            }
            c += 1
            if c >= cols:
                c = 0
                r = min(rows - 1, r + 1)
        elif b >= 0x80:
            ln = 4 if b >= 0xF0 else 3 if b >= 0xE0 else 2 if b >= 0xC0 else 1
            seq = data[i : i + ln]
            if len(seq) < ln:
                i += 1
                continue
            try:
                s = seq.decode("utf-8")
            except UnicodeDecodeError:
                i += 1
                continue
            grid[r][c] = {"t": s, "fg": fg, "bg": bg, "bold": bold}
            c += 2 if unicodedata.east_asian_width(s) in ("W", "F") else 1
            if c >= cols:
                c = 0
                r = min(rows - 1, r + 1)
            i += ln
            continue
        else:
            pass  # the other control bytes: ignored
        i += 1
    cells = {}
    for rr in range(rows):
        for cc in range(cols):
            cell = grid[rr][cc]
            if cell is None:
                continue
            if (
                cell["t"] == " "
                and cell["fg"] is None
                and cell["bg"] is None
                and not cell["bold"]
            ):
                continue
            out = {"t": cell["t"]}
            if cell["fg"] is not None:
                out["fg"] = cell["fg"]
            if cell["bg"] is not None:
                out["bg"] = cell["bg"]
            if cell["bold"]:
                out["b"] = True
            cells["%d:%d" % (rr, cc)] = out
    return {"cols": cols, "rows": rows, "cells": cells}
