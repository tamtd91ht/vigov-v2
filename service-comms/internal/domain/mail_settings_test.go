package domain

import (
	"errors"
	"testing"
)

func validInput() MailSettingsInput {
	return MailSettingsInput{Host: " SMTP.Example.Test ", Port: 587, Security: MailSecurityStartTLS,
		Username: "ubnd@example.test", FromAddress: "ubnd@example.test", FromName: "UBND xã"}
}

func TestNormalizeMailSettingsLowercasesHost(t *testing.T) {
	m, err := NormalizeMailSettings(validInput())
	if err != nil || m.Host != "smtp.example.test" {
		t.Fatalf("m = %+v err = %v", m, err)
	}
}

func TestNormalizeMailSettingsRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*MailSettingsInput)
		want error
	}{
		"plaintext mode":  {func(in *MailSettingsInput) { in.Security = "none" }, ErrMailSecurityUnknown},
		"internal port":   {func(in *MailSettingsInput) { in.Port = 5432 }, ErrMailPortNotAllowed},
		"host with port":  {func(in *MailSettingsInput) { in.Host = "smtp.example.test:587" }, ErrMailHostShape},
		"host with url":   {func(in *MailSettingsInput) { in.Host = "smtp://x.test" }, ErrMailHostShape},
		"empty host":      {func(in *MailSettingsInput) { in.Host = " " }, ErrMailHostEmpty},
		"empty username":  {func(in *MailSettingsInput) { in.Username = "" }, ErrMailUsernameEmpty},
		"named from":      {func(in *MailSettingsInput) { in.FromAddress = "UBND <ubnd@example.test>" }, ErrMailFromAddress},
		"header in name":  {func(in *MailSettingsInput) { in.FromName = "x\r\nBcc: y@example.test" }, ErrMailFromName},
		"control in user": {func(in *MailSettingsInput) { in.Username = "a\nb" }, ErrMailUsernameShape},
	} {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			tc.edit(&in)
			if _, err := NormalizeMailSettings(in); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateMailPassword(t *testing.T) {
	if ValidateMailPassword([]byte("ok-fixture")) != nil {
		t.Error("a normal password was refused")
	}
	for _, p := range [][]byte{nil, []byte("a\x00b"), make([]byte, 257)} {
		if !errors.Is(ValidateMailPassword(p), ErrMailPasswordShape) {
			t.Errorf("accepted %d-byte password", len(p))
		}
	}
}

func TestMailDestinationChanged(t *testing.T) {
	a, _ := NormalizeMailSettings(validInput())
	b := a
	b.FromName, b.Security, b.IsEnabled = "khác", MailSecurityTLS, true
	if MailDestinationChanged(a, b) {
		t.Error("name/security/enabled counted as a new destination")
	}
	for _, f := range []func(*MailSettings){
		func(m *MailSettings) { m.Host = "other.test" },
		func(m *MailSettings) { m.Port = 465 },
		func(m *MailSettings) { m.Username = "x@example.test" },
	} {
		c := a
		f(&c)
		if !MailDestinationChanged(a, c) {
			t.Errorf("not a destination change: %+v", c)
		}
	}
}
