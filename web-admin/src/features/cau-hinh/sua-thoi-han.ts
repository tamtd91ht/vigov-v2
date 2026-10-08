/**
 * Biểu mẫu sửa một dòng thời hạn: từ sáu ô nhập ra **đúng những trường đã đổi**. Hàm thuần: không
 * gọi mạng, không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO PHẢI SO VỚI DÒNG GỐC CHỨ KHÔNG GỬI CẢ SÁU Ô.
 *
 * `PATCH /api/v1/sla/{id}` đọc "không nhắc tới" là "giữ nguyên" (`suaSLAVao` toàn con trỏ). Gửi cả
 * sáu ô thì mọi lần sửa đều là một lần ghi đè sáu con số — và hai quản trị viên sửa hai cột khác
 * nhau trên cùng một dòng sẽ khiến người lưu sau lặng lẽ kéo các cột kia về giá trị mình đã đọc
 * lúc mở màn hình. Không có gì báo lỗi, và thứ bị kéo lùi là một cam kết của xã với người dân.
 *
 * VÌ SAO CÓ PHÉP KIỂM SỐ Ở ĐÂY.
 *
 * `Number("12 giờ")` ra `NaN`, và `JSON.stringify({x: NaN})` ra `{"x":null}` — mà `null` vào một
 * `*int` của Go là con trỏ rỗng, tức **"không nhắc tới"**; ở cột thứ sáu `null` còn tệ hơn: nó là
 * **"tắt báo"**. Nên một lỗi gõ sẽ đi qua toàn bộ đường ghi và trở thành "không đổi gì" hoặc "tắt
 * báo", với màn hình báo Đã lưu. Phép kiểm này chặn một giá trị biến thành sự im lặng.
 *
 * VÀ ĐÚNG MỘT QUAN HỆ GIỮA CÁC CỘT: Báo Chủ tịch ≥ Báo lãnh đạo trực tiếp. Máy chủ kiểm nó trên dòng
 * SAU KHI SỬA (`domain.KiemTraDongSLA`) và trả 400 kèm câu của nó; ở đây kiểm cùng dòng ấy để cán
 * bộ biết ngay ô nào sai mà không mất một lượt gửi. Bản sao này được phép vì quy tắc là một quyết
 * định đã chốt của người dùng (29/09/2026, ADR 0029 §Bổ sung 29/09), không phải một ngưỡng hay đổi;
 * nếu hai bên có lúc lệch nhau thì câu của máy chủ vẫn hiện ra nguyên văn. Cận trên và mọi từ chối
 * khác vẫn là việc của máy chủ — chép chúng xuống là dựng bản sao thứ hai sẽ trôi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import {
  UNASSIGNED_HOLD_KEY,
  type FieldRowKind,
  type RequiredHoursKey,
  type SuaGioVao,
} from "@/lib/api/thoi-han-xu-ly";
import type { identity_addSLAFieldRowIn, identity_dongSLARa, identity_suaSLAVao } from "@/lib/api/schema.gen";

import {
  FIELD_REQUIRED_ERROR,
  LOI_KHONG_DOI_GI,
  LOI_SO_GIO_LA,
  LOI_THIEU_LY_DO,
  NO_DEFAULT_ROW_ERROR,
  PRESIDENT_BELOW_LEADER_ERROR,
  RESOLVE_HOURS_ERROR,
  UNASSIGNED_HOLD_LABEL,
} from "./nhan-thoi-han";

/** Khoá của một ô giờ — lấy thẳng từ hợp đồng, không gõ lại. */
export type KhoaGio = keyof identity_suaSLAVao;

/**
 * Tên cột trên màn hình. `Record` ĐÒI ĐỦ SÁU KHOÁ: ngày hợp đồng có con số thứ bảy, dòng này đỏ
 * ngay — chứ không lặng lẽ sinh ra một ô nhập không có nhãn, hoặc một cột không ai sửa được.
 *
 * Năm tên đầu lấy đúng đầu cột của đặc tả (`14-cau-hinh.md §8`), không rút gọn: cán bộ đối chiếu
 * màn hình với bảng in trong tài liệu. Cột thứ sáu đặc tả §8 chưa vẽ (nó đến từ quyết định
 * 29/09/2026 cho câu "cả việc bộ phận giữ mà chưa phân công ai" ở §9, `:330`), nên tên của nó nằm ở
 * `nhan-thoi-han.ts` cùng lời giải thích.
 */
