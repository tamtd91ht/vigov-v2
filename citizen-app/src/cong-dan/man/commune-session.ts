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
 * EVERY OUTCOME HAS AN EXIT: `reset` works at any time (including while an open runs), every `ket-qua`
 * screen carries "Về trang chủ", and a retry only where a new tap can change the answer.
 *
 * WHEN ZALO GIVES NO NUMBER (ADR 0080, 08/10/2026) — the invitation to share the Zalo number still comes FIRST
 * (decision 1); typing a name and a number is the fallback, offered for SENDING only (`offersManualSend`):
 *   · the citizen declined (our "Không chia sẻ", or Zalo's own dialog) → `tu-choi` + "Gửi bằng họ tên và số
 *     điện thoại"; that tap (`manual`) opens a PHONE-LESS session (`skip`) and runs the send;
 *   · Zalo failed the phone step with a code → `thu-lai` + the same button;
 *   · identity issued a phone-less session (the commune app's one retry after a refused phone code) →
 *     `no-phone`, the session is already stored, the button just runs the send.
 *   Outside Zalo (`ngoai-zalo`) offers nothing: no session means no commune (rule 1).
 * What a phone-less session may run (`runsWithoutPhone`): sending, and looking up one's own petition by code.
 * "Phản ánh của tôi" and rating need a verified phone; with a phone-less session they ask for the number, or —
 * once Zalo has given none in this open — say so (`chua-xac-thuc-so`) without asking again.
 */
import {
  type CommuneAppSessionOutcome,
  type OpenCommuneAppSession,
  openCommuneAppSession,
  type SessionPhone,
  type ZaloFailure,
} from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import {
  COMMUNE_APP_SESSION,
  PHONE_VERIFICATION,
  PHONE_VERIFICATION_TASK,
  type PhoneVerificationTask,
  TYPED_CONTACT,
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
  /**
   * A personal act wants to run. Runs it at once when a session that can serve it exists; otherwise explains
   * and waits. `task` absent = the strict reading: only a verified session runs it, and no fallback is offered.
   */
  require(run: () => void, task?: PhoneVerificationTask): void;
  /** The citizen tapped "Đồng ý chia sẻ số điện thoại". */
  allow(): Promise<void>;
  /** The citizen tapped "Không chia sẻ". */
  decline(): void;
  /** The citizen tapped "Gửi bằng họ tên và số điện thoại" (ADR 0080) — only where `offersManualSend` says. */
  manual(): Promise<void>;
  /** The citizen started something else — clear the sentence. Never clears a final outcome. */
  reset(): void;
};

/** The outcomes a fresh tap can change. `no-phone` is not final: the typed-contact path stays open. */
const CAN_ASK_AGAIN: ReadonlySet<SessionGateStop> = new Set(["tu-choi", "thu-lai", "cho-lat", "tam-ngung", "no-phone"]);

/** The acts a phone-less session can serve (ADR 0080 #1, #3). Exported for tests. */
export function runsWithoutPhone(task: PhoneVerificationTask | undefined): boolean {
  return task === "submit" || task === "lookup";
}

/**
 * Whether "Gửi bằng họ tên và số điện thoại" is offered under this outcome — SENDING only, and only when Zalo
 * gave no number: the citizen declined, Zalo failed the PHONE step (a failure of the access-token step would
 * fail the phone-less open too), or identity issued a phone-less session. Never outside Zalo, never for a
 * network failure, a closed channel or another commune. PURE.
 */
export function offersManualSend(
  outcome: SessionGateStop,
  task: PhoneVerificationTask | undefined,
  zalo?: ZaloFailure,
): boolean {
  if (task !== "submit") return false;
  if (outcome === "tu-choi" || outcome === "no-phone") return true;
  return outcome === "thu-lai" && zalo?.capability === "phone";
}

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
  /** The task of `pending` — what decides whether a phone-less session may serve it. */
  let pendingTask: PhoneVerificationTask | undefined;
  let finalOutcome: SessionGateStop | null = null;
  /**
   * Zalo gave no number in THIS open although the citizen agreed (the phone-less answer of `ask`). Asking again
   * for an act that needs the number would only repeat that — so such an act is told so instead.
   */
  let phoneUnavailable = false;
  /** An open is in flight. Only one at a time: two would be two Zalo/server exchanges for one tap. */
  let busy = false;
  /**
   * The screen is waiting for the in-flight open. `reset` clears it WHILE `busy` too — the citizen's
   * "Quay lại" must always work; a gate that ignored it (before 01/10/2026) left the citizen on "Đang kết
   * nối…" with every button dead for as long as the opener took, or forever if it never settled.
   */
  let waiting = false;

  function runPending() {
    const run = pending;
    pending = null;
    setState(null);
    run?.();
  }

  function show(outcome: SessionGateStop, zalo?: ZaloFailure) {
    setState(
      outcome === "thu-lai" && zalo !== undefined
        ? { kieu: "ket-qua", outcome, zalo }
        : { kieu: "ket-qua", outcome },
    );
  }

  /** One open, `ask` or `skip`, and what follows it. Shared by `allow` and `manual`. */
  async function openAndFollow(phone: SessionPhone) {
    if (open === undefined) return;
    busy = true;
    waiting = true;
    setState({ kieu: "dang-mo" });
    let outcome: CommuneAppSessionOutcome;
    try {
      outcome = await openCommuneAppSession(open, communeName(), phone);
    } catch {
      // `openCommuneAppSession` already turns a throwing opener into `thu-lai`; this is the same promise
      // for anything around it, so no path ends with the citizen on "Đang kết nối…".
      outcome = { kieu: "thu-lai" };
    } finally {
      busy = false;
    }
    if (outcome.kieu === "no-phone" && phone === "ask") phoneUnavailable = true;
    if (!waiting) {
      // The citizen left while it ran. A session is already stored (the next act runs at once); a final
      // outcome is remembered so the next act says it; nothing runs and nothing is shown now.
      if (outcome.kieu !== "da-mo" && !CAN_ASK_AGAIN.has(outcome.kieu)) finalOutcome = outcome.kieu;
      return;
    }
    waiting = false;
    if (outcome.kieu === "da-mo") return runPending();
    if (outcome.kieu === "no-phone") {
      // A phone-less session is stored. The citizen tapped "Gửi bằng họ tên…" (`skip`) or the act can be served
      // without a number (lookup): run it. A SEND after `ask` stops here first: the citizen agreed to share the
      // number and must be told Zalo gave none before the form asks them to type it.
      if (phone === "skip" || (runsWithoutPhone(pendingTask) && pendingTask !== "submit")) return runPending();
      if (pendingTask === "submit") return show("no-phone");
      pending = null;
      return show("chua-xac-thuc-so");
    }
    if (!CAN_ASK_AGAIN.has(outcome.kieu)) {
      finalOutcome = outcome.kieu;
      pending = null;
    } else if (outcome.kieu === "tu-choi" && !offersManualSend("tu-choi", pendingTask)) {
      pending = null;
    }
    show(outcome.kieu, outcome.kieu === "thu-lai" ? outcome.zalo : undefined);
  }

  const gate: SessionGate = {
    require(run, task) {
      const session = layPhienViGov();
      if (session !== null && (session.phone_verified || runsWithoutPhone(task))) {
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
      if (session !== null && phoneUnavailable) {
        // A phone-less session, an act that needs the number, and Zalo already gave none in this open.
        pending = null;
        setState({ kieu: "ket-qua", outcome: "chua-xac-thuc-so" });
        return;
      }
      pending = run;
      pendingTask = task;
      if (busy) {
        // The citizen left and came back while the earlier open still runs: wait for THAT one, never a
        // second exchange — and never a silent tap.
        waiting = true;
        setState({ kieu: "dang-mo" });
        return;
      }
      setState({ kieu: "hoi" });
    },

    async allow() {
      if (open === undefined || pending === null || busy) return;
      await openAndFollow("ask");
    },

    decline() {
      // Sending keeps the act: "Gửi bằng họ tên và số điện thoại" may still run it (ADR 0080 #1).
      if (!offersManualSend("tu-choi", pendingTask)) pending = null;
      setState({ kieu: "ket-qua", outcome: "tu-choi" });
    },

    async manual() {
      if (open === undefined || pending === null || busy || pendingTask !== "submit") return;
      // The phone-less session from the commune app's retry is already stored: just run the send.
      if (layPhienViGov() !== null) return runPending();
      await openAndFollow("skip");
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
 * The sentence for an outcome, naming what did not happen on this screen. Never a code, in the sentence:
 * when `zalo` is given (with `thu-lai`) the sentence names the missing Zalo permission, and ZALO's code goes
 * on the separate `zaloSupportCode` line the screen renders under it.
 */
export function sessionGateMessage(
  outcome: SessionGateStop,
  task: PhoneVerificationTask,
  zalo?: ZaloFailure,
): string {
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
    case "no-phone":
      // Only sending has the typed-contact path; any other act reads as "the number could not be confirmed".
      return task === "submit" ? TYPED_CONTACT.zalo_gave_no_number(t) : PHONE_VERIFICATION.still_unverified(t);
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
