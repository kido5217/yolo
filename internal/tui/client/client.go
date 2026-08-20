// Package client is the TUI's wire-contract client for the core server.
// It depends only on internal/protocol (TUI purity: no server import).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
)

// Sentinels mapped from HTTP statuses; the server's envelope message is
// carried in the wrapped error text.
var (
	ErrNotFound   = errors.New("not found")
	ErrBusy       = errors.New("session busy")
	ErrBadRequest = errors.New("bad request")
)

// Client talks to one core server. Dir is the scope directory (abs); ""
// falls back to the server work dir (header omitted).
type Client struct {
	Base    string
	Dir     string
	HC      *http.Client
	Backoff func(int) time.Duration // SSE reconnect backoff (tests override)
}

// New returns a client for base with scope dir.
func New(base, dir string) *Client {
	return &Client{Base: base, Dir: dir, HC: &http.Client{}}
}

func (c *Client) backoff(n int) time.Duration {
	if c.Backoff != nil {
		return c.Backoff(n)
	}
	// Clamp the shift: for n >= 64 a duration shift yields 0, which would
	// tight-loop reconnects during a long outage.
	d := time.Second << uint(min(n, 5))
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func (c *Client) dirHeader(req *http.Request) {
	if c.Dir != "" {
		req.Header.Set("x-yolo-directory", url.PathEscape(c.Dir))
	}
}

// do performs one request: dir header, JSON body, JSON decode, error mapping.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var rd io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.dirHeader(req)
	resp, err := c.HC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return httpErr(resp.StatusCode, b)
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return nil
}

// httpErr maps non-2xx to sentinel errors (404/409/400) or a status error,
// carrying the server's error envelope message.
func httpErr(code int, b []byte) error {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := http.StatusText(code)
	if json.Unmarshal(b, &env) == nil && env.Error.Message != "" {
		msg = env.Error.Message
	}
	switch code {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, msg)
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrBusy, msg)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, msg)
	default:
		return fmt.Errorf("status %d: %s", code, msg)
	}
}

// PathEscapeID escapes a path segment (generated ids are already safe).
func PathEscapeID(id string) string { return url.PathEscape(id) }

// Health checks GET /global/health.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/global/health", nil, nil)
}

// ListSessions is GET /session.
func (c *Client) ListSessions(ctx context.Context) ([]protocol.Session, error) {
	var out []protocol.Session
	if err := c.do(ctx, http.MethodGet, "/session", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSession is POST /session (201).
func (c *Client) CreateSession(ctx context.Context, title string) (protocol.Session, error) {
	var out protocol.Session
	err := c.do(ctx, http.MethodPost, "/session", map[string]string{"title": title}, &out)
	return out, err
}

// GetSession is GET /session/{id}.
func (c *Client) GetSession(ctx context.Context, id string) (protocol.Session, error) {
	var out protocol.Session
	err := c.do(ctx, http.MethodGet, "/session/"+PathEscapeID(id), nil, &out)
	return out, err
}

// PatchSession is PATCH /session/{id}.
func (c *Client) PatchSession(ctx context.Context, id string, patch map[string]any) (protocol.Session, error) {
	var out protocol.Session
	err := c.do(ctx, http.MethodPatch, "/session/"+PathEscapeID(id), patch, &out)
	return out, err
}

// DeleteSession is DELETE /session/{id} (204).
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/session/"+PathEscapeID(id), nil, nil)
}

// ListMessages is GET /session/{id}/message.
func (c *Client) ListMessages(ctx context.Context, id string) ([]protocol.MessageWithParts, error) {
	var out []protocol.MessageWithParts
	if err := c.do(ctx, http.MethodGet, "/session/"+PathEscapeID(id)+"/message", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SendMessage is POST /session/{id}/message (202); ErrBusy on 409.
func (c *Client) SendMessage(ctx context.Context, id, text string) (string, error) {
	var out struct {
		MessageID string `json:"message_id"`
	}
	if err := c.do(ctx, http.MethodPost, "/session/"+PathEscapeID(id)+"/message", map[string]string{"text": text}, &out); err != nil {
		return "", err
	}
	return out.MessageID, nil
}

// Abort is POST /session/{id}/abort.
func (c *Client) Abort(ctx context.Context, id string) (bool, error) {
	var out struct {
		Aborted bool `json:"aborted"`
	}
	if err := c.do(ctx, http.MethodPost, "/session/"+PathEscapeID(id)+"/abort", nil, &out); err != nil {
		return false, err
	}
	return out.Aborted, nil
}

// Command is POST /session/{id}/command.
func (c *Client) Command(ctx context.Context, id, cmd string) (protocol.CommandResponse, error) {
	var out protocol.CommandResponse
	err := c.do(ctx, http.MethodPost, "/session/"+PathEscapeID(id)+"/command", map[string]string{"command": cmd}, &out)
	return out, err
}

// Status is GET /session/status. The wire carries plain strings
// ("idle"|"busy"|"retry") per session id; the plan's
// map[string]protocol.SessionStatus cannot be decoded from it (deviation).
func (c *Client) Status(ctx context.Context) (map[string]string, error) {
	var out struct {
		Sessions map[string]string `json:"sessions"`
	}
	if err := c.do(ctx, http.MethodGet, "/session/status", nil, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

// ListProviders is GET /provider.
func (c *Client) ListProviders(ctx context.Context) ([]protocol.Provider, error) {
	var out []protocol.Provider
	if err := c.do(ctx, http.MethodGet, "/provider", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetConfig is GET /config.
func (c *Client) GetConfig(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "/config", nil, &out)
	return out, err
}

// PatchConfig is PATCH /config (server deep-merges).
func (c *Client) PatchConfig(ctx context.Context, patch map[string]any) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPatch, "/config", patch, &out)
	return out, err
}

// GlobalConfig is GET /global/config; a non-nil patch makes it a PATCH.
func (c *Client) GlobalConfig(ctx context.Context, patch map[string]any) (map[string]any, error) {
	var out map[string]any
	if patch == nil {
		err := c.do(ctx, http.MethodGet, "/global/config", nil, &out)
		return out, err
	}
	err := c.do(ctx, http.MethodPatch, "/global/config", patch, &out)
	return out, err
}

// Auth is PUT /auth/{providerID} (key) or, with remove, DELETE (204).
func (c *Client) Auth(ctx context.Context, providerID, key string, remove bool) error {
	id := PathEscapeID(providerID)
	if remove {
		return c.do(ctx, http.MethodDelete, "/auth/"+id, nil, nil)
	}
	return c.do(ctx, http.MethodPut, "/auth/"+id, map[string]string{"key": key}, nil)
}

// ListAgents is GET /agent.
func (c *Client) ListAgents(ctx context.Context) ([]protocol.Agent, error) {
	var out []protocol.Agent
	if err := c.do(ctx, http.MethodGet, "/agent", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListCommands is GET /command.
func (c *Client) ListCommands(ctx context.Context) ([]protocol.Command, error) {
	var out []protocol.Command
	if err := c.do(ctx, http.MethodGet, "/command", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPermissions is GET /permission (pending asks for the dir).
func (c *Client) ListPermissions(ctx context.Context) ([]protocol.PermissionAskedProps, error) {
	var out []protocol.PermissionAskedProps
	if err := c.do(ctx, http.MethodGet, "/permission", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReplyPermission is POST /permission/{requestID}/reply (204).
func (c *Client) ReplyPermission(ctx context.Context, requestID, reply string) error {
	return c.do(ctx, http.MethodPost, "/permission/"+PathEscapeID(requestID)+"/reply", map[string]string{"response": reply}, nil)
}
