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
 * DEMO BUILD ONLY (`deploy.mjs --vao-thang --demo`, owner 30/09/2026 — dropped at submission): passed a
 * `demo` argument, a PHONE-STEP failure (`DEMO_PROCEEDS`) runs the act anyway, WITHOUT a session, and every
 * later act of this open runs at once — never the explanation again, never Zalo again, so no loop. Any
 * server call such an act makes answers "no session" without touching the network (`goi-vigov.ts` #1), and
 * the screen shows ONE demo sentence (`showDemoNotice`). Nothing here pretends a session exists: the commune
 * check, "not connected" and "paused" stay exactly as they are. Without `demo` nothing below changes.
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
  | { readonly kieu: "ket-qua"; readonly outcome: SessionGateStop; readonly zalo?: ZaloFailure }
  /** Demo build only: an act that needs the server ran without a session — one sentence, no retry. */
  | { readonly kieu: "demo" };

export type SessionGate = {
  /** A personal act wants to run. Runs it at once when a session exists; otherwise explains and waits. */
  require(run: () => void): void;
  /** The citizen tapped "Đồng ý chia sẻ số điện thoại". */
  allow(): Promise<void>;
  /** The citizen tapped "Không chia sẻ". */
  decline(): void;
  /** The citizen started something else — clear the sentence. Never clears a final outcome. */
  reset(): void;
  /** Demo build only: a phone-step failure let this open's acts run without a session. */
  demoWithoutSession(): boolean;
  /** Demo build only: say, in one sentence, that the server could not be reached without a session. */
  showDemoNotice(): void;
};

/** Demo build only — what the gate tells the screen when an act runs without a session. */
export type SessionGateDemo = { readonly onProceed: () => void };

/**
 * The outcomes that are a failure OF THE PHONE STEP itself — Zalo refused, lacks the permission, errored,
 * was busy, or there is no Zalo; or the server could not verify the number. Only these let a demo build
 * go on. `khac-xa` (another commune), `chua-ket-noi`, `tam-ngung` and `chua-mo` are not about the phone
 * and stop the act as in every build: a demo must never show a form addressed to a commune it cannot name.
 */
export const DEMO_PROCEEDS: ReadonlySet<SessionGateStop> = new Set([
  "tu-choi",
  "thu-lai",
  "cho-lat",
  "ngoai-zalo",
  "chua-xac-thuc-so",
]);

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
  demo?: SessionGateDemo,
): SessionGate {
  let pending: (() => void) | null = null;
  let finalOutcome: SessionGateStop | null = null;
  let busy = false;
  /** Demo only: set once, by a phone-step failure; from then on acts run at once, without a session. */
  let demoProceeded = false;

  return {
    require(run) {
      if (layPhienViGov() !== null) {
        pending = null;
        setState(null);
        run();
        return;
      }
      if (demoProceeded) {
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
      setState({ kieu: "hoi" });
    },

    async allow() {
      if (open === undefined || pending === null || busy) return;
      busy = true;
      setState({ kieu: "dang-mo" });
      const outcome = await openCommuneAppSession(open, communeName());
      busy = false;
      if (outcome.kieu === "da-mo") {
        const run = pending;
        pending = null;
        setState(null);
        run();
        return;
      }
      if (demo !== undefined && DEMO_PROCEEDS.has(outcome.kieu)) {
        demoProceeded = true;
        const run = pending;
        pending = null;
        setState(null);
        demo.onProceed();
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
      if (busy) return;
      pending = null;
      setState(null);
    },

    demoWithoutSession() {
      return demoProceeded;
    },

    showDemoNotice() {
      if (!demoProceeded) return;
      pending = null;
      setState({ kieu: "demo" });
    },
  };
}

/** Whether the result sentence of this outcome offers the "Đồng ý…" button again. */
export function sessionGateOffersRetry(outcome: SessionGateStop): boolean {
  return outcome === "thu-lai" || outcome === "cho-lat" || outcome === "tam-ngung";
}

/**
 * The sentence for an outcome, naming what did not happen on this screen. Never a code, in the sentence:
 * when `zalo` is given (with `thu-lai`) the sentence names the missing Zalo permission, and ZALO's code goes
 * on the separate `zaloSupportCode` line the screen renders under it.
 */
export function sessionGateMessage(outcome: SessionGateStop, task: PhoneVerificationTask, zalo?: ZaloFailure): string {
  const t = PHONE_VERIFICATION_TASK[task];
  if (outcome === "thu-lai" && zalo !== undefined) {
    return PHONE_VERIFICATION.zalo_failed(zaloFailureSentence(zalo), t, zalo.transient);
  }
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
