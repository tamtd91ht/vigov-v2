/**
 * The decision half of the Thôn / Tổ dân phố tab's writes (`docs/ui-ux/14-cau-hinh.md §2`, user
 * decision 29/09/2026, ADR 0059 §2): add, edit, Ngừng dùng / Dùng lại. Pure — no network, no DOM — so
 * every body that goes up, and every sentence staff read, has a test.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * THREE RULES THIS FILE EXISTS TO HOLD
 *
 * 1. A BLANK COUNT IS "CHƯA NHẬP", NEVER 0. `household_count` / `population_count` are `number | null`
 *    end to end; `0` asserts "no households", `null` is the absence of an assertion, and a 0 nobody
 *    asserted travels into the report sent upward as a real figure. So on create a blank box is
 *    ABSENT, and on edit clearing a box that held a number sends `null` (the server's "clear").
 * 2. NO DELETE. Taking a unit out of use is `active: false` on the PATCH; the unit stays on the list
 *    and every record pointing at it keeps printing its name (`residential_unit_write.go:20-24`).
 * 3. THE SERVER'S SENTENCE IS SHOWN VERBATIM. Name/code taken, list full, type or head not found —
 *    the rules live on the server with a Vietnamese sentence each; this file never re-checks them and
 *    never branches on `code` (`lib/api/goi.ts`). Only what cannot be SENT is checked here: an empty
 *    name and an order that is not a whole number. A count that is not a whole number ≥ 0 is sent as
 *    "chưa nhập" (`null`) — spec `04-thon-to-dan-pho.md` (ADR 0079) — never as 0, so rule 1 holds.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_canBoChonNguoiRa,
  identity_loaiDonViDanCuRa,
  identity_thonToDanPhoRa,
} from "@/lib/api/schema.gen";
import {
  createResidentialUnit,
  updateResidentialUnit,
  type CreateResidentialUnitBody,
  type UpdateResidentialUnitBody,
} from "@/lib/api/thon-to-dan-pho";

import { CHUA_CO_THAY_DOI, LOI_THU_TU } from "./nhan-so-do";

type Unit = identity_thonToDanPhoRa;

/** Which form is open. The create key is minted when the form OPENS — see `openCreate`. */
export type ResidentialUnitFormOpen =
  | { readonly kind: "create"; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly unit: Unit };

/** The draft being typed. Strings throughout — that is what inputs return. */
export type ResidentialUnitDraft = {
  readonly name: string;
  readonly code: string;
  readonly typeCode: string;
  readonly headStaffCode: string;
  readonly households: string;
  readonly population: string;
  readonly order: string;
};

/**
 * Open the create form — and MINT THE `Idempotency-Key` NOW, not at send. A second press of `Lưu`
 * after a network error must carry the first press's key: the first send may have reached the server,
 * and a new key turns the retry into a second unit of the same name. `mint` is a parameter so the test
 * counts calls; the tab passes `crypto.randomUUID`.
 */
export function openCreate(mint: () => string): ResidentialUnitFormOpen {
  return { kind: "create", idempotencyKey: mint() };
}

export function draftForCreate(): ResidentialUnitDraft {
  return { name: "", code: "", typeCode: "", headStaffCode: "", households: "", population: "", order: "" };
}

/** `null` counts become EMPTY boxes — never "0". */
export function draftFromUnit(u: Unit): ResidentialUnitDraft {
  return {
    name: u.name,
    code: u.code,
    typeCode: u.type_code,
    headStaffCode: u.head_staff_code,
    households: u.household_count === null ? "" : String(u.household_count),
    population: u.population_count === null ? "" : String(u.population_count),
    order: String(u.order),
  };
}

type Parsed<T> = { readonly ok: true; readonly value: T } | { readonly ok: false };

/**
 * A count box: blank = `null` ("chưa nhập"); a whole number ≥ 0 = that number; anything else = a LOCAL
 * error. Thousands written the way staff read them on this screen (`1.132`, `1 132`) are accepted —
 * only in the exact grouped shape, so `1.5` is an error, never 15.
 *
 * NOT `parseInt`: `parseInt("12 hộ")` is 12 — a typo swallowed into a figure.
 */
