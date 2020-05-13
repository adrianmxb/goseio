package sio

type Socket struct {
	namespace *Namespace
	adapter   IAdapter
	id        string
	client    *Client
	rooms     map[string]struct{}
}

func NewSocket(namespace *Namespace, client *Client, query string) *Socket {
	id := client.id
	if namespace.name != "/" {
		id = namespace.name + "#" + id
	}

	socket := &Socket{
		namespace: namespace,
		client:    client,
		id:        id,
		rooms:     make(map[string]struct{}),
	}

	return socket
}

func (s *Socket) onConnect() {
	s.Join(s.id)
}

func (s *Socket) Join(rooms ...string) {
	s.adapter.Add(s.id, rooms...)
	for _, room := range rooms {
		if _, ok := s.rooms[room]; ok {
			continue
		}
		s.rooms[room] = struct{}{}
	}
}

func (s *Socket) Leave(room string) {
	s.adapter.Del(s.id, room)
	delete(s.rooms, room)
}

func (s *Socket) LeaveAll() {
	s.adapter.DelAll(s.id)
	s.rooms = make(map[string]struct{})
}
