package domain

// The percentage columns of the budget board: which two columns each one divides, how a create
// request names them, and the per-line figure the server computes (docs/ui-ux/07-thu-chi-ngan-sach.md
// §9 rule 3; user decision 30/09/2026 "máy chủ tự tính").
//
// WHAT IS STORED AND WHAT IS NOT. The OPERANDS are stored (migration 0011: `numerator_column_id`,
// `denominator_column_id`); the PERCENTAGE never is. It is computed on every read from the line's own
// figures in those two columns, so it cannot drift from them — a stored ratio would be a second home
// for a derivable number, and the stale copy is the one that reaches the report.
//
// `cong_thuc` IS DISPLAY TEXT ONLY. It is the commune's wording (or one this file writes from the
// operand names) and nothing here parses it. Before migration 0011 it was the only statement of the
// formula; a legacy column whose formula 0011 could not resolve has NO operands, and that column is
// shown as "cannot be computed" with a sentence — never guessed from the text, which is exactly the
// guess 0011 refused to make.

import (
	"errors"
	"fmt"
	"math/big"
)

var (
	// --- write-side refusals (400) ---

	// ErrPercentOperandsMissing — a NEW `phan_tram` column without both operands. New data never lands
	// in the legacy unresolved state: that state exists only for columns created before 0011.
	ErrPercentOperandsMissing = errors.New(
		"ngan_sach: cột `phan_tram` phải chỉ rõ cả cột tử số và cột mẫu số (`numerator_index`, `denominator_index`)")

	// ErrOperandsOnNumberColumn — a `so` column naming operands. Refused rather than ignored: a client
	// that watched them vanish would believe it had declared a ratio.
	ErrOperandsOnNumberColumn = errors.New(
		"ngan_sach: cột `so` không mang `numerator_index` / `denominator_index` — chỉ cột `phan_tram` mới có tử số, mẫu số")

	// ErrOperandIndexOutOfRange — an index that names no column of the request.
	ErrOperandIndexOutOfRange = errors.New(
		"ngan_sach: `numerator_index` / `denominator_index` phải là vị trí (đếm từ 0) của một cột trong chính danh sách `columns` gửi lên")

	// ErrOperandsSameColumn — numerator and denominator are one column. A column divided by itself is
	// 100% on every line, which is not a ratio anybody meant (0011's backfill refuses it too).
	ErrOperandsSameColumn = errors.New(
		"ngan_sach: cột tử số và cột mẫu số phải là hai cột khác nhau")

	// ErrOperandNotNumberColumn — an operand that is a percentage column (itself included). A % of a %
	// has no stored figure under it; §9 rule 3 computes percentages from figures only.
	ErrOperandNotNumberColumn = errors.New(
		"ngan_sach: cột tử số và cột mẫu số phải là cột `so` — không lấy cột phần trăm làm tử số hay mẫu số")

	// ErrOperandNotInSheet — an operand id that is not a live column of the same sheet. Unreachable
	// through the create route (operands are named by index into the same request), and checked all the
	// same: the database cannot (0011 explains why there is no foreign key), so this is the only place.
	ErrOperandNotInSheet = errors.New(
		"ngan_sach: cột tử số / mẫu số phải là cột còn hiệu lực của cùng một bảng")

	// --- read-side reasons (the cell is null, the sentence says why) ---

	// ErrPercentOperandsUnresolved — a legacy column 0011 could not resolve unambiguously.
	ErrPercentOperandsUnresolved = errors.New(
		"ngan_sach: cột phần trăm này tạo trước khi hệ thống lưu rõ cột tử số và mẫu số, và công thức cũ không đọc được chắc chắn — không tính được; không đoán từ công thức")

	// ErrPercentOperandsNotIdentified — a column of a sheet loaded from Excel whose operands the headings
	// did not identify unambiguously (ADR 0081 #6). The figure in the file is not used (§9 rule 3).
	ErrPercentOperandsNotIdentified = errors.New(
		"ngan_sach: cột phần trăm này nạp từ Excel mà tiêu đề không cho xác định chắc chắn cột tử số và cột mẫu số — không tính được; hệ thống không lấy số % trong tệp và không đoán")

	// ErrPercentNumeratorEmpty / ErrPercentDenominatorEmpty — an operand cell of THIS line is empty.
	// Empty is not 0 (§9 rule 4): a ratio over an empty cell would print 0% for a figure nobody entered.
	ErrPercentNumeratorEmpty = errors.New(
		"ngan_sach: ô tử số của dòng này đang trống — không tính tỷ lệ; ô trống không phải số 0")
	ErrPercentDenominatorEmpty = errors.New(
		"ngan_sach: ô mẫu số của dòng này đang trống — không tính tỷ lệ; ô trống không phải số 0")

	// ErrPercentOutOfRange — the ratio in basis points leaves ±GiaTriToiDa, the largest integer the
	// browser reads exactly. Unavailable, never clamped: a clamped ratio is a figure nobody computed.
	ErrPercentOutOfRange = errors.New(
		"ngan_sach: tỷ lệ vượt mức một con số hiển thị chính xác được — không tính được; kiểm tra ô mẫu số của dòng này")
)

