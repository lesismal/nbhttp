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

	// ErrInvalidMethod .
	ErrInvalidMethod = errors.New("invalid method")

	// ErrInvalidContentLength .
	ErrInvalidContentLength = errors.New("invalid ContentLength")

	// ErrInvalidChunkSize .
	ErrInvalidChunkSize = errors.New("invalid chunk data")
)

var (
	blankCharMap = [128]bool{
		' ':  true,
		'\r': true,
		'\n': true,
	}

	validMethods = map[string]bool{
		"OPTIONS": true,
		"GET":     true,
		"HEAD":    true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"TRACE":   true,
		"CONNECT": true,
	}

	numCharMap      = [128]bool{}
	alphaCharMap    = [128]bool{}
	alphaNumCharMap = [128]bool{}

	validMethodCharMap = [128]bool{}
)

func init() {
	var dis byte = 'a' - 'A'

	for m := range validMethods {
		for _, c := range m {
			validMethodCharMap[c] = true
			validMethodCharMap[byte(c)+dis] = true
		}
	}

	for i := byte(0); i < 10; i++ {
		numCharMap['0'+i] = true
		alphaNumCharMap['0'+i] = true
	}

	for i := byte(0); i < 26; i++ {
		alphaCharMap['A'+i] = true
		alphaCharMap['A'+i+dis] = true
		alphaNumCharMap['A'+i] = true
		alphaNumCharMap['A'+i+dis] = true
	}
}

func isAlpha(c byte) bool {
	return alphaCharMap[c]
}

func isNum(c byte) bool {
	return numCharMap[c]
}

func isAlphaNum(c byte) bool {
	return alphaNumCharMap[c]
}

func isBlankChar(c byte) bool {
	return blankCharMap[c]
}

func isValidMethod(m string) bool {
	return validMethods[strings.ToUpper(m)]
}

func isValidMethodChar(c byte) bool {
	return validMethodCharMap[c]
}

