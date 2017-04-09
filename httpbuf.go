package httpbuf

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
)

// Buffer is a bytes.Buffer wrapper for easy reading of http.Request and http.Response bodies.
type Buffer struct {
	*bytes.Buffer
	limit int64
}

// New creates Buffer without limitation on request/response body size.
func New(buf *bytes.Buffer) Buffer {
	return Buffer{buf, -1}
}

// NewLimited creates Buffer that returns EOF after reading n bytes.
func NewLimited(buf *bytes.Buffer, n int64) Buffer {
	if n < 0 {
		n = 0
	}
	return Buffer{buf, n}
}

func limited(r io.Reader, n int64) io.Reader {
	if n >= 0 {
		return io.LimitReader(r, n)
	}
	return r
}

// ReadRequest reads http.Request body to Buffer's underlying buffer,
// closes the original body and replaces it with wrapper around read bytes
// so it can be re-read by another middleware or handler.
func (b Buffer) ReadRequest(r *http.Request) error {
	b.Buffer.Reset()
	defer r.Body.Close()
	_, err := b.Buffer.ReadFrom(limited(r.Body, b.limit))
	if err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(b.Buffer.Bytes()))
	return nil
}

// ReadResponse reads http.Response body to Buffer's underlying buffer,
// closes the original body and replaces it with wrapper around read bytes
// so it can be re-read by another middleware or handler.
func (b Buffer) ReadResponse(r *http.Response) error {
	b.Buffer.Reset()
	defer r.Body.Close()
	_, err := b.Buffer.ReadFrom(r.Body)
	if err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(b.Buffer.Bytes()))
	return nil
}

// ReadDo accepts result of http.Client.Do and similar methods,
// takes care about closing response body
// and proceeds with Buffer.ReadResponse in case of nil error.
func (b Buffer) ReadDo(r *http.Response, err error) (*http.Response, error) {
	if err != nil {
		if r != nil && r.Body != nil {
			r.Body.Close()
		}
		return r, err
	}
	return r, b.ReadResponse(r)
}