export const NHAN_COT: Record<KhoaGio, string> = {
  acknowledge_hours: "Tiếp nhận",
  resolve_hours: "Xử lý xong",
  due_soon_hours: "Sắp đến hạn khi còn",
  escalate_leader_hours: "Báo lãnh đạo trực tiếp",
  escalate_president_hours: "Báo Chủ tịch",
  unassigned_hold_hours: UNASSIGNED_HOLD_LABEL,
};

/** The five figures every row must carry, in the specification's column order. */
export const REQUIRED_HOURS_COLUMNS = [
  "acknowledge_hours",
  "resolve_hours",
  "due_soon_hours",
  "escalate_leader_hours",
  "escalate_president_hours",
] as const satisfies readonly RequiredHoursKey[];

/**
 * Thứ tự cột trên bảng và trong biểu mẫu — năm cột theo đúng thứ tự đặc tả, không theo vần, rồi
 * cột thứ sáu ở cuối: nó không phải một hạn, nên không chen vào giữa các hạn.
 */
export const COT_GIO = [...REQUIRED_HOURS_COLUMNS, UNASSIGNED_HOLD_KEY] as const satisfies readonly KhoaGio[];

/** Bản nháp đang gõ: chuỗi, vì ô nhập của trình duyệt trả về chuỗi. */
export type BanNhapGio = Record<KhoaGio, string>;

/**
 * Nạp dòng đang có vào biểu mẫu — mỗi ô có sẵn con số hiện tại. Riêng cột thứ sáu CHƯA ĐẶT thì ô
 * trống, vì trống là đúng nghĩa của nó ("không báo"), không phải một số bị thiếu.
 */
export function banTuDong(d: identity_dongSLARa): BanNhapGio {
  return {
    acknowledge_hours: String(d.acknowledge_hours),
    resolve_hours: String(d.resolve_hours),
    due_soon_hours: String(d.due_soon_hours),
    escalate_leader_hours: String(d.escalate_leader_hours),
    escalate_president_hours: String(d.escalate_president_hours),
    unassigned_hold_hours: d.unassigned_hold_hours === null ? "" : String(d.unassigned_hold_hours),
  };
}

export type KetQuaSoan = { ok: true; than: SuaGioVao } | { ok: false; loi: string };

/** A typed positive integer, or `null` for anything else — `NaN` must never reach the body. */
function positiveInteger(text: string): number | null {
  const value = text === "" ? Number.NaN : Number(text);
  return Number.isInteger(value) && value > 0 ? value : null;
}

/**
 * Sáu ô nhập + dòng gốc → thân `PATCH`.
 *
 * KHÔNG ĐỔI GÌ THÌ TỪ CHỐI TẠI CHỖ, không gửi `{}` lên để nhận 400. Cán bộ mở biểu mẫu rồi bấm Lưu
 * mà không sửa gì là chuyện thường; một câu ở đây đọc dễ hơn một lời từ chối của máy chủ.
 */
export function soanSua(goc: identity_dongSLARa, ban: BanNhapGio): KetQuaSoan {
  const than: { -readonly [K in RequiredHoursKey]?: number } & {
    [UNASSIGNED_HOLD_KEY]?: number | null;
  } = {};

  for (const khoa of REQUIRED_HOURS_COLUMNS) {
    // Ô TRỐNG KHÔNG PHẢI "GIỮ NGUYÊN". Biểu mẫu mở ra đã có sẵn con số, nên một ô bị xoá trắng là
    // một ý định — và ý định ấy (bỏ hẳn một con số) không có trên tuyến. Nói ra, đừng đoán.
    const text = ban[khoa].trim();
    const so = positiveInteger(text);
    // "Xử lý xong" at zero or below gets spec 08's own sentence; any other bad box the general one.
    if (so === null && khoa === "resolve_hours" && text !== "" && Number(text) <= 0) {
      return { ok: false, loi: RESOLVE_HOURS_ERROR };
    }
    if (so === null) return { ok: false, loi: LOI_SO_GIO_LA };
    if (so !== goc[khoa]) than[khoa] = so;
  }

  // THE SIXTH FIGURE: an empty box IS a value here — `null`, "do not report" — and is sent only when
  // the row had a threshold to clear. Zero is refused locally, as the server does: a stored 0 would
  // be indistinguishable from "off" (migration 0016, `sla_unassigned_hold_hours_positive`).
  const holdText = ban[UNASSIGNED_HOLD_KEY].trim();
  const hold = holdText === "" ? null : positiveInteger(holdText);
  if (holdText !== "" && hold === null) return { ok: false, loi: LOI_SO_GIO_LA };
  if (hold !== goc[UNASSIGNED_HOLD_KEY]) than[UNASSIGNED_HOLD_KEY] = hold;

  // CHECKED ON THE ROW AFTER THE EDIT, exactly as the server does: raising "Báo lãnh đạo trực tiếp"
  // alone can break the rule without touching the other box.
  const leader = than.escalate_leader_hours ?? goc.escalate_leader_hours;
  const president = than.escalate_president_hours ?? goc.escalate_president_hours;
  if (president < leader) return { ok: false, loi: PRESIDENT_BELOW_LEADER_ERROR };

  if (Object.keys(than).length === 0) return { ok: false, loi: LOI_KHONG_DOI_GI };
  return { ok: true, than };
}