const (
	// state: RequestLine
	stateMethodBefore int8 = iota
	stateMethod

	statePathBefore
	statePath
	stateProtoBefore
	stateProto
	stateStatusBefore
	stateStatus

	// state: Header
	stateHeaderKeyBefore
	stateHeaderKeyLF
	stateHeaderKey

	stateHeaderValueBefore
	stateHeaderValue

	// state: Body ContentLength
	stateBodyContentLengthBlankLine
	stateBodyContentLength

	// state: Body Chunk
	stateBodyChunkSizeBlankLine
	stateBodyChunkSizeBefore
	stateBodyChunkSize
	stateBodyChunkDataBefore
	stateBodyChunkData

	// state: Body Trailer
	stateBodyTrailerHeaderKeyLF
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
		case stateMethodBefore:
			if !isValidMethodChar(c) {
				continue
			}
			// data = data[i:]
			// i = 0
			start = i
			p.nextState(stateMethod)
		case stateMethod:
			if c == ' ' {
				var method = strings.ToUpper(string(data[start:i]))
				if !isValidMethod(method) {
					return ErrInvalidMethod
				}
				p.processor.Method(method)
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(statePathBefore)
			}
		case statePathBefore:
			if c == '/' {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(statePath)
			}

		case statePath:
			if c == ' ' {
				var uri = string(data[start:i])
				if err := p.processor.URL(uri); err != nil {
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
			case ' ', '\r':
				var proto = string(data[start:i])
				if err := p.processor.Proto(proto); err != nil {
					return err
				}
			case '\n':
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderKeyBefore)
			}
		// case stateStatus:
		case stateHeaderKeyLF:
			if c != '\n' {
				return ErrInvalidCRLF
			}
			// data = data[i+1:]
			// i = -1
			start = i + 1
			p.nextState(stateHeaderKeyBefore)
		case stateHeaderKeyBefore:
			if c == ' ' {
				continue
			}

			if c != '\r' {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateHeaderKey)
				continue
			}
			err := p.parseTransferEncoding()
			if err != nil {
				return err
			}
			err = p.parseLength()
			if err != nil {
				return err
			}
			p.processor.ContentLength(p.contentLength)
			err = p.parseTrailer()
			if err != nil {
				return err
			}
			// data = data[i+1:]
			// i = -1
			start = i + 1
			if p.chunked {
				p.nextState(stateBodyChunkSizeBlankLine)
			} else {
				p.nextState(stateBodyContentLengthBlankLine)
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
			}
		case stateHeaderValueBefore:
			if c != ' ' {
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

				p.processor.Header(p.headerKey, p.headerValue)
				p.headerKey = ""
				p.headerValue = ""

				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateHeaderKeyLF)
			}
		case stateBodyContentLengthBlankLine:
			if c != '\n' {
				return ErrInvalidCRLF
			}
			// data = data[i+1:]
			// i = -1
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
		case stateBodyContentLength:
			cl := p.contentLength
			if len(data)-start < cl {
				p.cache = data[start:]
				return nil
			}
			p.processor.Body(data[start : start+cl])
			// data = data[cl:]
			i = start + cl - 1
			start = cl

			p.handleMessage()
			p.nextState(stateMethodBefore)
		case stateBodyChunkSizeBlankLine:
			if c != '\n' {
				return ErrInvalidCRLF
			}

			// data = data[i+1:]
			// i = -1
			start = i + 1
			p.nextState(stateBodyChunkSizeBefore)
		case stateBodyChunkSizeBefore:
			if isNum(c) {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateBodyChunkSize)
			}
		case stateBodyChunkSize:
			if !isNum(c) {
				cs := string(data[start:i])
				chunkSize, err := strconv.ParseInt(cs, 10, 63)
				if err != nil {
					return fmt.Errorf("%s %q", "bad Content-Length", cs)
				}
				if chunkSize < 0 {
					return ErrInvalidChunkSize
				}
				p.chunkSize = int(chunkSize)
			}
			if c == '\r' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateBodyChunkDataBefore)
			} else {
				// chunk extension
			}
		case stateBodyChunkDataBefore:
			if c != '\n' {
				return ErrInvalidCRLF
			}
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
		case stateBodyChunkData:
			if len(data)-start < p.chunkSize {
				p.cache = data[start:]
				return nil
			}
			p.processor.Body(data[start : start+p.chunkSize])
			// data = data[p.chunkSize:]
			start += p.chunkSize
			i = start - 1
			p.nextState(stateBodyChunkSizeBefore)
		case stateBodyTrailerHeaderKeyLF:
			if c != '\n' {
				return ErrInvalidCRLF
			}
			// data = data[i+1:]
			// i = -1
			start = i
			p.nextState(stateBodyTrailerHeaderKeyBefore)
		case stateBodyTrailerHeaderKeyBefore:
			if c == ' ' {
				continue
			}

			// all trailer header readed
			if c == '\r' {
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.nextState(stateTailLF)
				continue
			}

			// first alpha letter: the beginning of a new header's 'key'
			if isAlpha(c) {
				// data = data[i:]
				// i = 0
				start = i
				p.nextState(stateBodyTrailerHeaderKey)
			}
		case stateBodyTrailerHeaderKey:
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
				p.nextState(stateBodyTrailerHeaderValueBefore)
			default:
			}
		case stateBodyTrailerHeaderValueBefore:
			if c != ' ' {
				// data = data[i:]
				// i = 0
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

				p.processor.TrailerHeader(p.headerKey, p.headerValue)
				// data = data[i+1:]
				// i = -1
				start = i + 1
				p.headerKey = ""
				p.headerValue = ""
				p.nextState(stateBodyTrailerHeaderKeyLF)
			}
		case stateTailCR:
			if c != '\r' {
				return ErrInvalidCRLF
			}
			p.nextState(stateTailLF)
		case stateTailLF:
			if c != '\n' {
				return ErrInvalidCRLF
			}
			// data = data[i+1:]
			// i = -1
			start = i + 1
			p.handleMessage()
			p.nextState(stateMethodBefore)
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
	if !p.chunked {
		if cl := p.header.Get("Content-Length"); cl != "" {
			l, err := strconv.ParseInt(cl, 10, 63)
			if err != nil {
				return fmt.Errorf("%s %q", "bad Content-Length", cl)
			}
			p.contentLength = int(l)
		}
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
	p.processor.Complete(addr)
	p.header = nil
}

// NewParser .
func NewParser(conn net.Conn, processor Processor, isClient bool, maxReadSize int) *Parser {
	if processor == nil {
		processor = NewEmptyProcessor()
	}
	return &Parser{
		conn:        conn,
		maxReadSize: maxReadSize,
		isClient:    isClient,
		processor:   processor,
	}
}
