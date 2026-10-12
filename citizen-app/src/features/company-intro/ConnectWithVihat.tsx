/**
 * "KẾT NỐI VỚI ViHAT" — ONE CARD FOR EVERY ASK OF THE PHONE NUMBER (owner 12/10/2026, Zalo resubmission).
 *
 * Before this card, Liên hệ asked for the number twice (login, "Chat với chuyên viên") and the home screen a third
 * time ("Nhận ưu đãi qua SMS"): three asks, three purposes, two screens, and a reviewer reading one of them could
 * not see the other two. Mini App policy 3.3.4 refuses a permission flow whose purpose is unclear, so the three now
 * sit together, each a separate action with its own heading, its own purpose sentence and its own button — the
 * sentence always rendered BEFORE the button, so it is read before Zalo's dialog can appear.
 *
 * ⚠ LAYOUT ONLY. Each action is the existing component, unchanged in what it calls and sends: `KhoiDangNhap`
 *   (`getPhoneNumber` + `getAccessToken` → `POST /api/v1/sessions`), `ChatWithSpecialist`, `SmsOffers` (consent
 *   sentence, stop button and list read untouched). Nothing here calls Zalo or the network.
 *
 * ⚠ CONNECTED = A SESSION IS HELD (in memory, `kho-phien.tsx`). Then the login action is replaced by one sentence
 *   instead of asking again. "Permission granted but no session" is NOT treated as connected: turning it into a
 *   session would mean logging in without a tap — a network call the citizen did not start. The sentence names no
 *   number, masked or not: the app never receives it (see `CONNECT_VIHAT`).
 */
import { useState } from "react";

import { bearerCua, dungPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing exports
import { KhoiDangNhap } from "../tinh-nang/index"; // vi-name-ok: existing export
import { noiDung } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export
import { ChatWithSpecialist } from "../yeu-cau/ChatWithSpecialist";
import { CONNECT_VIHAT } from "../yeu-cau/noi-dung";
import { SmsOffers } from "../yeu-cau/SmsOffers";

import { MOC_DANG_NHAP } from "./dieu-huong";
import { HandshakeGlyph } from "./icons";

const HEADING_ID = "connect-vihat";

/**
 * The login slot once a session is held. Its heading keeps the login block's `id`: "Đăng nhập lại" on Tư vấn and
 * Yêu cầu của tôi scrolls to that anchor (`MOC_DANG_NHAP`), and an anchor that exists only while logged out would
 * leave those buttons landing on the top of the screen.
 */
export function ConnectedLogin({ onRelogin }: { onRelogin: () => void }) {
  return (
    <section className="tn tn--muc" aria-labelledby={MOC_DANG_NHAP}>
      <div className="tn__dau">
        <span className="tn__huy-hieu" aria-hidden="true">
          <HandshakeGlyph className="tn__glyph" />
        </span>
        <h3 className="tn__tieu-de" id={MOC_DANG_NHAP}>
          {noiDung("dang-nhap").tieu_de}
        </h3>
      </div>
      {/* Said in WORDS, not by a colour or a tick alone. */}
      <p className="tn__xong" role="status">
        {CONNECT_VIHAT.connected}
      </p>
      <button type="button" className="tn__nut-phu" onClick={onRelogin}>
        {CONNECT_VIHAT.relogin}
      </button>
    </section>
  );
}

/** The card, PURE but for the session it reads — renders without Zalo, a server or a DOM. */
export function ConnectWithVihat() {
  const { phien } = dungPhien(); // vi-name-ok: existing context field
  // "Đăng nhập lại" chosen: show the ordinary login action again (its purpose, then its button). UI state only.
  const [relogin, setRelogin] = useState(false);
  const connected = bearerCua(phien) !== "";

  return (
    <section className="tn hien-len" aria-labelledby={HEADING_ID}>
      <div className="tn__dau">
        <h2 className="tn__tieu-de" id={HEADING_ID}>
          {CONNECT_VIHAT.title}
        </h2>
      </div>
      <p className="tn__vi-sao">{CONNECT_VIHAT.lead}</p>

      {connected && !relogin ? <ConnectedLogin onRelogin={() => setRelogin(true)} /> : <KhoiDangNhap heading="h3" />}
      <ChatWithSpecialist heading="h3" />
      <SmsOffers heading="h3" />
    </section>
  );
}
