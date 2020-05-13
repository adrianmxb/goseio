package main

import (
	"github.com/adrianmxb/goseio/pkg/sio"
	"log"
	"net/http"
)

func main() {

	/*
		srv, _ := eio.NewServer()

		srv.OnMessage(func(socket *eio.Socket, data []byte, isBinary bool) {
			//log.Println("hello")
			//log.Println(data)
			socket.SendMessage(data, isBinary)
		})

		srv.OnConnection(func(socket *eio.Socket) {
			log.Println("hello " + socket.Id)
		})
	*/

	srv, _ := sio.NewServer()

	srv.Of("/").OnConnect = func(socket *sio.Socket, next sio.NamespaceMiddleware) sio.NamespaceMiddleware {
		log.Println(socket)
		return nil
	}

	http.HandleFunc("/", srv.ServeHTTP)
	http.ListenAndServe(":3000", nil)
}
