package eio

import (
	"bytes"
	"github.com/adrianmxb/goseio/pkg/eio/transport"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Config struct {
	PingTimeout    int
	PingInterval   int
	UpgradeTimeout int
	//MaxHttpBufferSize ???
}

type ConnectHandlerFunc func(socket *Socket)
type MessageHandlerFunc func(socket *Socket, data []byte, isBinary bool)

const (
	UnknownTransport = iota
	UnknownSid
	BadHandshakeMethod
	BadRequest
	Forbidden
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

type RequestError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	clientsMutex sync.RWMutex
	clients      map[string]*Socket
	errors       map[int][]byte

	MsgHandler     MessageHandlerFunc
	ConnectHandler ConnectHandlerFunc

	Path         string
	PingInterval time.Duration
	PingTimeout  time.Duration

	// for sio 2.0 supports
	initialPacket []byte

	//poll
	MaxHttpBufferSize uint64
	HttpCompression   bool

	//ws
	ws                websocket.Upgrader
	PerMessageDeflate bool
}

func NewServer(path string, packet bytes.Buffer) (*Server, error) {
	pi := 20000
	pt := 20000
	deflate := false
	s := &Server{
		clients: make(map[string]*Socket),
		errors:  make(map[int][]byte),

		MsgHandler:     func(socket *Socket, data []byte, isBinary bool) {},
		ConnectHandler: func(socket *Socket) {},

		Path:              path + "/",
		PerMessageDeflate: deflate,
		ws: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			EnableCompression: deflate,
		},

		initialPacket: packet.Bytes(),

		PingInterval: time.Duration(pi) * time.Millisecond,
		PingTimeout:  time.Duration(pt) * time.Millisecond,
	}

	if b, err := json.Marshal(&RequestError{Code: UnknownTransport, Message: "Transport unknown"}); err != nil {
		return nil, err
	} else {
		s.errors[UnknownTransport] = b
	}

	if b, err := json.Marshal(&RequestError{Code: UnknownSid, Message: "Session ID unknown"}); err != nil {
		return nil, err
	} else {
		s.errors[UnknownSid] = b
	}

	if b, err := json.Marshal(&RequestError{Code: BadHandshakeMethod, Message: "Bad handshake method"}); err != nil {
		return nil, err
	} else {
		s.errors[BadHandshakeMethod] = b
	}

	if b, err := json.Marshal(&RequestError{Code: BadRequest, Message: "Bad request"}); err != nil {
		return nil, err
	} else {
		s.errors[BadRequest] = b
	}

	if b, err := json.Marshal(&RequestError{Code: Forbidden, Message: "Forbidden"}); err != nil {
		return nil, err
	} else {
		s.errors[Forbidden] = b
	}
	return s, nil
}

//if sync is set the message handler gets called in a synchronized manner so you don't have to
//synchronize access to data.
func (s *Server) OnMessage(handlerFunc MessageHandlerFunc) {
	s.MsgHandler = handlerFunc
}

func (s *Server) OnConnection(handlerFunc ConnectHandlerFunc) {
	s.ConnectHandler = handlerFunc
}

func (s *Server) VerifyRequest(query url.Values, r *http.Request, upgrade bool) (bool, int) {
	//eio := query.Get("eio")
	transport := query.Get("transport")
	sid := query.Get("sid")

	if transport != "polling" && transport != "websocket" {
		return false, UnknownTransport
	}

	//TODO: validate origin header?

	if sid != "" {
		s.clientsMutex.RLock()
		_ /*client*/, ok := s.clients[sid]
		s.clientsMutex.RUnlock()
		if !ok {
			s.clientsMutex.RUnlock()
			return false, UnknownSid
		} else {
			/* TODO:
			   if (!upgrade && this.clients[sid].Transport.name !== Transport) {
			     debug('bad request: unexpected Transport without upgrade');
			     return fn(Server.errors.BAD_REQUEST, false);
			   }
			*/
		}
	} else {
		if r.Method != "GET" {
			return false, BadHandshakeMethod
		}

		//TODO: add call to optional verification function provided by user. "return s.allowRequest(request)" -> returns ok,errorCode
	}
	return true, -1
}

func (s *Server) SendError(w http.ResponseWriter, r *http.Request, errCode int) {
	w.Header().Set("Content-Type", "application/json")

	err, ok := s.errors[errCode]
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		w.Write(s.errors[Forbidden])
		return
	}

	if origin := r.Header.Get("origin"); origin != "" {
		w.Header().Set("'Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	w.WriteHeader(400)
	w.Write(err)
}

func (s *Server) HandleUpgrade(query url.Values, w http.ResponseWriter, r *http.Request) {
	id := query.Get("sid")
	if id == "" {
		s.Handshake(query, w, r)
		return
	}

	conn, err := s.ws.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.clientsMutex.RLock()
	client, ok := s.clients[id]
	s.clientsMutex.RUnlock()

	if !ok {
		conn.Close()
		return
	}
	if client.upgradeState == UpgradeStateUpgrading ||
		client.upgradeState == UpgradeStateUpgraded {
		conn.Close()
		return
	}

	transport := transport.NewWebsocket(
		transport.WSOptions{
			TransportOptions: transport.TransportOptions{
				SupportsBinary: query.Get("b64") == "",
			},
		}, query.Get("sid"), conn)

	go client.HandleTransport(transport, true)
}

func (s *Server) Handshake(query url.Values, w http.ResponseWriter, r *http.Request) {
	id, err := GenerateID()
	if err != nil {
		return
	}

	var tsp transport.ITransport
	switch query.Get("transport") {
	case "websocket":
		conn, err := s.ws.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		tsp = transport.NewWebsocket(
			transport.WSOptions{
				TransportOptions: transport.TransportOptions{
					SupportsBinary: query.Get("b64") == "",
				},
			}, id, conn)
		break
	case "polling":
		pollingData := transport.PollingOptions{
			TransportOptions: transport.TransportOptions{
				SupportsBinary: query.Get("b64") == "",
			},
			MaxHttpBufferSize: s.MaxHttpBufferSize,
			HttpCompression:   s.HttpCompression,
		}
		if query.Get("j") != "" {
			pollingData.Type = transport.JSONP
			//JSONP
		} else {
			pollingData.Type = transport.XHR
			//XHR
		}
		tsp = transport.NewPolling(pollingData, id)
	default:
		return
	}

	socket := NewSocket(id, s, tsp, r)
	tsp.HandleRequest(r, w)

	s.clientsMutex.Lock()
	s.clients[id] = socket
	s.clientsMutex.Unlock()

	s.ConnectHandler(socket)
	// TODO: handle close!!! (delete from s.clients, ...)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	//TODO: add possibility to add a custom function handle the user can pass by config.
	if r.URL.Path != s.Path {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if ok, errCode := s.VerifyRequest(query, r, false); !ok {
		s.SendError(w, r, errCode)
		return
	}

	s.clientsMutex.RLock()
	client, ok := s.clients[query.Get("sid")]
	s.clientsMutex.RUnlock()

	if ok {
		if r.Header.Get("Connection") == "Upgrade" {
			s.HandleUpgrade(query, w, r)
		} else {
			client.Transport.HandleRequest(r, w)
		}
	} else {
		s.Handshake(query, w, r)
	}

}
