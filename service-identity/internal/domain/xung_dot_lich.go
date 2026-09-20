package domain

import (
	"fmt"
	"strings"
)

// SoNgayXungDotNeuTen is how many conflicting dates an error message names before it stops
// counting. Every date is kept on the error; only the SENTENCE is bounded, because a message
// listing forty dates is a message nobody reads to the end.
const SoNgayXungDotNeuTen = 5

// LoiNgayVuaNghiVuaLamBu says one or more dates appear BOTH in `ngay_nghi_le` and in
// `ngay_lam_bu` — the commune is recorded as closed and as working on the same day.
//
// ---------------------------------------------------------------------------------------------
// THIS IS A CONFIGURATION ERROR, NOT A PUZZLE TO RESOLVE WITH A PRECEDENCE RULE, and the refusal
// is the whole reason this type exists. Migration 0006:254 hands the obligation to the reader
// because no CHECK can span two tables:
//
//	"a date present in BOTH tables is a configuration error, not a puzzle to resolve with a rule.
//	 The function must refuse rather than pick a winner — a silent precedence rule would make one
//	 of two visible configuration rows do nothing, and nobody would ever see which."
//
// Either winner is defensible in the abstract and both are wrong here. "Holiday wins" makes the
// swap day the Prime Minister announced do nothing; "swap day wins" makes a holiday the commune
// entered do nothing. In both cases the configuration screen goes on showing two rows while the
// deadline uses one — and the person looking at the screen has no way to tell which.
//
// BOTH READ ROUTES REFUSE, NOT ONE. A refusal on only one of the two would leave the other screen
// looking healthy, which is the same failure one level up: the commune would fix nothing because
// nothing appeared wrong. Refusing on both is also what "do not pick a winner" means in practice —
// neither table is the one that gets to keep answering.
// ---------------------------------------------------------------------------------------------
type LoiNgayVuaNghiVuaLamBu struct {
	// Ngay holds every conflicting date, `YYYY-MM-DD`, in ascending order. ALL of them, so whoever
	// fixes the configuration is not sent back for a second round after correcting the first one.
	Ngay []string
}

func (e *LoiNgayVuaNghiVuaLamBu) Error() string {
	ten := e.Ngay
	duoi := ""
	if len(ten) > SoNgayXungDotNeuTen {
		duoi = fmt.Sprintf(" và %d ngày khác", len(ten)-SoNgayXungDotNeuTen)
		ten = ten[:SoNgayXungDotNeuTen]
	}
	// A DATE IS NOT PERSONAL DATA (rule 3): it names when an authority is open, not a person. That
	// is what makes it safe for this sentence to reach a screen — and the sentence is only useful
	// if it names the day somebody has to go and correct.
	return fmt.Sprintf("lịch làm việc: ngày %s vừa là ngày nghỉ lễ vừa là ngày làm bù%s",
		strings.Join(ten, ", "), duoi)
}