/* ---- thêm thời hạn riêng cho một lĩnh vực --------------------------------------------------- */

/** The add row as typed: the two figures the spec row shows, as strings from the boxes. */
export type AddDraft = {
  readonly workKind: FieldRowKind;
  readonly field: string;
  readonly acknowledgeHours: string;
  readonly resolveHours: string;
};

/**
 * The form as it opens. "8" and "48" are spec 08's prefill of two EDITABLE boxes, kept by the owner
 * (ADR 0079 lô 2 Q4) — nothing is stored until the officer presses Thêm, and the boxes show the figure
 * they are about to commit. They are not a commune's SLA: every other figure comes from the commune's
 * own default row (`composeAdd`).
 */
export function newAddDraft(): AddDraft {
  return { workKind: "phan-anh", field: "", acknowledgeHours: "8", resolveHours: "48" };
}

/** Changing the kind clears the field: a document-type code is not a petition field (spec 08). */
export function changeAddKind(draft: AddDraft, workKind: FieldRowKind): AddDraft {
  return { ...draft, workKind, field: "" };
}

export type AddComposed = { ok: true; body: identity_addSLAFieldRowIn } | { ok: false; error: string };

/**
 * Draft + the table → the `POST /api/v1/sla` body.
 *
 * THE FOUR FIGURES THE ROW DOES NOT SHOW come from THIS kind's default row: the spec row has two
 * boxes, the contract requires five figures (six with the hold). A field row with the commune's own
 * thresholds is what "a field differs only in its deadlines" means; any constant here would be an SLA
 * written into source (rule 10 forbidden #3). No default row → refused, never guessed. The officer
 * edits the other figures in place afterwards. Bounds and the ordering rule stay the server's; this
 * checks only what would otherwise send nonsense (no field, a non-number).
 */
export function composeAdd(draft: AddDraft, rows: readonly identity_dongSLARa[]): AddComposed {
  if (draft.field === "") return { ok: false, error: FIELD_REQUIRED_ERROR };

  const acknowledge = positiveInteger(draft.acknowledgeHours.trim());
  if (acknowledge === null) return { ok: false, error: LOI_SO_GIO_LA };

  const resolveText = draft.resolveHours.trim();
  const resolve = positiveInteger(resolveText);
  if (resolve === null) {
    const zeroOrBelow = resolveText !== "" && Number(resolveText) <= 0;
    return { ok: false, error: zeroOrBelow ? RESOLVE_HOURS_ERROR : LOI_SO_GIO_LA };
  }

  const base = rows.find((r) => r.is_default && r.work_kind === draft.workKind);
  if (base === undefined) return { ok: false, error: NO_DEFAULT_ROW_ERROR };

  return {
    ok: true,
    body: {
      work_kind: draft.workKind,
      field: draft.field,
      acknowledge_hours: acknowledge,
      resolve_hours: resolve,
      due_soon_hours: base.due_soon_hours,
      escalate_leader_hours: base.escalate_leader_hours,
      escalate_president_hours: base.escalate_president_hours,
      // `null` = "không báo", the default row's own choice — carried, not replaced by a number.
      unassigned_hold_hours: base.unassigned_hold_hours,
    },
  };
}

/** The reason of a removal, trimmed; empty is refused here — the server refuses it too (rule 7). */
export function composeRemoveReason(text: string): { ok: true; reason: string } | { ok: false; error: string } {
  const reason = text.trim();
  return reason === "" ? { ok: false, error: LOI_THIEU_LY_DO } : { ok: true, reason };
}
