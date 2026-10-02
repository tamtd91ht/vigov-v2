import { act, createElement, type ReactElement, StrictMode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * ẢNH "SAU KHI XỬ LÝ" on the citizen's own petition — commune app only (ADR 0047 row "Ảnh 'sau xử lý' của cán
 * bộ — THAY G8", (b)(c)(e)). What is pinned here:
 *
 *   the route              GET …/my-citizen-reports/{maTraCuu}/verification-photos, Bearer, no body, no "whose"
 *   404 is the lookup's    the same `khong-thay` as GET …/{maTraCuu} — one answer, never a second shape
 *   403 phone gate         `can-xac-thuc-so` → a retry line, NOT a second gate (the petition was just read)
 *   no empty frame         an empty list (the server's answer before `cho-dan-xac-nhan`) draws nothing
 *   before / after         both titled in WORDS, "Trước" above "Sau", "Trước" only when there is a "Sau"
 *   link refresh           the list is fetched again before the first link expires, by the SAME hook the
 *                          scene list uses (`usePhotoLinks`)
 *
 * Session and host are faked at module level, as in `scene-photos.test.tsx`.
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-test", ten_xa: "Xã Thử Nghiệm" } as { token: string; ten_xa: string } | null,
  host: "https://petitions.example.vn",
}));
vi.mock("../api/phien-vigov", () => ({ layPhienViGov: () => state.session }));
vi.mock("../api/dia-chi-vigov", () => ({
  diaChiViGov: (_service: string, path: string) => (state.host === "" ? "" : `${state.host}${path}`),
}));

import { listScenePhotos, listVerificationPhotos, traCuuPhieu } from "../api/goi-vigov"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import { type PhieuCuaToi, verificationPhotosAddress } from "../api/hop-dong-phan-anh"; // vi-name-ok: existing type, not renamed (rule 12 #3)

import { SCENE_PHOTOS, VERIFICATION_PHOTOS } from "./noi-dung";
import { PetitionBody } from "./PhanAnhAppXa";
import { ownPhotoListOutcome, VerificationPhotosView } from "./scene-photos";

const CODE = "PA7K2QX9M4TD";
const FUTURE = "2999-01-01T00:00:00Z";
const BASE = `https://petitions.example.vn/api/v1/my-citizen-reports/${CODE}`;
const AFTER_URL = `${BASE}/verification-photos`;
const OWN_URL = `${BASE}/photos`;
const noop = () => {};

const link = (id: string, expires = FUTURE) => ({
  id,
  content_type: "image/jpeg",
  size_bytes: 10,
  created_at: "2026-10-02T01:00:00Z",
  url: `https://store.example.vn/${id}?sig=x`,
  url_expires_at: expires,
});

function answer(status: number, body?: unknown) {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: async () => {
      if (body === undefined) throw new SyntaxError("no body");
      return body;
    },
    headers: { get: () => null },
  };
}

type Call = { url: string; init: RequestInit | undefined };
let calls: Call[] = [];

/** Answers by URL: the "after" list and the scene list each get their own reply. */
function routeFetch(routes: Record<string, () => ReturnType<typeof answer>>) {
  vi.stubGlobal("fetch", (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    const r = routes[url];
    return r === undefined ? Promise.reject(new TypeError("no route")) : Promise.resolve(r());
  });
}

const PETITION: PhieuCuaToi = {
  ma_tra_cuu: CODE,
  trang_thai: "cho-dan-xac-nhan",
  linh_vuc: "",
  nhan_linh_vuc: "",
  noi_dung: "Ổ gà",
  dia_chi: "",
  ho_ten_da_che: "",
  dien_thoai_da_che: "",
  an_danh: true,
  goc_dem_han: "2026-10-02T01:00:00Z",
  han_tiep_nhan: null,
  han_xu_ly_xong: null,
  ket_qua: "",
  ly_do: "",
  co_quan_nhan: "",
  rating: null,
  rated_at: null,
};

/* ─────────────────────────────── the route ─────────────────────────────── */

