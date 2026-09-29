/**
 * PHẦN THUẦN CỦA APP RIÊNG MỘT XÃ — họ tên lấy từ Zalo MỘT LẦN lúc mở app (`GetZaloName`, `NameAtEntry`),
 * biểu mẫu gửi phản ánh (`ReportDraft`, `checkDraft`), nháp trên máy (`FeedbackDraftStore`), danh mục lĩnh
 * vực tạm, và các tên cũ của bảng nhóm trạng thái.
 *
 * TÊN TỆP (30/09/2026, ADR 0061 lớp A): trước đây là `trai-nghiem.ts` — tới 29/09/2026 đây là "bản trải
 * nghiệm", phiếu chỉ sống trong bộ nhớ, mã `TN-`. Phần ấy đã bị gỡ; phiếu nay đi vào sổ phản ánh THẬT của xã
 * qua `api/vigov-client.ts` (`CommuneAppReports.tsx`), và hạn, trạng thái, đánh giá đều là của máy chủ.
 */
import { MAX_LENGTH } from "../api/citizen-report-contract";

import { groupOf, STATUS_GROUP_LABEL, type StatusFilter, type StatusGroup, STEP_LABEL } from "./status-groups";

/**
 * HỌ TÊN TỪ ZALO — LẤY MỘT LẦN, LÚC MỞ APP CỦA XÃ (quyết định của người dùng 29/09/2026, thay "xin quyền
 * tại chỗ cần" của ea76c9d). Lớp vỏ tiêm hàm thật (`getUserInfo`); nửa này chỉ khai kiểu.
 *
 * WHY AT ENTRY AND NOWHERE ELSE: the name is display and pre-fill only — it grants nothing (rule 4: a
 * Zalo display name is not an identity). A "Lấy từ Zalo" button on two screens was two places asking for
 * the same thing, and a citizen who had already answered at one met the question again at the other.
 * Now `CommuneHome` asks once; every screen below it only SHOWS what came back, or "Chưa xác định".
 *
 * `mode`: "check" never opens Zalo's dialog (already allowed → the name; otherwise a failure branch);
 * "ask" opens it, and is only sent after the entry card has said why the name is wanted (policy 3.3.4).
 *
 * SỐ ĐIỆN THOẠI KHÔNG CÓ Ở ĐÂY: `getPhoneNumber` chỉ trả MÃ, và từ 29/09/2026 mã ấy chỉ dùng để MỞ PHIÊN
 * (`vihat-miniapp` đổi mã thành số và chuyển sang ViGov — `commune-session.ts`). Số không bao giờ về điện
 * thoại, nên không điền sẵn được ô "Số điện thoại" — bà con tự gõ số nếu muốn xã gọi lại.
 */
export type NameResult =
  | { readonly kind: "xong"; readonly full_name: string }
  | { readonly kind: "tu-choi" }
  | { readonly kind: "ngoai-zalo" }
  | { readonly kind: "khong-lay-duoc" };
export type NameRequestMode = "check" | "ask";
export type GetZaloName = (mode: NameRequestMode) => Promise<NameResult>;

/**
 * Where the entry name request stands. `settled` is final for this open: `name` is the Zalo name, or
 * `null` (refused, failed, outside Zalo) — the screens then show "Chưa xác định" and an empty name field.
 */
export type NameAtEntry =
  | { readonly kind: "checking" }
  | { readonly kind: "needs-consent" }
  | { readonly kind: "asking" }
  | { readonly kind: "settled"; readonly name: string | null };

/**
 * After the silent check. PURE. Outside Zalo there is nothing to ask, so it settles on `null` instead of
 * showing a card whose button can only fail. Any other failure (not allowed yet, platform error) offers
 * the entry card: asking is the only way the name can still arrive.
 */
export function afterNameCheck(result: NameResult): NameAtEntry {
  if (result.kind === "xong") return { kind: "settled", name: result.full_name };
  if (result.kind === "ngoai-zalo") return { kind: "settled", name: null };
  return { kind: "needs-consent" };
}

