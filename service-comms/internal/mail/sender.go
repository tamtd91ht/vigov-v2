// Package mail sends a message through a commune's own SMTP server. It is the FIRST OUTBOUND MAIL
// PATH in this service, and today it carries exactly one message: the fixed test message of
// `Cấu hình → Máy chủ thư → Gửi thử` (docs/ui-ux/14-cau-hinh.md §10). That message holds NO
// CITIZEN DATA — a fixed subject and body, to an address the administrator types.
//
// WHAT CROSSES TO THE THIRD PARTY (rule 3, invariant 6 — declared here, in the adapter): the
// commune's SMTP account name and password (AUTH PLAIN, only ever inside TLS), the From address and
// display name, the one recipient, and the fixed subject and body. Nothing else.
//
// TRANSPORT SECURITY IS NOT CONFIGURABLE DOWNWARDS (rule 13 #1):
//
//	starttls   plain TCP, then STARTTLS BEFORE AUTH. A server that does not offer STARTTLS is
//	           refused — there is no fallback to sending the password in the clear.
//	tls        TLS from the first byte.
//	anything   refused before dialling.
//
// Certificates are ALWAYS verified against the host name, TLS 1.2 minimum. There is no
// InsecureSkipVerify anywhere in this package and no switch that would add one; the only knob is
// the root pool, which exists so a test can trust its own local server.
//
// WHAT NEVER LEAVES THIS PACKAGE: the server's reply TEXT. An SMTP error line can echo the account
// name, the envelope, or a banner naming the server software; the errors returned here carry the
// category and the three-digit reply code only. That is a deliberate redaction, not a swallowed
// error — the category is what the administrator can act on, and the code is what an operator
// needs to look it up.
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// DefaultTimeout bounds one whole exchange — dial, handshake, AUTH, DATA, QUIT. An administrator is
// waiting on the other side of the button; a server that answers nothing must not hold the request.
const DefaultTimeout = 15 * time.Second

// Security modes — the same values as domain.MailSecurity* (this package does not import domain so
// it stays an adapter with no business rules in it).
const (
	SecurityStartTLS = "starttls"
	SecurityTLS      = "tls"
)

// The failure categories. Each is a sentence the handler can turn into advice.
var (
	ErrPlaintextRefused  = errors.New("mail: a connection without TLS is refused")
	ErrConnect           = errors.New("mail: cannot connect to the server")
	ErrTimeout           = errors.New("mail: the server did not answer in time")
	ErrCertificate       = errors.New("mail: the server certificate cannot be verified")
	ErrTLS               = errors.New("mail: the TLS handshake failed")
	ErrStartTLSMissing   = errors.New("mail: the server does not offer STARTTLS")
	ErrAuthUnsupported   = errors.New("mail: the server offers no usable authentication")
	ErrAuthRejected      = errors.New("mail: the server rejected the account or password")
	ErrRecipientRejected = errors.New("mail: the server rejected the sender or the recipient")
	ErrProtocol          = errors.New("mail: the server answered outside the protocol")
)

// Account is everything needed to reach one commune's server.
type Account struct {
	Host     string
	Port     int
	Security string
	Username string
	Password secret.Secret
}

// Message is one plain-text message. From/To are bare addresses already validated by the caller.
type Message struct {
	FromAddress string
	FromName    string
	To          string
	Subject     string
	Body        string
}

// Sender is safe for concurrent use.
type Sender struct {
	roots   *x509.CertPool
	timeout time.Duration
	now     func() time.Time
}

// NewSender builds a Sender. roots nil means the system trust store — the production value.
func NewSender(roots *x509.CertPool, timeout time.Duration) *Sender {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Sender{roots: roots, timeout: timeout, now: time.Now}
}

