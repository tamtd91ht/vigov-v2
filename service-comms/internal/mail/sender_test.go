package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// A LOCAL SMTP SERVER, speaking just enough of the protocol for net/smtp, over a certificate minted
// here for 127.0.0.1. Nothing leaves the machine.
//
// What it records is the assertion: whether AUTH was ever received decides whether the password
// crossed the wire. Every refusal case below asserts it was NOT.

const fixturePassword = "fixture-not-a-real-password-7Q"

type smtpServer struct {
	ln   net.Listener
	cert tls.Certificate
	pool *x509.CertPool

	implicitTLS   bool // TLS from the first byte (port 465 style)
	offerSTARTTLS bool
	rejectAuth    bool
	silent        bool // accept the connection and never speak

	mu       sync.Mutex
	authSeen []string // decoded AUTH PLAIN payloads
	data     string
	rcpt     string
}

func mintCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "local test smtp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

func startSMTP(t *testing.T, s *smtpServer) *smtpServer {
	t.Helper()
	s.cert, s.pool = mintCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.ln = ln
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *smtpServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *smtpServer) serve(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	cfg := &tls.Config{Certificates: []tls.Certificate{s.cert}}
	if s.implicitTLS {
		tc := tls.Server(c, cfg)
		if err := tc.Handshake(); err != nil {
			return
		}
		c = tc
	}
	if s.silent {
		time.Sleep(2 * time.Second)
		return
	}
	r := bufio.NewReader(c)
	say := func(line string) { _, _ = c.Write([]byte(line + "\r\n")) }
	say("220 local ESMTP")
	isTLS := s.implicitTLS
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch verb {
		case "EHLO", "HELO":
			var ext []string
			if s.offerSTARTTLS && !isTLS {
				ext = append(ext, "STARTTLS")
			}
			ext = append(ext, "AUTH PLAIN")
			say("250-local")
			for i, e := range ext {
				if i == len(ext)-1 {
					say("250 " + e)
				} else {
					say("250-" + e)
				}
			}
		case "STARTTLS":
			say("220 go ahead")
			tc := tls.Server(c, cfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			c, r, isTLS = tc, bufio.NewReader(tc), true
			say = func(line string) { _, _ = c.Write([]byte(line + "\r\n")) }
		case "AUTH":
			parts := strings.Fields(line)
			if len(parts) == 3 {
				dec, _ := base64.StdEncoding.DecodeString(parts[2])
				s.mu.Lock()
				s.authSeen = append(s.authSeen, string(dec))
				s.mu.Unlock()
			}
			if s.rejectAuth {
				say("535 5.7.8 Authentication failed for ubnd@example.test on LocalMail 9.9")
			} else {
				say("235 ok")
			}
		case "MAIL":
			say("250 ok")
		case "RCPT":
			s.mu.Lock()
			s.rcpt = line
			s.mu.Unlock()
			say("250 ok")
		case "DATA":
			say("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.mu.Lock()
			s.data = b.String()
			s.mu.Unlock()
			say("250 queued")
		case "QUIT":
			say("221 bye")
			return
		default:
			say("502 unknown")
		}
	}
}

func (s *smtpServer) sawAuth() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.authSeen) > 0
}

func account(s *smtpServer, security string) Account {
	return Account{Host: "127.0.0.1", Port: s.port(), Security: security, Username: "ubnd@example.test",
		Password: secret.Secret(fixturePassword)}
}

var testMessage = Message{FromAddress: "ubnd@example.test", FromName: "UBND xã Thử", To: "canbo@example.test",
	Subject: "ViGov — thư thử", Body: "Đây là thư thử."}

