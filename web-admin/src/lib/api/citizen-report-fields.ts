/**
 * The commune's view of "Lĩnh vực phản ánh" — `GET /api/v1/citizen-report-fields` and
 * `PATCH /api/v1/citizen-report-fields/{code}`, both `admin.lookup`, owned by `service-petitions`.
 *
 * WHY A FILE OF ITS OWN, NOT AN ENTRY OF THE SEVEN WRITABLE CATALOGUES (`danh-muc.ts`): the codes are
 * the VENDOR's tier-1 list (ADR 0026, ADR 0060). A commune edits its label and order and switches a
 * code on or off; it never adds, deletes or renames one — so there is no POST, no DELETE, and an entry
 * is named by `code` on the PATH, not by `id`. Same shape as `trang-thai-nhiem-vu.ts`.
 *
 * `active` is the vendor's flag (a retired code); `enabled` is the commune's switch. Only `enabled` is
 * ever sent.
 *
 * RELATIVE PATHS, NO COOKIE HANDLING, NO `tenant_id`: see `goi.ts`.
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing exports of goi.ts, imported not declared (rule 12 inv 3)
import type {
  petitions_get_citizen_report_fields,
  petitions_patch_citizen_report_fields_by_code,
  petitions_petitionFieldListOut,
  petitions_petitionFieldOut,
  petitions_updatePetitionFieldIn,
} from "./schema.gen";

/** GET — every code, disabled and retired ones included, in the commune's order. */
export function readCitizenReportFields(): Promise<KetQua<petitions_petitionFieldListOut>> { // vi-name-ok: KetQua is goi.ts's existing result type
  const path: petitions_get_citizen_report_fields["duongDan"] = "/api/v1/citizen-report-fields"; // vi-name-ok: `duongDan` is the generated contract field
  return docJSON<petitions_petitionFieldListOut>(path);
}

/**
 * What the screen may send: the contract's body MINUS `code`, which it declares only to refuse it (a
 * code is never renamed, rule 7 invariant 3).
 */
export type CitizenReportFieldPatch = Omit<petitions_updatePetitionFieldIn, "code">;

/**
 * PATCH one code. Built field by field, never spread: an absent field means "unchanged", and a key the
 * contract refuses must not ride along. No `Idempotency-Key`: the route declares none (an absolute
 * upsert, `idem.KhongCan`).
 */
export function updateCitizenReportField(
  code: string,
  body: CitizenReportFieldPatch,
): Promise<KetQua<petitions_petitionFieldOut>> { // vi-name-ok: KetQua is goi.ts's existing result type
  const pattern: petitions_patch_citizen_report_fields_by_code["duongDan"] = "/api/v1/citizen-report-fields/{code}"; // vi-name-ok: generated contract field
  const sent: CitizenReportFieldPatch = { label: body.label, order: body.order, enabled: body.enabled };
  return docThanLoiGoi<petitions_petitionFieldOut>( // vi-name-ok: existing goi.ts helper
    goiGhi(pattern.replace("{code}", encodeURIComponent(code)), "PATCH", sent, 200), // vi-name-ok: existing goi.ts helper
  );
}
