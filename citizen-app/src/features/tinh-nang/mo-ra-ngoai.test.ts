import { beforeEach, describe, expect, it, vi } from "vitest";

// Test files are outside the two-halves sweep (`ranh-gioi-hai-nua.test.ts` filters `.test.`), so this one
// may read the state half's rule to pin that the opener's local copy has not drifted from it.
import { readHttpsLink } from "../../cong-dan/api/hop-dong-cong-khai";

const { openWebPage } = vi.hoisted(() => ({
  openWebPage: vi.fn(async (_url: string) => ({ kieu: "xong" as const })),
}));
vi.mock("./zalo-api", () => ({ moTrangWeb: openWebPage }));

import { moRaNgoai } from "./mo-ra-ngoai";

/**
 * F2 (security review 02/10/2026): the door itself refuses a commune link that is not an absolute https
 * URL without a user part — whatever the caller checked or forgot to check.
 */
describe("moRaNgoai — commune links (lien-ket-xa) must be https", () => {
  beforeEach(() => openWebPage.mockClear());

  const REFUSED = [
    "http://ubnd.example.gov.vn/tin",
    "javascript:alert(1)",
    "/tin-tuc/1",
    "tin-tuc/1",
    "//ubnd.example.gov.vn/tin",
    "https://gov.vn@other.example/",
    "https://user:pass@ubnd.example.gov.vn/",
    "data:text/html,hi",
    "https://",
    "",
  ];

  for (const url of REFUSED) {
    it(`refuses ${JSON.stringify(url)} and never reaches the platform`, async () => {
      expect(await moRaNgoai("lien-ket-xa", url)).toBe(false);
      expect(openWebPage).not.toHaveBeenCalled();
    });
  }

  it("opens an absolute https link, passing the address unchanged", async () => {
    const url = "https://ubnd.example.gov.vn/tin/1?x=1#a";
    expect(await moRaNgoai("lien-ket-xa", url)).toBe(true);
    expect(openWebPage).toHaveBeenCalledWith(url);
  });

  it("still reports a platform refusal as failure", async () => {
    openWebPage.mockResolvedValueOnce({ kieu: "tu-choi" } as never);
    expect(await moRaNgoai("lien-ket-xa", "https://ubnd.example.gov.vn/")).toBe(false);
  });

  it("the local rule matches readHttpsLink on every case — a drifted copy is red", async () => {
    for (const url of [...REFUSED, "https://ubnd.example.gov.vn/tin/1"]) {
      openWebPage.mockClear();
      const opened = await moRaNgoai("lien-ket-xa", url);
      expect(opened, url).toBe(readHttpsLink(url) !== null && url !== "");
    }
  });

  it("video follows the same rule (02/10/2026): userinfo, http and javascript refused; https opens", async () => {
    for (const url of ["https://user@video.example/v", "https://gov.vn@other.example/v", "http://video.example/v", "javascript:alert(1)", "/v"]) {
      expect(await moRaNgoai("video", url), url).toBe(false);
    }
    expect(openWebPage).not.toHaveBeenCalled();
    expect(await moRaNgoai("video", "https://video.example/v/1")).toBe(true);
    expect(openWebPage).toHaveBeenCalledWith("https://video.example/v/1");
  });

  it("does not change other destinations: ma-qr still opens an http address", async () => {
    expect(await moRaNgoai("ma-qr", "http://vidu.vn")).toBe(true);
    expect(openWebPage).toHaveBeenCalledWith("http://vidu.vn");
  });
});
