package store

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// boiTuChoiHoSoDaKy — migration 0012's trigger refusals become the 409 sentinels, and nothing else
// does. A P0001 read as "anything" would turn an outage into a 409; a trigger refusal read as nothing
// would turn a rule doing its job into a 500.

type loiSQLStateThu struct{ ma, msg string }

func (e *loiSQLStateThu) Error() string    { return e.msg }
func (e *loiSQLStateThu) SQLState() string { return e.ma }

func TestBoiTuChoiHoSoDaKy(t *testing.T) {
	for _, ca := range []struct {
		ten  string
		loi  error
		muon error
	}{
		{"khoá biên bản đã ký", &loiSQLStateThu{"P0001",
			"archival record bien_ban_hop: signed minutes cannot be edited or removed"}, domain.ErrBienBanDaKy},
		{"kết luận của biên bản đã ký", &loiSQLStateThu{"P0001",
			"archival record ket_luan_hop: signed minutes take no new conclusion"}, domain.ErrBienBanDaKy},
		{"thông báo chỉ một lần", &loiSQLStateThu{"P0001",
			"archival record bien_ban_hop: the conclusion notice reference of signed minutes is recorded once"},
			domain.ErrDaCoThongBao},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			got := boiTuChoiHoSoDaKy(ca.loi)
			if !errors.Is(got, ca.muon) {
				t.Errorf("lỗi = %v, muốn %v", got, ca.muon)
			}
			var e sqlStateLoi
			if !errors.As(got, &e) {
				t.Error("lỗi gốc của driver rơi khỏi chuỗi — nhật ký vận hành mất nguyên nhân")
			}
		})
	}

	for _, loi := range []error{
		&loiSQLStateThu{"23514", "check_violation"},
		errors.New("kết nối hỏng"),
	} {
		if got := boiTuChoiHoSoDaKy(loi); errors.Is(got, domain.ErrBienBanDaKy) || errors.Is(got, domain.ErrDaCoThongBao) {
			t.Errorf("%v bị đọc thành từ chối của trigger", loi)
		}
	}
}
