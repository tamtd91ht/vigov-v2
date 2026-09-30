/// <reference types="vite/client" />
import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE CITIZEN'S NAME IN THE COMMUNE APP — asked ONCE, at entry, and only SHOWN afterwards (user decision
 * 29/09/2026, reversing ea76c9d's "ask where it is needed").
 *
 * What is pinned here, and how:
 *   · the entry (`TrangXa`) asks Zalo exactly once, even under StrictMode's double mount, and hands the
 *     name down — MOUNTED for real, effects running, on a minimal DOM (same technique and same reason as
 *     `hieu-ung-khong-phien.test.tsx`: the repo has no jsdom, and adding one is a dependency to ask for);
 *   · no screen offers "Lấy từ Zalo" any more — rendered markup AND source text;
 *   · Cá nhân says "Chưa xác định" without a name; the send form starts pre-filled, or empty;
 *   · the stylesheet defect that broke the Cá nhân "Tiện ích của tôi" card cannot come back silently.
 *
 * Every name below is invented test data.
 */

const commune = vi.hoisted(() => ({ ten: "Xã Thử Nghiệm", tinh: "Tỉnh Thử Nghiệm" }));

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const real = await importOriginal<typeof import("../api/goi-vigov")>();
  return {
    ...real,
    // The public commune lookup answers at once; news answers "nothing" — no network in this file.
    traXaTheoTenMien: vi.fn(async () => ({ kieu: "xong" as const, gia_tri: [commune] })),
    tinCuaXa: vi.fn(async () => ({ kieu: "khong-thay" as const })),
  };
});

import { XA_PA, XA_TN } from "./noi-dung";
import * as sendScreens from "./PhanAnhAppXa";
import { blankForm, CommuneSendScreen } from "./PhanAnhAppXa";
import { CaNhanXa } from "./TienIchAppXa";
// vi-name-ok: importing the EXISTING type `LayTenZalo` — no new name
import { afterNameAsk, afterNameCheck, type LayTenZalo, type NameRequestMode, nameShown } from "./trai-nghiem";
import { TrangXa } from "./TrangXa";

const NAME = "Nguyễn Văn An";
const OLD_BUTTON = "Lấy họ tên từ Zalo";

// ---------------------------------------------------------------------------------------------
// Minimal DOM — enough for `react-dom/client` to mount, update and unmount a tree with SVG and no
// inputs. It simulates no events: a case that needs a tap uses the pure functions instead.
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
  // The commune app draws its icons as SVG (`BieuTuong.tsx`); React creates those through this.
  createElementNS(ns: string, name: string) {
    return new FakeElement(name, ns, this);
  }
  createTextNode(text: string) {
    return new FakeText(text, this);
  }
}

// `react-dom/client` decides "is there a DOM" WHEN THE MODULE LOADS — so the globals come first.
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
});

