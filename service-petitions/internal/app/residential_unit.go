package app

// The residential unit (thôn / tổ dân phố — the glossary's `ResidentialUnit`) on a petition, ADR 0088 §1.
//
// `thon_to_dan_pho` belongs to identity (ADR 0024) and this service may not read its database (rule 2,
// forbidden #2), so an id arriving in a request body is CLIENT-SUPPLIED until identity says otherwise —
// possibly another commune's unit (rule 1, forbidden #2 in spirit). checkResidentialUnit is the one place
// the three writing acts (citizen intake, staff intake, classification) ask that question, OUTSIDE and
// BEFORE their transaction: a gRPC round trip inside it would hold the row lock for the network, and the
// intakes must refuse before a lookup code is minted (rule 7, invariant 3).
//
// THE RECORD STORES THE ID, NEVER THE NAME. The name identity answers is today's wording; a renamed or
// retired unit must not rewrite an archival record (ADR 0088 "Theo thời gian"; ADR 0059 §2). Names are
// resolved on read, through ResidentialUnitNamer.
//
// NEVER DERIVED FROM COORDINATES (ADR 0088 stop condition #2): there is no digitised boundary, and a
// nearest-point guess is wrong on every petition near a boundary, silently, straight into the report.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
)

// ActiveResidentialUnitChecker is identity's ResolveActiveResidentialUnits as core/identityclient exposes
// it: id -> today's name for the ids that are ACTIVE units of the commune the context carries. An id
// absent from the map is refused; an error means the check did not happen. *identityclient.Client
// satisfies it.
type ActiveResidentialUnitChecker interface {
	ActiveResidentialUnits(ctx context.Context, ids []string) (map[string]string, error)
}

// ResidentialUnitNamer is identity's ResolveResidentialUnitNames, batched — names for ids a STORED record
// carries, including units since retired. *identityclient.Client satisfies it.
type ResidentialUnitNamer interface {
	ResidentialUnitNamesInBatches(ctx context.Context, ids []string) (map[string]identityclient.ResidentialUnitName, error)
}

// ErrResidentialUnitNotActive — the picked unit is not an active unit of this commune. ONE ERROR FOR
// EVERY REASON (unknown, retired, soft deleted, another commune's): identity collapses them into one
// answer, and re-splitting "another commune" from "unknown" would leak that an id exists elsewhere
// (rule 1). 400 to the sender. Nothing was written and no code was issued.
var ErrResidentialUnitNotActive = errors.New("phan_anh: thôn / tổ dân phố đã chọn không thuộc danh sách xã đang dùng")

// ErrResidentialUnitCheckUnavailable — identity could not be asked. NEVER "not active" and NEVER "write it
// unchecked" or "drop it": the officer or citizen chose a unit, and a petition stored without it, or with
// an unchecked one, is the record ADR 0088 stop condition #1 forbids. 503, retryable.
var ErrResidentialUnitCheckUnavailable = errors.New("phan_anh: chưa kiểm được thôn / tổ dân phố")

// residentialUnitIDMax bounds what is sent to identity. Ids are ULIDs (26 characters); anything longer
// cannot be one, and is refused with the same answer as an unknown id rather than becoming load.
const residentialUnitIDMax = 64

// normaliseResidentialUnitID trims. "" means "none picked" on every act.
func normaliseResidentialUnitID(raw string) string {
	return strings.TrimSpace(raw)
}

// checkResidentialUnit asks identity whether `id` (already normalised, non-empty) is an active unit of
// the commune in ctx. `units` nil is a WIRING FAULT (500), never "accept unchecked".
//
// The commune travels in gRPC metadata from ctx (core/grpcx, set by identityclient.Dial) — never as an
// argument (rule 1, invariant 4); identity answers for that commune only.
func checkResidentialUnit(ctx context.Context, units ActiveResidentialUnitChecker, id string) error {
	if units == nil {
		return errors.New("phan_anh: thiếu bộ kiểm thôn / tổ dân phố — sai nối dây")
	}
	if len(id) > residentialUnitIDMax {
		return ErrResidentialUnitNotActive
	}
	active, err := units.ActiveResidentialUnits(ctx, []string{id})
	if err != nil {
		// The id is not in the message: not personal data, but nothing an operator needs either — the
		// commune is, and identityclient has already logged the gRPC code.
		return fmt.Errorf("%w cho xã %s: %w", ErrResidentialUnitCheckUnavailable, tenant.MustFrom(ctx), err)
	}
	if _, ok := active[id]; !ok {
		return ErrResidentialUnitNotActive
	}
	return nil
}
