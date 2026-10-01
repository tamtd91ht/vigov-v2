/// <reference types="vite/client" />
import { createElement, isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { type BaiTinXa as ArticleData, docBaiTin, docTrangTinXa, readBroadcastAudio } from "../api/hop-dong-cong-khai"; // vi-name-ok: existing contract names

import {
  audioLinkExpired,
  BroadcastPlayer,
  BroadcastPlayerView,
  createPlayback,
  durationWords,
  formatClock,
  type MediaLike,
  type PlayerPhase,
} from "./broadcast-player";
import { XA_TN } from "./noi-dung"; // vi-name-ok: existing strings module
import { HangTin, NewsArticle } from "./TinTucAppXa"; // vi-name-ok: existing screen file

/**
 * The Truyền thanh player (ADR 0067 §4, comms 026ae398): parsing the three audio fields, the clock, the controls'
 * accessible names, no autoplay, and the expired-link path — one re-read, then one sentence. The playback rules are
 * driven through `createPlayback` with a fake media element (no DOM here, as in the other commune-app tests). Every
 * URL is fake.
 */

const AUDIO = "https://storage.example.vn/content/t_x/a.mp3?X-Amz-Signature=abc";
const FRESH = "https://storage.example.vn/content/t_x/a.mp3?X-Amz-Signature=def";

const BROADCAST = {
  id: "tt-1",
  type: "truyen-thanh",
  title: "Bản tin sáng",
  summary: "",
  published_on: "2026-10-01",
  category_name: "",
};

const html = (el: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(el);

const page = (item: Record<string, unknown>) => docTrangTinXa({ items: [item], next_cursor: "", has_more: false })?.muc[0] ?? null;

function expand(node: unknown, out: ReactElement[] = []): ReactElement[] {
  if (Array.isArray(node)) {
    for (const n of node) expand(n, out);
    return out;
  }
  if (!isValidElement(node)) return out;
  if (typeof node.type === "function") return expand((node.type as (p: unknown) => unknown)(node.props), out);
  out.push(node);
  expand((node.props as { children?: unknown }).children, out);
  return out;
}

/** The visible words of an element's subtree (icons are `aria-hidden` SVG, so they add none). */
function words(node: unknown): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(words).join("");
  if (!isValidElement(node)) return "";
  return words((node.props as { children?: unknown }).children);
}

describe("parsing — audio_url · audio_url_expires_at · audio_duration_seconds", () => {
  it("a broadcast with all three keeps them, on the list and on the detail", () => {
    const wire = { ...BROADCAST, audio_url: AUDIO, audio_url_expires_at: "2026-10-01T03:15:00Z", audio_duration_seconds: 200 };
    const want = { url: AUDIO, expiresAt: "2026-10-01T03:15:00Z", durationSeconds: 200 };
    expect(page(wire)?.audio).toStrictEqual(want);
    expect(docBaiTin({ ...wire, body: "x" })?.audio).toStrictEqual(want);
  });

  it("no audio_url → no audio, and an older server's item is unchanged (no `audio` key at all)", () => {
    expect(page(BROADCAST)).not.toHaveProperty("audio");
    expect(page({ ...BROADCAST, audio_duration_seconds: 200 })).not.toHaveProperty("audio");
  });

  it("only an https link is kept; anything else drops the audio — LENIENT: the item still lists", () => {
    for (const v of ["http://storage.example.vn/a.mp3", "javascript:alert(1)", "/a.mp3", "https://x@storage.example.vn/a", "", 5, null, {}]) {
      const t = page({ ...BROADCAST, audio_url: v, audio_duration_seconds: 200 });
      expect(t, String(v)).not.toBeNull();
      expect(t, String(v)).not.toHaveProperty("audio");
    }
  });

  it("an odd expiry or duration is dropped, the link stays", () => {
    expect(readBroadcastAudio(AUDIO, "không phải ngày", 200)).toStrictEqual({ url: AUDIO, durationSeconds: 200 });
    expect(readBroadcastAudio(AUDIO, 5, 200)).toStrictEqual({ url: AUDIO, durationSeconds: 200 });
    for (const d of [0, -3, 1.5, "200", Number.NaN, null]) {
      expect(readBroadcastAudio(AUDIO, undefined, d), String(d)).toStrictEqual({ url: AUDIO });
    }
  });

  it("audio on any other type is DROPPED — the server gates it by type, so does this app", () => {
    for (const type of ["tin-tuc", "su-kien", "thong-bao", "video", "banner", undefined]) {
      expect(page({ ...BROADCAST, type, audio_url: AUDIO }), String(type)).not.toHaveProperty("audio");
    }
  });
});

describe("the clock", () => {
  it("formatClock: m:ss, h:mm:ss from an hour, junk → 0:00", () => {
    expect(formatClock(0)).toBe("0:00");
    expect(formatClock(5)).toBe("0:05");
    expect(formatClock(200)).toBe("3:20");
    expect(formatClock(599.9)).toBe("9:59");
    expect(formatClock(3725)).toBe("1:02:05");
    expect(formatClock(-4)).toBe("0:00");
    expect(formatClock(Number.NaN)).toBe("0:00");
    expect(formatClock(Number.POSITIVE_INFINITY)).toBe("0:00");
  });

  it("durationWords reads it in words", () => {
    expect(durationWords(200)).toBe("3 phút 20 giây");
    expect(durationWords(45)).toBe("45 giây");
    expect(durationWords(120)).toBe("2 phút");
    expect(durationWords(3725)).toBe("1 giờ 2 phút 5 giây");
    expect(durationWords(0)).toBe("0 giây");
  });

  it("audioLinkExpired: now ≥ expiry; no or unreadable expiry → not expired (the media error decides)", () => {
    const at = Date.parse("2026-10-01T03:15:00Z");
    expect(audioLinkExpired("2026-10-01T03:15:00Z", at - 1)).toBe(false);
    expect(audioLinkExpired("2026-10-01T03:15:00Z", at)).toBe(true);
    expect(audioLinkExpired(undefined, at)).toBe(false);
    expect(audioLinkExpired("x", at)).toBe(false);
  });
});

describe("the controls — words, sizes, no autoplay", () => {
  const view = (phase: PlayerPhase, total: number | null = 200) =>
    createElement(BroadcastPlayerView, { phase, elapsed: 65, total, onToggle: () => {}, onSkip: () => {} });

  it("names every control in words; the progress bar reads its value in words", () => {
    const els = expand(view("paused"));
    const buttons = els.filter((e) => e.type === "button");
    expect(buttons.map(words)).toEqual([XA_TN.broadcast_play, XA_TN.broadcast_back, XA_TN.broadcast_forward]);
    for (const b of buttons) expect((b.props as { className: string }).className).toContain("xa-player__nut");
    // No input control (phase1-collects-nothing.test.ts): the seek is the two buttons, the bar only shows.
    expect(els.some((e) => e.type === "input")).toBe(false);
    const bar = els.find((e) => e.type === "progress")!;
    expect(bar.props).toMatchObject({
      max: 200,
      value: 65,
      "aria-label": XA_TN.broadcast_position,
      "aria-valuetext": "1 phút 5 giây trên tổng 3 phút 20 giây",
    });
    expect(html(view("paused"))).toContain("1:05 / 3:20");
    expect(html(view("paused"))).toContain(`aria-label="${XA_TN.broadcast_player}"`);
  });

  it("while playing the button says Tạm dừng; loading says so; no total → no bar, elapsed alone", () => {
    expect(words(expand(view("playing")).find((e) => e.type === "button"))).toBe(XA_TN.broadcast_pause);
    expect(words(expand(view("loading")).find((e) => e.type === "button"))).toBe(XA_TN.broadcast_loading);
    const none = expand(view("playing", null));
    expect(none.some((e) => e.type === "progress")).toBe(false);
    expect(html(view("playing", null))).toContain("1:05");
    expect(html(view("playing", null))).not.toContain(" / ");
  });

  it("before the first tap the ±15 s buttons are off; a tap on one moves by 15 s; a failure is ONE sentence in an alert", () => {
    const skips: number[] = [];
    const live = expand(createElement(BroadcastPlayerView, { phase: "playing", elapsed: 65, total: 200, onToggle: () => {}, onSkip: (d) => skips.push(d) }));
    for (const b of live.filter((e) => e.type === "button").slice(1)) (b.props as { onClick: () => void }).onClick();
    expect(skips).toEqual([-15, 15]);
    const idle = expand(view("idle"));
    expect(idle.filter((e) => e.type === "button").slice(1).every((b) => (b.props as { disabled: boolean }).disabled)).toBe(true);
    expect(html(view("idle"))).not.toContain('role="alert"');
    const failed = html(view("failed"));
    expect(failed).toContain('role="alert"');
    expect(failed).toContain(XA_TN.broadcast_failed);
    expect(XA_TN.broadcast_failed).not.toMatch(/\d{3}|lỗi|error/i);
  });

  it("the element never starts by itself: no autoplay, preload none, NO src until the tap", () => {
    const out = html(createElement(BroadcastPlayer, { audio: { url: AUDIO, durationSeconds: 200 } }));
    const tag = /<audio[^>]*>/.exec(out)?.[0] ?? "";
    expect(tag).toContain('preload="none"');
    expect(tag).not.toMatch(/autoplay/i);
    expect(tag).not.toMatch(/\bsrc=/);
    expect(tag).not.toMatch(/\bcontrols\b/);
    expect(out).not.toContain("X-Amz-Signature");
  });
});

/** A media element that records what it was told. `play` resolves unless `reject` names an error. */
function fakeMedia() {
  const m = {
    src: "",
    currentTime: 0,
    reject: null as string | null,
    play: vi.fn(async () => {
      if (m.reject !== null) throw Object.assign(new Error(m.reject), { name: m.reject });
    }),
    pause: vi.fn(),
    load: vi.fn(),
  };
  return m satisfies MediaLike;
}

const NOW = Date.parse("2026-10-01T03:00:00Z");
const LIVE = { url: AUDIO, expiresAt: "2026-10-01T03:15:00Z", durationSeconds: 200 };
const EXPIRED = { url: AUDIO, expiresAt: "2026-10-01T02:59:00Z", durationSeconds: 200 };

function setup(initial: typeof LIVE, refresh?: () => Promise<typeof LIVE | null>) {
  const media = fakeMedia();
  const phases: PlayerPhase[] = [];
  const refreshSpy = refresh === undefined ? undefined : vi.fn(refresh);
  const c = createPlayback({ media: () => media, initial, refresh: refreshSpy, now: () => NOW, onPhase: (p) => phases.push(p) });
  return { media, phases, c, refresh: refreshSpy };
}

const tick = () => new Promise((r) => setTimeout(r, 0));

describe("playback — the expired link is re-read ONCE, then one sentence", () => {
  it("a live link: the tap sets the source and plays; nothing is re-read", async () => {
    const { media, c, refresh } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    expect(media.src).toBe("");
    await c.toggle();
    expect(media.src).toBe(AUDIO);
    expect(media.play).toHaveBeenCalledTimes(1);
    expect(refresh).not.toHaveBeenCalled();
    c.playing();
    expect(c.phase()).toBe("playing");
    await c.toggle();
    expect(media.pause).toHaveBeenCalled();
    expect(c.phase()).toBe("paused");
  });

  it("expired by the clock: the tap re-reads the item first, then plays the FRESH link", async () => {
    const { media, c, refresh } = setup(EXPIRED, async () => ({ ...LIVE, url: FRESH }));
    await c.toggle();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(media.src).toBe(FRESH);
    expect(media.load).toHaveBeenCalledTimes(1);
    expect(media.play).toHaveBeenCalledTimes(1);
  });

  it("the media fails (a 403 is a plain media error): one re-read, retry; it fails AGAIN → failed, no second re-read", async () => {
    const { media, c, refresh, phases } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    await c.toggle();
    await c.failed(); // the element's `error` event on the old link
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(media.src).toBe(FRESH);
    expect(media.play).toHaveBeenCalledTimes(2);
    await c.failed(); // the fresh link fails too
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(c.phase()).toBe("failed");
    expect(phases.at(-1)).toBe("failed");
  });

  it("ONE failure is counted once: play()'s rejection is not a second signal beside the element's error event", async () => {
    const { media, c, refresh } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    media.reject = "NotSupportedError";
    await c.toggle();
    await tick(); // play() rejected — not acted on
    expect(refresh).not.toHaveBeenCalled();
    await c.failed(); // the element's own `error` event
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(media.src).toBe(FRESH);
  });

  it("the item cannot be re-read (or has no audio any more) → failed", async () => {
    const { media, c, refresh } = setup(EXPIRED, async () => null);
    await c.toggle();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(media.play).not.toHaveBeenCalled();
    expect(c.phase()).toBe("failed");
    const rejected = setup(EXPIRED, async () => Promise.reject(new Error("network")));
    await rejected.c.toggle();
    expect(rejected.c.phase()).toBe("failed");
  });

  it("no re-reader at all → an expired link fails at once, never a loop", async () => {
    const { media, c } = setup(EXPIRED);
    await c.toggle();
    expect(media.play).not.toHaveBeenCalled();
    expect(c.phase()).toBe("failed");
  });

  it("after the sentence, a NEW tap is a new attempt with its own single re-read", async () => {
    const { c, refresh } = setup(EXPIRED, async () => null);
    await c.toggle();
    expect(c.phase()).toBe("failed");
    await c.toggle();
    expect(refresh).toHaveBeenCalledTimes(2);
  });

  it("audio that played resets the count: a later expiry gets its own re-read", async () => {
    const { c, refresh } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    await c.toggle();
    await c.failed();
    c.playing();
    await c.failed();
    expect(refresh).toHaveBeenCalledTimes(2);
    expect(c.phase()).not.toBe("failed");
  });

  it("the re-read resumes where the citizen was", async () => {
    const { media, c } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    await c.toggle();
    c.playing();
    c.timeUpdate(42);
    await c.failed();
    c.metadataLoaded();
    expect(media.currentTime).toBe(42);
  });

  it("a failure while PAUSED does not re-read and start playing on its own", async () => {
    const { media, c, refresh } = setup(LIVE, async () => ({ ...LIVE, url: FRESH }));
    await c.toggle();
    c.playing();
    c.stop();
    await c.failed();
    expect(refresh).not.toHaveBeenCalled();
    expect(media.play).toHaveBeenCalledTimes(1);
  });

  it("leaving during the re-read (unmount, background) → it never starts playing", async () => {
    let answer: (v: typeof LIVE) => void = () => {};
    const { media, c } = setup(EXPIRED, () => new Promise((r) => (answer = r)));
    const tap = c.toggle();
    c.stop();
    answer({ ...LIVE, url: FRESH });
    await tap;
    expect(media.play).not.toHaveBeenCalled();
    expect(c.phase()).toBe("paused");
  });

  it("a webview refusing play() after the network wait is not a failure: the button offers Nghe again", async () => {
    const { media, c } = setup(EXPIRED, async () => ({ ...LIVE, url: FRESH }));
    media.reject = "NotAllowedError";
    await c.toggle();
    await tick();
    expect(c.phase()).toBe("paused");
  });
});

describe("on the screens", () => {
  const ARTICLE: ArticleData = {
    id: "tt-1",
    tieu_de: "Bản tin sáng",
    tom_tat: "",
    ngay_dang: "2026-10-01",
    chuyen_muc: "",
    type: "truyen-thanh",
    noi_dung: "Nội dung bản tin.",
    audio: LIVE,
  };
  const article = (bai: ArticleData) =>
    html(createElement(NewsArticle, { bai, coverFailed: false, onCoverFail: () => {}, ds: [] }));

  it("a broadcast's article carries the player; without audio there is none", () => {
    expect(article(ARTICLE)).toContain(`aria-label="${XA_TN.broadcast_player}"`);
    const { audio: _drop, ...noAudio } = ARTICLE;
    expect(article(noAudio)).not.toContain("xa-player");
    expect(article(noAudio)).not.toContain("<audio");
  });

  it("a list row shows the length — 3:20 for the eye, the words for a screen reader", () => {
    const row = html(createElement(HangTin, { tin: ARTICLE, onMo: () => {}, today: "2026-10-01" }));
    expect(row).toContain("3:20");
    expect(row).toContain("Bản tin dài 3 phút 20 giây");
    const { audio: _drop, ...noAudio } = ARTICLE;
    expect(html(createElement(HangTin, { tin: noAudio, onMo: () => {}, today: "2026-10-01" }))).not.toContain("xa-hang-tin__duration");
  });
});
