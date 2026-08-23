package session

import (
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

//go:embed prompt/*.txt
var prompts embed.FS

var (
	planReminder   = mustEmbed("prompt/plan.txt")
	buildSwitchMsg = mustEmbed("prompt/build-switch.txt")
	titlePrompt    = mustEmbed("prompt/title.txt")
)

// TitlePrompt returns the title-generation system prompt (prompt/title.txt).
func TitlePrompt() string {
	return titlePrompt
}

func mustEmbed(path string) string {
	b, err := prompts.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// familyName picks the embedded prompt file for a model. Selection order is
// pinned (upstream system.ts first-match-wins):
//
//  1. apiID contains "muse"                         -> meta.txt
//  2. apiID contains "gpt-4" | "o1" | "o3"          -> beast.txt
//  3. apiID contains "gpt" ("codex" inside too)     -> codex.txt | gpt.txt
//  4. apiID contains "gemini-"                      -> gemini.txt
//  5. apiID contains "claude"                       -> anthropic.txt
//  6. lower(apiID) contains "trinity"               -> trinity.txt
//  7. lower(apiID) contains "kimi" OR provider id   -> kimi.txt
//     in {kimi-for-coding, moonshotai, moonshotai-cn}
//  8. anything else                                 -> default.txt
func familyName(apiID, providerID string) string {
	switch {
	case strings.Contains(apiID, "muse"):
		return "prompt/meta.txt"
	case strings.Contains(apiID, "gpt-4") || strings.Contains(apiID, "o1") || strings.Contains(apiID, "o3"):
		return "prompt/beast.txt"
	case strings.Contains(apiID, "gpt"):
		if strings.Contains(apiID, "codex") {
			return "prompt/codex.txt"
		}
		return "prompt/gpt.txt"
	case strings.Contains(apiID, "gemini-"):
		return "prompt/gemini.txt"
	case strings.Contains(apiID, "claude"):
		return "prompt/anthropic.txt"
	case strings.Contains(strings.ToLower(apiID), "trinity"):
		return "prompt/trinity.txt"
	case strings.Contains(strings.ToLower(apiID), "kimi") ||
		providerID == "kimi-for-coding" || providerID == "moonshotai" || providerID == "moonshotai-cn":
		return "prompt/kimi.txt"
	default:
		return "prompt/default.txt"
	}
}

// FamilyName returns the embedded file name (e.g. "prompt/default.txt").
func FamilyName(apiID, providerID string) string {
	return familyName(apiID, providerID)
}

// FamilyPrompt returns (embedded name, prompt text, error). For the meta
// family the {{MODEL_NAME}} placeholder is substituted per model.
func FamilyPrompt(apiID, providerID string) (string, string, error) {
	name := familyName(apiID, providerID)
	b, err := prompts.ReadFile(name)
	if err != nil {
		return "", "", err
	}
	text := string(b)
	if name == "prompt/meta.txt" {
		model := "Muse Spark"
		if strings.Contains(apiID, "muse-glimmer") {
			model = "Muse Glimmer"
		}
		text = strings.ReplaceAll(text, "{{MODEL_NAME}}", model)
	}
	return name, text, nil
}

// EnvBlock renders the environment block (v1: workspace root == working dir).
func EnvBlock(dir, apiID, providerID string) string {
	var b strings.Builder
	b.WriteString("You are powered by the model named " + apiID +
		". The exact model ID is " + providerID + "/" + apiID + "\n")
	b.WriteString("Here is some useful information about the environment you are running in:\n")
	b.WriteString("<env>\n")
	b.WriteString("  Working directory: " + dir + "\n")
	b.WriteString("  Workspace root folder: " + dir + "\n")
	b.WriteString("  Is directory a git repo: " + yesNo(gitRepo(dir)) + "\n")
	b.WriteString("  Platform: " + runtime.GOOS + "\n")
	b.WriteString("  Today's date: " + time.Now().Format("Mon Jan 02 2006") + "\n")
	b.WriteString("</env>")
	return b.String()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// gitCacheTTL bounds a cached git-repo answer: a dir that becomes a git repo
// mid-process stops being permanently "no". Upstream keeps the answer static
// per instance (ctx.project.vcs, detected at project scan) — the expiry is a
// deliberate deviation (behavior/low, deviation 99).
var gitCacheTTL = 60 * time.Second

// gitCacheMaxEntries caps the cache: a churning directory set must not grow
// the map unbounded (a cap breach drops the whole cache — a recheck storm is
// bounded by the 2 s exec timeout anyway).
const gitCacheMaxEntries = 1024

type gitCacheEntry struct {
	isRepo bool
	expiry time.Time
}

var gitCache struct {
	mu sync.Mutex
	m  map[string]gitCacheEntry
}

// gitRepo reports whether dir is inside a git work tree. Detection is
// `git -C dir rev-parse --is-inside-work-tree` with a 2s timeout; any failure
// counts as "no". Results are cached per directory, bounded and expiring.
func gitRepo(dir string) bool {
	now := time.Now()
	gitCache.mu.Lock()
	if e, ok := gitCache.m[dir]; ok && now.Before(e.expiry) {
		gitCache.mu.Unlock()
		return e.isRepo
	}
	gitCache.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree").Output()
	v := err == nil && strings.TrimSpace(string(out)) == "true"

	gitCache.mu.Lock()
	if gitCache.m == nil || len(gitCache.m) >= gitCacheMaxEntries {
		gitCache.m = map[string]gitCacheEntry{}
	}
	gitCache.m[dir] = gitCacheEntry{isRepo: v, expiry: now.Add(gitCacheTTL)}
	gitCache.mu.Unlock()
	return v
}

// BuildSystemPrompt returns the ordered system texts: [family, env,
// instructions...]. v1 instruction resolution is the AGENTS.md walk-up
// (nearest wins); config instructions[] are appended by the engine which owns
// the loaded config.
func BuildSystemPrompt(dir string, model provider.Model, apiID, providerID string) ([]string, error) {
	return buildCore(dir, apiID, providerID, nil)
}

func buildCore(dir, apiID, providerID string, instructionPaths []string) ([]string, error) {
	_, famText, err := FamilyPrompt(apiID, providerID)
	if err != nil {
		return nil, err
	}
	out := []string{famText, EnvBlock(dir, apiID, providerID)}
	if ag, ok := nearestAgentsMD(dir); ok {
		out = append(out, ag)
	}
	for _, p := range instructionPaths {
		if b, err := os.ReadFile(p); err == nil {
			out = append(out, string(b))
		}
	}
	return out, nil
}

// nearestAgentsMD walks up from dir (max 32 hops) returning the nearest
// AGENTS.md content.
func nearestAgentsMD(dir string) (string, bool) {
	d := dir
	for i := 0; i < 32; i++ {
		b, err := os.ReadFile(filepath.Join(d, "AGENTS.md"))
		if err == nil {
			return string(b), true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// PlanReminders returns the extra system texts for plan-mode transitions:
// current agent is plan -> plan.txt; build resuming after a plan turn ->
// build-switch.txt.
func PlanReminders(history []protocol.MessageWithParts, currentAgent string) []string {
	switch currentAgent {
	case "plan":
		return []string{planReminder}
	case "build":
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Info.Role == "assistant" && history[i].Info.Agent == "plan" {
				return []string{buildSwitchMsg}
			}
		}
	}
	return nil
}
