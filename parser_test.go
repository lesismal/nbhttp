package nbhttp

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

// var parser = newParser()
func TestServerParserContentLength(t *testing.T) {
	data := []byte("POST /echo HTTP/1.1\r\nHost: localhost:8080\r\nConnection: close \r\nAccept-Encoding : gzip \r\n\r\n")
	testParser(t, false, data)

	data = []byte("POST /echo HTTP/1.1\r\nHost: localhost:8080\r\nConnection: close \r\nContent-Length :  0\r\nAccept-Encoding : gzip \r\n\r\n")
	testParser(t, false, data)

	data = []byte("POST /echo HTTP/1.1\r\nHost: localhost:8080\r\nConnection: close \r\nContent-Length :  5\r\nAccept-Encoding : gzip \r\n\r\nhello")
	testParser(t, false, data)
}

func TestServerParserChunks(t *testing.T) {
	data := []byte("POST / HTTP/1.1\r\nHost: localhost:1235\r\nUser-Agent: Go-http-client/1.1\r\nTransfer-Encoding: chunked\r\nAccept-Encoding: gzip\r\n\r\n4\r\nbody\r\n0\r\n\r\n")
	testParser(t, false, data)
}

func TestServerParserTrailer(t *testing.T) {
	data := []byte("POST / HTTP/1.1\r\nHost: localhost:1235\r\nUser-Agent: Go-http-client/1.1\r\nTransfer-Encoding: chunked\r\nTrailer: Md5,Size\r\nAccept-Encoding: gzip\r\n\r\n4\r\nbody\r\n0\r\nMd5: 841a2d689ad86bd1611447453c22c6fc\r\nSize: 4\r\n\r\n")
	testParser(t, false, data)
}

func TestClientParserContentLength(t *testing.T) {
	data := []byte("HTTP/1.1 200 OK\r\nHost: localhost:8080\r\nConnection: close \r\nAccept-Encoding : gzip \r\n\r\n")
	testParser(t, true, data)

	data = []byte("HTTP/1.1 200 OK\r\nHost: localhost:8080\r\nConnection: close \r\nContent-Length :  0\r\nAccept-Encoding : gzip \r\n\r\n")
	testParser(t, true, data)

	data = []byte("HTTP/1.1 200 OK\r\nHost: localhost:8080\r\nConnection: close \r\nContent-Length :  5\r\nAccept-Encoding : gzip \r\n\r\nhello")
	testParser(t, true, data)
}

func TestClientParserChunks(t *testing.T) {
	data := []byte("HTTP/1.1 200 OK\r\nHost: localhost:1235\r\nUser-Agent: Go-http-client/1.1\r\nTransfer-Encoding: chunked\r\nAccept-Encoding: gzip\r\n\r\n4\r\nbody\r\n0\r\n\r\n")
	testParser(t, true, data)
}

func TestClientParserTrailer(t *testing.T) {
	data := []byte("HTTP/1.1 200 OK\r\nHost: localhost:1235\r\nUser-Agent: Go-http-client/1.1\r\nTransfer-Encoding: chunked\r\nTrailer: Md5,Size\r\nAccept-Encoding: gzip\r\n\r\n4\r\nbody\r\n0\r\nMd5: 841a2d689ad86bd1611447453c22c6fc\r\nSize: 4\r\n\r\n")
	testParser(t, true, data)
}

