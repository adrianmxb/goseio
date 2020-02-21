package transports

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"github.com/adrianmxb/goseio/pkg/eio/parser"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var PollingCloseTimeout time.Duration = 30 * time.Second

type PollingOpt struct {
	TransportBase
	SupportsBinary    bool
	MaxHttpBufferSize uint64
	HttpCompression   bool
}

type PollingBase struct {
	TransportBase
	SupportsBinary    bool
	MaxHttpBufferSize uint64
	HttpCompression   bool
	//TODO: make this pkg private
	dataReady    chan bool
	pollReady    chan bool
	sendPacket   chan packet.Packet
	sendData     chan []byte
	readDeadline *time.Timer
}

func CreatePollingBase(opts PollingOpt, sid string) *PollingBase {
	base := &PollingBase{
		TransportBase: TransportBase{
			Sid:         sid,
			discarded:   false,
			state:       ReadyStateOpen,
			recvPacket:  make(chan packet.Packet),
			recvData:    make(chan []byte),
			killChannel: make(chan struct{}),
		},
		SupportsBinary:    opts.SupportsBinary,
		MaxHttpBufferSize: opts.MaxHttpBufferSize,
		HttpCompression:   opts.HttpCompression,

		//redesign this so we dont need buffered channels here.
		dataReady:  make(chan bool, 1),
		pollReady:  make(chan bool, 1),
		sendPacket: make(chan packet.Packet, 1),
		sendData:   make(chan []byte, 1),
	}
	base.pollReady <- true
	base.dataReady <- true

	return base
}

func (p *PollingBase) Name() string {
	return "polling"
}

func (p *PollingBase) Close() {
	p.state = ReadyStateClosed
}

func (p *PollingBase) SendRaw(encodedPacket []byte) (int, error) {
	panic("implement me")
}

func (p *PollingBase) SetReadDeadline(t time.Time) error {
	if p.readDeadline != nil {
		p.readDeadline.Stop()
	}
	p.readDeadline = time.AfterFunc(t.Sub(time.Now()), func() {
		fmt.Println("read deadline. DELETE CLIENTS!")
	})
	return nil
}

func (p *PollingBase) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

func (p *PollingBase) Send(packet packet.Packet, data []byte) error {
	//do whatever...
	p.sendPacket <- packet
	p.sendData <- data

	return nil
}

func (p *PollingBase) HandlePollRequest(r *http.Request, w http.ResponseWriter) error {
	//do shit.
	header := <-p.sendPacket
	data := <-p.sendData

	respond := func(data []byte) error {
		preparedWriter := parser.PrepareWriter(w, header, p.SupportsBinary)
		defer preparedWriter.Close()

		length, err := parser.EncodePayloadLength(w, header, data, p.SupportsBinary)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Length", strconv.Itoa(length))
		err = parser.WriteHeader(w, header, p.SupportsBinary)
		if err != nil {
			return err
		}

		if _, err := preparedWriter.Write(data); err != nil {
			return err
		}

		preparedWriter.Close()

		p.pollReady <- true
		return nil
	}

	//TODO: make compression threshold configurable
	COMPRESSION_THRESHOLD := 1024
	if header.IsBinary {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	}

	if !p.HttpCompression || len(data) < COMPRESSION_THRESHOLD {
		return respond(data)
	}

	encodingSupported := ""
	for _, encoding := range r.TransferEncoding {
		if encoding == "gzip" || encoding == "deflate" {
			encodingSupported = encoding
			break
		}
	}

	switch encodingSupported {
	case "gzip":
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		if _, err := writer.Write(data); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		writer.Close()
		data = buf.Bytes()
		w.Header().Set("Content-Encoding", encodingSupported)
	case "deflate":
		var buf bytes.Buffer
		writer := zlib.NewWriter(&buf)
		if _, err := writer.Write(data); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		writer.Close()
		data = buf.Bytes()
		w.Header().Set("Content-Encoding", encodingSupported)
	}

	return respond(data)
}

func (p *PollingBase) HandleDataRequest(r *http.Request, w http.ResponseWriter) {
	isBinary := r.Header.Get("content-type") == "application/octet-stream"

	length, err := strconv.Atoi(r.Header.Get("Content-Length"))
	if err != nil {
		p.dataReady <- true
		return
	}

	readers, err := parser.DecodePayload(r.Body, length)
	if err != nil {
		//what now?
		return
	}

	go func() {
		for _, reader := range readers {
			packetType, data, err := parser.AnalyzeReader(reader)
			if err != nil {
				continue
			}

			p.recvPacket <- *packetType
			p.recvData <- data
		}
	}()

	if isBinary {

	}

	p.dataReady <- true
}

func (p *PollingBase) HandleRequest(r *http.Request, w http.ResponseWriter) {
	switch r.Method {
	case "GET":
		select {
		case <-p.pollReady:
			p.HandlePollRequest(r, w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			//this.onError("overlap from client!")
		}
	case "POST":
		select {
		case <-p.dataReady:
			p.HandleDataRequest(r, w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			//this.onError("data erquest overlap from cleitn");
		}
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *PollingBase) SetHeaders(r *http.Request, w http.ResponseWriter) {
	if strings.Contains(r.UserAgent(), ";MSIE") || strings.Contains(r.UserAgent(), "Trident/") {
		w.Header().Set("X-XSS-Protection", "0")
	}
}