func TestSendOverImplicitTLSVerifiesAndDelivers(t *testing.T) {
	s := startSMTP(t, &smtpServer{implicitTLS: true})
	if err := NewSender(s.pool, 3*time.Second).Send(context.Background(), account(s, SecurityTLS), testMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(s.authSeen) != 1 || s.authSeen[0] != "\x00ubnd@example.test\x00"+fixturePassword {
		t.Fatalf("AUTH PLAIN payload not as expected (%d seen)", len(s.authSeen))
	}
	if !strings.Contains(s.rcpt, "canbo@example.test") || !strings.Contains(s.data, "Subject: =?utf-8?b?") {
		t.Errorf("rcpt %q / data %q", s.rcpt, s.data)
	}
}

func TestSendOverSTARTTLSUpgradesBeforeAuth(t *testing.T) {
	s := startSMTP(t, &smtpServer{offerSTARTTLS: true})
	if err := NewSender(s.pool, 3*time.Second).Send(context.Background(), account(s, SecurityStartTLS), testMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !s.sawAuth() {
		t.Fatal("no AUTH received")
	}
}

func TestSendRefusesUnverifiedCertificateAndNeverSendsPassword(t *testing.T) {
	for _, mode := range []string{SecurityTLS, SecurityStartTLS} {
		t.Run(mode, func(t *testing.T) {
			s := startSMTP(t, &smtpServer{implicitTLS: mode == SecurityTLS, offerSTARTTLS: true})
			// An EMPTY root pool: the local certificate is untrusted, exactly like a self-signed or
			// intercepted server in production. Verification is ON — there is no way to turn it off.
			err := NewSender(x509.NewCertPool(), 3*time.Second).Send(context.Background(), account(s, mode), testMessage)
			if !errors.Is(err, ErrCertificate) {
				t.Fatalf("err = %v, want ErrCertificate", err)
			}
			if s.sawAuth() {
				t.Fatal("the password was sent to a server whose certificate did not verify")
			}
		})
	}
}

func TestSendRefusesWrongHostName(t *testing.T) {
	s := startSMTP(t, &smtpServer{implicitTLS: true})
	a := account(s, SecurityTLS)
	a.Host = "localhost" // resolves to the server, but the certificate names 127.0.0.1 only
	err := NewSender(s.pool, 3*time.Second).Send(context.Background(), a, testMessage)
	if !errors.Is(err, ErrCertificate) {
		t.Fatalf("err = %v, want a certificate refusal", err)
	}
	if s.sawAuth() {
		t.Fatal("password sent under a certificate for another name")
	}
}

func TestSendRefusesServerWithoutSTARTTLS(t *testing.T) {
	s := startSMTP(t, &smtpServer{offerSTARTTLS: false})
	err := NewSender(s.pool, 3*time.Second).Send(context.Background(), account(s, SecurityStartTLS), testMessage)
	if !errors.Is(err, ErrStartTLSMissing) {
		t.Fatalf("err = %v, want ErrStartTLSMissing", err)
	}
	if s.sawAuth() {
		t.Fatal("fell back to sending the password in the clear")
	}
}

func TestSendRefusesPlaintextModeBeforeDialling(t *testing.T) {
	s := startSMTP(t, &smtpServer{})
	for _, mode := range []string{"", "none", "plain"} {
		if err := NewSender(s.pool, time.Second).Send(context.Background(), account(s, mode), testMessage); !errors.Is(err, ErrPlaintextRefused) {
			t.Errorf("mode %q: err = %v, want ErrPlaintextRefused", mode, err)
		}
	}
	if s.sawAuth() {
		t.Fatal("a plaintext mode reached the server")
	}
}

func TestSendAuthRejectedKeepsCodeDropsServerText(t *testing.T) {
	s := startSMTP(t, &smtpServer{implicitTLS: true, rejectAuth: true})
	err := NewSender(s.pool, 3*time.Second).Send(context.Background(), account(s, SecurityTLS), testMessage)
	if !errors.Is(err, ErrAuthRejected) {
		t.Fatalf("err = %v, want ErrAuthRejected", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "535") {
		t.Errorf("reply code missing: %q", msg)
	}
	for _, leak := range []string{"LocalMail", "ubnd@example.test", fixturePassword} {
		if strings.Contains(msg, leak) {
			t.Errorf("error leaks %q: %q", leak, msg)
		}
	}
}

func TestSendTimesOutOnSilentServer(t *testing.T) {
	s := startSMTP(t, &smtpServer{silent: true})
	start := time.Now()
	err := NewSender(s.pool, 300*time.Millisecond).Send(context.Background(), account(s, SecurityStartTLS), testMessage)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout not honoured: %v", time.Since(start))
	}
}

func TestSendConnectFailure(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	a := Account{Host: "127.0.0.1", Port: port, Security: SecurityTLS, Username: "u", Password: secret.Secret("x")}
	if err := NewSender(nil, time.Second).Send(context.Background(), a, testMessage); !errors.Is(err, ErrConnect) {
		t.Fatalf("err = %v, want ErrConnect", err)
	}
}

func TestBuildEncodesHeadersSoNoFieldCanInjectOne(t *testing.T) {
	m := testMessage
	m.Subject = "a\r\nBcc: x@example.test"
	m.FromName = "b\r\nBcc: y@example.test"
	out := string(NewSender(nil, time.Second).build(m))
	for _, line := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "bcc:") {
			t.Fatalf("header injected: %q", out)
		}
	}
}
