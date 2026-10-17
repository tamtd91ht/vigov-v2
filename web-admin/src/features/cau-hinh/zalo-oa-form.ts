/**
 * "OA Zalo của xã" and "Mẫu tin ZBS" in Cấu hình → Kênh Zalo (ADR 0098 khối A; several OAs per commune
 * since ADR 0099 #1) — the pure half: the
 * words, the milestone table, the outcome sentences, the row checks. Every check here is a HINT: comms
 * re-checks each rule (one template one milestone, ENABLE only, the three parameter names) and its
 * sentence still reaches the screen.
 *
 * NO PROTOTYPE FOR THIS PART (ADR 0098 §Xung đột): built on the shape of "Con bot của xã" and ADR 0068 §14.
 */

import type { comms_communeZaloOAOut, comms_zaloOATemplateOut, ZaloOAReauthorizationReason } from "@/lib/api/zalo-oa";

// ---- "OA Zalo của xã" -----------------------------------------------------------------------------

export const OA_DESCRIPTION =
  "OA của xã là người gửi tin ZBS/ZNS cho người dân. Xã có thể nối nhiều OA; mỗi OA phải được uỷ quyền cho ứng dụng của nền tảng thì xã mới gửi được tin từ OA ấy.";

/** The section's button: the first OA, or one more beside those already connected (ADR 0099 #1). */
export const CONNECT_FIRST_LABEL = "Kết nối OA";
export const CONNECT_MORE_LABEL = "Kết nối thêm OA";
export const REAUTHORIZE_LABEL = "Uỷ quyền lại";

/** GET says `configured: false`: the platform's ZALO_OA_* variables are not set (ADR 0098 #2). */
export const OA_NOT_CONFIGURED = "Nền tảng chưa khai ứng dụng Zalo dùng chung — liên hệ ViHAT.";

/** Under "Kết nối OA" / "Uỷ quyền lại": what happens in the tab that opens. 10 minutes = Zalo's authorization code. */
export const AUTHORIZE_INSTRUCTION =
  "Đăng nhập Zalo bằng tài khoản Quản trị viên của OA xã, chọn đúng OA rồi bấm Cho phép. Liên kết dùng một lần, hết hạn sau 10 phút.";

/**
 * After the link is opened. `window.open(…, "noopener")` always returns null, so a blocked pop-up cannot
 * be told from an opened one: the sentence says what to do in both cases rather than claim success.
 */
export const LINK_OPENED_TOAST =
  "Đã mở trang uỷ quyền của Zalo ở tab mới. Nếu không thấy tab mới, hãy cho phép cửa sổ bật lên cho trang này rồi bấm lại.";

/** "Kiểm tra kết nối" when the call itself failed unexplained — the bot's sentence (`zalo-channel-form.ts`). */
export const OA_CHECK_FAILED_TOAST = "Không kiểm tra được.";

export const DISCONNECT_REASON_MAX = 200;

/**
 * The consequence named in the disconnect confirmation (ADR 0098 #11, ADR 0099 #1): only the milestones
 * attached to a template of THIS OA are detached; the commune's other OAs keep theirs.
 */
export const DISCONNECT_CONSEQUENCE =
  "Mọi mốc đã gắn mẫu của OA này sẽ bị gỡ, và tin cho người dân theo các mốc ấy sẽ dừng tới khi xã gắn mẫu khác. Các OA khác của xã giữ nguyên.";

export const DISCONNECTED_TOAST = "Đã ngắt OA.";

/** Pill next to the section title: how many OAs are connected. */
export function oaListPill(items: readonly unknown[]): string {
  return items.length === 0 ? "Chưa nối" : `Đã nối ${items.length} OA`;
}

/** Pill on one OA's card. */
export function oaPill(oa: Pick<comms_communeZaloOAOut, "needs_reauthorization">): string {
  return oa.needs_reauthorization !== null ? "Cần uỷ quyền lại" : "Đã nối";
}

/** "{n} mẫu đã nạp · {m} mốc đã gắn" — comms' own counts for that OA, never computed here. */
export function templateCountText(oa: Pick<comms_communeZaloOAOut, "template_count" | "attached_count">): string {
  return `${oa.template_count} mẫu đã nạp · ${oa.attached_count} mốc đã gắn`;
}

/** "Nối lúc {at} · bởi {name} ({code})" — `at` already formatted by the caller. */
export function authorizedText(at: string, by: string, byName: string): string {
  return `Nối lúc ${at} · bởi ${who(by, byName)}`;
}

/** "Gắn lúc {at} · bởi …" under a template's milestone select. */
export function attachedText(at: string, by: string, byName: string | null): string {
  return `Gắn lúc ${at} · bởi ${who(by, byName ?? "")}`;
}

