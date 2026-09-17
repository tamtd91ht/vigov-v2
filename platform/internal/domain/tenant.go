package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Tenant is a commune as the platform knows it: its existence, its display name, and whether
// it is still operating. Business data belonging to the commune lives in the other services
// and is never visible from here (ADR 0003).
type Tenant struct {
	ID           string // ULID, 26 chars, opaque and immutable
	Ten          string // display name AT THIS MOMENT — never used as an identifier
	TinhThanh    string
	DangHoatDong bool
}

// Domain is one host that resolves to a commune. A commune may hold several: after a merger
// the old commune's address must keep working while people learn the new one.
type Domain struct {
	Host     string
	TenantID string
	LaChinh  bool
}

var (
	ErrIDKhongHopLe   = errors.New("tenant: id phải là ULID 26 ký tự")
	ErrTenTrong       = errors.New("tenant: tên không được để trống")
	ErrHostTrong      = errors.New("tenant: host không được để trống")
	ErrHostCoGiaoThuc = errors.New("tenant: host chỉ là tên miền, không kèm giao thức hay đường dẫn")
)

// ULIDLength is fixed by the format. tenant.ID.Valid() checks the same thing on the way in
// from a request; this checks it on the way in from an operator.
const ULIDLength = 26

func (t Tenant) Validate() error {
	if len(t.ID) != ULIDLength {
		return fmt.Errorf("%w: %q dài %d", ErrIDKhongHopLe, t.ID, len(t.ID))
	}
	if strings.TrimSpace(t.Ten) == "" {
		return ErrTenTrong
	}
	return nil
}

// NormaliseHost lower-cases a host and strips the port, so that "TanPhu.ViGov.vn:443" and
// "tanphu.vigov.vn" resolve to the same commune.
//
// WHY THIS IS NOT COSMETIC: Host arrives from the client. Two spellings of one host that do
// not compare equal mean the lookup misses, the edge returns 404, and a whole commune is
// offline for a reason nobody can see in the logs.
func NormaliseHost(host string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return "", ErrHostTrong
	}
	if strings.Contains(h, "://") || strings.Contains(h, "/") {
		return "", fmt.Errorf("%w: %q", ErrHostCoGiaoThuc, host)
	}
	// Strip the port. IPv6 literals ("[::1]:8080") keep their brackets.
	if i := strings.LastIndex(h, ":"); i > 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	return h, nil
}
