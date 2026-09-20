package domain

import "time"

// QuanHeCongDanXa is one citizen's relationship with ONE commune — @entity CitizenCommune,
// table quan_he_cong_dan_xa (ADR 0023, migration 0004).
//
// # TWO FACTS, TWO FIELDS, AND THEY ARE TWO DIFFERENT TYPES ON PURPOSE
//
//	Khai       what the CITIZEN SAID
//	TrangThai  whether a COMMUNE OFFICER HAS CHECKED IT
//
// They are independent. Merging them — here, in a DTO, in a report — is the defect the table
// exists to avoid, and it is silent: the column still holds a legal value, the query still
// runs, the report still produces a number. Under Luật Cư trú 2020 thường trú and tạm trú are
// states REGISTERED WITH THE POLICE; a report saying "1.200 công dân thường trú" is read as
// 1,200 people holding a registration, and it goes upward to leadership. The migration argues
// this at length above CREATE TABLE quan_he_cong_dan_xa.
//
// KhaiCuTru AND TrangThaiXacThuc ARE TWO NAMED TYPES, not two strings, so that assigning one
// where the other belongs does not compile. That is the cheapest guard that survives a
// refactor: a comment can be skimmed past, a type error cannot.
//
// THERE IS NO METHOD HERE THAT COLLAPSES THE PAIR — no DaThuongTru(), no CuTru(). A helper
// like that is the merge, wearing a convenience name, and every caller that used it would be
// counting claims as registrations. Read both fields, always.
//
// `DaXacThuc` IS CONVENIENCE DATA, NEVER LEGAL GROUNDS (ADR 0023 §A1, customer decision of
// 2026-09-17): pre-filling a form, suggesting a commune, filtering a list. An officer can
// confirm THAT THIS ACCOUNT DECLARED THIS; they cannot confirm that the person holding the
// phone is the person named — citizen identity is deliberately weak (rule 4) and stays weak
// after verification. Any procedure with legal consequences runs through its own paperwork.
type QuanHeCongDanXa struct {
	// CongDanID is the platform-wide citizen identity (dinh_danh_cong_dan.id), never a phone
	// number. An identifier that can be reversed into personal data IS personal data (rule 3).
	CongDanID string

	// Khai is what the citizen declared. NOT a legal status — see the block above.
	Khai KhaiCuTru

	// NguonKhai records WHO put that value there: the citizen in the app, or an officer at the
	// counter. Without it the two kinds of row are indistinguishable a year later, and "the
	// citizen claimed this" is not the same evidence as "an officer wrote this down".
	NguonKhai NguonKhai

	// KhaiLuc is when the declaration was last made. A re-declaration moves it.
	KhaiLuc time.Time

	// TrangThai is whether the commune has reviewed the declaration. Independent of Khai.
	TrangThai TrangThaiXacThuc

	// XetDuyetBoi is the officer who decided, and XetDuyetLuc is when. Both empty/nil exactly
	// while TrangThai is ChoXacThuc — the schema enforces that pairing
	// (CONSTRAINT quan_he_da_xet_thi_co_nguoi_xet), so a row that has left the queue always
	// names the person who took it out.
	//
	// XetDuyetBoi IS AN INTERNAL FIELD AND MUST NOT REACH THE CITIZEN (rule 4, forbidden #5).
	// The citizen-facing read does not select it at all — see
	// store/crosstenant/quan_he_cong_dan_xa.go, which carries a different type for that reason.
	XetDuyetBoi string
	XetDuyetLuc *time.Time

	// LyDoTuChoi is PROSE AN OFFICER WROTE FOR THE CITIZEN TO READ — not an internal code, not
	// a staff note. Set exactly when TrangThai is TuChoi (CONSTRAINT
	// quan_he_tu_choi_phai_co_ly_do). It carries no personal data (rule 3, forbidden #3): it
	// describes the DECLARATION, not the person.
	LyDoTuChoi string
}

// KhaiCuTru is WHAT THE CITIZEN SAID. Mirrors CHECK quan_he_khai_cu_tru_hop_le.
type KhaiCuTru string

const (
	KhaiThuongTru KhaiCuTru = "thuong_tru"
	KhaiTamTru    KhaiCuTru = "tam_tru"

	// KhaiChuaKhai means no declaration exists for this (citizen, commune) pair. It is NOT
	// `vang_lai`: the row is created the moment a citizen sends something to a commune
	// (ADR 0005), including a person sitting in Hà Nội reporting to a commune in Đà Nẵng.
	// "Vãng lai" asserts physical presence, which nothing in the data supports — and a status
	// the data cannot support is a status somebody will eventually count (ADR 0023 §2).
	KhaiChuaKhai KhaiCuTru = "chua_khai"
)

// TrangThaiXacThuc is WHETHER AN OFFICER CHECKED IT. Mirrors CHECK
// quan_he_trang_thai_xac_thuc_hop_le.
type TrangThaiXacThuc string

const (
	ChoXacThuc TrangThaiXacThuc = "cho_xac_thuc"
	DaXacThuc  TrangThaiXacThuc = "da_xac_thuc"

	// TuChoi keeps the declaration and its reason. It never resets to ChoXacThuc or to
	// KhaiChuaKhai: a citizen who cannot tell "nobody has looked at this yet" from "this was
	// refused" is being closed out in silence, which is what rule 10, invariant 6 forbids on a
	// petition and what ADR 0023 §A2 decided here.
	TuChoi TrangThaiXacThuc = "tu_choi"
)

// NguonKhai is who recorded the declaration. Mirrors CHECK quan_he_nguon_khai_hop_le.
type NguonKhai string

const (
	NguonKhaiCongDan NguonKhai = "cong_dan"
	NguonKhaiCanBo   NguonKhai = "can_bo"
)
