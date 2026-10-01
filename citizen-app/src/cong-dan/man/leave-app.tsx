/**
 * LEAVING THE APP FOR A PAGE THE COMMUNE LINKED — ask first, then open (owner, 01/10/2026; ADR 0067 §1, §5).
 *
 * Two places offer such a page: a link inside an article (`body_blocks`, `article-body.tsx`) and a banner on the
 * home strip whose `link_to` is an https URL (`TrangXa.tsx`). Both go through the SAME question here: "Bạn sắp
 * rời ứng dụng để mở <host>", then "Mở trang" or "Ở lại ứng dụng". WHY ASK: the link's WORDS are whatever the
 * commune typed, and an elderly citizen who taps "Xem thông báo" and lands on a page with another name, in a
 * window that is not the commune's app, cannot tell what happened. The HOST is read from the link itself, so
 * the question names where the tap actually goes.
 *
 * THIS HALF OPENS NOTHING ITSELF. The opener is injected by the shell (`App.tsx`, through the one declared door
 * `moRaNgoai("lien-ket-xa", …)`) exactly like the video opener, because this half may not import `features/` or
 * `zmp-sdk` (`ranh-gioi-hai-nua.test.ts` §3a). No opener (tests, the shared app) → links are plain words and
 * banners with an https target are not tappable — never a button that does nothing.
 *
 * NOTHING IS RECORDED: no log line, no counter, no call to any server on a tap. The privacy policy says so
 * (`content/chinh-sach-rieng-tu.ts`, "ben-thu-ba"); a click log added here would make that sentence false.
 */
import { useState } from "react";

import { readHttpsLink } from "../api/hop-dong-cong-khai";

import { XA_TN } from "./noi-dung";

/** Opens an https page outside the app; `true` when it opened. Same shape as `OpenVideo`. */
export type OpenExternal = (url: string) => Promise<boolean>;

/**
 * The host the question names — `new URL(url).hostname` of a link `readHttpsLink` accepts, else `null` (then
 * nothing may be offered at all). IDN hosts come back in their `xn--` form: what the browser will show, and a
 * look-alike Unicode host cannot pass for a familiar one. PURE.
 */
export function linkHost(url: string): string | null {
  if (readHttpsLink(url) === null) return null;
  return new URL(url).hostname;
}

/** One tap on "Mở trang": open, and answer whether it FAILED (a rejection is a failure). PURE apart from `open`. */
export async function externalOpenFailed(open: OpenExternal, url: string): Promise<boolean> {
  return !(await open(url).catch(() => false));
}

/**
 * The question, over the screen. `role="alertdialog"` + `aria-modal`: a screen reader reads the question and
 * stays in it. Two full-width buttons (`xa-nut` ≥ 48px, body-size text); "Ở lại ứng dụng" takes the focus — the
 * choice that changes nothing. A failed open says what to do next in words (`role="alert"`). Tapping the dimmed
 * area outside the box is "Ở lại". PURE: `failed` is the caller's state.
 */
export function LeaveAppDialog(props: { host: string; failed: boolean; onOpen: () => void; onStay: () => void }) {
  return (
    <div
      className="xa-leave-app"
      onClick={(e) => {
        if (e.target === e.currentTarget) props.onStay();
      }}
    >
      <div
        className="xa-the xa-leave-app__box"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="xa-leave-app-question"
        aria-describedby="xa-leave-app-note"
      >
        <h2 className="xa-dau-khoi__tieu-de xa-leave-app__question" id="xa-leave-app-question">
          {XA_TN.leave_app_question(props.host)}
        </h2>
        <p id="xa-leave-app-note">{XA_TN.leave_app_note}</p>
        {props.failed && (
          <p className="xa-error-box" role="alert">
            {XA_TN.leave_app_failed}
          </p>
        )}
        <button type="button" className="xa-nut" onClick={props.onOpen}>
          {XA_TN.leave_app_open}
        </button>
        <button type="button" className="xa-nut xa-nut--phu" onClick={props.onStay} autoFocus>
          {XA_TN.leave_app_stay}
        </button>
      </div>
    </div>
  );
}

/** What is waiting for the citizen's answer: the link, its host, and whether the last "Mở trang" failed. */
export type PendingLink = { readonly url: string; readonly host: string; readonly failed: boolean };

/** A tap on a link → the question to ask, or `null` when the link is not one this app may open. PURE. */
export function askToLeave(url: string): PendingLink | null {
  const host = linkHost(url);
  return host === null ? null : { url, host, failed: false };
}

/**
 * The flow, for one screen: `ask(url)` raises the question; "Mở trang" opens and closes it, or keeps it with the
 * failure sentence; "Ở lại" closes it. `ask` is `undefined` without an opener, so the caller draws no tappable
 * link at all.
 */
export function useLeaveApp(open: OpenExternal | undefined) {
  const [pending, setPending] = useState<PendingLink | null>(null);
  const ask = open === undefined ? undefined : (url: string) => setPending(askToLeave(url));
  const dialog =
    open === undefined || pending === null ? null : (
      <LeaveAppDialog
        host={pending.host}
        failed={pending.failed}
        onStay={() => setPending(null)}
        onOpen={() => {
          const url = pending.url;
          setPending({ ...pending, failed: false });
          void externalOpenFailed(open, url).then((failed) => setPending(failed ? { ...pending, failed: true } : null));
        }}
      />
    );
  return { ask, dialog };
}
