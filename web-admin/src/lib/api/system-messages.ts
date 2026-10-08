/**
 * "Lời hệ thống" (`docs/ui-ux/14-cau-hinh.md §7`, ADR 0079 Q2/Q5) — the sentences a service says when it
 * refuses, which a commune may reword, switch off, and (since Q2) add to. All routes are `admin.lookup`.
 *
 * A SHIPPED sentence (`origin: "shipped"`) is reworded through its `override` sub-resource, in all three
 * services:
 *
 *   GET    /api/v1/{petitions,finance,reporting}-system-messages
 *   PUT    …/{code}/override   { text }       → 200  the commune's wording
 *   PATCH  …/{code}/override   { is_active }  → 200  "Tắt / Bật lại" of that wording (409 no_commune_wording
 *                                                    when the commune never reworded it)
 *   DELETE …/{code}/override                  → 204  back to the software's sentence
 *
 * A COMMUNE sentence (`origin: "commune"`, "Xã tự thêm") lives in petitions (groups `phan-anh`, `chung`)
 * and finance (`giai-ngan`) only — reporting takes none (ADR 0079 Q5a):
 *
 *   POST   /api/v1/{petitions,finance}-system-messages   { group_code, code, text, description? } → 201
 *          Idempotency-Key REQUIRED (409 message_code_taken for a code already used, deleted ones included)
 *   PATCH  …/{code}   { text?, is_active? } → 200
 *   DELETE …/{code}   { reason }            → 204, a soft delete (rule 7)
 *
 * PUT, both PATCHes and both DELETEs declare no idempotency: each sets an absolute state.
 *
 * `reporting` owns the `report.*` keys (ADR 0024 §Phụ, Bổ sung 29/09/2026).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";
import type {
  finance_createCustomMessageIn,
  finance_delete_finance_system_messages_by_code,
  finance_delete_finance_system_messages_by_code_override,
  finance_deleteCustomMessageIn,
  finance_editCustomMessageIn,
  finance_get_finance_system_messages,
  finance_patch_finance_system_messages_by_code,
  finance_patch_finance_system_messages_by_code_override,
  finance_post_finance_system_messages,
  finance_put_finance_system_messages_by_code_override,
  finance_rewordSystemMessageIn,
  finance_switchSystemMessageIn,
  finance_systemMessageListOut,
  finance_systemMessageOut,
  petitions_createCustomMessageIn,
  petitions_delete_petitions_system_messages_by_code,
  petitions_delete_petitions_system_messages_by_code_override,
  petitions_deleteCustomMessageIn,
  petitions_editCustomMessageIn,
  petitions_get_petitions_system_messages,
  petitions_patch_petitions_system_messages_by_code,
  petitions_patch_petitions_system_messages_by_code_override,
  petitions_post_petitions_system_messages,
  petitions_put_petitions_system_messages_by_code_override,
  petitions_rewordSystemMessageIn,
  petitions_switchSystemMessageIn,
  petitions_systemMessageListOut,
  petitions_systemMessageOut,
  reporting_delete_reporting_system_messages_by_code_override,
  reporting_get_reporting_system_messages,
  reporting_patch_reporting_system_messages_by_code_override,
  reporting_put_reporting_system_messages_by_code_override,
  reporting_rewordSystemMessageIn,
  reporting_switchSystemMessageIn,
  reporting_systemMessageListOut,
  reporting_systemMessageOut,
} from "./schema.gen";

export type SystemMessageModule = "petitions" | "finance" | "reporting";

/** The services that store commune sentences. `reporting` is not one of them (ADR 0079 Q5a). */
export type CommuneMessageModule = Exclude<SystemMessageModule, "reporting">;

/** The services answer the same shape; the union keeps each generated type, copies none. */
export type SystemMessage = petitions_systemMessageOut | finance_systemMessageOut | reporting_systemMessageOut;
type SystemMessageList =
  | petitions_systemMessageListOut
  | finance_systemMessageListOut
  | reporting_systemMessageListOut;
type RewordBody = petitions_rewordSystemMessageIn &
  finance_rewordSystemMessageIn &
  reporting_rewordSystemMessageIn;
type SwitchBody = petitions_switchSystemMessageIn & finance_switchSystemMessageIn & reporting_switchSystemMessageIn;
export type CreateMessageBody = petitions_createCustomMessageIn & finance_createCustomMessageIn;
/**
 * `description` is left out ON PURPOSE: the generator renders the server's three-state
 * `optionalDescription` as `Record<string, never>`, so no string fits it. The screen never edits a
 * commune sentence's description, so nothing is lost; absent means "keep" on the server.
 */
type EditCommuneBody = Omit<petitions_editCustomMessageIn & finance_editCustomMessageIn, "description">;
type DeleteCommuneBody = petitions_deleteCustomMessageIn & finance_deleteCustomMessageIn;

const LIST_PATH = {
  petitions: "/api/v1/petitions-system-messages" satisfies petitions_get_petitions_system_messages["duongDan"] &
    petitions_post_petitions_system_messages["duongDan"],
  finance: "/api/v1/finance-system-messages" satisfies finance_get_finance_system_messages["duongDan"] &
    finance_post_finance_system_messages["duongDan"],
  reporting: "/api/v1/reporting-system-messages" satisfies reporting_get_reporting_system_messages["duongDan"],
} as const;

