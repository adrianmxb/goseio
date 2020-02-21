package transports

import (
	"net/http"
)

type PollingOptions struct {
	SupportsBinary    bool
	MaxHttpBufferSize int
	HttpCompression   bool
}

type XHRPolling struct {
	*PollingBase
}

func (p XHRPolling) HandleRequest(r *http.Request, w http.ResponseWriter) {
	if r.Method == "OPTIONS" {
		p.SetHeaders(r, w)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(200)
		return
	}

	p.SetHeaders(r, w)
	p.PollingBase.HandleRequest(r, w)
}

func (p *XHRPolling) SetHeaders(r *http.Request, w http.ResponseWriter) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	p.PollingBase.SetHeaders(r, w)
}

func NewXHRPolling(pb *PollingBase) Transport {
	return &XHRPolling{
		PollingBase: pb,
	}
}