function who(code: string, name: string): string {
  return name.trim() !== "" ? `${name} (${code})` : code;
}

/** Why the OA must be authorised again, in the commune's words (ADR 0098 #6). */
export function reauthorizationText(reason: ZaloOAReauthorizationReason | string): string {
  switch (reason) {
    case "refresh-token-bi-tu-choi":
      return "Zalo không còn nhận uỷ quyền của xã — quyền của ứng dụng có thể đã bị thu hồi trong OA Manager. Bấm “Uỷ quyền lại”.";
    case "refresh-token-het-han":
      return "Uỷ quyền đã quá 3 tháng không được làm mới nên Zalo không còn nhận. Bấm “Uỷ quyền lại”.";
    default:
      return "Zalo không còn nhận uỷ quyền của xã. Bấm “Uỷ quyền lại”.";
  }
}

/**
 * The sentence for each Zalo call outcome comms records on the OA row (the bot's seven classes,
 * `service-comms/internal/domain/zalo_bot.go` ZaloCall*). ZALO IS NEVER QUOTED. The words are the OA's,
 * not the bot's: there is no "mã bot" here, the remedy is to authorise again.
 */
const OA_OUTCOME_TEXT: Readonly<Record<string, { long: string; short: string }>> = {
  "thanh-cong": { long: "Kết nối tốt: Zalo nhận uỷ quyền OA của xã.", short: "kết nối tốt" },
  "chua-cau-hinh": { long: "Xã chưa nối OA nên hệ thống chưa hỏi Zalo.", short: "chưa nối OA" },
  "token-bi-tu-choi": {
    long: "Zalo không nhận uỷ quyền của xã (đã bị thu hồi hoặc hết hạn). Bấm “Uỷ quyền lại”.",
    short: "Zalo không nhận uỷ quyền",
  },
  "gioi-han-tan-suat": {
    long: "Zalo đang giới hạn số lần gọi. Vui lòng thử lại sau ít phút.",
    short: "Zalo đang giới hạn số lần gọi",
  },
  "khong-kha-dung": {
    long: "Không nhận được trả lời từ Zalo. Vui lòng thử lại sau ít phút.",
    short: "Zalo không trả lời",
  },
  "bi-tu-choi": {
    long: "Zalo từ chối yêu cầu. Kiểm tra quyền của ứng dụng trong OA Manager của xã.",
    short: "Zalo từ chối yêu cầu",
  },
  "phan-hoi-sai-dang": {
    long: "Zalo trả lời theo dạng hệ thống không đọc được. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
    short: "Zalo trả lời sai dạng",
  },
};

/** A class this build does not know is a failure of unknown cause — never a success. */
const UNKNOWN_OUTCOME = {
  long: "Không rõ kết quả lần hỏi Zalo. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
  short: "không rõ kết quả",
};

function oaOutcome(result: string): { long: string; short: string } {
  // Own keys only: `"toString" in OA_OUTCOME_TEXT` is true and would print a function as a sentence.
  return (Object.prototype.hasOwnProperty.call(OA_OUTCOME_TEXT, result) ? OA_OUTCOME_TEXT[result] : undefined) ?? UNKNOWN_OUTCOME;
}

export const isOaOutcomeOk = (result: string): boolean => result === "thanh-cong";

/** "Kiểm tra kết nối" toast. `oaName` is the OA's name as Zalo knows it — an authority's name, not personal data. */
export function oaCheckResultText(result: string, oaName?: string): string {
  if (isOaOutcomeOk(result) && oaName !== undefined && oaName.trim() !== "") {
    return `Kết nối tốt: Zalo nhận uỷ quyền OA của xã (${oaName}).`;
  }
  return oaOutcome(result).long;
}

/** "Lần kiểm gần nhất: {lúc} · {kết quả}" — `at` already formatted by the caller. */
export function oaLastCheckText(at: string, result: string): string {
  return `Lần kiểm gần nhất: ${at} · ${oaOutcome(result).short}`;
}

/** The disconnect reason, trimmed, or why not (comms: required, at most 200 characters). */
export function checkDisconnectReason(raw: string): { ok: true; reason: string } | { ok: false; text: string } {
  const reason = raw.trim();
  if (reason === "" || [...reason].length > DISCONNECT_REASON_MAX) {
    return { ok: false, text: `Cần nhập lý do ngắt OA (tối đa ${DISCONNECT_REASON_MAX} ký tự).` };
  }
  return { ok: true, reason };
}

// ---- the callback's outcome, read ONCE from `?oa=` ----------------------------------------------------

