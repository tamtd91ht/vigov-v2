import { act, createElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE PHẢN ÁNH CLUSTER OF THE CUSTOMER SHEET "App Xã", 12/10/2026 — rows 17, 19, 21, 22, 23, 24, and the call buttons
 * of the disaster alerts (row 11's defect). One case per owner decision, so a later change that undoes one turns a
 * named case red. Static markup where the markup is the decision; a mounted tree (a DOM shim, as
 * `hieu-ung-khong-phien.test.tsx` — the package has no jsdom) where the decision is a behaviour: the viewer opens,
 * closes on Escape, says one line on a broken link; the dialogs leave the screen's tree.
 */

import type { DisasterAlert } from "../api/hop-dong-cong-khai";
// vi-name-ok: existing contract types imported, not renamed (rule 12 #3)
import { type PhieuCuaToi, type PhieuCuaToiTomTat, RATING_REOPEN_THRESHOLD, type ScenePhotoLink } from "../api/hop-dong-phan-anh";
import { MAP_GESTURES } from "../map/scene-map";

import { ConsentDialog, DialogHostContext } from "./consent-dialog";
import { DisasterAlertCard } from "./disaster-alerts";
import { SCENE_PHOTOS, XA_GIAO_DIEN, XA_PA } from "./noi-dung";
import {
  completeLookupCode,
  filterOwnPetitions,
  type MyPetitions,
  PetitionBody,
  PetitionList,
  PetitionSearchResults,
  reopenedByRating,
  SendDone,
} from "./PhanAnhAppXa";
import { PhotoThumbList, SendDonePhotoStatus, type UploadJob } from "./scene-photos";

const noop = () => {};
const html = (el: ReactElement) => renderToStaticMarkup(el);

type Row = PhieuCuaToiTomTat; // vi-name-ok: alias of the existing contract type
type Petition = PhieuCuaToi; // vi-name-ok: alias of the existing contract type

const row = (over: Partial<Row>): Row => ({
  ma_tra_cuu: "PA-4K7M-92XR-BTVD",
  trang_thai: "dang-xu-ly",
  linh_vuc: "giao-thong",
  nhan_linh_vuc: "Hạ tầng giao thông",
  trich_noi_dung: "Ổ gà lớn trước cổng chợ",
  goc_dem_han: "2026-09-28T01:00:00Z",
  han_tiep_nhan: null,
  han_xu_ly_xong: null,
  rating: null,
  rated_at: null,
  ...over,
});

const ROWS = [
  row({}),
  row({
    ma_tra_cuu: "PA-ZZ22-HH33-KK44",
    linh_vuc: "moi-truong",
    nhan_linh_vuc: "Môi trường",
    trich_noi_dung: "Rác tồn đọng đầu ngõ 12",
  }),
];

const ready = (items: readonly Row[], hasMore = false): MyPetitions => ({
  kind: "ready",
  items,
  cursor: hasMore ? "c" : "",
  hasMore,
  loadingMore: false,
  moreFailure: null,
});

const PETITION: Petition = {
  ma_tra_cuu: "PA-4K7M-92XR-BTVD",
  trang_thai: "dang-xu-ly",
  linh_vuc: "giao-thong",
  nhan_linh_vuc: "Hạ tầng giao thông",
  noi_dung: "Ổ gà lớn trước cổng chợ.",
  dia_chi: "Trước cổng chợ",
  ho_ten_da_che: "",
  dien_thoai_da_che: "",
  an_danh: true,
  goc_dem_han: "2026-09-28T01:00:00Z",
  han_tiep_nhan: "2026-09-28T03:00:00Z",
  han_xu_ly_xong: null,
  ket_qua: "",
  ly_do: "",
  co_quan_nhan: "",
  rating: null,
  rated_at: null,
};

/** The host elements of a hookless element tree, depth first (components are not expanded). */
function hostElements(node: ReactNode): ReactElement<Record<string, unknown>>[] {
  if (Array.isArray(node)) return node.flatMap(hostElements);
  if (!isValidElement(node)) return [];
  const el = node as ReactElement<Record<string, unknown>>;
  const own = typeof el.type === "string" ? [el] : [];
  return [...own, ...hostElements(el.props["children"] as ReactNode)];
}

const searchProps = (state: MyPetitions, query: string, onLookupCode = vi.fn()) => ({
  state,
  query,
  onOpenPetition: noop,
  onLookupCode,
  onOpen: noop,
  onRetry: noop,
  onLoadMore: noop,
});

/* ═══════════════════════════════ ROW 17 — the search box of the Phản ánh tab ═══════════════════════════════ */

describe("row 17 — search of the citizen's OWN petitions, and one lookup for a complete code", () => {
  it("filters on the phone: field, excerpt, without case or diacritics; the code without dashes from 3 characters", () => {
    expect(filterOwnPetitions(ROWS, "moi truong").map((p) => p.ma_tra_cuu)).toEqual(["PA-ZZ22-HH33-KK44"]);
    expect(filterOwnPetitions(ROWS, "O GA").map((p) => p.ma_tra_cuu)).toEqual(["PA-4K7M-92XR-BTVD"]);
    expect(filterOwnPetitions(ROWS, "rác TỒN").map((p) => p.ma_tra_cuu)).toEqual(["PA-ZZ22-HH33-KK44"]);
    expect(filterOwnPetitions(ROWS, "#4k7m92").map((p) => p.ma_tra_cuu)).toEqual(["PA-4K7M-92XR-BTVD"]);
    expect(filterOwnPetitions(ROWS, "  ")).toBe(ROWS);
    // Every code starts with "PA": two letters must not match the whole list through the prefix.
    expect(filterOwnPetitions(ROWS, "pa")).toEqual([]);
    expect(filterOwnPetitions(ROWS, "khong co gi")).toEqual([]);
  });

  it("keywords, not a phrase (owner 12/10/2026): every typed word, any order, no diacritics, punctuation ignored", () => {
    const rows = [
      { ...ROWS[0]!, ma_tra_cuu: "PA-RRRR-AAAA-CCCC", trich_noi_dung: "Rác thải ở hẻm 13 quá nhiều, gây mùi hôi" },
      { ...ROWS[0]!, ma_tra_cuu: "PA-HHHH-EEEE-MMMM", trich_noi_dung: "Đèn đường hẻm 130 hỏng" },
    ];
    const codes = (q: string) => filterOwnPetitions(rows, q).map((p) => p.ma_tra_cuu);
    for (const q of ["hem 13", "rac thai", "Rác Thải", "mui hoi rac", "13 hem", "rac th", "hẻm 13,", "gay mui"]) {
      expect(codes(q), q).toEqual(["PA-RRRR-AAAA-CCCC"]);
    }
    // A number is a whole word: "13" is not alley 130, and "130" is not alley 13.
    expect(codes("hem 130")).toEqual(["PA-HHHH-EEEE-MMMM"]);
    expect(codes("hem")).toEqual(["PA-RRRR-AAAA-CCCC", "PA-HHHH-EEEE-MMMM"]);
    // Every word must be there: one missing word drops the row.
    expect(codes("rac thai den")).toEqual([]);
  });

  it("a complete code, typed any way an elderly citizen reads it off a slip → the issued shape; anything else → null", () => {
    for (const typed of ["PA-4K7M-92XR-BTVD", "#pa4k7m92xrbtvd", " pa 4k7m 92xr btvd ", "pa-4k7m92xr-btvd"]) {
      expect(completeLookupCode(typed), typed).toBe("PA-4K7M-92XR-BTVD");
    }
    // Too short, a letter the alphabet lacks (O, I, L, U, 0, 1), another prefix: not a code.
    for (const typed of ["PA-4K7M-92XR", "PA-4K7M-92XR-BTV0", "PA-4K7M-92XR-BTVO", "PB-4K7M-92XR-BTVD", "rác"]) {
      expect(completeLookupCode(typed), typed).toBeNull();
    }
  });

  it("matching petitions only; nothing matching → one short line, never anybody else's list", () => {
    const one = html(createElement(PetitionSearchResults, searchProps(ready(ROWS), "môi trường")));
    expect(one).toContain("Rác tồn đọng đầu ngõ 12");
    expect(one).not.toContain("Ổ gà lớn");
    const none = html(createElement(PetitionSearchResults, searchProps(ready(ROWS), "đèn đường")));
    expect(none).toContain(XA_PA.search_empty);
    expect(none).toContain(XA_PA.search_empty_hint);
    expect(none).not.toContain("xa-petition-card");
    expect(none).not.toContain(XA_PA.lookup_code_why);
  });

  it("more pages exist: the result says it looked at the loaded ones only", () => {
    const partial = html(createElement(PetitionSearchResults, searchProps(ready(ROWS, true), "môi trường")));
    expect(partial).toContain(XA_PA.search_loaded_only);
    const unsearched = html(createElement(PetitionSearchResults, searchProps(ready(ROWS, true), "")));
    expect(unsearched).not.toContain(XA_PA.search_loaded_only);
  });

  it("a complete code NOT loaded → ONE lookup action with the issued code; a loaded one → just its card", () => {
    const onLookupCode = vi.fn();
    const tree = PetitionSearchResults(searchProps(ready(ROWS), "pa-aaaa-bbbb-cccc", onLookupCode));
    const buttons = hostElements(tree).filter((e) => e.type === "button");
    expect(buttons).toHaveLength(1);
    (buttons[0]!.props["onClick"] as () => void)();
    expect(onLookupCode).toHaveBeenCalledWith("PA-AAAA-BBBB-CCCC");
    const offered = html(createElement(PetitionSearchResults, searchProps(ready(ROWS), "pa-aaaa-bbbb-cccc")));
    expect(offered).toContain(XA_PA.lookup_code_button("PA-AAAA-BBBB-CCCC"));
    // The empty-result line would contradict the offer: not both.
    expect(offered).not.toContain(XA_PA.search_empty);

    const loaded = html(createElement(PetitionSearchResults, searchProps(ready(ROWS), "PA-4K7M-92XR-BTVD")));
    expect(loaded).not.toContain(XA_PA.lookup_code_why);
    expect(loaded).toContain("Ổ gà lớn trước cổng chợ");
  });

  it("before any session: the card that asks stays, and a complete code can still be looked up (phone-less, ADR 0080)", () => {
    const idle = html(createElement(PetitionSearchResults, searchProps({ kind: "idle" }, "PA-AAAA-BBBB-CCCC")));
    expect(idle).toContain(XA_PA.need_session_button);
    expect(idle).toContain(XA_PA.lookup_code_button("PA-AAAA-BBBB-CCCC"));
  });

  it("the tab: the search box at the top like the directory, and no 'Tra cứu phiếu' row any more", () => {
    const tab = html(
      createElement(PetitionList, {
        state: ready(ROWS),
        onOpenPetition: noop,
        onLookupCode: noop,
        onOpen: noop,
        onRetry: noop,
        onLoadMore: noop,
      }),
    );
    expect(tab.startsWith('<div class="xa-tim"><div class="xa-search">')).toBe(true);
    expect(tab).toContain('id="xa-tim-phan-anh"');
    expect(tab).toContain(`placeholder="${XA_PA.search_placeholder}"`);
    expect(tab).not.toContain(XA_PA.tra_cuu_tieu_de);
  });
});

/* ═══════════════════════════════ ROW 21 — the reopening, in the citizen's own stars ═══════════════════════════════ */

describe("row 21 — 'Bà con đánh giá n sao — phiếu được mở lại để xử lý tiếp.'", () => {
  it("shown only for the citizen's own low rating while the petition is back in processing", () => {
    expect(RATING_REOPEN_THRESHOLD).toBe(2);
    expect(reopenedByRating({ ...PETITION, rating: 2, rated_at: "2026-10-01T01:00:00Z" })).toBe(2);
    expect(reopenedByRating({ ...PETITION, rating: 1, rated_at: "x", trang_thai: "da-chuyen-xu-ly" })).toBe(1);
    // Not rated; rated 3–5 (no reopening); handled again or closed since; an unknown status (no guessed group).
    expect(reopenedByRating(PETITION)).toBeNull();
    expect(reopenedByRating({ ...PETITION, rating: 3, rated_at: "x" })).toBeNull();
    for (const status of ["da-xu-ly", "cho-dan-xac-nhan", "da-dong", "khong-tiep-nhan", "trang-thai-moi"]) {
      expect(reopenedByRating({ ...PETITION, rating: 2, rated_at: "x", trang_thai: status }), status).toBeNull();
    }
  });

  it("the body says it in words under the summary; the threshold itself is never printed", () => {
    const body = html(createElement(PetitionBody, { petition: { ...PETITION, rating: 2, rated_at: "2026-10-01T01:00:00Z" } }));
    expect(body).toContain("Bà con đánh giá 2 sao — phiếu được mở lại để xử lý tiếp.");
    expect(body).not.toMatch(/1[–-]2 sao|1 hoặc 2 sao/);
    expect(html(createElement(PetitionBody, { petition: PETITION }))).not.toContain("phiếu được mở lại");
  });
});

/* ═══════════════════════════════ ROW 22 — one finger moves the map ═══════════════════════════════ */

describe("row 22 — the map pans with one finger", () => {
  it("cooperative gestures are OFF; rotation and tilt stay off", () => {
    expect(MAP_GESTURES).toEqual({ dragRotate: false, pitchWithRotate: false, touchPitch: false, cooperativeGestures: false });
  });
});

/* ═══════════════════════════════ ROW 23 — the commune on the send button ═══════════════════════════════ */

describe("row 23 — the send button names the commune verbatim (README non-negotiable #5)", () => {
  it("the label carries the name as given, nothing shortened", () => {
    const long = "Phường Hòa Thuận Đông Mở Rộng Thuộc Thành Phố";
    expect(XA_PA.send_to(long)).toBe(`Gửi phản ánh tới ${long}`);
  });
});

/* ═══════════════════════════════ ROW 24 — "Đã gửi phản ánh" as the prototype ═══════════════════════════════ */

describe("row 24 — the done screen: no acknowledge line, the resolve line kept, a failed photo said", () => {
  const job = (over: Partial<UploadJob>): UploadJob => ({ key: "photo-0", index: 1, status: "stored", ...over });

  it("no 'Cán bộ xã sẽ xem phiếu trước …'; 'Dự kiến xử lý xong trước …' when the server returned it", () => {
    const done = html(
      createElement(SendDone, {
        communeName: "Xã Thử",
        petition: { ...PETITION, han_xu_ly_xong: "2026-10-02T09:00:00Z" },
        onFollow: noop,
        onHome: noop,
      }),
    );
    expect(done).not.toContain("Cán bộ xã sẽ xem phiếu");
    expect(done).toContain(XA_PA.resolve_by("16h ngày 2/10"));
    expect(done).toContain("#PA-4K7M-92XR-BTVD"); // the code WHOLE on this screen
  });

  it("photos: nothing when all arrived, one status line while going, the failure + retry when one did not", () => {
    expect(html(createElement(SendDonePhotoStatus, { jobs: [], onRetry: noop }))).toBe("");
    expect(html(createElement(SendDonePhotoStatus, { jobs: [job({}), job({ key: "p1", index: 2 })], onRetry: noop }))).toBe("");
    const going = html(
      createElement(SendDonePhotoStatus, { jobs: [job({}), job({ key: "p1", index: 2, status: "sending" })], onRetry: noop }),
    );
    expect(going).toContain(SCENE_PHOTOS.sending(2, 2));
    const failed = html(
      createElement(SendDonePhotoStatus, {
        jobs: [job({}), job({ key: "p1", index: 2, status: "failed", failure: "network", canRetry: true })],
        onRetry: noop,
      }),
    );
    expect(failed).toContain('role="alert"');
    expect(failed).toContain(SCENE_PHOTOS.done_failed(1, 2));
    expect(failed).toContain(SCENE_PHOTOS.failures.network);
    expect(failed).toContain(SCENE_PHOTOS.retry(2));
    // A final refusal says why, with no button that would meet the same answer.
    const final = html(
      createElement(SendDonePhotoStatus, {
        jobs: [job({ status: "failed", failure: "not-a-photo", canRetry: false })],
        onRetry: noop,
      }),
    );
    expect(final).toContain(SCENE_PHOTOS.failures["not-a-photo"]);
    expect(final).not.toContain(SCENE_PHOTOS.retry(1));
  });
});

/* ═══════════════════════════════ row 11's defect on the alert screen ═══════════════════════════════ */

const ALERT: DisasterAlert = {
  id: "a1",
  kind: "bao",
  title: "Bão số 5",
  headline: "",
  level: "",
  detail: "",
  effectiveFrom: "2026-10-12T01:00:00Z",
  dangerZone: [],
  evacuationPoints: [],
  emergencyContacts: [{ role: "Trực ban PCTT xã", name: "", phone: "0900 000 000" }],
};

describe("disaster alerts — the call button uses the shell's dialer inside Zalo, `tel:` outside", () => {
  it("no dialer → a tel: link; a dialer → a button, no tel: link", () => {
    expect(html(createElement(DisasterAlertCard, { alert: ALERT, communeName: "Xã Thử" }))).toContain('href="tel:0900000000"');
    const zalo = html(createElement(DisasterAlertCard, { alert: ALERT, communeName: "Xã Thử", dial: async () => true }));
    expect(zalo).not.toContain("tel:");
    expect(zalo).toContain('<button type="button" class="xa-call-button" aria-label="Gọi Trực ban PCTT xã, số 0900 000 000">');
  });
});

/* ═══════════════════════════════ MOUNTED — row 19 (viewer), row 23 (portal), the dialer ═══════════════════════════════ */

class FakeNode {
  childNodes: FakeNode[] = [];
  parentNode: FakeNode | null = null;
  readonly listeners = new Map<string, Array<{ fn: (e: unknown) => void; capture: boolean }>>();
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
  addEventListener(type: string, fn: (e: unknown) => void, opt?: boolean | { capture?: boolean }) {
    const capture = typeof opt === "boolean" ? opt : opt?.capture === true;
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), { fn, capture }]);
  }
  removeEventListener(type: string, fn: (e: unknown) => void, opt?: boolean | { capture?: boolean }) {
    const capture = typeof opt === "boolean" ? opt : opt?.capture === true;
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((l) => l.fn !== fn || l.capture !== capture));
  }
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

