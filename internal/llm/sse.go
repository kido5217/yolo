package llm

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// sseLoop is the shared SSE data-frame pump (refactor-4): the byte-based
// line loop both drivers carried verbatim. process receives each
// blank-line-delimited frame's trimmed "data:" values joined with '\n'
// (joinDataLines); onErr receives the first non-EOF read error; when
// done() reports true (the driver's finish already ran — [DONE] /
// message_stop) the loop stops early so a body that never terminates
// does not hold the engine round hostage; flushTail controls whether a
// partial frame at stream end is processed before finish() runs exactly
// once.
func sseLoop(body io.ReadCloser, process func(payload []byte), onErr func(error), flushTail bool, done func() bool, finish func()) {
	// Byte-based line reading: the payload is assembled as []byte and
	// handed to json.Unmarshal directly, with the same parse semantics as
	// the string join (per-value trim, multi-data join with '\n').
	rd := bufio.NewReader(body)
	var dataVals [][]byte
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			switch {
			case len(line) == 0:
				if len(dataVals) > 0 {
					process(joinDataLines(dataVals))
					dataVals = nil
				}
			case bytes.HasPrefix(line, sseDataPrefix):
				dataVals = append(dataVals, bytes.TrimSpace(line[len(sseDataPrefix):]))
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				onErr(err)
			}
			break
		}
		// [DONE] already ended the stream: stop reading so a body that
		// never terminates does not hold the engine round hostage
		// (anReadSSE parity).
		if done() {
			break
		}
	}
	if flushTail && len(dataVals) > 0 && !done() {
		process(joinDataLines(dataVals))
	}
	finish()
}

// sseDataPrefix is the SSE "data:" field marker.
var sseDataPrefix = []byte("data:")

// joinDataLines joins the trimmed "data:" values of one SSE frame with
// '\n' (multi-line data fields are valid SSE).
func joinDataLines(vals [][]byte) []byte {
	n := len(vals) - 1
	for _, v := range vals {
		n += len(v)
	}
	out := make([]byte, 0, n)
	for i, v := range vals {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, v...)
	}
	return out
}
