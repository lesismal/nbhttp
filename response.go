package nbhttp

import (
	"net"
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
	conn net.Conn
	req  *http.Request // request for this response

	status   int // status code passed to WriteHeader
	header   http.Header
	trailers http.Header

	bodyBuffers [][]byte
}