func testParser(t *testing.T, isClient bool, data []byte) error {
	parser := newParser(isClient)
	err := parser.ReadRequest(data)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < len(data)-1; i++ {
		err := parser.ReadRequest(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
	}
	err = parser.ReadRequest(data[len(data)-1:])
	if err != nil {
		t.Fatal(err)
	}

	nRequest := 0
	data = append(data, data...)

	maxReadSize := 1024 * 1024 * 4
	mux := &http.ServeMux{}
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		nRequest++
	})
	processor := NewServerProcessor(mux)
	if isClient {
		processor = NewClientProcessor(func(*http.Response) {
			nRequest++
		})
	}
	parser = NewParser(nil, processor, isClient, maxReadSize)
	tBegin := time.Now()
	loop := 100000
	for i := 0; i < loop; i++ {
		tmp := data
		reads := [][]byte{}
		for len(tmp) > 0 {
			nRead := int(rand.Intn(len(tmp)) + 1)
			readBuf := tmp[:nRead]
			reads = append(reads, readBuf)
			tmp = tmp[nRead:]
			err = parser.ReadRequest(readBuf)
			if err != nil {
				t.Fatalf("nRead: %v, numOne: %v, reads: %v, error: %v", len(data)-len(tmp), len(data), reads, err)
			}

		}
		if nRequest != (i+1)*2 {
			return fmt.Errorf("nRequest: %v, %v", i, nRequest)
		}
	}
	tUsed := time.Since(tBegin)
	fmt.Printf("%v loops, %v s used, %v ns/op, %v req/s\n", loop, tUsed.Seconds(), tUsed.Nanoseconds()/int64(loop), float64(loop)/tUsed.Seconds())

	return nil
}

func newParser(isClient bool) *Parser {
	maxReadSize := 1024 * 1024 * 4
	if isClient {
		processor := NewClientProcessor(func(*http.Response) {})
		return NewParser(nil, processor, isClient, maxReadSize)
	}
	mux := &http.ServeMux{}
	mux.HandleFunc("/", pirntMessage)
	processor := NewServerProcessor(mux)
	return NewParser(nil, processor, isClient, maxReadSize)
}

func pirntMessage(w http.ResponseWriter, request *http.Request) {
	fmt.Printf("----------------------------------------------------------------\n")
	fmt.Println("OnRequest")
	fmt.Println("Method:", request.Method)
	fmt.Println("Path:", request.URL.Path)
	fmt.Println("Proto:", request.Proto)
	fmt.Println("Host:", request.URL.Host)
	fmt.Println("Rawpath:", request.URL.RawPath)
	fmt.Println("Content-Length:", request.ContentLength)
	for k, v := range request.Header {
		fmt.Printf("Header: [\"%v\": \"%v\"]\n", k, v)
	}
	for k, v := range request.Trailer {
		fmt.Printf("Trailer: [\"%v\": \"%v\"]\n", k, v)
	}
	body := request.Body
	if body != nil {
		b, _ := io.ReadAll(body)
		fmt.Println("body:", string(b))
	} else {
		fmt.Println("body: null")
	}
}

var benchData = []byte("POST /joyent/http-parser HTTP/1.1\r\n" +
	"Host: github.com\r\n" +
	"DNT: 1\r\n" +
	"Accept-Encoding: gzip, deflate, sdch\r\n" +
	"Accept-Language: ru-RU,ru;q=0.8,en-US;q=0.6,en;q=0.4\r\n" +
	"User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/39.0.2171.65 Safari/537.36\r\n" +
	"Accept: text/html,application/xhtml+xml,application/xml;q=0.9," +
	"image/webp,*/*;q=0.8\r\n" +
	"Referer: https://github.com/joyent/http-parser\r\n" +
	"Connection: keep-alive\r\n" +
	"Transfer-Encoding: chunked\r\n" +
	"Cache-Control: max-age=0\r\n\r\nb\r\nhello world\r\n0\r\n\r\n")

func BenchmarkServerProcessor(b *testing.B) {
	maxReadSize := 1024 * 1024 * 4
	isClient := false
	mux := &http.ServeMux{}
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {})
	processor := NewServerProcessor(mux)
	parser := NewParser(nil, processor, isClient, maxReadSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := parser.ReadRequest(benchData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmpryProcessor(b *testing.B) {
	maxReadSize := 1024 * 1024 * 4
	isClient := false
	// processor := NewEmptyProcessor()
	parser := NewParser(nil, nil, isClient, maxReadSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := parser.ReadRequest(benchData); err != nil {
			b.Fatal(err)
		}
	}
}