export function parseCount(box: string): Parsed<number | null> {
  const s = box.trim();
  if (s === "") return { ok: true, value: null };
  const digits = /^\d{1,3}([. ]\d{3})+$/.test(s) ? s.replace(/[. ]/g, "") : s;
  if (!/^\d+$/.test(digits)) return { ok: false };
  const n = Number(digits);
  return Number.isSafeInteger(n) ? { ok: true, value: n } : { ok: false };
}

/** The order box: blank = not given; an integer = that integer; else a local error. */
function parseOrder(box: string): Parsed<number | undefined> {
  const s = box.trim();
  if (s === "") return { ok: true, value: undefined };
  const n = Number(s);
  return Number.isInteger(n) ? { ok: true, value: n } : { ok: false };
}

export type BodyResult<T> =
  | { readonly kind: "send"; readonly body: T }
  | { readonly kind: "unchanged" }
  | { readonly kind: "error"; readonly message: string };

type ParsedNumbers = {
  readonly households: number | null;
  readonly population: number | null;
  readonly order: number | undefined;
};

/** Spec 04: an invalid count is sent as `null` ("chưa nhập") — the absence of a figure, never a 0. */
function countOrNull(box: string): number | null {
  const parsed = parseCount(box);
  return parsed.ok ? parsed.value : null;
}

function parseNumbers(d: ResidentialUnitDraft): { ok: true; v: ParsedNumbers } | { ok: false; message: string } {
  const order = parseOrder(d.order);
  if (!order.ok) return { ok: false, message: LOI_THU_TU };
  return { ok: true, v: { households: countOrNull(d.households), population: countOrNull(d.population), order: order.value } };
}

/**
 * POST body. Every optional box left blank is ABSENT: `code` blank = the server derives it from the
 * name; `type_code` blank = not classified; `head_staff_code` blank = no head; a blank count = not
 * entered (rule 1 of the header — absent, never 0). The name is sent AS TYPED; the server checks it.
 */
export function createBody(d: ResidentialUnitDraft): BodyResult<CreateResidentialUnitBody> {
  const p = parseNumbers(d);
  if (!p.ok) return { kind: "error", message: p.message };
  const code = d.code.trim();
  return {
    kind: "send",
    body: {
      name: d.name,
      code: code === "" ? undefined : code,
      type_code: d.typeCode === "" ? undefined : d.typeCode,
      head_staff_code: d.headStaffCode === "" ? undefined : d.headStaffCode,
      household_count: p.v.households ?? undefined,
      population_count: p.v.population ?? undefined,
      order: p.v.order,
    },
  };
}

/**
 * PATCH body — ONLY the fields that differ from the unit as it was when the form opened.
 *
 * `type_code: ""` / `head_staff_code: ""` CLEAR (the picker's "none" option is `""`), distinct from
 * absent. A count box emptied where the unit held a number sends `null` — CLEAR, "chưa nhập" — never
 * 0. An emptied `Thứ tự` is NO CHANGE, not 0: an edit to the name must not move the unit to the top.
 * `code` is never here (400 `code_not_editable`); `active` goes only through `activeToggleBody`.
 *
 * NOTHING CHANGED = NOTHING SENT: the server answers 400 to an empty body, and a press of `Lưu` that
 * changed nothing deserves a sentence that says so.
 */
export function updateBody(u: Unit, d: ResidentialUnitDraft): BodyResult<UpdateResidentialUnitBody> {
  const p = parseNumbers(d);
  if (!p.ok) return { kind: "error", message: p.message };
  const body: {
    name?: string;
    type_code?: string;
    head_staff_code?: string;
    household_count?: number | null;
    population_count?: number | null;
    order?: number;
  } = {};
  if (d.name !== u.name) body.name = d.name;
  if (d.typeCode !== u.type_code) body.type_code = d.typeCode;
  if (d.headStaffCode !== u.head_staff_code) body.head_staff_code = d.headStaffCode;
  if (p.v.households !== u.household_count) body.household_count = p.v.households;
  if (p.v.population !== u.population_count) body.population_count = p.v.population;
  if (p.v.order !== undefined && p.v.order !== u.order) body.order = p.v.order;
  return Object.keys(body).length === 0 ? { kind: "unchanged" } : { kind: "send", body };
}

