#!/usr/bin/env python3
"""normalize.py — the S8.2/S8.3 shared normalizer (spec §7.3, D3), extended
in S8.4 to a faithful terminal replay (deviation 257).

Replays a raw terminal byte stream (the upstream pty capture or the yolo
teatest dump) into ONE canonical screen JSON:

    {"cols": W, "rows": H,
     "cells": {"<row>:<col>": {"t": char, "fg": color, "bg": color,
                                 "b": true}}}

Replay model: a bounded terminal-emulator core. The D3 minimal replay
(cursor H/F + SGR + printables) was validated against the upstream stream
only and is not faithful for the bubbletea v2 yolo streams (deviation
256: the LNM-OFF bare-LF staircase, the ESC M "M" leak, the erase
no-ops, the consumed-and-ignored moves/scrolls). The S8.4 core adds:
LNM (LF also CRs; default ON, tracked over CSI ?20 h/l), IND (ESC M),
NEL (ESC D), RI (ESC K), the pending-wrap state (DECAWM default ON: a
full row parks the cursor at the last column; the next printable / TAB /
linefeed steps to the next line), the erase ops (ED J / EL K / ECH X —
erase to the unpainted default background, i.e. the cell drops out of
the screen JSON like an unpainted cell), the DECSTBM scroll region (r)
with the bounded scrolls (SU S / SD T; a linefeed / IND / HPR / VPR at
the region edge scrolls the region), and the relative (CUU A / CUD B /
CUF C / CUB D) + absolute (CUP H/F incl. the bare and 1-param forms,
CHA G, VPA d, HPR E, VPR F) cursor moves. ESC 7/8 save/restore, ESC c
full reset, TAB, and the OSC 8 hyperlink wrapper (the BEL / ST-
terminated opener and closer are consumed; the link text between them
is normal content — and the ST terminator no longer leaks a literal
"\\", which the S8.2 fixtures carried on the link line). The SGR state
handling is UNCHANGED. Unknown CSI forms (the DECRQM ?2026$p / ?2027$p
queries, the >4m / >1u / <1u secondary-DA class, the DECSCUSR
\\x1b[1 q / \\x1b[0 q with a space in the parameters — deviation 255,
and any form carrying an intermediate byte) and unknown ESC finals are
consumed-and-ignored, never replayed as literal text. The ?1049 h/l
alt-screen ops keep the S8.3 consume-and-ignore semantics: the final
screen is the last visible app screen; the app-exit restore to the
pre-app main screen is out of scope for both captures (the upstream
capture kills the pty with the app still on the alt screen for 16 of
17 surfaces; the yolo dump test always ends with the exit path). The
stream is replayed IN ORDER so the last repaint of a cell wins — both
TUIs (opentui, bubbletea) repaint with absolute cursor positioning, so
the result is the true final screen.

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
    # S8.4 (deviation 257): the yolo dump runs the app with a volatile Go
    # test temp dir as the scope dir (testutil boot + t.TempDir —
    # /tmp/Test<Name><rand>/00N), which the home destination line
    # ("the scope dir, home-abbreviated") renders verbatim. The upstream
    # referent of the same slot is /tmp/opencode-parity/run/proj (the
    # masked <SCRATCH>/run/proj) — the yolo path is masked to the same
    # token so the slot compares as the slot (the environment is noise).
    # The path appears in several forms: the full path with the /00N
    # temp-dir counter, the full path without it (the session-list rows),
    # and — in the session-delete footer, where the cwd wraps mid-path
    # after the confirm hint — only the leading run
    # (/tmp/Test<Name><surface>-). The optional (?:/\d+)? lets one mask
    # cover all of them; the volatile random suffix that survives the
    # wrap (a bare <digits>/00N run on the next line) is canonicalized by
    # the \d{5,}/00N mask below.
    (re.compile(rb"/tmp/Test[A-Za-z0-9-]+(?:/\d+)?"), b"<SCRATCH>/run/proj"),
    (re.compile(rb"\d{5,}/00\d"), b"<RAND>/002"),
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

# The CSI parse (S8.4, deviation 257): after ESC [ the param bytes
# (0x30-0x3F: digits, ';', ':', '<', '=', '>', '?') are followed by any
# intermediate bytes (0x20-0x2F) and the final byte (0x40-0x7E). The
# S8.3 regex \x1b\[([0-9;?<>$]*)([A-Za-z]) stopped at the first
# intermediate — enough for the corpus minus the upstream DECSCUSR
# forms \x1b[1 q / \x1b[0 q, which carry a SPACE in the parameters
# (deviation 255): the old parse left the "[1 q" fragment un-consumed
# and it replayed as literal text. The parse below consumes the whole
# sequence; a form carrying intermediates is not SGR / not a standard
# cursor op and is consumed-and-ignored. The private param markers
# ('?' '<' '>' '$') admit the bubbletea v2 boot/exit queries (the
# DECRQM \x1b[?2026$p / \x1b[?2027$p, the secondary-DA \x1b[>4m /
# \x1b[>1u / \x1b[<1u) — consumed-and-ignored below (deviation 256).

_FG_BASIC = {c: "ansi:%d" % (30 + c) for c in range(8)}
_BG_BASIC = {c: "ansi:%d" % (40 + c) for c in range(8)}
_FG_BRIGHT = {c: "ansi:%d" % (90 + c) for c in range(8)}
_BG_BRIGHT = {c: "ansi:%d" % (100 + c) for c in range(8)}


def mask(data: bytes) -> bytes:
    """Apply the volatile-bit masks (D3)."""
    for rx, sub in _MASKS:
        data = rx.sub(sub, data)
    return data


def _csi_parse(data: bytes, i: int):
    """Parse the CSI starting at data[i] (data[i] == ESC, data[i+1] == '[').

    Returns (params_raw, has_intermediates, final, end) — params_raw is
    the raw param bytes, has_intermediates True when a 0x20-0x2F byte
    follows the params, final the final byte as a str, end the index
    just past the sequence — or None when the sequence is unterminated
    at the end of the stream.
    """
    n = len(data)
    j = i + 2
    while j < n and not (0x40 <= data[j] <= 0x7E):
        j += 1
    if j >= n:
        return None
    body = data[i + 2 : j]
    k = 0
    while k < len(body) and 0x30 <= body[k] <= 0x3F:
        k += 1
    return body[:k], k < len(body), chr(data[j]), j + 1


def screen(data: bytes, cols: int, rows: int) -> dict:
    """Replay the raw stream into the final cols x rows cell grid."""
    data = mask(data)
    grid = [[None] * cols for _ in range(rows)]
    r = c = 0
    fg = bg = None
    bold = False
    lnm = True  # DECOM 20: LF also CRs. Default ON (yolo never sends ?20l).
    wrap = True  # DECAWM: autowrap at the right edge. Default ON.
    pend = False  # pending-wrap: the last column of a row was just written.
    top, bot = 0, rows - 1  # the DECSTBM region (0-based, inclusive).
    saved = None  # the ESC 7 saved cursor (r, c).
    i, n = 0, len(data)

    def blank(rr, cc):
        grid[rr][cc] = {"t": " ", "fg": None, "bg": None, "bold": False}

    def scroll_up(count):
        # the region scrolls up: content shifts up, the top row is pushed
        # out of the region and the bottom row clears for the new content.
        for _ in range(count):
            for rr in range(top, bot):
                grid[rr] = grid[rr + 1][:]
            grid[bot] = [None] * cols

    def scroll_down(count):
        # the region scrolls down: content shifts down, the bottom row is
        # pushed out and the top row clears.
        for _ in range(count):
            for rr in range(bot, top, -1):
                grid[rr] = grid[rr - 1][:]
            grid[top] = [None] * cols

    def advance_line():
        # one line down: at the bottom of the region the region scrolls;
        # a cursor parked below the region snaps to its bottom row.
        nonlocal r
        if r < bot:
            r += 1
        else:
            r = bot
            scroll_up(1)

    def wrap_cursor():
        nonlocal r, c, pend
        advance_line()
        c = 0
        pend = False

    def linefeed():
        # LF/VT/FF/IND: advance one line; the pending wrap consumes the
        # advance (no double-step); CR when lnm.
        nonlocal r, c, pend
        if pend:
            pend = False
            advance_line()
            c = 0
        else:
            advance_line()
            if lnm:
                c = 0

    def reverse_index():
        # RI: one line up; at the top of the region the region scrolls.
        nonlocal r, pend
        pend = False
        if r == top:
            scroll_down(1)
        else:
            r = max(top, r - 1)

    def put_cell(s, width):
        nonlocal r, c, pend
        if pend:
            wrap_cursor()
        cell = {"t": s, "fg": fg, "bg": bg, "bold": bold}
        if c >= cols - 1:
            # the last column: a wide rune draws its first half only; the
            # cursor parks in the pending-wrap state.
            grid[r][cols - 1] = cell
            pend = wrap
        else:
            grid[r][c] = cell
            c += width
            if c >= cols:
                c = cols - 1
                pend = wrap

    def tab():
        nonlocal r, c, pend
        if pend:
            wrap_cursor()
        nxt = (c // 8 + 1) * 8
        if nxt >= cols:
            c = cols - 1
            pend = wrap
        else:
            c = nxt
            pend = False

    while i < n:
        b = data[i]
        if b == 0x1B:
            if data[i : i + 2] == b"\x1b[":
                p = _csi_parse(data, i)
                if p is None:
                    i = n  # unterminated at the stream end: drop it
                    break
                params_raw, has_inter, fin, i = p
                if not has_inter:
                    ps = [x for x in params_raw.split(b";") if x != b""]
                    if fin == "m":
                        if not ps:
                            ps = [b"0"]
                        if not all(x.isdigit() for x in ps):
                            # a non-numeric-param "m" sequence (the
                            # private \x1b[>4m secondary-DA query) is not
                            # SGR: consumed-and-ignored (deviation 256).
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
                    elif fin in ("H", "F") and (fin == "H" or len(ps) >= 2):
                        # CUP (H/F): the bare form homes; one param is the
                        # row; two are row;col.
                        row = int(ps[0]) if ps and ps[0] else 1
                        col = int(ps[1]) if len(ps) > 1 and ps[1] else 1
                        r = max(0, min(rows - 1, row - 1))
                        c = max(0, min(cols - 1, col - 1))
                        pend = False
                    elif fin in ("A", "B", "C", "D"):
                        k = int(ps[0]) if ps and ps[0] else 1
                        if k < 1:
                            k = 1
                        if fin == "A":
                            r = max(0, r - k)
                        elif fin == "B":
                            r = min(rows - 1, r + k)
                        elif fin == "C":
                            c = min(cols - 1, c + k)
                        else:
                            c = max(0, c - k)
                        pend = False
                    elif fin == "G":
                        col = int(ps[0]) if ps and ps[0] else 1
                        c = max(0, min(cols - 1, col - 1))
                        pend = False
                    elif fin == "d":
                        row = int(ps[0]) if ps and ps[0] else 1
                        r = max(0, min(rows - 1, row - 1))
                        pend = False
                    elif fin in ("E", "F"):
                        # HPR (E) / VPR (F): n lines down / up, scrolling
                        # the region at its edges (a 1-param F is VPR;
                        # a 2-param F is CUP above).
                        k = int(ps[0]) if ps and ps[0] else 1
                        if k < 1:
                            k = 1
                        pend = False
                        if fin == "E":
                            for _ in range(k):
                                advance_line()
                        else:
                            for _ in range(k):
                                if r == top:
                                    scroll_down(1)
                                else:
                                    r = max(top, r - 1)
                    elif fin == "J":
                        mode = int(ps[0]) if ps else 0
                        if mode == 0:
                            for cc in range(c, cols):
                                blank(r, cc)
                            for rr in range(r + 1, rows):
                                for cc in range(cols):
                                    blank(rr, cc)
                        elif mode == 1:
                            for rr in range(r):
                                for cc in range(cols):
                                    blank(rr, cc)
                            for cc in range(c + 1):
                                blank(r, cc)
                        else:
                            for rr in range(rows):
                                for cc in range(cols):
                                    blank(rr, cc)
                    elif fin == "K":
                        mode = int(ps[0]) if ps else 0
                        if mode == 0:
                            for cc in range(c, cols):
                                blank(r, cc)
                        elif mode == 1:
                            for cc in range(c + 1):
                                blank(r, cc)
                        else:
                            for cc in range(cols):
                                blank(r, cc)
                    elif fin == "X":
                        k = int(ps[0]) if ps else 1
                        if k < 1:
                            k = 1
                        for cc in range(c, min(cols, c + k)):
                            blank(r, cc)
                    elif fin == "r":
                        # DECSTBM: the top;bottom scroll region (1-based,
                        # default 1;rows); an invalid range is ignored.
                        t = int(ps[0]) if ps and ps[0] else 1
                        bt = int(ps[1]) if len(ps) > 1 and ps[1] else rows
                        if 1 <= t < bt <= rows:
                            top, bot = t - 1, bt - 1
                    elif fin in ("S", "T"):
                        k = int(ps[0]) if ps else 1
                        if k < 1:
                            k = 1
                        if fin == "S":
                            scroll_up(k)
                        else:
                            scroll_down(k)
                    elif fin in ("h", "l") and len(ps) == 1 and ps[0] == b"20":
                        lnm = fin == "h"
                    # the other finals (q DECSCUSR — incl. the
                    # space-in-params \x1b[1 q / \x1b[0 q, the u / p
                    # DECRQM class, the bare ?1049 / ?25 / ?2004 modes):
                    # consumed-and-ignored.
                continue
            if data[i : i + 2] == b"\x1b]":
                # an OSC the masks did not claim: skip to the terminator
                # (BEL, or the ST = ESC \\ — the ST is consumed in full so
                # the backslash no longer leaks as literal text); a bare
                # ESC inside the OSC aborts it and starts a new sequence.
                # The OSC 8 hyperlink opener/closer ends at the terminator
                # and the link text between the two is normal content.
                j = i + 2
                while j < n:
                    if data[j] == 0x07:
                        i = j + 1
                        break
                    if data[j] == 0x1B:
                        if j + 1 < n and data[j + 1] == 0x5C:
                            i = j + 2
                        else:
                            i = j
                        break
                    j += 1
                else:
                    i = n
                continue
            if i + 1 < n:
                f2 = data[i + 1]
                if f2 == 0x4D:  # ESC M — IND: a linefeed (the yolo
                    linefeed()  # transcript line separator).
                    i += 2
                elif f2 == 0x44:  # ESC D — NEL: a linefeed + CR.
                    linefeed()
                    c = 0
                    i += 2
                elif f2 == 0x4B:  # ESC K — RI: a reverse index.
                    reverse_index()
                    i += 2
                elif f2 == 0x37:  # ESC 7 — DECSC: save the cursor.
                    saved = (r, c)
                    i += 2
                elif f2 == 0x38:  # ESC 8 — DECRC: restore the cursor.
                    if saved is not None:
                        r, c = saved
                    pend = False
                    i += 2
                elif f2 == 0x63:  # ESC c — RIS: the full reset.
                    for rr in range(rows):
                        for cc in range(cols):
                            grid[rr][cc] = None
                    fg = bg = None
                    bold = False
                    lnm = True
                    wrap = True
                    top, bot = 0, rows - 1
                    r = c = 0
                    pend = False
                    saved = None
                    i += 2
                else:
                    # an unknown ESC final (incl. the intermediate-
                    # carrying forms like the ESC ( B charset designations):
                    # consume the whole sequence — never leak it as text.
                    j = i + 1
                    while j < n and 0x20 <= data[j] <= 0x2F:
                        j += 1
                    if j >= n:
                        i = n
                    else:
                        i = j + 1
                continue
            i += 1
            continue
        if b == 0x0D:
            c = 0
            pend = False
        elif b in (0x0A, 0x0B, 0x0C):
            linefeed()
        elif b == 0x09:
            tab()
        elif 0x20 <= b <= 0x7E:
            put_cell(bytes([b]).decode("ascii"), 1)
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
            put_cell(s, 2 if unicodedata.east_asian_width(s) in ("W", "F") else 1)
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
