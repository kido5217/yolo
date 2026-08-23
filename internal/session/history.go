package session

import (
	"encoding/json"
	"strings"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/protocol"
)

func (e *Engine) messagesFor(t *turn) ([]llm.Message, error) {
	out := make([]llm.Message, 0, len(t.sys)+8)
	for _, s := range t.sys {
		out = append(out, llm.Message{Role: llm.RoleSystem, Content: s})
	}
	lastUserIdx := -1
	for i := range t.hist {
		if t.hist[i].Info.Role == "user" {
			lastUserIdx = i
		}
	}
	reminders := PlanReminders(t.hist, t.agent)
	for i, mw := range t.hist {
		switch mw.Info.Role {
		case "user":
			content := joinTextParts(mw.Parts)
			if i == lastUserIdx {
				content = appendReminders(content, reminders)
			}
			out = append(out, llm.Message{Role: llm.RoleUser, Content: content})
		case "assistant":
			var texts []string
			var calls []llm.ToolCall
			var toolMsgs []llm.Message
			for _, p := range mw.Parts {
				switch {
				case p.Type == "text" && p.Text != "":
					texts = append(texts, p.Text)
				case p.Type == "tool" && p.State != nil &&
					(p.State.Status == "completed" || p.State.Status == "error"):
					args, err := json.Marshal(p.State.Input)
					if err != nil || len(args) == 0 {
						args = json.RawMessage("{}")
					}
					calls = append(calls, llm.ToolCall{ID: p.ID, Name: p.Tool, Args: args})
					content := p.State.Output
					if p.State.Status == "error" {
						content = p.State.Error
					}
					toolMsgs = append(toolMsgs, llm.Message{Role: llm.RoleTool, ToolCallID: p.ID, Content: content})
				}
			}
			if len(texts) == 0 && len(calls) == 0 {
				continue
			}
			out = append(out, llm.Message{Role: llm.RoleAssistant, Content: strings.Join(texts, "\n"), ToolCalls: calls})
			if len(toolMsgs) > 0 {
				out = append(out, toolMsgs...)
			}
		}
	}
	return out, nil
}

func joinTextParts(parts []protocol.Part) string {
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func appendReminders(content string, reminders []string) string {
	if len(reminders) == 0 {
		return content
	}
	blocks := strings.Join(reminders, "\n\n")
	if content == "" {
		return blocks
	}
	return content + "\n\n" + blocks
}

// isSyntheticPart reports whether a part is engine-generated and excluded
// from history replay.
func isSyntheticPart(p protocol.Part) bool {
	return p.IsSynthetic != nil && *p.IsSynthetic
}
