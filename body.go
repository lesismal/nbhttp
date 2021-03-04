package nbhttp

import (
	"io"
)

// BodyReader .
type BodyReader struct {
	buffer []byte
}

// Read implements io.Reader
func (br *BodyReader) Read(p []byte) (int, error) {
	n := len(p)
	if len(br.buffer) >= n {
		copy(p, br.buffer[:n])
		br.buffer = br.buffer[n:]
		return n, nil
	}
	n = len(br.buffer)
	if n > 0 {
		copy(p, br.buffer[:n])
		br.buffer = br.buffer[n:]
	}
	return n, io.EOF
}

// Close implements io. Closer
func (br *BodyReader) Close() error {
	br.buffer = nil
	return nil
}
