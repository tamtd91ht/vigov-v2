import { describe, expect, it } from "vitest";

import { movedTabRoute } from "./moved-tabs";
import { TAB_CAU_HINH } from "./thanh-tab-cau-hinh";

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
  });
});
