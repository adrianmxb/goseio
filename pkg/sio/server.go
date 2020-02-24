package sio

import "github.com/adrianmxb/goseio/pkg/eio"

type Server struct {
	eio *eio.Server
}

type ServerOptions struct {
	path      string
	origins   []string
	sockets   map[string]*Socket
	connected map[string]*Socket
}

//func NewServer()
