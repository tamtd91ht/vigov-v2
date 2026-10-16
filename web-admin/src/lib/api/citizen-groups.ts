/**
 * Nhóm người dân — the GROUP half of `service-identity`'s directory routes (`internal/http/
 * citizen_directory.go`, ADR 0097 D4, chapter 16 §6). A group names a KIND of person ("Tổ trưởng dân
 * phố", "Hộ kinh doanh"), not a person: nothing on these routes is personal data.
 *
 *   GET   /api/v1/citizen-groups                citizen.read    every group, with its live `member_count`
 *   POST  /api/v1/citizen-groups                citizen.update  201 `{id}` · 409 `name_taken` · Idempotency-Key
 *   PATCH /api/v1/citizen-groups/{id}           citizen.update  204 — name, description, order
 *   POST  /api/v1/citizen-groups/{id}/deletion  citizen.update  204 · `{reason}` OPTIONAL (D7); members stay in the directory
 *
 * The Nhóm tab draws no edit control (chapter 16 §6: the prototype has none); the PATCH client is here
 * because the route is, and so that the day a screen needs it the path is already typed.
 *
 * Types come from `schema.gen.ts`; paths are `satisfies <route>["duongDan"]` (rule 9). No `tenant_id`,
 * relative paths, `credentials: same-origin` (`goi.ts`).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type {
  identity_citizenGroupCreatedOut,
  identity_citizenGroupDeletionIn,
  identity_citizenGroupIn,
  identity_citizenGroupsOut,
  identity_get_citizen_groups,
  identity_patch_citizen_groups_by_id,
  identity_post_citizen_groups,
  identity_post_citizen_groups_by_id_deletion,
} from "./schema.gen";

export type { identity_citizenGroupIn as CitizenGroupBody, identity_citizenGroupOut as CitizenGroup } from "./schema.gen";

const LIST_PATH = "/api/v1/citizen-groups" satisfies identity_get_citizen_groups["duongDan"];
const CREATE_PATH = "/api/v1/citizen-groups" satisfies identity_post_citizen_groups["duongDan"];
const UPDATE_PATH = "/api/v1/citizen-groups/{id}" satisfies identity_patch_citizen_groups_by_id["duongDan"];
const DELETION_PATH = "/api/v1/citizen-groups/{id}/deletion" satisfies identity_post_citizen_groups_by_id_deletion["duongDan"];

function withId(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/**
 * The body of a NEW group from the Nhóm tab's one input: the name as typed (the server trims and refuses
 * an empty one), no description, order 0. Every field is sent because the generated type requires all
 * three — an absent field is not "leave the default", it is a 400.
 */
export function newCitizenGroupBody(name: string): identity_citizenGroupIn {
  return { name, description: "", sort_order: 0 };
}

/** `GET /api/v1/citizen-groups` — the whole list, or a refusal. Not paged: a commune has a few dozen at most. */
export function listCitizenGroups(): Promise<KetQua<identity_citizenGroupsOut>> {
  return docJSON<identity_citizenGroupsOut>(LIST_PATH);
}

/**
 * `POST /api/v1/citizen-groups` — 201 with the id. `idempotencyKey` is the caller's, minted per attempt
 * and kept across retries of it (the route declares `idem.Required`): a retry after a lost answer replays
 * the id instead of answering 409 `name_taken` for a group the first send already made.
 */
export function createCitizenGroup(body: identity_citizenGroupIn, idempotencyKey: string): Promise<KetQua<identity_citizenGroupCreatedOut>> {
  return docThanLoiGoi<identity_citizenGroupCreatedOut>(goiGhi(CREATE_PATH, "POST", body, 201, { "Idempotency-Key": idempotencyKey }));
}

/** `PATCH /api/v1/citizen-groups/{id}` — 204. No key: resending one edit lands on one state. */
export async function updateCitizenGroup(id: string, body: identity_citizenGroupIn): Promise<KetQua<null>> {
  const kq = await goiGhi(withId(UPDATE_PATH, id), "PATCH", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * `POST /api/v1/citizen-groups/{id}/deletion` — soft delete, reason OPTIONAL (ADR 0097 D7: removing a
 * group is tidying a catalogue; the people in it stay in the directory). `""` is sent as `""`, never
 * dropped: the generated type requires the field.
 */
export async function deleteCitizenGroup(id: string, reason: string): Promise<KetQua<null>> {
  const body: identity_citizenGroupDeletionIn = { reason };
  const kq = await goiGhi(withId(DELETION_PATH, id), "POST", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
