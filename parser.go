package nbhttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

var (

	// ErrInvalidCRLF .
	ErrInvalidCRLF = errors.New("invalid cr/lf at the end of line")

	// ErrInvalidHTTPVersion .
	ErrInvalidHTTPVersion = errors.New("invalid HTTP version")

	// ErrInvalidHTTPStatusCode .
	ErrInvalidHTTPStatusCode = errors.New("invalid HTTP status code")

	// ErrInvalidMethod .
	ErrInvalidMethod = errors.New("invalid HTTP method")

	// ErrInvalidRequestURI .
	ErrInvalidRequestURI = errors.New("invalid URL")

	// ErrInvalidHost .
	ErrInvalidHost = errors.New("invalid host")

	// ErrInvalidPort .
	ErrInvalidPort = errors.New("invalid port")

	// ErrInvalidPath .
	ErrInvalidPath = errors.New("invalid path")

	// ErrInvalidQueryString .
	ErrInvalidQueryString = errors.New("invalid query string")

	// ErrInvalidFragment .
	ErrInvalidFragment = errors.New("invalid fragment")

	// ErrCRExpected .
	ErrCRExpected = errors.New("CR character expected")

	// ErrLFExpected .
	ErrLFExpected = errors.New("LF character expected")

	// ErrInvalidCharInHeader .
	ErrInvalidCharInHeader = errors.New("invalid character in header")

	// ErrUnexpectedContentLength .
	ErrUnexpectedContentLength = errors.New("unexpected content-length header")

	// ErrInvalidContentLength .
	ErrInvalidContentLength = errors.New("invalid ContentLength")

	// ErrInvalidChunkSize .
	ErrInvalidChunkSize = errors.New("invalid chunk size")

	// ErrTrailerExpected .
	ErrTrailerExpected = errors.New("trailer expected")
)

const (
	// state: RequestLine
	stateInit int8 = iota
	stateMethodBefore
	stateMethod

	statePathBefore
	statePath
	stateProtoBefore
	stateProto
	stateProtoLF
	stateStatusBefore
	stateStatus

	// state: Header
	stateHeaderKeyBefore
	stateHeaderValueLF
	stateHeaderKey

	stateHeaderValueBefore
	stateHeaderValue

	// state: Body ContentLength
	stateBodyContentLength

	// state: Body Chunk
	stateHeaderOverLF
	stateBodyChunkSizeBlankLine
	stateBodyChunkSizeBefore
	stateBodyChunkSize
	stateBodyChunkSizeLF
	stateBodyChunkData
	stateBodyChunkDataCR
	stateBodyChunkDataLF

	// state: Body Trailer
	stateBodyTrailerHeaderValueLF
	stateBodyTrailerHeaderKeyBefore
	stateBodyTrailerHeaderKey
	stateBodyTrailerHeaderValueBefore
	stateBodyTrailerHeaderValue

	// state: Body CRLF
	stateTailCR
	stateTailLF
)

// Parser .
type Parser struct {
	conn net.Conn

	state int8

	cache []byte

	proto       string
	headerKey   string
	headerValue string

	chunkSize     int
	header        http.Header
	chunked       bool
	contentLength int
	trailer       http.Header
	// todo
	readLimit   int
	maxReadSize int
	isClient    bool

	processor Processor

	session interface{}
}

func (p *Parser) nextState(state int8) {
	p.state = state
}

