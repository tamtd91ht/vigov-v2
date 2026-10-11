/**
 * THE "NHẬN THÔNG BÁO" SWITCH OF CÁ NHÂN — its state and its two acts (ADR 0091 §4). PURE (no React), so tests step
 * it without a DOM — the stance of `commune-session.ts`.
 *
 * NEVER OPTIMISTIC: a tap does not flip the switch. The value shown is always one the SERVER answered — the GET, or
 * the PUT's 200. While the PUT runs the old value stays, marked "saving"; on a failure it simply stays (that is the
 * revert) and one sentence says what to do next. A switch that showed "off" before the server agreed would tell a
 * citizen they had opted out when they had not.
 *
 * NO STATE IS SHOWN UNTIL ONE IS KNOWN (`unknown`): without a session nothing can be read, and drawing the
 * server's default ("on") there would be the app deciding for a citizen it has not asked.
 *
 * SESSION LOSS (no session, 401, 403 `chua_xac_thuc_so`) is the caller's (`onSessionLost`): it drops the session
 * and runs the act again through the commune app's gate — the same path as every personal act.
 */
import type { NotificationPreferenceResult } from "../api/goi-vigov";
import { XA_TN } from "./noi-dung";

export type NotificationPreferenceState =
  | { readonly kind: "unknown" }
  | { readonly kind: "loading" }
  /** The read failed: `message` says what to do; a tap reads again. */
  | { readonly kind: "read-failed"; readonly message: string }
  /** `saving`: a PUT is in flight. `error`: the last PUT failed — `enabled` is still the server's value. */
  | { readonly kind: "ready"; readonly enabled: boolean; readonly saving: boolean; readonly error: string | null };

export type NotificationPreferenceControl = {
  /** Read the citizen's own switch. Ignored while a read or a save is in flight. */
  load(): Promise<void>;
  /** Ask the server for the opposite of the value shown. Only from `ready` and not while saving. */
  toggle(): Promise<void>;
};

export type NotificationPreferenceDeps = {
  read: () => Promise<NotificationPreferenceResult>;
  write: (enabled: boolean) => Promise<NotificationPreferenceResult>;
  /** The session is gone or unverified: drop it and run `retry` through the gate. */
  onSessionLost: (retry: () => void) => void;
  getState: () => NotificationPreferenceState;
  setState: (s: NotificationPreferenceState) => void;
};

/** The result branches that mean "the session cannot serve this" — the gate's job, never a sentence here. */
function sessionLost(kq: NotificationPreferenceResult): boolean {
  return kq.kieu === "chua-co-phien" || kq.kieu === "het-phien" || kq.kieu === "can-xac-thuc-so";
}

/** The sentence for a failed read or save: the network, or anything else. Never a code. PURE. */
export function preferenceFailure(kq: NotificationPreferenceResult, act: "read" | "save"): string {
  const network = kq.kieu === "loi-mang";
  if (act === "read") return network ? XA_TN.notify_read_network : XA_TN.notify_read_failed;
  return network ? XA_TN.notify_save_network : XA_TN.notify_save_failed;
}

export function createNotificationPreference(d: NotificationPreferenceDeps): NotificationPreferenceControl {
  const control: NotificationPreferenceControl = {
    async load() {
      const s = d.getState();
      if (s.kind === "loading" || (s.kind === "ready" && s.saving)) return;
      const before = s;
      d.setState({ kind: "loading" });
      const kq = await d.read();
      if (kq.kieu === "xong") {
        d.setState({ kind: "ready", enabled: kq.zaloEnabled, saving: false, error: null });
        return;
      }
      if (sessionLost(kq)) {
        // Back to what was shown before the tap (nothing is known yet), then the gate runs the read again.
        d.setState(before.kind === "ready" ? before : { kind: "unknown" });
        d.onSessionLost(() => void control.load());
        return;
      }
      d.setState({ kind: "read-failed", message: preferenceFailure(kq, "read") });
    },

    async toggle() {
      const s = d.getState();
      if (s.kind !== "ready" || s.saving) return;
      const wanted = !s.enabled;
      d.setState({ ...s, saving: true, error: null });
      const kq = await d.write(wanted);
      if (kq.kieu === "xong") {
        // The SERVER's value, not `wanted`: they agree on a 200, and if they ever did not, the server is the truth.
        d.setState({ kind: "ready", enabled: kq.zaloEnabled, saving: false, error: null });
        return;
      }
      // Unchanged value — the revert is that nothing was ever flipped.
      d.setState({ kind: "ready", enabled: s.enabled, saving: false, error: null });
      if (sessionLost(kq)) {
        d.onSessionLost(() => void control.toggle());
        return;
      }
      d.setState({ kind: "ready", enabled: s.enabled, saving: false, error: preferenceFailure(kq, "save") });
    },
  };
  return control;
}
