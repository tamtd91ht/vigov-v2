/// <reference types="vite/client" />
import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE DEMO BUILD OF THE COMMUNE APP (`deploy.mjs --vao-thang --demo`, owner 30/09/2026 — dropped at
 * submission). This file compiles the screens AS THE DEMO BUILD DOES: `DEMO_BUILD` is mocked `true`.
 * `demo-mode-off.test.tsx` pins the other half — the same pieces with the real, `false` constant.
 *
 * What is pinned:
 *   · the name Zalo does not give becomes the sample name, silently; a real name still wins;
 *   · a PHONE-STEP failure lets the act run without a session, once asked, never asked again (no loop);
 *     a failure that is not about the phone (other commune, not connected) stops as in every build;
 *   · the act that needs the server ends in ONE demo sentence — no retry button, no invented code;
 *   · the send form starts with the agreed fake number (rule 3 #5) and says why step 1 has no list;
 *   · the band "Chế độ demo" is on the screen.
 */

vi.mock("../../lib/demo-build", () => ({ DEMO_BUILD: true }));

const commune = vi.hoisted(() => ({ ten: "Xã Thử Nghiệm", tinh: "Tỉnh Thử Nghiệm" }));

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const real = await importOriginal<typeof import("../api/goi-vigov")>();
  return {
    ...real,
    traXaTheoTenMien: vi.fn(async () => ({ kieu: "xong" as const, gia_tri: [commune] })),
    tinCuaXa: vi.fn(async () => ({ kieu: "khong-thay" as const })),
  };
});

import { guiPhanAnh } from "../api/goi-vigov";
import { taoLanGui } from "../api/lan-gui";
import type { CommuneAppSessionResult, ReopenWithPhoneResult } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { createSessionGate, DEMO_PROCEEDS, type SessionGateState } from "./commune-session";
import { DEMO_CITIZEN_NAME, DEMO_PHONE, DEMO_WORDS, DemoBand, demoPhonePrefill, withDemoName } from "./demo-mode";
import { PHONE_VERIFICATION, XA_TN } from "./noi-dung";
import { blankForm, CommuneSendScreen, sendBody, sendOutcome } from "./PhanAnhAppXa";
import { createPhoneVerification, PhoneVerificationPanel, type PhoneVerificationState } from "./phone-verification";
import type { KetQuaLayTen, NameRequestMode } from "./trai-nghiem";
import { SessionGateScreen, TrangXa } from "./TrangXa";

const REAL_NAME = "Trần Thị Bình"; // invented test data

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

const nameBridge = (answer: KetQuaLayTen) => {
  const calls: NameRequestMode[] = [];
  return {
    calls,
    bridge: async (mode: NameRequestMode) => {
      calls.push(mode);
      return answer;
    },
  };
};

/* ═══════════════════════════════════════ 1. NAME ═══════════════════════════════════════ */

