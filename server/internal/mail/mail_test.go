package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

// fakeSMTP accepts one message and returns everything the client sent.
func fakeSMTP(t *testing.T) (port string, got <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	out := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var all strings.Builder
		reply := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		reply("220 fake")
		sc, inData := bufio.NewScanner(conn), false
		for sc.Scan() {
			line := sc.Text()
			all.WriteString(line + "\n")
			switch {
			case inData && line == ".":
				inData = false
				reply("250 queued")
			case inData:
			case line == "DATA":
				inData = true
				reply("354 go")
			case line == "QUIT":
				reply("221 bye")
				out <- all.String()
				return
			default:
				reply("250 ok")
			}
		}
	}()
	_, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port, out
}

func TestSend(t *testing.T) {
	port, got := fakeSMTP(t)
	s := SMTP{Host: "127.0.0.1", Port: port, From: "Orbit <no-reply@example.com>"}
	if err := s.Send(context.Background(), "a@example.com", "Sign in", "link\n"); err != nil {
		t.Fatal(err)
	}
	sent := <-got
	for _, want := range []string{"MAIL FROM:<no-reply@example.com>", "RCPT TO:<a@example.com>", "Subject: Sign in", "link"} {
		if !strings.Contains(sent, want) {
			t.Errorf("missing %q in:\n%s", want, sent)
		}
	}
}

func TestSendRejectsHeaderInjection(t *testing.T) {
	s := SMTP{Host: "127.0.0.1", Port: "1", From: "no-reply@example.com"}
	for _, to := range []string{"a@example.com\r\nBcc: b@example.com", "a@example.com\n"} {
		err := s.Send(context.Background(), to, "x", "y")
		if err == nil || !strings.Contains(err.Error(), "line break") {
			t.Errorf("Send(%q) = %v", to, err)
		}
	}
}

// "localhost." resolves to this machine but isn't treated as local, like a mail
// server elsewhere that offers no encryption.
func TestPlaintextNeedsPermission(t *testing.T) {
	tests := []struct {
		allow   bool
		wantErr bool
	}{{false, true}, {true, false}}
	for _, tt := range tests {
		port, _ := fakeSMTP(t)
		s := SMTP{Host: "localhost.", Port: port, From: "no-reply@example.com", AllowPlaintext: tt.allow}
		if err := s.Send(context.Background(), "a@example.com", "x", "y"); (err != nil) != tt.wantErr {
			t.Errorf("allow=%v: %v", tt.allow, err)
		}
	}
}