// OperandIndexes is how a CREATE request names a percentage column's operands: by 0-based position
// in the SAME request's column list.
//
// AN INDEX AND NOT `order` / `thu_tu`: `thu_tu` is not unique (migration 0006:293-294 is an index, not
// a unique key), and that ambiguity is exactly what left legacy columns unresolved in 0011. AN INDEX
// AND NOT AN ID: the server issues column ids inside the create, so a client has no id to name — and
// a client that could name one could name a column of another sheet or another commune.
type OperandIndexes struct {
	Numerator   *int
	Denominator *int
}

// AssignOperandsByIndex turns each column's OperandIndexes into operand column IDS. The columns must
// already carry their ids; refs is parallel to cols (refs[i] belongs to cols[i]).
//
// ONLY THE SHAPE OF THE REFERENCE IS CHECKED HERE — both-or-neither, on a `phan_tram` column only,
// inside the list. WHAT the reference points at (a live `so` column of the same sheet, two distinct
// ones) is KiemTraBoCot's, which checks ids and so holds for any writer, not only this one.
func AssignOperandsByIndex(cols []CotNganSach, refs []OperandIndexes) error {
	if len(refs) != len(cols) {
		// A caller bug, not a client mistake: the two slices are built together.
		return fmt.Errorf("ngan_sach: %d khai báo tử số/mẫu số cho %d cột", len(refs), len(cols))
	}
	for i, r := range refs {
		if r.Numerator == nil && r.Denominator == nil {
			continue // KiemTraBoCot refuses a `phan_tram` column left without operands
		}
		if cols[i].Kieu != CotPhanTram {
			return fmt.Errorf("%w (cột %d)", ErrOperandsOnNumberColumn, i)
		}
		if r.Numerator == nil || r.Denominator == nil {
			return fmt.Errorf("%w (cột %d)", ErrPercentOperandsMissing, i)
		}
		n, d := *r.Numerator, *r.Denominator
		if n < 0 || n >= len(cols) || d < 0 || d >= len(cols) {
			return fmt.Errorf("%w (cột %d)", ErrOperandIndexOutOfRange, i)
		}
		cols[i].NumeratorColumnID = cols[n].ID
		cols[i].DenominatorColumnID = cols[d].ID
	}
	return nil
}

