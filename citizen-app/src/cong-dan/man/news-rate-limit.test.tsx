import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * 429 on the public commune-news reads (owner, 02/10/2026: 120/min per client in comms; over it → 429
 * `rate_limited` with `Retry-After` in seconds). What the citizen channel must do with it:
 *
 *   - a branch of its own (`rate-limited`), carrying the wait — never the generic "system is broken" sentence;
 *   - a calm sentence that says what to do next, never a code;
 *   - NO automatic retry anywhere, except ONE delayed retry for the home banner strip, which stays silent;
 *   - the broadcast player's link refresh treats it as its existing failure.
 *
 * The petition routes and the identity public reads have no 429 in their contract: there it stays `loi-may-chu`.
 * The session is faked for THIS file only (as `goi-vigov.test.tsx` does) so the petition route can be reached.
 */
vi.mock("../api/phien-vigov", () => ({ layPhienViGov: () => ({ token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" }) }));

import { baiTinCuaXa, communeBanners, newsCategories, phanAnhCuaToi, tinCuaXa, traXaTheoTenMien } from "../api/goi-vigov";
import { type CommuneBannerItem } from "../api/hop-dong-cong-khai";
import { createPlayback, type MediaLike, type PlayerPhase } from "./broadcast-player";
import { TIN_XA } from "./noi-dung"; // vi-name-ok: existing strings module
import { audioOfReread, NewsListBody } from "./TinTucAppXa"; // vi-name-ok: existing screen file
import { errorSentence, sauKhiTaiBai, sauKhiTaiTin, ThanTinXa, TIN_DAU, waitWords } from "./TinTucXaScreen"; // vi-name-ok: existing screen file
import { loadBanners } from "./TrangXa"; // vi-name-ok: existing screen file

const DOMAIN = "xa-thu.vigov.example";
const OWNER_SENTENCE = "Bạn thao tác hơi nhanh, vui lòng thử lại sau ít giây.";

type FakeAnswer = { status: number; ok: boolean; json: () => Promise<unknown>; headers?: { get: (n: string) => string | null } };

function answer(status: number, body: unknown, retryAfter?: string): FakeAnswer {
  const a: FakeAnswer = { status, ok: status >= 200 && status < 300, json: async () => body };
  if (retryAfter !== undefined) a.headers = { get: (n) => (n.toLowerCase() === "retry-after" ? retryAfter : null) };
  return a;
}

/** Stubs `fetch` with a fixed sequence (the last answer repeats); returns the list of URLs called. */
function stubFetch(...answers: FakeAnswer[]): string[] {
  const calls: string[] = [];
  vi.stubGlobal("fetch", (url: string) => {
    calls.push(url);
    return Promise.resolve(answers[Math.min(calls.length - 1, answers.length - 1)]!);
  });
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

describe("429 on the public news reads → `rate-limited` with the server's wait", () => {
  it("every comms read maps it, reads `Retry-After` as seconds, and calls ONCE (no retry in the client)", async () => {
    const calls = stubFetch(answer(429, { code: "rate_limited" }, "12"));
    expect(await tinCuaXa(DOMAIN, "")).toEqual({ kieu: "rate-limited", retryAfterSeconds: 12 });
    expect(await newsCategories(DOMAIN, "tin-tuc")).toEqual({ kieu: "rate-limited", retryAfterSeconds: 12 });
    expect(await communeBanners(DOMAIN)).toEqual({ kieu: "rate-limited", retryAfterSeconds: 12 });
    expect(await baiTinCuaXa(DOMAIN, "tin-01")).toEqual({ kieu: "rate-limited", retryAfterSeconds: 12 });
    expect(await tinCuaXa(DOMAIN, "", "truyen-thanh")).toEqual({ kieu: "rate-limited", retryAfterSeconds: 12 });
    expect(calls.length).toBe(5);
  });

  it("no header, an HTTP-date, junk or a negative → the wait is unknown (`null`), never a guess", async () => {
    for (const h of [undefined, "Wed, 21 Oct 2026 07:28:00 GMT", "soon", "-5", "1.5"]) {
      stubFetch(answer(429, {}, h));
      expect(await tinCuaXa(DOMAIN, ""), String(h)).toEqual({ kieu: "rate-limited", retryAfterSeconds: null });
    }
  });

  it("routes with no 429 in their contract keep `loi-may-chu` (identity lookup, my petitions)", async () => {
    stubFetch(answer(429, {}, "12"));
    expect((await traXaTheoTenMien(DOMAIN)).kieu).toBe("loi-may-chu");
    expect((await phanAnhCuaToi("")).kieu).toBe("loi-may-chu");
  });
});

describe("the sentence — calm, with the wait, never a code", () => {
  it("unknown wait → the owner's sentence exactly; a known one adds how long, then the next step", () => {
    expect(errorSentence("rate-limited", null)).toBe(OWNER_SENTENCE);
    expect(errorSentence("rate-limited", 0)).toBe(OWNER_SENTENCE);
    const s = errorSentence("rate-limited", 12);
    expect(s.startsWith(OWNER_SENTENCE)).toBe(true);
    expect(s).toContain("12 giây");
    expect(s).toContain("Thử lại");
    expect(s).not.toMatch(/429|rate_limited/);
    expect(waitWords(300)).toBe("5 phút");
  });

  it("is not the server-failure sentence, and the other branches are unchanged", () => {
    expect(errorSentence("rate-limited", 12)).not.toBe(TIN_XA.loi_may_chu);
    expect(errorSentence("loi-may-chu")).toBe(TIN_XA.loi_may_chu);
    expect(errorSentence("loi-mang")).toBe(TIN_XA.loi_mang);
  });

  it("the list and the article carry the branch and the wait; the screen shows the sentence and a Thử lại button", () => {
    const ds = sauKhiTaiTin(TIN_DAU, { kieu: "rate-limited", retryAfterSeconds: 20 });
    expect(ds.loi).toBe("rate-limited");
    expect(ds.retryAfterSeconds).toBe(20);
    expect(ds.dang_tai).toBe(false);
    const noop = () => {};
    const app = renderToStaticMarkup(createElement(NewsListBody, { ds, onMo: noop, onTai: noop, empty: "" }));
    expect(app).toContain("20 giây");
    expect(app).toContain(TIN_XA.nut_thu_lai);
    const shared = renderToStaticMarkup(createElement(ThanTinXa, { ds, onMo: noop, onTai: noop }));
    expect(shared).toContain("20 giây");
    expect(sauKhiTaiBai({ kieu: "rate-limited", retryAfterSeconds: null })).toEqual({
      kieu: "loi",
      loi: "rate-limited",
      retryAfterSeconds: null,
    });
  });
});

describe("home banner strip — at most ONE delayed retry, and silent either way", () => {
  const ITEMS: readonly CommuneBannerItem[] = [{ id: "b1", title: "T", imageUrl: "https://anh.vidu.example/1.png" }];

  it("429 then 200 → waits the server's seconds once, then shows the strip", async () => {
    const load = vi.fn().mockResolvedValueOnce({ kieu: "rate-limited", retryAfterSeconds: 12 }).mockResolvedValueOnce({ kieu: "xong", gia_tri: ITEMS });
    const wait = vi.fn(async () => {});
    expect(await loadBanners(load, wait)).toEqual(ITEMS);
    expect(load).toHaveBeenCalledTimes(2);
    expect(wait).toHaveBeenCalledTimes(1);
    expect(wait).toHaveBeenCalledWith(12_000);
  });

  it("429 twice → bundled picture (`null`), and NO third call — never a loop", async () => {
    const load = vi.fn().mockResolvedValue({ kieu: "rate-limited", retryAfterSeconds: 3 });
    const wait = vi.fn(async () => {});
    expect(await loadBanners(load, wait)).toBeNull();
    expect(load).toHaveBeenCalledTimes(2);
    expect(wait).toHaveBeenCalledTimes(1);
  });

  it("no wait named → 5 s; a long one is capped at 60 s", async () => {
    const wait = vi.fn(async () => {});
    await loadBanners(vi.fn().mockResolvedValue({ kieu: "rate-limited", retryAfterSeconds: null }), wait);
    await loadBanners(vi.fn().mockResolvedValue({ kieu: "rate-limited", retryAfterSeconds: 3600 }), wait);
    expect(wait.mock.calls).toEqual([[5_000], [60_000]]);
  });

  it("any other failure → bundled picture at once, no wait, no retry", async () => {
    for (const kieu of ["loi-may-chu", "loi-mang", "tam-ngung", "khong-hop-le"] as const) {
      const load = vi.fn().mockResolvedValue({ kieu });
      const wait = vi.fn(async () => {});
      expect(await loadBanners(load, wait)).toBeNull();
      expect(load).toHaveBeenCalledTimes(1);
      expect(wait).not.toHaveBeenCalled();
    }
  });

  it("through the real client: 429 then 429 is exactly two network calls", async () => {
    const calls = stubFetch(answer(429, {}, "1"));
    expect(await loadBanners(() => communeBanners(DOMAIN), async () => {})).toBeNull();
    expect(calls.length).toBe(2);
  });
});

describe("broadcast link refresh — a 429 is the existing failure, not a hammer", () => {
  it("the re-read maps 429 to `null`", () => {
    expect(audioOfReread({ kieu: "rate-limited", retryAfterSeconds: 10 })).toBeNull();
  });

  it("an expired link + 429 on the re-read → `failed` after ONE call; the element's error does not re-ask", async () => {
    const calls = stubFetch(answer(429, {}, "10"));
    const phases: PlayerPhase[] = [];
    const media: MediaLike = { src: "", currentTime: 0, play: async () => {}, pause: () => {}, load: () => {} };
    const p = createPlayback({
      media: () => media,
      initial: { url: "https://media.vidu.example/a.mp3", expiresAt: "2026-10-02T00:00:00Z" },
      refresh: async () => audioOfReread(await baiTinCuaXa(DOMAIN, "tin-01")),
      now: () => Date.parse("2026-10-02T01:00:00Z"),
      onPhase: (ph) => phases.push(ph),
    });
    await p.toggle();
    expect(phases.at(-1)).toBe("failed");
    expect(calls.length).toBe(1);
    await p.failed();
    expect(calls.length).toBe(1);
  });
});