describe("the route — what goes out, and what each answer becomes", () => {
  beforeEach(() => {
    calls = [];
    state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" };
    state.host = "https://petitions.example.vn";
  });
  afterEach(() => vi.unstubAllGlobals());

  it("the address: under the lookup code, encoded once; empty host = empty address", () => {
    expect(verificationPhotosAddress(CODE)).toBe(AFTER_URL);
    expect(verificationPhotosAddress("a/b")).toBe(
      "https://petitions.example.vn/api/v1/my-citizen-reports/a%2Fb/verification-photos",
    );
    state.host = "";
    expect(verificationPhotosAddress(CODE)).toBe("");
  });

  it("GET with the session's Bearer, no body, no key — and nothing of the citizen in the URL", async () => {
    routeFetch({ [AFTER_URL]: () => answer(200, { items: [link("a"), link("b")] }) });
    const kq = await listVerificationPhotos(CODE);
    expect(kq).toEqual({
      kieu: "xong",
      gia_tri: [
        { id: "a", url: link("a").url, url_expires_at: FUTURE },
        { id: "b", url: link("b").url, url_expires_at: FUTURE },
      ],
    });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(AFTER_URL);
    expect(calls[0]!.init?.method).toBe("GET");
    expect(calls[0]!.init?.body).toBeUndefined();
    const h = (calls[0]!.init?.headers ?? {}) as Record<string, string>;
    expect(h["Authorization"]).toBe("Bearer tok-test");
    expect(h["Idempotency-Key"]).toBeUndefined();
  });

  it("an empty list (the server's answer before cho-dan-xac-nhan) is a success with no items", async () => {
    routeFetch({ [AFTER_URL]: () => answer(200, { items: [] }) });
    expect(await listVerificationPhotos(CODE)).toEqual({ kieu: "xong", gia_tri: [] });
  });

  it("403 chua_xac_thuc_so is the phone gate's branch; another 403 is a server fault", async () => {
    routeFetch({ [AFTER_URL]: () => answer(403, { code: "chua_xac_thuc_so" }) });
    expect(await listVerificationPhotos(CODE)).toEqual({ kieu: "can-xac-thuc-so" });
    routeFetch({ [AFTER_URL]: () => answer(403, { code: "khac" }) });
    expect(await listVerificationPhotos(CODE)).toEqual({ kieu: "loi-may-chu" });
  });

  it("404 is IDENTICAL to the lookup's 404 — one answer for none / someone else's / another commune's", async () => {
    const body = { code: "not_found", message: "x", trace_id: "t" };
    routeFetch({ [AFTER_URL]: () => answer(404, body), [BASE]: () => answer(404, body) });
    const after = await listVerificationPhotos(CODE);
    const lookup = await traCuuPhieu(CODE);
    expect(after).toEqual({ kieu: "khong-thay" });
    expect(after).toEqual(lookup);
  });

  it("401 · 500 · 503 · 429 · a malformed page · a dropped line — each its own branch, never a guess", async () => {
    const cases: Array<[ReturnType<typeof answer> | Error, string]> = [
      [answer(401, {}), "het-phien"],
      [answer(500, {}), "loi-may-chu"],
      [answer(503, {}), "kenh-chua-mo"],
      [answer(429, {}), "loi-may-chu"], // not in the contract
      [answer(200, { items: [{ id: "a", url: "http://plain/a", url_expires_at: FUTURE }] }), "loi-may-chu"],
      [new TypeError("Failed to fetch"), "loi-mang"],
    ];
    for (const [a, kieu] of cases) {
      vi.stubGlobal("fetch", () => (a instanceof Error ? Promise.reject(a) : Promise.resolve(a)));
      expect((await listVerificationPhotos(CODE)).kieu, kieu).toBe(kieu);
    }
  });

  it("no session, or no host: nothing goes out", async () => {
    routeFetch({});
    state.session = null;
    expect((await listVerificationPhotos(CODE)).kieu).toBe("chua-co-phien");
    state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" };
    state.host = "";
    expect((await listVerificationPhotos(CODE)).kieu).toBe("chua-cau-hinh");
    expect(calls).toHaveLength(0);
  });

  it("a session problem on the list is a retry line, not a second phone gate", () => {
    expect(ownPhotoListOutcome({ kieu: "can-xac-thuc-so" })).toEqual({ kind: "failed" });
    expect(ownPhotoListOutcome({ kieu: "het-phien" })).toEqual({ kind: "failed" });
  });
});

/* ─────────────────────────────── the block ─────────────────────────────── */

