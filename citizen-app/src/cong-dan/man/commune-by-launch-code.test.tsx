/// <reference types="vite/client" />
import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { communeByLaunchCode } from "../api/goi-vigov";
import { docXa, DUONG_DAN_XA, readCommunesByCode } from "../api/hop-dong-cong-khai"; // vi-name-ok: existing exports
import { CommuneByLaunchCode, hostFromLaunchCode } from "./commune-by-launch-code";
import { APP_RIENG } from "./noi-dung"; // vi-name-ok: existing string table

/**
 * THE SHARED APP'S LAUNCH CODE (`?x=`, ADR 0047 §Sửa đổi 12/10/2026) — resolved ONCE to the commune's domain by
 * the public `GET identity /api/v1/communes?code=`. Pinned here:
 *   · the call: right host and path, the code in `?code=`, no bearer, no body;
 *   · the CODE gate: a value that is not 12 lowercase hex characters sends NOTHING;
 *   · every status lands in the branch the `?host=` lookup uses (400 · 503 · 429 · 500 · malformed · network);
 *   · the reader: `host` must pass `laTenMien`, or the commune does not exist for this app;
 *   · the step on screen, MOUNTED with effects running (same minimal DOM as `name-at-entry.test.tsx` — the repo
 *     has no jsdom): resolved → the children get the domain; no commune → the sentence `TrangXa` already used for
 *     an unknown domain; 503 → the retry sentence. The code is never drawn.
 *
 * Codes, domains and commune names below are invented.
 */

const CODE = "0a1b2c3d4e5f";
const HOST = "xa-vi-du.vigov.example";
const COMMUNE = { name: "Xã Thử Nghiệm", province: "Tỉnh Ví Dụ", host: HOST };

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];

function reply(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(...answers: Array<ReturnType<typeof reply> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
    calls.push({ url, init });
    const a = answers[Math.min(i++, answers.length - 1)]!;
    return a instanceof Error ? Promise.reject(a) : Promise.resolve(a);
  });
}

// ---------------------------------------------------------------------------------------------
// Minimal DOM — copied from `name-at-entry.test.tsx` (enough for `react-dom/client` to mount a tree with SVG).
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
  calls = [];
  reactErrors = [];
  vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void reactErrors.push(a));
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.mocked(console.error).mockRestore();
  expect(reactErrors).toEqual([]);
});

/** Mounts in StrictMode, waits for every effect and promise. */
async function mount(el: ReactElement) {
  const doc = (globalThis as unknown as { document: FakeDocument }).document;
  const host = doc.createElement("div");
  doc.body.appendChild(host);
  const root = createRoot(host as unknown as Element);
  await act(async () => {
    root.render(createElement(StrictMode, null, el));
  });
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  return { text: host.textContent, unmount: () => act(() => root.unmount()) };
}

/** What `App.tsx` passes, reduced to a marker that proves which domain came through. */
const renderCommune = (host: string) => createElement("p", null, `COMMUNE-UI:${host}`);

// ---------------------------------------------------------------------------------------------

describe("the call — `GET identity /api/v1/communes?code=`", () => {
  it("right host and path, the code in `?code=`, no bearer, no body; the answer carries the domain", async () => {
    stubFetch(reply(200, { items: [COMMUNE] }));
    const result = await communeByLaunchCode(CODE);
    expect(result).toEqual({ kieu: "xong", gia_tri: [{ ten: COMMUNE.name, tinh: COMMUNE.province, host: HOST }] });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(`https://identity.api.vigov.vn${DUONG_DAN_XA}?code=${CODE}`);
    expect(calls[0]!.init.method).toBe("GET");
    expect(calls[0]!.init.body).toBeUndefined();
    expect(Object.keys(calls[0]!.init.headers as Record<string, string>)).toEqual(["Accept"]);
  });

  it("a code of the wrong shape sends NOTHING", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    for (const code of ["", "0a1b2c3d4e5", "0a1b2c3d4e5f0", "0A1B2C3D4E5F", "0a1b2c3d4e5g", HOST, "0a1b2c&host=x"]) {
      expect(await communeByLaunchCode(code), code).toEqual({ kieu: "khong-hop-le" });
    }
    expect(fake).not.toHaveBeenCalled();
  });

  it("each status lands in the branch the `?host=` lookup uses", async () => {
    const cases: Array<[ReturnType<typeof reply> | Error, string]> = [
      [reply(200, { items: [] }), "xong"],
      [reply(400, { error: "invalid_code" }), "khong-hop-le"],
      [reply(503, { error: "platform_unavailable" }), "tam-ngung"],
      [reply(429, {}), "loi-may-chu"],
      [reply(500, {}), "loi-may-chu"],
      [reply(200, { items: [{ name: "a", province: "b" }] }), "loi-may-chu"],
      [new Error("offline"), "loi-mang"],
    ];
    for (const [answer, kind] of cases) {
      stubFetch(answer);
      expect((await communeByLaunchCode(CODE)).kieu, kind).toBe(kind);
    }
  });
});

