package domain

// The WRITE side of the Phân quyền matrix: saving ONE role's column (docs/ui-ux/14-cau-hinh.md §4,
// §12.5 — "Lưu phân quyền theo từng cột vai trò… không lưu toàn ma trận").
//
// WHAT LIVES HERE IS THE PART OF THE DECISION THAT NEEDS NO DATABASE: the shape of a submitted key
// set, the difference between two sets, and the arithmetic of open question #13 on the state AFTER
// the save. The reads that feed them — under row locks, inside one transaction — are the use case's
// job (app/phan_quyen_vai_tro.go). Pure functions here are what lets the refusals be tested case by
// case without a fake driver standing between the test and the rule.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	// ErrKhoaQuyenSaiDangThuc is a key that cannot be a permission key at all — empty, no dot, too
	// long. Checked before any read so a malformed body never holds a row lock. Whether a well-formed
	// key EXISTS is a different question, answered against `quyen` inside the transaction.
	ErrKhoaQuyenSaiDangThuc = errors.New("phan_quyen: khoá quyền không đúng dạng \"<nhóm>.<việc>\"")

	// ErrQuaNhieuKhoaQuyen bounds the body. The catalogue is a few dozen keys; a list far past it is
	// not a column of the matrix but memory a client chose.
	ErrQuaNhieuKhoaQuyen = errors.New("phan_quyen: gửi quá nhiều khoá quyền")
)

const (
	// tranDoDaiKhoaQuyen — the longest key seeded today is under 30 characters.
	tranDoDaiKhoaQuyen = 100
	// TranSoKhoaQuyenGui — the same order as store.TranDanhMucQuyen (300). A set longer than the
	// whole catalogue cannot be valid, so refusing it early refuses nothing legitimate.
	TranSoKhoaQuyenGui = 300
)

// NguoiGiuQuyen is one person who can exercise a key today, and the role through which they do.
//
// THE ROLE IS THE POINT. Saving a column changes what the holders OF THAT ROLE can do and nothing
// else, so #13's question — "who would still hold the key afterwards" — is answered by splitting
// the current holders on this field.
type NguoiGiuQuyen struct {
	CanBoID  string // nguoi_dung.id — internal, decides; never written to a trail
	VaiTroID string // vai_tro.id the key reaches them through
}

// ChuanHoaTapQuyen validates a submitted key set and returns it DE-DUPLICATED AND SORTED.
//
// DUPLICATES COLLAPSE, THEY ARE NOT REFUSED. The body is a SET by contract (PUT carries the whole
// column), and `["task.read","task.read"]` names the same set as `["task.read"]`. Refusing it would
// be a rule that fires on a harmless client quirk, which is how a rule stops being believed.
//
// SORTED, so the stored diff, the audit delta and the response are stable between two identical
// requests — an unordered list makes every comparison compare sets by accident.
//
// AN EMPTY SET IS LEGITIMATE: a role may hold nothing. Whether emptying THIS role is safe is #13's
// question, asked later on the locked state.
func ChuanHoaTapQuyen(ds []string) ([]string, error) {
	if len(ds) > TranSoKhoaQuyenGui {
		return nil, fmt.Errorf("%w (tối đa %d)", ErrQuaNhieuKhoaQuyen, TranSoKhoaQuyenGui)
	}
	co := make(map[string]bool, len(ds))
	ra := make([]string, 0, len(ds))
	for _, k := range ds {
		// NOT TRIMMED: a key with spaces is not the key the route declared, and silently repairing
		// it would store a string different from the one the administrator's client sent.
		if k == "" || utf8.RuneCountInString(k) > tranDoDaiKhoaQuyen || strings.Index(k, ".") < 1 {
			return nil, ErrKhoaQuyenSaiDangThuc
		}
		if !co[k] {
			co[k] = true
			ra = append(ra, k)
		}
	}
	sort.Strings(ra)
	return ra, nil
}

// HieuTapQuyen returns the keys `sau` adds to `truoc` and the keys it removes, both sorted.
func HieuTapQuyen(truoc, sau []string) (them, bo []string) {
	cu := make(map[string]bool, len(truoc))
	for _, k := range truoc {
		cu[k] = true
	}
	moi := make(map[string]bool, len(sau))
	for _, k := range sau {
		moi[k] = true
		if !cu[k] {
			them = append(them, k)
		}
	}
	for _, k := range truoc {
		if !moi[k] {
			bo = append(bo, k)
		}
	}
	sort.Strings(them)
	sort.Strings(bo)
	return them, bo
}

// ConNguoiGiuSauKhiLuu reports whether, after role `vaiTroID` is saved with the set `sau`, the
// commune is PROVEN to still have at least one active holder of `khoa`.
//
// `dangGiu` is every current holder of `khoa`, read UNDER LOCK in the same transaction.
//
// PROVEN, NOT ASSUMED — two ways to be sure and nothing else:
//
//	someone outside   a holder whose key comes through ANOTHER role is untouched by this save.
//	kept, non-empty   if `sau` still carries the key, nobody loses it: current holders through this
//	                  role keep it, and anybody else keeps it too. The post-state is a superset of
//	                  the current one, so a non-empty current set is enough.
//
// EVERYTHING ELSE ANSWERS false, including a commune whose holder set is ALREADY empty. Such a
// commune cannot be repaired through this route anyway — granting a key needs the actor to hold it
// (#14) — so refusing every save there refuses nothing this route could have fixed, and it keeps the
// rule to one line with no branch that could be half-removed.
func ConNguoiGiuSauKhiLuu(dangGiu []NguoiGiuQuyen, vaiTroID string, sau []string, khoa string) bool {
	for _, n := range dangGiu {
		if n.VaiTroID != vaiTroID {
			return true
		}
	}
	return len(dangGiu) > 0 && coKhoa(sau, khoa)
}

func coKhoa(ds []string, k string) bool {
	for _, v := range ds {
		if v == k {
			return true
		}
	}
	return false
}
