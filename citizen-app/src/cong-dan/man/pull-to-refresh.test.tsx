import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQuaDanhSach } from "../api/goi-vigov"; // vi-name-ok: existing contract type, imported
import type { TinXaTomTat } from "../api/hop-dong-cong-khai"; // vi-name-ok: existing contract type, imported
import { directoryAfterRefresh } from "./DanhBaXa";
import { loadDisasterAlerts } from "./disaster-alerts";
import { TrangCon } from "./khung-xa";
import { PULL_REFRESH } from "./noi-dung";
import { type MyPetitions, petitionsAfterRefresh } from "./PhanAnhAppXa";
import { PULL_TRIGGER_PX, PullIndicator, pullDistance, runRefreshers } from "./pull-to-refresh";
import { afterNewsRefresh } from "./TinTucAppXa";
import { TIN_DAU } from "./TinTucXaScreen"; // vi-name-ok: existing shared state constant

/**
 * Customer sheet 12/10/2026, rows 2 and 8: pull to refresh on every commune-app screen, and the petition list re-read
 * on a return to its tab. These are the PURE halves — what a refresh does to what is on screen. The gesture itself,
 * mounted, is in `hieu-ung-khong-phien.test.tsx` (tab re-entry, the pull on Phản ánh) and `news-view-count.test.tsx`
 * (an article's pull adds no view). Every value below is invented test data.
 */

const item = (id: string): TinXaTomTat => ({ id, tieu_de: `Tin ${id}`, tom_tat: "", ngay_dang: "2026-10-01", chuyen_muc: "", type: "tin-tuc" });

describe("the gesture's arithmetic", () => {
  it("the band trails the finger at half its travel, never negative, capped", () => {
    expect(pullDistance(-40)).toBe(0);
    expect(pullDistance(0)).toBe(0);
    expect(pullDistance(100)).toBe(50);
    expect(pullDistance(1000)).toBe(96);
    // A deliberate pull reaches the trigger; a scroll jitter does not.
    expect(pullDistance(PULL_TRIGGER_PX * 2)).toBe(PULL_TRIGGER_PX);
    expect(pullDistance(40)).toBeLessThan(PULL_TRIGGER_PX);
  });

  it("all loaders run; ONE failure — or one that throws — makes the pull report failure", async () => {
    const ran: string[] = [];
    const ok = (n: string) => async () => (ran.push(n), true);
    expect(await runRefreshers([ok("a"), ok("b")])).toBe(true);
    expect(ran).toEqual(["a", "b"]);
    expect(await runRefreshers([ok("c"), async () => false])).toBe(false);
    expect(await runRefreshers([ok("d"), () => Promise.reject(new Error("x"))])).toBe(false);
    expect(ran).toEqual(["a", "b", "c", "d"]);
    expect(await runRefreshers([])).toBe(true);
  });
});

describe("the band: words, never a spinner or a colour alone", () => {
  it("at rest it is NOTHING — a resting screen renders exactly the scroller it rendered before", () => {
    expect(renderToStaticMarkup(createElement(PullIndicator, { phase: { kind: "idle" } }))).toBe("");
    expect(renderToStaticMarkup(createElement(TrangCon, null, "x"))).toBe('<div class="xa-trang xa-trang--con">x</div>');
  });

  it("pulling: 'Kéo xuống…', past the trigger 'Thả tay…'; decoration for a screen reader (a touch lasts a moment)", () => {
    const short = renderToStaticMarkup(createElement(PullIndicator, { phase: { kind: "pulling", px: 20 } }));
    expect(short).toContain('aria-hidden="true"');
    expect(short).toContain("height:20px");
    expect(short).toContain(PULL_REFRESH.pull);
    const far = renderToStaticMarkup(createElement(PullIndicator, { phase: { kind: "pulling", px: PULL_TRIGGER_PX } }));
    expect(far).toContain(PULL_REFRESH.release);
  });

  it("refreshing says so in words (`role=status`); a failure says what to do next (`role=alert`)", () => {
    const busy = renderToStaticMarkup(createElement(PullIndicator, { phase: { kind: "refreshing" } }));
    expect(busy).toContain('role="status"');
    expect(busy).toContain(PULL_REFRESH.refreshing);
    const failed = renderToStaticMarkup(createElement(PullIndicator, { phase: { kind: "failed" } }));
    expect(failed).toContain('role="alert"');
    expect(failed).toContain(PULL_REFRESH.failed);
    // Rule 7 of the citizen channel: no error code, and the next step is named.
    expect(PULL_REFRESH.failed).not.toMatch(/\d{3}/);
    expect(PULL_REFRESH.failed).toContain("kéo xuống để thử lại");
  });
});

