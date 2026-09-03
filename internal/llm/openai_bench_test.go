package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

// benchOAStream builds an OpenAI chat/completions SSE stream: nText content
// deltas, two tool calls (id + name announced once, arguments streamed in
// 48 fragments each), a finish frame, the usage frame and [DONE].
func benchOAStream(nText int) []byte {
	var b strings.Builder
	for i := 0; i < nText; i++ {
		b.WriteString(`data: {"choices":[{"index":0,"delta":{"content":"tok "}}]}` + "\n\n")
	}
	const frag = 48
	nameFrame := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_0",` +
		`"function":{"name":"bash","arguments":"{\"command\":\"ls -la \"}}]}}]}` + "\n\n"
	fmt.Fprint(&b, nameFrame)
	for i := 0; i < frag; i++ {
		const fragFrame = `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,` +
			`"function":{"arguments":"fragment %d "}}]}}]}`
		fmt.Fprintf(&b, fragFrame+"\n\n", i)
	}
	readFrame := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_1",` +
		`"function":{"name":"read","arguments":"{\"path\":\"/x\""}}]}}]}` + "\n\n"
	fmt.Fprint(&b, readFrame)
	for i := 0; i < frag; i++ {
		const chunkFrame = `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,` +
			`"function":{"arguments":"chunk %d "}}]}}]}`
		fmt.Fprintf(&b, chunkFrame+"\n\n", i)
	}
	b.WriteString(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n")
	usageFrame := `data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":100,` +
		`"completion_tokens_details":{"reasoning_tokens":10},` +
		`"prompt_tokens_details":{"cached_tokens":5}}}` + "\n\n"
	b.WriteString(usageFrame)
	b.WriteString("data: [DONE]\n\n")
	return []byte(b.String())
}

// BenchmarkOAReadSSE measures the per-token SSE decode pipeline (openai.go
// oaReadSSE): byte line scan, frame assembly, json.Unmarshal into the typed
// delta, and the part-channel hand-off that the engine drains.
func BenchmarkOAReadSSE(b *testing.B) {
	d := &OpenAI{}
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("frames/%d", n), func(b *testing.B) {
			payload := benchOAStream(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ch := make(chan Part, 64)
				go d.oaReadSSE(context.Background(), io.NopCloser(bytes.NewReader(payload)), ch)
				parts := 0
				for range ch {
					parts++
				}
				if parts < n {
					b.Fatalf("decoded %d parts, want >= %d", parts, n)
				}
			}
		})
	}
	b.ReportAllocs()
}