const HTML_NS = "http://www.w3.org/1999/xhtml";

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
  focus() {
    if (this.ownerDocument !== null) this.ownerDocument.activeElement = this;
  }
}

class FakeDocument extends FakeNode {
  readonly documentElement: FakeElement;
  readonly body: FakeElement;
  activeElement: FakeElement | null = null;
  defaultView: unknown = null;
  oninput: null = null;
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

function all(n: FakeNode): FakeElement[] {
  return n.childNodes.flatMap((c) => (c instanceof FakeElement ? [c, ...all(c)] : []));
}

function dispatch(target: FakeElement, type: string, extra: Record<string, unknown> = {}) {
  const path: FakeNode[] = [];
  for (let n: FakeNode | null = target; n !== null; n = n.parentNode) path.push(n);
  let stopped = false;
  const event = {
    type,
    target,
    bubbles: true,
    cancelable: true,
    button: 0,
    defaultPrevented: false,
    timeStamp: Date.now(),
    preventDefault() {
      this.defaultPrevented = true;
    },
    stopPropagation() {
      stopped = true;
    },
    ...extra,
  };
  for (const n of [...path].reverse()) for (const l of n.listeners.get(type) ?? []) if (l.capture && !stopped) l.fn(event);
  for (const n of path) for (const l of n.listeners.get(type) ?? []) if (!l.capture && !stopped) l.fn(event);
}

describe("mounted — the viewer, the portal, the dialer", () => {
  let createRoot: typeof import("react-dom/client").createRoot;
  const saved = {
    window: (globalThis as Record<string, unknown>).window,
    document: (globalThis as Record<string, unknown>).document,
  };
  let reactErrors: unknown[][];
  const doc = () => (globalThis as unknown as { document: FakeDocument }).document;

  beforeAll(async () => {
    const d = new FakeDocument();
    const win: Record<string, unknown> = {
      document: d,
      event: undefined,
      HTMLIFrameElement: class {},
      addEventListener() {},
      removeEventListener() {},
      location: { protocol: "about:" },
    };
    win.top = win;
    win.self = win;
    d.defaultView = win;
    Object.assign(globalThis, { window: win, document: d, IS_REACT_ACT_ENVIRONMENT: true });
    createRoot = (await import("react-dom/client")).createRoot;
  });

  afterAll(() => {
    Object.assign(globalThis, { window: saved.window, document: saved.document, IS_REACT_ACT_ENVIRONMENT: undefined });
  });

  beforeEach(() => {
    reactErrors = [];
    vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void reactErrors.push(a));
  });

  afterEach(() => {
    vi.mocked(console.error).mockRestore();
    // Whatever the shim lacks, React says through `console.error` — a broken mount must not pass.
    expect(reactErrors).toEqual([]);
  });

  async function mount(el: ReactElement) {
    const host = doc().createElement("div");
    doc().body.appendChild(host);
    const root = createRoot(host as unknown as Element);
    await act(async () => {
      root.render(el);
    });
    return {
      host,
      rerender: (next: ReactElement) =>
        act(async () => {
          root.render(next);
        }),
      unmount: () => act(() => root.unmount()),
    };
  }

  async function press(el: FakeElement, type = "click", extra: Record<string, unknown> = {}) {
    await act(async () => {
      dispatch(el, type, extra);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }

  const link = (id: string, url: string): ScenePhotoLink => ({ id, url, url_expires_at: "2026-10-12T09:00:00Z" });
  const viewer = () => all(doc()).find((e) => e.getAttribute("role") === "dialog") ?? null;
  const thumbs = (host: FakeElement) => all(host).filter((e) => e.getAttribute("class") === "xa-photo-open");
  const imageOf = (el: FakeElement) => all(el).find((e) => e.tagName === "IMG") ?? null;
  const list = (items: readonly ScenePhotoLink[]) =>
    createElement(PhotoThumbList, {
      items,
      alt: SCENE_PHOTOS.photo_alt,
      listClass: "xa-photos__list",
      imageClass: "xa-photos__thumb",
      onImageError: noop,
    });

  it("row 19: a tap opens the photo full screen, OUTSIDE the screen's tree; 'Đóng ảnh' and Escape close it", async () => {
    const { host, unmount } = await mount(list([link("p1", "https://s.example/a?sig=1"), link("p2", "https://s.example/b?sig=1")]));
    expect(viewer()).toBeNull();
    await press(thumbs(host)[1]!);
    const open = viewer()!;
    expect(open.getAttribute("aria-modal")).toBe("true");
    expect(host.contains(open)).toBe(false);
    expect(imageOf(open)!.getAttribute("class")).toBe("xa-viewer__image");
    expect(imageOf(open)!.getAttribute("src")).toBe("https://s.example/b?sig=1");
    const close = all(open).find((e) => e.getAttribute("aria-label") === SCENE_PHOTOS.viewer_close)!;
    // The focus is on "Đóng ảnh".
    expect(doc().activeElement).toBe(close);
    expect(open.textContent).toContain(SCENE_PHOTOS.viewer_position(2, 2));

    await press(close);
    expect(viewer()).toBeNull();

    await press(thumbs(host)[0]!);
    expect(viewer()).not.toBeNull();
    await press(viewer()!, "keydown", { key: "Escape" });
    expect(viewer()).toBeNull();
    await unmount();
  });

  it("row 19: 'Ảnh sau' steps within the block; the link shown is the list's CURRENT one", async () => {
    const m = await mount(list([link("p1", "https://s.example/a?sig=1"), link("p2", "https://s.example/b?sig=1")]));
    await press(thumbs(m.host)[0]!);
    const next = all(viewer()!).find((e) => e.tagName === "BUTTON" && e.textContent === SCENE_PHOTOS.viewer_next)!;
    await press(next);
    expect(imageOf(viewer()!)!.getAttribute("src")).toBe("https://s.example/b?sig=1");
    // The block refetched its links (they expire ≤ 15 min): the open viewer shows the new one.
    await m.rerender(list([link("p1", "https://s.example/a?sig=2"), link("p2", "https://s.example/b?sig=2")]));
    expect(imageOf(viewer()!)!.getAttribute("src")).toBe("https://s.example/b?sig=2");
    await m.unmount();
  });

  it("row 19: a link that fails → ONE sentence, no image, no retry of the same link", async () => {
    const { host, unmount } = await mount(list([link("p1", "https://s.example/expired")]));
    await press(thumbs(host)[0]!);
    await press(imageOf(viewer()!)!, "error");
    const open = viewer()!;
    expect(open.textContent).toContain(SCENE_PHOTOS.viewer_failed);
    expect(imageOf(open)).toBeNull();
    expect(all(open).some((e) => e.getAttribute("aria-label") === SCENE_PHOTOS.viewer_close)).toBe(true);
    await unmount();
  });

  it("row 23: the permission question is mounted in the app's dialog host, never inside the scrolling screen", async () => {
    const appRoot = doc().createElement("div");
    doc().body.appendChild(appRoot);
    const dialog = createElement(ConsentDialog, { id: "q", question: SCENE_PHOTOS.camera_question, onAllow: noop, onDeny: noop });
    const inHost = await mount(
      createElement(
        DialogHostContext.Provider,
        { value: appRoot as unknown as HTMLElement },
        createElement("div", { className: "xa-trang" }, dialog),
      ),
    );
    const found = all(appRoot).find((e) => e.getAttribute("role") === "alertdialog")!;
    expect(found.textContent).toContain(SCENE_PHOTOS.camera_question);
    expect(inHost.host.contains(found)).toBe(false);
    await inHost.unmount();
    expect(all(appRoot)).toEqual([]);

    // No host (a screen rendered alone): `document.body`, still outside the screen.
    const alone = await mount(createElement("div", { className: "xa-trang" }, dialog));
    const inBody = all(doc().body).find((e) => e.getAttribute("role") === "alertdialog")!;
    expect(alone.host.contains(inBody)).toBe(false);
    await alone.unmount();
  });

  it("alerts: GỌI calls the shell's dialer with the digits; a refused call says what to do next", async () => {
    const dial = vi.fn(async (_digits: string) => false);
    const alert: DisasterAlert = { ...ALERT, emergencyContacts: [{ role: "Trực ban", name: "", phone: "0900.000 000" }] };
    const { host, unmount } = await mount(createElement(DisasterAlertCard, { alert, communeName: "Xã Thử", dial }));
    const call = all(host).find((e) => e.tagName === "BUTTON" && e.getAttribute("class") === "xa-call-button")!;
    expect(host.textContent).not.toContain(XA_GIAO_DIEN.call_failed);
    await press(call);
    expect(dial).toHaveBeenCalledWith("0900000000");
    expect(host.textContent).toContain(XA_GIAO_DIEN.call_failed);
    await unmount();
  });
});