describe("the 'Sau khi xử lý' block — only when there are photos", () => {
  const view = (over: Partial<Parameters<typeof VerificationPhotosView>[0]>) =>
    renderToStaticMarkup(
      createElement(VerificationPhotosView, {
        list: { kind: "ready", items: [] },
        status: "cho-dan-xac-nhan",
        onRetry: noop,
        onImageError: noop,
        ...over,
      }),
    );
  const item = (id: string) => ({ id, url: `https://store.example.vn/${id}?sig=x`, url_expires_at: FUTURE });

  it("empty list and loading: no block at all (ADR 0047:254 (12) — no empty frame)", () => {
    for (const status of ["cho-dan-xac-nhan", "da-dong", "dang-xu-ly"]) {
      expect(view({ status })).toBe("");
      expect(view({ status, list: { kind: "loading" } })).toBe("");
    }
  });

  it("photos: the title in words and one described image each, linked by the server's URL", () => {
    const html = view({ list: { kind: "ready", items: [item("a"), item("b")] } });
    expect(html).toContain(VERIFICATION_PHOTOS.after_title);
    expect(html.match(/<img /g)).toHaveLength(2);
    expect(html).toContain(`alt="${VERIFICATION_PHOTOS.photo_alt(1)}"`);
    expect(html).toContain(`alt="${VERIFICATION_PHOTOS.photo_alt(2)}"`);
    expect(html).toContain('src="https://store.example.vn/a?sig=x"');
  });

  it("a failed load says so with Thử lại where the photos can exist — and draws nothing elsewhere", () => {
    for (const status of ["cho-dan-xac-nhan", "da-dong"]) {
      const html = view({ status, list: { kind: "failed" } });
      expect(html, status).toContain(VERIFICATION_PHOTOS.failed);
      expect(html, status).toContain(VERIFICATION_PHOTOS.retry);
    }
    for (const status of ["da-tiep-nhan", "dang-xu-ly", "da-xu-ly", "khong-tiep-nhan"]) {
      expect(view({ status, list: { kind: "failed" } }), status).toBe("");
    }
  });

  it("every sentence is plain words with a next step — no code, no status number", () => {
    expect(VERIFICATION_PHOTOS.failed).toMatch(/Thử lại/);
    for (const s of [VERIFICATION_PHOTOS.failed, VERIFICATION_PHOTOS.before_title, VERIFICATION_PHOTOS.after_title]) {
      expect(s).not.toMatch(/\d{3}|chua_xac_thuc_so|not_found/);
    }
  });
});

/* ─────────────────────────────── mounted: effects, refresh, both screens ─────────────────────────────── */

// Minimal DOM — the same shape as `name-at-entry.test.tsx`: enough for `react-dom/client` to mount, update and
// unmount a tree with SVG. It simulates no events.
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

/** Every element under `n` with this tag name, in document order. */
function elements(n: FakeNode, tag: string): FakeElement[] {
  const out: FakeElement[] = [];
  for (const c of n.childNodes) {
    if (c instanceof FakeElement && c.tagName === tag) out.push(c);
    out.push(...elements(c, tag));
  }
  return out;
}

