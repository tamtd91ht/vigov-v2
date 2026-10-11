package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// POST /api/v1/citizen-reports/{maTraCuu}/staff-recorded-rating and the record on the staff detail (owner
// rule 11/10/2026, bug sheet row 60).
//
//	PROVED HERE   rule 5 invariant 7 (401 no session · 403 wrong key · right key WRONG COMMUNE — 401, the
//	              repository's answer: authz compares the commune before the key (rule 1 inv 8) · 200) with
//	              the use case NOT reached in the first three · the act gets the path code, the body, the
//	              BUSINESS code and the restricted fact · every refusal's status, code and fixed sentence ·
//	              the detail carries the record BESIDE the citizen's `rating`, with `source` staff_recorded ·
//	              a read failure refuses the detail · no citizen response carries it.
//	NOT PROVED    the rules and the transaction (internal/app), the SQL (internal/store, migration 0042).

type staffRatingsFake struct {
	calls      int
	tenant     tenant.ID
	code       string
	req        app.StaffRatingRequest
	actor      audit.Actor
	restricted app.QuyenXemHanChe
	err        error

	byPetition map[string]domain.StaffRecordedRating
	readErr    error
}

var staffRatedAtHTTP = time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)

func (f *staffRatingsFake) Record(ctx context.Context, code string, req app.StaffRatingRequest, actor audit.Actor,
	restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, domain.StaffRecordedRating, error) {
	f.calls++
	f.tenant, f.code, f.req, f.actor, f.restricted = tenant.MustFrom(ctx), code, req, actor, restricted
	if f.err != nil {
		return domain.PhieuPhanAnh{}, domain.StaffRecordedRating{}, f.err
	}
	return domain.PhieuPhanAnh{ID: "pa-001", MaTraCuu: code, TrangThai: domain.DaXuLy},
		domain.StaffRecordedRating{ID: "sr-1", PetitionID: "pa-001", Stars: req.Stars,
			Channel: domain.StaffRatingChannel(req.Channel), Note: req.Note, Source: domain.StaffRecordedSource,
			RecordedBy: actor.ID, RecordedAt: staffRatedAtHTTP}, nil
}

func (f *staffRatingsFake) ForPetition(_ context.Context, petitionID string) (domain.StaffRecordedRating, bool, error) {
	if f.readErr != nil {
		return domain.StaffRecordedRating{}, false, f.readErr
	}
	r, ok := f.byPetition[petitionID]
	return r, ok, nil
}

func staffRatingPath(code string) string { return duong(code) + "/staff-recorded-rating" }

func staffRatingBody() staffRecordedRatingIn {
	return staffRecordedRatingIn{Stars: 4, Channel: "dien-thoai", Note: "Dân gọi điện báo đã sạch."}
}

// --- rule 5 invariant 7 ------------------------------------------------------------------------------------

func TestStaffRatingNoSession401(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.resolve")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), nil, staffRatingBody()),
		http.StatusUnauthorized)
	if m.staffRatings.calls != 0 {
		t.Error("reached the use case without a session")
	}
}

func TestStaffRatingWrongPermission403(t *testing.T) {
	m := dungMayChu(t)
	// feedback.read and feedback.assign are processing keys too — neither is the act's key.
	grantBoth(m, t, "feedback.read", "feedback.assign")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), canBoCuaXa(xaA), staffRatingBody()),
		http.StatusForbidden)
	if m.staffRatings.calls != 0 {
		t.Error("reached the use case with the wrong key")
	}
}

// Right key, granted in BOTH communes, and a session of commune B at commune A's host: refused before
// anything is read. 401 — rule 1 invariant 8 ("mismatch = 401"), the answer every route here gives.
func TestStaffRatingRightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.resolve")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), canBoCuaXa(xaB), staffRatingBody()),
		http.StatusUnauthorized)
	if m.staffRatings.calls != 0 {
		t.Error("commune A reached with commune B's session")
	}
}

func TestStaffRatingRightPermissionRightCommune200(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.resolve")
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), canBoCuaXa(xaA), staffRatingBody())
	doiMa(t, w, http.StatusOK)
	f := m.staffRatings
	if f.calls != 1 || f.tenant != xaA || f.code != maPhieuThuong || f.actor.ID != maCanBo || f.restricted ||
		f.req.Stars != 4 || f.req.Channel != "dien-thoai" || f.req.Note != "Dân gọi điện báo đã sạch." {
		t.Errorf("handed down %+v", f)
	}
	var out phieuPhanAnhRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	sr := out.StaffRecordedRating
	if sr == nil || sr.Stars != 4 || sr.Channel != "dien-thoai" || sr.Source != "staff_recorded" ||
		sr.RecordedBy != maCanBo || !sr.RecordedAt.Equal(staffRatedAtHTTP) {
		t.Errorf("reply staff_recorded_rating = %+v", sr)
	}
	// THE CITIZEN'S `rating` IS NOT WHERE THE STAFF RECORD GOES.
	if out.Rating != nil {
		t.Errorf("the staff record leaked into the citizen's rating: %v", *out.Rating)
	}

	grantBoth(m, t, "feedback.resolve", "feedback.restricted")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), canBoCuaXa(xaA), staffRatingBody()),
		http.StatusOK)
	if !m.staffRatings.restricted {
		t.Error("feedback.restricted held, yet the use case was told it is not")
	}
}