/** Ngừng dùng / Dùng lại — the ONE field, nothing else rides along. */
export function activeToggleBody(u: Unit): UpdateResidentialUnitBody {
  return { active: !u.active };
}

/* ---- sending ------------------------------------------------------------------------------- */

/** The two write calls. A parameter so tests substitute them; the tab passes `RESIDENTIAL_UNIT_API`. */
export type ResidentialUnitApi = {
  readonly create: (body: CreateResidentialUnitBody, key: string) => Promise<KetQua<Unit>>;
  readonly update: (id: string, body: UpdateResidentialUnitBody) => Promise<KetQua<Unit>>;
};

export const RESIDENTIAL_UNIT_API: ResidentialUnitApi = {
  create: createResidentialUnit,
  update: updateResidentialUnit,
};

/**
 * One press of `Lưu` / `Ngừng dùng` / `Dùng lại`. TWO KINDS OF FAILURE, kept apart: a local one
 * (nothing was sent — a box is mistyped) and the server's sentence, verbatim.
 */
export type SubmitResult =
  | { readonly kind: "done"; readonly sentence: string }
  | { readonly kind: "localError"; readonly message: string }
  | { readonly kind: "serverError"; readonly message: string };

export async function submitResidentialUnitForm(
  open: ResidentialUnitFormOpen,
  d: ResidentialUnitDraft,
  api: ResidentialUnitApi,
): Promise<SubmitResult> {
  if (d.name.trim() === "") return { kind: "localError", message: NAME_REQUIRED };
  if (open.kind === "create") {
    const b = createBody(d);
    if (b.kind === "error") return { kind: "localError", message: b.message };
    if (b.kind === "unchanged") return { kind: "localError", message: CHUA_CO_THAY_DOI };
    // The key minted at open — never one minted here (see `openCreate`).
    const r = await api.create(b.body, open.idempotencyKey);
    return r.ok ? { kind: "done", sentence: ADDED_TOAST } : { kind: "serverError", message: r.thongBao };
  }
  const b = updateBody(open.unit, d);
  if (b.kind === "error") return { kind: "localError", message: b.message };
  if (b.kind === "unchanged") return { kind: "localError", message: CHUA_CO_THAY_DOI };
  const r = await api.update(open.unit.id, b.body);
  return r.ok ? { kind: "done", sentence: SAVED_TOAST } : { kind: "serverError", message: r.thongBao };
}

export async function toggleResidentialUnitActive(u: Unit, api: ResidentialUnitApi): Promise<SubmitResult> {
  const r = await api.update(u.id, activeToggleBody(u));
  if (!r.ok) return { kind: "serverError", message: r.thongBao };
  return { kind: "done", sentence: r.duLieu.active ? reactivatedSentence(r.duLieu.name) : retiredSentence(r.duLieu.name) };
}

/* ---- pickers of the form ------------------------------------------------------------------- */

export type PickerOption = { readonly value: string; readonly label: string };

/**
 * The `Loại` picker: types IN USE only — the server refuses an out-of-use type with 400
 * `residential_unit_type_not_found`, so offering one is offering a refusal.
 *
 * ONE EXCEPTION, on edit: the unit's CURRENT type, when it has since been taken out of use (or its
 * label has gone), stays as an option marked as such. Without it the `<select>` would show the first
 * option while the unit still carries the old code — and a save would silently re-classify it.
 */
export function typeOptions(
  types: readonly identity_loaiDonViDanCuRa[],
  current: { readonly code: string; readonly label: string } | null,
): readonly PickerOption[] {
  const inUse = types.filter((t) => t.active).map((t) => ({ value: t.code, label: t.label }));
  if (current === null || current.code === "" || inUse.some((o) => o.value === current.code)) return inUse;
  const name = current.label === "" ? current.code : current.label;
  return [...inUse, { value: current.code, label: `${name} (đã ngừng dùng — đang gán cho địa bàn này)` }];
}

