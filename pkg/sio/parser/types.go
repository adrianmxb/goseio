package parser

type PacketTypes int

const (
	Connect PacketTypes = iota
	Disconnect
	Event
	Ack
	Error
	BinaryEvent
	BinaryAck
)
