/**
 * "GỌI HOTLINE" — ONE LINK, SIX PLACES (owner 12/10/2026, Zalo resubmission).
 *
 * WHY THIS EXISTS: the Mini App's webview ignores `tel:`. The six hotline links of the intro (home quick menu,
 * Liên hệ, a solution's page, Gợi ý giải pháp, the sign-in invitation, the contacts under the sign-in block) did
 * nothing inside Zalo — the most visible broken button a reviewer can press.
 *
 * THE SHAPE: still an `<a href="tel:…">`. Outside Zalo (a browser, a test) that link is what works, so it stays the
 * default; inside Zalo (the runtime set an App ID, `readRuntimeAppId`) the tap is taken over and dialled through
 * `openPhone` (`dialHotline`), which opens the phone's own dialer — the phone asks before calling. Kept in
 * `features/tinh-nang/` because it calls the platform (`phase1-collects-nothing.test.ts`).
 *
 * A FAILED CALL SAYS ONE SENTENCE, with the number to dial by hand — never a code (README §Non-negotiables #7).
 */
import { type ReactNode, useState } from "react";

import { CONTACT } from "../../content/company-profile";

import { dialHotline, readRuntimeAppId } from "./zalo-api";

/** What a failed platform call shows. Names the next step and the number; no error code. */
export const HOTLINE_CALL_FAILED = `Chưa mở được màn hình gọi. Vui lòng gọi số ${CONTACT.hotlineLabel} từ điện thoại của bạn.`;

/**
 * Whether a tap should be dialled through the platform rather than left to `tel:`. PURE given the App ID read:
 * an App ID is present only inside Zalo, where `tel:` does nothing.
 */
export function dialsThroughPlatform(appId: string | null): boolean {
  return appId !== null;
}

export function HotlineLink(props: { className: string; children: ReactNode }) {
  const [failed, setFailed] = useState(false);

  async function dial() {
    const result = await dialHotline(CONTACT.hotlineDialable);
    setFailed(result.kieu !== "xong");
  }

  return (
    <>
      <a
        className={props.className}
        href={`tel:${CONTACT.hotlineDialable}`}
        onClick={(event) => {
          if (!dialsThroughPlatform(readRuntimeAppId())) return;
          event.preventDefault();
          void dial();
        }}
      >
        {props.children}
      </a>
      {failed && (
        <p className="tn__loi" role="alert">
          {HOTLINE_CALL_FAILED}
        </p>
      )}
    </>
  );
}
