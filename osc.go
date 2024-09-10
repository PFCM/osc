// package osc sends and receives Open Sound Control messages, per the
// OSC 1.0 spec (https://ccrma.stanford.edu/groups/osc/spec-1_0.html)
package osc

import (
	"net"
	"sync"

	"golang.org/x/exp/constraints"
)

// Send builds and sends a message using the provided arguments, to the given
// pattern at the provided address.
// TODO: not a great api?
func Send(conn net.PacketConn, addr, pattern string, args ...Argument) error {
	nAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	msg := Message{
		Pattern:   pattern,
		Arguments: args,
	}
	b := getBuf()
	b = msg.Append(b)
	defer putBuf(b)
	_, err = conn.WriteTo(b, nAddr)
	return err
}

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 1024)
		return &b
	},
}

func getBuf() []byte {
	b := bufPool.Get().(*[]byte)
	return (*b)[:0]
}

func putBuf(b []byte) {
	bufPool.Put(&b)
}

func AsString(s string) *String {
	os := String(s)
	return &os
}

func AsInt32[T constraints.Integer](i T) *Int32 {
	ii := Int32(i)
	return &ii
}
