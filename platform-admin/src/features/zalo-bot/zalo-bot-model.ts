import type { ApiError, SharedZaloBotChange, ZaloBotCommuneStats, ZaloBotOutcome } from "@/lib/api";

/**
 * The pure half of the Zalo Bot screen (ADR 0074): the sentence for each Zalo call outcome, the
 * client-side check of the token form, the relink confirmation. No React, no fetch — every branch is
 * tested without rendering.
 *
 * ZALO IS NEVER QUOTED. The server sends an outcome CLASS only (`ZaloBotCallOutcome`); each sentence
 * below says what happened and what the operator does next, in the console's own words.
 */

const OUTCOME_TEXT: Record<ZaloBotOutcome, string> = {
  "thanh-cong": "Zalo đã chấp nhận yêu cầu.",
  "chua-cau-hinh": "Chưa đặt token cho bot dùng chung nên hệ thống chưa gọi Zalo. Đặt token trước.",
  "token-bi-tu-choi":
    "Zalo từ chối token của bot (token sai, đã bị thu hồi, hoặc bot không còn). Lấy token mới trên trang quản lý bot của Zalo rồi đặt lại.",
  "gioi-han-tan-suat": "Zalo đang giới hạn số lần gọi. Vui lòng thử lại sau ít phút.",
  "khong-kha-dung": "Không nhận được trả lời từ Zalo (quá thời gian chờ hoặc Zalo đang lỗi). Vui lòng thử lại sau ít phút.",
  "bi-tu-choi":
    "Zalo từ chối yêu cầu. Kiểm tra cấu hình bot trên trang quản lý của Zalo; nếu vẫn bị từ chối, báo bộ phận kỹ thuật.",
  "phan-hoi-sai-dang": "Zalo trả lời theo dạng hệ thống không đọc được. Báo bộ phận kỹ thuật kèm thời điểm thử.",
};

/** A class this build does not know: a failure of unknown cause, never a success (proto, UNSPECIFIED). */
export const UNKNOWN_OUTCOME = "Không rõ kết quả lần gọi Zalo. Báo bộ phận kỹ thuật kèm thời điểm thử.";

export function outcomeText(outcome: string): string {
  // Own keys only: `"toString" in OUTCOME_TEXT` is true, and would print a function as a sentence.
  return Object.prototype.hasOwnProperty.call(OUTCOME_TEXT, outcome) ? OUTCOME_TEXT[outcome as ZaloBotOutcome] : UNKNOWN_OUTCOME;
}

export function isOk(outcome: string): boolean {
  return outcome === "thanh-cong";
}

/**
 * After PUT …/webhook. `khong-kha-dung` and `phan-hoi-sai-dang` are AMBIGUOUS: Zalo may already be
 * posting with the new secret (comms keeps it accepted), so the operator is told to look, not that it failed.
 */
export function webhookSetText(outcome: string): string {
  if (isOk(outcome)) return "Đã trỏ webhook của bot về hệ thống. Từ giờ tin nhắn cán bộ gửi bot được chuyển về hệ thống.";
  if (outcome === "khong-kha-dung" || outcome === "phan-hoi-sai-dang") {
    return `${outcomeText(outcome)} Chưa rõ Zalo đã nhận địa chỉ mới hay chưa: bấm “Kiểm tra webhook” để xem, hoặc trỏ lại.`;
  }
  return outcomeText(outcome);
}

/** The owner's sentence (05/10/2026), with the count the server sent. */
export function relinkQuestion(count: number): string {
  return `Đổi sang tài khoản bot khác: ${count} cán bộ sẽ phải ghép nối lại. Tiếp tục?`;
}

/** After a save: how many old-bot links the server ended in the same transaction, when any. */
export function endedLinksText(count: number | undefined): string | null {
  return count !== undefined && count > 0 ? `Đã gỡ ${count} liên kết của cán bộ với tài khoản bot cũ.` : null;
}

/** A 409 `can-xac-nhan-ghep-lai` without a readable count: no confirmation box with an invented figure. */
export const RELINK_COUNT_MISSING =
  "Đổi sang tài khoản bot khác cần xác nhận số cán bộ phải ghép nối lại, nhưng máy chủ không gửi con số ấy. Chưa có gì được thay đổi; báo bộ phận kỹ thuật.";

/**
 * What a refused PUT /zalo-bots/shared asks of the operator: `confirm` the count the server sent,
 * `missing` when it is a relink refusal without a readable count, `none` for any other refusal.
 */
export function relinkStep(err: ApiError): { kind: "confirm"; count: number } | { kind: "missing" } | { kind: "none" } {
  if (err.status !== 409 || err.code !== "can-xac-nhan-ghep-lai") return { kind: "none" };
  return err.relinkCount === null ? { kind: "missing" } : { kind: "confirm", count: err.relinkCount };
}

// --- the token form -------------------------------------------------------------------------------

export type BotField = "token" | "botName" | "chatUrl" | "form";

export type BotDraft = { token: string; botName: string; chatUrl: string };

