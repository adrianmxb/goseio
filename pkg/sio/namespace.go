package sio

type NamespaceMiddleware func(socket *Socket, next NamespaceMiddleware) NamespaceMiddleware

type Namespace struct {
	//keep ackId up here so we have correct 64bit alignment on ARM processors
	ackId uint64

	name      string
	server    *Server
	sockets   map[string]*Socket
	connected map[string]*Socket
}
