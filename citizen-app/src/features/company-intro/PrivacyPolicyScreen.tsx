import { TIEU_DE_CHINH_SACH } from "../../content/chinh-sach-rieng-tu";

import { ChinhSachRiengTu } from "./ChinhSachRiengTu";
import { DocumentGlyph } from "./icons";

/**
 * THE PRIVACY POLICY AS ITS OWN SUB-SCREEN (owner 12/10/2026: "text walls"). Until today it was the last block of
 * Liên hệ, nine sections long under the contacts, and a reviewer or a user looking for it had to scroll past every
 * other block to find it. Now one row on Liên hệ ("Chính sách quyền riêng tư") and one on Quản lý quyền open it.
 *
 * A child of the Liên hệ tab, not a tab: the tab bar's 320px measurement leaves no room for a fifth label
 * (`accessibility.test.ts`), and Zalo's own back button closes the Mini App, so the screen carries its own way back.
 * `backLabel` names WHERE it goes back to — Liên hệ or Quản lý quyền — because the two entries return to different
 * places, and a button that says "Quay lại" without saying where is a guess for an older user.
 */
export function PrivacyPolicyScreen(props: { backLabel: string; onBack: () => void }) {
  return (
    <>
      <button type="button" className="quay-lai" onClick={props.onBack}>
        ‹ {props.backLabel}
      </button>

      <section className="banner">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <DocumentGlyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">{TIEU_DE_CHINH_SACH}</h1>
        </div>
      </section>

      <ChinhSachRiengTu />
    </>
  );
}
