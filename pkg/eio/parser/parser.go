package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"io"
	"io/ioutil"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

type FakeWriteCloser struct {
	io.Writer
}

func (fwc *FakeWriteCloser) Close() error {
	return nil
}

func PrepareWriter(w io.Writer, packet packet.Packet, supportsBinary bool) io.WriteCloser {
	var writer io.WriteCloser = &FakeWriteCloser{w}
	if packet.IsBinary && !supportsBinary {
		writer = base64.NewEncoder(base64.StdEncoding, w)
	}
	return writer
}

func WriteHeader(w io.Writer, packet packet.Packet, supportsBinary bool) error {
	packetType := packet.ToByte(supportsBinary)
	if packet.IsBinary && !supportsBinary {
		packetType = append([]byte{'b'}, packetType...)
	}

	_, err := w.Write(packetType)
	return err
}

func AnalyzeReader(r io.Reader) (*packet.Packet, []byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, nil, err
	}

	if b[0] == 'b' {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, nil, err
		}

		bytes, _ := ioutil.ReadAll(r)
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(bytes)))
		len, _ := base64.StdEncoding.Decode(decoded, bytes)

		p, err := packet.GetPacketFromByte(b[0], true)
		return p, decoded[:len], err
	}

	bytes, _ := ioutil.ReadAll(r)

	p, err := packet.GetPacketFromByte(b[0], false)

	if p.PacketType > packet.Noop {
		fmt.Println("invalid")
	}

	return p, bytes, err
}

var binaryByte = []byte{1}
var endByte = []byte{255}

func EncodePayloadLength(w io.Writer, packet packet.Packet, data []byte, supportsBinary bool) (int, error) {
	count := len(data)
	//adjust packetRuneCount for utf-16 size we got from engine.io
	if !packet.IsBinary || packet.IsBinary && !supportsBinary {
		count = utf8.RuneCount(data)
		uncheckedRunes := data
		for len(uncheckedRunes) > 0 {
			lastRune, size := utf8.DecodeLastRune(uncheckedRunes)
			if lastRune == utf8.RuneError {
				return 0, fmt.Errorf("invalid payload")
			}
			if r1, r2 := utf16.EncodeRune(lastRune); r1 != utf8.RuneError || r2 != utf8.RuneError {
				count++
			}
			uncheckedRunes = uncheckedRunes[:len(uncheckedRunes)-size]
		}
	}

	//packet type (open,close,...) needs 1 character -> count++
	count++

	if packet.IsBinary && supportsBinary {
		w.Write(binaryByte)

		var encodedLength []byte
		for count > 0 {
			encodedLength = append([]byte{byte(count % 10)}, encodedLength...)
			count /= 10
		}
		w.Write(encodedLength)
		w.Write(endByte)

		return len(binaryByte) + len(encodedLength) + len(endByte) + count, nil
	} else {
		if packet.IsBinary {
			count = base64.StdEncoding.EncodedLen(count)
			// 'b' (1 character -> count++) character to signal that the packet uses base64 encoding
			count++
		}

		var payloadLen []byte
		payloadLen = strconv.AppendInt(payloadLen, int64(count), 10)
		payloadLen = append(payloadLen, ':')
		w.Write(payloadLen)

		return count + len(payloadLen), nil
	}
}

func DecodePayload(r io.Reader, length int) ([]*bytes.Reader, error) {
	payload := make([]byte, length)
	_, err := io.ReadFull(r, payload)
	if err != nil {
		return nil, err
	}

	var readers []*bytes.Reader
	//packet length, in UTF-16 (wow, thank you javascript)
	packetLength := 0
	read := 0
	for len(payload) != 0 {
		if payload[read] != ':' {
			if payload[read] > '9' || payload[read] < '0' {
				return nil, fmt.Errorf("invalid payload")
			}
			packetLength = packetLength*10 + int(payload[read]-'0')
			read++
			continue
		}
		//found packet!
		packetStart := read + 1
		packetSlice := payload[packetStart : packetStart+packetLength]

		//got utf-8, find the "real" packet length.
		packetRuneCount := packetLength

		//adjust packetRuneCount for utf-16 size we got from engine.io
		uncheckedRunes := packetSlice
		for len(uncheckedRunes) > 0 {
			lastRune, size := utf8.DecodeLastRune(uncheckedRunes)
			if lastRune == utf8.RuneError {
				fmt.Errorf("invalid payload")
				return nil, err
			}
			if r1, r2 := utf16.EncodeRune(lastRune); r1 != utf8.RuneError || r2 != utf8.RuneError {
				packetRuneCount--
			}
			uncheckedRunes = uncheckedRunes[:len(uncheckedRunes)-size]
		}

		for utf8.RuneCount(packetSlice) < packetRuneCount || !utf8.Valid(packetSlice) {
			packetLength++
			if packetStart+packetLength > len(payload) {
				return nil, fmt.Errorf("invalid payload")
			}
			packetSlice = payload[packetStart : packetStart+packetLength]

			//fixup for utf-16
			lastRune, _ := utf8.DecodeLastRune(packetSlice)
			if lastRune != utf8.RuneError {
				if r1, r2 := utf16.EncodeRune(lastRune); r1 != utf8.RuneError || r2 != utf8.RuneError {
					packetRuneCount--
				}
			}
		}

		readers = append(readers, bytes.NewReader(packetSlice))
		payload = payload[packetStart+packetLength:]
	}
	return readers, nil
}
