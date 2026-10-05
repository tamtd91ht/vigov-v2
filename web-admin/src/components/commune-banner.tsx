/**
 * The commune's web-admin banner — a full-width image strip under the topbar on EVERY signed-in page
 * (ADR 0069 #5, owner's choice "Đầu mọi trang"). Rendered by `DauTrang` and nowhere else, so every page
 * that has a topbar has the strip and no page adds it on its own.
 *
 * `src` "" — no banner uploaded, or platform did not answer — renders NOTHING: no empty frame, no
 * placeholder picture (ADR 0069 #7, "chưa có banner thì không có dải").
 *
 * `alt=""`: the banner is the commune's own picture, its content is unknown to this app, and the
 * commune's name is already printed as text in the header. Naming the body again in an alt
 * would be a third reading of the same name for a screen-reader user, and anything else would be a
 * guess at what the picture shows.
 *
 * Fixed height with `object-fit: cover` (`.commune-banner` in globals.css: 72 px on a phone, 112 px from
 * 768 px): the strip never pushes the page's work below the fold, whatever shape the commune uploaded.
 * No overlay, gradient or blur on it (ADR 0068 §11). A plain `<img>` for the reason given on
 * `CommuneLogoImage` (`commune-identity.tsx`); `width`/`height` are the published 1600 px derivative's
 * width and the strip's ratio, so nothing jumps while it loads.
 */
export function CommuneBanner({ src }: { src: string }) {
  if (src === "") return null;
  return (
    <div className="commune-banner">
      {/* eslint-disable-next-line @next/next/no-img-element -- see the doc comment above */}
      <img src={src} alt="" width={1600} height={112} decoding="async" referrerPolicy="no-referrer" />
    </div>
  );
}