// checkOperandSet is the id-level check of every column's operands against the set they belong to.
// Called by KiemTraBoCot after the per-column checks.
func checkOperandSet(cols []CotNganSach) error {
	byID := make(map[string]CotNganSach, len(cols))
	for _, c := range cols {
		byID[c.ID] = c
	}
	for i, c := range cols {
		hasNum, hasDen := c.NumeratorColumnID != "", c.DenominatorColumnID != ""
		if c.Kieu != CotPhanTram {
			if hasNum || hasDen {
				return fmt.Errorf("%w (cột %d)", ErrOperandsOnNumberColumn, i)
			}
			continue
		}
		if !hasNum || !hasDen {
			return fmt.Errorf("%w (cột %d)", ErrPercentOperandsMissing, i)
		}
		if c.NumeratorColumnID == c.DenominatorColumnID {
			return fmt.Errorf("%w (cột %d)", ErrOperandsSameColumn, i)
		}
		for _, id := range []string{c.NumeratorColumnID, c.DenominatorColumnID} {
			op, found := byID[id]
			if !found || op.BangID != c.BangID {
				return fmt.Errorf("%w (cột %d)", ErrOperandNotInSheet, i)
			}
			if op.Kieu != CotSo {
				// The column itself lands here too: it is a `phan_tram` column.
				return fmt.Errorf("%w (cột %d)", ErrOperandNotNumberColumn, i)
			}
		}
	}
	return nil
}

// PercentFormula is the display text written into `cong_thuc` when the client sends none: "<tên tử
// số> / <tên mẫu số> × 100".
//
// WITHIN CongThucToiDa, ALWAYS. Two column names may each be TenCotToiDa (200) runes, and 0006's CHECK
// keeps `cong_thuc` NOT NULL on a `phan_tram` column, so the text must fit rather than be refused: the
// request is valid, only its generated caption is long. The shorter name is kept whole where it can
// be and the longer one cut with "…". It is a CAPTION — the computation reads the operand ids, never
// this string — so a cut name costs readability and nothing else.
func PercentFormula(numeratorName, denominatorName string) string {
	const sep, tail = " / ", " × 100"
	room := CongThucToiDa - len([]rune(sep)) - len([]rune(tail))
	a, b := []rune(numeratorName), []rune(denominatorName)
	if len(a)+len(b) > room {
		roomA, roomB := room/2, room-room/2
		switch {
		case len(a) <= roomA:
			roomB = room - len(a)
		case len(b) <= roomB:
			roomA = room - len(b)
		}
		a, b = truncateRunes(a, roomA), truncateRunes(b, roomB)
	}
	return string(a) + sep + string(b) + tail
}

// truncateRunes cuts s to at most n runes, the last one being "…" when anything was cut.
func truncateRunes(s []rune, n int) []rune {
	if len(s) <= n {
		return s
	}
	if n <= 0 {
		return nil
	}
	return append(append([]rune{}, s[:n-1]...), '…')
}