// ReadRequest .
func (p *Parser) ReadRequest(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var c byte
	var start = 0
	var offset = len(p.cache)
	if offset > 0 {
		data = append(p.cache, data...)
	}
	for i := offset; i < len(data); i++ {
		c = data[i]
		switch p.state {
		// case stateInit:
		// 	if !isValidMethodChar(c) {
		// 		return ErrInvalidMethod
		// 	}
		case stateMethodBefore:
			if isValidMethodChar(c) {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateMethod)
				continue
			}
			return ErrInvalidMethod
		case stateMethod:
			if c == ' ' {
				var method = strings.ToUpper(string(data[start:i]))
				if !isValidMethod(method) {
					return ErrInvalidMethod
				}
				p.processor.OnMethod(method)
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(statePathBefore)
				continue
			}
			if !isAlpha(c) {
				return ErrInvalidMethod
			}
		case statePathBefore:
			if c == '/' {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(statePath)
				continue
			}
			if c != ' ' {
				return ErrInvalidRequestURI
			}
		case statePath:
			if c == ' ' {
				var uri = string(data[start:i])
				if err := p.processor.OnURL(uri); err != nil {
					return err
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateProtoBefore)
			}
		case stateProtoBefore:
			if c != ' ' {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateProto)
			}
		case stateProto:
			switch c {
			case ' ':
				if p.proto == "" {
					p.proto = string(data[start:i])
				}
			case '\r':
				if p.proto == "" {
					p.proto = string(data[start:i])
				}
				if err := p.processor.OnProto(p.proto); err != nil {
					p.proto = ""
					return err
				}
				p.proto = ""
				p.nextState(stateProtoLF)
			}
		case stateProtoLF:
			if c == '\n' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				if !p.isClient {
					p.nextState(stateHeaderKeyBefore)
				} else {
					p.nextState(stateStatusBefore)
				}
				continue
			}
			return ErrLFExpected
			// case stateStatus:
		case stateHeaderValueLF:
			if c == '\n' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderKeyBefore)
				continue
			}
			return ErrLFExpected
		case stateHeaderKeyBefore:
			// if c == ' ' {
			// 	continue
			// }

			switch c {
			case '\r':
				err := p.parseTransferEncoding()
				if err != nil {
					return err
				}
				err = p.parseLength()
				if err != nil {
					return err
				}
				p.processor.OnContentLength(p.contentLength)
				err = p.parseTrailer()
				if err != nil {
					return err
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderOverLF)
			default:
				// data = data[i:]
				// i = 0
				if isAlpha(c) {
					start = i
					p.nextState(stateHeaderKey)
					continue
				}
				return ErrInvalidCharInHeader
			}
		case stateHeaderKey:
			switch c {
			case ' ':
				if p.headerKey == "" {
					p.headerKey = http.CanonicalHeaderKey(string(data[start:i]))
				}
			case ':':
				if p.headerKey == "" {
					p.headerKey = http.CanonicalHeaderKey(string(data[start:i]))
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderValueBefore)
			default:
				if !isToken(c) {
					return ErrInvalidCharInHeader
				}
			}
		case stateHeaderValueBefore:
			switch c {
			case ' ':
			default:
				if !isToken(c) {
					return ErrInvalidCharInHeader
				}
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateHeaderValue)
			}
		case stateHeaderValue:
			switch c {
			case ' ':
				if p.headerValue == "" {
					p.headerValue = string(data[start:i])
				}
			case '\r':
				if p.headerValue == "" {
					p.headerValue = string(data[start:i])
				}
				switch p.headerKey {
				case "Transfer-Encoding", "Trailer", "Content-Length":
					if p.header == nil {
						p.header = http.Header{}
					}
					p.header.Add(p.headerKey, p.headerValue)
				default:
				}

				p.processor.OnHeader(p.headerKey, p.headerValue)
				p.headerKey = ""
				p.headerValue = ""

				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderValueLF)
			default:
				// if !isToken(c) {
				// 	return ErrInvalidCharInHeader
				// }
			}
		case stateHeaderOverLF:
			if c == '\n' {
				if p.chunked {
					// data = data[i+1:]
					// i = -1
					start = i + 1
					p.nextState(stateBodyChunkSizeBefore)
				} else {
					start = i + 1
					if p.contentLength < 0 {
						return ErrInvalidContentLength
					}
					if p.contentLength > 0 {
						p.nextState(stateBodyContentLength)
					} else {
						p.handleMessage()
						p.nextState(stateMethodBefore)
					}
				}
				continue
			}
			return ErrLFExpected
		case stateBodyContentLength:
			cl := p.contentLength
			if len(data)-start < cl {
				p.cache = data[start:]
				return nil
			}
			p.processor.OnBody(data[start : start+cl])
			// data = data[cl:]
			i = start + cl - 1
			start = cl

			p.handleMessage()
			p.nextState(stateMethodBefore)
		case stateBodyChunkSizeBefore:
			if isNum(c) {
				p.chunkSize = -1
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateBodyChunkSize)
				continue
			}
			return ErrInvalidChunkSize
		case stateBodyChunkSize:
			switch c {
			case '\r':
				if p.chunkSize < 0 {
					cs := string(data[start:i])
					chunkSize, err := strconv.ParseInt(cs, 10, 63)
					if err != nil || chunkSize < 0 {
						return fmt.Errorf("invalid chunk size %v", cs)
					}
					p.chunkSize = int(chunkSize)
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateBodyChunkSizeLF)
			default:
				if !isNum(c) && p.chunkSize < 0 {
					cs := string(data[start:i])
					chunkSize, err := strconv.ParseInt(cs, 10, 63)
					if err != nil || chunkSize < 0 {
						return fmt.Errorf("invalid chunk size %v", cs)
					}
					p.chunkSize = int(chunkSize)
				} else {
					// chunk extension
				}
			}
		case stateBodyChunkSizeLF:
			if c == '\n' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				if p.chunkSize > 0 {
					p.nextState(stateBodyChunkData)
				} else {
					// chunk size is 0

					if len(p.trailer) > 0 {
						// read trailer headers
						p.nextState(stateBodyTrailerHeaderKeyBefore)
					} else {
						// read tail cr lf
						p.nextState(stateTailCR)
					}
				}
				continue
			}
			return ErrLFExpected
		case stateBodyChunkData:
			if len(data)-start < p.chunkSize {
				p.cache = data[start:]
				return nil
			}
			p.processor.OnBody(data[start : start+p.chunkSize])
			// data = data[p.chunkSize:]
			start += p.chunkSize
			i = start - 1
			p.nextState(stateBodyChunkDataCR)
		case stateBodyChunkDataCR:
			if c == '\r' {
				p.nextState(stateBodyChunkDataLF)
				continue
			}
			return ErrCRExpected
		case stateBodyChunkDataLF:
			if c == '\n' {
				p.nextState(stateBodyChunkSizeBefore)
				continue
			}
			return ErrLFExpected
		case stateBodyTrailerHeaderValueLF:
			if c == '\n' {
				// data = data[i+1:]
				// i = -1
				start = i
				p.nextState(stateBodyTrailerHeaderKeyBefore)
				continue
			}
			return ErrLFExpected
		case stateBodyTrailerHeaderKeyBefore:
			if isAlpha(c) {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateBodyTrailerHeaderKey)
				continue
			}

			// all trailer header readed
			if c == '\r' {
				if len(p.trailer) > 0 {
					return ErrTrailerExpected
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateTailLF)
				continue
			}
		case stateBodyTrailerHeaderKey:
			switch c {
			case ' ':
				if p.headerKey == "" {
					p.headerKey = http.CanonicalHeaderKey(string(data[start:i]))
				}
				continue
			case ':':
				if p.headerKey == "" {
					p.headerKey = http.CanonicalHeaderKey(string(data[start:i]))
				}
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateBodyTrailerHeaderValueBefore)
				continue
			}
			if !isToken(c) {
				return ErrInvalidCharInHeader
			}
		case stateBodyTrailerHeaderValueBefore:
			if c != ' ' {
				// data = data[i:]
				// i = 0
				if !isToken(c) {
					return ErrInvalidCharInHeader
				}
				start = i
				p.nextState(stateBodyTrailerHeaderValue)
			}
		case stateBodyTrailerHeaderValue:
			switch c {
			case ' ':
				if p.headerValue == "" {
					p.headerValue = string(data[start:i])
				}
			case '\r':
				if p.headerValue == "" {
					p.headerValue = string(data[start:i])
				}
				if len(p.trailer) == 0 {
					return fmt.Errorf("invalid trailer '%v'", p.headerKey)
				}
				delete(p.trailer, p.headerKey)

				p.processor.OnTrailerHeader(p.headerKey, p.headerValue)
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.headerKey = ""
				p.headerValue = ""
				p.nextState(stateBodyTrailerHeaderValueLF)
			default:
				// if !isToken(c) {
				// 	return ErrInvalidCharInHeader
				// }
			}
		case stateTailCR:
			if c == '\r' {
				p.nextState(stateTailLF)
				continue
			}
			return ErrCRExpected
		case stateTailLF:
			if c == '\n' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.handleMessage()
				p.nextState(stateMethodBefore)
				continue
			}
			return ErrLFExpected
		default:
		}
	}
	p.cache = data[start:]
	return nil
}

