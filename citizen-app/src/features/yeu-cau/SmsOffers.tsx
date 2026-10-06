/**
 * "NHẬN ƯU ĐÃI QUA SMS" — a block on the home screen, beside "Tin ViHAT" (owner decision 07/10/2026).
 *
 * ONE TAP: log in if needed (the existing one-tap login), record `sms_promo`, say "Đã ghi nhận". Stopping is the
 * same shape: one tap, `sms_optout`.
 *
 * ⚠ THE STOP BUTTON IS NEVER OUT OF REACH. The consent sentence promises "bấm Huỷ ngay tại đây, bất cứ lúc
 *   nào", so "Huỷ nhận ưu đãi SMS" is hidden ONLY when the server's own list says this citizen is not
 *   subscribed. With no session (the app was reopened — sessions live in memory) the state is UNKNOWN, and both
 *   buttons show: an opt-out sent by someone not subscribed costs nothing; a subscriber who cannot find the
 *   stop button is a consent promise broken.
 *
 * ⚠ THE SUBSCRIPTION IS DERIVED, NEVER STORED: the latest `sms_promo` / `sms_optout` in `GET /api/v1/requests`
 *   decides (`deriveSmsSubscription`), and the answer lives in this component's memory. Nothing is written to
 *   the device (`phase1-collects-nothing.test.ts`).
 */
import { useEffect, useRef, useState } from "react";

