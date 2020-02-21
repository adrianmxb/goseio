package transports

import (
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"net/http"
	"time"
)

type UpgradeState int

const (
	UpgradeStateNone UpgradeState = iota
	UpgradeStateUpgrading
	UpgradeStateUpgraded
)

type ReadyState int

const (
	ReadyStateOpening ReadyState = iota
	ReadyStateOpen
	ReadyStateClosing
	ReadyStateClosed
)

func (e ReadyState) String() string {
	switch e {
	case ReadyStateOpening:
		return "Opening"
	case ReadyStateOpen:
		return "Open"
	case ReadyStateClosing:
		return "Closing"
	case ReadyStateClosed:
		return "Closed"
	}
	return ""
}

type Transport interface {
	Name() string
	Close()
	Discard()
	Send(packet packet.Packet, data []byte) error
	SendRaw(encodedPacket []byte) (int, error)
	Recv() (packet.Packet, []byte, error)
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	HandleRequest(r *http.Request, w http.ResponseWriter)
	GetKillChannel() chan struct{}
}

type TransportBase struct {
	Sid         string
	state       ReadyState
	discarded   bool
	recvPacket  chan packet.Packet
	recvData    chan []byte
	killChannel chan struct{}
}

func (t *TransportBase) Discard() {
	t.discarded = true
}

func (t *TransportBase) GetKillChannel() chan struct{} {
	return t.killChannel
}

func (t *TransportBase) Recv() (packet.Packet, []byte, error) {
	return <-t.recvPacket, <-t.recvData, nil
}

func (t *TransportBase) HandleRequest(r *http.Request, w http.ResponseWriter) {
	return
}

/*
func(t *Transport) Discard() {
	t.discarded = true
}
*/
