package sio

import "github.com/adrianmxb/goseio/pkg/eio"

type Client struct {
	server        *Server
	conn          *eio.Socket
	id            string
	sockets       map[string]*Socket
	namespaces    map[string]*Socket
	connectBuffer []string
}

func NewClient(server *Server, conn *eio.Socket) *Client {
	client := &Client{
		server:     server,
		conn:       conn,
		id:         conn.Id,
		sockets:    make(map[string]*Socket),
		namespaces: make(map[string]*Socket),
	}
	return client
}

func (c *Client) Connect(name string, query string) {
	namespace, ok := c.server.namespaces[name]
	if !ok {
		return
	}

	//client not in main namespace yet... queue connection up.
	if name != "/" {
		if _, ok := c.namespaces["/"]; !ok {
			c.connectBuffer = append(c.connectBuffer, name)
			return
		}
	}

	socket := namespace.Add(c, query)

	c.sockets[socket.id] = socket
	c.namespaces[namespace.name] = socket

	if namespace.name == "/" && len(c.connectBuffer) > 0 {
		for _, name := range c.connectBuffer {
			c.Connect(name, "")
		}
		c.connectBuffer = nil
	}
}
