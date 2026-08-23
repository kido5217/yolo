package server

import (
	"fmt"

	"github.com/kido5217/yolo/internal/llm/fake"
)

// FakeFromEnv resolves YOLO_LLM/YOLO_FAKE_SCRIPT (M5 gate):
//   - unset          -> (nil, nil): production drivers
//   - "fake" + script -> scripted driver loaded from YOLO_FAKE_SCRIPT
//   - "fake" w/o script, or any other value -> error (500 at boot)
func FakeFromEnv(env map[string]string) (*fake.Driver, error) {
	mode, ok := env["YOLO_LLM"]
	if !ok || mode == "" {
		return nil, nil
	}
	if mode != "fake" {
		return nil, fmt.Errorf("YOLO_LLM=%q unsupported; use \"fake\"", mode)
	}
	script := env["YOLO_FAKE_SCRIPT"]
	if script == "" {
		return nil, fmt.Errorf("YOLO_LLM=fake requires YOLO_FAKE_SCRIPT (json script path)")
	}
	return fake.FromScript(script)
}
