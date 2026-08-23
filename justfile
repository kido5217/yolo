version := `git describe --tags --always --dirty || echo 0.0.0-dev`

build:
    go build -ldflags "-X main.version={{version}}" -o yolo ./cmd/yolo

e2e-live:
    scripts/e2e-live.sh
