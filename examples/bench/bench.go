package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lesismal/nbhttp"
)

func init() {
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			panic(err)
		}
	}()
}

var loop = flag.Int("n", 1000000, "bench times")
var emptyProcessor = flag.Bool("e", true, "use empty processor")

// copy from nodejs/http-parser: https://github.com/nodejs/http-parser/blob/master/bench.c#L31
var data = []byte("POST /joyent/http-parser HTTP/1.1\r\n" +
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

func bench() {
	maxReadSize := 1024 * 1024 * 4
	isClient := false
	processor := nbhttp.NewEmptyProcessor()
	if !(*emptyProcessor) {
		mux := &http.ServeMux{}
		mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {})
		processor = nbhttp.NewServerProcessor(mux)
	}
	parser := nbhttp.NewParser(nil, processor, isClient, maxReadSize)
	t := time.Now()
	for i := 0; i < *loop; i++ {
		err := parser.Read(data)
		if err != nil {
			log.Printf("parser.Read failed: %v", err)
			return
		}
	}

	used := time.Since(t)
	fmt.Printf("%v times, use empty processor: %v, time used: %v s, %v ns/op, %v req/s\n", *loop, *emptyProcessor, used.Seconds(), used.Nanoseconds()/int64(*loop), float64(*loop)/used.Seconds())
}

func main() {
	flag.Parse()
	bench()
}