describe("demo: the name Zalo did not give is the sample name, silently", () => {
  it("pure: no permission, declined, Zalo's error code, outside Zalo → the sample name, no error left", () => {
    expect(withDemoName({ kind: "needs-consent" })).toEqual({ kind: "settled", name: DEMO_CITIZEN_NAME });
    expect(withDemoName({ kind: "settled", name: null })).toEqual({ kind: "settled", name: DEMO_CITIZEN_NAME });
    expect(
      withDemoName({ kind: "settled", name: null, zalo: { capability: "name", code: -1402, transient: false } }),
    ).toEqual({ kind: "settled", name: DEMO_CITIZEN_NAME });
  });

  it("pure: a real name from Zalo wins; an unfinished request is left alone", () => {
    expect(withDemoName({ kind: "settled", name: REAL_NAME })).toEqual({ kind: "settled", name: REAL_NAME });
    expect(withDemoName({ kind: "checking" })).toEqual({ kind: "checking" });
    expect(withDemoName({ kind: "asking" })).toEqual({ kind: "asking" });
  });

  it.each<[string, KetQuaLayTen]>([
    ["no permission yet", { kieu: "tu-choi" }],
    ["Zalo refused with a code", { kieu: "khong-lay-duoc", zalo: { capability: "name", code: -1402, transient: false } }],
    ["nothing measured", { kieu: "khong-lay-duoc" }],
  ])("mounted, %s: greeting by the sample name, no card, no error line, the band", async (_, answer) => {
    const { bridge, calls } = nameBridge(answer);
    const { host, unmount } = await mount(createElement(TrangXa, { ten_mien: "thu.vigov.vn", lay_ten: bridge }));
    expect(calls).toEqual(["check"]); // never the dialog: the card that would ask is gone
    expect(host.textContent).toContain(XA_TN.xin_chao_ten(DEMO_CITIZEN_NAME));
    expect(host.textContent).not.toContain(XA_TN.name_card_title);
    expect(host.textContent).not.toContain("Mã hỗ trợ");
    expect(host.textContent).toContain(DEMO_WORDS.band_title);
    expect(host.textContent).toContain(commune.ten);
    await unmount();
  });

  it("mounted, Zalo gives the real name: the real name, not the sample", async () => {
    const { bridge } = nameBridge({ kieu: "xong", ho_ten: REAL_NAME });
    const { host, unmount } = await mount(createElement(TrangXa, { ten_mien: "thu.vigov.vn", lay_ten: bridge }));
    expect(host.textContent).toContain(XA_TN.xin_chao_ten(REAL_NAME));
    expect(host.textContent).not.toContain(DEMO_CITIZEN_NAME);
    await unmount();
  });
});

/* ═══════════════════════════════════════ 2. PHONE — THE GATE ═══════════════════════════════════════ */

function gateWith(answers: CommuneAppSessionResult[]) {
  const open = vi.fn(async () => answers.shift() ?? { kieu: "thu-lai" as const });
  const states: Array<SessionGateState | null> = [];
  const onProceed = vi.fn();
  const gate = createSessionGate(open, () => commune.ten, (s) => void states.push(s), { onProceed });
  return { gate, open, states, onProceed, last: () => states[states.length - 1] };
}

const PHONE_REFUSED = { capability: "phone", code: -1402, transient: false } as const;

