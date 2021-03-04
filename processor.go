package nbhttp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// Processor .
type Processor interface {
	Method(method string)
	URL(uri string) error
	Proto(proto string) error
	Status(status string)
	Header(key, value string)
	ContentLength(contentLength int)
	Body([]byte)
	TrailerHeader(key, value string)
	Complete(addr string)
}

// ServerProcessor .
type ServerProcessor struct {
	request *http.Request
	handler http.Handler
}

// Method .
func (p *ServerProcessor) Method(method string) {
	if p.request == nil {
		p.request = &http.Request{
			Method: method,
			Header: http.Header{},
		}
	} else {
		p.request.Method = method
	}
}

// URL .
func (p *ServerProcessor) URL(uri string) error {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return err
	}
	p.request.URL = u
	p.request.RequestURI = uri
	return nil
}

// Proto .
func (p *ServerProcessor) Proto(proto string) error {
	protoMajor, protoMinor, ok := http.ParseHTTPVersion(proto)
	if !ok {
		return fmt.Errorf("%s %q", "malformed HTTP version", proto)
	}
	p.request.Proto = proto
	p.request.ProtoMajor = protoMajor
	p.request.ProtoMinor = protoMinor
	return nil
}

// Status .
func (p *ServerProcessor) Status(status string) {

}

// Header .
func (p *ServerProcessor) Header(key, value string) {
	p.request.Header.Add(key, value)
}

// ContentLength .
func (p *ServerProcessor) ContentLength(contentLength int) {
	p.request.ContentLength = int64(contentLength)
}

// Body .
func (p *ServerProcessor) Body(data []byte) {
	if p.request.Body == nil {
		p.request.Body = &BodyReader{buffer: data}
	} else {
		br := p.request.Body.(*BodyReader)
		br.buffer = append(br.buffer, data...)
	}
}

// TrailerHeader .
func (p *ServerProcessor) TrailerHeader(key, value string) {
	if p.request.Trailer == nil {
		p.request.Trailer = http.Header{}
	}
	p.request.Trailer.Add(key, value)
}

// Complete .
func (p *ServerProcessor) Complete(addr string) {
	if p.request.URL.Host == "" {
		p.request.URL.Host = p.request.Header.Get("Host")
	}
	p.request.RemoteAddr = addr
	p.handler.ServeHTTP(nil, p.request)
	p.request = nil
}

// HandleMessage .
func (p *ServerProcessor) HandleMessage(handler http.Handler) {
	if handler != nil {
		p.handler = handler
	}
}

// NewServerProcessor .
func NewServerProcessor(handler http.Handler) Processor {
	if handler == nil {
		panic(errors.New("invalid handler for ServerProcessor: nil"))
	}
	return &ServerProcessor{
		handler: handler,
	}
}

// EmptyProcessor .
type EmptyProcessor struct{}

// Method .
func (p *EmptyProcessor) Method(method string) {

}

// URL .
func (p *EmptyProcessor) URL(uri string) error {
	return nil
}

// Proto .
func (p *EmptyProcessor) Proto(proto string) error {
	return nil
}

// Status .
func (p *EmptyProcessor) Status(status string) {

}

// Header .
func (p *EmptyProcessor) Header(key, value string) {

}

// ContentLength .
func (p *EmptyProcessor) ContentLength(contentLength int) {

}

// Body .
func (p *EmptyProcessor) Body(data []byte) {

}

// TrailerHeader .
func (p *EmptyProcessor) TrailerHeader(key, value string) {

}

// Complete .
func (p *EmptyProcessor) Complete(addr string) {

}

// HandleMessage .
func (p *EmptyProcessor) HandleMessage(handler http.Handler) {

}

// NewEmptyProcessor .
func NewEmptyProcessor() Processor {
	return &EmptyProcessor{}
}