/** After Zalo's dialog. PURE. Whatever the answer, it is not asked again in this open. */
export function afterNameAsk(result: NameResult): NameAtEntry {
  return { kind: "settled", name: result.kind === "xong" ? result.full_name : null };
}

/** The name the screens may show, or `null` while the request is unfinished or came back empty. */
export function nameShown(state: NameAtEntry): string | null {
  return state.kind === "settled" ? state.name : null;
}

/*
 * VỊ TRÍ HIỆN TẠI — the token-only `LayMaViTri` that stood here was replaced on 29/09/2026 by
 * `GetSceneLocation` (`scene-location.tsx`): the shell now exchanges the `getLocation` token at
 * `vihat-miniapp` `POST /api/v1/location` and hands down real coordinates, never a guessed address.
 */

/** Chữ cái đầu của TÊN GỌI (từ cuối) — quy ước của bản mẫu. */
export function initials(full_name: string): string {
  const last = full_name.trim().split(/\s+/).pop() ?? "";
  return (last[0] ?? "C").toUpperCase();
}

/** Lời chào theo giờ, như bản mẫu. Giờ Việt Nam do lớp gọi truyền vào — hàm không đọc đồng hồ. */
export function greeting(hour: number): string {
  if (hour < 11) return "Chào buổi sáng";
  if (hour < 18) return "Chào buổi chiều";
  return "Chào buổi tối";
}

/* ═══════════════════════════════ VÒNG ĐỜI · LĨNH VỰC · BIỂU MẪU ═══════════════════════════════ */

/**
 * THỨ TỰ VÒNG ĐỜI cho dòng thời gian — đúng bảy trạng thái của nhánh chính trong `STATUS`
 * (`copy.ts`). Hai nhánh kết thúc (`khong-tiep-nhan`, `chuyen-cap-tren`) không nằm trên đường này:
 * phiếu ở hai nhánh ấy dừng sau "Đang phân loại".
 */
export const LIFECYCLE: readonly string[] = [
  "da-tiep-nhan",
  "dang-phan-loai",
  "da-chuyen-xu-ly",
  "dang-xu-ly",
  "da-xu-ly",
  "cho-dan-xac-nhan",
  "da-dong",
];

/*
 * LĨNH VỰC — the temporary twelve-name list that stood here (`LINH_VUC_TAM`) was removed on 29/09/2026:
 * step 1 now reads the commune's own catalogue (`GET /api/v1/my-citizen-report-fields`, `CommuneAppReports.tsx`
 * `FieldStep`), and there is deliberately NO built-in list to fall back on (ADR 0060 §3). The staff-conduct
 * field (`can-bo`) is never offered on the citizen form by the server, so its special note went with it.
 */

/*
 * ĐÁNH GIÁ — the in-memory rating rules that stood here (a client-side reopen threshold, a reopen counter)
 * were removed on 29/09/2026: whether a petition may be rated, and what a low rating does, are the SERVER's
 * lifecycle rules (`isRateable`, `CitizenReportRating.tsx`), per commune (ADR 0008). A copy here would drift.
 */

/**
 * BỐN NHÓM NGƯỜI DÂN THẤY (ADR 0050 điểm 5) — bảng gộp và hai bảng nhãn nay nằm ở `status-groups.ts`, dùng
 * chung cho màn công dân của CẢ app chung lẫn app riêng (chủ dự án 28/09/2026). Ba tên cũ dưới đây giữ lại
 * cho các tệp đang nhập chúng; chúng trỏ về đúng một bảng, không phải một bản chép.
 */
export type FilterGroup = StatusFilter;

/**
 * Nhóm của app riêng — exactly `groupOf`, as in the shared app: an unknown code is `null`, NEVER a guessed
 * group. Falling back to "Đã đóng" told the citizen a ticket still open was closed; the screens now show the
 * shared neutral sentence (`statusLabel`) and the ticket appears only under "Tất cả", never under a group.
 */
export function communeAppGroupOf(status: string): StatusGroup | null {
  return groupOf(status);
}

export const COMMUNE_APP_GROUP_LABEL = STATUS_GROUP_LABEL;

