version := `git describe --tags --always --dirty || echo 0.0.0-dev`

build:
    go build -ldflags "-X main.version={{version}}" -o yolo ./cmd/yolo

e2e-live:
    scripts/e2e-live.sh

# Parity capture (S8.2) — on-demand, user-run, NEVER CI: re-captures the
# 17 upstream pty fixtures + MANIFEST.json. See scripts/parity/capture.sh.
parity-capture:
    bash scripts/parity/capture.sh

# Parity sweep (S8.3) — on-demand, user-run, NEVER CI: renders the yolo
# side + diffs all 17 surfaces against the pinned upstream fixtures.
# See scripts/parity/sweep.py.
parity-sweep:
    python3 scripts/parity/sweep.py
