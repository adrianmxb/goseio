package transports

import (
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"github.com/adrianmxb/goseio/pkg/eio/parser"
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
)

type WSOptions struct {
	SupportsBinary bool
}

type Websocket struct {
	TransportBase
	SupportsBinary bool
	con            *websocket.Conn
	writerMutex    sync.Mutex
	readerMutex    sync.Mutex
}

func (ws *Websocket) Name() string {
	return "websocket"
}

func (ws *Websocket) Close() {
	panic("implement me")
}

func (ws *Websocket) Discard() {
	panic("implement me")
}

func NewWebsocket(opts WSOptions, sid string, con *websocket.Conn) Transport {
	ws := &Websocket{
		TransportBase: TransportBase{
			Sid:         sid,
			state:       ReadyStateOpen,
			discarded:   false,
			recvPacket:  make(chan packet.Packet),
			recvData:    make(chan []byte),
			killChannel: make(chan struct{}),
		},
		SupportsBinary: opts.SupportsBinary,
		con:            con,
	}
	go func() {
		defer con.Close()
		for {
			mt, reader, err := con.NextReader()
			fmt.Println(mt)
			if err != nil {
				//ws connection closed do something here.
				break
			}

			packetType, data, err := parser.AnalyzeReader(reader)
			if err != nil {
				continue
			}

			ws.recvPacket <- *packetType
			ws.recvData <- data
		}
	}()
	return ws
}

func (ws *Websocket) SendRaw(packet []byte) (int, error) {
	defer ws.writerMutex.Unlock()
	ws.writerMutex.Lock()
	writer, err := ws.con.NextWriter(websocket.TextMessage)

	if err != nil {
		return 0, err
	}

	n := 0
	if n, err = writer.Write(packet); err != nil {
		writer.Close()
		return n, err
	}

	if err := writer.Close(); err != nil {
		return n, err
	}

	return n, nil
}

func (ws *Websocket) Send(packet packet.Packet, data []byte) error {
	defer ws.writerMutex.Unlock()
	ws.writerMutex.Lock()
	var writer io.WriteCloser
	var err error

	if packet.IsBinary && ws.SupportsBinary {
		writer, err = ws.con.NextWriter(websocket.BinaryMessage)
	} else {
		writer, err = ws.con.NextWriter(websocket.TextMessage)
	}

	if err != nil {
		return err
	}

	preparedWriter := parser.PrepareWriter(writer, packet, ws.SupportsBinary)
	err = parser.WriteHeader(writer, packet, ws.SupportsBinary)
	if err != nil {
		if err := preparedWriter.Close(); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		return err
	}

	if data != nil {
		if _, err := preparedWriter.Write(data); err != nil {
			if err := preparedWriter.Close(); err != nil {
				return err
			}
			if err := writer.Close(); err != nil {
				return err
			}
			return err
		}
	}

	if err := preparedWriter.Close(); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func (ws *Websocket) SetReadDeadline(t time.Time) error {
	ws.readerMutex.Lock()
	defer ws.readerMutex.Unlock()
	return ws.con.SetReadDeadline(t)
}

func (ws *Websocket) SetWriteDeadline(t time.Time) error {
	ws.writerMutex.Lock()
	defer ws.writerMutex.Unlock()
	return ws.con.SetWriteDeadline(t)
}