// PercentCell is the per-line percentage of one `phan_tram` column: the line's figure in the numerator
// column over its figure in the denominator column, in basis points (PhanVan).
//
// THE OPERAND FIGURES ARE THE ONES THE SCREEN SHOWS — GiaTri, so a parent's cell is computed from its
// children's rolled-up sums and an `entries` leaf from its batch sums. The ratio of a parent is
// therefore sum-over-sum, never a sum of the children's ratios (which would be meaningless).
//
// EVERY WAY TO HAVE NO NUMBER RETURNS A SENTENCE (the rule at the top of thu_chi_ngan_sach.go):
//
//	legacy column 0011 could not resolve         ErrPercentOperandsUnresolved
//	operand no longer a live `so` column here    ErrOperandNotInSheet / ErrOperandNotNumberColumn
//	operand figure cannot be computed            that figure's own sentence, unchanged
//	numerator or denominator cell empty          ErrPercentNumeratorEmpty / ErrPercentDenominatorEmpty
//	denominator zero                             ErrMauSoBangKhong (§9 rule 3: blank, not ∞, not 100%)
//	ratio past what the browser reads exactly    ErrPercentOutOfRange
func (b BangDayDu) PercentCell(lineID, columnID string) TyLe {
	var col CotNganSach
	found := false
	for _, c := range b.Cot {
		if c.ID == columnID {
			col, found = c, true
			break
		}
	}
	if !found || col.Kieu != CotPhanTram {
		return tyLeKhong(ErrKhongThayCot)
	}
	if col.NumeratorColumnID == "" || col.DenominatorColumnID == "" {
		// Two ways to arrive here, two sentences: a sheet loaded from Excel whose headings did not
		// identify the operands (budget_import.go), or a legacy column 0011 could not resolve.
		if b.Bang.NguonTep != "" {
			return tyLeKhong(ErrPercentOperandsNotIdentified)
		}
		return tyLeKhong(ErrPercentOperandsUnresolved)
	}
	for _, id := range []string{col.NumeratorColumnID, col.DenominatorColumnID} {
		if err := b.operandColumnError(id); err != nil {
			return tyLeKhong(err)
		}
	}

	num := b.GiaTri(lineID, col.NumeratorColumnID)
	if num.LyDo != "" {
		return TyLe{LyDo: num.LyDo}
	}
	den := b.GiaTri(lineID, col.DenominatorColumnID)
	if den.LyDo != "" {
		return TyLe{LyDo: den.LyDo}
	}
	if !num.Co {
		return tyLeKhong(ErrPercentNumeratorEmpty)
	}
	if !den.Co {
		return tyLeKhong(ErrPercentDenominatorEmpty)
	}
	bp, err := PercentBasisPoints(num.Gia, den.Gia)
	if err != nil {
		return tyLeKhong(err)
	}
	return TyLe{Gia: bp, Co: true}
}

// operandColumnError reports whether an operand id is a live `so` column of this sheet, on READ.
// A soft-deleted operand is not in b.Cot (the store never returns it), so it lands here.
func (b BangDayDu) operandColumnError(id string) error {
	for _, c := range b.Cot {
		if c.ID != id {
			continue
		}
		if c.Kieu != CotSo {
			return ErrOperandNotNumberColumn
		}
		return nil
	}
	return ErrOperandNotInSheet
}

// PercentBasisPoints is num / den × 10 000, EXACT, rounded half away from zero — THE SAME ROUNDING
// TyLeDong uses for the indicators, so a ratio in a cell and the same ratio on the indicator card never
// differ in the last digit.
//
// EXACT ARITHMETIC AND NOT TyLeDong ITSELF: operands reach ±GiaTriToiDa (≈ 9 × 10^15), so num × 10^4
// reaches ≈ 9 × 10^19, past int64. TyLeDong scales both sides down by 1000 to stay inside int64 and
// then answers 0 whenever the scaled denominator reaches 0 — a plausible "0%" on a line whose ratio is
// in fact enormous. math/big has no such corner.
//
// A RESULT PAST ±GiaTriToiDa IS ErrPercentOutOfRange, never clamped (the ceiling every figure of this
// board respects, GiaTriToiDa's comment). A zero denominator is ErrMauSoBangKhong.
func PercentBasisPoints(num, den Dong) (PhanVan, error) {
	if den == 0 {
		return 0, ErrMauSoBangKhong
	}
	scaled := new(big.Int).Mul(big.NewInt(int64(num)), big.NewInt(10_000))
	divisor := big.NewInt(int64(den))

	// |q| = (|scaled| + floor(|divisor| / 2)) / |divisor|, truncated; the sign is the quotient's. That
	// is round half away from zero — exactly what TyLeDong computes on the values it does not scale.
	absDivisor := new(big.Int).Abs(divisor)
	q := new(big.Int).Abs(scaled)
	q.Add(q, new(big.Int).Rsh(absDivisor, 1))
	q.Quo(q, absDivisor)
	if (num < 0) != (den < 0) {
		q.Neg(q)
	}
	if !q.IsInt64() || vuotGiaTriToiDa(Dong(q.Int64())) {
		return 0, ErrPercentOutOfRange
	}
	return PhanVan(q.Int64()), nil
}
