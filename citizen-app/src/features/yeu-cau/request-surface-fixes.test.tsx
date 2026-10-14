/// <reference types="vite/client" />
import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * FIXES ON THE REQUEST SURFACE (owner-approved, 14/10/2026) — one block per finding of the read-only audit:
 *
 *   A  a 401 while sending / reading forgets the session (Liên hệ stops saying "Đã kết nối"), the sentence no
 *      longer promises the form is kept, and after logging in Liên hệ offers the way back
 *   D  "Vui lòng chọn ít nhất một giải pháp." is red only after a tap; before it, a neutral hint
 *   G  the login button stays disabled until the server exchange (`PhatHanhPhien`) settles
 *   H  network failure · server failure · no answer in time are three sentences, three next steps
 *   I  the note counter is no live region under the cap, one line instead of two, announced over the cap
 *
 * Flows that need a tap are MOUNTED on a minimal DOM (same shim and same reason as
 * `cong-dan/man/news-view-count.test.tsx`: the repo has no jsdom). Every value below is invented test data.
 */

const login = vi.hoisted(() => ({
  xin: vi.fn(),
  exchange: vi.fn(),
  readList: vi.fn(),
}));

vi.mock("../tinh-nang/zalo-api", async (importOriginal) => {
  const real = await importOriginal<typeof import("../tinh-nang/zalo-api")>();
  return { ...real, xinMaDangNhap: login.xin }; // vi-name-ok: existing export, mocked not renamed
});
vi.mock("../dang-nhap/goi-may-chu", async (importOriginal) => {
  const real = await importOriginal<typeof import("../dang-nhap/goi-may-chu")>();
  return { ...real, phatHanhPhien: login.exchange }; // vi-name-ok: existing export, mocked not renamed
});
vi.mock("../../api/goi-may-chu", async (importOriginal) => {
  const real = await importOriginal<typeof import("../../api/goi-may-chu")>();
  return { ...real, docYeuCauCuaToi: login.readList }; // vi-name-ok: existing export, mocked not renamed
});

