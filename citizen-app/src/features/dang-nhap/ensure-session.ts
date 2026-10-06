/**
 * "HAVE A SESSION BY THE END OF THIS TAP" — the one-tap login of `KhoiDangNhap`, reused by a button that is
 * not the login button (07/10/2026: "Chat với chuyên viên", "Nhận ưu đãi qua SMS").
 *
 * NOT A SECOND LOGIN. The two steps are the existing ones, called in the existing order: `xinMaDangNhap`
 * (`getAccessToken` then `getPhoneNumber`, ADR 0020 — no OTP, never) and `phatHanhPhien` (the only caller of
 * `/api/v1/sessions`). What this file adds is only the decision "a session already in memory is used as is",
 * and the reduction of both steps' branches to the four a one-tap feature acts on.
 *
 * ⚠ THE SESSION STAYS IN MEMORY. The caller hands it to `kho-phien.tsx`'s `datPhien`, exactly as
 * `PhatHanhPhien` does; nothing here writes to the device (`phase1-collects-nothing.test.ts`).
 *
 * ⚠ AN EMPTY PHONE CODE IS NOT SENT. Zalo returns an empty code outside a real phone (`MA_RONG`); sending it
 * would earn a 401 that reads "code expired" — a wrong instruction. It is `failed` here, before any call.
 */
import type { KetQuaXin, MaDangNhap } from "../tinh-nang/zalo-api"; // vi-name-ok: existing exports, not new names
import { xinMaDangNhap } from "../tinh-nang/zalo-api"; // vi-name-ok: existing export, not a new name

import { type KetQuaPhien, phatHanhPhien } from "./goi-may-chu"; // vi-name-ok: existing exports, not new names
import type { Phien } from "./hop-dong"; // vi-name-ok: existing export, not a new name

/**
 * One branch per thing the screen does next:
 *
 *   `ready`         a session to send with — already held, or issued by this tap
 *   `refused`       the citizen said no on Zalo's phone dialog: a NORMAL answer, nothing was sent
 *   `outside-zalo`  the SDK cannot load: no Zalo call of any kind works here
 *   `failed`        Zalo gave no usable code, or the server issued no session — the citizen may tap again
 */
export type SessionAttempt =
  | { kind: "ready"; session: Phien; issued: boolean }
  | { kind: "refused" }
  | { kind: "outside-zalo" }
  | { kind: "failed" };

/** The two existing steps, injectable so the branches can be tested without Zalo or a server. */
export type SessionSteps = {
  requestCodes: () => Promise<KetQuaXin<MaDangNhap>>; // vi-name-ok: existing types
  issue: (codes: MaDangNhap) => Promise<KetQuaPhien>; // vi-name-ok: existing types
};

export const LIVE_SESSION_STEPS: SessionSteps = {
  requestCodes: xinMaDangNhap, // vi-name-ok: existing export
  issue: (codes) => phatHanhPhien(codes), // vi-name-ok: existing export
};

/** Never throws: both steps already reduce every path to a branch. */
export async function ensureSession(
  current: Phien | null, // vi-name-ok: existing type
  steps: SessionSteps = LIVE_SESSION_STEPS,
): Promise<SessionAttempt> {
  if (current !== null) return { kind: "ready", session: current, issued: false };

  const codes = await steps.requestCodes();
  if (codes.kieu === "tu-choi") return { kind: "refused" };
  if (codes.kieu === "ngoai-zalo") return { kind: "outside-zalo" };
  if (codes.kieu !== "xong" || codes.du_lieu.ma_so_dien_thoai === "") return { kind: "failed" };

  const issued = await steps.issue(codes.du_lieu);
  return issued.kieu === "xong" ? { kind: "ready", session: issued.phien, issued: true } : { kind: "failed" };
}
