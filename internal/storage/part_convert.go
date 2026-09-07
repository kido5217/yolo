package storage

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kido5217/yolo/internal/protocol"
)

// nullStr renders "" as SQL NULL for nullable text columns.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullPtr renders nil as SQL NULL for nullable integer columns.
func nullPtr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// ProtocolToPart encodes a wire part into a row. Text/reasoning parts store
// {"text":..., "end":n, "synthetic":true} (end/synthetic omitted when unset);
// file parts store the merged envelope document
// {end?, filename, mime, synthetic?, url} (alphabetical keys); tool parts
// store the full protocol.ToolState JSON. CallID is transient and not
// persisted. A marshal failure (e.g. NaN in a tool state) is an error —
// persisting "" would 500 every later read.
func ProtocolToPart(p protocol.Part) (PartRow, error) {
	r := PartRow{
		ID:          p.ID,
		MessageID:   p.MessageID,
		SessionID:   p.SessionID,
		Type:        p.Type,
		Tool:        p.Tool,
		TimeCreated: p.Time.Start,
	}
	switch {
	case p.State != nil:
		b, err := json.Marshal(p.State)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s state: %w", p.ID, err)
		}
		r.StateJSON = string(b)
	case p.Type == protocol.PartTypeFile:
		// File part (yolo run, spec §4.3): the merged envelope document —
		// file fields plus the end/synthetic envelope shape, keys in
		// alphabetical order (end < filename < mime < synthetic < url),
		// compact separators (the hot-path text convention).
		f, err := json.Marshal(p.Filename)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s filename: %w", p.ID, err)
		}
		m, err := json.Marshal(p.MIME)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s mime: %w", p.ID, err)
		}
		u, err := json.Marshal(p.URL)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s url: %w", p.ID, err)
		}
		b := make([]byte, 0, len(f)+len(m)+len(u)+32)
		b = append(b, '{')
		if p.Time.End != 0 {
			b = append(b, `"end":`...)
			b = strconv.AppendInt(b, p.Time.End, 10)
			b = append(b, ',')
		}
		b = append(b, `"filename":`...)
		b = append(b, f...)
		b = append(b, `,"mime":`...)
		b = append(b, m...)
		if p.IsSynthetic != nil && *p.IsSynthetic {
			b = append(b, `,"synthetic":true`...)
		}
		b = append(b, `,"url":`...)
		b = append(b, u...)
		b = append(b, '}')
		r.StateJSON = string(b)
	default:
		// Hot path (streamed deltas): build the fixed 3-key document
		// directly. Must stay byte-identical to the map marshal: sorted
		// keys (end, synthetic, text), compact separators.
		t, err := json.Marshal(p.Text)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s text: %w", p.ID, err)
		}
		b := make([]byte, 0, len(t)+16)
		b = append(b, '{')
		if p.Time.End != 0 {
			b = append(b, `"end":`...)
			b = strconv.AppendInt(b, p.Time.End, 10)
			b = append(b, ',')
		}
		if p.IsSynthetic != nil && *p.IsSynthetic {
			b = append(b, `"synthetic":true,`...)
		}
		b = append(b, `"text":`...)
		b = append(b, t...)
		b = append(b, '}')
		r.StateJSON = string(b)
	}
	return r, nil
}

// PartToProtocol decodes a row into a wire part (inverse of ProtocolToPart).
func PartToProtocol(r PartRow) (protocol.Part, error) {
	p := protocol.Part{
		ID:        r.ID,
		SessionID: r.SessionID,
		MessageID: r.MessageID,
		Type:      r.Type,
		Tool:      r.Tool,
	}
	switch r.Type {
	case "tool":
		st := &protocol.ToolState{}
		if err := json.Unmarshal([]byte(r.StateJSON), st); err != nil {
			return p, fmt.Errorf("part %s state: %w", r.ID, err)
		}
		p.State = st
	case "file":
		var st struct {
			Filename  string `json:"filename"`
			MIME      string `json:"mime"`
			URL       string `json:"url"`
			End       int64  `json:"end"`
			Synthetic *bool  `json:"synthetic"`
		}
		if err := json.Unmarshal([]byte(r.StateJSON), &st); err != nil {
			return p, fmt.Errorf("part %s state: %w", r.ID, err)
		}
		p.Filename, p.MIME, p.URL = st.Filename, st.MIME, st.URL
		p.Time = protocol.PartTime{Start: r.TimeCreated, End: st.End}
		p.IsSynthetic = st.Synthetic
	default:
		var st struct {
			Text      string `json:"text"`
			End       int64  `json:"end"`
			Synthetic *bool  `json:"synthetic"`
		}
		if err := json.Unmarshal([]byte(r.StateJSON), &st); err != nil {
			return p, fmt.Errorf("part %s state: %w", r.ID, err)
		}
		p.Text = st.Text
		p.Time = protocol.PartTime{Start: r.TimeCreated, End: st.End}
		p.IsSynthetic = st.Synthetic
	}
	return p, nil
}

// SessionFromRow assembles the wire session, recomputing cost/tokens as the
// sum over assistant messages (session.cost column is ignored by design).
func SessionFromRow(r SessionRow, msgs []MessageRow) protocol.Session {
	var cost float64
	var tok protocol.Tokens
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		cost += m.Cost
		tok.Input += m.Tokens.Input
		tok.Output += m.Tokens.Output
		tok.Reasoning += m.Tokens.Reasoning
		tok.Cache.Read += m.Tokens.Cache.Read
		tok.Cache.Write += m.Tokens.Cache.Write
	}
	return protocol.Session{
		ID:        r.ID,
		ProjectID: projectIDFromDir(r.ProjectDir),
		Directory: r.ProjectDir,
		Title:     r.Title,
		Agent:     r.Agent,
		Model:     modelRefFromString(r.Model),
		Cost:      cost,
		Tokens:    tok,
		Version:   "yolo",
		Time:      protocol.SessionTime{Created: r.TimeCreated, Updated: r.TimeUpdated},
	}
}
