package fake_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
)

func userReq(content string) llm.Request {
	return llm.Request{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Content: content}}}
}

func cancelAfter(t *testing.T, cancel context.CancelFunc, d time.Duration) {
	t.Helper()
	g := make(chan struct{})
	go func() {
		defer close(g)
		time.Sleep(d)
		cancel()
	}()
	t.Cleanup(func() { <-g })
}

func drain(t *testing.T, s llm.PartStream) []llm.Part {
	t.Helper()
	var out []llm.Part
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	for {
		p, err := s.Next(ctx)
		if err != nil {
			t.Fatalf("drain: %v (parts so far: %+v)", err, out)
		}
		out = append(out, p)
		if p.Finish != "" {
			break
		}
	}
	return out
}

func TestScriptedTurnsOrder(t *testing.T) {
	t.Parallel()
	d := fake.New(
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "one", Finish: "stop"}}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "two", Finish: "stop"}}},
	)
	got := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		s, err := d.Stream(context.Background(), userReq("q"))
		if err != nil {
			t.Fatal(err)
		}
		parts := drain(t, s)
		if len(parts) != 1 || parts[0].Finish != "stop" {
			t.Fatalf("turn %d = %+v", i+1, parts)
		}
		got = append(got, parts[0].Text)
	}
	if got[0] != "one" || got[1] != "two" {
		t.Fatalf("order = %v, want [one two]", got)
	}
	if reqs := d.Requests(); len(reqs) != 2 {
		t.Fatalf("requests = %d", len(reqs))
	}
}

func TestTitleTurnsRouting(t *testing.T) {
	t.Parallel()
	d := fake.New(
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "chat", Finish: "stop"}}},
	)
	d.TitleTurns = append(d.TitleTurns, fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "titled", Finish: "stop"}}})

	title := llm.Request{
		Model:    "m",
		Messages: []llm.Message{{Role: llm.RoleSystem, Content: "You are a title generator. Write a short title."}},
	}
	s, err := d.Stream(context.Background(), title)
	if err != nil {
		t.Fatal(err)
	}
	if parts := drain(t, s); parts[0].Text != "titled" {
		t.Fatalf("title turn = %+v", parts)
	}
	// the chat Turns track is untouched: the next non-title request still
	// draws from it.
	s2, err := d.Stream(context.Background(), userReq("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if parts := drain(t, s2); parts[0].Text != "chat" {
		t.Fatalf("chat turn = %+v", parts)
	}
	// TitleTurns exhausted and no AutoText: next title request is an
	// immediate empty stream (io.EOF, no parts).
	s3, err := d.Stream(context.Background(), title)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := s3.Next(context.Background()); err != io.EOF || p.Text != "" {
		t.Fatalf("exhausted title route = %+v %v, want zero part + io.EOF", p, err)
	}
	if reqs := d.Requests(); len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
}

func TestAutoTextSynthesizesNumbered(t *testing.T) {
	t.Parallel()
	d := fake.New(fake.AutoText())
	want := []string{"ok-1", "ok-2", "ok-3"}
	for _, w := range want {
		s, err := d.Stream(context.Background(), userReq("q"))
		if err != nil {
			t.Fatal(err)
		}
		parts := drain(t, s)
		if len(parts) != 1 || parts[0].Kind != "text" || parts[0].Finish != "stop" || parts[0].Text != w {
			t.Fatalf("auto parts = %+v, want %q", parts, w)
		}
		if u := parts[0].Usage; u == nil || u.Input != 1 || u.Output != 1 {
			t.Fatalf("auto usage = %+v", u)
		}
	}
}

func TestErrTurn(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	d := fake.New(fake.Turn{Err: boom})
	s, err := d.Stream(context.Background(), userReq("q"))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if s.Parts != nil {
		t.Fatal("want zero stream on Err turn")
	}
}

func TestDelayHeldUntilCanceled(t *testing.T) {
	t.Parallel()
	d := fake.New(fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "late", Finish: "stop"}}, Delay: 2 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancelAfter(t, cancel, 50*time.Millisecond)
	s, err := d.Stream(ctx, userReq("q"))
	if err != nil {
		t.Fatal(err)
	}
	parts := drain(t, s)
	if len(parts) != 1 || parts[0].Finish != "error" || parts[0].Text != "" {
		t.Fatalf("parts = %+v", parts)
	}
	if !errors.Is(parts[0].Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", parts[0].Err)
	}
}

