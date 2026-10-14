/**
 * The dropdown lists of `Tải mẫu giao việc` (owner 14/10/2026): the template's unit, assignee and bloc
 * columns become Excel dropdowns, built from codes the CLIENT sends. The server re-checks each code with
 * `identity` and prints identity's names, so only codes leave here — never a label.
 *
 * WHICH ROWS — the ones the task form offers, so the template proposes nothing the form would not:
 *   org units  `GET /api/v1/org-units` — the commune's live units (`code`, the slug the unit column holds)
 *   staff      `GET /api/v1/staff-directory`, unfiltered — the assignee picker's list (active accounts)
 *   blocs      `GET /api/v1/task-blocs`, `active` only — what the form's bloc select offers
 *
 * THE SCREEN'S LISTS FIRST: the Nhiệm vụ register already read all three. A list it does not hold (not
 * read yet, refused, or empty) is read again at the click, through the same clients. A read that fails
 * leaves its list out and the template still downloads — a column without a dropdown beats no template,
 * and the server says nothing about a list it was not sent.
 */

import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import { layKhoiNhiemVu } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import { downloadTaskImportTemplate, type TaskTemplateChoices } from "@/lib/api/task-import";

/** The three reads, injectable so the tests need no network. */
export type TaskChoiceLoaders = {
  readonly orgUnits: () => Promise<KetQua<{ readonly items: readonly { readonly code: string }[] }>>;
  readonly staff: () => Promise<KetQua<{ readonly items: readonly { readonly code: string }[] }>>;
  readonly blocs: () => Promise<KetQua<{ readonly items: readonly { readonly code: string; readonly active: boolean }[] }>>;
};

export const TASK_CHOICE_LOADERS: TaskChoiceLoaders = {
  orgUnits: () => layDanhMucBoPhan(),
  // No unit, no permission filter: the assignee picker's own call (`so-nhiem-vu.tsx`).
  staff: () => layDanhBaChonNguoi(),
  blocs: () => layKhoiNhiemVu(),
};

/** A list counts as held only when it has something in it; otherwise it is read at the click. */
function held(codes: readonly string[] | undefined): codes is readonly string[] {
  return codes !== undefined && codes.length > 0;
}

/**
 * A read's codes, or `undefined` when it failed or threw — never an error the user sees. Shared with the
 * letter template (`features/van-ban/letter-import-dialog.tsx`).
 */
export async function readChoiceCodes<T extends { readonly code: string }>(
  read: () => Promise<KetQua<{ readonly items: readonly T[] }>>,
  keep: (row: T) => boolean = () => true,
): Promise<readonly string[] | undefined> {
  try {
    const r = await read();
    return r.ok ? r.duLieu.items.filter(keep).map((row) => row.code) : undefined;
  } catch {
    return undefined;
  }
}

/**
 * The lists to send: each one the screen holds, else read now. The reads run side by side, and a failed
 * one costs only its own list.
 */
export async function gatherTaskTemplateChoices(
  onScreen: TaskTemplateChoices = {},
  loaders: TaskChoiceLoaders = TASK_CHOICE_LOADERS,
): Promise<TaskTemplateChoices> {
  const [orgUnitCodes, staffCodes, blocCodes] = await Promise.all([
    held(onScreen.orgUnitCodes) ? onScreen.orgUnitCodes : readChoiceCodes(loaders.orgUnits),
    held(onScreen.staffCodes) ? onScreen.staffCodes : readChoiceCodes(loaders.staff),
    held(onScreen.blocCodes) ? onScreen.blocCodes : readChoiceCodes(loaders.blocs, (b) => b.active),
  ]);
  return { orgUnitCodes, staffCodes, blocCodes };
}

/** `Tải mẫu giao việc`: the lists, then the template. Only the template's own refusal is an error. */
export async function fetchTaskTemplate(
  onScreen: TaskTemplateChoices = {},
  loaders: TaskChoiceLoaders = TASK_CHOICE_LOADERS,
): Promise<KetQua<Blob>> {
  return downloadTaskImportTemplate(await gatherTaskTemplateChoices(onScreen, loaders));
}
