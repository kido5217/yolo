#!/usr/bin/env python3
"""capture.py — the S8.2 upstream pty-capture driver (spec §7.3).

ON-DEMAND, user-run, NEVER CI (the root e2e-live.sh pattern — the entry
is capture.sh / `just parity-capture`). Drives the npm
opencode-ai@1.18.18 TUI (the Bun single-file binary; the core server runs
in-process in a worker) in a pty against the S8.1 mock OpenAI-compatible
SSE server, per surface. Determinism (D5): a FRESH hermetic HOME +
project scratch per surface (fixed prefix /tmp/opencode-parity — masked
by normalize.py), the pinned catalog (OPENCODE_MODELS_PATH),
OPENCODE_DISABLE_MODELS_FETCH/AUTOUPDATE/MOUSE=1, --pure --auto, NO
--print-logs (it writes into the pty — verified at detail time), fixed
terminal sizes, the volatile bits masked, and a DOUBLE capture per
surface (the two normalized screens must be byte-identical or the run
fails). Writes the pinned fixtures + MANIFEST.json (D4) to
internal/tui/testdata/parity/upstream/ and prints the re-baselined
fixture list (the executor commits them in the same commit — root
principle 3).
"""

import fcntl
import hashlib
import json
import os
import pty
import re
import select
import shutil
import signal
import struct
import subprocess
import sys
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
import normalize  # noqa: E402

TESTDATA = os.path.join(REPO, "internal", "tui", "testdata", "parity")
UPSTREAM = os.path.join(TESTDATA, "upstream")
CATALOG = os.path.join(TESTDATA, "catalog-pin.json")
CANNED = os.path.join(TESTDATA, "canned.json")
SCRATCH = "/tmp/opencode-parity"
BIN = os.path.join(
    SCRATCH, "node", "node_modules", "opencode-ai", "bin", "opencode.exe"
)
MOCK = os.path.join(SCRATCH, "mock")
NPM_VERSION = "1.18.18"

# The D2 17-surface table (frozen at detail time): (name, (cols, rows),
# canned turn, key script). A step is ("wait", secs) | ("keys", bytes,
# label). The turn step expands to: type the canned prompt, enter, wait.
TURN = [
    ("keys", b"__PROMPT__", "prompt"),
    ("wait", 0.5),
    ("keys", b"\r", "enter"),
    ("wait", 8.0),
]


def _turn():
    return [list(s) for s in TURN]


SURFACES = [
    ("home", (80, 24), "text", [("wait", 8.0)]),
    ("session-text", (80, 24), "text", _turn()),
    ("session-tool", (80, 24), "tool", _turn()),
    (
        "palette",
        (80, 24),
        "text",
        _turn() + [("keys", b"\x10", "ctrl+p"), ("wait", 4.0)],
    ),
    (
        "help",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"/help", "slash"),
            ("wait", 0.5),
            ("keys", b"\r", "enter"),
            ("wait", 4.0),
        ],
    ),
    (
        "model",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"m", "model_list"),
            ("wait", 4.0),
        ],
    ),
    (
        "agent",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"a", "agent_list"),
            ("wait", 4.0),
        ],
    ),
    (
        "theme",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"t", "theme_list"),
            ("wait", 4.0),
        ],
    ),
    (
        "session-list",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"l", "session_list"),
            ("wait", 4.0),
        ],
    ),
    (
        "session-rename",
        (80, 24),
        "text",
        _turn() + [("keys", b"\x12", "ctrl+r"), ("wait", 4.0)],
    ),
    (
        "session-delete",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"l", "session_list"),
            ("wait", 3.0),
            ("keys", b"\x04", "ctrl+d arm"),
            ("wait", 3.0),
        ],
    ),
    (
        "status",
        (80, 24),
        "text",
        _turn()
        + [
            ("keys", b"\x18", "leader"),
            ("wait", 0.5),
            ("keys", b"s", "status_view"),
            ("wait", 4.0),
        ],
    ),
    (
        "which-key",
        (80, 24),
        "text",
        [("wait", 8.0), ("keys", b"\x18", "leader held"), ("wait", 3.0)],
    ),
    ("sidebar", (140, 30), "todo", _turn()),
    (
        "prompt-slash",
        (80, 24),
        "text",
        _turn() + [("keys", b"/", "slash menu"), ("wait", 3.0)],
    ),
    (
        "prompt-mention",
        (80, 24),
        "text",
        _turn() + [("keys", b"@par", "mention"), ("wait", 3.0)],
    ),
    (
        "epilogue",
        (80, 24),
        "text",
        _turn() + [("keys", b"\x03", "ctrl+c exit"), ("wait", 4.0)],
    ),
]

