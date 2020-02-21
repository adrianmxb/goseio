package main

import (
	"github.com/adrianmxb/goseio/pkg/eio"
	"net/http"
)

func main() {
	srv := eio.NewServer()
	http.HandleFunc("/", srv.ServeHTTP)
	http.ListenAndServe(":3000", nil)
}
