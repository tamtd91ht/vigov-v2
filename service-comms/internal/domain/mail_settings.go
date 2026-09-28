package domain

// The commune's own mail server — docs/ui-ux/14-cau-hinh.md §10 ("Máy chủ thư").
//
// WHAT IS VALIDATED HERE, AND WHY EACH RULE EXISTS:
//
//	security    starttls | tls ONLY. §10's table sketch lists 'none'; it is refused because the
//	            password crosses this connection (rule 13 #1: traffic encrypted and verified).
//	port        25 · 465 · 587 · 2525 ONLY. The test-send action connects from INSIDE the cluster
//	            to whatever this row names; a free port number turns that button into a probe of
//	            internal services (5432, 6379, 9090 …). These four are the ports SMTP is served
//	            on; anything else is not a mail server a commune would be given.
//	from_name   no control characters: it is written into a message header, and a CR/LF there is
//	            header injection — the classic way a configuration field adds a Bcc.
//	addresses   a BARE address ("ubnd@xa.gov.vn"), never "Name <addr>", for the same reason.
//	password    write-only, 1..256 bytes, no NUL: AUTH PLAIN separates its fields with NUL, so a
//	            NUL inside the password would silently change which account is asked for.

import (
	"errors"
	"net"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The two transport-security modes (values per ADR 0011: lower-case ASCII).
const (
	MailSecurityStartTLS = "starttls" // plain connect, then STARTTLS before anything else — port 587
	MailSecurityTLS      = "tls"      // TLS from the first byte — port 465
)

// DefaultMailPort and DefaultMailSecurity are what an unconfigured commune's screen shows: §10 marks
// STARTTLS "mặc định" and its example port is 587.
const (
	DefaultMailPort     = 587
	DefaultMailSecurity = MailSecurityStartTLS
)

// allowedMailPorts — see the file header.
var allowedMailPorts = map[int]bool{25: true, 465: true, 587: true, 2525: true}

// Bounds. The column CHECKs in migrations/0008 hold the same numbers.
const (
	mailHostMax     = 253
	mailUsernameMax = 254
	mailAddressMax  = 254
	mailFromNameMax = 100
	mailPasswordMax = 256
)

// MailSettings is one commune's mail server as the application sees it. THERE IS NO PASSWORD FIELD:
// the password exists in this process only as a secret on its way to being sealed or to an SMTP
// AUTH, never as part of a value that could be returned, logged or compared.
type MailSettings struct {
	Host        string
	Port        int
	Security    string
	Username    string
	FromAddress string
	FromName    string
	IsEnabled   bool
	// PasswordSet reports whether a sealed password is stored. It is the only thing any read says
	// about the password (§12.6).
	PasswordSet bool
}

// MailSettingsSubject is the audit subject: one row per commune, so the business address of the row
// is the configuration itself. A VALUE, not an identifier — Vietnamese, like every action this
// service writes (ADR 0011).
const MailSettingsSubject = "cau_hinh_may_chu_thu"

var (
	ErrMailHostEmpty       = errors.New("may_chu_thu: thiếu `host` (máy chủ SMTP)")
	ErrMailHostShape       = errors.New("may_chu_thu: `host` phải là tên máy (ví dụ smtp.xa.gov.vn) hoặc địa chỉ IP, không kèm cổng hay giao thức")
	ErrMailPortNotAllowed  = errors.New("may_chu_thu: `port` phải là 587 (START TLS), 465 (TLS ngay từ đầu), 25 hoặc 2525")
	ErrMailSecurityUnknown = errors.New("may_chu_thu: `security` phải là starttls hoặc tls — hệ thống không gửi thư qua kết nối không mã hoá")
	ErrMailUsernameEmpty   = errors.New("may_chu_thu: thiếu `username` (tài khoản)")
	ErrMailUsernameShape   = errors.New("may_chu_thu: `username` quá dài hoặc chứa ký tự điều khiển")
	ErrMailFromAddress     = errors.New("may_chu_thu: `from_address` phải là một địa chỉ thư điện tử hợp lệ, không kèm tên hiển thị")
	ErrMailFromName        = errors.New("may_chu_thu: `from_name` tối đa 100 ký tự, không chứa ký tự xuống dòng hay ký tự điều khiển")
	ErrMailPasswordShape   = errors.New("may_chu_thu: `password` tối đa 256 byte và không được chứa ký tự NUL")
	ErrMailRecipient       = errors.New("may_chu_thu: `recipient` phải là một địa chỉ thư điện tử hợp lệ, không kèm tên hiển thị")
)

// MailSettingsInput is what the screen sends, before validation. No password: that travels
// separately, as a secret, and is validated by ValidateMailPassword.
type MailSettingsInput struct {
	Host        string
	Port        int
	Security    string
	Username    string
	FromAddress string
	FromName    string
	IsEnabled   bool
}

// NormalizeMailSettings validates and normalises the non-secret fields. The host is lower-cased so
// the "destination changed" comparison is not fooled by capitalisation.
func NormalizeMailSettings(in MailSettingsInput) (MailSettings, error) {
	host := strings.ToLower(strings.TrimSpace(in.Host))
	if host == "" {
		return MailSettings{}, ErrMailHostEmpty
	}
	if !validMailHost(host) {
		return MailSettings{}, ErrMailHostShape
	}
	if !allowedMailPorts[in.Port] {
		return MailSettings{}, ErrMailPortNotAllowed
	}
	if in.Security != MailSecurityStartTLS && in.Security != MailSecurityTLS {
		return MailSettings{}, ErrMailSecurityUnknown
	}
	user := strings.TrimSpace(in.Username)
	if user == "" {
		return MailSettings{}, ErrMailUsernameEmpty
	}
	if utf8.RuneCountInString(user) > mailUsernameMax || hasControl(user) {
		return MailSettings{}, ErrMailUsernameShape
	}
	from, err := NormalizeMailAddress(in.FromAddress)
	if err != nil {
		return MailSettings{}, ErrMailFromAddress
	}
	name := strings.TrimSpace(in.FromName)
	if utf8.RuneCountInString(name) > mailFromNameMax || hasControl(name) {
		return MailSettings{}, ErrMailFromName
	}
	return MailSettings{
		Host: host, Port: in.Port, Security: in.Security, Username: user,
		FromAddress: from, FromName: name, IsEnabled: in.IsEnabled,
	}, nil
}

// ValidateMailPassword checks a NEW password. It never echoes the value.
func ValidateMailPassword(p []byte) error {
	if len(p) == 0 || len(p) > mailPasswordMax {
		return ErrMailPasswordShape
	}
	for _, b := range p {
		if b == 0 {
			return ErrMailPasswordShape
		}
	}
	return nil
}

// NormalizeMailAddress accepts exactly one bare address and returns it trimmed.
func NormalizeMailAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > mailAddressMax || hasControl(s) {
		return "", ErrMailRecipient
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s {
		return "", ErrMailRecipient
	}
	return s, nil
}

// MailDestinationChanged reports whether the password would be sent somewhere it has not been sent
// before: another host, another port, or another account.
//
// WHY THIS DECIDES WHETHER A BLANK PASSWORD MAY MEAN "KEEP" (../vigov-require, email_config.py
// save_settings; docs/spec/07-viec-nen-va-thong-bao.md:98-100): the person editing the screen
// cannot read the password, but if they could point `host` at a machine of their own and press
// `Gửi thử`, that machine would receive it. A secret never follows an address it was not typed for.
// And the practical half: an old password almost never works on a new server, and the failure only
// shows the day a real notice is sent.
func MailDestinationChanged(before, after MailSettings) bool {
	return before.Host != after.Host || before.Port != after.Port || before.Username != after.Username
}

// SameMailSettings compares every stored non-secret field.
func SameMailSettings(a, b MailSettings) bool {
	return a.Host == b.Host && a.Port == b.Port && a.Security == b.Security &&
		a.Username == b.Username && a.FromAddress == b.FromAddress &&
		a.FromName == b.FromName && a.IsEnabled == b.IsEnabled
}

func validMailHost(h string) bool {
	if len(h) > mailHostMax {
		return false
	}
	if ip := net.ParseIP(h); ip != nil {
		return true
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