// The contract's templates, kept so a renamed route turns `tsc` red here.
const OVERRIDE_TEMPLATE = {
  petitions: "/api/v1/petitions-system-messages/{code}/override" satisfies
    petitions_put_petitions_system_messages_by_code_override["duongDan"] &
      petitions_patch_petitions_system_messages_by_code_override["duongDan"] &
      petitions_delete_petitions_system_messages_by_code_override["duongDan"],
  finance: "/api/v1/finance-system-messages/{code}/override" satisfies
    finance_put_finance_system_messages_by_code_override["duongDan"] &
      finance_patch_finance_system_messages_by_code_override["duongDan"] &
      finance_delete_finance_system_messages_by_code_override["duongDan"],
  reporting: "/api/v1/reporting-system-messages/{code}/override" satisfies
    reporting_put_reporting_system_messages_by_code_override["duongDan"] &
      reporting_patch_reporting_system_messages_by_code_override["duongDan"] &
      reporting_delete_reporting_system_messages_by_code_override["duongDan"],
} as const;

const COMMUNE_TEMPLATE = {
  petitions: "/api/v1/petitions-system-messages/{code}" satisfies
    petitions_patch_petitions_system_messages_by_code["duongDan"] &
      petitions_delete_petitions_system_messages_by_code["duongDan"],
  finance: "/api/v1/finance-system-messages/{code}" satisfies
    finance_patch_finance_system_messages_by_code["duongDan"] &
      finance_delete_finance_system_messages_by_code["duongDan"],
} as const;

function overridePath(module: SystemMessageModule, code: string): string {
  return OVERRIDE_TEMPLATE[module].replace("{code}", encodeURIComponent(code));
}

function communePath(module: CommuneMessageModule, code: string): string {
  return COMMUNE_TEMPLATE[module].replace("{code}", encodeURIComponent(code));
}

export async function listSystemMessages(
  module: SystemMessageModule,
): Promise<KetQua<readonly SystemMessage[]>> {
  const r = await docJSON<SystemMessageList>(LIST_PATH[module]);
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}

/**
 * Save the commune's wording of a SHIPPED sentence. `text` goes up exactly as given — the form trims
 * and bounds it (`system-message-form.ts`), and the server normalises again and refuses with a sentence
 * (400) what it will not store; that sentence reaches the screen verbatim.
 */
export function rewordSystemMessage(
  module: SystemMessageModule,
  code: string,
  text: string,
): Promise<KetQua<SystemMessage>> {
  const body: RewordBody = { text };
  return docThanLoiGoi<SystemMessage>(goiGhi(overridePath(module, code), "PUT", body, 200));
}

/** Back to the software's sentence. 204, no body. */
export async function restoreSystemMessage(
  module: SystemMessageModule,
  code: string,
): Promise<KetQua<null>> {
  const r = await goiGhi(overridePath(module, code), "DELETE", undefined, 204);
  return r.ok ? { ok: true, duLieu: null } : r;
}

/**
 * "Tắt / Bật lại" of the commune's wording of a SHIPPED sentence: off keeps the words and puts the
 * software's sentence back in force. A sentence never reworded answers 409 `no_commune_wording`.
 */
export function switchSystemMessage(
  module: SystemMessageModule,
  code: string,
  isActive: boolean,
): Promise<KetQua<SystemMessage>> {
  const body: SwitchBody = { is_active: isActive };
  return docThanLoiGoi<SystemMessage>(goiGhi(overridePath(module, code), "PATCH", body, 200));
}

/**
 * Add a commune sentence. `idempotencyKey` is minted when the add form OPENS and kept across a retry,
 * so a second click after a lost answer is the same add, not a second sentence.
 */
export function createCommuneMessage(
  module: CommuneMessageModule,
  body: CreateMessageBody,
  idempotencyKey: string,
): Promise<KetQua<SystemMessage>> {
  return docThanLoiGoi<SystemMessage>(
    goiGhi(LIST_PATH[module], "POST", body, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/** Reword or switch a commune sentence (PATCH …/{code}); only the fields given change. */
export function editCommuneMessage(
  module: CommuneMessageModule,
  code: string,
  body: EditCommuneBody,
): Promise<KetQua<SystemMessage>> {
  return docThanLoiGoi<SystemMessage>(goiGhi(communePath(module, code), "PATCH", body, 200));
}

/**
 * Soft delete a commune sentence (rule 7). The reason travels in the BODY — a query string would put
 * free text into every access log. 204, no body. A shipped code answers 409 `system_message`.
 */
export async function deleteCommuneMessage(
  module: CommuneMessageModule,
  code: string,
  reason: string,
): Promise<KetQua<null>> {
  const body: DeleteCommuneBody = { reason };
  const r = await goiGhi(communePath(module, code), "DELETE", body, 204);
  return r.ok ? { ok: true, duLieu: null } : r;
}
