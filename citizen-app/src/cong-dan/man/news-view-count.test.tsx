import { act, createElement, type ReactElement, StrictMode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE VIEW COUNT'S NETWORK SIDE (owner, 02/10/2026). The server counts a view on every read of
 * `GET /api/v1/commune-news/{id}` unless the read carries `no_view=1`. So the citizen channel must make EXACTLY ONE
 * counted read per open:
 *
 *   · the parser keeps a whole number ≥ 0 and drops anything else, without refusing the item;
 *   · a mount — under StrictMode's double effect run — is ONE read, without `no_view`, in both apps;
 *   · "Thử lại" after a failed read is a new counted read (the failed one was not counted);
 *   · the broadcast player's link re-read carries `no_view=1`, so an expired link adds no view.
 *
 * Mounted for real on a minimal DOM — the technique and the reason are `name-at-entry.test.tsx`'s (no jsdom in the
 * repo, and adding one is a dependency to ask for). Every value below is invented test data.
 */

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const real = await importOriginal<typeof import("../api/goi-vigov")>();
  // The real client, wrapped so each call's arguments are recorded; `fetch` underneath is stubbed per case.
  return { ...real, baiTinCuaXa: vi.fn(real.baiTinCuaXa) };
});

import { baiTinCuaXa } from "../api/goi-vigov";
import { diaChiBaiTin, docBaiTin, docTrangTinXa, readViewCount } from "../api/hop-dong-cong-khai";
import { TIN_XA } from "./noi-dung"; // vi-name-ok: existing strings module
import { BaiTinXa, rereadAudio } from "./TinTucAppXa"; // vi-name-ok: existing screen component
import { ManBaiTin } from "./TinTucXaScreen"; // vi-name-ok: existing shared-app screen

const DOMAIN = "xa-thu.vigov.example";

const ITEM = {
  id: "tin-01",
  title: "Lịch tiêm chủng tháng 10",
  summary: "Tóm tắt",
  published_on: "2026-10-01",
  category_name: "Y tế",
  body: "Đoạn một.",
};

// ---------------------------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------------------------

describe("`view_count` in the public news item", () => {
  it("a whole number ≥ 0 is kept; anything else is absent — never a malformed page", () => {
    expect(readViewCount(0)).toBe(0);
    expect(readViewCount(1234)).toBe(1234);
    for (const bad of [-1, 1.5, Number.NaN, Number.POSITIVE_INFINITY, "12", null, undefined, true, {}]) {
      expect(readViewCount(bad), String(bad)).toBeUndefined();
    }
  });

  it("detail and list carry it as `viewCount`; an older server (no key) or a bad value → no key at all", () => {
    expect(docBaiTin({ ...ITEM, view_count: 7 })?.viewCount).toBe(7);
    expect(docBaiTin({ ...ITEM, view_count: 0 })?.viewCount).toBe(0);
    const old = docBaiTin(ITEM);
    expect(old).not.toBeNull();
    expect(old).not.toHaveProperty("viewCount");
    const bad = docBaiTin({ ...ITEM, view_count: -3 });
    expect(bad).not.toBeNull();
    expect(bad).not.toHaveProperty("viewCount");
    const page = docTrangTinXa({ items: [{ ...ITEM, view_count: 1234 }, { ...ITEM, id: "tin-02", view_count: "x" }], next_cursor: "", has_more: false });
    expect(page?.muc.map((t) => t.viewCount)).toEqual([1234, undefined]);
  });
});

// ---------------------------------------------------------------------------------------------
// Addresses
// ---------------------------------------------------------------------------------------------

type FakeAnswer = { status: number; ok: boolean; json: () => Promise<unknown> };
const answer = (status: number, body: unknown): FakeAnswer => ({ status, ok: status >= 200 && status < 300, json: async () => body });

/** Stubs `fetch` with a fixed sequence (the last answer repeats); returns the URLs called. */
function stubFetch(...answers: FakeAnswer[]): string[] {
  const calls: string[] = [];
  vi.stubGlobal("fetch", (url: string) => {
    calls.push(url);
    return Promise.resolve(answers[Math.min(calls.length - 1, answers.length - 1)]!);
  });
  return calls;
}

const noViewOf = (url: string) => new URL(url).searchParams.get("no_view");

