import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * ZALO'S OWN TOP BAR ON A COMMUNE QR (owner, 08/10/2026) — `setCommuneActionBar` in `zalo-api.ts`, and the shell's
 * `nameNativeBar` / `COMMUNE_HEADER_TOP_COLOR` in `App.tsx`.
 *
 * What must hold:
 *   · the bar gets the commune's name, the header's top colour and white text — nothing else of the view changes
 *   · NO NAME, NO CALL: an empty or blank name leaves "ViHAT Group" rather than a guessed commune (fail closed)
 *   · a failure is not reported to `vihat-miniapp`: the call follows no tap (`content/ket-xuat-ho-so.ts`)
 *   · the colour is the stylesheet's, not a second opinion: it is read back from `styles.css`
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; the functions under test are real.
 */
const sdk = vi.hoisted(() => ({
  configAppView: vi.fn<(args: Record<string, unknown>) => Promise<void>>(),
}));
vi.mock("zmp-sdk", () => sdk);
const report = vi.hoisted(() => vi.fn(async () => {}));
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: report }));

import { COMMUNE_HEADER_TOP_COLOR, nameNativeBar } from "../../App";

import { setCommuneActionBar } from "./zalo-api";

const nodeFs = "node:fs";
const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
  readFileSync: (path: URL, encoding: "utf8") => string;
};
const styles = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");

beforeEach(() => {
  sdk.configAppView.mockReset().mockResolvedValue(undefined);
  report.mockClear();
});

describe("setCommuneActionBar", () => {
  it("title = the commune's name, the given colour, white text", async () => {
    expect(await setCommuneActionBar("  Xã Thử Nghiệm ", "#96060f")).toEqual({ kieu: "xong", du_lieu: undefined });
    expect(sdk.configAppView.mock.calls).toEqual([
      [{ actionBar: { title: "Xã Thử Nghiệm" }, headerColor: "#96060f", headerTextColor: "white" }],
    ]);
  });

  it("no name, or a colour that is not #rrggbb: nothing is called", async () => {
    for (const [name, colour] of [
      ["", "#96060f"],
      ["   ", "#96060f"],
      ["Xã Thử Nghiệm", "var(--xa-brand-dark)"],
      ["Xã Thử Nghiệm", ""],
    ] as const) {
      expect(await setCommuneActionBar(name, colour)).toEqual({ kieu: "khong-lay-duoc" });
    }
    expect(sdk.configAppView).not.toHaveBeenCalled();
  });

  it("a coded failure is a failure on the phone only — no report leaves for it", async () => {
    sdk.configAppView.mockRejectedValue(Object.assign(new Error("platform text"), { code: -1403 }));
    expect(await setCommuneActionBar("Xã Thử Nghiệm", "#96060f")).toMatchObject({ kieu: "khong-lay-duoc" });
    expect(report).not.toHaveBeenCalled();
  });
});

describe("App.tsx — the shell's half", () => {
  it("nameNativeBar passes the name and the header's top colour", async () => {
    nameNativeBar("Xã Thử Nghiệm");
    await vi.waitFor(() => expect(sdk.configAppView).toHaveBeenCalledTimes(1));
    expect(sdk.configAppView.mock.calls[0]![0]).toMatchObject({
      actionBar: { title: "Xã Thử Nghiệm" },
      headerColor: COMMUNE_HEADER_TOP_COLOR,
    });
  });

  it("the colour IS the stylesheet's --xa-brand-dark, the 0% stop at the top of the commune header", () => {
    const token = /--xa-brand-dark:\s*([^;]+);/.exec(styles)?.[1]?.trim();
    expect(token).toBe(COMMUNE_HEADER_TOP_COLOR);
    expect(styles).toMatch(/\.xa-header\s*\{[^}]*background:\s*linear-gradient\(160deg,\s*var\(--xa-brand-dark\)\s+0%/);
  });
});
