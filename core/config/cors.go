package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNguonCORSHong is returned by Load when CITIZEN_CORS_ALLOWED_ORIGINS holds an entry that is
// not an https origin or an https wildcard of the one accepted shape.
var ErrNguonCORSHong = errors.New("config: CITIZEN_CORS_ALLOWED_ORIGINS không hợp lệ")

// NguonCORS is the list of browser origins allowed to read CITIZEN-edge responses (the Zalo Mini
// App webview). Read from CITIZEN_CORS_ALLOWED_ORIGINS; httpx.CORSCongDan is the one consumer.
//
// THE MATCHING RULE LIVES HERE, BESIDE THE PARSER, AND NOWHERE ELSE. The parser decides what an
// entry may look like and ChoPhep decides what it matches; split across two packages they are two
// owners of one rule, and the day they disagree an entry Load accepted matches something Load
// would have refused (rule 9, invariant 2).
type NguonCORS []mauNguon

// mauNguon is one parsed entry: either an exact origin (`https://h5.zdn.vn`, optional port) or a
// wildcard (`https://*.zdn.vn`), stored lowercased.
type mauNguon struct {
	chinhXac string // "https://host[:port]" — exact entry; empty for a wildcard
	duoi     string // "zdn.vn" — wildcard suffix; empty for an exact entry
}

// Rong reports whether no origin is configured. Empty means httpx.CORSCongDan adds nothing at all.
func (n NguonCORS) Rong() bool { return len(n) == 0 }

// ChoPhep reports whether a request's Origin header matches one configured entry.
//
// EXACT: the whole origin, case-insensitively (scheme and host are case-insensitive; a browser
// sends them lowercase anyway). A port or a trailing slash makes it a different origin.
//
// WILDCARD `https://*.<duoi>`: scheme https, NO port, and a host that is at least one well-formed
// label followed by "." + <duoi>. So `https://evil.zdn.vn.attacker.com` (the suffix is not at the
// END), `https://zdn.vn` (no label before it), `http://h5.zdn.vn` (not https) and
// `https://x.zdn.vn:8443` (a port) all fail. A suffix test on the raw string would accept
// `https://evilzdn.vn` for `*.zdn.vn` — which is why the dot is part of the comparison.
func (n NguonCORS) ChoPhep(origin string) bool {
	o := strings.ToLower(origin)
	if !strings.HasPrefix(o, "https://") {
		return false
	}
	host := strings.TrimPrefix(o, "https://")
	for _, m := range n {
		if m.chinhXac != "" {
			if o == m.chinhXac {
				return true
			}
			continue
		}
		dau, ok := strings.CutSuffix(host, "."+m.duoi)
		if ok && tenMienHopLe(dau) {
			return true
		}
	}
	return false
}

// PhanTichNguonCORS parses CITIZEN_CORS_ALLOWED_ORIGINS. Blank returns an empty list — CORS off.
//
// REFUSED, BY NAME AND POSITION, rather than skipped — the TRUSTED_PROXY_CIDRS discipline:
//
//	`*` alone          allow-all is never a configuration this edge accepts; it would let ANY web
//	                   page read a citizen's petitions with a token it phished
//	not https://       the Mini App runs on https; an http origin is readable by anyone on the path
//	a path, a query,   an Origin never carries one, so the entry could only ever match nothing —
//	a trailing slash   which reads as "CORS is broken" with no hint that the entry is the cause
//	`*` anywhere but   `https://h5*.zdn.vn`, `https://*` and the like are shapes nobody can reason
//	a leading `*.`     about; one wildcard shape is the whole vocabulary
//	`*.<one label>`    `https://*.vn` would admit every site in a country
//
// The index is the 1-based position in the RAW comma-split value, empty entries counted, so it
// points at the same place an operator sees in the ConfigMap.
func PhanTichNguonCORS(raw string) (NguonCORS, error) {
	var ra NguonCORS
	for i, phan := range strings.Split(raw, ",") {
		phan = strings.TrimSpace(phan)
		if phan == "" {
			continue
		}
		m, lyDo := phanTichMotNguon(strings.ToLower(phan))
		if lyDo != "" {
			return nil, fmt.Errorf("%w: mục thứ %d (%q): %s", ErrNguonCORSHong, i+1, phan, lyDo)
		}
		ra = append(ra, m)
	}
	return ra, nil
}

func phanTichMotNguon(p string) (mauNguon, string) {
	if p == "*" {
		return mauNguon{}, "\"*\" cho phép mọi trang web — không bao giờ được chấp nhận"
	}
	host, ok := strings.CutPrefix(p, "https://")
	if !ok {
		return mauNguon{}, "chỉ chấp nhận origin https://"
	}
	if strings.ContainsAny(host, "/?#@") {
		return mauNguon{}, "origin không có đường dẫn, dấu / cuối, query hay thông tin đăng nhập"
	}
	if duoi, laMau := strings.CutPrefix(host, "*."); laMau {
		if strings.Contains(duoi, "*") || strings.Contains(duoi, ":") {
			return mauNguon{}, "mẫu đại diện chỉ có dạng https://*.<tên-miền>, không cổng, không * thứ hai"
		}
		if !tenMienHopLe(duoi) || !strings.Contains(duoi, ".") {
			return mauNguon{}, "phần sau *. phải là một tên miền có ít nhất hai nhãn (zdn.vn, không phải vn)"
		}
		return mauNguon{duoi: duoi}, ""
	}
	if strings.Contains(host, "*") {
		return mauNguon{}, "* chỉ được đứng đầu, dạng https://*.<tên-miền>"
	}
	ten, cong, coCong := strings.Cut(host, ":")
	if !tenMienHopLe(ten) {
		return mauNguon{}, "tên máy không hợp lệ"
	}
	if coCong {
		n, err := strconv.Atoi(cong)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != cong {
			return mauNguon{}, "cổng không hợp lệ"
		}
	}
	return mauNguon{chinhXac: "https://" + host}, ""
}

// tenMienHopLe accepts one or more dot-separated labels of [a-z0-9-], none empty, none starting or
// ending with '-'. Input is already lowercased.
func tenMienHopLe(s string) bool {
	if s == "" {
		return false
	}
	for _, nhan := range strings.Split(s, ".") {
		if nhan == "" || nhan[0] == '-' || nhan[len(nhan)-1] == '-' {
			return false
		}
		for _, c := range nhan {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}
