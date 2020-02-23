package transport

import (
	"errors"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"net/http"
	"time"
)

type TransportOptions struct {
	SupportsBinary bool
}

type Transport struct {
	SupportsBinary bool

	tspModSignal chan string
	//if this channel is alive, everything is fine.
	tspModerator chan struct{}
	tspActive    chan struct{}
	tspClosing   chan struct{}

	sendPacket chan packet.Packet
	sendData   chan []byte

	recvPacket chan packet.Packet
	recvData   chan []byte
}

type ITransport interface {
	GetName() string
	Discard()
	Close()
	Recv() (packet.Packet, []byte, error)
	Send(pack packet.Packet, data []byte, force bool) bool
	//
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error

	HandleRequest(r *http.Request, w http.ResponseWriter)
}

func NewTransport(opt TransportOptions) *Transport {
	transport := &Transport{
		SupportsBinary: opt.SupportsBinary,

		tspModSignal: make(chan string, 1),

		// ^= closed
		tspModerator: make(chan struct{}),

		// ^= discarded
		tspActive: make(chan struct{}),

		// ^= closing
		tspClosing: make(chan struct{}),

		sendPacket: make(chan packet.Packet, 30),
		sendData:   make(chan []byte, 30),
		recvPacket: make(chan packet.Packet),
		recvData:   make(chan []byte),
	}

	//run the moderator!
	//this little fellow takes care of altering transport state.
	go func() {
		active := true
		sending := true
		for {
			signal := <-transport.tspModSignal

			switch signal {
			case "close":
				close(transport.tspModerator)
				return
			case "discard":
				if active {
					active = false
					close(transport.tspActive)
				}
			case "closing":
				if sending {
					sending = false
					close(transport.tspClosing)
				}
			}
		}
	}()

	return transport
}

func (t *Transport) Discard() {
	t.tspModSignal <- "discard"
}

func (t *Transport) Close() {
	t.tspModSignal <- "closing"
}

var recvErr = errors.New("recv channel closed")
var closeErr = errors.New("got close packet")

func (t *Transport) Recv() (packet.Packet, []byte, error) {
	recvPacket, ok := <-t.recvPacket

	if !ok {
		return recvPacket, <-t.recvData, recvErr
	}

	if recvPacket.PacketType == packet.Close {
		t.tspModSignal <- "close"
		return recvPacket, <-t.recvData, closeErr
	}

	return recvPacket, <-t.recvData, nil
}

func (t *Transport) Send(pack packet.Packet, data []byte, force bool) bool {
	select {
	case <-t.tspModerator:
		//moderator is dead.
		return false
		//make sure no new requests get sent to closed connection.
	case <-t.tspClosing:
		if force == true {
			break
		}
		return false
	default:
	}

	//process old requests, abort only if connection was closed by force.
	select {
	case <-t.tspModerator:
		return false
	case t.sendPacket <- pack:
		t.sendData <- data
		return true
	}
}

func (t *Transport) HandleRequest(r *http.Request, w http.ResponseWriter) {
	return
}