describe("demo: Zalo refuses the phone → the screen asked for opens anyway, once asked, never again", () => {
  it.each<[string, CommuneAppSessionResult]>([
    ["Zalo lacks the phone permission (a code)", { kieu: "thu-lai", zalo: PHONE_REFUSED }],
    ["Zalo's dialog refused", { kieu: "tu-choi" }],
    ["Zalo busy", { kieu: "cho-lat" }],
    ["outside Zalo", { kieu: "ngoai-zalo" }],
    ["server could not verify the number", { kieu: "xong", token: "t", ten_xa: commune.ten, da_xac_thuc_so: false }],
  ])("%s: the act runs, without a session", async (_, answer) => {
    const { gate, open, onProceed, last } = gateWith([answer]);
    const run = vi.fn();
    gate.require(run);
    // Still explanation first (policy 3.3.4): nothing is asked of Zalo before the tap.
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
    await gate.allow();
    expect(open).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledTimes(1);
    expect(onProceed).toHaveBeenCalledTimes(1);
    expect(last()).toBeNull();
    expect(layPhienViGov()).toBeNull(); // NO session was made up
    expect(gate.demoWithoutSession()).toBe(true);
  });

  it("NO LOOP: every later act runs at once — no explanation, no Zalo call", async () => {
    const { gate, open, states } = gateWith([{ kieu: "thu-lai", zalo: PHONE_REFUSED }]);
    gate.require(() => {});
    await gate.allow();
    const before = states.length;
    const again = vi.fn();
    for (let i = 0; i < 3; i += 1) gate.require(again);
    expect(again).toHaveBeenCalledTimes(3);
    expect(open).toHaveBeenCalledTimes(1);
    expect(states.slice(before).every((s) => s === null)).toBe(true);
  });

  it("the server call then ends in the demo notice — one state, no retry", async () => {
    const { gate, last } = gateWith([{ kieu: "thu-lai", zalo: PHONE_REFUSED }]);
    gate.require(() => {});
    await gate.allow();
    gate.showDemoNotice();
    expect(last()).toEqual({ kieu: "demo" });
    gate.reset();
    expect(last()).toBeNull();
  });

  it.each<[string, CommuneAppSessionResult, string]>([
    ["another commune", { kieu: "xong", token: "t", ten_xa: "Xã Khác", da_xac_thuc_so: true }, "khac-xa"],
    ["app not connected", { kieu: "chua-ket-noi" }, "chua-ket-noi"],
    ["channel paused", { kieu: "tam-ngung" }, "tam-ngung"],
  ])("%s is NOT a phone failure: the act stops exactly as in every build", async (_, answer, outcome) => {
    const { gate, onProceed, last } = gateWith([answer]);
    const run = vi.fn();
    gate.require(run);
    await gate.allow();
    expect(run).not.toHaveBeenCalled();
    expect(onProceed).not.toHaveBeenCalled();
    expect(last()).toEqual({ kieu: "ket-qua", outcome });
    expect(gate.demoWithoutSession()).toBe(false);
    gate.showDemoNotice(); // does nothing when the demo did not proceed
    expect(last()).toEqual({ kieu: "ket-qua", outcome });
  });

  it("the proceed set is exactly the phone-step failures", () => {
    expect([...DEMO_PROCEEDS].sort()).toEqual(["cho-lat", "chua-xac-thuc-so", "ngoai-zalo", "thu-lai", "tu-choi"]);
  });

  it("Zalo DOES give the phone: a real session, the act runs, no demo state", async () => {
    const { gate, onProceed } = gateWith([{ kieu: "xong", token: "t", ten_xa: commune.ten, da_xac_thuc_so: true }]);
    const run = vi.fn();
    gate.require(run);
    await gate.allow();
    expect(run).toHaveBeenCalledTimes(1);
    expect(onProceed).not.toHaveBeenCalled();
    expect(layPhienViGov()).not.toBeNull();
    expect(gate.demoWithoutSession()).toBe(false);
  });
});

