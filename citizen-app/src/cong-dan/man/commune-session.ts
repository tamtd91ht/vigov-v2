/**
 * THE COMMUNE APP'S SESSION GATE — every personal act (send a petition, "my petitions", look up my ticket,
 * rate) passes through here first (ADR 0047:251, ADR 0045:148-176 and 311).
 *
 * WHY A GATE AND NOT A LOGIN AT OPEN: the commune app opens with no login (ADR 0047 §6). A session is only
 * needed for a personal act, and every commune-app session asks Zalo for the phone (`vihat-miniapp`
 * verifies the App ID by exchanging the phone token), so opening one at launch would put a phone dialog in
 * front of someone who only came to read the news.
 *
 * ASK FIRST, PROMPT SECOND (policy 3.3.4 — the pattern of `phone-verification.tsx`, 1e33603): `require`
 * never opens Zalo's dialog. It shows the explanation (`hoi`); only the citizen's tap on "Đồng ý chia sẻ số
 * điện thoại" (`allow`) calls the injected opener, which is what opens the dialog.
 *
 * NO LOOP: an outcome a new tap cannot change (not connected, other commune, unverified, outside Zalo, a
 * session the server made unusable) is FINAL — the next `require` shows that sentence and asks nothing.
 * Only refusal (the citizen may change their mind), network and "wait" outcomes allow asking again.
 *
 * THE 403 `chua_xac_thuc_so` PATH IS NOT HERE: screens hand `communeAppReopen(open)` to
 * `usePhoneVerification`, which already owns that machine (same opener, same commune check).
 *
 * PURE — no React, so tests step it without a DOM (the same reason as `createPhoneVerification`).
 *
 * `openAtOnce` (the `--demo` build only, ADR 0047 §6 row of 01/10/2026): that build's opener asks Zalo for
 * nothing — there is no phone dialog — so the explanation written for that dialog has nothing to explain,
 * and `require` opens the session at once. Every outcome after that is handled as in every build; only the
 * retry button's words differ (`sessionGateRetryLabel`), since no "Đồng ý chia sẻ" act exists there.
 *
 * EVERY OUTCOME HAS AN EXIT: `reset` works at any time (including while an open runs), every `ket-qua`
 * screen carries "Về trang chủ", and a retry only where a new tap can change the answer.
 */
import {
  type CommuneAppSessionOutcome,
  type OpenCommuneAppSession,
  openCommuneAppSession,
  type ZaloFailure,
} from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import {
  COMMUNE_APP_SESSION,
  PHONE_VERIFICATION,
  PHONE_VERIFICATION_TASK,
  type PhoneVerificationTask,
  zaloFailureSentence,
} from "./noi-dung";

/** Outcomes that stop the act — each one sentence. */
export type SessionGateStop = Exclude<CommuneAppSessionOutcome["kieu"], "da-mo">;

/** What the screen shows. `null` = nothing (no act pending, or the act is running). */
export type SessionGateState =
  | { readonly kieu: "hoi" }
  | { readonly kieu: "dang-mo" }
  /** `zalo` (with `thu-lai` only): Zalo refused a step with a code — the sentence names it. */
  | { readonly kieu: "ket-qua"; readonly outcome: SessionGateStop; readonly zalo?: ZaloFailure };

export type SessionGate = {
  /** A personal act wants to run. Runs it at once when a session exists; otherwise explains and waits. */
  require(run: () => void): void;
  /** The citizen tapped "Đồng ý chia sẻ số điện thoại". */
  allow(): Promise<void>;
  /** The citizen tapped "Không chia sẻ". */
  decline(): void;
  /** The citizen started something else — clear the sentence. Never clears a final outcome. */
  reset(): void;
};

/** The outcomes a fresh tap can change. */
const CAN_ASK_AGAIN: ReadonlySet<SessionGateStop> = new Set(["tu-choi", "thu-lai", "cho-lat", "tam-ngung"]);

/**
 * `communeName` is read at call time — the name on the header, from the public lookup of this build's
 * domain. It is the name the session must match (`openCommuneAppSession`). Without an opener (tests,
 * outside Zalo in development) every act ends in `chua-ket-noi` and nothing is called.
 */
