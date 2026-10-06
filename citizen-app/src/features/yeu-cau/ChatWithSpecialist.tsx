/**
 * "CHAT VỚI CHUYÊN VIÊN" — a block on Liên hệ (owner decision 07/10/2026).
 *
 * ONE TAP: log in if needed (the existing one-tap login, `ensure-session.ts`), ask Zalo for the display name,
 * record a `chat` request, open the OA chat. No account to create, no status to wait on.
 *
 * ⚠ THE FLOATING "Chat Zalo" BUTTON IS UNTOUCHED AND STAYS THE WAY TO CHAT WITHOUT SHARING ANYTHING. This block
 *   is the second door, for a citizen who wants the specialist to know who is writing. Saying so on screen is
 *   what makes refusing a normal answer rather than a dead end.
 *
 * ⚠ THE NAME IS NEVER RENDERED, LOGGED OR KEPT: it goes from `layTenZalo` into one request body and nowhere else.
 */
import { useState } from "react";

import { MessagingGlyph } from "../company-intro/icons";
import { dungPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing export
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export

import { CHAT_SPECIALIST } from "./noi-dung";
import { type ChatOutcome, campaignSource, chatWithSpecialist, LIVE_STEPS, type QuickRequestSteps } from "./quick-request";

export type ChatViewState = { kind: "idle" } | { kind: "busy" } | ChatOutcome;

/** The `id` of the heading, for `aria-labelledby`. */
const HEADING_ID = "chat-specialist";

/** One sentence per outcome. A table, so a new outcome without its sentence fails `tsc`. */
function outcomeSentence(state: ChatOutcome): { text: string; done: boolean } {
  switch (state.kind) {
    case "refused":
      return { text: CHAT_SPECIALIST.refused, done: false };
    case "outside-zalo":
      return { text: LOI_MO_NGOAI, done: false };
    case "opened":
      return state.delivered
        ? { text: CHAT_SPECIALIST.opened_delivered, done: true }
        : { text: CHAT_SPECIALIST.opened_undelivered, done: false };
    case "not-opened":
      return { text: CHAT_SPECIALIST.not_opened, done: false };
  }
}

/** The block, PURE — every state renders without Zalo, a server or a DOM. */
export function ChatWithSpecialistView({ state, onPress }: { state: ChatViewState; onPress: () => void }) {
  const busy = state.kind === "busy";
  const sentence = state.kind === "idle" || state.kind === "busy" ? null : outcomeSentence(state);

  return (
    <section className="tn hien-len" aria-labelledby={HEADING_ID}>
      <div className="tn__dau">
        <span className="tn__huy-hieu" aria-hidden="true">
          <MessagingGlyph className="tn__glyph" />
        </span>
        <h2 className="tn__tieu-de" id={HEADING_ID}>
          {CHAT_SPECIALIST.title}
        </h2>
      </div>
      <p className="tn__vi-sao">{CHAT_SPECIALIST.why}</p>

      {/* Busy is said in WORDS on the button itself, not by its colour. */}
      <button type="button" className="tn__nut" onClick={onPress} disabled={busy} aria-busy={busy}>
        {busy ? CHAT_SPECIALIST.busy : CHAT_SPECIALIST.button}
      </button>

      <div className="tn__ket-qua" role="status">
        {sentence !== null && <p className={sentence.done ? "tn__xong" : "tn__loi"}>{sentence.text}</p>}
      </div>

      <p className="tn__giai-thich">{CHAT_SPECIALIST.share_nothing}</p>
    </section>
  );
}

export function ChatWithSpecialist({ steps = LIVE_STEPS }: { steps?: QuickRequestSteps }) {
  const { phien, datPhien } = dungPhien(); // vi-name-ok: existing context fields
  const [state, setState] = useState<ChatViewState>({ kind: "idle" });

  async function press() {
    setState({ kind: "busy" });
    // Never throws: every step already reduces its failures to a branch.
    setState(await chatWithSpecialist(phien, campaignSource(), datPhien, steps));
  }

  return <ChatWithSpecialistView state={state} onPress={() => void press()} />;
}
