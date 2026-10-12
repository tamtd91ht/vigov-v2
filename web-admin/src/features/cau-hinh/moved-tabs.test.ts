import { describe, expect, it } from "vitest";

import { movedTabRoute } from "./moved-tabs";
import { SYSTEM_TABS, systemTabFromQuery, TAB_CAU_HINH } from "./thanh-tab-cau-hinh";

describe("old /cau-hinh?tab= links to the two tabs that moved (05/10/2026)", () => {
  it("nguoi-dung → /nguoi-dung, phan-quyen → /nguoi-dung/phan-quyen", () => {
    expect(movedTabRoute("nguoi-dung")).toBe("/nguoi-dung");
    expect(movedTabRoute("phan-quyen")).toBe("/nguoi-dung/phan-quyen");
  });

  it("a tab still on /cau-hinh, no tab, or a look-alike → no redirect", () => {
    for (const t of TAB_CAU_HINH) expect(movedTabRoute(t.ma)).toBeNull();
    for (const v of [undefined, "", "Nguoi-Dung", "nguoi-dung/", "phan-quyen ", "constructor", "__proto__"]) {
      expect(movedTabRoute(v)).toBeNull();
    }
  });

  it("a repeated ?tab= is ambiguous → no redirect", () => {
    expect(movedTabRoute(["nguoi-dung", "phan-quyen"])).toBeNull();
    expect(movedTabRoute(["nguoi-dung"])).toBeNull();
  });

  it("no moved tab id is still a tab on /cau-hinh — a tab is in one place only", () => {
    const ids = TAB_CAU_HINH.map((t) => t.ma as string);
    expect(ids).not.toContain("nguoi-dung");
    expect(ids).not.toContain("phan-quyen");
    for (const t of SYSTEM_TABS) expect(ids).not.toContain(t.ma);
  });
});

describe("old /cau-hinh?tab= links to the three tabs that moved to /he-thong (12/10/2026, S27)", () => {
  it("each lands on /he-thong with the same tab chosen", () => {
    expect(movedTabRoute("lich-lam-viec")).toBe("/he-thong?tab=lich-lam-viec");
    expect(movedTabRoute("nhat-ky-he-thong")).toBe("/he-thong?tab=nhat-ky-he-thong");
    expect(movedTabRoute("nhan-dien-xa")).toBe("/he-thong?tab=nhan-dien-xa");
  });

  it("every /he-thong tab has a redirect, and /he-thong reads back the tab it names", () => {
    for (const t of SYSTEM_TABS) {
      const to = movedTabRoute(t.ma);
      expect(to, t.ma).not.toBeNull();
      const url = new URL(to!, "https://xa.example.test");
      expect(url.pathname).toBe("/he-thong");
      expect(systemTabFromQuery(url.searchParams.get("tab") ?? undefined)).toBe(t.ma);
    }
  });

  it("look-alikes redirect nowhere", () => {
    for (const v of ["Lich-Lam-Viec", "nhat-ky", "nhan-dien-xa/", " nhan-dien-xa"]) expect(movedTabRoute(v)).toBeNull();
    expect(movedTabRoute(["nhan-dien-xa"])).toBeNull();
  });
});
