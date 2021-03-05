package nbhttp

import (
	"io"
	"net/http"
)

// type ResponseWriter interface {
// 	Header() http.Header
// 	Write([]byte) (int, error)
// 	WriteHeader(statusCode int)
// }

// Response represents the server side of an HTTP response.
// todo:
type Response struct {
	writer    io.Writer
	processor Processor

	sequence uint64
	request  *http.Request // request for this response

	statusCode int // status code passed to WriteHeader
	status     string
	header     http.Header
	trailers   http.Header

	body []byte
}

// Header .
func (response *Response) Header() http.Header {
	return response.header
}

// Write .
func (response *Response) Write(data []byte) (int, error) {
	response.WriteHeader(http.StatusOK)
	if len(data) > 0 {
		response.body = append(response.body, data...)
	}
	return response.processor.WriteTo(response.writer, data)
}

// WriteHeader .
func (response *Response) WriteHeader(statusCode int) {
	if response.statusCode == 0 {
		response.status = http.StatusText(statusCode)
		if response.status != "" {
			response.statusCode = statusCode
		}
	}
}
