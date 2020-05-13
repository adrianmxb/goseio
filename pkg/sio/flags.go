package sio

const (
	FlagJson     = 1 << 0
	FlagVolatile = 1 << 1
	FlagLocal    = 1 << 2
)

type EmitData struct {
	flags int
	rooms map[string]bool
}
