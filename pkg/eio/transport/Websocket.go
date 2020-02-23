package transport

import (
	"github.com/adrianmxb/goseio/pkg/eio/parser"
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
)

type WSOptions struct {
	TransportOptions
}

type Websocket struct {
	*Transport
	connection  *websocket.Conn
	readerMutex sync.Mutex
	writerMutex sync.Mutex
}

func NewWebsocket(opt WSOptions, sid string, con *websocket.Conn) *Websocket {
	ws := &Websocket{
		Transport:  NewTransport(opt.TransportOptions),
		connection: con,
	}

	go ws.startReceiver()
	go ws.startSender()
	go ws.startModeratorBuddy()

	return ws
}

func (ws *Websocket) GetName() string {
	return "websocket"
}

func (ws *Websocket) startModeratorBuddy() {
	for {
		select {
		case <-ws.tspModerator:
			//oh no! my mate died... i can't live without him. :(
			ws.connection.Close()
			return
		}
	}
}

func (ws *Websocket) startReceiver() {
	defer ws.connection.Close()
	for {
		select {
		case <-ws.tspClosing:
			ws.readerMutex.Lock()
			_, _, err := ws.connection.NextReader()
			ws.readerMutex.Unlock()
			if err != nil {
				ws.tspModSignal <- "close"
				return
			}
		default:
		}

		ws.readerMutex.Lock()
		_, reader, err := ws.connection.NextReader()
		ws.readerMutex.Unlock()

		if err != nil {
			close(ws.recvPacket)
			close(ws.recvData)
			select {
			case <-ws.tspModerator:
				return
			case ws.tspModSignal <- "close":
				return
			}
		}

		pack, data, err := parser.AnalyzeReader(reader)
		if err != nil {
			//write this to error channel, that the socket consumes?
			continue
		}

		//read from connection until NextReader throws, handle everything in err handler.
		ws.recvPacket <- *pack
		ws.recvData <- data
	}
}

func (ws *Websocket) startSender() {
	var writer io.WriteCloser
	var err error
	for {
		select {
		case <-ws.tspModerator:
			return
		case pack := <-ws.sendPacket:
			data := <-ws.sendData

			ws.writerMutex.Lock()
			if pack.IsBinary && ws.SupportsBinary {
				writer, err = ws.connection.NextWriter(websocket.BinaryMessage)
			} else {
				writer, err = ws.connection.NextWriter(websocket.TextMessage)
			}
			ws.writerMutex.Unlock()

			//couldn't get writer, connection got force killed or timed out (closing state?)
			if err != nil {
				select {
				case <-ws.tspModerator:
					return
				case ws.tspModSignal <- "close":
					return
				}
			}
			err = parser.WriteHeader(writer, pack, ws.SupportsBinary)
			wrappedWriter := parser.PrepareWriter(writer, pack.IsBinary, ws.SupportsBinary)

			wrappedWriter.Write(data)

			wrappedWriter.Close()
			writer.Close()
		}
	}
}

func (ws *Websocket) SetReadDeadline(t time.Time) error {
	defer ws.readerMutex.Unlock()
	ws.readerMutex.Lock()
	return ws.connection.SetReadDeadline(t)
}

func (ws *Websocket) SetWriteDeadline(t time.Time) error {
	defer ws.writerMutex.Unlock()
	ws.writerMutex.Lock()
	return ws.connection.SetWriteDeadline(t)
}
