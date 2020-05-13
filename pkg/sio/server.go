package sio

import (
	"bufio"
	"bytes"
	"github.com/adrianmxb/goseio/pkg/eio"
	"github.com/adrianmxb/goseio/pkg/sio/parser"
	"net/http"
)

type Server struct {
	eio        *eio.Server
	sockets    map[string]*Socket
	connected  map[string]*Socket
	namespaces map[string]*Namespace
}

type ServerOptions struct {
	path    string
	origins []string
}

func NewServer() (*Server, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	err := parser.Encode(parser.Packet{
		Id:        nil,
		Type:      parser.Connect,
		Namespace: "/",
		Data:      nil,
	}, writer)

	if err != nil {
		return nil, err
	}

	eioSrv, err := eio.NewServer("/socket.io", buf)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		eio:        eioSrv,
		sockets:    make(map[string]*Socket),
		connected:  make(map[string]*Socket),
		namespaces: make(map[string]*Namespace),
	}

	srv.eio.ConnectHandler = srv.HandleConnection

	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.eio.ServeHTTP(w, r)
}

func (s *Server) HandleConnection(socket *eio.Socket) {
	client := NewClient(s, socket)
	client.Connect("/", "")
}

func (s *Server) Of(name string) *Namespace {
	if name[0] != '/' {
		name = "/" + name
	}

	namespace, ok := s.namespaces[name]
	if !ok {
		namespace = NewNamespace(s, name)
		s.namespaces[name] = namespace
	}
	return namespace
}