/** The query parameter comms' callback appends when it sends the browser back to this tab (ADR 0098 #4). */
export const OA_OUTCOME_PARAM = "oa";

/**
 * The sentence for each outcome comms can send back. `da-noi` is the only success. `oa-dang-o-xa-khac`
 * NAMES NO COMMUNE, on purpose: which commune holds that OA is that commune's business (rule 1).
 */
const CALLBACK_OUTCOME_TEXT: Readonly<Record<string, { ok: boolean; text: string }>> = {
  "da-noi": { ok: true, text: "Đã nối OA của xã. Tiếp theo: nạp mẫu từ OA rồi gắn mẫu cho từng mốc." },
  "zalo-tu-choi": { ok: false, text: "Zalo từ chối uỷ quyền, hoặc Quản trị viên OA đã bấm Từ chối. Chưa có gì được nối." },
  "khong-kha-dung": { ok: false, text: "Không nhận được trả lời từ Zalo khi đổi mã uỷ quyền. Vui lòng bấm “Kết nối OA” lần nữa." },
  "phan-hoi-sai-dang": {
    ok: false,
    text: "Zalo trả lời theo dạng hệ thống không đọc được. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
  },
  "het-han": { ok: false, text: "Liên kết uỷ quyền đã hết hạn (10 phút) hoặc đã dùng rồi. Bấm “Kết nối OA” để lấy liên kết mới." },
  "oa-dang-o-xa-khac": { ok: false, text: "OA này đang được một đơn vị khác dùng. Hãy chọn OA của chính xã." },
  "sai-khuon": { ok: false, text: "Zalo gửi về một yêu cầu không đúng khuôn. Bấm “Kết nối OA” để thử lại." },
};

/** An outcome this build does not know is a failure, never a success. */
const UNKNOWN_CALLBACK_OUTCOME = { ok: false, text: "Chưa rõ kết quả uỷ quyền. Hãy xem trạng thái OA bên dưới rồi thử lại nếu cần." };

export function callbackOutcomeText(outcome: string): { ok: boolean; text: string } {
  return (
    (Object.prototype.hasOwnProperty.call(CALLBACK_OUTCOME_TEXT, outcome) ? CALLBACK_OUTCOME_TEXT[outcome] : undefined) ??
    UNKNOWN_CALLBACK_OUTCOME
  );
}

/** The `oa=` value of a `location.search`, or null when absent or empty. */
export function outcomeFromSearch(search: string): string | null {
  const v = new URLSearchParams(search).get(OA_OUTCOME_PARAM);
  return v === null || v === "" ? null : v;
}

/** The same search string without `oa=` — "" when nothing is left, else "?…" (`?tab=kenh-zalo` stays). */
export function searchWithoutOutcome(search: string): string {
  const params = new URLSearchParams(search);
  params.delete(OA_OUTCOME_PARAM);
  const rest = params.toString();
  return rest === "" ? "" : `?${rest}`;
}

// ---- "Mẫu tin ZBS" -----------------------------------------------------------------------------------

export const TEMPLATES_DESCRIPTION =
  "Mẫu tin do xã đăng ký và được Zalo duyệt trên từng OA. Nạp mẫu về rồi gắn mỗi mốc xử lý phản ánh với một mẫu — một mốc chỉ một mẫu trong cả xã, dù xã có nhiều OA; tin cho người dân đi từ OA của mẫu ấy.";

/** The OA picker above the templates table, drawn only when the commune has more than one OA. */
export const PICK_OA_LABEL = "OA";

/** ADR 0098 #9: no mapping table — the template's parameter NAMES must be exactly these. */
export const REQUIRED_PARAMS: readonly string[] = ["ma_tra_cuu", "moc_nhan", "viec_tiep_theo"];

export const PARAMS_RULE = `Mẫu đăng ký với Zalo phải có đúng ba tham số: ${REQUIRED_PARAMS.join(", ")}.`;

export const NO_TEMPLATES = "Chưa nạp mẫu nào từ OA. Bấm “Nạp mẫu từ OA” sau khi mẫu được Zalo duyệt.";

export const NO_MILESTONE_OPTION = "— Không gắn —";

/**
 * The eight milestones a template may be attached to (ADR 0098 #8): the six target statuses of ADR 0041's
 * notification table, then merge / split (ADR 0041 §Sửa đổi 09/10/2026). Codes are what comms stores; the
 * order is the order shown.
 */
