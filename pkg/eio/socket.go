package eio

import (
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"github.com/adrianmxb/goseio/pkg/eio/transports"
	"net/http"
	"sync"
	"time"
)

type Socket struct {
	id           string
	server       *Server
	Transport    transports.Transport
	stateLock    sync.Mutex
	upgradeState transports.UpgradeState
	readyState   transports.ReadyState
	request      *http.Request
}

func NewSocket(id string, server *Server, transport transports.Transport, req *http.Request) *Socket {
	sock := &Socket{
		id:           id,
		server:       server,
		upgradeState: transports.UpgradeStateNone,
		readyState:   transports.ReadyStateOpening,
		request:      req,
		Transport:    transport,
	}

	sock.Open()
	return sock
}

func (s *Socket) Upgrade(transport transports.Transport) {
	s.upgradeState = transports.UpgradeStateUpgrading

}

func (s *Socket) HandleTransport(transport transports.Transport, upgrading bool) {
	go func() {
		stopNoop := make(chan struct{}, 1)
		for {
			//we stop handling packets if the transport gets killed.
			select {
			case <-transport.GetKillChannel():
				return
			default:
				break
			}
			pack, data, error := transport.Recv()
			if error != nil {
				fmt.Println(error)
			}
			if upgrading {
				switch pack.PacketType {
				case packet.Ping:
					if string(data) == "probe" {
						transport.Send(packet.Packet{
							PacketType: packet.Pong,
							IsBinary:   pack.IsBinary,
						}, data)

						go func() {
							for {
								time.Sleep(100 * time.Millisecond)
								select {
								case <-stopNoop:
									return
								default:
									s.Transport.Send(packet.Packet{
										PacketType: packet.Noop,
										IsBinary:   pack.IsBinary,
									}, nil)
								}
							}
						}()
					}
				case packet.Upgrade:
					s.stateLock.Lock()
					if s.readyState != transports.ReadyStateClosed {
						s.Transport.Discard()
						s.upgradeState = transports.UpgradeStateUpgraded
						s.Transport.Close()
						s.Transport = transport
						upgrading = false
						stopNoop <- struct{}{}
						if s.readyState == transports.ReadyStateClosing {
							transport.Close()
						}
					}
				default:
					fmt.Println("unhandled packet (upgrading)")
					fmt.Println(pack)
					fmt.Println(data)
				}
				transport.SetReadDeadline(time.Now().Add(s.server.PingInterval).Add(s.server.PingTimeout))
				continue
			}

			switch pack.PacketType {
			case packet.Ping:
				transport.Send(packet.Packet{
					PacketType: packet.Pong,
					IsBinary:   pack.IsBinary,
				}, nil)
			case packet.Message:
				fmt.Println(data)
				fmt.Printf("%s\n", data)
				s.Transport.Send(packet.Packet{
					PacketType: packet.Message,
					IsBinary:   pack.IsBinary,
				}, data)
			default:
				fmt.Println("unhandled packet.")
				fmt.Println(pack)
				fmt.Println(data)
			}
			transport.SetReadDeadline(time.Now().Add(s.server.PingInterval).Add(s.server.PingTimeout))
		}
	}()
}

func (s *Socket) Open() {
	s.readyState = transports.ReadyStateOpen

	//send open msg
	openPacket, _ := json.Marshal(&packet.OpenPacket{
		SID:          s.id,
		Upgrades:     []string{"websocket"},
		PingInterval: s.server.PingInterval.Milliseconds(),
		PingTimeout:  s.server.PingTimeout.Milliseconds(),
	})

	s.HandleTransport(s.Transport, false)

	s.Transport.Send(packet.Packet{
		PacketType: packet.Open,
		IsBinary:   false,
	}, openPacket)

	if s.server.initialPacket != nil {
		s.Transport.Send(packet.Packet{
			PacketType: packet.Message,
			IsBinary:   false,
		}, s.server.initialPacket)
	}
}

func (s *Socket) SendPacket(ptype string, data string) {

}
