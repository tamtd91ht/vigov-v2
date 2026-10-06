import { describe, expect, it } from "vitest";

import {
  landingTab,
  LEGACY_DIRECTORY_REDIRECT,
  legacyContentRedirect,
  MINI_APP_MENU_KEYS,
  MINI_APP_TABS,
  miniAppTabHref,
  requestedTab,
  visibleTabs,
} from "./mini-app-tabs";

describe("tabs of /mini-app — the prototype's two, each on the key its read routes declare", () => {
  it("Nội dung then Danh bạ cán bộ; content.read and admin.user", () => {
    expect(MINI_APP_TABS.map((t) => [t.key, t.label, t.permission])).toEqual([
      ["noi-dung", "Nội dung", "content.read"],
      ["danh-ba", "Danh bạ cán bộ", "admin.user"],
    ]);
  });

  it("the menu opens on either tab's key, and on nothing else", () => {
    expect(MINI_APP_MENU_KEYS).toEqual(["content.read", "admin.user"]);
  });

  it("?tab= — only exactly `danh-ba` opens the directory; absent, unknown, repeated → content", () => {
    expect(requestedTab("danh-ba")).toBe("danh-ba");
    for (const v of [undefined, "", "noi-dung", "DANH-BA", "danh-ba ", ["danh-ba"], ["danh-ba", "noi-dung"], "constructor"]) {
      expect(requestedTab(v)).toBe("noi-dung");
    }
    expect(miniAppTabHref("danh-ba")).toBe("/mini-app?tab=danh-ba");
  });
});

describe("which tabs show, and where a person lands", () => {
  it("both keys → both tabs, prototype order", () => {
    expect(visibleTabs(["admin.user", "content.read"])).toEqual(["noi-dung", "danh-ba"]);
  });

  it("exact match only — near keys open nothing (DENIED)", () => {
    expect(visibleTabs(["content.update", "content.reads", "admin.user.delete", "admin.users", "admin"])).toEqual([]);
    expect(visibleTabs([])).toEqual([]);
  });

  it("only admin.user: a bare /mini-app lands on the directory, not on a refusal", () => {
    const visible = visibleTabs(["admin.user"]);
    expect(visible).toEqual(["danh-ba"]);
    expect(landingTab("noi-dung", visible)).toBe("danh-ba");
    expect(landingTab("danh-ba", visible)).toBe("danh-ba");
  });

  it("only content.read: ?tab=danh-ba lands on the content tab — the directory is never shown", () => {
    const visible = visibleTabs(["content.read", "content.update"]);
    expect(landingTab("danh-ba", visible)).toBe("noi-dung");
  });

  it("neither key → null (the screen shows the refusal)", () => {
    expect(landingTab("noi-dung", [])).toBeNull();
    expect(landingTab("danh-ba", [])).toBeNull();
  });
});

describe("old links", () => {
  it("/noi-dung → /mini-app, keeping every query but `tab`", () => {
    expect(legacyContentRedirect({})).toBe("/mini-app");
    expect(legacyContentRedirect({ tab: "danh-ba" })).toBe("/mini-app");
    expect(legacyContentRedirect({ type: "tin-tuc" })).toBe("/mini-app?type=tin-tuc");
    expect(legacyContentRedirect({ type: ["a", "b"], tab: "x", z: undefined })).toBe("/mini-app?type=a&type=b");
  });

  it("/danh-ba → /mini-app?tab=danh-ba, nothing else carried", () => {
    expect(LEGACY_DIRECTORY_REDIRECT).toBe("/mini-app?tab=danh-ba");
  });
});
