package nbhttp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// Processor .
type Processor interface {
	OnMethod(method string)
	OnURL(uri string) error
	OnProto(proto string) error
	OnStatus(code int, status string)
	OnHeader(key, value string)
	OnContentLength(contentLength int)
	OnBody([]byte)
	OnTrailerHeader(key, value string)
	OnComplete(addr string)
}

// ServerProcessor .
type ServerProcessor struct {
	request *http.Request
	handler http.Handler
}

// OnMethod .
func (p *ServerProcessor) OnMethod(method string) {
	if p.request == nil {
		p.request = &http.Request{
			Method: method,
			Header: http.Header{},
		}
	} else {
		p.request.Method = method
	}
}

// OnURL .
func (p *ServerProcessor) OnURL(uri string) error {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return err
	}
	p.request.URL = u
	p.request.RequestURI = uri
	return nil
}

// OnProto .
func (p *ServerProcessor) OnProto(proto string) error {
	protoMajor, protoMinor, ok := http.ParseHTTPVersion(proto)
	if !ok {
		return fmt.Errorf("%s %q", "malformed HTTP version", proto)
	}
	p.request.Proto = proto
	p.request.ProtoMajor = protoMajor
	p.request.ProtoMinor = protoMinor
	return nil
}

// OnStatus .
func (p *ServerProcessor) OnStatus(code int, status string) {

}

// OnHeader .
func (p *ServerProcessor) OnHeader(key, value string) {
	p.request.Header.Add(key, value)
}

// OnContentLength .
func (p *ServerProcessor) OnContentLength(contentLength int) {
	p.request.ContentLength = int64(contentLength)
}

// OnBody .
func (p *ServerProcessor) OnBody(data []byte) {
	if p.request.Body == nil {
		p.request.Body = &BodyReader{buffer: data}
	} else {
		br := p.request.Body.(*BodyReader)
		br.buffer = append(br.buffer, data...)
	}
}

// OnTrailerHeader .
func (p *ServerProcessor) OnTrailerHeader(key, value string) {
	if p.request.Trailer == nil {
		p.request.Trailer = http.Header{}
	}
	p.request.Trailer.Add(key, value)
}

// OnComplete .
func (p *ServerProcessor) OnComplete(addr string) {
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

// ClientProcessor .
type ClientProcessor struct {
	response *http.Response
	handler  func(*http.Response)
}

// OnMethod .
func (p *ClientProcessor) OnMethod(method string) {
}

// OnURL .
func (p *ClientProcessor) OnURL(uri string) error {
	return nil
}

// OnProto .
func (p *ClientProcessor) OnProto(proto string) error {
	protoMajor, protoMinor, ok := http.ParseHTTPVersion(proto)
	if !ok {
		return fmt.Errorf("%s %q", "malformed HTTP version", proto)
	}
	if p.response == nil {
		p.response = &http.Response{
			Proto:  proto,
			Header: http.Header{},
		}
	} else {
		p.response.Proto = proto
	}
	p.response.ProtoMajor = protoMajor
	p.response.ProtoMinor = protoMinor
	return nil
}

// OnStatus .
func (p *ClientProcessor) OnStatus(code int, status string) {
	p.response.StatusCode = code
	p.response.Status = status
}

// OnHeader .
func (p *ClientProcessor) OnHeader(key, value string) {
	p.response.Header.Add(key, value)
}

// OnContentLength .
func (p *ClientProcessor) OnContentLength(contentLength int) {
	p.response.ContentLength = int64(contentLength)
}

// OnBody .
func (p *ClientProcessor) OnBody(data []byte) {
	if p.response.Body == nil {
		p.response.Body = &BodyReader{buffer: data}
	} else {
		br := p.response.Body.(*BodyReader)
		br.buffer = append(br.buffer, data...)
	}
}

// OnTrailerHeader .
func (p *ClientProcessor) OnTrailerHeader(key, value string) {
	if p.response.Trailer == nil {
		p.response.Trailer = http.Header{}
	}
	p.response.Trailer.Add(key, value)
}

// OnComplete .
func (p *ClientProcessor) OnComplete(addr string) {
	p.handler(p.response)
	p.response = nil
}

// HandleMessage .
func (p *ClientProcessor) HandleMessage(handler func(*http.Response)) {
	if handler != nil {
		p.handler = handler
	}
}

// NewClientProcessor .
func NewClientProcessor(handler func(*http.Response)) Processor {
	if handler == nil {
		panic(errors.New("invalid handler for ClientProcessor: nil"))
	}
	return &ClientProcessor{
		handler: handler,
	}
}

// EmptyProcessor .
type EmptyProcessor struct{}

// OnMethod .
func (p *EmptyProcessor) OnMethod(method string) {

}

// OnURL .
func (p *EmptyProcessor) OnURL(uri string) error {
	return nil
}

// OnProto .
func (p *EmptyProcessor) OnProto(proto string) error {
	return nil
}

// OnStatus .
func (p *EmptyProcessor) OnStatus(code int, status string) {

}

// OnHeader .
func (p *EmptyProcessor) OnHeader(key, value string) {

}

// OnContentLength .
func (p *EmptyProcessor) OnContentLength(contentLength int) {

}

// OnBody .
func (p *EmptyProcessor) OnBody(data []byte) {

}

// OnTrailerHeader .
func (p *EmptyProcessor) OnTrailerHeader(key, value string) {

}

// OnComplete .
func (p *EmptyProcessor) OnComplete(addr string) {

}

// HandleMessage .
func (p *EmptyProcessor) HandleMessage(handler http.Handler) {

}

// NewEmptyProcessor .
func NewEmptyProcessor() Processor {
	return &EmptyProcessor{}
}