export const TOKEN_HINT =
  "Sao chép từ trang quản lý bot của Zalo. Token chỉ được ghi, không hiện lại sau khi lưu — kể cả với chính bạn.";
export const BOT_NAME_HINT = "Tên cán bộ thấy ở trang “Nhận nhắc việc qua Zalo”. Tối đa 100 ký tự.";
export const CHAT_URL_HINT = "Liên kết mở cuộc trò chuyện với bot, bắt đầu bằng https://. Cán bộ bấm vào đây để ghép nối.";

/** Token rules of `SetSharedZaloBotRequest.token`: 1–256 printable ASCII, no space, none of `/ ? # %`. */
const TOKEN_SHAPE = /^[!-~]{1,256}$/;
const TOKEN_FORBIDDEN = /[/?#%]/;

/**
 * The body to send, or the first field that is wrong. A HINT ONLY — comms re-checks every rule and
 * its refusal still reaches the form. The token is taken VERBATIM (not trimmed): it is spliced into a
 * URL path and a trimmed copy would be a different token.
 */
export function buildBotChange(draft: BotDraft): { ok: true; change: SharedZaloBotChange } | { ok: false; field: BotField; text: string } {
  if (draft.token === "") return { ok: false, field: "token", text: "Hãy dán token của bot." };
  if (!TOKEN_SHAPE.test(draft.token) || TOKEN_FORBIDDEN.test(draft.token)) {
    return {
      ok: false,
      field: "token",
      text: "Token không đúng dạng: không có khoảng trắng, không chứa / ? # %, tối đa 256 ký tự. Sao chép lại từ trang quản lý bot của Zalo.",
    };
  }
  const botName = draft.botName.trim();
  if (botName === "" || [...botName].length > 100) {
    return { ok: false, field: "botName", text: "Hãy nhập tên bot, tối đa 100 ký tự." };
  }
  const chatUrl = draft.chatUrl.trim();
  if (!isHttpsUrl(chatUrl) || chatUrl.length > 300) {
    return { ok: false, field: "chatUrl", text: "Liên kết trò chuyện phải là địa chỉ đầy đủ bắt đầu bằng https://, tối đa 300 ký tự." };
  }
  return { ok: true, change: { token: draft.token, bot_name: botName, chat_url: chatUrl } };
}

/**
 * The chat link as an `href`, or null. Re-checked here although comms validates it on write: an
 * `href` built from stored text is where a `javascript:` URL would run, and the check costs one line.
 */
export function safeHttpsHref(v: string | undefined): string | null {
  return v !== undefined && isHttpsUrl(v) ? v : null;
}

function isHttpsUrl(v: string): boolean {
  try {
    const u = new URL(v);
    return u.protocol === "https:" && u.username === "" && u.password === "";
  } catch {
    return false;
  }
}

/**
 * Field refusals of the `zalo-bots/shared…` routes, by `code`. ASSUMED CODES — the platform routes
 * were built in parallel with this screen and only `can-xac-nhan-ghep-lai` / `token-khong-dung-duoc`
 * were confirmed; an unknown code falls back to the generic sentence (`handleGuardedError`), never to
 * the server's text.
 */
const ZALO_BOT_ERRORS: Record<string, { field: BotField; text: string }> = {
  invalid_token: {
    field: "token",
    text: "Token không đúng dạng: không có khoảng trắng, không chứa / ? # %, tối đa 256 ký tự. Sao chép lại từ trang quản lý bot của Zalo.",
  },
  invalid_bot_name: { field: "botName", text: "Hãy nhập tên bot, tối đa 100 ký tự." },
  invalid_chat_url: {
    field: "chatUrl",
    text: "Liên kết trò chuyện phải là địa chỉ đầy đủ bắt đầu bằng https://, tối đa 300 ký tự.",
  },
};

export function zaloBotError(err: ApiError): { field: BotField; text: string } | null {
  // 422 `token-khong-dung-duoc`: comms asked Zalo with the new token before keeping it, and Zalo's
  // answer was not OK. Nothing was saved; the sentence is the outcome's, under the token box.
  if (err.code === "token-khong-dung-duoc") {
    return { field: "token", text: `Chưa lưu token. ${outcomeText(err.zaloOutcome ?? "")}` };
  }
  return ZALO_BOT_ERRORS[err.code] ?? null;
}

// --- the communes table ---------------------------------------------------------------------------

/** Name the platform joined, else the ULID — a commune the operator can still look up, never blank. */
export function communeLabel(row: ZaloBotCommuneStats): string {
  return row.commune_name !== undefined && row.commune_name.trim() !== "" ? row.commune_name : row.tenant_id;
}

export function communeTotals(rows: readonly ZaloBotCommuneStats[]): { enabled: number; linked: number } {
  return rows.reduce((t, r) => ({ enabled: t.enabled + (r.channel_enabled ? 1 : 0), linked: t.linked + r.linked_staff_count }), {
    enabled: 0,
    linked: 0,
  });
}
