import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE INTRO'S HOTLINE DOOR (owner 12/10/2026, Zalo resubmission) — `dialHotline` (`openPhone`) in `zalo-api.ts` and the
 * `HotlineLink` that every "Gọi hotline" of the ViHAT intro now goes through.
 *
 * What must hold:
 *   · the markup is still a `tel:` link to the published hotline — that is what works outside Zalo
 *   · inside Zalo (an App ID is set) the tap goes to `openPhone`; outside, it is left to `tel:`
 *   · only digits reach the SDK; a failed call ends in one sentence naming the number, never a code
 *   · the declaration row Quản lý quyền reads names the hotline call, in the intro's words
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; the functions under test are real.
 */
const sdk = vi.hoisted(() => ({
  openPhone: vi.fn<(args: { phoneNumber: string }) => Promise<void>>(),
}));
vi.mock("zmp-sdk", () => sdk);
const report = vi.hoisted(() => vi.fn(async () => {}));
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: report }));

import { CONTACT } from "../../content/company-profile";

import { dialsThroughPlatform, HOTLINE_CALL_FAILED, HotlineLink } from "./HotlineLink";
import { dialHotline, KHAI_BAO_LOI_GOI } from "./zalo-api";

beforeEach(() => {
  sdk.openPhone.mockReset().mockResolvedValue(undefined);
  report.mockClear();
});

describe("dialHotline", () => {
  it("hands the hotline's digits to the phone's dialer", async () => {
    expect(await dialHotline(CONTACT.hotlineDialable)).toEqual({ kieu: "xong", du_lieu: undefined });
    expect(sdk.openPhone.mock.calls).toEqual([[{ phoneNumber: CONTACT.hotlineDialable }]]);
  });

  it("anything but digits is refused before the SDK loads", async () => {
    for (const bad of ["", "19", `${CONTACT.hotlineDialable} `, `tel:${CONTACT.hotlineDialable}`, "javascript:alert(1)"]) {
      expect(await dialHotline(bad), bad).toEqual({ kieu: "khong-lay-duoc" });
    }
    expect(sdk.openPhone).not.toHaveBeenCalled();
  });

  it("a coded failure comes back as `khong-lay-duoc` (the screen then says one sentence)", async () => {
    sdk.openPhone.mockRejectedValue(Object.assign(new Error("x"), { code: -1403 }));
    expect(await dialHotline(CONTACT.hotlineDialable)).toMatchObject({ kieu: "khong-lay-duoc" });
  });
});

describe("HotlineLink", () => {
  it("renders a tel: link to the published hotline, with its content", () => {
    const html = renderToStaticMarkup(<HotlineLink className="action">Gọi</HotlineLink>);
    expect(html).toBe(`<a class="action" href="tel:${CONTACT.hotlineDialable}">Gọi</a>`);
  });

  it("dials through the platform only inside Zalo (an App ID is set)", () => {
    expect(dialsThroughPlatform("1234567890")).toBe(true);
    expect(dialsThroughPlatform(null)).toBe(false);
  });

  it("the failure sentence says what to do next and names the number — no code", () => {
    expect(HOTLINE_CALL_FAILED).toContain(CONTACT.hotlineLabel);
    expect(HOTLINE_CALL_FAILED).toMatch(/Vui lòng gọi/);
    expect(HOTLINE_CALL_FAILED).not.toMatch(/-\d{3,}|\bcode\b|lỗi \d/i);
  });
});

describe("the declaration Quản lý quyền reads names the hotline call", () => {
  it("openPhone's intro view says 'Gọi hotline' and the screens where it is pressed", () => {
    const row = KHAI_BAO_LOI_GOI.find((k) => k.api === "openPhone")!;
    expect(row.intro_app?.tinh_nang).toContain("Gọi hotline");
    for (const screen of ["Trang chủ", "Giải pháp", "Liên hệ"]) expect(row.intro_app?.man).toContain(screen);
    expect(row.intro_app?.de_lam_gi).toMatch(/hotline/);
    // The shared dossier fields describe the whole App ID, so they name it too.
    expect(row.tinh_nang).toContain("Gọi hotline");
  });
});