import { docYeuCauCuaToi, guiYeuCau, type KetQuaGui } from "../../api/goi-may-chu"; // vi-name-ok: existing exports
import { TRAN_GHI_CHU, type YeuCauMoi } from "../../api/hop-dong-yeu-cau"; // vi-name-ok: existing exports
import { SOLUTIONS } from "../../content/company-profile";
import { ConnectWithVihat } from "../company-intro/ConnectWithVihat";
import { LOGIN_FROM_CONSULT, LOGIN_FROM_REQUESTS, loginReturnScreen, MOC_DANG_NHAP } from "../company-intro/dieu-huong";
import { forgetSessionOn401, NhaCungCapPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing export
import { KhungTinhNang } from "../tinh-nang/khung"; // vi-name-ok: existing export
import { KhoiDangNhap } from "../tinh-nang/LienHeTinhNang"; // vi-name-ok: existing export
import { noiDung } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export

import type { ConsultSteps } from "./consult-photos";
import { CONNECT_VIHAT, SMS_OFFERS, TU_VAN, YEU_CAU_CUA_TOI } from "./noi-dung";
import { OGhiChu } from "./OGhiChu"; // vi-name-ok: existing export
import { SmsOffersView } from "./SmsOffers";
import { CauTraLoi, consultLoginInvite, pickOneMessage, TuVanBaoGiaScreen } from "./TuVanBaoGiaScreen"; // vi-name-ok: existing exports
import { requestsLoginInvite, ThanDanhSach, YeuCauCuaToiScreen } from "./YeuCauCuaToiScreen"; // vi-name-ok: existing exports

const SESSION = { token: "tok-test", het_han: "2030-01-01T00:00:00Z" };
const ADDRESS = "https://vi-du.test/api/v1/requests";
const REQUEST: YeuCauMoi = { loai: "consult", quan_tam: ["messaging"], quy_mo: "", ghi_chu: "", nguon: "" }; // vi-name-ok: existing type
const FIRST_SOLUTION = SOLUTIONS[0]!.product ?? SOLUTIONS[0]!.headline;
const LOGIN_BUTTON = noiDung("dang-nhap").nut;
const LOGIN_WAITING = noiDung("dang-nhap").dang_cho;

const text = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/&amp;/g, "&").replace(/\s+/g, " ");

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * Minimal DOM (copied from `cong-dan/man/news-view-count.test.tsx`) — mounts, updates, unmounts; no events.
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

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
  set textContent(value: string) {
    for (const c of this.childNodes) c.parentNode = null;
    this.childNodes = [];
    if (value !== "" && this.ownerDocument !== null) this.appendChild(this.ownerDocument.createTextNode(value));
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
  override set textContent(value: string) {
    this.nodeValue = value;
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
  createTextNode(value: string) {
    return new FakeText(value, this);
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
    location: { protocol: "about:", search: "" },
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

beforeEach(() => {
  login.xin.mockReset();
  login.exchange.mockReset();
  login.readList.mockReset();
  // SmsOffers on Liên hệ reads the list too; by default it learns nothing.
  login.readList.mockResolvedValue({ kieu: "chua-khai-host" });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

const settle = () =>
  act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });

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

function findButton(host: FakeNode, match: (label: string) => boolean): FakeElement | null {
  const walk = (n: FakeNode): FakeElement | null => {
    if (n instanceof FakeElement && n.tagName === "BUTTON" && match(n.textContent)) return n;
    for (const c of n.childNodes) {
      const hit = walk(c);
      if (hit !== null) return hit;
    }
    return null;
  };
  return walk(host);
}

/** Taps through the props React keeps on the element — the shim has no events, and a `disabled` is NOT honoured. */
async function tap(host: FakeNode, match: (label: string) => boolean, name: string) {
  const button = findButton(host, match);
  expect(button, `no button "${name}"`).not.toBeNull();
  const key = Object.keys(button!).find((k) => k.startsWith("__reactProps$"))!;
  await act(async () => {
    ((button as unknown as Record<string, { onClick: () => void }>)[key]!).onClick();
  });
  await settle();
}

const is = (label: string) => (t: string) => t === label;

/* ════════════════════════════════════════════════════════════════════════════════════════════ */

describe("A — a 401 forgets the session, says the truth, and leaves a way back", () => {
  const consultSteps = (answer: KetQuaGui): ConsultSteps & { sent: unknown[] } => {
    const sent: unknown[] = [];
    return { sent, send: async (_b, r) => (sent.push(r), answer), upload: async () => ({ kind: "failed", failure: "server" }) };
  };

  it("the sentence no longer promises the form is kept", () => {
    for (const s of [TU_VAN.phien_het_han, YEU_CAU_CUA_TOI.phien_het_han]) {
      expect(s).not.toMatch(/vẫn được giữ|giữ lại/);
    }
    expect(TU_VAN.phien_het_han).toContain("chưa được gửi");
  });

  it("forgetSessionOn401 calls `forget` on 401 only, and returns the answer untouched", () => {
    for (const kieu of ["xong", "tu-choi", "khong-goi-duoc", "server-error", "unconfirmed", "chua-khai-host"]) {
      const forget = vi.fn();
      const answer = { kieu };
      expect(forgetSessionOn401(answer, forget)).toBe(answer);
      expect(forget, kieu).not.toHaveBeenCalled();
    }
    const forget = vi.fn();
    forgetSessionOn401({ kieu: "chua-dang-nhap" }, forget);
    expect(forget).toHaveBeenCalledTimes(1);
  });

  it("Tư vấn: 401 on send → login card says it expired, Liên hệ no longer says 'connected', login goes with the way back", async () => {
    const onDi = vi.fn();
    const steps = consultSteps({ kieu: "chua-dang-nhap" });
    const { host, unmount } = await mount(
      <NhaCungCapPhien phien_ban_dau={SESSION}>
        <TuVanBaoGiaScreen onDi={onDi} steps={steps} />
        <ConnectWithVihat />
      </NhaCungCapPhien>,
    );
    expect(host.textContent).toContain(CONNECT_VIHAT.connected);

    await tap(host, (t) => t.endsWith(FIRST_SOLUTION), FIRST_SOLUTION);
    await tap(host, is(TU_VAN.nut_gui), TU_VAN.nut_gui);

    expect(steps.sent).toHaveLength(1);
    expect(host.textContent).toContain(TU_VAN.session_expired_title);
    expect(host.textContent).toContain(TU_VAN.phien_het_han);
    expect(host.textContent, "Liên hệ still claims a dead session").not.toContain(CONNECT_VIHAT.connected);
    expect(findButton(host, is(LOGIN_BUTTON)), "the login action is offered again").not.toBeNull();

    await tap(host, is(TU_VAN.nut_toi_dang_nhap), TU_VAN.nut_toi_dang_nhap);
    expect(onDi).toHaveBeenLastCalledWith({ man: "contact", moc: LOGIN_FROM_CONSULT });
    await unmount();
  });

  it("Yêu cầu của tôi: 401 on read → same, and login goes with the way back to the list", async () => {
    login.readList.mockResolvedValue({ kieu: "chua-dang-nhap" });
    const onDi = vi.fn();
    const { host, unmount } = await mount(
      <NhaCungCapPhien phien_ban_dau={SESSION}>
        <YeuCauCuaToiScreen onDi={onDi} />
        <ConnectWithVihat />
      </NhaCungCapPhien>,
    );
    expect(host.textContent).toContain(YEU_CAU_CUA_TOI.session_expired_title);
    expect(host.textContent).toContain(YEU_CAU_CUA_TOI.phien_het_han);
    expect(host.textContent).not.toContain(CONNECT_VIHAT.connected);

    await tap(host, is(YEU_CAU_CUA_TOI.nut_toi_dang_nhap), YEU_CAU_CUA_TOI.nut_toi_dang_nhap);
    expect(onDi).toHaveBeenLastCalledWith({ man: "contact", moc: LOGIN_FROM_REQUESTS });
    await unmount();
  });

  it("before any 401 the invitations keep their ordinary words", () => {
    expect(consultLoginInvite(false).title).toBe(TU_VAN.can_dang_nhap_tieu_de);
    expect(requestsLoginInvite(false).title).toBe(YEU_CAU_CUA_TOI.can_dang_nhap_tieu_de);
    expect(consultLoginInvite(true).title).toBe(TU_VAN.session_expired_title);
    expect(requestsLoginInvite(true).title).toBe(YEU_CAU_CUA_TOI.session_expired_title);
  });

  it("the way-back anchors map to their screens; anything else is no way back", () => {
    expect(loginReturnScreen(LOGIN_FROM_CONSULT)).toBe("tu-van");
    expect(loginReturnScreen(LOGIN_FROM_REQUESTS)).toBe("yeu-cau");
    for (const other of [undefined, "", MOC_DANG_NHAP, "tu-van"]) expect(loginReturnScreen(other)).toBeNull();
  });

  it("connected + a way-back anchor → the back button first, and it goes back", async () => {
    for (const [moc, label, man] of [
      [LOGIN_FROM_CONSULT, CONNECT_VIHAT.back_to_consult, "tu-van"],
      [LOGIN_FROM_REQUESTS, CONNECT_VIHAT.back_to_requests, "yeu-cau"],
    ] as const) {
      const onDi = vi.fn();
      const { host, unmount } = await mount(
        <NhaCungCapPhien phien_ban_dau={SESSION}>
          <ConnectWithVihat moc={moc} onDi={onDi} />
        </NhaCungCapPhien>,
      );
      await tap(host, is(label), label);
      expect(onDi).toHaveBeenCalledWith({ man });
      await unmount();
    }
  });

  it("the anchor is a real `id` (the shell scrolls to it) in both states; no back button without it", () => {
    const loggedIn = (moc?: string) =>
      renderToStaticMarkup(
        <NhaCungCapPhien phien_ban_dau={SESSION}>
          <ConnectWithVihat moc={moc} onDi={() => {}} />
        </NhaCungCapPhien>,
      );
    const loggedOut = renderToStaticMarkup(<ConnectWithVihat moc={LOGIN_FROM_CONSULT} onDi={() => {}} />);
    expect(loggedIn(LOGIN_FROM_CONSULT)).toContain(`id="${LOGIN_FROM_CONSULT}"`);
    expect(loggedOut).toContain(`id="${LOGIN_FROM_CONSULT}"`);
    expect(text(loggedOut)).not.toContain(CONNECT_VIHAT.back_to_consult);
    for (const moc of [undefined, MOC_DANG_NHAP]) {
      const t = text(loggedIn(moc));
      expect(t).not.toContain(CONNECT_VIHAT.back_to_consult);
      expect(t).not.toContain(CONNECT_VIHAT.back_to_requests);
    }
  });
});

describe("D — 'pick at least one' is a hint until the person taps send", () => {
  it("pure: hint before, error after, nothing once something is picked", () => {
    expect(pickOneMessage(true, false)).toEqual({ text: TU_VAN.pick_one_hint, isError: false });
    expect(pickOneMessage(true, true)).toEqual({ text: TU_VAN.chua_chon_gi, isError: true });
    expect(pickOneMessage(false, true)).toBeNull();
  });

  it("on open: the neutral hint, no red sentence; a tap with nothing picked sends nothing and turns it red", async () => {
    const sent: unknown[] = [];
    const steps: ConsultSteps = {
      send: async (_b, r) => (sent.push(r), { kieu: "xong", ma_yeu_cau: "YC-TEST" }),
      upload: async () => ({ kind: "failed", failure: "server" }),
    };
    const { host, unmount } = await mount(
      <NhaCungCapPhien phien_ban_dau={SESSION}>
        <TuVanBaoGiaScreen steps={steps} />
      </NhaCungCapPhien>,
    );
    expect(host.textContent).toContain(TU_VAN.pick_one_hint);
    expect(host.textContent).not.toContain(TU_VAN.chua_chon_gi);

    await tap(host, is(TU_VAN.nut_gui), TU_VAN.nut_gui);
    expect(sent).toHaveLength(0);
    expect(host.textContent).toContain(TU_VAN.chua_chon_gi);

    await tap(host, (t) => t.endsWith(FIRST_SOLUTION), FIRST_SOLUTION);
    expect(host.textContent).not.toContain(TU_VAN.chua_chon_gi);
    expect(host.textContent).not.toContain(TU_VAN.pick_one_hint);
    await unmount();
  });

  it("the server-rendered form shows no `tn__loi` on open", () => {
    const markup = renderToStaticMarkup(
      <NhaCungCapPhien phien_ban_dau={SESSION}>
        <TuVanBaoGiaScreen />
      </NhaCungCapPhien>,
    );
    expect(markup).not.toContain("tn__loi");
  });
});

describe("G — the login button waits for the server exchange", () => {
  it("pure frame: `busy` disables the button and says so in words, whatever Zalo answered", () => {
    const markup = renderToStaticMarkup(
      <KhungTinhNang
        ma="dang-nhap"
        trang_thai={{ kieu: "xong", du_lieu: "x" }}
        onBam={() => {}}
        veKetQua={() => null}
        busy
      />,
    );
    expect(markup).toMatch(/<button[^>]*disabled=""/);
    expect(markup).toContain('aria-busy="true"');
    expect(text(markup)).toContain(LOGIN_WAITING);
  });

  it("mounted: disabled while the exchange runs, a second tap asks Zalo for nothing, enabled once it settles", async () => {
    login.xin.mockResolvedValue({ kieu: "xong", du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" } });
    let settleExchange: (v: unknown) => void = () => {};
    login.exchange.mockImplementation(() => new Promise((r) => (settleExchange = r)));

    const { host, unmount } = await mount(<KhoiDangNhap />);
    await tap(host, is(LOGIN_BUTTON), LOGIN_BUTTON);
    expect(login.xin).toHaveBeenCalledTimes(1);

    const waiting = findButton(host, is(LOGIN_WAITING));
    expect(waiting, "the button re-enabled while the exchange is still running").not.toBeNull();
    expect(waiting!.hasAttribute("disabled")).toBe(true);

    await tap(host, is(LOGIN_WAITING), LOGIN_WAITING);
    expect(login.xin, "a second tap mid-exchange asked Zalo again").toHaveBeenCalledTimes(1);

    await act(async () => settleExchange({ kieu: "khong-goi-duoc" }));
    await settle();
    const again = findButton(host, is(LOGIN_BUTTON));
    expect(again, "the button never came back after the exchange settled").not.toBeNull();
    expect(again!.hasAttribute("disabled")).toBe(false);
    await unmount();
  });
});

describe("H — network, server, and 'no answer in time' are three different sentences", () => {
  function answer(status: number, body: () => Promise<unknown>) {
    vi.stubGlobal("fetch", () => Promise.resolve({ status, ok: status >= 200 && status < 300, json: body }));
  }

  it("send: 5xx / 409 / 413 → server-error", async () => {
    for (const status of [500, 502, 409, 413]) {
      answer(status, () => Promise.resolve({}));
      expect(await guiYeuCau("t", REQUEST, ADDRESS), String(status)).toEqual({ kieu: "server-error" });
    }
  });

  it("send: a 2xx we cannot read → unconfirmed, never 'check your network'", async () => {
    answer(201, () => Promise.reject(new Error("not json")));
    expect(await guiYeuCau("t", REQUEST, ADDRESS)).toEqual({ kieu: "unconfirmed" });
    answer(201, () => Promise.resolve({ nothing: true }));
    expect(await guiYeuCau("t", REQUEST, ADDRESS)).toEqual({ kieu: "unconfirmed" });
  });

  it("send: our timeout → unconfirmed; the network refusing → khong-goi-duoc", async () => {
    vi.useFakeTimers();
    vi.stubGlobal(
      "fetch",
      (_a: string, init: RequestInit) =>
        new Promise((_r, reject) => init.signal?.addEventListener("abort", () => reject(new Error("aborted")))),
    );
    const pending = guiYeuCau("t", REQUEST, ADDRESS);
    await vi.advanceTimersByTimeAsync(15_000);
    expect(await pending).toEqual({ kieu: "unconfirmed" });
    vi.useRealTimers();

    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("Failed to fetch")));
    expect(await guiYeuCau("t", REQUEST, ADDRESS)).toEqual({ kieu: "khong-goi-duoc" });
  });

  it("read: non-2xx or an unreadable body → server-error", async () => {
    const real = (await vi.importActual<typeof import("../../api/goi-may-chu")>("../../api/goi-may-chu"))
      .docYeuCauCuaToi; // vi-name-ok: existing export
    answer(500, () => Promise.resolve({}));
    expect(await real("t", ADDRESS)).toEqual({ kieu: "server-error" });
    answer(200, () => Promise.reject(new Error("not json")));
    expect(await real("t", ADDRESS)).toEqual({ kieu: "server-error" });
    // The mocked import is the one the screens use in this file — keep it out of this case.
    expect(docYeuCauCuaToi).not.toHaveBeenCalled();
  });

  it("each branch says its own next step, never a code", () => {
    const sentence = (kieu: KetQuaGui) =>
      text(renderToStaticMarkup(<CauTraLoi ket_qua={kieu} onDangNhapLai={() => {}} onXemDanhSach={() => {}} />));
    const server = sentence({ kieu: "server-error" });
    const network = sentence({ kieu: "khong-goi-duoc" });
    const unconfirmed = sentence({ kieu: "unconfirmed" });
    expect(server).toContain(TU_VAN.server_error);
    expect(server).not.toContain("kiểm tra mạng");
    expect(network).toContain(TU_VAN.khong_goi_duoc);
    expect(unconfirmed).toContain(TU_VAN.unconfirmed);
    expect(unconfirmed, "no way to check before resending").toContain(TU_VAN.nut_xem_yeu_cau);
    for (const t of [server, network, unconfirmed]) expect(t).not.toMatch(/\b(4\d\d|5\d\d)\b|server-error|unconfirmed/);

    const list = text(renderToStaticMarkup(<ThanDanhSach ket_qua={{ kieu: "server-error" }} onDangNhapLai={() => {}} onGuiMoi={() => {}} />));
    expect(list).toContain(YEU_CAU_CUA_TOI.server_error);
    expect(list).not.toContain("kiểm tra mạng");

    for (const [kieu, words] of [
      ["server-error", SMS_OFFERS.server_error],
      ["unconfirmed", SMS_OFFERS.unconfirmed],
    ] as const) {
      const sms = renderToStaticMarkup(
        <SmsOffersView
          subscription="unknown"
          action={{ kind: "done", choice: "sms_promo", outcome: { kind: "sent", result: { kieu } } }}
          onChoose={() => {}}
        />,
      );
      expect(text(sms)).toContain(words);
    }
  });
});

describe("I — the note counter: no live region under the cap, one line, announced over it", () => {
  const ID = "yc-ghi-chu";
  const under = renderToStaticMarkup(<OGhiChu id={ID} gia_tri="" onDoi={() => {}} />);
  const over = renderToStaticMarkup(<OGhiChu id={ID} gia_tri={"ệ".repeat(TRAN_GHI_CHU + 5)} onDoi={() => {}} />);

  it("under the cap: no `role=\"status\"`, the count and the cap on ONE line", () => {
    expect(under).not.toContain('role="status"');
    expect(under).not.toContain("o-ghi-chu__tran");
    expect(text(under)).toContain(TU_VAN.con_lai(TRAN_GHI_CHU, TRAN_GHI_CHU));
    // One paragraph under the field — the old second one ("Tối đa N ký tự.") said the cap again.
    expect(under.match(/<p\b/g) ?? []).toHaveLength(1);
    // The alert region exists already, and is empty.
    expect(under).toContain('<div role="alert"></div>');
  });

  it("over the cap: the line sits inside the alert region and says what to do", () => {
    const alert = /<div role="alert">([\s\S]*?)<\/div>/.exec(over)?.[1] ?? "";
    expect(text(alert)).toContain(TU_VAN.qua_dai(5, TRAN_GHI_CHU));
    expect(TU_VAN.qua_dai(5, TRAN_GHI_CHU)).toContain("rút gọn");
  });

  it("aria-describedby always resolves to exactly one element", () => {
    for (const markup of [under, over]) {
      expect(markup).toContain(`aria-describedby="${ID}-dem"`);
      expect(markup.split(`id="${ID}-dem"`)).toHaveLength(2);
    }
  });
});
