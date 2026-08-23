package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kido5217/yolo/internal/protocol"
)

//go:embed desc/todowrite.txt
var todoWriteDesc string

type todoWriteTool struct{}

var _ Tool = todoWriteTool{}

func (todoWriteTool) ID() string         { return "todowrite" }
func (todoWriteTool) Permission() string { return "todowrite" }
func (todoWriteTool) Desc() string       { return todoWriteDesc }

var (
	todoStatuses   = map[string]bool{"pending": true, "in_progress": true, "completed": true, "cancelled": true}
	todoPriorities = map[string]bool{"high": true, "medium": true, "low": true}
)

func (todoWriteTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"type":        "array",
				"description": "The updated todo list",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
							"type":        "string",
							"description": "Brief description of the task",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
							"description": "Current status of the task: pending, in_progress, completed, cancelled",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"high", "medium", "low"},
							"description": "Priority of the task: high, medium, or low",
						},
					},
					"required": []string{"content"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

// Patterns: upstream asks permission "todowrite" with patterns ["*"] and
// always ["*"].
func (todoWriteTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	return []string{"*"}, []string{"*"}, nil
}

func (todoWriteTool) External(raw json.RawMessage) ([]string, error) {
	return nil, nil
}

func (todoWriteTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	var in struct {
		Todos []struct {
			Content  string `json:"content"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
		} `json:"todos"`
	}
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Output{}, err
	}
	if in.Todos == nil {
		return Output{}, errors.New("todos is required")
	}
	todos := make([]protocol.Todo, 0, len(in.Todos))
	open := 0
	for _, it := range in.Todos {
		if it.Content == "" {
			return Output{}, errors.New("content is required")
		}
		if !todoStatuses[it.Status] {
			return Output{}, fmt.Errorf("invalid status: %s", it.Status)
		}
		p := it.Priority
		if p == "" {
			p = "medium"
		}
		if !todoPriorities[p] {
			return Output{}, fmt.Errorf("invalid priority: %s", p)
		}
		if it.Status != "completed" {
			open++
		}
		todos = append(todos, protocol.Todo{Content: it.Content, Status: it.Status, Priority: p})
	}
	if env == nil {
		env = &Env{}
	}
	if env.Storage != nil && env.SessionID != "" {
		if err := env.Storage.SaveTodos(ctx, env.SessionID, todos); err != nil {
			return Output{}, err
		}
	}
	text, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return Output{}, err
	}
	return Output{
		Title: fmt.Sprintf("%d todos", open),
		Text:  string(text),
		Meta:  map[string]any{"todos": todos},
	}, nil
}
