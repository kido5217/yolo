package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

// benchAnStream builds an Anthropic Messages SSE stream: nText text deltas,
// one tool_use block streaming nTool input_json fragments, and the
// message_start / content_block / message_delta / message_stop envelope.
func benchAnStream(nText, nTool int) []byte {
	var b strings.Builder
	ev := func(event, data string) {
		b.WriteString("event: " + event + "\ndata: " + data + "\n\n")
	}
	b.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_bench\",\"usage\":{\"input_tokens\":7}}}\n\n")
	b.WriteString(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`)
	for i := 0; i < nText; i++ {
		b.WriteString(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"tok "}}

`)
	}
	b.WriteString(`event: content_block_stop
data: {"type":"content_block_stop","index":0}

`)
	b.WriteString(`event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"bash","input":{}}}

`)
	for i := 0; i < nTool; i++ {
		fmt.Fprintf(&b, `event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"fragment %d "}}

`, i)
	}
	ev("content_block_stop", `{"type":"content_block_stop","index":1}`)
	b.WriteString(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":100}}

`)
	b.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	return []byte(b.String())
}

// BenchmarkAnReadSSE measures the per-token SSE decode pipeline (anthropic.go
// anReadSSE): event + data line scan, frame assembly, json.Unmarshal into
// the typed event, and the part-channel hand-off that the engine drains.
func BenchmarkAnReadSSE(b *testing.B) {
	d := &Anthropic{}
	for _, c := range []struct {
		text, tool int
	}{
		{900, 100},
		{9000, 1000},
	} {
		b.Run(fmt.Sprintf("deltas/%d", c.text+c.tool), func(b *testing.B) {
			payload := benchAnStream(c.text, c.tool)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ch := make(chan Part, 64)
				go d.anReadSSE(context.Background(), io.NopCloser(bytes.NewReader(payload)), ch)
				parts := 0
				for range ch {
					parts++
				}
				if parts < c.text {
					b.Fatalf("decoded %d parts, want >= %d", parts, c.text)
				}
			}
		})
	}
}
