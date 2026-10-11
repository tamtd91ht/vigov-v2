import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

/**
 * THE COMMUNE APP AFTER "LÀM HẾT THEO PROTOTYPE" (owner 11/10/2026) — what this pass added that no older test pins:
 * the prototype's date formats, the Video and Truyền thanh libraries, the minimise bubble, the directory search,
 * the shared-article link, the home header and frame, and the hard rules each of them must still keep (the 16px /
 * 44px floors, no client-side deadline, nothing personal in a link, no new storage).
 */

import { articleShareLink, sharedArticleId } from "../../App";
import type { TinXaTomTat } from "../api/hop-dong-cong-khai";
import { formatDate, formatDateTime, formatDayGroup, formatDeadline } from "../../lib/vn-format";
import { ThanDanhBaXa } from "./DanhBaXa";
import { BroadcastBar, BroadcastRow, groupByDay, nextSpeed, speedLabel, VideoTile } from "./media-screens";
import { clampToFrame, MinimiseBubble } from "./minimise-bubble";
import { COMMUNE_OFFLINE_NOTICE, COMMUNE_SEND_ERRORS, LOI_GUI, XA_GIAO_DIEN, XA_PA, XA_TN } from "./noi-dung";
import { BannerStrip } from "./TrangXa";

const html = (el: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(el);
const noop = () => {};

describe("dates, the prototype's way, pinned to +07", () => {
  it("formatDate · formatDateTime: no leading zeros in the date; the hour in Vietnam whatever the phone's zone", () => {
    expect(formatDate("2026-08-24T01:00:00Z")).toBe("24/8/2026");
    expect(formatDateTime("2026-08-22T07:30:00Z")).toBe("14:30 ngày 22/8/2026");
    // 18:30 UTC on the 21st is 01:30 on the 22nd in Vietnam.
    expect(formatDateTime("2026-08-21T18:30:00Z")).toBe("01:30 ngày 22/8/2026");
    expect(formatDate("không phải ngày")).toBeNull();
  });

  it("formatDeadline: '15h ngày 2/10' — and the minutes when there are some, never a deadline earlier than stored", () => {
    expect(formatDeadline("2026-10-02T08:00:00Z")).toBe("15h ngày 2/10");
    expect(formatDeadline("2026-10-02T08:30:00Z")).toBe("15h30 ngày 2/10");
    expect(formatDeadline("x")).toBeNull();
  });

  it("formatDayGroup: today, yesterday, else the weekday and the date — by DAY, across a month", () => {
    expect(formatDayGroup("2026-10-11", "2026-10-11")).toBe("Hôm nay");
    expect(formatDayGroup("2026-09-30", "2026-10-01")).toBe("Hôm qua");
    expect(formatDayGroup("2026-08-25", "2026-10-11")).toBe("Thứ Ba, 25/8/2026");
    expect(formatDayGroup("2026-02-30", "2026-10-11")).toBeNull();
  });
});

const item = (id: string, over: Partial<TinXaTomTat> = {}): TinXaTomTat => ({
  id,
  tieu_de: `Bản tin ${id}`,
  tom_tat: "Tóm tắt bản tin",
  chuyen_muc: "Thời sự",
  ngay_dang: "2026-10-11",
  type: "truyen-thanh",
  ...over,
});

describe("Video (`VideoPage.tsx`)", () => {
  it("a tile: the cover with a Play mark, the title, 'x lượt xem' when counted — and no invented duration", () => {
    const tile = html(
      createElement(VideoTile, {
        tin: item("v1", { type: "video", imageUrl: "https://media.vigov.example/t_TENANT/v1.jpg", viewCount: 1284 }),
        onMo: noop,
      }),
    );
    expect(tile).toContain(`aria-label="${XA_TN.video_open("Bản tin v1")}"`);
    expect(tile).toContain('class="xa-video-tile__play"');
    expect(tile).toContain("1.284 lượt xem");
    expect(tile).not.toMatch(/\d+:\d\d/);
    const noCount = html(createElement(VideoTile, { tin: item("v2", { type: "video" }), onMo: noop }));
    expect(noCount).not.toContain("lượt xem");
    expect(noCount).not.toContain("<img");
  });

  it("the empty sentences are the prototype's, verbatim", () => {
    expect(XA_TN.video_trong).toBe("Chưa có video trong chủ đề này");
    expect(XA_TN.video_empty_hint).toBe("Bà con chọn chủ đề khác hoặc xem mục Tất cả.");
    expect(XA_TN.video_topics).toBe("Chủ đề");
  });
});

describe("Truyền thanh (`BroadcastPage.tsx`)", () => {
  it("groups by DAY, newest group first, the server's order inside a day", () => {
    const groups = groupByDay(
      [item("1"), item("2"), item("3", { ngay_dang: "2026-10-10" }), item("4", { ngay_dang: "2026-08-25" })],
      "2026-10-11",
    );
    expect(groups.map((g) => [g.day, g.items.map((t) => t.id)])).toEqual([
      ["Hôm nay", ["1", "2"]],
      ["Hôm qua", ["3"]],
      ["Thứ Ba, 25/8/2026", ["4"]],
    ]);
  });

  it("a row with a file: the round button plays it HERE, named by title and day; 'chuyên mục — thời lượng'; the summary", () => {
    const tin = item("1", { audio: { url: "https://store.example/a.mp3", durationSeconds: 755 } });
    const row = html(createElement(BroadcastRow, { tin, active: false, playing: false, onPlay: noop, onOpen: noop }));
    expect(row).toContain('aria-label="Phát Bản tin 1 ngày 11/10/2026"');
    expect(row).toContain("Thời sự — 12 phút 35 giây");
    expect(row).toContain("Tóm tắt bản tin");
    expect(row).not.toContain("lượt xem");
    // The link is a presigned URL: it is NOT in the markup — it goes to `<audio src>` only, on the tap.
    expect(row).not.toContain("store.example");
    const playing = html(createElement(BroadcastRow, { tin, active: true, playing: true, onPlay: noop, onOpen: noop }));
    expect(playing).toContain('aria-label="Tạm dừng Bản tin 1 ngày 11/10/2026"');
    expect(playing).toContain("xa-broadcast-row--active");
  });

  it("a row with no file opens the item instead — never a play button that cannot play", () => {
    const row = html(createElement(BroadcastRow, { tin: item("2"), active: false, playing: false, onPlay: noop, onOpen: noop }));
    expect(row).not.toContain("xa-broadcast-row__play");
    expect(row).toContain(`aria-label="${XA_TN.broadcast_open_item("Bản tin 2")}"`);
  });

  it("the bar: the seek slider named in words, ±15 s with '15 giây' under each, the speed; every control a button or the slider", () => {
    const tin = item("1", { audio: { url: "https://store.example/a.mp3", durationSeconds: 120 } });
    const bar = html(
      createElement(BroadcastBar, {
        tin,
        phase: "playing",
        elapsed: 30,
        total: 120,
        speed: 1.25,
        onToggle: noop,
        onSeek: noop,
        onSpeed: noop,
      }),
    );
    expect(bar).toMatch(/<input class="xa-seek" type="range" min="0" max="120" step="1"[^>]*aria-label="Vị trí đang phát"/);
    expect(bar).toContain('aria-valuetext="30 giây trên tổng 2 phút"');
    expect(bar.match(/>15 giây</g)).toHaveLength(2);
    expect(bar).toContain('aria-label="Tua lùi 15 giây"');
    expect(bar).toContain('aria-label="Tua tới 15 giây"');
    expect(bar).toContain('aria-label="Tốc độ phát 1,25 lần, chạm để đổi"');
    expect(bar).toContain(">1,25x</button>");
    expect(bar).toContain(`aria-label="${XA_TN.broadcast_pause}"`);
  });

  it("speeds cycle 0.75 → 1 → 1.25 → 0.75", () => {
    expect(nextSpeed(0.75)).toBe(1);
    expect(nextSpeed(1)).toBe(1.25);
    expect(nextSpeed(1.25)).toBe(0.75);
    expect(speedLabel(0.75)).toBe("0,75");
  });

  it("the title and the empty sentences are the prototype's", () => {
    expect(XA_TN.truyen_thanh_tieu_de).toBe("Truyền thanh xã");
    expect(XA_TN.broadcast_empty).toBe("Chưa có bản tin nào");
    expect(XA_TN.broadcast_category_empty("Thời sự")).toBe("Chưa có bản tin nào trong chuyên mục Thời sự");
  });
});

describe("the minimise bubble (`MinimiseBubble.tsx`)", () => {
  it("a 44px round button named in words; its position is kept on the screen, never off it", () => {
    const bubble = html(createElement(MinimiseBubble, { onMinimise: noop }));
    expect(bubble).toMatch(/^<button type="button" class="xa-minimise" aria-label="Thu nhỏ ứng dụng">/);
    expect(clampToFrame({ x: -50, y: 9999 }, 390, 800)).toEqual({ x: 10, y: 800 - 44 - 10 });
    expect(clampToFrame({ x: 100, y: 100 }, 390, 800)).toEqual({ x: 100, y: 100 });
  });

  it("only where the shell gives the platform call, and nothing written on the phone", async () => {
    const files = import.meta.glob(["./minimise-bubble.tsx", "./TrangXa.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    expect(files["./TrangXa.tsx"]).toContain("{props.minimize !== undefined && <MinimiseBubble onMinimise={props.minimize} />}");
    const code = files["./minimise-bubble.tsx"]!.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
    expect(code).not.toMatch(/localStorage|sessionStorage|indexedDB|zmp-sdk/);
  });
});

describe("the directory (`DirectoryPage.tsx`)", () => {
  it("the search is there even while the list loads: the magnifier, the label for screen readers, the example", () => {
    const loading = html(createElement(ThanDanhBaXa, { trang: { kieu: "dang-tai" }, onTai: noop }));
    expect(loading).toContain('<label class="xa-sr-only" for="xa-tim-can-bo">Tìm theo tên, chức danh hoặc số điện thoại</label>');
    expect(loading).toContain('placeholder="Tìm tên, chức danh hoặc số máy"');
    expect(loading).toContain('class="xa-search__icon"');
    expect(XA_TN.directory_title).toBe("Danh bạ chính quyền");
    expect(XA_GIAO_DIEN.khong_thay_can_bo).toBe("Không tìm thấy ai phù hợp");
    expect(XA_GIAO_DIEN.goi).toBe("GỌI");
  });
});

describe("the shared article link (ADR 0047 G6)", () => {
  it("zalo.me/s/<App ID> with the article's public id — nothing of the citizen's", () => {
    expect(articleShareLink("1234567890", "01JABCDEF")).toBe("https://zalo.me/s/1234567890/?article=01JABCDEF");
    expect(articleShareLink("1234567890", "a b/c")).toBe("https://zalo.me/s/1234567890/?article=a%20b%2Fc");
  });

  it("the launch parameter steers only: a plain bounded id, or nothing", () => {
    expect(sharedArticleId({ article: "01JABCDEF" })).toBe("01JABCDEF");
    expect(sharedArticleId({ article: " 01JABCDEF " })).toBe("01JABCDEF");
    for (const bad of ["", "a b", "../x", "x".repeat(65), "<script>"]) expect(sharedArticleId({ article: bad }), bad).toBeNull();
    expect(sharedArticleId({})).toBeNull();
  });
});

describe("home and frame", () => {
  it("the banner strip: full-width slides, dots under it — the current one longer, decoration only", () => {
    const items = [
      { id: "b1", title: "Ảnh một", imageUrl: "https://media.vigov.example/t_T/b1.jpg" },
      { id: "b2", title: "Ảnh hai", imageUrl: "https://media.vigov.example/t_T/b2.jpg" },
    ];
    const strip = html(createElement(BannerStrip, { items, canOpenWeb: false, onScreen: noop, onWeb: noop, onFail: noop, current: 1 }));
    expect(strip).toContain('<div class="xa-banner-dots" aria-hidden="true">');
    expect(strip.match(/xa-banner-dots__dot--on/g)).toHaveLength(1);
    expect(strip).toMatch(/<span class="xa-banner-dots__dot"><\/span><span class="xa-banner-dots__dot xa-banner-dots__dot--on">/);
    const one = html(createElement(BannerStrip, { items: [items[0]!], canOpenWeb: false, onScreen: noop, onWeb: noop, onFail: noop }));
    expect(one).not.toContain("xa-banner-dots");
  });

  it("the section is 'Tin mới', the bottom bar 'Điều hướng chính'; no office block on home", () => {
    expect(XA_GIAO_DIEN.muc_tin_moi).toBe("Tin mới");
    expect(XA_GIAO_DIEN.thanh_tab).toBe("Điều hướng chính");
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    const home = src.slice(src.indexOf("function TrangChuXa("), src.indexOf("/* ═════════════════════════════════ CÁC TAB KHÁC"));
    expect(home).not.toContain("<CommuneOffice");
    expect(home).toContain("subtitle={ho_ten !== null ? XA_TN.xin_chao_ten(ho_ten) : undefined}");
  });

  it("the commune frame's tokens: the prototype's page, border, text, muted, card; the text floor and the large-text scale", async () => {
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8").replace(/\r\n/g, "\n");
    expect(css).toContain("--xa-nen: #f4f8fb;");
    expect(css).toContain("--xa-vien: #dde7ef;");
    expect(css).toMatch(/\.xa-app \{[^}]*--ink: #1e3a52;[^}]*--ink-muted: #566e8b;/);
    expect(css).toMatch(/\.xa-the \{[^}]*border-radius: 16px;[^}]*box-shadow: 0 1px 4px rgba\(15, 42, 67, 0\.07\);/);
    // "Cỡ chữ lớn" scales UP from the floor (17px body, 16px small) by 20% — never below it.
    expect(css).toMatch(/\.xa-co-chu--lon \{[^}]*--text-body: calc\(17px \* 1\.2\);[^}]*--text-small: calc\(16px \* 1\.2\);/);
    // The minimise bubble and the photo "X" keep the 44px floor.
    expect(css).toMatch(/\.xa-minimise \{[^}]*width: 44px;[^}]*height: 44px;/);
    expect(css).toMatch(/\.xa-photo-tile__remove \{[^}]*width: calc\(var\(--tap-min\) \+ 4px\);/);
  });
});

describe("Gửi phản ánh — what the prototype changed, and what it did not", () => {
  it("the offline box and the commune's 'bà con' refusals keep the retry rule of `LOI_GUI`", () => {
    expect(COMMUNE_OFFLINE_NOTICE).toBe(
      "Chưa gửi được do mất mạng. Phản ánh của bà con đã được lưu, sẽ tự gửi khi có mạng trở lại.",
    );
    for (const [k, v] of Object.entries(COMMUNE_SEND_ERRORS)) {
      expect(v.co_the_gui_lai, k).toBe(LOI_GUI[k as keyof typeof LOI_GUI].co_the_gui_lai);
      expect(v.cau, k).not.toMatch(/\bbạn\b|\bBạn\b/);
    }
  });

  it("the network coming back resends the SAME attempt — never a new key", () => {
    const src = Object.values(import.meta.glob("./PhanAnhAppXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(src).toMatch(/failure !== "loi-mang" \|\| attempt === null \|\| sending/);
    expect(src).toContain("const resend = () => void sendRef.current(attempt);");
    expect(src).toContain('window.addEventListener("online", resend);');
  });

  it("the done screen's words and the deadline come from the server only (no clock read, no arithmetic)", () => {
    expect(XA_PA.done_received("Xã Thử")).toBe("Xã Thử đã nhận được phản ánh của bà con.");
    expect(XA_PA.resolve_by("15h ngày 2/10")).toBe("Dự kiến xử lý xong trước 15h ngày 2/10");
    const fmt = Object.values(import.meta.glob("../../lib/vn-format.ts", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(fmt).not.toMatch(/Date\.now\(\)|new Date\(\)/);
  });
});
