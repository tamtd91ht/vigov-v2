package app

import (
	"bytes"
	"io"
)

// uploadBodyFake is the handler's UploadBody (core/httpx.UploadRequest) in memory. `reads` and
// `finished` let a test prove a refusal never touched the body, and that Finish ran before anything was
// recorded. readErr replaces the data with a failure, as a dropped or timed-out client does.
type uploadBodyFake struct {
	r         io.Reader
	readErr   error
	finishErr error
	reads     int
	finished  int
}

func uploadBodyOf(data []byte) *uploadBodyFake { return &uploadBodyFake{r: bytes.NewReader(data)} }

func (b *uploadBodyFake) Read(p []byte) (int, error) {
	b.reads++
	if b.readErr != nil {
		return 0, b.readErr
	}
	return b.r.Read(p)
}

func (b *uploadBodyFake) Finish() error {
	b.finished++
	return b.finishErr
}
