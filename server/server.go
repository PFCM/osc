// package server implements an osc server, that receives messages over UDP.
package server

import (
	"errors"
	"fmt"
	"net"

	"github.com/pfcm/osc"
)

// Handler is something that can handle OSC messages.
type Handler interface {
	Handle(osc.Message) error
}

// HandlerFunc converts a function into a Handler.
func HandlerFunc(f func(osc.Message) error) Handler {
	return handlerFunc(f)
}

type handlerFunc func(osc.Message) error

func (h handlerFunc) Handle(m osc.Message) error {
	return h(m)
}

type Listener struct {
	conn net.PacketConn
}

// Handle registers a handler to receive messages on the provided pattern.
func (l *Listener) Handle(pattern string, h Handler) {}

// handlerNode is a node in the handler tree, it has a portion of the message
// pattern and either children or a handler.
type handlerNode struct {
	pattern  string
	children []*handlerNode
	handler  Handler
}

func (h *handlerNode) dispatch(pattern []string, msg osc.Message) error {
	if len(pattern) == 0 {
		if h.handler == nil {
			return unmatched(msg)
		}
		return h.handler.Handle(msg)
	}
	return errors.New("no")
}

type UnmatchedPatternError struct {
	msg osc.Message
}

func unmatched(msg osc.Message) UnmatchedPatternError {
	return UnmatchedPatternError{msg}
}

func (u UnmatchedPatternError) Error() string {
	return fmt.Sprintf("no handlers for message: %v", u.msg)
}