import { docYeuCauCuaToi, type KetQuaDoc } from "../../api/goi-may-chu"; // vi-name-ok: existing exports
import { MessagingGlyph } from "../company-intro/icons";
import { bearerCua, dungPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing exports
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export

import { SMS_OFFERS, TU_VAN } from "./noi-dung"; // vi-name-ok: existing export TU_VAN
import {
  campaignSource,
  deriveSmsSubscription,
  LIVE_STEPS,
  type QuickRequestSteps,
  sendSmsChoice,
  type SmsKind,
  type SmsOutcome,
} from "./quick-request";

export type SmsSubscription = "unknown" | "subscribed" | "not-subscribed";

export type SmsAction = { kind: "idle" } | { kind: "busy"; choice: SmsKind } | { kind: "done"; choice: SmsKind; outcome: SmsOutcome };

const HEADING_ID = "sms-offers";

/** The sentence for one outcome, and whether it is the success sentence. */
function outcomeSentence(choice: SmsKind, outcome: SmsOutcome): { text: string; done: boolean } {
  const subscribing = choice === "sms_promo";
  switch (outcome.kind) {
    case "refused":
      return { text: subscribing ? SMS_OFFERS.refused_subscribe : SMS_OFFERS.refused_unsubscribe, done: false };
    case "outside-zalo":
      return { text: LOI_MO_NGOAI, done: false };
    case "session-failed":
      return { text: SMS_OFFERS.session_failed, done: false };
    case "sent":
      switch (outcome.result.kieu) {
        case "xong":
          return { text: subscribing ? SMS_OFFERS.done_subscribe : SMS_OFFERS.done_unsubscribe, done: true };
        case "chua-dang-nhap":
          return { text: SMS_OFFERS.expired, done: false };
        // The server's own Vietnamese sentence, verbatim (`docCauLoi`); ours only when it said nothing.
        case "tu-choi":
          return { text: outcome.result.cau === "" ? TU_VAN.cau_lui_tu_choi : outcome.result.cau, done: false };
        case "chua-khai-host":
          return { text: TU_VAN.chua_khai_host, done: false };
        case "khong-goi-duoc":
          return { text: SMS_OFFERS.network, done: false };
      }
  }
}

/** The block, PURE — every state renders without Zalo, a server or a DOM. */
export function SmsOffersView({
  subscription,
  action,
  onChoose,
}: {
  subscription: SmsSubscription;
  action: SmsAction;
  onChoose: (choice: SmsKind) => void;
}) {
  const busy = action.kind === "busy";
  const sentence = action.kind === "done" ? outcomeSentence(action.choice, action.outcome) : null;

  return (
    <section className="tn hien-len" aria-labelledby={HEADING_ID}>
      <div className="tn__dau">
        <span className="tn__huy-hieu" aria-hidden="true">
          <MessagingGlyph className="tn__glyph" />
        </span>
        <h2 className="tn__tieu-de" id={HEADING_ID}>
          {SMS_OFFERS.title}
        </h2>
      </div>
      <p className="tn__vi-sao">{SMS_OFFERS.consent}</p>

      {/* The state is said in WORDS, never by a coloured badge alone. */}
      {subscription === "subscribed" && <p className="tn__xong">{SMS_OFFERS.status_subscribed}</p>}

      {subscription !== "subscribed" && (
        <button
          type="button"
          className="tn__nut"
          onClick={() => onChoose("sms_promo")}
          disabled={busy}
          aria-busy={busy && action.choice === "sms_promo"}
        >
          {busy && action.choice === "sms_promo" ? SMS_OFFERS.busy : SMS_OFFERS.subscribe}
        </button>
      )}
      {subscription !== "not-subscribed" && (
        <button
          type="button"
          className="tn__nut-phu"
          onClick={() => onChoose("sms_optout")}
          disabled={busy}
          aria-busy={busy && action.choice === "sms_optout"}
        >
          {busy && action.choice === "sms_optout" ? SMS_OFFERS.busy : SMS_OFFERS.unsubscribe}
        </button>
      )}

      <div className="tn__ket-qua" role="status">
        {sentence !== null && <p className={sentence.done ? "tn__xong" : "tn__loi"}>{sentence.text}</p>}
      </div>
    </section>
  );
}

export function SmsOffers({
  steps = LIVE_STEPS,
  readList = docYeuCauCuaToi, // vi-name-ok: existing export
}: {
  steps?: QuickRequestSteps;
  readList?: (bearer: string) => Promise<KetQuaDoc>; // vi-name-ok: existing type
}) {
  const { phien, datPhien } = dungPhien(); // vi-name-ok: existing context fields
  const bearer = bearerCua(phien); // vi-name-ok: existing export
  const [subscription, setSubscription] = useState<SmsSubscription>("unknown");
  const [action, setAction] = useState<SmsAction>({ kind: "idle" });
  /**
   * Set once a choice of the citizen's was recorded. The tap that logs in also changes the bearer, which
   * starts a list read RACING the POST; a read answered after the POST but begun before it would put back the
   * old state. The citizen's own recorded act is newer than any such read, so the read no longer overrides it.
   */
  const recorded = useRef(false);

  // Read the citizen's own list ONLY when a session already exists — never log in just to look. A failed
  // read leaves the state unknown, which shows both buttons (see the block at the top of this file).
  useEffect(() => {
    if (bearer === "") {
      setSubscription("unknown");
      return;
    }
    let mounted = true;
    void readList(bearer).then((answer) => {
      if (!mounted || recorded.current) return;
      setSubscription(answer.kieu === "xong" ? deriveSmsSubscription(answer.danh_sach) : "unknown");
    });
    return () => {
      mounted = false;
    };
  }, [bearer, readList]);

  async function choose(choice: SmsKind) {
    setAction({ kind: "busy", choice });
    const outcome = await sendSmsChoice(choice, phien, campaignSource(), datPhien, steps);
    // The server recorded the citizen's own act: that IS the new state, no second read needed.
    if (outcome.kind === "sent" && outcome.result.kieu === "xong") {
      recorded.current = true;
      setSubscription(choice === "sms_promo" ? "subscribed" : "not-subscribed");
    }
    setAction({ kind: "done", choice, outcome });
  }

  return <SmsOffersView subscription={subscription} action={action} onChoose={(c) => void choose(c)} />;
}
