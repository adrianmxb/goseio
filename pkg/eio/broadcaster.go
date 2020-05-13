package eio

import "sync"

type Broadcaster struct {
	Publisher chan<- interface{}

	lock    sync.Mutex
	readers map[chan<- interface{}]struct{}
}
