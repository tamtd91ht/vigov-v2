/// <reference types="vite/client" />
import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE `--demo` BUILD OF THE COMMUNE APP (`deploy.mjs --vao-thang --demo`, owner 01/10/2026, ADR 0047 §6 —
 * replacing the 30/09 design). This file compiles the screens AS THAT BUILD DOES: `DEMO_BUILD` is mocked
 * `true`. `demo-mode-off.test.tsx` pins the other half with the real, `false` constant.
 *
 * What is pinned — the flag changes the IDENTITY SOURCE and nothing else:
 *   · the name is the fixed one and `getUserInfo` (`lay_ten`) is never called — no check, no card;
 *   · the gate opens the session AT ONCE — no explanation card, since there is no phone dialog — and every
 *     outcome after that is the ordinary one (a real session runs the act; a failure is the ordinary sentence);
 *   · the send form's phone field starts with the fixed number;
 *   · no screen carries the words "demo", "trình diễn", "trải nghiệm" — not a band, not a notice.
 * The opener itself (no `getPhoneNumber`, the `demoIdentity` body) is pinned in
 * `features/dang-nhap/demo-identity-session.test.ts`.
 */

vi.mock("../../lib/demo-build", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/demo-build")>()),
  DEMO_BUILD: true,
}));

const commune = vi.hoisted(() => ({ ten: "Xã Thử Nghiệm", tinh: "Tỉnh Thử Nghiệm" }));

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const real = await importOriginal<typeof import("../api/goi-vigov")>();
  return {
    ...real,
    traXaTheoTenMien: vi.fn(async () => ({ kieu: "xong" as const, gia_tri: [commune] })),
    tinCuaXa: vi.fn(async () => ({ kieu: "khong-thay" as const })),
  };
});

import type { CommuneAppSessionResult } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";
import { DEMO_BUILD, DEMO_CITIZEN_NAME, DEMO_CITIZEN_PHONE } from "../../lib/demo-build";

import { createSessionGate, type SessionGateState } from "./commune-session";
import { XA_TN } from "./noi-dung";
import { blankForm, CommuneSendScreen } from "./PhanAnhAppXa";
import type { KetQuaLayTen, NameRequestMode } from "./trai-nghiem";
import { TrangXa } from "./TrangXa";

/** The owner's words (01/10/2026): none of these on any screen, in any build. */
const BANNED = /demo|trình diễn|trải nghiệm/i;

/* ─────────────── minimal DOM (same technique as `name-at-entry.test.tsx`: no jsdom in this repo) ─────────────── */

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
    return new FakeElement(name.toUpperCase(), "http://www.w3.org/1999/xhtml", this);
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
  datPhienViGov(null);
  reactErrors = [];
  vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void reactErrors.push(a));
});

afterEach(() => {
  vi.mocked(console.error).mockRestore();
  datPhienViGov(null);
  expect(reactErrors).toEqual([]);
});

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

/** A `getUserInfo` bridge that records every call — in this build there must be none. */
function nameBridge() {
  const calls: NameRequestMode[] = [];
  return {
    calls,
    bridge: async (mode: NameRequestMode): Promise<KetQuaLayTen> => {
      calls.push(mode);
      return { kieu: "xong", ho_ten: "Trần Thị Bình" }; // invented test data
    },
  };
}

it("this file really compiles the `--demo` build", () => {
  expect(DEMO_BUILD).toBe(true);
  expect(DEMO_CITIZEN_NAME).toBe("Nguyễn Văn Hùng");
  expect(DEMO_CITIZEN_PHONE).toBe("0900000000"); // the repo's agreed fake number (rule 3 #5)
});

/* ═══════════════════════════════════════ 1. NAME ═══════════════════════════════════════ */

