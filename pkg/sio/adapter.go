package sio

type socketData struct {
	joinedRooms map[string]bool
}

type Adapter struct {
	namespace *Namespace
	rooms     map[string][]string
	sids      map[string]*socketData
}

type IAdapter interface {
	Add(id string, rooms ...string)
	Del(id string, room string)
	DelAll(id string)

	GetClientsIn(rooms ...string) []*Socket
	GetRoomsOf(id string) []string
}

func NewAdapter(namespace *Namespace) *Adapter {
	return &Adapter{
		namespace: namespace,
		rooms:     make(map[string][]string),
		sids:      make(map[string]*socketData),
	}
}

func (a *Adapter) Add(id string, rooms ...string) {
	for _, room := range rooms {
		if val, ok := a.sids[id]; !ok {
			a.sids[id] = &socketData{
				map[string]bool{room: true},
			}
		} else {
			val.joinedRooms[room] = true
		}
		if val, ok := a.rooms[room]; !ok {
			a.rooms[room] = []string{id}
		} else {
			a.rooms[room] = append(val, id)
		}
	}
}

func (a *Adapter) Del(id string, room string) {
	sd, ok := a.sids[id]
	if ok {
		delete(sd.joinedRooms, room)
		if len(sd.joinedRooms) == 0 {
			delete(a.sids, id)
		}
	}

	r, ok := a.rooms[room]
	if ok {
		for i, val := range r {
			if val == id {
				r[i] = r[len(r)-1]
				r = r[:len(r)-1]
				if len(r) > 0 {
					a.rooms[room] = r
				} else {
					delete(a.rooms, room)
				}
				break
			}
		}
	}
}

func (a *Adapter) DelAll(id string) {
	sd, ok := a.sids[id]
	if !ok {
		return
	}

	for room, _ := range sd.joinedRooms {
		a.Del(id, room)
	}
}

func (a *Adapter) GetClientsIn(rooms ...string) []*Socket {
	var sids []*Socket
	ids := make(map[string]bool)
	if len(rooms) == 0 {
		for id, _ := range a.sids {
			sids = append(sids, a.namespace.connected[id])
		}
		return sids
	}

	for _, passedRoom := range rooms {
		room, ok := a.rooms[passedRoom]
		if !ok {
			continue
		}
		for _, id := range room {
			if _, ok := ids[id]; ok {
				continue
			}
			if socket, ok := a.namespace.connected[id]; ok {
				sids = append(sids, socket)
				ids[id] = true
			}
		}
	}
	return sids
}

func (a *Adapter) GetRoomsOf(id string) []string {
	sd, ok := a.sids[id]
	if !ok {
		return nil
	}

	keys := make([]string, len(sd.joinedRooms))
	i := 0
	for k := range sd.joinedRooms {
		keys[i] = k
		i++
	}

	return keys
}
