package identity

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTPRequiresTLSUnlessExplicitLocalTest(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(fmt.Sprint(allow), func(t *testing.T) {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = listener.Close() }()
			messages := make(chan string, 1)
			go func() {
				conn, e := listener.Accept()
				if e != nil {
					messages <- ""
					return
				}
				defer func() { _ = conn.Close() }()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = fmt.Fprint(conn, "220 local.test ESMTP\r\n")
				scanner := bufio.NewScanner(conn)
				var body strings.Builder
				data := false
				for scanner.Scan() {
					line := scanner.Text()
					if data {
						if line == "." {
							data = false
							_, _ = fmt.Fprint(conn, "250 accepted\r\n")
						} else {
							body.WriteString(line + "\n")
						}
						continue
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						_, _ = fmt.Fprint(conn, "250 local.test\r\n")
					case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						_, _ = fmt.Fprint(conn, "250 OK\r\n")
					case line == "DATA":
						data = true
						_, _ = fmt.Fprint(conn, "354 send message\r\n")
					case line == "QUIT":
						_, _ = fmt.Fprint(conn, "221 bye\r\n")
						messages <- body.String()
						return
					}
				}
				messages <- body.String()
			}()
			send := SMTPMailer(SMTPConfig{Address: listener.Addr().String(), From: "pluginpocket@example.com", AllowLocalInsecure: allow})
			e = send(context.Background(), "owner@example.com", "https://pluginpocket.example/verify-email?token=one-time")
			if allow && e != nil {
				t.Fatal(e)
			}
			if !allow && e == nil {
				t.Fatal("plaintext SMTP silently accepted")
			}
			select {
			case message := <-messages:
				if allow && !strings.Contains(message, "token=one-time") {
					t.Fatalf("mail body absent: %s", message)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("SMTP fixture stuck")
			}
		})
	}
}
