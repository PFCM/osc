// package server implements an osc server, that receives messages over UDP.
package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"golang.org/x/sync/errgroup"

	"github.com/pfcm/osc"
)

// Handler is something that can handle OSC messages.
type Handler interface {
	Handle(*osc.Message) error
}

// HandlerFunc converts a function into a Handler.
func HandlerFunc(f func(*osc.Message) error) Handler {
	return handlerFunc(f)
}

type handlerFunc func(*osc.Message) error

func (h handlerFunc) Handle(m *osc.Message) error {
	return h(m)
}

// Listener listens to a connection and dispatches messages to registered
// handlers. Each handler may be called in a separate goroutine, even if they
// are handling the same message. Note this means even multiple instances of the
// same handler may be executed concurrently.
type Listener struct {
	conn net.PacketConn
	// TODO: this could definitely be more efficient, but is it worth it?
	handlers []handler
	// workers sets the number of messages handled in parallel. Note this is
	// separate to the total number of message handlers running in parallel,
	// because a message may match many handlers.
	workers int
}

type handler struct {
	p string
	h Handler
}

func NewListener(conn net.PacketConn, workers int) *Listener {
	return &Listener{
		conn:    conn,
		workers: workers,
	}
}

// Handle registers a handler to receive messages on the provided pattern.
func (l *Listener) Handle(pattern string, h Handler) {
	l.handlers = append(l.handlers, handler{pattern, h})
}

// handle actually dispatches an individual message to each of the applicable
// Handlers.
func (l *Listener) handle(msg *osc.Message) error {
	pattern, err := ParsePattern(msg.Pattern)
	if err != nil {
		return err
	}
	for _, m := range l.handlers {
		if pattern.Match(m.p) {
			// TODO: do these concurrently?
			if err := m.h.Handle(msg); err != nil {
				log.Printf("Error from handler %q: %v (message: %v)", m.p, err, msg)
			}
		}
	}
	return nil
}

// Serve starts listening to OSC packets and dispatching them to registered
// handlers. It blocks until the context is cancelled or it receives an error
// from the underlying connection.
func (l *Listener) Serve(ctx context.Context) error {
	recv := make(chan *osc.Message, 100)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		buf := make([]byte, 1<<16) // ~max UDP packet size.
		for {
			n, addr, err := l.conn.ReadFrom(buf)
			if n > 0 {
				msg, err := osc.ParseMessage(buf[:n])
				if err != nil {
					log.Printf("Received invalid message from %v: %v", addr, err)
				}
				select {
				case recv <- msg:
				case <-gctx.Done():
					return gctx.Err()
				}
			}
			if err != nil {
				return err
			}
		}
	})
	for range l.workers {
		g.Go(func() error {
			for {
				var msg *osc.Message
				select {
				case <-gctx.Done():
					return gctx.Err()
				case msg = <-recv:
				}
				if err := l.handle(msg); err != nil {
					log.Printf("Error handling message: %v (message: %v)", err, msg)
				}
			}
		})
	}

	return g.Wait()
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