export const MILESTONES: readonly { code: string; label: string }[] = [
  { code: "da-tiep-nhan", label: "Đã tiếp nhận" },
  { code: "da-chuyen-xu-ly", label: "Đã chuyển xử lý" },
  { code: "cho-dan-xac-nhan", label: "Chờ dân xác nhận" },
  { code: "da-dong", label: "Đã đóng" },
  { code: "khong-tiep-nhan", label: "Không tiếp nhận" },
  { code: "chuyen-cap-tren", label: "Chuyển cấp trên" },
  { code: "gop-phieu", label: "Ghép với phản ánh cùng vụ việc" },
  { code: "tach-phieu", label: "Xử lý riêng" },
];

/** The label of a milestone code; a code this build does not know is shown as is, never hidden. */
export function milestoneLabel(code: string): string {
  return MILESTONES.find((m) => m.code === code)?.label ?? code;
}

/** Zalo's template status, in the officer's words. An unknown word is shown as Zalo sent it. */
export function zaloStatusLabel(status: string): string {
  switch (status) {
    case "ENABLE":
      return "Đang bật";
    case "PENDING_REVIEW":
      return "Chờ duyệt";
    case "REJECT":
      return "Bị từ chối";
    case "DISABLE":
      return "Đã tắt";
    case "DELETE":
      return "Đã xoá";
    default:
      return status;
  }
}

/** Zalo's template quality, in the officer's words. */
export function zaloQualityLabel(quality: string): string {
  switch (quality) {
    case "HIGH":
      return "Cao";
    case "MEDIUM":
      return "Trung bình";
    case "LOW":
      return "Thấp";
    case "UNDEFINED":
      return "Chưa đánh giá";
    default:
      return quality;
  }
}

/** "Thiếu: a, b" for a template whose parameters do not match; null when they do. */
export function missingText(t: Pick<comms_zaloOATemplateOut, "params_match" | "missing_params">): string | null {
  if (t.params_match) return null;
  return t.missing_params.length > 0 ? `Thiếu: ${t.missing_params.join(", ")}` : "Tham số chưa khớp";
}

/** ADR 0098 #8–#9: only an ENABLE template whose parameters match may be attached. */
export function canAttach(t: Pick<comms_zaloOATemplateOut, "zalo_status" | "params_match">): boolean {
  return t.zalo_status === "ENABLE" && t.params_match;
}

/** Why the milestone select is disabled — the same causes comms answers 422 for, said before the click. */
export function attachBlockedReason(t: Pick<comms_zaloOATemplateOut, "zalo_status" | "params_match" | "missing_params">): string | null {
  if (t.zalo_status !== "ENABLE") return `Mẫu đang ở trạng thái “${zaloStatusLabel(t.zalo_status)}” trên Zalo nên chưa gắn mốc được.`;
  if (!t.params_match) return `${missingText(t)} — ${PARAMS_RULE}`;
  return null;
}

/** Milestones no template is attached to, in display order. Pass the templates of EVERY OA: one milestone, one template in the whole commune. */
export function unattachedMilestones(items: readonly Pick<comms_zaloOATemplateOut, "milestone">[]): string[] {
  const attached = new Set(items.map((t) => t.milestone).filter((m): m is string => m !== null));
  return MILESTONES.filter((m) => !attached.has(m.code)).map((m) => m.code);
}

/** Templates still attached to a milestone but no longer usable (not ENABLE, or parameters drifted after a sync). */
export function staleAttachments<T extends Pick<comms_zaloOATemplateOut, "milestone" | "zalo_status" | "params_match">>(
  items: readonly T[],
): T[] {
  return items.filter((t) => t.milestone !== null && !canAttach(t));
}

/** "Chưa có mẫu cho: …" */
export function unattachedText(codes: readonly string[]): string {
  return `Chưa có mẫu cho: ${codes.map(milestoneLabel).join(", ")}.`;
}

/**
 * "Mẫu đã gắn cho mốc X (OA “Y”) không còn dùng được — …" (ADR 0098 #10: that milestone's notices will not
 * go out). The OA is named: with several OAs the template may sit on one not shown in the table.
 */
export function staleText(milestone: string, oaName: string): string {
  return `Mẫu đã gắn cho mốc “${milestoneLabel(milestone)}” (OA “${oaName}”) không còn dùng được — tin cho mốc này sẽ không gửi được.`;
}

/** The "Nạp mẫu từ OA" toast: the server's three counts, never estimated here. */
export function syncResultText(r: { loaded: number; enabled: number; matching: number }): string {
  return `Đã nạp ${r.loaded} mẫu, ${r.enabled} đang bật, ${r.matching} khớp tham số.`;
}

/** "Nạp lúc …" under the button; null when never synced. */
export function syncedText(at: string | null): string | null {
  return at === null ? null : `Nạp lúc ${at}`;
}
