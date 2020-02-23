package main

import (
	"github.com/adrianmxb/goseio/pkg/eio"
	"net/http"
)

func main() {
	srv, _ := eio.NewServer()

	srv.OnMessage(func(socket *eio.Socket, data []byte, isBinary bool) {
		//log.Println("hello")
		//log.Println(data)
		socket.SendMessage(data, isBinary)
	})
	http.HandleFunc("/", srv.ServeHTTP)
	http.ListenAndServe(":3000", nil)
}