// Send delivers one message, or returns one of the Err* categories above (wrapped with the SMTP
// reply code where there is one).
func (s *Sender) Send(ctx context.Context, a Account, m Message) error {
	if a.Security != SecurityStartTLS && a.Security != SecurityTLS {
		return ErrPlaintextRefused
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	tlsCfg := &tls.Config{ServerName: a.Host, MinVersion: tls.VersionTLS12, RootCAs: s.roots}
	addr := net.JoinHostPort(a.Host, strconv.Itoa(a.Port))
	dialer := &net.Dialer{Timeout: s.timeout}

	var conn net.Conn
	var err error
	if a.Security == SecurityTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return classifyDial(err)
		}
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return classifyDial(err)
		}
	}
	defer conn.Close()
	// ONE deadline for the whole exchange, on the socket — net/smtp takes no context.
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("%w: set deadline", ErrConnect)
	}

	c, err := smtp.NewClient(conn, a.Host)
	if err != nil {
		return classifySMTP(err, ErrProtocol)
	}
	defer c.Close()

	if a.Security == SecurityStartTLS {
		ok, _ := c.Extension("STARTTLS")
		if !ok {
			return ErrStartTLSMissing
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return classifyTLS(err)
		}
	}
	// From here on the connection is TLS in both modes. net/smtp.PlainAuth refuses a non-TLS
	// connection on its own too; this check says so first, in our words.
	if st, ok := c.TLSConnectionState(); !ok || !st.HandshakeComplete {
		return ErrPlaintextRefused
	}

	ok, mechs := c.Extension("AUTH")
	if !ok || !hasMechanism(mechs, "PLAIN") {
		return ErrAuthUnsupported
	}
	// net/smtp takes the password as a string; this is the one conversion, and it lives only for
	// the duration of this call.
	if err := c.Auth(smtp.PlainAuth("", a.Username, string(a.Password.Lo()), a.Host)); err != nil {
		return classifySMTP(err, ErrAuthRejected)
	}
	if err := c.Mail(m.FromAddress); err != nil {
		return classifySMTP(err, ErrRecipientRejected)
	}
	if err := c.Rcpt(m.To); err != nil {
		return classifySMTP(err, ErrRecipientRejected)
	}
	w, err := c.Data()
	if err != nil {
		return classifySMTP(err, ErrRecipientRejected)
	}
	if _, err := w.Write(s.build(m)); err != nil {
		return classifySMTP(err, ErrProtocol)
	}
	if err := w.Close(); err != nil {
		return classifySMTP(err, ErrRecipientRejected)
	}
	// A failed QUIT after the server accepted DATA is not a failed delivery.
	_ = c.Quit()
	return nil
}

// build renders the message. Every header value is either validated by the caller (addresses) or
// MIME-encoded here (name, subject), and the body is base64 — so no field can inject a header line.
func (s *Sender) build(m Message) []byte {
	var b bytes.Buffer
	from := "<" + m.FromAddress + ">"
	if m.FromName != "" {
		from = mime.BEncoding.Encode("utf-8", m.FromName) + " " + from
	}
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: <%s>\r\n", m.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.BEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", s.now().UTC().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(m.Body))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.Bytes()
}

func hasMechanism(list, want string) bool {
	for _, m := range strings.Fields(list) {
		if strings.EqualFold(m, want) {
			return true
		}
	}
	return false
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

func isCertificate(err error) bool {
	var (
		verr *tls.CertificateVerificationError
		uerr x509.UnknownAuthorityError
		herr x509.HostnameError
		cerr x509.CertificateInvalidError
	)
	return errors.As(err, &verr) || errors.As(err, &uerr) || errors.As(err, &herr) || errors.As(err, &cerr)
}

// classifyDial — a failure before any SMTP was spoken. With implicit TLS the handshake happens
// inside the dial, so certificate and TLS failures are recognised here too.
//
// The underlying error IS kept (both are wrapped): a dial or TLS error names the address and the
// certificate problem — configuration, not a secret — and that is what an operator needs.
func classifyDial(err error) error {
	switch {
	case isTimeout(err):
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	case isCertificate(err):
		return fmt.Errorf("%w: %w", ErrCertificate, err)
	}
	var rec tls.RecordHeaderError
	var alert tls.AlertError
	if errors.As(err, &rec) || errors.As(err, &alert) {
		return fmt.Errorf("%w: %w", ErrTLS, err)
	}
	return fmt.Errorf("%w: %w", ErrConnect, err)
}

func classifyTLS(err error) error {
	switch {
	case isTimeout(err):
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	case isCertificate(err):
		return fmt.Errorf("%w: %w", ErrCertificate, err)
	}
	return fmt.Errorf("%w: %w", ErrTLS, err)
}

// classifySMTP keeps the reply CODE and drops the reply TEXT (see the package comment).
func classifySMTP(err error, category error) error {
	if isTimeout(err) {
		return ErrTimeout
	}
	var te *textproto.Error
	if errors.As(err, &te) {
		return fmt.Errorf("%w (smtp %d)", category, te.Code)
	}
	return category
}
