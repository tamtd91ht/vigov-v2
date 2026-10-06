/**
 * ONE-TAP REQUESTS — "Chat với chuyên viên" (Liên hệ) and "Nhận ưu đãi qua SMS" (Trang chủ), owner decision
 * 07/10/2026: a permission is asked at the moment the citizen taps, the request is fire-and-forget, and nobody
 * creates an account or waits on a status.
 *
 * WHY THE FLOWS LIVE HERE AND NOT IN THE COMPONENTS: the test suite renders with `react-dom/server` and has no
 * DOM to tap. Every branch a citizen meets — above all the REFUSALS, the most common ones — is reachable only
 * if the flow is a plain async function with its steps passed in. The components are thin over these.
 *
 * ⚠ IDENTITY COMES FROM THE SESSION, NEVER FROM THE BODY (rule 4, invariant 2). The body is `thanYeuCau`'s,
 *   which carries no phone and no user id; the server knows the requester from the bearer. The only personal
 *   value a body here may carry is the Zalo display name, on `chat` only.
 *
 * ⚠ NOTHING IS STORED ON THE DEVICE AND NOTHING IS LOGGED. The name lives for one call; the session goes to
 *   `kho-phien.tsx` (memory); the subscription state is derived from the server's list, in memory.
 */
import { guiYeuCau, type KetQuaGui } from "../../api/goi-may-chu"; // vi-name-ok: existing exports
import type { LoaiYeuCau, YeuCauDaGui, YeuCauMoi } from "../../api/hop-dong-yeu-cau"; // vi-name-ok: existing types
import { KHOA_CHIEN_DICH, maChienDichHopLe } from "../../content/chien-dich"; // vi-name-ok: existing exports
import { DUONG_DAN_CHAT_OA } from "../../content/dich-ra-ngoai"; // vi-name-ok: existing export
import { thamSo, thamSoMoApp } from "../../lib/launch-params"; // vi-name-ok: existing exports
import { type SessionAttempt, ensureSession } from "../dang-nhap/ensure-session";
import type { Phien } from "../dang-nhap/hop-dong"; // vi-name-ok: existing type
import { moRaNgoai } from "../tinh-nang/mo-ra-ngoai"; // vi-name-ok: existing export
import { type KetQuaXin, layTenZalo } from "../tinh-nang/zalo-api"; // vi-name-ok: existing exports

/** Every step a one-tap request takes, injectable. `LIVE_STEPS` are the real ones. */
export type QuickRequestSteps = {
  ensureSession: (current: Phien | null) => Promise<SessionAttempt>; // vi-name-ok: existing type
  requestName: () => Promise<KetQuaXin<string>>; // vi-name-ok: existing type
  send: (bearer: string, request: YeuCauMoi) => Promise<KetQuaGui>; // vi-name-ok: existing types
  openChat: () => Promise<boolean>;
};

export const LIVE_STEPS: QuickRequestSteps = {
  ensureSession: (current) => ensureSession(current),
  // `true` = Zalo's own dialog: the block has already said, next to the button, why the name is wanted.
  requestName: () => layTenZalo(true),
  send: (bearer, request) => guiYeuCau(bearer, request), // vi-name-ok: existing export
  // The SAME door the floating "Chat Zalo" button uses — a declared destination, no new platform call.
  openChat: () => moRaNgoai("chat-oa", DUONG_DAN_CHAT_OA), // vi-name-ok: existing export
};

/**
 * The campaign code of the link that opened the app, or "" — read exactly as "Tư vấn và báo giá" reads it:
 * only a code that matches a real row of `content/chien-dich.ts` is sent, so an outsider's string never
 * reaches the server's database and never turns the citizen's tap into a 400.
 */
export function campaignSource(): string {
  return maChienDichHopLe(thamSo(thamSoMoApp())[KHOA_CHIEN_DICH] ?? ""); // vi-name-ok: existing exports
}

/** The request body for a one-tap kind: no interests, no scale, no note — only what the kind itself says. */
export function quickRequest(kind: LoaiYeuCau, source: string, displayName = ""): YeuCauMoi { // vi-name-ok: existing types
  return { loai: kind, quan_tam: [], quy_mo: "", ghi_chu: "", nguon: source, display_name: displayName };
}

/**
 * What the session holder is told after a send. A 401 means the bearer died between taps: the session is
 * forgotten (`null`) so the next tap logs in again, rather than failing the same way forever.
 */
