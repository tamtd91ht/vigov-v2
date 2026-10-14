import { afterEach, describe, expect, it, vi } from "vitest";

import { cuonToiMoc, scrollMainToTop } from "./cuon-toi";

/**
 * FIX B (14/10/2026) — `.app-main` is the scroll container and is never remounted, so every navigation that has
 * no anchor to land on must put it back at the top. These cases stand in for the DOM with the two calls the helper
 * makes (`getElementById`, `querySelector(".app-main")`); the suite has no DOM library and renders on the server.
 */

type FakeElement = { scrollIntoView: ReturnType<typeof vi.fn> };

function fakeDocument(ids: readonly string[]) {
  const main = { scrollTop: 640 };
  const elements = new Map<string, FakeElement>(ids.map((id) => [id, { scrollIntoView: vi.fn() }]));
  const document = {
    getElementById: (id: string) => elements.get(id) ?? null,
    querySelector: (selector: string) => (selector === ".app-main" ? main : null),
  };
  vi.stubGlobal("document", document);
  return { main, elements };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("cuonToiMoc — anchor when there is one, top of .app-main otherwise", () => {
  it("no anchor (a tab press) → .app-main back at the top", () => {
    const { main } = fakeDocument([]);
    cuonToiMoc(undefined);
    expect(main.scrollTop, "a tab press kept the previous screen's scroll position").toBe(0);
  });

  it("empty anchor → top as well", () => {
    const { main } = fakeDocument([]);
    cuonToiMoc("");
    expect(main.scrollTop).toBe(0);
  });

  it("an anchor that exists (Văn phòng, Chặng đường) → scrolls to it, without smooth motion, and leaves the top alone", () => {
    const { main, elements } = fakeDocument(["tn-van-phong"]);
    cuonToiMoc("tn-van-phong");
    expect(elements.get("tn-van-phong")!.scrollIntoView).toHaveBeenCalledWith({ behavior: "auto", block: "start" });
    expect(main.scrollTop, "the anchor jump was overwritten by a scroll to the top").toBe(640);
  });

  it("an anchor that names a sub-screen or a solution (no such id) → top, so the sub-screen opens at its back button", () => {
    const { main } = fakeDocument(["tn-van-phong"]);
    cuonToiMoc("quan-ly-quyen");
    expect(main.scrollTop).toBe(0);
  });

  it("no DOM (server render in tests) → does nothing and does not throw", () => {
    expect(() => cuonToiMoc("tn-van-phong")).not.toThrow();
    expect(() => scrollMainToTop()).not.toThrow();
  });
});

describe("scrollMainToTop", () => {
  it("sets .app-main to the top", () => {
    const { main } = fakeDocument([]);
    scrollMainToTop();
    expect(main.scrollTop).toBe(0);
  });

  it("no .app-main on the page (the commune page) → nothing happens", () => {
    vi.stubGlobal("document", { getElementById: () => null, querySelector: () => null });
    expect(() => scrollMainToTop()).not.toThrow();
  });
});
