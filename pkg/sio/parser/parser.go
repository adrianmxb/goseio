package parser

import (
	"bufio"
	"encoding/json"
	"strconv"
)

type Packet struct {
	Id          *int
	Type        PacketTypes
	Namespace   string
	Attachments string // ???
	Data        interface{}
}

func Encode(packet Packet, writer *bufio.Writer) error {
	switch packet.Type {
	case BinaryEvent:
	case BinaryAck:
		//encode as binary...
	default:
		return strEncode(packet, writer)
	}
	return nil
}

func strEncode(packet Packet, writer *bufio.Writer) error {
	writer.WriteByte(byte('0' + packet.Type))
	// no idea how this can happen. remove if not used...
	if packet.Type == BinaryEvent || packet.Type == BinaryAck {
		writer.WriteString(packet.Attachments)
		writer.WriteByte('-')
	}

	if packet.Namespace != "/" {
		writer.WriteString(packet.Namespace)
		writer.WriteByte(',')
	}

	if packet.Id != nil {
		writer.WriteString(strconv.Itoa(*packet.Id))
	}

	if packet.Data != nil {
		b, err := json.Marshal(packet.Data)
		if err != nil {
			return err //RETURN ERROR PACKET!
		}
		writer.Write(b)
		writer.Flush()
	}
	return nil
}