describe("`no_view=1` — only on the read that is not an open", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("an open sends no `no_view`; the flag adds exactly `no_view=1`", () => {
    expect(noViewOf(diaChiBaiTin(DOMAIN, "tin-01"))).toBeNull();
    expect(noViewOf(diaChiBaiTin(DOMAIN, "tin-01", { noView: true }))).toBe("1");
    expect(noViewOf(diaChiBaiTin(DOMAIN, "tin-01", { noView: false }))).toBeNull();
  });

  it("the broadcast player's link re-read goes out with `no_view=1`", async () => {
    const calls = stubFetch(answer(200, { ...ITEM, type: "truyen-thanh", audio_url: "https://media.vidu.example/a.mp3" }));
    expect(await rereadAudio(DOMAIN, "tin-01")).toEqual({ url: "https://media.vidu.example/a.mp3" });
    expect(calls).toHaveLength(1);
    expect(noViewOf(calls[0]!)).toBe("1");
  });
});

// ---------------------------------------------------------------------------------------------
// Mount — minimal DOM (copied from `name-at-entry.test.tsx`; see the header)
// ---------------------------------------------------------------------------------------------

class FakeNode {
  childNodes: FakeNode[] = [];
  parentNode: FakeNode | null = null;
  constructor(
    readonly nodeType: number,
    readonly nodeName: string,
    public ownerDocument: FakeDocument | null,
  ) {}
  get firstChild(): FakeNode | null {
    return this.childNodes[0] ?? null;
  }
  get lastChild(): FakeNode | null {
    return this.childNodes[this.childNodes.length - 1] ?? null;
  }
  get parentElement(): FakeNode | null {
    return this.parentNode;
  }
  appendChild(child: FakeNode) {
    child.parentNode?.removeChild(child);
    this.childNodes.push(child);
    child.parentNode = this;
    return child;
  }
  insertBefore(child: FakeNode, before: FakeNode | null) {
    if (before === null) return this.appendChild(child);
    child.parentNode?.removeChild(child);
    const i = this.childNodes.indexOf(before);
    if (i < 0) throw new Error("insertBefore: reference node is not a child");
    this.childNodes.splice(i, 0, child);
    child.parentNode = this;
    return child;
  }
  removeChild(child: FakeNode) {
    const i = this.childNodes.indexOf(child);
    if (i < 0) throw new Error("removeChild: not a child");
    this.childNodes.splice(i, 1);
    child.parentNode = null;
    return child;
  }
  contains(node: FakeNode | null): boolean {
    for (let n = node; n !== null; n = n.parentNode) if (n === this) return true;
    return false;
  }
  get textContent(): string {
    return this.childNodes.map((c) => c.textContent).join("");
  }
  set textContent(text: string) {
    for (const c of this.childNodes) c.parentNode = null;
    this.childNodes = [];
    if (text !== "" && this.ownerDocument !== null) this.appendChild(this.ownerDocument.createTextNode(text));
  }
  addEventListener() {}
  removeEventListener() {}
}

class FakeText extends FakeNode {
  constructor(
    public nodeValue: string,
    doc: FakeDocument,
  ) {
    super(3, "#text", doc);
  }
  get data() {
    return this.nodeValue;
  }
  override get textContent() {
    return this.nodeValue;
  }
  override set textContent(text: string) {
    this.nodeValue = text;
  }
}

class FakeElement extends FakeNode {
  readonly attributes = new Map<string, string>();
  readonly style: Record<string, string> = {};
  onclick: unknown = null;
  constructor(
    readonly tagName: string,
    readonly namespaceURI: string,
    doc: FakeDocument,
  ) {
    super(1, tagName, doc);
  }
  setAttribute(name: string, value: string) {
    this.attributes.set(name, String(value));
  }
  getAttribute(name: string) {
    return this.attributes.get(name) ?? null;
  }
  hasAttribute(name: string) {
    return this.attributes.has(name);
  }
  removeAttribute(name: string) {
    this.attributes.delete(name);
  }
}

const HTML_NS = "http://www.w3.org/1999/xhtml";

class FakeDocument extends FakeNode {
  readonly documentElement: FakeElement;
  readonly body: FakeElement;
  activeElement: FakeElement | null = null;
  defaultView: unknown = null;
  constructor() {
    super(9, "#document", null);
    this.documentElement = this.createElement("html");
    this.body = this.createElement("body");
    this.appendChild(this.documentElement);
    this.documentElement.appendChild(this.body);
  }
  createElement(name: string) {
    return new FakeElement(name.toUpperCase(), HTML_NS, this);
  }
  createElementNS(ns: string, name: string) {
    return new FakeElement(name, ns, this);
  }
  createTextNode(text: string) {
    return new FakeText(text, this);
  }
}