// --- refusals ----------------------------------------------------------------------------------------------

func TestStaffRatingRefusalsAnswerFixedSentences(t *testing.T) {
	wrap := func(e error) error {
		return errors.Join(errors.New("xu_ly_phan_anh: ghi nhận cho xã "+string(xaA)), e)
	}
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
		says   string
	}{
		"unknown / other commune": {wrap(petstore.ErrPhieuKhongTonTai), http.StatusNotFound, "not_found", "Không tìm thấy"},
		"can-bo without the key":  {wrap(app.ErrPhieuHanChe), http.StatusNotFound, "not_found", "Không tìm thấy"},
		"stars":                   {domain.ErrRatingStarsOutOfRange, http.StatusBadRequest, "invalid_request", "1 đến 5"},
		"channel":                 {domain.ErrStaffRatingChannelInvalid, http.StatusBadRequest, "invalid_request", "điện thoại"},
		"note":                    {domain.ErrStaffRatingNoteTooLong, http.StatusBadRequest, "invalid_request", "quá dài"},
		"citizen rated":           {wrap(domain.ErrStaffRatingCitizenRated), http.StatusConflict, "citizen_rated", "đã tự đánh giá"},
		"already recorded":        {wrap(domain.ErrStaffRatingAlreadyRecorded), http.StatusConflict, "staff_rating_exists", "đã có"},
		"system failure":          {errors.New("pg: connection refused on db-07"), http.StatusInternalServerError, "internal", "Đã xảy ra lỗi"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.staffRatings.err = c.err
			grantBoth(m, t, "feedback.resolve")
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffRatingPath(maPhieuThuong), canBoCuaXa(xaA), staffRatingBody())
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code || !strings.Contains(e.Message, c.says) {
				t.Errorf("error = %+v", e)
			}
			for _, leak := range []string{string(xaA), "xu_ly_phan_anh", "phan_anh:", "db-07", "Dân gọi điện"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("internals on the wire: %q in %s", leak, w.Body.String())
				}
			}
		})
	}
}

// --- the detail ----------------------------------------------------------------------------------------------

func TestDetailCarriesStaffRecordedRatingBesideCitizenRating(t *testing.T) {
	m := dungMayChu(t)
	// Nothing recorded: no key at all.
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "staff_recorded_rating") {
		t.Errorf("a petition with no staff record carries the key: %s", w.Body.String())
	}

	// Staff recorded 2 stars, then the citizen rated 5: BOTH are shown, apart — the citizen's in `rating`.
	m.staffRatings.byPetition = map[string]domain.StaffRecordedRating{"pa-001": {ID: "sr-1", PetitionID: "pa-001",
		Stars: 2, Channel: domain.StaffRatingAtCounter, Source: domain.StaffRecordedSource,
		RecordedBy: "CB-00999", RecordedAt: staffRatedAtHTTP}}
	p := m.phieu.theo[xaA][maPhieuThuong]
	p.Rating, p.RatedAt = 5, staffRatedAtHTTP.Add(time.Hour)
	m.phieu.theo[xaA][maPhieuThuong] = p
	w = m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out phieuPhanAnhRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Rating == nil || *out.Rating != 5 {
		t.Errorf("citizen rating = %v, want 5 (authoritative)", out.Rating)
	}
	sr := out.StaffRecordedRating
	if sr == nil || sr.Stars != 2 || sr.Channel != "tai-quay" || sr.Source != "staff_recorded" || sr.RecordedBy != "CB-00999" {
		t.Errorf("staff_recorded_rating = %+v", sr)
	}
}

func TestDetailRefusesWhenStaffRatingCannotBeRead(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "feedback.unmask")
	m.staffRatings.readErr = errors.New("pg: connection refused on db-07")
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if len(m.vet.ghi) != 0 {
		t.Error("a disclosure was recorded for a read that never answered")
	}
}

// Staff surface only — no citizen response type has the field (rule 4, invariant 5).
func TestCitizenResponsesNeverCarryStaffRecordedRating(t *testing.T) {
	p := domain.PhieuPhanAnh{ID: "pa-1", MaTraCuu: "PA-AAAA-0000-0001", TrangThai: domain.DaXuLy,
		GocDemHan: mocGui, VaoSoLuc: mocVaoSo}
	for name, v := range map[string]any{
		"my-citizen-reports":      phieuCuaToiRaNgoai(p, ""),
		"my-citizen-reports list": tomTatPhieuCuaToi(phieuCuaToiRaNgoai(p, "")),
		"public lookup":           accountlessLookupOf(p),
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "staff_recorded") {
			t.Errorf("%s carries the staff record: %s", name, b)
		}
	}
}
