package sio

type NamespaceMiddleware func(socket *Socket, next NamespaceMiddleware) NamespaceMiddleware

type F struct {
	rooms []string
}

type Namespace struct {
	//keep ackId up here so we have correct 64bit alignment on ARM processors
	ackId uint64

	name      string
	server    *Server
	sockets   map[string]*Socket
	connected map[string]*Socket
	OnConnect NamespaceMiddleware
	adapter   IAdapter
}

func NewNamespace(server *Server, name string) *Namespace {
	nsp := &Namespace{
		ackId:     0,
		name:      name,
		server:    server,
		sockets:   make(map[string]*Socket),
		connected: make(map[string]*Socket),
	}
	nsp.adapter = NewAdapter(nsp)
	return nsp
}

func (n *Namespace) Add(client *Client, query string) *Socket {
	socket := NewSocket(n, client, query)

	n.sockets[socket.id] = socket
	n.connected[socket.id] = socket

	// TODO: fire namespace related events. (connect, connection)
	n.OnConnect(socket, nil)

	return socket
}