func TestSetDelayAppliesToSynthesizedReplies(t *testing.T) {
	t.Parallel()
	d := fake.New(fake.AutoText())
	d.SetDelay(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancelAfter(t, cancel, 50*time.Millisecond)
	s, err := d.Stream(ctx, userReq("q"))
	if err != nil {
		t.Fatal(err)
	}
	parts := drain(t, s)
	if len(parts) != 1 || parts[0].Finish != "error" {
		t.Fatalf("parts = %+v", parts)
	}
	if !errors.Is(parts[0].Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", parts[0].Err)
	}
}

func TestRequestsCopyIsDetached(t *testing.T) {
	t.Parallel()
	d := fake.New()
	if _, err := d.Stream(context.Background(), userReq("q")); err != nil {
		t.Fatal(err)
	}
	one := d.Requests()
	if len(one) != 1 {
		t.Fatalf("requests = %d", len(one))
	}
	one[0].Model = "mutated"
	if got := d.Requests(); got[0].Model != "m" {
		t.Fatalf("Requests() must return a copy, got %q", got[0].Model)
	}
}

func TestFromScript(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "script.json")
	script := `[
		{"parts":[{"kind":"text","text":"hi","finish":"stop","usage":{"input":2,"output":3}}],"delay_ms":0},
		{"parts":[{"kind":"tool","name":"read","call_id":"c1","args":{"filePath":"/x"},"text":"{\"filePath\":\"/x\"}","finish":"tool_calls"}],"delay_ms":5}
	]`
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := fake.FromScript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Turns) != 2 {
		t.Fatalf("turns = %d", len(d.Turns))
	}
	t0 := d.Turns[0]
	if len(t0.Parts) != 1 || t0.Parts[0].Text != "hi" || t0.Parts[0].Finish != "stop" ||
		t0.Parts[0].Usage == nil || t0.Parts[0].Usage.Input != 2 || t0.Parts[0].Usage.Output != 3 {
		t.Fatalf("turn 0 = %+v", t0)
	}
	t1 := d.Turns[1]
	if len(t1.Parts) != 1 || t1.Delay != 5*time.Millisecond {
		t.Fatalf("turn 1 = %+v", t1)
	}
	p := t1.Parts[0]
	if p.Kind != "tool" || p.Name != "read" || p.CallID != "c1" || string(p.Args) != `{"filePath":"/x"}` || p.Finish != "tool_calls" {
		t.Fatalf("turn 1 part = %+v (args %s)", p, p.Args)
	}
	// the loaded driver actually serves the parsed turns
	s, err := d.Stream(context.Background(), userReq("q"))
	if err != nil {
		t.Fatal(err)
	}
	if parts := drain(t, s); parts[0].Text != "hi" {
		t.Fatalf("served = %+v", parts)
	}
}

func TestFromScriptBadJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"not":"a list"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.FromScript(path); err == nil {
		t.Fatal("want error for non-list script")
	}
}

func TestFromScriptMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := fake.FromScript(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("want error for missing script")
	}
}

func TestParallelStreamsRecordEveryRequest(t *testing.T) {
	t.Parallel()
	d := fake.New(fake.AutoText())
	const n = 10
	var wg sync.WaitGroup
	texts := make([]llm.Part, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := d.Stream(context.Background(), userReq("q"))
			if err != nil {
				errs[i] = err
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			p, err := s.Next(ctx)
			if err != nil {
				errs[i] = err
				return
			}
			texts[i] = p
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("goroutine %d: %v", i, e)
		}
	}
	if reqs := d.Requests(); len(reqs) != n {
		t.Fatalf("requests = %d, want %d", len(reqs), n)
	}
	got := make([]string, n)
	for i, p := range texts {
		if p.Finish != "stop" {
			t.Fatalf("part %d = %+v", i, p)
		}
		got[i] = p.Text
	}
	sort.Strings(got)
	for i, w := range []string{"ok-1", "ok-10", "ok-2", "ok-3", "ok-4", "ok-5", "ok-6", "ok-7", "ok-8", "ok-9"} {
		if got[i] != w {
			t.Fatalf("synthesized set = %v", got)
		}
	}
}
