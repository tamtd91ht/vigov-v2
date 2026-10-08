/**
 * The four routes of a commune's citizen-letter deadline rules (ADR 0084 #3, ADR 0085 B and câu 2–4):
 * `GET/POST /api/v1/citizen-letter-deadline-rules`, `PATCH/DELETE /api/v1/citizen-letter-deadline-rules/{id}`.
 *
 * ALL FOUR DECLARE `admin.sla` — the key of the `sla` routes (`service-identity/internal/http/routes.go`).
 * Hiding the block without it is UX; the server checks every request (rule 5, forbidden #1).
 *
 * NOTHING IS SEEDED (ADR 0085 câu 4): an empty list is the honest state of a commune that has not typed
 * its figures, and every letter of a type without a rule is booked "Không đặt hạn".
 *
 * NO DEADLINE IS COMPUTED HERE. `identity` owns the rule and the day count (ADR 0085 B4, câu 5); a
 * figure saved here changes no deadline already stored on a letter (rule 10, invariant 2).
 *
 * Relative paths, no cookie handling, no `tenant_id`: those hold for every route and are said once in
 * `goi.ts`.
 */

// vi-name-ok: imports the existing exports of goi.ts unchanged (rule 12 invariant 3)
import {
  docJSON,
  docThanKetQua,
  docThanLoiGoi,
  goiGhi,
  type KetQua,
} from "./goi";
import type {
  identity_citizenLetterDeadlineRuleOut,
  identity_citizenLetterDeadlineRulesOut,
  identity_createCitizenLetterDeadlineRuleIn,
  identity_delete_citizen_letter_deadline_rules_by_id,
  identity_get_citizen_letter_deadline_rules,
  identity_patch_citizen_letter_deadline_rules_by_id,
  identity_post_citizen_letter_deadline_rules,
  identity_removeCitizenLetterDeadlineRuleIn,
  identity_updateCitizenLetterDeadlineRuleIn,
} from "./schema.gen";

/** GET — the commune's live rules, each with `required_unit` and, when unusable, `problem`. */
export function readCitizenLetterDeadlineRules(): Promise<
  KetQua<identity_citizenLetterDeadlineRulesOut>
> {
  const path: identity_get_citizen_letter_deadline_rules["duongDan"] =
    "/api/v1/citizen-letter-deadline-rules";
  return docJSON<identity_citizenLetterDeadlineRulesOut>(path);
}

/**
 * POST — one rule for one (letter type, deadline kind). `Idempotency-Key` REQUIRED (`idem.Required`),
 * held by the open edit so a retry replays the 201 instead of meeting 409
 * `citizen_letter_deadline_rule_exists`. Every refusal is the server's sentence, verbatim.
 */
export function createCitizenLetterDeadlineRule(
  body: identity_createCitizenLetterDeadlineRuleIn,
  idempotencyKey: string,
): Promise<KetQua<identity_citizenLetterDeadlineRuleOut>> {
  // Built field by field, never spread: a key the contract lacks must not ride along.
  const sent: identity_createCitizenLetterDeadlineRuleIn = {
    letter_type: body.letter_type,
    deadline_kind: body.deadline_kind,
    amount: body.amount,
    unit: body.unit,
  };
  const path: identity_post_citizen_letter_deadline_rules["duongDan"] =
    "/api/v1/citizen-letter-deadline-rules";
  return docThanLoiGoi<identity_citizenLetterDeadlineRuleOut>(
    goiGhi(path, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * PATCH — amount and/or unit of an existing rule. No `Idempotency-Key` (`idem.KhongCan`): an identical
 * second PATCH writes nothing. `{}` is refused 400 by the server, on purpose.
 */
export async function updateCitizenLetterDeadlineRule(
  id: string,
  body: identity_updateCitizenLetterDeadlineRuleIn,
): Promise<KetQua<identity_citizenLetterDeadlineRuleOut>> {
  const sent: identity_updateCitizenLetterDeadlineRuleIn = {};
  if (body.amount !== undefined) sent.amount = body.amount;
  if (body.unit !== undefined) sent.unit = body.unit;
  const template: identity_patch_citizen_letter_deadline_rules_by_id["duongDan"] =
    "/api/v1/citizen-letter-deadline-rules/{id}";
  const result = await goiGhi(
    template.replace("{id}", encodeURIComponent(id)),
    "PATCH",
    sent,
    200,
  );
  return docThanKetQua<identity_citizenLetterDeadlineRuleOut>(result);
}

/**
 * DELETE — soft delete with the reason the trail keeps (rule 7). 204, no body. From then on letters of
 * that type are booked "Không đặt hạn". No `Idempotency-Key` (`idem.KhongCan`): a second send is a 404.
 */
export async function removeCitizenLetterDeadlineRule(
  id: string,
  reason: string,
): Promise<KetQua<null>> {
  const body: identity_removeCitizenLetterDeadlineRuleIn = { reason };
  const template: identity_delete_citizen_letter_deadline_rules_by_id["duongDan"] =
    "/api/v1/citizen-letter-deadline-rules/{id}";
  const result = await goiGhi(
    template.replace("{id}", encodeURIComponent(id)),
    "DELETE",
    body,
    204,
  );
  return result.ok ? { ok: true, duLieu: null } : result;
}
