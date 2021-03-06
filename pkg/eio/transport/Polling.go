package transport

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"github.com/adrianmxb/goseio/pkg/eio/packet"
	"github.com/adrianmxb/goseio/pkg/eio/parser"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PollingType int

const (
	XHR   PollingType = 0
	JSONP PollingType = 1
)

var payloadBufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type PollingOptions struct {
	TransportOptions
	Type              PollingType
	MaxHttpBufferSize uint64
	HttpCompression   bool
}

type Polling struct {
	*Transport
	PollingOptions PollingOptions

	dataReady     chan bool
	pollReady     chan bool
	finishPayload chan bool
	readDeadline  chan time.Time
}

func NewPolling(opt PollingOptions, sid string) *Polling {
	polling := &Polling{
		Transport:      NewTransport(opt.TransportOptions),
		PollingOptions: opt,

		pollReady:    make(chan bool, 1),
		dataReady:    make(chan bool, 1),
		readDeadline: make(chan time.Time),
	}

	// TODO: is this correct?
	if opt.Type == JSONP {
		polling.SupportsBinary = false
	}

	// make sure poll and data channel are set ready at the beginning.
	polling.pollReady <- true
	polling.dataReady <- true

	go polling.startModeratorBuddy()

	go func() {
		dead := false
		deadline := <-polling.readDeadline
		for {
			if dead {
				deadline = <-polling.readDeadline
				continue
			}

			select {
			case <-time.After(deadline.Sub(time.Now())):
				dead = true
				fmt.Println("reached deadline, closing transport. REWORK THIS!")
				polling.tspModSignal <- "closing"
			case deadline = <-polling.readDeadline:
			}
		}
	}()

	return polling
}

func (p *Polling) startPayloadWriter() {

}

func (p *Polling) startModeratorBuddy() {
	ackClosing := false
	for {
		select {
		case <-p.tspClosing:
			if !ackClosing {
				ackClosing = true
				p.Send(packet.Packet{
					PacketType: packet.Close,
					IsBinary:   false,
				}, nil, true)
			}

		case <-p.tspModerator:
			return
		}
	}
}

func (p *Polling) GetName() string {
	return "polling"
}

func (p *Polling) SetReadDeadline(t time.Time) error {
	p.readDeadline <- t
	return nil
}

func (p *Polling) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

