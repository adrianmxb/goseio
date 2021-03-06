package eio

import (
	"bytes"
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"github.com/adrianmxb/goseio/pkg/eio/transport"
	"net/http"
	"sync"
	"time"
)

type Socket struct {
	Id            string
	server        *Server
	Transport     transport.ITransport
	transportLock sync.RWMutex
	stateLock     sync.Mutex
	upgradeState  UpgradeState
	readyState    ReadyState
	remoteAddr    string
}

func NewSocket(id string, server *Server, transport transport.ITransport, req *http.Request) *Socket {
	sock := &Socket{
		Id:           id,
		server:       server,
		upgradeState: UpgradeStateNone,
		readyState:   ReadyStateOpening,
		Transport:    transport,
		remoteAddr:   req.RemoteAddr,
	}

	sock.Open()
	return sock
}

func (s *Socket) Close() {
	defer s.stateLock.Unlock()
	s.stateLock.Lock()
	if s.readyState != ReadyStateOpen {
		return
	}

	s.readyState = ReadyStateClosing

	defer s.transportLock.RUnlock()
	s.transportLock.RLock()
	s.Transport.Close()
}

var probeBytes = []byte("probe")

func (s *Socket) HandleTransport(transport transport.ITransport, upgrading bool) {
	stopNoop := make(chan struct{}, 1)
	for {
		pack, data, err := transport.Recv()
		if err != nil {
			return
		}
		switch pack.PacketType {
		case packet.Ping:
			transport.Send(packet.Packet{
				PacketType: packet.Pong,
				IsBinary:   pack.IsBinary,
			}, data, false)
			if upgrading && bytes.Compare(data, probeBytes) == 0 {
				go func() {
					for {
						time.Sleep(100 * time.Millisecond)
						select {
						case <-stopNoop:
							return
						default:
							s.transportLock.RLock()
							s.Transport.Send(packet.Packet{
								PacketType: packet.Noop,
								IsBinary:   pack.IsBinary,
							}, nil, false)
							s.transportLock.RUnlock()
						}
					}
				}()
			}
		case packet.Message:
			s.server.MsgHandler(s, data, pack.IsBinary)
		case packet.Upgrade:
			s.stateLock.Lock()
			if s.readyState != ReadyStateClosed {
				stopNoop <- struct{}{}
				s.transportLock.Lock()
				s.Transport.Discard()
				s.upgradeState = UpgradeStateUpgraded
				s.Transport.Close()
				s.Transport = transport
				upgrading = false
				if s.readyState == ReadyStateClosing {
					transport.Close()
				}
				s.transportLock.Unlock()
			}
			s.stateLock.Unlock()
		default:
			fmt.Println("unhandled packet.")
			fmt.Println(pack)
			fmt.Println(data)
		}
		transport.SetReadDeadline(time.Now().Add(s.server.PingInterval).Add(s.server.PingTimeout))
	}
}

func (s *Socket) Open() {
	s.readyState = ReadyStateOpen

	//send open msg
	openPacket, _ := json.Marshal(&packet.OpenPacket{
		SID:          s.Id,
		Upgrades:     []string{"websocket"},
		PingInterval: s.server.PingInterval.Milliseconds(),
		PingTimeout:  s.server.PingTimeout.Milliseconds(),
	})

	go s.HandleTransport(s.Transport, false)

	s.Transport.Send(packet.Packet{
		PacketType: packet.Open,
		IsBinary:   false,
	}, openPacket, false)

	if s.server.initialPacket != nil {
		s.Transport.Send(packet.Packet{
			PacketType: packet.Message,
			IsBinary:   false,
		}, s.server.initialPacket, false)
	}
}

func (s *Socket) SendMessage(data []byte, isBinary bool) {
	defer s.transportLock.RUnlock()
	s.transportLock.RLock()
	s.Transport.Send(packet.Packet{
		PacketType: packet.Message,
		IsBinary:   isBinary,
	}, data, false)
}