CANNED_PROMPTS = {}  # filled from canned.json at main()


def sha256(path):
    with open(path, "rb") as fh:
        return hashlib.sha256(fh.read()).hexdigest()


def env_for(rundir):
    home = os.path.join(rundir, "home")
    os.makedirs(home, exist_ok=True)
    e = dict(os.environ)
    e.update(
        {
            "HOME": home,
            "XDG_DATA_HOME": os.path.join(home, ".local", "share"),
            "XDG_CACHE_HOME": os.path.join(home, ".cache"),
            "XDG_CONFIG_HOME": os.path.join(home, ".config"),
            "XDG_STATE_HOME": os.path.join(home, ".local", "state"),
            "OPENCODE_MODELS_PATH": CATALOG,
            "OPENCODE_DISABLE_MODELS_FETCH": "1",
            "OPENCODE_DISABLE_AUTOUPDATE": "1",
            "OPENCODE_DISABLE_MOUSE": "1",
            "TERM": "xterm-256color",
            "COLORTERM": "truecolor",
        }
    )
    return e


def write_project(rundir, port):
    proj = os.path.join(rundir, "proj")
    os.makedirs(proj, exist_ok=True)
    for f in ("parity-a.txt", "parity-b.txt"):  # the mention-surface files
        with open(os.path.join(proj, f), "w") as fh:
            fh.write("x")
    cfg = {
        "$schema": "https://opencode.ai/config.json",
        "model": "mockllm/canned",
        # the tool/todo surfaces auto-approve (the yolo side seeds the
        # equivalent config permission rules — both sides reach the
        # second turn without a permission overlay).
        "permission": {"bash": "allow", "todowrite": "allow"},
        "provider": {
            "mockllm": {
                "name": "Mock LLM",
                "npm": "@ai-sdk/openai-compatible",
                "options": {
                    "baseURL": "http://127.0.0.1:%d/v1" % port,
                    "apiKey": "parity",
                },
                "models": {"canned": {"name": "Canned"}},
            },
        },
    }
    with open(os.path.join(proj, "opencode.json"), "w") as fh:
        json.dump(cfg, fh)
    return proj


def start_mock(turn):
    p = subprocess.Popen(
        [MOCK, "-addr", "127.0.0.1:0", "-turn", turn, "-canned", CANNED],
        stdout=subprocess.PIPE,
    )
    line = p.stdout.readline()
    m = re.match(rb"MOCK_PORT=(\d+)", line.strip())
    if not m:
        p.kill()
        raise SystemExit("mock handshake failed: %r" % line)
    return p, int(m.group(1))


def set_winsize(fd, rows, cols):
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))


def pump(fd, secs, raw):
    end = time.time() + secs
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.05)
        if fd in r:
            try:
                data = os.read(fd, 65536)
            except OSError:
                break
            if not data:
                break
            raw.extend(data)


def reap(pid, fd):
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.waitpid(pid, 0)
    except ChildProcessError:
        pass
    try:
        os.close(fd)
    except OSError:
        pass