/**
 * The `Trưởng thôn / Tổ trưởng` picker, from `GET /api/v1/staff-directory` — live, unlocked staff of
 * THIS commune only (the server's own filter), sent back by `code`, the business code the server
 * checks (`head_staff_code`).
 *
 * Same exception as `typeOptions`: a current head no longer in the directory (locked, left) stays as
 * one marked option on edit, so opening the form does not silently change who the head is.
 */
export function headOptions(
  directory: readonly identity_canBoChonNguoiRa[],
  current: { readonly code: string; readonly name: string } | null,
): readonly PickerOption[] {
  const live = directory.map((s) => ({
    value: s.code,
    label: s.position === "" ? `${s.full_name} (${s.code})` : `${s.full_name} — ${s.position} (${s.code})`,
  }));
  if (current === null || current.code === "" || live.some((o) => o.value === current.code)) return live;
  const name = current.name === "" ? current.code : current.name;
  return [{ value: current.code, label: `${name} (không còn trong danh bạ cán bộ)` }, ...live];
}

/* ---- words --------------------------------------------------------------------------------- */

// No leading "+": the screen draws a lucide `Plus` beside the words (ADR 0068 §2).
export const ADD_BUTTON = "Thêm thôn / tổ dân phố";
// No emoji: the screen draws a lucide `Pencil` beside the word (ADR 0068 §2).
export const EDIT_BUTTON = "Sửa";
export const RETIRE_BUTTON = "Ngừng dùng";
export const REACTIVATE_BUTTON = "Dùng lại";
export const CONFIRM_RETIRE_BUTTON = "Xác nhận ngừng dùng";

export const CREATE_TITLE = "Thêm thôn / tổ dân phố";
export function editTitle(name: string): string {
  return `Sửa ${name}`;
}
export function editButtonLabel(name: string): string {
  return `Sửa ${name}`;
}
export function retireButtonLabel(name: string): string {
  return `Ngừng dùng ${name}`;
}
export function reactivateButtonLabel(name: string): string {
  return `Dùng lại ${name}`;
}

export const FIELD_NAME = "Tên";
export const FIELD_TYPE = "Loại";
export const FIELD_HEAD = "Trưởng thôn / Tổ trưởng";
export const FIELD_HOUSEHOLDS = "Số hộ";
export const FIELD_POPULATION = "Nhân khẩu";
export const FIELD_ORDER = "Thứ tự";

// The spec's first option. On edit it also CLEARS the type (`type_code: ""`): Loại stays editable
// after create (ADR 0079 decision 3).
export const TYPE_NONE = "— Chọn loại —";
export const HEAD_NONE = "Chưa có";

export const NAME_PLACEHOLDER = "Ví dụ: Tổ dân phố 5";
/** A cell with no value (null count, unclassified, no head) — spec 04 "rỗng thì —". */
export const EMPTY_CELL = "—";

/**
 * Said before `Ngừng dùng` is confirmed — the one thing staff need to know before pressing it: nothing
 * is erased, and nothing already recorded changes.
 */
export function retireConfirmSentence(name: string): string {
  return (
    `Ngừng dùng ${name}? Địa bàn vẫn nằm trong danh sách, và mọi hồ sơ, phản ánh đã lập ở đây vẫn giữ ` +
    `nguyên tên địa bàn. Chỉ là không chọn được địa bàn này cho hồ sơ mới. Có thể bấm Dùng lại bất cứ lúc nào.`
  );
}

// Spec 04's sentences: toasts on success, in place in the form on failure.
export const ADDED_TOAST = "Đã thêm đơn vị dân cư mới.";
export const SAVED_TOAST = "Đã lưu.";
export const NAME_REQUIRED = "Vui lòng nhập tên thôn hoặc tổ dân phố.";
/** Said before the server's own sentence, when there is one. */
export const SAVE_FAILED = "Không lưu được đơn vị dân cư.";
export function retiredSentence(name: string): string {
  return `Đã ngừng dùng ${name}. Hồ sơ đã lập ở địa bàn này vẫn giữ nguyên tên địa bàn.`;
}
export function reactivatedSentence(name: string): string {
  return `Đã đưa ${name} vào dùng lại.`;
}