describe("--demo: the name is fixed, and Zalo is never asked for it", () => {
  it("mounted: `getUserInfo` never called, greeting by the fixed name, no name card, no demo word", async () => {
    const { bridge, calls } = nameBridge();
    const { host, unmount } = await mount(createElement(TrangXa, { ten_mien: "thu.vigov.vn", lay_ten: bridge }));
    expect(calls).toEqual([]); // not even the silent "check"
    expect(host.textContent).toContain(XA_TN.xin_chao_ten(DEMO_CITIZEN_NAME));
    expect(host.textContent).not.toContain("Trần Thị Bình"); // a Zalo name cannot come in: Zalo is not asked
    expect(host.textContent).not.toContain(XA_TN.name_card_title);
    expect(host.textContent).toContain(commune.ten);
    expect(host.textContent).not.toMatch(BANNED);
    await unmount();
  });

  it("the loading screen, before the commune name arrives, carries no demo word either", () => {
    const html = renderToStaticMarkup(createElement(TrangXa, { ten_mien: "thu.vigov.vn" }));
    expect(html).not.toMatch(BANNED);
  });
});

/* ═══════════════════════════════════════ 2. SESSION — THE GATE ═══════════════════════════════════════ */

function gateAtOnce(answers: CommuneAppSessionResult[]) {
  const open = vi.fn(async () => answers.shift() ?? { kieu: "thu-lai" as const });
  const states: Array<SessionGateState | null> = [];
  const gate = createSessionGate(open, () => commune.ten, (s) => void states.push(s), true);
  return { gate, open, states };
}

/** Let the `allow` that `require` started finish. */
const settle = () => new Promise((r) => setTimeout(r, 0));

describe("--demo: the gate opens the session at once — no explanation card, since there is no dialog", () => {
  it("a real session: opened on the first personal act, the act runs, never the 'hoi' card", async () => {
    const { gate, open, states } = gateAtOnce([{ kieu: "xong", token: "t", ten_xa: commune.ten, da_xac_thuc_so: true }]);
    const run = vi.fn();
    gate.require(run);
    expect(open).toHaveBeenCalledTimes(1); // at once, no tap needed
    await settle();
    expect(run).toHaveBeenCalledTimes(1);
    expect(layPhienViGov()).not.toBeNull(); // a REAL session — everything after goes to the server
    expect(states).toEqual([{ kieu: "dang-mo" }, null]);
    expect(states).not.toContainEqual({ kieu: "hoi" });
  });

  it("with a session: later acts run at once, the opener is not called again", async () => {
    const { gate, open } = gateAtOnce([{ kieu: "xong", token: "t", ten_xa: commune.ten, da_xac_thuc_so: true }]);
    gate.require(() => {});
    await settle();
    const again = vi.fn();
    gate.require(again);
    gate.require(again);
    expect(again).toHaveBeenCalledTimes(2);
    expect(open).toHaveBeenCalledTimes(1);
  });

  it.each<[string, CommuneAppSessionResult, string]>([
    ["server not reachable", { kieu: "thu-lai" }, "thu-lai"],
    ["app not connected (App ID not in DEMO_APP_IDS answers 4xx)", { kieu: "chua-ket-noi" }, "chua-ket-noi"],
    ["channel paused", { kieu: "tam-ngung" }, "tam-ngung"],
    ["another commune", { kieu: "xong", token: "t", ten_xa: "Xã Khác", da_xac_thuc_so: true }, "khac-xa"],
  ])("%s: the ORDINARY outcome — the act does not run, no session is made up", async (_, answer, outcome) => {
    const { gate, states } = gateAtOnce([answer]);
    const run = vi.fn();
    gate.require(run);
    await settle();
    expect(run).not.toHaveBeenCalled();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome });
    expect(layPhienViGov()).toBeNull();
  });
});

/* ═══════════════════════════════════════ 3. THE SEND FORM ═══════════════════════════════════════ */

describe("--demo: the send form starts with the fixed identity", () => {
  const props = {
    ten_xa: commune.ten,
    ho_ten: DEMO_CITIZEN_NAME,
    onBack: () => {},
    onSessionLost: () => {},
    onSent: () => {},
    onOpenPetition: () => {},
  };

  it("the phone field is pre-filled with the fixed number; the name with the fixed name", () => {
    const form = blankForm(DEMO_CITIZEN_NAME); // the very function the send screen opens with
    expect(form.dien_thoai).toBe(DEMO_CITIZEN_PHONE);
    expect(form.ho_ten).toBe(DEMO_CITIZEN_NAME);
  });

  it("rendered: step 1 is the ordinary field step, and no demo word anywhere", () => {
    const html = renderToStaticMarkup(createElement(CommuneSendScreen, props));
    expect(html).not.toMatch(BANNED);
  });
});