let createRoot: typeof import("react-dom/client").createRoot;
const saved = {
  window: (globalThis as Record<string, unknown>).window,
  document: (globalThis as Record<string, unknown>).document,
};

beforeAll(async () => {
  const doc = new FakeDocument();
  const win: Record<string, unknown> = {
    document: doc,
    event: undefined,
    HTMLIFrameElement: class {},
    addEventListener() {},
    removeEventListener() {},
    location: { protocol: "about:" },
  };
  win.top = win;
  win.self = win;
  doc.defaultView = win;
  Object.assign(globalThis, { window: win, document: doc, IS_REACT_ACT_ENVIRONMENT: true });
  createRoot = (await import("react-dom/client")).createRoot;
});

afterAll(() => {
  Object.assign(globalThis, { window: saved.window, document: saved.document, IS_REACT_ACT_ENVIRONMENT: undefined });
});

let reactErrors: unknown[][];

beforeEach(() => {
  reactErrors = [];
  vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void reactErrors.push(a));
  vi.mocked(baiTinCuaXa).mockClear();
});

afterEach(() => {
  vi.mocked(console.error).mockRestore();
  vi.unstubAllGlobals();
  expect(reactErrors).toEqual([]);
});

const settle = () =>
  act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });

/** Mounts in StrictMode (effects run TWICE in development), waits for every effect and promise. */
async function mount(el: ReactElement) {
  const doc = (globalThis as unknown as { document: FakeDocument }).document;
  const host = doc.createElement("div");
  doc.body.appendChild(host);
  const root = createRoot(host as unknown as Element);
  await act(async () => {
    root.render(createElement(StrictMode, null, el));
  });
  await settle();
  return { host, unmount: () => act(() => root.unmount()) };
}

/** The button whose text is `label`, tapped through the props React keeps on the element (the shim has no events). */
async function tap(host: FakeNode, label: string) {
  const find = (n: FakeNode): FakeElement | null => {
    if (n instanceof FakeElement && n.tagName === "BUTTON" && n.textContent === label) return n;
    for (const c of n.childNodes) {
      const hit = find(c);
      if (hit !== null) return hit;
    }
    return null;
  };
  const button = find(host);
  expect(button, `no button "${label}"`).not.toBeNull();
  const key = Object.keys(button!).find((k) => k.startsWith("__reactProps$"))!;
  await act(async () => {
    ((button as unknown as Record<string, { onClick: () => void }>)[key]!).onClick();
  });
  await settle();
}

/** Every detail read this case made: [domain, id, options]. */
const reads = () => vi.mocked(baiTinCuaXa).mock.calls;
const counted = (call: Parameters<typeof baiTinCuaXa>) => call[2]?.noView !== true;

const SCREENS: ReadonlyArray<readonly [string, (id: string) => ReactElement]> = [
  ["commune app", (id) => createElement(BaiTinXa, { ten_mien: DOMAIN, id, onQuayLai: () => {} })],
  ["shared app", (id) => createElement(ManBaiTin, { ten_mien: DOMAIN, id, onQuayLai: () => {}, ten_xa: "Xã Thử" })],
];

describe("one counted read per open, both apps", () => {
  for (const [name, screen] of SCREENS) {
    it(`${name}: a StrictMode mount is ONE read, without \`no_view\`, and shows the count`, async () => {
      const calls = stubFetch(answer(200, { ...ITEM, view_count: 1234 }));
      const { host, unmount } = await mount(screen("tin-01"));
      expect(reads()).toHaveLength(1);
      expect(counted(reads()[0]!)).toBe(true);
      expect(calls).toHaveLength(1);
      expect(noViewOf(calls[0]!)).toBeNull();
      expect(host.textContent).toContain("1.234 lượt xem");
      await unmount();
    });

    it(`${name}: "Thử lại" after a failure is one more COUNTED read — never two`, async () => {
      const calls = stubFetch(answer(503, {}), answer(200, { ...ITEM, view_count: 0 }));
      const { host, unmount } = await mount(screen("tin-01"));
      expect(reads()).toHaveLength(1);
      await tap(host, TIN_XA.nut_thu_lai);
      expect(reads()).toHaveLength(2);
      expect(reads().every(counted)).toBe(true);
      expect(calls.map(noViewOf)).toEqual([null, null]);
      expect(host.textContent).toContain("0 lượt xem");
      await unmount();
    });
  }
});