export function createSessionGate(
  open: OpenCommuneAppSession | undefined,
  communeName: () => string,
  setState: (s: SessionGateState | null) => void,
  openAtOnce = false,
): SessionGate {
  let pending: (() => void) | null = null;
  let finalOutcome: SessionGateStop | null = null;
  /** An open is in flight. Only one at a time: two would be two Zalo/server exchanges for one tap. */
  let busy = false;
  /**
   * The screen is waiting for the in-flight open. `reset` clears it WHILE `busy` too — the citizen's
   * "Quay lại" must always work; a gate that ignored it (before 01/10/2026) left the citizen on "Đang kết
   * nối…" with every button dead for as long as the opener took, or forever if it never settled.
   */
  let waiting = false;

  const gate: SessionGate = {
    require(run) {
      if (layPhienViGov() !== null) {
        pending = null;
        setState(null);
        run();
        return;
      }
      if (finalOutcome !== null) {
        pending = null;
        setState({ kieu: "ket-qua", outcome: finalOutcome });
        return;
      }
      if (open === undefined) {
        pending = null;
        finalOutcome = "chua-ket-noi";
        setState({ kieu: "ket-qua", outcome: "chua-ket-noi" });
        return;
      }
      pending = run;
      if (busy) {
        // The citizen left and came back while the earlier open still runs: wait for THAT one, never a
        // second exchange — and never a silent tap.
        waiting = true;
        setState({ kieu: "dang-mo" });
        return;
      }
      if (openAtOnce) {
        void gate.allow();
        return;
      }
      setState({ kieu: "hoi" });
    },

    async allow() {
      if (open === undefined || pending === null || busy) return;
      busy = true;
      waiting = true;
      setState({ kieu: "dang-mo" });
      let outcome: CommuneAppSessionOutcome;
      try {
        outcome = await openCommuneAppSession(open, communeName());
      } catch {
        // `openCommuneAppSession` already turns a throwing opener into `thu-lai`; this is the same promise
        // for anything around it, so no path ends with the citizen on "Đang kết nối…".
        outcome = { kieu: "thu-lai" };
      } finally {
        busy = false;
      }
      if (!waiting) {
        // The citizen left while it ran. A session is already stored (the next act runs at once); a final
        // outcome is remembered so the next act says it; nothing runs and nothing is shown now.
        if (outcome.kieu !== "da-mo" && !CAN_ASK_AGAIN.has(outcome.kieu)) finalOutcome = outcome.kieu;
        return;
      }
      waiting = false;
      if (outcome.kieu === "da-mo") {
        const run = pending;
        pending = null;
        setState(null);
        run();
        return;
      }
      if (!CAN_ASK_AGAIN.has(outcome.kieu)) {
        finalOutcome = outcome.kieu;
        pending = null;
      } else if (outcome.kieu === "tu-choi") {
        pending = null;
      }
      setState(
        outcome.kieu === "thu-lai" && outcome.zalo !== undefined
          ? { kieu: "ket-qua", outcome: outcome.kieu, zalo: outcome.zalo }
          : { kieu: "ket-qua", outcome: outcome.kieu },
      );
    },

    decline() {
      pending = null;
      setState({ kieu: "ket-qua", outcome: "tu-choi" });
    },

    reset() {
      pending = null;
      waiting = false;
      setState(null);
    },
  };
  return gate;
}

/**
 * Whether the result offers a retry button. Not after Zalo refused with a code that is not "try again
 * later": the sentence itself says pressing again changes nothing, and a button that repeats the same
 * refusal is the loop a citizen reads as a frozen app.
 */
export function sessionGateOffersRetry(outcome: SessionGateStop, zalo?: ZaloFailure): boolean {
  if (outcome === "thu-lai" && zalo !== undefined && !zalo.transient) return false;
  return outcome === "thu-lai" || outcome === "cho-lat" || outcome === "tam-ngung";
}

/**
 * The retry button's words. With `atOnce` (the `--demo` build) nothing is shared and no Zalo dialog opens,
 * so "Đồng ý chia sẻ số điện thoại" would name an act that does not happen: the button is "Thử lại".
 */
export function sessionGateRetryLabel(atOnce: boolean): string {
  return atOnce ? COMMUNE_APP_SESSION.retry : PHONE_VERIFICATION.allow;
}

/**
 * The sentence for an outcome, naming what did not happen on this screen. Never a code, in the sentence:
 * when `zalo` is given (with `thu-lai`) the sentence names the missing Zalo permission, and ZALO's code goes
 * on the separate `zaloSupportCode` line the screen renders under it.
 */
export function sessionGateMessage(
  outcome: SessionGateStop,
  task: PhoneVerificationTask,
  zalo?: ZaloFailure,
  atOnce = false,
): string {
  const t = PHONE_VERIFICATION_TASK[task];
  if (outcome === "thu-lai" && zalo !== undefined) {
    return PHONE_VERIFICATION.zalo_failed(zaloFailureSentence(zalo), t, zalo.transient);
  }
  // `atOnce`: the two sentences that name the retry button name the one this build shows (`sessionGateRetryLabel`).
  if (atOnce && outcome === "thu-lai") return COMMUNE_APP_SESSION.retry_at_once(t);
  if (atOnce && outcome === "cho-lat") return COMMUNE_APP_SESSION.wait_at_once(t);
  switch (outcome) {
    case "tu-choi":
      return PHONE_VERIFICATION.refused(t);
    case "thu-lai":
      return PHONE_VERIFICATION.retry(t);
    case "ngoai-zalo":
      return PHONE_VERIFICATION.outside_zalo(t);
    case "chua-mo":
      return PHONE_VERIFICATION.unavailable(t);
    case "chua-xac-thuc-so":
      return PHONE_VERIFICATION.still_unverified(t);
    case "chua-ket-noi":
      return COMMUNE_APP_SESSION.not_connected(t);
    case "tam-ngung":
      return COMMUNE_APP_SESSION.paused(t);
    case "cho-lat":
      return COMMUNE_APP_SESSION.wait(t);
    case "khac-xa":
      return COMMUNE_APP_SESSION.other_commune(t);
  }
}
