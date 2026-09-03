package session

import (
	"encoding/json"
	"strings"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/protocol"
)

// messagesFor maps the persisted history onto the LLM request.
//
// LOCKED mapping (plan Task 16):
//   - system prompt entries lead as separate RoleSystem messages;
//   - user messages join their text parts with "\n"; plan reminders attach to
//     the LAST user message;
//   - assistant messages carry text only (reasoning excluded) plus ToolCalls
//     derived from their completed/error tool parts (Args = persisted state
//     input);
//   - every tool part produces one RoleTool message right after its assistant
//     (completed -> output, error -> error text);
//   - empty assistant messages are skipped;
//   - the request mirrors the persisted history 1:1 (upstream
//     message-v2.toModelMessagesEffect): a tool round ends with the TOOL
//     result — the user message is NEVER re-appended (deviation 77: the
//     plan's re-append made the model see its instruction re-issued every
//     round, which looped weak models into re-running tools).
//
// loadHistory builds the turn's system prompts and the full in-memory
// history snapshot once (⑪). messagesFor maps this snapshot; the mapping is
// unchanged (LOCKED).
func mapHistory(hist []protocol.MessageWithParts, agent string, sys []string) []llm.Message {
	// The request mirrors the snapshot 1:1 (plus per-tool result messages),
	// so the known sizes preallocate the common case.
	out := make([]llm.Message, 0, len(sys)+len(hist))
	for _, s := range sys {
		out = append(out, llm.Message{Role: llm.RoleSystem, Content: s})
	}
	lastUserIdx := -1
	for i := range hist {
		if hist[i].Info.Role == "user" {
			lastUserIdx = i
		}
	}
	reminders := PlanReminders(hist, agent)
	for i, mw := range hist {
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
	return out
}

func (e *Engine) messagesFor(t *turn) ([]llm.Message, error) {
	return mapHistory(t.hist, t.agent, t.sys), nil
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