export const COMMUNE_APP_STEP_LABEL = STEP_LABEL;

/**
 * Năm ô của hợp đồng thật (`ACCEPTED_FIELDS`) cộng lĩnh vực dân chọn (ADR 0050). `linh_vuc` là MÃ lĩnh
 * vực trong danh mục của xã (`CitizenField.code`) và đi lên thành `field` (`CommuneAppReports.tsx` `sendBody`);
 * nháp giữ đúng mã ấy và chỉ khôi phục khi xã còn mở nó. `an_danh` do công tắc "Gửi ẩn danh" của bà con
 * đặt (SRS M4.2, ADR 0050 #3).
 *
 * WHY THE SIX FIELD NAMES ARE STILL VIETNAMESE (ADR 0061, layer A): they are the stored keys of the draft
 * record `vigov.feedback.draft.v1` (`commune-app/feedback-draft-store.ts` writes this shape as it is).
 * Renaming them here would silently orphan every draft already on a phone; they move with the `v2` key bump
 * in layer C, together with a reader for the old keys.
 */
export type ReportDraft = {
  readonly linh_vuc: string;
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly ho_ten: string;
  readonly dien_thoai: string;
  readonly an_danh: boolean;
};

/**
 * DRAFT OF A FEEDBACK BEING WRITTEN — commune's own app ONLY (ADR 0050 #7; owner, 28/09/2026: "3 điểm còn
 * lại cũng theo require nhé", prototype `store/draft.ts`). The shell (`App.tsx`, `CommuneApp` only) injects
 * it, exactly like `GetZaloName`: this half never touches a storage API itself (`two-halves-boundary.test.ts`
 * §3b), and the shared ViHAT app never receives one, so its "không lưu gì xuống máy" promise still holds.
 *
 * `load` returns `null` for "no draft", a malformed one, or storage that is unavailable; `save`/`clear`
 * never throw. The draft never leaves the phone.
 */
export type FeedbackDraftStore = {
  readonly load: () => ReportDraft | null;
  readonly save: (draft: ReportDraft) => void;
  readonly clear: () => void;
};

export type DraftError =Partial<Record<"noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai", string>>;

const charCount = (s: string): number => [...s].length;

/**
 * Kiểm lúc bấm gửi, cùng giới hạn máy chủ (`MAX_LENGTH`). THUẦN.
 *
 * Bắt buộc theo SRS M4.2 (chủ dự án 28/09/2026: "theo require"): mô tả, và NGƯỜI GỬI khi không ẩn danh —
 * họ tên; gửi ẩn danh thì không. Lĩnh vực bắt buộc do bước 1 (không qua được nếu chưa chọn). Ảnh/video và
 * vị trí trên bản đồ cũng bắt buộc theo SRS, nhưng ứng dụng CHƯA có hai thứ ấy — chặn nút gửi vì chúng thì
 * không gửi được phiếu nào (máy chủ cũng để chúng tuỳ chọn); màn hình nói rõ là "bắt buộc — sắp có" (ADR 0050).
 */
export function checkDraft(
  draft: ReportDraft,
  text: { missing: string; missing_reporter: string; too_long: (max: number) => string },
): DraftError {
  const error: DraftError = {};
  if (draft.noi_dung.trim() === "") error.noi_dung = text.missing;
  else if (charCount(draft.noi_dung.trim()) > MAX_LENGTH.content) error.noi_dung = text.too_long(MAX_LENGTH.content);
  if (charCount(draft.dia_chi.trim()) > MAX_LENGTH.address) error.dia_chi = text.too_long(MAX_LENGTH.address);
  if (!draft.an_danh && draft.ho_ten.trim() === "") error.ho_ten = text.missing_reporter;
  else if (!draft.an_danh && charCount(draft.ho_ten.trim()) > MAX_LENGTH.full_name) error.ho_ten = text.too_long(MAX_LENGTH.full_name);
  if (!draft.an_danh && charCount(draft.dien_thoai.trim()) > MAX_LENGTH.phone) {
    error.dien_thoai = text.too_long(MAX_LENGTH.phone);
  }
  return error;
}