describe("demo: what the screens show without a session", () => {
  it.each(["submit", "mine", "lookup", "rate"] as const)(
    "the notice for %s: ONE demo sentence and the way back — no 'Đồng ý…' button, no code",
    (task) => {
      const html = renderToStaticMarkup(
        createElement(SessionGateScreen, {
          state: { kieu: "demo" },
          task,
          onAllow: () => {},
          onDecline: () => {},
          onClose: () => {},
        }),
      );
      expect(html).toContain(DEMO_WORDS.task[task]);
      expect(html).toContain(DEMO_WORDS.back);
      expect(html).not.toContain(PHONE_VERIFICATION.allow);
      expect(html).not.toContain("Mã hỗ trợ");
      expect(DEMO_WORDS.task[task]).toMatch(/^Chế độ demo: [^.]+\.$/); // one sentence
    },
  );

  it("submit without a session: no network, no invented code — the send screen goes to the notice path", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const form = { ...blankForm(DEMO_CITIZEN_NAME, demoPhonePrefill(true)), noi_dung: "Ổ gà trước cổng chợ" };
    const out = sendOutcome(await guiPhanAnh(taoLanGui(sendBody(form, null))));
    expect(out).toEqual({ kind: "session" }); // → `onSessionLost` → TrangXa → `showDemoNotice`, never a success
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it("the send form starts with the agreed fake number and the sample name", () => {
    expect(DEMO_PHONE).toBe("0900000000");
    expect(demoPhonePrefill(true)).toBe(DEMO_PHONE);
    expect(demoPhonePrefill(false)).toBe(""); // a real session: as today
    const form = blankForm(DEMO_CITIZEN_NAME, demoPhonePrefill(true));
    expect(form.dien_thoai).toBe(DEMO_PHONE);
    expect(form.ho_ten).toBe(DEMO_CITIZEN_NAME);
  });

  it("send screen step 1 without a session: says why there is no list, and lets the form be seen", () => {
    const props = {
      ten_xa: commune.ten,
      ho_ten: DEMO_CITIZEN_NAME,
      onBack: () => {},
      onSessionLost: () => {},
      onSent: () => {},
      onOpenPetition: () => {},
    };
    const demoHtml = renderToStaticMarkup(createElement(CommuneSendScreen, { ...props, demoWithoutSession: true }));
    expect(demoHtml).toContain(DEMO_WORDS.fields);
    expect(demoHtml).toContain(DEMO_WORDS.fields_continue);
    // With a real session the same screen loads the commune's list, as in every build.
    const realHtml = renderToStaticMarkup(createElement(CommuneSendScreen, props));
    expect(realHtml).not.toContain(DEMO_WORDS.fields);
  });
});

/* ═══════════════════════════════════ 2b. PHONE — THE RATING PATH ═══════════════════════════════════ */

describe("demo: the rating path's phone re-check fails → one sentence, final", () => {
  it.each<[string, ReopenWithPhoneResult]>([
    ["Zalo error", { kieu: "thu-lai", zalo: PHONE_REFUSED }],
    ["Zalo's dialog refused", { kieu: "tu-choi" }],
    ["outside Zalo", { kieu: "ngoai-zalo" }],
    ["not verified", { kieu: "xong", token: "t2", ten_xa: commune.ten, da_xac_thuc_so: false }],
  ])("%s: demo state, nothing re-run, the next 403 asks nothing", async (_, answer) => {
    datPhienViGov({ token: "t1", ten_xa: commune.ten });
    const reopen = vi.fn(async () => answer);
    const states: Array<PhoneVerificationState | null> = [];
    const m = createPhoneVerification(reopen, (s) => void states.push(s));
    const rerun = vi.fn();
    m.onPhoneRequired(rerun);
    expect(states.at(-1)).toEqual({ kieu: "hoi" });
    await m.allow();
    expect(states.at(-1)).toEqual({ kieu: "demo" });
    expect(rerun).not.toHaveBeenCalled();
    m.onPhoneRequired(rerun);
    expect(states.at(-1)).toEqual({ kieu: "demo" });
    expect(reopen).toHaveBeenCalledTimes(1);
  });

  it("another commune keeps its own sentence", async () => {
    datPhienViGov({ token: "t1", ten_xa: commune.ten });
    const states: Array<PhoneVerificationState | null> = [];
    const m = createPhoneVerification(
      async () => ({ kieu: "xong", token: "t2", ten_xa: "Xã Khác", da_xac_thuc_so: true }),
      (s) => void states.push(s),
    );
    m.onPhoneRequired(() => {});
    await m.allow();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "khac-xa" });
  });

  it("the panel: the rating sentence, no button", () => {
    const html = renderToStaticMarkup(
      createElement(PhoneVerificationPanel, { state: { kieu: "demo" }, task: "rate", onAllow: () => {}, onDecline: () => {} }),
    );
    expect(html).toContain(DEMO_WORDS.task.rate);
    expect(html).not.toContain("<button");
  });
});

/* ═══════════════════════════════════════ 3. THE BAND ═══════════════════════════════════════ */

describe("demo: the band", () => {
  it("is words, not colour alone", () => {
    const html = renderToStaticMarkup(createElement(DemoBand));
    expect(html).toContain(DEMO_WORDS.band_title);
    expect(html).toContain(DEMO_WORDS.band_body);
    expect(html).toContain('role="note"');
    expect(html).toContain('class="xa-demo"');
  });

  it("stands on the loading screen too, before the commune name has arrived", () => {
    const html = renderToStaticMarkup(createElement(TrangXa, { ten_mien: "thu.vigov.vn" }));
    expect(html).toContain(DEMO_WORDS.band_title);
  });
});
