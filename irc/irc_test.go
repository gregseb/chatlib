package irc_test

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/gregseb/chatlib"
	"github.com/gregseb/chatlib/irc"
	"github.com/pkg/errors"
	"golang.org/x/net/nettest"
)

func TestStartStop(t *testing.T) {
	c := context.Background()
	server, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverAddr := server.Addr().String()
	parts := strings.Split(serverAddr, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	api, err := irc.New(
		irc.WithNetwork(parts[0], port),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Accept the connection and send an init message so the start function
	// knows the connection is live.
	var conn net.Conn
	go func() {
		conn, err = server.Accept()
		if err != nil {
			t.Error(err)
		}
		_, err := conn.Write([]byte(msgInit))
		if err != nil {
			t.Error(err)
		}
	}()
	go api.ReceiveMessage(c)
	if err := api.Start(c); err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := api.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := api.Stop(c); err != nil {
		t.Fatal(err)
	}
	// Test that the irc client has closed its connection
	if err := api.Ping(); err == nil {
		t.Fatal("expected net.OpError, got nil")
	} else if _, ok := err.(*net.OpError); !ok {
		t.Fatalf("expected net.OpError, got %+v", err)
	} else if err.(*net.OpError).Err.Error() != "use of closed network connection" {
		t.Fatalf("expected 'use of closed network connection', got %s", err.(*net.OpError).Err.Error())
	}
}

func TestStartTimeout(t *testing.T) {
	c := context.Background()
	server, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverAddr := server.Addr().String()
	parts := strings.Split(serverAddr, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	api, err := irc.New(
		irc.WithNetwork(parts[0], port),
		irc.WithDialTimeout(0.1),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := api.Start(c); err == nil {
		t.Fatal("expected timeout error, got nil")
	} else if errors.Cause(err) != chatlib.ErrTimeout {
		t.Fatalf("expected timeout error, got %+v", err)
	}
}

func TestLoginAndJoin(t *testing.T) {
	c := context.Background()
	server, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverAddr := server.Addr().String()
	parts := strings.Split(serverAddr, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	api, err := irc.New(
		irc.WithNetwork(parts[0], port),
		irc.WithChannel("#test"),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Accept the connection and send an init message so the start function
	// knows the connection is live.
	var conn net.Conn
	go func() {
		conn, err = server.Accept()
		if err != nil {
			t.Error(err)
		}
		_, err := conn.Write([]byte(msgInit))
		if err != nil {
			t.Error(err)
		}
	}()
	var msgChain string
	go func() {
		msg, err := api.ReceiveMessage(c)
		if err != nil {
			t.Error(err)
		}
		msgChain += msg.Raw
	}()
	if err := api.Start(c); err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer api.Stop(c)

	for i := 0; i < 3; i++ {
		msg, err := api.ReceiveMessage(c)
		if err != nil {
			t.Fatal(err)
		}
		msgChain += msg.Raw
	}
	if msgChain != msgInit {
		t.Fatal("expected init message")
	}
	msgChain = ""
	r := bufio.NewReader(conn)
	// Check for login messages on server
	for i := 0; i < 2; i++ {
		msg, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		msgChain += msg
	}
	exp := msgNick + msgUser
	if msgChain != exp {
		t.Fatalf("expected nick and user messages, got %s", msgChain)
	}
	// Send accept messages from server to client
	_, err = conn.Write([]byte(msgAccept))
	if err != nil {
		t.Fatal(err)
	}
	// Receive accept messages from client
	msgChain = ""
	for i := 0; i < 5; i++ {
		msg, err := api.ReceiveMessage(c)
		if err != nil {
			t.Fatal(err)
		}
		msgChain += msg.Raw
	}
	if msgChain != msgAccept {
		t.Fatalf("expected accept messages, got %s", msgChain)
	}
}

func TestPing(t *testing.T) {
	c := context.Background()
	server, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverAddr := server.Addr().String()
	parts := strings.Split(serverAddr, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	api, err := irc.New(
		irc.WithNetwork(parts[0], port),
		irc.WithChannel("#test"),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Accept the connection and send an init message so the start function
	// knows the connection is live.
	var conn net.Conn
	go func() {
		conn, err = server.Accept()
		if err != nil {
			t.Error(err)
		}
		_, err := conn.Write([]byte(msgInit))
		if err != nil {
			t.Error(err)
		}
	}()
	go api.ReceiveMessage(c)
	if err := api.Start(c); err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer api.Stop(c)

	for i := 0; i < 3; i++ {
		_, err := api.ReceiveMessage(c)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := bufio.NewReader(conn)
	// Read and throw away NICK and USER commands on server
	r.ReadString('\n')
	r.ReadString('\n')
	// Send ping message from server to client
	_, err = conn.Write([]byte(msgPing))
	if err != nil {
		t.Fatal(err)
	}
	// Receive ping message from client
	msg, err := api.ReceiveMessage(c)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Raw != msgPing {
		t.Fatalf("expected ping message, got %s", msg.Raw)
	}
	// Check for pong message on server
	sm, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if sm != msgPong {
		t.Fatalf("expected pong message, got %s", sm)
	}
}

// Test IRC Server Messages
const (
	msgInit   = ":irc.test.foo NOTICE * :*** Looking up your hostname...\r\n:irc.test.foo NOTICE * :*** Checking Ident\r\n:irc.test.foo NOTICE * :*** Couldn't look up your hostname\r\n:irc.test.foo NOTICE * :*** No Ident response\r\n"
	msgPing   = "PING :irc.test.foo\r\n"
	msgAccept = ":irc.test.foo 001 freyabot :Welcome to the freyabot IRC Network\r\n:irc.test.foo 002 freyabot :Your host is irc.test.foo, running version test123\r\n:irc.test.foo 003 freyabot :This server was created Jun 27 2022 at 15:27:35\r\n:irc.test.foo 004 freyabot irc.test.foo test123 asdf fdsa afsd\r\n:irc.test.foo 005 freyabot CALLERID CASEMAPPING=rfc1459 DEAF=D KICKLEN=180 MODES=4 PREFIX=(qaohv)~&@%+ STATUSMSG=~&@%%+ EXCEPTS=e INVEX=I NICKLEN=30 NETWORK=Rizon MAXLIST=beI:250 MAXTARGETS=4 :are supported by this server\r\n"
)

// Test IRC Client Messages
const (
	msgNick = "NICK freyabot\n"
	msgUser = "USER freyabot 0 * :FreyaBot\n"
	msgPong = "PONG :irc.test.foo\n"
)
