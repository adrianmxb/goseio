package packet

import "errors"

type PacketType int

const (
	Open    PacketType = 0 //done
	Close   PacketType = 1 //??? (xhr only?)
	Ping    PacketType = 2 // done
	Pong    PacketType = 3 // done
	Message PacketType = 4 // started, low level right now.
	Upgrade PacketType = 5 //!!! (polling->websocket)
	Noop    PacketType = 6 //!!! (for xhr)
)

type Packet struct {
	PacketType PacketType
	IsBinary   bool
}

var packetError = errors.New("invalid packet")

func GetPacketFromByte(b byte, isBase64 bool) (*Packet, error) {
	p := &Packet{
		PacketType: PacketType(b),
		IsBinary:   true,
	}
	if p.PacketType < Open || p.PacketType > Noop {
		p.PacketType -= '0'
		if !isBase64 {
			p.IsBinary = false
		}
	}

	if p.PacketType >= Open && p.PacketType <= Noop {
		return p, nil
	}

	return nil, packetError
}

type OpenPacket struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int64    `json:"pingInterval"`
	PingTimeout  int64    `json:"pingTimeout"`
}

func (p Packet) String() string {
	return []string{"Open", "Close", "Ping", "Pong", "Message", "Upgrade", "Noop"}[p.PacketType]
}

func (p Packet) ToByte(supportsBinary bool) []byte {
	var b [1]byte
	b[0] = byte(p.PacketType)
	if !p.IsBinary || !supportsBinary {
		b[0] += '0'
	}

	return b[:]
}