func (p *Polling) HandlePollRequest(r *http.Request, w http.ResponseWriter) error {
	pack := <-p.sendPacket
	data := <-p.sendData
	interrupt := false

	select {
	case <-p.tspModerator:
		pack = packet.Packet{PacketType: packet.Noop, IsBinary: false}
		data = nil
		interrupt = true
	default:
		select {
		case <-p.tspClosing:
			pack = packet.Packet{PacketType: packet.Close, IsBinary: false}
			data = nil
			interrupt = true
		default:
		}
	}

	hasBinary := pack.IsBinary
	packSlice := append([]packet.Packet{}, pack)
	dataSlice := append([][]byte{}, data)

	if !interrupt {
	Loop:
		for {
			select {
			case pack := <-p.sendPacket:
				packSlice = append(packSlice, pack)
				dataSlice = append(dataSlice, <-p.sendData)
				if pack.IsBinary {
					hasBinary = true
				}
			default:
				break Loop
			}
		}
	}

	//TODO: is it a good idea to pool this?
	buf := payloadBufPool.Get().(*bytes.Buffer)
	defer payloadBufPool.Put(buf)
	buf.Reset()
	writer := bufio.NewWriter(buf)

	preparedWriter := parser.PrepareWriter(writer, hasBinary, p.SupportsBinary)
	for i, pack := range packSlice {
		length, err := parser.EncodePayloadLength(writer, pack, dataSlice[i], p.SupportsBinary)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Length", strconv.Itoa(length))
		err = parser.WriteHeader(writer, pack, p.SupportsBinary)
		if err != nil {
			return err
		}
		if _, err := preparedWriter.Write(dataSlice[i]); err != nil {
			return err
		}
		preparedWriter.Close()
	}
	writer.Flush()

	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	if p.PollingOptions.Type == JSONP {
		buf := bytes.NewBuffer(nil)
		writer := bufio.NewWriter(buf)
		jsonp := r.URL.Query().Get("j")

		w.Header().Set("Content-Type", "text/javascript; charset=UTF-8")
		p.pollReady <- true
		w.Write([]byte("___eio[" + jsonp + "](\""))
		w.Write(buf.Bytes())
		writer.Flush()

		template.JSEscape(w, buf.Bytes())
		w.Write([]byte("\");"))
		return nil
	}

	//TODO: make compression threshold configurable
	COMPRESSION_THRESHOLD := 1024
	if hasBinary {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	}

	if !p.PollingOptions.HttpCompression || buf.Len() < COMPRESSION_THRESHOLD {
		p.pollReady <- true
		w.Write(buf.Bytes())
		return nil
	}

	encodingSupported := ""
	for _, encoding := range r.TransferEncoding {
		if encoding == "gzip" || encoding == "deflate" {
			encodingSupported = encoding
			break
		}
	}

	ebuf := new(bytes.Buffer)
	switch encodingSupported {
	case "gzip":
		writer := gzip.NewWriter(ebuf)
		if _, err := writer.Write(buf.Bytes()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		writer.Close()
	case "deflate":
		writer := zlib.NewWriter(ebuf)
		if _, err := writer.Write(buf.Bytes()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}
		writer.Close()
	}
	w.Header().Set("Content-Encoding", encodingSupported)

	p.pollReady <- true
	w.Write(ebuf.Bytes())
	return nil
}

var regexSlashes, _ = regexp.Compile(`/(\\)?\\n/g`)
var regexDoubleSlashes, _ = regexp.Compile(`/\\\\n/g`)

func (p *Polling) HandleDataRequest(r *http.Request, w http.ResponseWriter) {
	//isBinary := r.Header.Get("content-type") == "application/octet-stream"
	length, err := strconv.Atoi(r.Header.Get("Content-Length"))
	if err != nil {
		p.dataReady <- true
		return
	}

	//this sucks and is really expensive.
	if p.PollingOptions.Type == JSONP {
		/*
			body, _ := ioutil.ReadAll(r.Body)
			parsedBody, _ := url.ParseQuery(string(body))
			data := parsedBody.Get("d")
		*/

		//TODO: regex!
		/*
		   data = data.replace(rSlashes, function (match, slashes) {
		     return slashes ? match : '\n';
		   });
		   Polling.prototype.onData.call(this, data.replace(rDoubleSlashes, '\\n'));
		*/
		fmt.Println("JSONP HandleDataRequest not implemented, regex missing! feel free to implement.")
		p.dataReady <- true
		return
	}

	readers, err := parser.DecodePayload(r.Body, length)
	if err != nil {
		//what now?
		return
	}

	select {
	case <-p.tspClosing:
		p.dataReady <- true
		return
	default:
	}

	go func() {
		for _, reader := range readers {
			pack, data, err := parser.AnalyzeReader(reader)
			if err != nil {
				continue
			}

			if pack.PacketType == packet.Close {
				//handle close?
			}

			p.recvPacket <- *pack
			p.recvData <- data
		}
	}()

	p.dataReady <- true
}

func (p *Polling) HandleRequest(r *http.Request, w http.ResponseWriter) {
	switch r.Method {
	case "OPTIONS":
		if p.PollingOptions.Type == XHR {
			p.SetHeaders(r, w)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(200)
		}
	case "GET":
		p.SetHeaders(r, w)
		select {
		case <-p.pollReady:
			p.HandlePollRequest(r, w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "POST":
		p.SetHeaders(r, w)
		select {
		case <-p.dataReady:
			p.HandleDataRequest(r, w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *Polling) SetHeaders(r *http.Request, w http.ResponseWriter) {
	if strings.Contains(r.UserAgent(), ";MSIE") || strings.Contains(r.UserAgent(), "Trident/") {
		w.Header().Set("X-XSS-Protection", "0")
	}

	if p.PollingOptions.Type == XHR {
		origin := r.Header.Get("Origin")
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
	}
}
