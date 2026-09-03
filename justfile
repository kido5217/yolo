version := `git describe --tags --always --dirty || echo 0.0.0-dev`

build:
    go build -ldflags "-X main.version={{version}}" -o yolo ./cmd/yolo

e2e-live:
    scripts/e2e-live.sh

wiki-stale:
    scripts/wiki-stale.sh

# Parity capture (S8.2) — on-demand, user-run, NEVER CI: re-captures the
# 17 upstream pty fixtures + MANIFEST.json. See scripts/parity/capture.sh.
parity-capture:
    bash scripts/parity/capture.sh
