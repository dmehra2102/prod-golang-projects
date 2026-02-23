package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Grab a buffer from the pool
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset() //  Critical always reset before use.

		fmt.Fprintf(buf, "method=%s path=%s remote=%s", r.Method, r.URL.Path, r.RemoteAddr)
		fmt.Println(buf.String())

		bufPool.Put(buf)

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.ListenAndServe(":8080", loggingMiddleware(mux))
}
