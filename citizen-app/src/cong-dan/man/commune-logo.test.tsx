import { createElement, isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { CommuneProfile } from "../api/hop-dong-cong-khai";
import { BUNDLED_LOGO_SRC, CommuneLogoView, LogoXa, logoSources, markLogoFailed, RootTabHeader } from "./khung-xa";
import { profileLogoUrl } from "./TrangXa";

/**
 * The commune logo in the root-tab header (ADR 0069 #4, #7, #8): the logo the commune uploaded, read at runtime
 * from `/commune-profiles` → the file bundled by `deploy.mjs --vao-thang` (kept as a fallback, ADR 0067 B11's
 * reasoning) → the building icon. A picture that fails to load falls to the next step, never straight to the
 * icon. Server-rendered markup and hookless calls, as in `commune-screens.test.tsx` — no DOM here.
 */

const UPLOADED = "https://media.example.test/vigov-public/t_01HZZZZZZZZZZZZZZZZZZZZZZZ/branding/logo.png";
const noop = () => {};
const html = (el: ReactElement) => renderToStaticMarkup(el);

/** The `<img>` a hookless element is, or null. */
function asImg(node: unknown): ReactElement<Record<string, unknown>> | null {
  if (!isValidElement(node)) return null;
  const el = node as ReactElement<Record<string, unknown>>;
  return el.type === "img" ? el : null;
}

describe("precedence: runtime logo → bundled file → icon", () => {
  it("an uploaded logo comes first, the bundled file second", () => {
    expect(logoSources(UPLOADED)).toEqual([UPLOADED, BUNDLED_LOGO_SRC]);
    expect(BUNDLED_LOGO_SRC).toBe("./logo-xa.png");
  });

  it("no uploaded logo (\"\") → the bundled file alone", () => {
    expect(logoSources("")).toEqual([BUNDLED_LOGO_SRC]);
  });

  it("the header draws the uploaded logo when there is one, decorative, in the fixed 40×40 class", () => {
    const out = html(createElement(RootTabHeader, { title: "Xã Thử", logoUrl: UPLOADED }));
    expect(out).toContain(`<img class="xa-hero__logo" src="${UPLOADED}" alt=""`);
    expect(out).not.toContain(`src="${BUNDLED_LOGO_SRC}"`);
    // The name is beside it in words: the picture says nothing a screen reader would repeat.
    expect(out).toContain(">Xã Thử</h1>");
  });

  it("without one — or before the profile has loaded — the bundled file", () => {
    for (const el of [
      createElement(RootTabHeader, { title: "Xã Thử" }),
      createElement(RootTabHeader, { title: "Xã Thử", logoUrl: "" }),
      createElement(LogoXa, {}),
    ]) {
      expect(html(el)).toContain(`<img class="xa-hero__logo" src="${BUNDLED_LOGO_SRC}" alt=""`);
    }
  });
});

describe("a picture that fails falls to the next step", () => {
  it("uploaded fails → bundled; bundled fails → building icon, aria-hidden", () => {
    const sources = logoSources(UPLOADED);
    let failed: readonly string[] = [];
    const onFail = (src: string) => {
      failed = markLogoFailed(failed, src);
    };

    const first = asImg(CommuneLogoView({ sources, failed, onFail }));
    expect(first?.props.src).toBe(UPLOADED);
    expect(first?.props.alt).toBe("");
    (first!.props.onError as () => void)();
    expect(failed).toEqual([UPLOADED]);

    const second = asImg(CommuneLogoView({ sources, failed, onFail }));
    expect(second?.props.src).toBe(BUNDLED_LOGO_SRC);
    (second!.props.onError as () => void)();
    expect(failed).toEqual([UPLOADED, BUNDLED_LOGO_SRC]);

    const last = html(createElement(CommuneLogoView, { sources, failed, onFail }));
    expect(last).toMatch(/^<span class="xa-hero__dai-dien" aria-hidden="true"><svg/);
    expect(last).not.toContain("<img");
  });

  it("no uploaded logo and the bundled file missing → the icon (a commune app built without a logo file)", () => {
    const out = html(createElement(CommuneLogoView, { sources: logoSources(""), failed: [BUNDLED_LOGO_SRC], onFail: noop }));
    expect(out).toMatch(/^<span class="xa-hero__dai-dien" aria-hidden="true"><svg/);
  });

  it("a bundled file that already failed stays skipped when the uploaded logo arrives later", () => {
    // First paint: no profile yet, the bundled file fails. Then the profile brings a logo: it is tried.
    const failed = markLogoFailed([], BUNDLED_LOGO_SRC);
    expect(asImg(CommuneLogoView({ sources: logoSources(UPLOADED), failed, onFail: noop }))?.props.src).toBe(UPLOADED);
    // …and if it fails too, the icon — not the bundled file a second time.
    const both = markLogoFailed(failed, UPLOADED);
    expect(html(createElement(CommuneLogoView, { sources: logoSources(UPLOADED), failed: both, onFail: noop }))).not.toContain("<img");
  });

  it("the same failure reported twice is recorded once", () => {
    const once = markLogoFailed([], UPLOADED);
    expect(markLogoFailed(once, UPLOADED)).toBe(once);
  });
});

describe("where the runtime logo comes from", () => {
  const profile: CommuneProfile = { name: "Xã Thử", office_address: "", hotline: "", office_hours_text: "", logo_url: UPLOADED };

  it("the profile this open read, or \"\" while it loads / when the read failed", () => {
    expect(profileLogoUrl(profile)).toBe(UPLOADED);
    expect(profileLogoUrl({ ...profile, logo_url: "" })).toBe("");
    expect(profileLogoUrl(null)).toBe("");
  });

  it("every root-tab header passes it — the four tabs show the same logo", () => {
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(src.match(/<DauTab tieu_de=\{[^}]+\} logoUrl=\{profileLogoUrl\(profile\)\} \/>/g)).toHaveLength(3);
    expect(src).toContain("logoUrl={profileLogoUrl(props.profile)}");
    expect(src).not.toMatch(/<DauTab tieu_de=\{[^}]+\} \/>/);
  });
});
