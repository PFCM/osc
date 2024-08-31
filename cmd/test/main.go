package main

import (
	"context"
	"flag"
	"log"
	"net"

	"github.com/pfcm/osc"
	"github.com/pfcm/osc/server"
)

var (
	modeFlag       = flag.String("mode", "", "`mode` in which to run, must be one of \"send\" or \"receive\"")
	listenAddrFlag = flag.String("listen_addr", "127.0.0.1:0", "`host:port`: the address to listen on.")
	sendAddrFlag   = flag.String("send_addr", "", "`host:port`: the address to send to.")
	patternFlag    = flag.String("pattern", "/test", "`address pattern` to to send a message to, in send mode")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	switch *modeFlag {
	case "send":
		if err := send(ctx); err != nil {
			log.Fatal(err)
		}
	case "receive":
		if err := receive(ctx); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown mode %q", *modeFlag)
	}
}

func send(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", *listenAddrFlag)
	if err != nil {
		return err
	}
	i := osc.Int32(12)
	msg := &osc.Message{
		Pattern:   *patternFlag,
		Arguments: []osc.Argument{&i},
	}
	enc := msg.Append([]byte(nil))
	addr, err := net.ResolveUDPAddr("udp", *sendAddrFlag)
	if err != nil {
		return err
	}
	log.Printf("Sending %v to %v", msg, addr)

	_, err = conn.WriteTo(enc, addr)
	return err
}

func receive(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", *listenAddrFlag)
	if err != nil {
		return err
	}
	log.Printf("Listening on %v", conn.LocalAddr())

	l := server.NewListener(conn, 1)
	for _, p := range []string{
		"/test",
		"/test/a",
		"/test/b",
		"/test/c",
	} {
		l.Handle(p, server.HandlerFunc(func(msg *osc.Message) error {
			log.Printf("%s: recv: %v", p, msg)
			return nil
		}))
	}
	return l.Serve(ctx)
}
