package protocol

// AttachFileMaxBytes is the cap on a single attached file (the 10 MiB
// file bound, spec §4.2). Shared by the run client (file validation) and
// the server (decoded re-cap) so the two cannot drift.
const AttachFileMaxBytes = 10 << 20

// FileRef is one attached file of a send request (a client-validated
// data URL, spec §3).
type FileRef struct {
	MIME     string `json:"mime"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// SendMessageRequest is the POST /session/{id}/message body: text is
// required; files is omitted when empty (wire compatibility — a
// no-files body is byte-identical to today's).
type SendMessageRequest struct {
	Text  string    `json:"text"`
	Files []FileRef `json:"files,omitempty"`
}