function sessionAfter(sent: KetQuaGui, session: Phien): Phien | null { // vi-name-ok: existing types
  return sent.kieu === "chua-dang-nhap" ? null : session;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * CHAT WITH A SPECIALIST
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 *   `refused`       phone sharing refused — nothing sent, chat NOT opened (the floating button stays for that)
 *   `outside-zalo`  no Zalo call works, the chat window included
 *   `opened`        the chat window opened; `delivered` says whether the specialist got the name and number
 *   `not-opened`    the platform did not open the window; `delivered` as above
 */
export type ChatOutcome =
  | { kind: "refused" }
  | { kind: "outside-zalo" }
  | { kind: "opened"; delivered: boolean }
  | { kind: "not-opened"; delivered: boolean };

/**
 * Session → Zalo name → POST `chat` → open the OA chat.
 *
 * THE CHAT OPENS EVEN WHEN THE POST FAILS (owner, 07/10/2026): the conversation is what the citizen came for;
 * the record only helps the specialist recognise them. Same when the session could not be issued for a reason
 * other than a refusal. A refusal is different: the citizen said no to being identified, so the button does
 * not turn into "open the chat anyway" — the floating button, which identifies nobody, is pointed to instead.
 *
 * A REFUSED NAME IS NOT A FAILURE: the request goes without one.
 */
export async function chatWithSpecialist(
  current: Phien | null, // vi-name-ok: existing type
  source: string,
  onSession: (session: Phien | null) => void, // vi-name-ok: existing type
  steps: QuickRequestSteps = LIVE_STEPS,
): Promise<ChatOutcome> {
  const attempt = await steps.ensureSession(current);
  if (attempt.kind === "refused") return { kind: "refused" };
  if (attempt.kind === "outside-zalo") return { kind: "outside-zalo" };

  let delivered = false;
  if (attempt.kind === "ready") {
    if (attempt.issued) onSession(attempt.session);
    const name = await steps.requestName();
    const displayName = name.kieu === "xong" ? name.du_lieu : "";
    const sent = await steps.send(attempt.session.token, quickRequest("chat", source, displayName));
    delivered = sent.kieu === "xong";
    if (sessionAfter(sent, attempt.session) === null) onSession(null);
  }

  return (await steps.openChat()) ? { kind: "opened", delivered } : { kind: "not-opened", delivered };
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * PROMOTIONAL SMS — subscribe and unsubscribe
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type SmsKind = "sms_promo" | "sms_optout";

/**
 *   `refused` · `outside-zalo` · `session-failed`   no session, nothing sent
 *   `sent`                                           the server's answer, every branch of `KetQuaGui`
 */
export type SmsOutcome =
  | { kind: "refused" }
  | { kind: "outside-zalo" }
  | { kind: "session-failed" }
  | { kind: "sent"; result: KetQuaGui }; // vi-name-ok: existing type

/** Session → POST `sms_promo` / `sms_optout`. No name is asked: the SMS goes to the session's own number. */
export async function sendSmsChoice(
  kind: SmsKind,
  current: Phien | null, // vi-name-ok: existing type
  source: string,
  onSession: (session: Phien | null) => void, // vi-name-ok: existing type
  steps: QuickRequestSteps = LIVE_STEPS,
): Promise<SmsOutcome> {
  const attempt = await steps.ensureSession(current);
  if (attempt.kind === "refused") return { kind: "refused" };
  if (attempt.kind === "outside-zalo") return { kind: "outside-zalo" };
  if (attempt.kind === "failed") return { kind: "session-failed" };

  if (attempt.issued) onSession(attempt.session);
  const result = await steps.send(attempt.session.token, quickRequest(kind, source));
  if (sessionAfter(result, attempt.session) === null) onSession(null);
  return { kind: "sent", result };
}

/**
 * Is this citizen subscribed? The LATEST of their `sms_promo` / `sms_optout` requests decides, by `createdAt`.
 *
 *   `subscribed` / `not-subscribed`   the list said so (no SMS row at all = never subscribed)
 *
 * Rows whose time cannot be read are skipped — a guess about which came last is a guess about consent. On an
 * exact tie the opt-out wins: of two readings of the same instant, "stop" is the one that cannot send an
 * unwanted message.
 */
export function deriveSmsSubscription(items: readonly YeuCauDaGui[]): "subscribed" | "not-subscribed" { // vi-name-ok: existing type
  let latest: { at: number; kind: SmsKind } | null = null;
  for (const item of items) {
    if (item.loai !== "sms_promo" && item.loai !== "sms_optout") continue;
    const at = Date.parse(item.tao_luc);
    if (Number.isNaN(at)) continue;
    if (latest === null || at > latest.at || (at === latest.at && item.loai === "sms_optout")) {
      latest = { at, kind: item.loai };
    }
  }
  return latest?.kind === "sms_promo" ? "subscribed" : "not-subscribed";
}