afterEach(() => {
  vi.mocked(console.error).mockRestore();
  // Whatever the shim lacks, React says through `console.error` — a broken mount must not pass.
  expect(reactErrors).toEqual([]);
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
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  return { host, unmount: () => act(() => root.unmount()) };
}

type NameBridge = (mode: NameRequestMode) => ReturnType<LayTenZalo>;

/** A fake of the shell's name bridge that records every mode it is asked in. */
function nameBridge(answer: Awaited<ReturnType<NameBridge>>) {
  const calls: NameRequestMode[] = [];
  const bridge: NameBridge = async (mode) => {
    calls.push(mode);
    return answer;
  };
  return { bridge, calls };
}

const entry = (bridge?: NameBridge) =>
  createElement(TrangXa, { ten_mien: "thu.vigov.vn", ...(bridge ? { lay_ten: bridge } : {}) });

describe("the entry asks Zalo for the name ONCE and passes it down", () => {
  it("already allowed: one silent check under StrictMode, no card, the home screen greets by name", async () => {
    const { bridge, calls } = nameBridge({ kieu: "xong", ho_ten: NAME });
    const { host, unmount } = await mount(entry(bridge));
    // StrictMode mounted twice; the platform was asked once, and never with the dialog.
    expect(calls).toEqual(["check"]);
    expect(host.textContent).toContain(XA_TN.xin_chao_ten(NAME));
    expect(host.textContent).not.toContain(XA_TN.name_card_title);
    expect(host.textContent).not.toContain(OLD_BUTTON);
    await unmount();
  });

  it("not allowed yet: the card says WHY before any dialog, and nothing is asked until the citizen taps", async () => {
    const { bridge, calls } = nameBridge({ kieu: "tu-choi" });
    const { host, unmount } = await mount(entry(bridge));
    expect(calls).toEqual(["check"]);
    expect(host.textContent).toContain(XA_TN.name_card_title);
    expect(host.textContent).toContain(XA_TN.name_card_why);
    expect(host.textContent).toContain(XA_TN.name_card_agree);
    expect(host.textContent).toContain(XA_TN.name_card_decline);
    // No greeting with a name that does not exist: the province stands where the name would.
    expect(host.textContent).toContain(commune.tinh);
    await unmount();
  });

  it("outside Zalo: no card whose button could only fail, and no name", async () => {
    const { bridge, calls } = nameBridge({ kieu: "ngoai-zalo" });
    const { host, unmount } = await mount(entry(bridge));
    expect(calls).toEqual(["check"]);
    expect(host.textContent).not.toContain(XA_TN.name_card_title);
    expect(host.textContent).toContain(commune.tinh);
    await unmount();
  });

  it("opening the app opens NO session: the opener is not called, 'Phản ánh của tôi' only offers to ask", async () => {
    const openSession = vi.fn(async () => ({ kieu: "tu-choi" as const }));
    const { host, unmount } = await mount(createElement(TrangXa, { ten_mien: "thu.vigov.vn", openSession }));
    expect(openSession).not.toHaveBeenCalled();
    expect(host.textContent).toContain(XA_PA.need_session_button);
    await unmount();
  });

  it("no bridge injected (dev, tests): nothing is asked, nothing is offered", async () => {
    const { host, unmount } = await mount(entry());
    expect(host.textContent).not.toContain(XA_TN.name_card_title);
    expect(host.textContent).toContain(commune.ten);
    await unmount();
  });
});

describe("the entry state, step by step (the tap the minimal DOM cannot simulate)", () => {
  it("check: a name settles it; outside Zalo settles on null; anything else offers the card", () => {
    expect(afterNameCheck({ kieu: "xong", ho_ten: NAME })).toEqual({ kind: "settled", name: NAME });
    expect(afterNameCheck({ kieu: "ngoai-zalo" })).toEqual({ kind: "settled", name: null });
    expect(afterNameCheck({ kieu: "tu-choi" })).toEqual({ kind: "needs-consent" });
    expect(afterNameCheck({ kieu: "khong-lay-duoc" })).toEqual({ kind: "needs-consent" });
  });

  it("ask: whatever Zalo answers, it is final for this open — never asked a second time", () => {
    expect(afterNameAsk({ kieu: "xong", ho_ten: NAME })).toEqual({ kind: "settled", name: NAME });
    for (const kieu of ["tu-choi", "ngoai-zalo", "khong-lay-duoc"] as const) {
      expect(afterNameAsk({ kieu })).toEqual({ kind: "settled", name: null });
    }
  });

  it("screens see a name only once settled", () => {
    expect(nameShown({ kind: "checking" })).toBeNull();
    expect(nameShown({ kind: "needs-consent" })).toBeNull();
    expect(nameShown({ kind: "asking" })).toBeNull();
    expect(nameShown({ kind: "settled", name: NAME })).toBe(NAME);
  });
});

const personal = (name: string | null) =>
  renderToStaticMarkup(
    createElement(CaNhanXa, {
      ho_ten: name,
      ten_xa: commune.ten,
      tinh: commune.tinh,
      so_phieu: null,
      co_chu: "vua",
      onDoiCoChu: () => {},
      onMoPhanAnh: () => {},
      onMoTraCuu: () => {},
      onOpenNotifications: () => {},
    }),
  );

describe("Cá nhân: the name is DISPLAY ONLY", () => {
  it("no name → \"Chưa xác định\", and no button to go and fetch one", () => {
    const html = personal(null);
    expect(XA_TN.chua_co_ten).toBe("Chưa xác định");
    expect(html).toContain(XA_TN.chua_co_ten);
    expect(html).not.toContain(OLD_BUTTON);
    expect(html).not.toContain(XA_TN.name_card_agree);
  });

  it("with a name → the name", () => {
    const html = personal(NAME);
    expect(html).toContain(NAME);
    expect(html).not.toContain(XA_TN.chua_co_ten);
  });
});

describe("Gửi phản ánh: the name field is pre-filled from entry, or empty and typed", () => {
  it("pre-fills with the entry name; otherwise empty — never a placeholder name that would be sent", () => {
    expect(blankForm(NAME).ho_ten).toBe(NAME);
    expect(blankForm(null).ho_ten).toBe("");
    // The rest of a fresh form is the same either way.
    expect({ ...blankForm(NAME), ho_ten: "" }).toEqual(blankForm(null));
  });

  it("the screen offers no Zalo button and takes no Zalo function", () => {
    const html = renderToStaticMarkup(
      createElement(CommuneSendScreen, {
        ten_xa: commune.ten,
        ho_ten: null,
        onBack: () => {},
        onSessionLost: () => {},
        onSent: () => {},
        onOpenPetition: () => {},
      }),
    );
    expect(html).not.toContain(OLD_BUTTON);
    expect("NutLayTen" in sendScreens).toBe(false);
  });
});

describe("no screen below the entry calls Zalo for the name", () => {
  const RAW = import.meta.glob("../**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }) as Record<
    string,
    string
  >;
  const sources = Object.entries(RAW).filter(([path]) => !path.includes(".test."));

  it("the old button component is gone everywhere", () => {
    expect(sources.length).toBeGreaterThan(10);
    expect(sources.filter(([, code]) => /\bNutLayTen\b/.test(code)).map(([p]) => p)).toEqual([]);
  });

  it("only the entry (`TrangXa.tsx`) holds the name bridge", () => {
    const holders = sources.filter(([, code]) => /\blay_ten\b/.test(code)).map(([p]) => p);
    expect(holders).toEqual(["./TrangXa.tsx"]);
  });
});

describe("stylesheet: commune cards are blocks, the discovery card keeps its own scope", () => {
  const nodeFs = "node:fs";
  const read = async () =>
    ((await import(/* @vite-ignore */ nodeFs)) as { readFileSync: (p: URL, e: "utf8") => string }).readFileSync(
      new URL("../../styles.css", import.meta.url),
      "utf8",
    );

  it("the unscoped `.xa-the` rule sets no layout — so a card of rows stacks them", async () => {
    const css = (await read()).replace(/\/\*[\s\S]*?\*\//g, " ");
    const bare = [...css.matchAll(/(?:^|})\s*\.xa-the\s*\{([^}]*)\}/g)].map((m) => m[1]!);
    expect(bare.length, "exactly one bare `.xa-the` rule").toBe(1);
    // `display: flex` / `padding` here turned "Tiện ích của tôi" into two squeezed side-by-side rows.
    expect(bare[0]).not.toMatch(/\bdisplay\s*:/);
    expect(bare[0]).not.toMatch(/(^|[\s;])padding\s*:/);
    expect(css).toMatch(/\.goi-y\s+\.xa-the\s*\{[^}]*display:\s*flex/);
  });
});
