/**
 * "Lời hệ thống" (`docs/ui-ux/14-cau-hinh.md §7`) — the sentences a service says when it refuses,
 * which a commune may reword. Two services own such sentences today, each its own three routes, all
 * `admin.lookup`:
 *
 *   GET    /api/v1/{petitions,finance}-system-messages
 *   PUT    /api/v1/{petitions,finance}-system-messages/{code}/override   { text } → 200 the message
 *   DELETE /api/v1/{petitions,finance}-system-messages/{code}/override   → 204, back to the default
 *
 * THERE IS NO "TẮT" AND NO "THÊM": the catalogue is closed and lives in each service's code (a key
 * outside it answers 404), and a sentence cannot be switched off — a refusal with nothing to say is
 * one the officer cannot act on. DELETE removes the commune's wording, never the sentence.
 *
 * NO Idempotency-Key: neither route declares one (no `x-vigov-idempotency` in the contract). PUT sets
 * an absolute value and DELETE restores one; repeating either lands in the same state.
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";
import type {
  finance_delete_finance_system_messages_by_code_override,
  finance_get_finance_system_messages,
  finance_put_finance_system_messages_by_code_override,
  finance_rewordSystemMessageIn,
  finance_systemMessageListOut,
  finance_systemMessageOut,
  petitions_delete_petitions_system_messages_by_code_override,
  petitions_get_petitions_system_messages,
  petitions_put_petitions_system_messages_by_code_override,
  petitions_rewordSystemMessageIn,
  petitions_systemMessageListOut,
  petitions_systemMessageOut,
} from "./schema.gen";

export type SystemMessageModule = "petitions" | "finance";

/** Both services answer the same shape; the union keeps each generated type, copies neither. */
export type SystemMessage = petitions_systemMessageOut | finance_systemMessageOut;
type SystemMessageList = petitions_systemMessageListOut | finance_systemMessageListOut;
type RewordBody = petitions_rewordSystemMessageIn & finance_rewordSystemMessageIn;

const LIST_PATH = {
  petitions: "/api/v1/petitions-system-messages" satisfies petitions_get_petitions_system_messages["duongDan"],
  finance: "/api/v1/finance-system-messages" satisfies finance_get_finance_system_messages["duongDan"],
} as const;

// The contract's templates, kept so a renamed route turns `tsc` red here.
const OVERRIDE_TEMPLATE = {
  petitions: "/api/v1/petitions-system-messages/{code}/override" satisfies
    petitions_put_petitions_system_messages_by_code_override["duongDan"] &
      petitions_delete_petitions_system_messages_by_code_override["duongDan"],
  finance: "/api/v1/finance-system-messages/{code}/override" satisfies
    finance_put_finance_system_messages_by_code_override["duongDan"] &
      finance_delete_finance_system_messages_by_code_override["duongDan"],
} as const;

function overridePath(module: SystemMessageModule, code: string): string {
  return OVERRIDE_TEMPLATE[module].replace("{code}", encodeURIComponent(code));
}

export async function listSystemMessages(
  module: SystemMessageModule,
): Promise<KetQua<readonly SystemMessage[]>> {
  const r = await docJSON<SystemMessageList>(LIST_PATH[module]);
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}

/**
 * Save the commune's wording. `text` goes up exactly as given — the form trims and bounds it
 * (`system-message-form.ts`), and the server normalises again and refuses with a sentence (400) what
 * it will not store; that sentence reaches the screen verbatim.
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
