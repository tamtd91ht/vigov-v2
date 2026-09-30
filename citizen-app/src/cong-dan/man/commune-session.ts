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
 */
import { type CommuneAppSessionOutcome, type OpenCommuneAppSession, openCommuneAppSession } from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { COMMUNE_APP_SESSION, PHONE_VERIFICATION, PHONE_VERIFICATION_TASK, type PhoneVerificationTask } from "./noi-dung";

/** Outcomes that stop the act — each one sentence. */
export type SessionGateStop = Exclude<CommuneAppSessionOutcome["kieu"], "da-mo">;

/** What the screen shows. `null` = nothing (no act pending, or the act is running). */
export type SessionGateState =
  | { readonly kieu: "hoi" }
  | { readonly kieu: "dang-mo" }
  | { readonly kieu: "ket-qua"; readonly outcome: SessionGateStop };

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
): SessionGate {
  let pending: (() => void) | null = null;
  let finalOutcome: SessionGateStop | null = null;
  let busy = false;

  return {
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
      if (!CAN_ASK_AGAIN.has(outcome.kieu)) {
        finalOutcome = outcome.kieu;
        pending = null;
      } else if (outcome.kieu === "tu-choi") {
        pending = null;
      }
      setState({ kieu: "ket-qua", outcome: outcome.kieu });
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
  };
}

/** Whether the result sentence of this outcome offers the "Đồng ý…" button again. */
export function sessionGateOffersRetry(outcome: SessionGateStop): boolean {
  return outcome === "thu-lai" || outcome === "cho-lat" || outcome === "tam-ngung";
}

/** The sentence for an outcome, naming what did not happen on this screen. Never a code. */
export function sessionGateMessage(outcome: SessionGateStop, task: PhoneVerificationTask): string {
  const t = PHONE_VERIFICATION_TASK[task];
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
