package store

import (
	"database/sql/driver"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// `publication_status` is read BY NAME from the store's own SELECT list, in the position the
// superseded `hien_cong_khai` held — and that column is no longer asked for at all.
//
// NOT PROVED HERE: SetPublicationStatus and the classify UPDATE. This fake has no transactions; both
// run over the transaction-capable driver in internal/app/petition_publication_test.go.
func TestLookupReadsPublicationStatus(t *testing.T) {
	for _, st := range []domain.PublicationStatus{domain.PublicationPending, domain.PublicationPublic,
		domain.PublicationHidden} {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
			"publication_status": string(st),
			"so_lan_mo_lai":      int64(4),
		})}}
		s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

		p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
		if err != nil {
			t.Fatalf("đọc phiếu: %v", err)
		}
		if p.PublicationStatus != st {
			t.Errorf("PublicationStatus = %q, muốn %q", p.PublicationStatus, st)
		}
		// The neighbour is unchanged — a destination shifted by one would read it here.
		if p.SoLanMoLai != 4 {
			t.Errorf("so_lan_mo_lai = %d, muốn 4 — cột lân cận bị lệch", p.SoLanMoLai)
		}
		if strings.Contains(k.lenh[0].sql, "hien_cong_khai") {
			t.Errorf("câu đọc vẫn hỏi cột đã bị thay `hien_cong_khai`: %q", k.lenh[0].sql)
		}
	}
}