describe("the reader", () => {
  it("a `host` that is not a well-formed domain is no commune; a missing field is malformed", () => {
    expect(readCommunesByCode({ items: [{ ...COMMUNE, host: "https://x.vn" }] })).toEqual([]);
    expect(readCommunesByCode({ items: [{ ...COMMUNE, host: "localhost" }] })).toEqual([]);
    expect(readCommunesByCode({ items: [{ name: "a", province: "b" }] })).toBeNull();
    expect(readCommunesByCode({})).toBeNull();
    expect(readCommunesByCode({ items: [] })).toEqual([]);
  });

  it("the `?host=` reader ignores a `launch_code` field — this app never needs it", () => {
    expect(docXa({ items: [{ name: "a", province: "b", launch_code: CODE }] })).toEqual([{ ten: "a", tinh: "b" }]);
  });
});

describe("answer → domain or sentence (pure)", () => {
  const one = { ten: COMMUNE.name, tinh: COMMUNE.province, host: HOST };

  it("exactly one commune with a name and a domain → that domain", () => {
    expect(hostFromLaunchCode({ kieu: "xong", gia_tri: [one] })).toEqual({ host: HOST });
  });

  it("none, two, or a nameless one → the sentence `TrangXa` shows for an unknown domain", () => {
    for (const items of [[], [one, { ...one, host: "xa-khac.vigov.example" }], [{ ...one, ten: "  " }]]) {
      expect(hostFromLaunchCode({ kieu: "xong", gia_tri: items })).toEqual({ message: APP_RIENG.khong_thay });
    }
    expect(hostFromLaunchCode({ kieu: "khong-hop-le" })).toEqual({ message: APP_RIENG.khong_thay });
  });

  it("network, 503, server error → the retry sentence", () => {
    for (const kind of ["tam-ngung", "loi-may-chu", "loi-mang", "chua-cau-hinh"] as const) {
      expect(hostFromLaunchCode({ kieu: kind })).toEqual({ message: APP_RIENG.chua_ket_noi });
    }
  });
});

describe("the step on screen", () => {
  it("first paint: the commune app's waiting line — no ViHAT intro, no code, the commune UI not yet", () => {
    const markup = renderToStaticMarkup(createElement(CommuneByLaunchCode, { code: CODE, children: renderCommune }));
    expect(markup).toContain(APP_RIENG.dang_mo);
    expect(markup).toContain("xa-app");
    expect(markup).not.toContain(CODE);
    expect(markup).not.toContain("COMMUNE-UI");
  });

  it("resolved → the commune UI on the RESOLVED domain, the waiting line gone", async () => {
    stubFetch(reply(200, { items: [COMMUNE] }));
    const m = await mount(createElement(CommuneByLaunchCode, { code: CODE, children: renderCommune }));
    expect(m.text).toBe(`COMMUNE-UI:${HOST}`);
    await m.unmount();
  });

  it("no commune → the same sentence as an unknown domain, with Thử lại; no commune UI, no code", async () => {
    stubFetch(reply(200, { items: [] }));
    const m = await mount(createElement(CommuneByLaunchCode, { code: CODE, children: renderCommune }));
    expect(m.text).toContain(APP_RIENG.khong_thay);
    expect(m.text).toContain(APP_RIENG.thu_lai);
    expect(m.text).not.toContain("COMMUNE-UI");
    expect(m.text).not.toContain(CODE);
    await m.unmount();
  });

  it("503 → the retry sentence", async () => {
    stubFetch(reply(503, { error: "platform_unavailable" }));
    const m = await mount(createElement(CommuneByLaunchCode, { code: CODE, children: renderCommune }));
    expect(m.text).toContain(APP_RIENG.chua_ket_noi);
    expect(m.text).toContain(APP_RIENG.thu_lai);
    expect(m.text).not.toContain("COMMUNE-UI");
    await m.unmount();
  });

  it("a malformed code never reaches the network, and shows the not-found sentence", async () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    const m = await mount(createElement(CommuneByLaunchCode, { code: "not-a-code", children: renderCommune }));
    expect(fake).not.toHaveBeenCalled();
    expect(m.text).toContain(APP_RIENG.khong_thay);
    await m.unmount();
  });
});