def run_pty(cols, rows, steps, rundir):
    proj = os.path.join(rundir, "proj")
    pid, fd = pty.fork()
    if pid == 0:
        try:
            os.chdir(proj)
            os.execve(BIN, ["opencode", "--pure", "--auto"], env_for(rundir))
        except Exception:
            os._exit(127)
    set_winsize(fd, rows, cols)
    raw = bytearray()
    pump(fd, 6.0, raw)  # boot settle: the TUI must be ready to accept keys
    for step in steps:
        if step[0] == "wait":
            pump(fd, step[1], raw)
        elif step[0] == "keys":
            os.write(fd, step[1])
    pump(fd, 2.0, raw)  # the final settle
    reap(pid, fd)
    return bytes(raw)


def run_once(cols, rows, steps, turn):
    # the FRESH hermetic HOME + project scratch the docstring promises
    # (deviation 254): the pinned fixed rundir leaked the previous run's
    # sessions into the next one (D5 double-run mismatch).
    rundir = os.path.join(SCRATCH, "run")
    shutil.rmtree(rundir, ignore_errors=True)
    os.makedirs(rundir, exist_ok=True)
    mock, port = start_mock(turn)
    try:
        write_project(rundir, port)
        raw = run_pty(cols, rows, steps, rundir)
    finally:
        mock.kill()
        mock.wait()
    return raw


def expand(steps, prompt):
    """Substitute the canned prompt into the __PROMPT__ placeholder."""
    return [
        s
        if s[0] != "keys" or s[1] != b"__PROMPT__"
        else ["keys", prompt.encode(), "prompt"]
        for s in steps
    ]


def capture_surface(name, size, steps, turn, prompt):
    """The D5 double-run: two fresh pty runs must normalize identically."""
    st = expand(steps, prompt)
    a = normalize.screen(run_once(size[0], size[1], st, turn), size[0], size[1])
    b = normalize.screen(run_once(size[0], size[1], st, turn), size[0], size[1])
    if a != b:
        ka = set(a["cells"])
        kb = set(b["cells"])
        diffs = [k for k in sorted(ka & kb) if a["cells"][k] != b["cells"][k]]
        raise SystemExit(
            "FAIL: %s — the double-capture screens differ (first: %s)"
            % (name, diffs[:5] or (ka ^ kb))
        )
    return a


def main():
    for req in (CATALOG, CANNED, MOCK, BIN):
        if not os.path.exists(req):
            raise SystemExit("FAIL: missing %s (capture.sh prepares the runtime)" % req)
    book = json.load(open(CANNED))
    os.makedirs(UPSTREAM, exist_ok=True)
    man_path = os.path.join(UPSTREAM, "MANIFEST.json")
    if os.path.exists(man_path):
        man = json.load(open(man_path))
    else:
        man = {"npm_version": NPM_VERSION, "surfaces": []}
    changed = []
    for name, size, turn, steps in SURFACES:
        prompt = book[turn]["prompt"]
        screen = capture_surface(name, size, steps, turn, prompt)
        blob = json.dumps(screen, sort_keys=True, separators=(",", ":")).encode()
        path = os.path.join(UPSTREAM, "%s.screen.json" % name)
        old = open(path, "rb").read() if os.path.exists(path) else None
        if old != blob:
            with open(path, "wb") as fh:
                fh.write(blob)
            changed.append(name)
            print("   re-baselined: %s" % name)
        else:
            print("   stable:       %s" % name)
        man["surfaces"] = [s for s in man.get("surfaces", []) if s["name"] != name]
        man["surfaces"].append(
            {
                "name": name,
                "cols": size[0],
                "rows": size[1],
                "sha256": hashlib.sha256(blob).hexdigest(),
            }
        )
    man["catalog_sha256"] = sha256(CATALOG)
    man["canned_sha256"] = sha256(CANNED)
    man["npm_version"] = NPM_VERSION
    with open(man_path, "w") as fh:
        json.dump(man, fh, sort_keys=True, indent=1)
        fh.write("\n")
    if changed:
        print("PASS: %d fixtures re-baselined: %s" % (len(changed), ", ".join(changed)))
    else:
        print("PASS: all 17 fixtures stable")


if __name__ == "__main__":
    main()