describe("news: a pull replaces the list with the first page — or keeps it", () => {
  const shown = {
    ...TIN_DAU,
    muc: [item("a"), item("b"), item("c")],
    con_tro: "c2",
    con_nua: true,
    da_co_trang_dau: true,
    dang_tai: false,
  };

  it("answered: the fresh first page REPLACES what was loaded (a withdrawn item goes, paging restarts)", () => {
    const next = afterNewsRefresh(shown, { kieu: "xong", gia_tri: { muc: [item("n"), item("a")], con_tro: "c1", con_nua: true } });
    expect(next.muc.map((t) => t.id)).toEqual(["n", "a"]);
    expect(next.con_tro).toBe("c1");
    expect(next.loi).toBeNull();
    expect(next.dang_tai).toBe(false);
  });

  it("a 429 or any failure with a list on screen: the list stays exactly as it was", () => {
    expect(afterNewsRefresh(shown, { kieu: "rate-limited", retryAfterSeconds: 30 })).toBe(shown);
    expect(afterNewsRefresh(shown, { kieu: "loi-mang" })).toBe(shown);
  });

  it("a failure with nothing on screen: the first load's failure, with its 'Thử lại'", () => {
    const next = afterNewsRefresh({ ...TIN_DAU, dang_tai: false, loi: "loi-mang" }, { kieu: "rate-limited", retryAfterSeconds: 30 });
    expect(next.loi).toBe("rate-limited");
    expect(next.retryAfterSeconds).toBe(30);
    expect(next.muc).toEqual([]);
  });
});

describe("Phản ánh của tôi: a refresh re-reads the FIRST page", () => {
  const petition = (code: string, status: string) => ({
    ma_tra_cuu: code,
    trang_thai: status,
    linh_vuc: "moi-truong",
    nhan_linh_vuc: "Môi trường",
    trich_noi_dung: "Nội dung thử",
    goc_dem_han: "2026-10-01T08:00:00+07:00",
    han_tiep_nhan: null,
    han_xu_ly_xong: null,
    rating: null,
    rated_at: null,
  });
  const ready: MyPetitions = {
    kind: "ready",
    items: [petition("PA-THU-01", "da-tiep-nhan")],
    cursor: "k2",
    hasMore: true,
    loadingMore: false,
    moreFailure: null,
  };
  const page = (status: string): Extract<KetQuaDanhSach, { kieu: "xong" }> =>
    ({ kieu: "xong", trang: { muc: [petition("PA-THU-01", status)], con_tro: "", con_nua: false } }) as Extract<
      KetQuaDanhSach,
      { kieu: "xong" }
    >;

  it("the page → a fresh list: a status the commune moved shows the new one", () => {
    const next = petitionsAfterRefresh(ready, page("dang-xu-ly"));
    expect(next.kind).toBe("ready");
    expect(next.kind === "ready" && next.items[0]!.trang_thai).toBe("dang-xu-ly");
    expect(next.kind === "ready" && next.hasMore).toBe(false);
  });

  it("a failure keeps the list; a lost session goes back to the card that asks — never the old list", () => {
    expect(petitionsAfterRefresh(ready, "network")).toBe(ready);
    expect(petitionsAfterRefresh(ready, "server")).toBe(ready);
    expect(petitionsAfterRefresh(ready, "session")).toEqual({ kind: "idle" });
  });
});

describe("directory and alerts", () => {
  it("directory: a failed re-read keeps the numbers on screen; with nothing on screen the failure shows", () => {
    const list = { kieu: "xong" as const, can_bo: [] };
    expect(directoryAfterRefresh(list, { kieu: "loi", loi: "loi-mang" })).toBe(list);
    expect(directoryAfterRefresh({ kieu: "loi", loi: "loi-mang" }, { kieu: "loi", loi: "loi-may-chu" })).toEqual({
      kieu: "loi",
      loi: "loi-may-chu",
    });
    const fresh = { kieu: "xong" as const, can_bo: [] };
    expect(directoryAfterRefresh(list, fresh)).toBe(fresh);
  });

  it("alerts: a pull does not wait out Retry-After — one read, the 429 is the answer", async () => {
    const waits: number[] = [];
    let reads = 0;
    const s = await loadDisasterAlerts(
      async () => (reads++, { kieu: "rate-limited", retryAfterSeconds: 45 }),
      async (ms) => void waits.push(ms),
      false,
    );
    expect(reads).toBe(1);
    expect(waits).toEqual([]);
    expect(s).toEqual({ kind: "failed", error: "rate-limited", retryAfterSeconds: 45 });
    // The mount's path is unchanged: one wait, one retry.
    reads = 0;
    await loadDisasterAlerts(async () => (reads++, { kieu: "rate-limited", retryAfterSeconds: 45 }), async (ms) => void waits.push(ms));
    expect(reads).toBe(2);
    expect(waits).toEqual([45_000]);
  });
});