// Session returns user session
func (p *Parser) Session() interface{} {
	return p.session
}

// SetSession sets user session
func (p *Parser) SetSession(session interface{}) {
	p.session = session
}

func (p *Parser) parseTransferEncoding() error {
	raw, present := p.header["Transfer-Encoding"]
	if !present {
		return nil
	}
	delete(p.header, "Transfer-Encoding")

	if len(raw) != 1 {
		return fmt.Errorf("too many transfer encodings: %q", raw)
	}
	if strings.ToLower(textproto.TrimString(raw[0])) != "chunked" {
		return fmt.Errorf("unsupported transfer encoding: %q", raw[0])
	}
	delete(p.header, "Content-Length")
	p.chunked = true

	return nil
}

func (p *Parser) parseLength() (err error) {
	if cl := p.header.Get("Content-Length"); cl != "" {
		if p.chunked {
			return ErrUnexpectedContentLength
		}
		l, err := strconv.ParseInt(cl, 10, 63)
		if err != nil {
			return fmt.Errorf("%s %q", "bad Content-Length", cl)
		}
		p.contentLength = int(l)
	}
	return nil
}

func (p *Parser) parseTrailer() error {
	if !p.chunked {
		return nil
	}
	header := p.header

	trailers, ok := header["Trailer"]
	if !ok {
		return nil
	}

	header.Del("Trailer")

	trailer := http.Header{}
	for _, key := range trailers {
		key = textproto.TrimString(key)
		if key == "" {
			continue
		}
		if !strings.Contains(key, ",") {
			key = http.CanonicalHeaderKey(key)
			switch key {
			case "Transfer-Encoding", "Trailer", "Content-Length":
				return fmt.Errorf("%s %q", "bad trailer key", key)
			default:
				trailer[key] = nil
			}
			continue
		}
		for _, k := range strings.Split(key, ",") {
			if k = textproto.TrimString(k); k != "" {
				k = http.CanonicalHeaderKey(k)
				switch k {
				case "Transfer-Encoding", "Trailer", "Content-Length":
					return fmt.Errorf("%s %q", "bad trailer key", k)
				default:
					trailer[k] = nil
				}
			}
		}
	}
	if len(trailer) > 0 {
		p.trailer = trailer
	}
	return nil
}

func (p *Parser) handleMessage() {
	var addr string
	if p.conn != nil {
		addr = p.conn.RemoteAddr().String()
	}
	p.processor.OnComplete(addr)
	p.header = nil
}

// NewParser .
func NewParser(conn net.Conn, processor Processor, isClient bool, maxReadSize int) *Parser {
	if processor == nil {
		processor = NewEmptyProcessor()
	}
	return &Parser{
		conn:        conn,
		state:       stateMethodBefore,
		maxReadSize: maxReadSize,
		isClient:    isClient,
		processor:   processor,
	}
}