describe("mounted — the detail and the lookup screen (both render `PetitionBody`)", () => {
  // `react-dom/client` decides "is there a DOM" WHEN THE MODULE LOADS — so the globals come first.
  let createRoot: typeof import("react-dom/client").createRoot;
  const saved = {
    window: (globalThis as Record<string, unknown>).window,
    document: (globalThis as Record<string, unknown>).document,
  };
  let reactErrors: unknown[][];

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

  beforeEach(() => {
    calls = [];
    state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" };
    state.host = "https://petitions.example.vn";
    reactErrors = [];
    vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void reactErrors.push(a));
    // Only the timers: the refresh is a `setTimeout`, and promises must still resolve on their own.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "Date"] });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.mocked(console.error).mockRestore();
    // Whatever the shim lacks, React says through `console.error` — a broken mount must not pass.
    expect(reactErrors).toEqual([]);
  });

  async function settle(ms = 0) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  }

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

  const body = (over: Partial<PhieuCuaToi> = {}, onSessionLost = vi.fn()) =>
    createElement(PetitionBody, { petition: { ...PETITION, ...over }, photos: { onSessionLost } });

  it("an empty 'after' list: no 'Sau khi xử lý', and the citizen's own block keeps its own title", async () => {
    routeFetch({
      [AFTER_URL]: () => answer(200, { items: [] }),
      [OWN_URL]: () => answer(200, { items: [link("own-1")] }),
    });
    const { host, unmount } = await mount(body());
    const text = host.textContent;
    expect(text).not.toContain(VERIFICATION_PHOTOS.after_title);
    expect(text).not.toContain(VERIFICATION_PHOTOS.before_title);
    expect(text).toContain(SCENE_PHOTOS.own_title);
    expect(calls.some((c) => c.url === AFTER_URL)).toBe(true); // the server decided, not the app
    await unmount();
  });

  it("with photos: 'Trước khi xử lý' above 'Sau khi xử lý', each image described in words", async () => {
    routeFetch({
      [AFTER_URL]: () => answer(200, { items: [link("after-1"), link("after-2")] }),
      [OWN_URL]: () => answer(200, { items: [link("own-1")] }),
    });
    const { host, unmount } = await mount(body({ trang_thai: "da-dong" }));
    const text = host.textContent;
    expect(text).toContain(VERIFICATION_PHOTOS.before_title);
    expect(text).toContain(VERIFICATION_PHOTOS.after_title);
    expect(text.indexOf(VERIFICATION_PHOTOS.before_title)).toBeLessThan(text.indexOf(VERIFICATION_PHOTOS.after_title));
    expect(text).not.toContain(SCENE_PHOTOS.own_title);
    const alts = elements(host, "IMG").map((i) => i.getAttribute("alt"));
    expect(alts).toEqual([SCENE_PHOTOS.photo_alt(1), VERIFICATION_PHOTOS.photo_alt(1), VERIFICATION_PHOTOS.photo_alt(2)]);
    await unmount();
  });

  it("'after' photos with no own photos: only 'Sau khi xử lý' — no empty 'before' frame", async () => {
    routeFetch({
      [AFTER_URL]: () => answer(200, { items: [link("after-1")] }),
      [OWN_URL]: () => answer(200, { items: [] }),
    });
    const { host, unmount } = await mount(body());
    expect(host.textContent).toContain(VERIFICATION_PHOTOS.after_title);
    expect(host.textContent).not.toContain(VERIFICATION_PHOTOS.before_title);
    expect(elements(host, "IMG")).toHaveLength(1);
    await unmount();
  });

  it("the links are fetched again before the first one expires, and the new links replace the old", async () => {
    const soon = new Date(Date.now() + 2 * 60_000).toISOString(); // refresh at 2 min − 30 s
    let round = 0;
    routeFetch({
      [AFTER_URL]: () => {
        round += 1;
        return answer(200, { items: [{ ...link("after-1", soon), url: `https://store.example.vn/after-1?sig=${round}` }] });
      },
      [OWN_URL]: () => answer(200, { items: [] }),
    });
    const { host, unmount } = await mount(body());
    const afterCalls = () => calls.filter((c) => c.url === AFTER_URL).length;
    const before = afterCalls();
    const firstSrc = elements(host, "IMG")[0]!.getAttribute("src");
    await settle(60_000);
    expect(afterCalls(), "refetched too early").toBe(before);
    await settle(60_000);
    expect(afterCalls(), "not refetched before the link expired").toBeGreaterThan(before);
    expect(elements(host, "IMG")[0]!.getAttribute("src")).not.toBe(firstSrc);
    await unmount();
  });

  it("403 chua_xac_thuc_so on the 'after' list: a retry line, and the phone gate is NOT opened again", async () => {
    const onSessionLost = vi.fn();
    routeFetch({
      [AFTER_URL]: () => answer(403, { code: "chua_xac_thuc_so" }),
      [OWN_URL]: () => answer(200, { items: [] }),
    });
    const { host, unmount } = await mount(body({}, onSessionLost));
    expect(host.textContent).toContain(VERIFICATION_PHOTOS.failed);
    expect(host.textContent).toContain(VERIFICATION_PHOTOS.retry);
    expect(onSessionLost).not.toHaveBeenCalled();
    await unmount();
  });

  it("404 on the 'after' list: the same retry line as any failure — nothing that tells 'not yours' apart", async () => {
    const seen: string[] = [];
    for (const a of [answer(404, { code: "not_found" }), answer(500, { code: "x" })]) {
      routeFetch({ [AFTER_URL]: () => a, [OWN_URL]: () => answer(200, { items: [] }) });
      const { host, unmount } = await mount(body());
      seen.push(host.textContent);
      await unmount();
    }
    expect(seen[0]).toContain(VERIFICATION_PHOTOS.failed);
    expect(seen[0]).toBe(seen[1]);
  });

  it("the scene list still uses its own route — the two lists are never mixed", async () => {
    routeFetch({ [OWN_URL]: () => answer(200, { items: [] }), [AFTER_URL]: () => answer(200, { items: [] }) });
    await listScenePhotos(CODE);
    expect(calls.map((c) => c.url)).toEqual([OWN_URL]);
  });
});
