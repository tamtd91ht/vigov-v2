import { act, createElement, type ReactElement, StrictMode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * "CHƯA CÓ PHIÊN ViGov THÌ MÀN KHÔNG GỌI MẠNG" — kiểm khi màn ĐƯỢC GẮN THẬT và hiệu ứng CHẠY THẬT.
 *
 * VÌ SAO CẦN TỆP NÀY: `cong-dan.test.tsx` dựng màn bằng `renderToStaticMarkup`, và đường ấy KHÔNG BAO
 * GIỜ chạy `useEffect`. Nhánh chặn trong hiệu ứng (`PhanAnhCuaToiScreen.tsx` `phien === null` trong
 * `useEffect`, `TraCuuPhieuScreen.tsx` cùng chỗ) vì thế chưa từng được một ca nào chạm tới: xoá vế
 * `phien === null` đi thì mọi ca cũ vẫn xanh.
 *
 * VÌ SAO MỘT DOM TỰ DỰNG TRONG TỆP NÀY, KHÔNG PHẢI jsdom: kho không có jsdom / happy-dom /
 * @testing-library, và thêm một phụ thuộc là việc phải hỏi. React 19 `react-dom/client` chỉ cần một
 * nhúm phương thức nút (`createElement`, `appendChild`, `setAttribute`, …) để gắn một cây KHÔNG có ô
 * nhập nào — đúng cây của "kênh chưa mở". Giới hạn: shim này không mô phỏng sự kiện; ca nào cần bấm
 * thì cần một DOM thật. Mọi `console.error` của React bị bắt và làm ca đỏ, nên shim thiếu gì thì ca
 * nói ra chứ không xanh sai.
 *
 * VÌ SAO HAI GIÁN ĐIỆP, KHÔNG PHẢI MỘT: `api/goi-vigov.ts` TỰ chặn trước `fetch` khi không có phiên.
 * Nên "`fetch` không được gọi" đúng cả khi màn bỏ mất nhánh chặn của nó — lớp dưới đỡ hộ. Gián điệp
 * trên hàm gọi API (`phanAnhCuaToi`, `traCuuPhieu`, `guiPhanAnh`) mới bắt được chính nhánh của MÀN;
 * gián điệp `fetch` giữ lời hứa cả tầng.
 *
 * NGUỒN PHIÊN LÀ NGUỒN THẬT trong mọi ca "không phiên": `vi.mock` bên dưới CHUYỂN THẲNG tới
 * `layPhienViGov` thật trừ khi một ca đối chứng đặt `trang.phien`. Mã sản phẩm không có khe nào để
 * đưa phiên vào; việc thay thế chỉ sống trong tệp test này (cùng cách `api/goi-vigov.test.tsx` làm).
 */

const trang = vi.hoisted(() => ({
  /** `undefined` = dùng nguồn phiên THẬT. Chỉ ca đối chứng đặt giá trị. */
  phien: undefined as { token: string; ten_xa: string; phone_verified?: boolean } | undefined,
}));

vi.mock("../api/phien-vigov", async (importOriginal) => {
  const goc = await importOriginal<typeof import("../api/phien-vigov")>();
  // `...goc`: the commune app's gate STORES a session through the real `datPhienViGov` (09/10/2026 cases).
  return { ...goc, layPhienViGov: () => (trang.phien === undefined ? goc.layPhienViGov() : trang.phien) };
});

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const goc = await importOriginal<typeof import("../api/goi-vigov")>();
  return {
    ...goc,
    guiPhanAnh: vi.fn(goc.guiPhanAnh),
    traCuuPhieu: vi.fn(goc.traCuuPhieu),
    phanAnhCuaToi: vi.fn(goc.phanAnhCuaToi),
    citizenReportFields: vi.fn(goc.citizenReportFields),
    traXaTheoTenMien: vi.fn(goc.traXaTheoTenMien),
  };
});

import { citizenReportFields, guiPhanAnh, phanAnhCuaToi, traCuuPhieu, traXaTheoTenMien } from "../api/goi-vigov";
import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";
import type { OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { GuiPhanAnhScreen, ID_DAU_BUOC } from "./GuiPhanAnhScreen";
import {
  ACCOUNTLESS,
  COMMUNE_APP_SESSION,
  CUA_TOI,
  GUI,
  KENH_CHUA_MO,
  LOI_GUI,
  PHONE_VERIFICATION,
  QUAY_LAI,
  SCENE_PHOTOS,
  TYPED_CONTACT,
  XA_GIAO_DIEN,
  XA_PA,
  XA_TN,
} from "./noi-dung";
import { CommuneSendScreen, SERVER_ATTACHES_SESSION_PHONE } from "./PhanAnhAppXa";
import { PhanAnhCuaToiScreen } from "./PhanAnhCuaToiScreen";
// vi-name-ok: importing the EXISTING type `KetQuaLayTen` — no new name
import type { KetQuaLayTen, NameRequestMode } from "./trai-nghiem";
import { TraCuuPhieuScreen } from "./TraCuuPhieuScreen";
import { TrangXa } from "./TrangXa";

// ---------------------------------------------------------------------------------------------
// DOM tối thiểu — đủ cho `react-dom/client` gắn, cập nhật và gỡ một cây tĩnh.
// ---------------------------------------------------------------------------------------------

const HTML_NS = "http://www.w3.org/1999/xhtml";

class NutGia {
  childNodes: NutGia[] = [];
  parentNode: NutGia | null = null;
  constructor(
    readonly nodeType: number,
    readonly nodeName: string,
    public ownerDocument: TaiLieuGia | null,
  ) {}
  get firstChild(): NutGia | null {
    return this.childNodes[0] ?? null;
  }
  get lastChild(): NutGia | null {
    return this.childNodes[this.childNodes.length - 1] ?? null;
  }
  get parentElement(): NutGia | null {
    return this.parentNode;
  }
  appendChild(con: NutGia) {
    con.parentNode?.removeChild(con);
    this.childNodes.push(con);
    con.parentNode = this;
    return con;
  }
  insertBefore(con: NutGia, truoc: NutGia | null) {
    if (truoc === null) return this.appendChild(con);
    con.parentNode?.removeChild(con);
    const i = this.childNodes.indexOf(truoc);
    if (i < 0) throw new Error("insertBefore: nút tham chiếu không phải con");
    this.childNodes.splice(i, 0, con);
    con.parentNode = this;
    return con;
  }
  removeChild(con: NutGia) {
    const i = this.childNodes.indexOf(con);
    if (i < 0) throw new Error("removeChild: không phải con");
    this.childNodes.splice(i, 1);
    con.parentNode = null;
    return con;
  }
  contains(nut: NutGia | null): boolean {
    for (let n = nut; n !== null; n = n.parentNode) if (n === this) return true;
    return false;
  }
  get textContent(): string {
    return this.childNodes.map((c) => c.textContent).join("");
  }
  set textContent(chu: string) {
    for (const c of this.childNodes) c.parentNode = null;
    this.childNodes = [];
    if (chu !== "" && this.ownerDocument !== null) this.appendChild(this.ownerDocument.createTextNode(chu));
  }
  /**
   * Listeners are KEPT (since 01/10/2026) so `press` / `typeInto` below can deliver an event: React 19 listens
   * once, on the root container, and dispatches from there. Before that the shim could not press a button, and
   * the shared app's field step could only be tested as static markup.
   */
  readonly listeners = new Map<string, Array<{ fn: (e: unknown) => void; capture: boolean }>>();
  addEventListener(type: string, fn: (e: unknown) => void, opt?: boolean | { capture?: boolean }) {
    const capture = typeof opt === "boolean" ? opt : opt?.capture === true;
    const list = this.listeners.get(type) ?? [];
    list.push({ fn, capture });
    this.listeners.set(type, list);
  }
  removeEventListener(type: string, fn: (e: unknown) => void, opt?: boolean | { capture?: boolean }) {
    const capture = typeof opt === "boolean" ? opt : opt?.capture === true;
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((l) => l.fn !== fn || l.capture !== capture));
  }
}

class ChuGia extends NutGia {
  constructor(
    public nodeValue: string,
    doc: TaiLieuGia,
  ) {
    super(3, "#text", doc);
  }
  get data() {
    return this.nodeValue;
  }
  override get textContent() {
    return this.nodeValue;
  }
  override set textContent(chu: string) {
    this.nodeValue = chu;
  }
}

class PhanTuGia extends NutGia {
  readonly namespaceURI = HTML_NS;
  readonly thuoc_tinh = new Map<string, string>();
  readonly style: Record<string, string> = {};
  onclick: unknown = null;
  constructor(
    readonly tagName: string,
    doc: TaiLieuGia,
  ) {
    super(1, tagName, doc);
  }
  setAttribute(ten: string, gia_tri: string) {
    this.thuoc_tinh.set(ten, String(gia_tri));
  }
  getAttribute(ten: string) {
    return this.thuoc_tinh.get(ten) ?? null;
  }
  hasAttribute(ten: string) {
    return this.thuoc_tinh.has(ten);
  }
  removeAttribute(ten: string) {
    this.thuoc_tinh.delete(ten);
  }
  /** The screen moves focus to the new step's heading (`ID_DAU_BUOC`); the shim records where it went. */
  focus() {
    if (this.ownerDocument !== null) this.ownerDocument.activeElement = this;
  }
}

class TaiLieuGia extends NutGia {
  readonly documentElement: PhanTuGia;
  readonly body: PhanTuGia;
  activeElement: PhanTuGia | null = null;
  defaultView: unknown = null;
  /**
   * Present so React (`isEventSupported("input")`, read when `react-dom` loads) takes the `input` event path
   * for typing, not the old-IE polyfill — that one calls `attachEvent`, which no shim needs to fake.
   */
  oninput: null = null;
  constructor() {
    super(9, "#document", null);
    this.documentElement = this.createElement("html");
    this.body = this.createElement("body");
    this.appendChild(this.documentElement);
    this.documentElement.appendChild(this.body);
  }
  createElement(ten: string) {
    return new PhanTuGia(ten.toUpperCase(), this);
  }
  /** The commune app's icons are inline SVG (`BieuTuong.tsx`); the shim keeps them as plain nodes. */
  createElementNS(_ns: string, ten: string) {
    return new PhanTuGia(ten, this);
  }
  createTextNode(chu: string) {
    return new ChuGia(chu, this);
  }
  getElementById(id: string): PhanTuGia | null {
    return tatCaPhanTu(this).find((p) => p.getAttribute("id") === id) ?? null;
  }
}

function tatCaPhanTu(nut: NutGia): PhanTuGia[] {
  return nut.childNodes.flatMap((c) => (c instanceof PhanTuGia ? [c, ...tatCaPhanTu(c)] : []));
}

/** Deliver one bubbling event to `target`: capture listeners root → target, then bubble target → root. */
function dispatch(target: PhanTuGia, type: string) {
  const path: NutGia[] = [];
  for (let n: NutGia | null = target; n !== null; n = n.parentNode) path.push(n);
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
  };
  for (const n of [...path].reverse()) {
    for (const l of n.listeners.get(type) ?? []) if (l.capture && !stopped) l.fn(event);
  }
  for (const n of path) {
    for (const l of n.listeners.get(type) ?? []) if (!l.capture && !stopped) l.fn(event);
  }
}

/** Press a button, then let every promise the press started settle (a mocked call answers on the next tick). */
async function press(button: PhanTuGia) {
  await act(async () => {
    dispatch(button, "click");
  });
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

/** Type into a text box: set its value as the browser would, then the `input` event React listens for. */
async function typeInto(box: PhanTuGia, text: string) {
  (box as unknown as { value: string }).value = text;
  await act(async () => {
    dispatch(box, "input");
  });
}

/**
 * What the text box `id` holds. A box React CREATES (a step drawn again) gets its text as `value`, `defaultValue`
 * or child text depending on the element; a box typed into holds `value`. Any of them is what the citizen sees.
 */
function boxText(id: string): string {
  const doc = (globalThis as unknown as { document: TaiLieuGia }).document;
  const box = doc.getElementById(id) as unknown as { value?: string; defaultValue?: string; textContent: string };
  return box.value ?? box.defaultValue ?? box.textContent;
}

/** The buttons under `root` whose text is exactly `text`. */
function buttonsNamed(root: PhanTuGia, text: string): PhanTuGia[] {
  return tatCaPhanTu(root).filter((p) => p.tagName === "BUTTON" && p.textContent === text);
}

// `react-dom/client` quyết định "có DOM không" LÚC NẠP MÔ-ĐUN — nên dựng `window`/`document` trước,
// rồi mới `import()` nó.
let taoGoc: typeof import("react-dom/client").createRoot;
const cu = { window: (globalThis as Record<string, unknown>).window, document: (globalThis as Record<string, unknown>).document };

beforeAll(async () => {
  const doc = new TaiLieuGia();
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
  taoGoc = (await import("react-dom/client")).createRoot;
});

afterAll(() => {
  Object.assign(globalThis, { window: cu.window, document: cu.document, IS_REACT_ACT_ENVIRONMENT: undefined });
});

let fetch_gia: ReturnType<typeof vi.fn>;
let loi_react: unknown[][];

beforeEach(() => {
  trang.phien = undefined;
  vi.mocked(guiPhanAnh).mockClear();
  vi.mocked(traCuuPhieu).mockClear();
  vi.mocked(phanAnhCuaToi).mockClear();
  vi.mocked(citizenReportFields).mockClear();
  fetch_gia = vi.fn();
  vi.stubGlobal("fetch", fetch_gia);
  loi_react = [];
  vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void loi_react.push(a));
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.mocked(console.error).mockRestore();
  // Shim thiếu gì, React sẽ nói qua `console.error` — và ca không được xanh trên một cây gắn hỏng.
  expect(loi_react).toEqual([]);
});

/**
 * Gắn thật trong StrictMode (hiệu ứng chạy HAI lần ở chế độ dev), chờ mọi hiệu ứng và mọi promise
 * chúng khởi động, rồi trả cây và hàm gỡ.
 */
async function gan(man: ReactElement) {
  const doc = (globalThis as unknown as { document: TaiLieuGia }).document;
  const khung = doc.createElement("div");
  doc.body.appendChild(khung);
  const goc = taoGoc(khung as unknown as Element);
  await act(async () => {
    goc.render(createElement(StrictMode, null, man));
  });
  // Một lượt nữa: `tai("")` / `tra()` là async — kết quả của chúng rơi vào lượt sau.
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  return {
    khung,
    go: () => act(() => goc.unmount()),
  };
}

function khangDinhKenhChuaMo(khung: PhanTuGia) {
  const chu = khung.textContent;
  expect(chu).toContain(KENH_CHUA_MO.tieu_de);
  expect(chu).not.toContain(CUA_TOI.dang_tai);
  expect(chu).not.toContain(CUA_TOI.trong);
  const the = tatCaPhanTu(khung).map((p) => p.tagName);
  expect(the).not.toContain("INPUT");
  expect(the).not.toContain("TEXTAREA");
  expect(the).not.toContain("FORM");
  // Nút DUY NHẤT là "Quay lại": không có nút gửi / tra / xem thêm nào để một cú bấm đi ra mạng.
  const nut = tatCaPhanTu(khung).filter((p) => p.tagName === "BUTTON");
  expect(nut.map((n) => n.textContent)).toEqual([QUAY_LAI]);
}

describe("nguồn phiên THẬT (null) — màn gắn thật, hiệu ứng chạy, không một lời gọi nào đi ra", () => {
  it("tiền đề: nguồn phiên của tệp này đúng là nguồn thật và trả `null`", () => {
    expect(layPhienViGov()).toBeNull();
  });

  it("Phản ánh của tôi: hiệu ứng tải trang đầu KHÔNG gọi `phanAnhCuaToi`, không `fetch`", async () => {
    const { khung, go } = await gan(
      createElement(PhanAnhCuaToiScreen, { onQuayLai: () => {}, onMoPhieu: () => {}, onGuiPhanAnh: () => {} }),
    );
    khangDinhKenhChuaMo(khung);
    expect(phanAnhCuaToi).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
    await go();
  });

  it("Tra cứu có mã điền sẵn: hiệu ứng tra ngay KHÔNG gọi `traCuuPhieu`, không `fetch`", async () => {
    const { khung, go } = await gan(
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD" }),
    );
    khangDinhKenhChuaMo(khung);
    expect(traCuuPhieu).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
    await go();
  });

  it("Tra cứu không mã: không ô nhập, không nút tra, không gọi gì", async () => {
    const { khung, go } = await gan(createElement(TraCuuPhieuScreen, { onQuayLai: () => {} }));
    khangDinhKenhChuaMo(khung);
    expect(traCuuPhieu).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
    await go();
  });

  it("Gửi phản ánh: không nút gửi, không gọi `guiPhanAnh`, không tải danh mục lĩnh vực, không `fetch`", async () => {
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    khangDinhKenhChuaMo(khung);
    expect(guiPhanAnh).not.toHaveBeenCalled();
    // The catalogue hook runs on every mount (hooks before the early return); its effect must stop itself.
    expect(citizenReportFields).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
    await go();
  });
});

/**
 * ĐỐI CHỨNG — chứng minh ba ca trên KHÔNG xanh vì hiệu ứng không bao giờ chạy trong khung gắn này.
 * Có phiên (giả, chỉ trong tệp này) thì cùng khung gắn ấy PHẢI thấy lời gọi. Nếu ca này đỏ, "không
 * gọi" ở trên chẳng chứng minh gì.
 */
describe("đối chứng: có phiên thì chính khung gắn này thấy hiệu ứng gọi đi — đúng một lần", () => {
  it("Phản ánh của tôi gọi `phanAnhCuaToi('')` một lần dù StrictMode chạy hiệu ứng hai lần", async () => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    vi.mocked(phanAnhCuaToi).mockResolvedValueOnce({ kieu: "chua-co-phien" });
    const { khung, go } = await gan(
      createElement(PhanAnhCuaToiScreen, { onQuayLai: () => {}, onMoPhieu: () => {}, onGuiPhanAnh: () => {} }),
    );
    expect(vi.mocked(phanAnhCuaToi).mock.calls).toEqual([[""]]);
    // Máy chủ nói "chưa có phiên" → màn đóng kênh, không kẹt ở "đang tải".
    expect(khung.textContent).toContain(KENH_CHUA_MO.tieu_de);
    await go();
  });

  /**
   * 403 `chua_xac_thuc_so` NGAY LÚC GẮN (tải trang đầu / tra mã điền sẵn): màn HỎI, và chỉ hỏi — hàm mở
   * lại phiên kèm số (thứ bật hộp thoại xin số của Zalo) KHÔNG được gọi khi công dân chưa bấm gì.
   */
  it("403 `chua_xac_thuc_so` lúc gắn: khung hỏi số hiện ra, KHÔNG tự xin số, không gọi lại", async () => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    const reopen = vi.fn();
    vi.mocked(phanAnhCuaToi).mockResolvedValueOnce({ kieu: "can-xac-thuc-so" });
    const mine = await gan(
      createElement(PhanAnhCuaToiScreen, {
        onQuayLai: () => {},
        onMoPhieu: () => {},
        onGuiPhanAnh: () => {},
        reopenWithPhone: reopen,
      }),
    );
    expect(mine.khung.textContent).toContain(PHONE_VERIFICATION.title);
    expect(mine.khung.textContent).toContain(PHONE_VERIFICATION.allow);
    // Không kẹt ở "đang tải", không nói "chưa gửi phản ánh nào" — chưa biết gì về danh sách.
    expect(mine.khung.textContent).not.toContain(CUA_TOI.dang_tai);
    expect(mine.khung.textContent).not.toContain(CUA_TOI.trong);
    expect(vi.mocked(phanAnhCuaToi)).toHaveBeenCalledTimes(1);
    await mine.go();

    vi.mocked(traCuuPhieu).mockResolvedValueOnce({ kieu: "can-xac-thuc-so" });
    const lookup = await gan(
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD", reopenWithPhone: reopen }),
    );
    expect(lookup.khung.textContent).toContain(PHONE_VERIFICATION.title);
    expect(vi.mocked(traCuuPhieu)).toHaveBeenCalledTimes(1);
    await lookup.go();

    expect(reopen).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
  });

  it("Tra cứu có mã điền sẵn gọi `traCuuPhieu(mã)` một lần", async () => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    vi.mocked(traCuuPhieu).mockResolvedValueOnce({ kieu: "khong-thay" });
    const { go } = await gan(createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD" }));
    expect(vi.mocked(traCuuPhieu).mock.calls).toEqual([["PA7K2QX9M4TD"]]);
    await go();
  });

  it("Gửi phản ánh gọi `citizenReportFields()` một lần dù StrictMode chạy hiệu ứng hai lần, và vẽ danh mục", async () => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    vi.mocked(citizenReportFields).mockResolvedValueOnce({ kieu: "xong", fields: FIELDS });
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    expect(vi.mocked(citizenReportFields).mock.calls).toEqual([[]]);
    expect(khung.textContent).toContain(GUI.field_title);
    expect(khung.textContent).toContain("Rác thải – Vệ sinh môi trường");
    expect(guiPhanAnh).not.toHaveBeenCalled();
    expect(fetch_gia).not.toHaveBeenCalled();
    await go();
  });
});

/* ─────────────── the shared app's field step, mounted and pressed (owner, 01/10/2026) ─────────────── */

const FIELDS = [
  { code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", icon: "Trash2", tone: "orange" },
  { code: "an-ninh", label: "An ninh trật tự", icon: null, tone: null },
] as const;

const SENT: PhieuCuaToi = {
  ma_tra_cuu: "PA7K2QX9M4TD",
  trang_thai: "da-tiep-nhan",
  linh_vuc: "an-ninh",
  nhan_linh_vuc: "An ninh trật tự",
  noi_dung: "Tụ tập gây ồn sau 23 giờ",
  dia_chi: "",
  ho_ten_da_che: "",
  dien_thoai_da_che: "",
  an_danh: true,
  goc_dem_han: "2026-10-01T01:30:00Z",
  han_tiep_nhan: null,
  han_xu_ly_xong: null,
  ket_qua: "",
  ly_do: "",
  co_quan_nhan: "",
  rating: null,
  rated_at: null,
};

describe("app chung — bước lĩnh vực: bắt buộc, mã chọn được đi lên `field`, không danh sách dự phòng", () => {
  beforeEach(() => {
    // A VERIFIED session: these cases are about the field step, not the phone (ADR 0080 asks first otherwise).
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm", phone_verified: true };
    vi.mocked(guiPhanAnh).mockReset();
    vi.mocked(citizenReportFields).mockReset();
  });

  const radios = (root: PhanTuGia) => tatCaPhanTu(root).filter((p) => p.getAttribute("role") === "radio");
  const nextButton = (root: PhanTuGia) => buttonsNamed(root, GUI.nut_tiep)[0]!;
  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;

  /** Mount, pick `label`, go on, write the content, go on — the screen is then on the confirmation step. */
  async function upToConfirm(label: string) {
    const mounted = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    await press(radios(mounted.khung).find((r) => r.textContent.startsWith(label))!);
    await press(nextButton(mounted.khung));
    await typeInto(doc().getElementById("cd-noi-dung")!, "Tụ tập gây ồn sau 23 giờ");
    await press(nextButton(mounted.khung));
    return mounted;
  }

  it("'Tiếp tục' is disabled until a field is picked; picking says 'Đã chọn' in words and enables it", async () => {
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    expect(radios(khung).map((r) => r.getAttribute("aria-checked"))).toEqual(["false", "false"]);
    expect(nextButton(khung).hasAttribute("disabled")).toBe(true);
    expect(khung.textContent).toContain(GUI.field_pick_first);
    // Pressing the disabled button does nothing: still step 1, no textarea.
    await press(nextButton(khung));
    expect(doc().getElementById("cd-noi-dung")).toBeNull();

    await press(radios(khung)[1]!);
    expect(radios(khung).map((r) => r.getAttribute("aria-checked"))).toEqual(["false", "true"]);
    expect(radios(khung)[1]!.textContent).toContain(GUI.field_picked);
    expect(nextButton(khung).hasAttribute("disabled")).toBe(false);
    expect(khung.textContent).not.toContain(GUI.field_pick_first);
    await go();
  });

  it("step 2 shows 'Lĩnh vực: <tên> · Đổi'; 'Đổi' goes back to step 1 with the pick kept, focus on its heading", async () => {
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    await press(radios(khung)[0]!);
    await press(nextButton(khung));
    expect(doc().activeElement?.getAttribute("id")).toBe(ID_DAU_BUOC.nhap);
    expect(khung.textContent).toContain(`${GUI.field_label}: ${FIELDS[0].label}`);
    const change = buttonsNamed(khung, GUI.field_change)[0]!;
    expect(change.getAttribute("aria-label")).toBe(GUI.field_change_name);
    await press(change);
    expect(doc().activeElement?.getAttribute("id")).toBe(ID_DAU_BUOC.field);
    expect(radios(khung)[0]!.getAttribute("aria-checked")).toBe("true");
    // Only one catalogue call for all of it: going back does not reload.
    expect(citizenReportFields).toHaveBeenCalledTimes(1);
    await go();
  });

  it("the confirmation step reads the field's name with the commune; the picked CODE reaches the POST body", async () => {
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
    vi.mocked(guiPhanAnh).mockResolvedValue({ kieu: "xong", phieu: SENT });
    const { khung, go } = await upToConfirm("An ninh trật tự");
    expect(khung.textContent).toContain(GUI.xac_nhan_tieu_de);
    expect(khung.textContent).toContain(`${GUI.field_label}: An ninh trật tự`);
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);
    expect(guiPhanAnh).toHaveBeenCalledTimes(1);
    const body = JSON.parse(vi.mocked(guiPhanAnh).mock.calls[0]![0].than) as Record<string, unknown>;
    expect(body["field"]).toBe("an-ninh");
    expect(body["content"]).toBe("Tụ tập gây ồn sau 23 giờ");
    expect(khung.textContent).toContain(SENT.ma_tra_cuu);
    await go();
  });

  it("400 field_not_offered: back to step 1 with the reason, the catalogue reloaded, nothing picked, words kept", async () => {
    vi.mocked(citizenReportFields)
      .mockResolvedValueOnce({ kieu: "xong", fields: FIELDS })
      .mockResolvedValueOnce({ kieu: "xong", fields: [FIELDS[0]] });
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "field-not-offered" });
    const { khung, go } = await upToConfirm("An ninh trật tự");
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);

    expect(doc().activeElement?.getAttribute("id")).toBe(ID_DAU_BUOC.field);
    expect(khung.textContent).toContain(LOI_GUI["field-not-offered"].cau);
    expect(citizenReportFields).toHaveBeenCalledTimes(2);
    expect(radios(khung).map((r) => r.textContent)).toEqual([FIELDS[0].label]);
    expect(nextButton(khung).hasAttribute("disabled")).toBe(true);
    // Not the dead-end error step of before: no "Gửi lại", and the next send is a NEW attempt with the new pick.
    expect(buttonsNamed(khung, GUI.nut_gui_lai)).toEqual([]);
    await press(radios(khung)[0]!);
    expect(khung.textContent).not.toContain(LOI_GUI["field-not-offered"].cau);
    await press(nextButton(khung));
    expect(doc().getElementById("cd-noi-dung")).not.toBeNull();

    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    // Straight on: the words were kept, so step 2 passes without typing them again.
    await press(nextButton(khung));
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);
    const [first, second] = vi.mocked(guiPhanAnh).mock.calls.map((c) => c[0]);
    const body = JSON.parse(second!.than) as Record<string, unknown>;
    expect(body["field"]).toBe("rac-thai");
    expect(body["content"]).toBe("Tụ tập gây ồn sau 23 giờ");
    expect(second!.khoa).not.toBe(first!.khoa);
    await go();
  });

  it("503 field_catalogue_unavailable: the sentence and 'Thử lại', no tile, no 'Tiếp tục'; 'Thử lại' loads again", async () => {
    vi.mocked(citizenReportFields)
      .mockResolvedValueOnce({ kieu: "field-catalogue-unavailable" })
      .mockResolvedValueOnce({ kieu: "xong", fields: FIELDS });
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    expect(khung.textContent).toContain(GUI.field_unavailable);
    expect(radios(khung)).toEqual([]);
    expect(buttonsNamed(khung, GUI.nut_tiep)).toEqual([]);
    await press(buttonsNamed(khung, CUA_TOI.nut_thu_lai)[0]!);
    expect(citizenReportFields).toHaveBeenCalledTimes(2);
    expect(radios(khung)).toHaveLength(2);
    await go();
  });

  it("no session on the server's side (chua-co-phien) → the channel is closed, as on every other branch", async () => {
    vi.mocked(citizenReportFields).mockResolvedValueOnce({ kieu: "chua-co-phien" });
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    khangDinhKenhChuaMo(khung);
    await go();
  });
});

/* ─────────────── ADR 0080 (08/10/2026): Zalo gives no number — ask first, then typed contact ─────────────── */

describe("app chung — phiên chưa xác thực số: mời chia sẻ TRƯỚC khi gửi, rồi đường họ tên + số tự nhập", () => {
  beforeEach(() => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm", phone_verified: false };
    vi.mocked(guiPhanAnh).mockReset();
    vi.mocked(citizenReportFields).mockReset();
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
  });

  const radios = (root: PhanTuGia) => tatCaPhanTu(root).filter((p) => p.getAttribute("role") === "radio");
  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;
  const next = (root: PhanTuGia) => buttonsNamed(root, GUI.nut_tiep)[0]!;

  it("send → the phone panel BEFORE any call; declined → typed path → name + number required → body carries them", async () => {
    const reopen = vi.fn();
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {}, reopenWithPhone: reopen }));
    await press(radios(khung).find((r) => r.textContent.startsWith("An ninh trật tự"))!);
    await press(next(khung));
    await typeInto(doc().getElementById("cd-noi-dung")!, "Tụ tập gây ồn sau 23 giờ");
    await press(next(khung));
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);

    // Decision 1: the invitation to share the Zalo number comes first — nothing was sent, Zalo was not asked.
    expect(guiPhanAnh).not.toHaveBeenCalled();
    expect(reopen).not.toHaveBeenCalled();
    expect(khung.textContent).toContain(PHONE_VERIFICATION.title);
    expect(buttonsNamed(khung, PHONE_VERIFICATION.allow)).toHaveLength(1);
    expect(buttonsNamed(khung, TYPED_CONTACT.button)).toEqual([]);

    await press(buttonsNamed(khung, PHONE_VERIFICATION.decline)[0]!);
    expect(khung.textContent).toContain(TYPED_CONTACT.offer);
    await press(buttonsNamed(khung, TYPED_CONTACT.button)[0]!);

    // Back on the writing step, words kept, the typed-contact rules on.
    expect(khung.textContent).toContain(TYPED_CONTACT.form_note);
    expect(khung.textContent).toContain(TYPED_CONTACT.name_label);
    expect(khung.textContent).toContain(TYPED_CONTACT.anonymous_off);
    const toggle = tatCaPhanTu(khung).find((p) => p.getAttribute("class") === "cd-cong-tac")!;
    expect(toggle.hasAttribute("disabled")).toBe(true);
    expect(boxText("cd-noi-dung")).toBe("Tụ tập gây ồn sau 23 giờ");

    await press(next(khung));
    expect(khung.textContent).toContain(TYPED_CONTACT.name_missing);
    await typeInto(doc().getElementById("cd-ho-ten")!, "Nguyễn Văn An");
    await typeInto(doc().getElementById("cd-dien-thoai")!, "12345");
    await press(next(khung));
    expect(khung.textContent).toContain(TYPED_CONTACT.phone_invalid);
    await typeInto(doc().getElementById("cd-dien-thoai")!, "0900000000");
    await press(next(khung));

    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: { ...SENT, contact_unverified: true } });
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);
    expect(guiPhanAnh).toHaveBeenCalledTimes(1);
    const body = JSON.parse(vi.mocked(guiPhanAnh).mock.calls[0]![0].than) as Record<string, unknown>;
    expect(body).toMatchObject({
      reporter_name: "Nguyễn Văn An",
      reporter_phone: "0900000000",
      anonymous: false,
      field: "an-ninh",
    });
    // The code screen says no notification will come.
    expect(khung.textContent).toContain(SENT.ma_tra_cuu);
    expect(khung.textContent).toContain(TYPED_CONTACT.done_no_notice);
    expect(reopen).not.toHaveBeenCalled();
    await go();
  });

  it("429 `unverified_daily_limit` → the server's sentence, no 'Gửi lại', the words still there via 'Sửa lại'", async () => {
    const reopen = vi.fn();
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {}, reopenWithPhone: reopen }));
    await press(radios(khung)[0]!);
    await press(next(khung));
    await typeInto(doc().getElementById("cd-noi-dung")!, "Rác tồn đọng");
    await press(next(khung));
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);
    await press(buttonsNamed(khung, PHONE_VERIFICATION.decline)[0]!);
    await press(buttonsNamed(khung, TYPED_CONTACT.button)[0]!);
    await typeInto(doc().getElementById("cd-ho-ten")!, "Nguyễn Văn An");
    await typeInto(doc().getElementById("cd-dien-thoai")!, "0900000000");
    await press(next(khung));
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "unverified-daily-limit", message: "Câu của máy chủ." });
    await press(buttonsNamed(khung, GUI.nut_gui("Xã Thử Nghiệm"))[0]!);
    expect(khung.textContent).toContain("Câu của máy chủ.");
    expect(buttonsNamed(khung, GUI.nut_gui_lai)).toEqual([]);
    await press(buttonsNamed(khung, GUI.nut_sua)[0]!);
    expect(boxText("cd-noi-dung")).toBe("Rác tồn đọng");
    await go();
  });
});

describe("app xã — phiên chưa xác thực số: họ tên + số bắt buộc, ẩn danh tắt, 429 giữ nháp", () => {
  beforeEach(() => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm", phone_verified: false };
    vi.mocked(guiPhanAnh).mockReset();
    vi.mocked(citizenReportFields).mockReset();
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
  });

  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;
  const mount = () =>
    gan(
      createElement(CommuneSendScreen, {
        ten_xa: "Xã Thử Nghiệm",
        ho_ten: null,
        typedContact: true,
        onBack: () => {},
        onSessionLost: () => {},
        onSent: () => {},
        onOpenPetition: () => {},
      }),
    );

  it("required name + number with a plain sentence; the switch is off and disabled; the body carries the typed contact", async () => {
    const { khung, go } = await mount();
    await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    expect(khung.textContent).toContain(XA_PA.contact_note);
    expect(khung.textContent).toContain(XA_PA.anonymous_off);
    expect(khung.textContent).toContain(XA_PA.contact_required);
    const toggle = tatCaPhanTu(khung).find((p) => p.getAttribute("role") === "switch")!;
    expect(toggle.hasAttribute("disabled")).toBe(true);
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    expect(doc().getElementById("xa-dien-thoai")!.getAttribute("aria-required")).toBe("true");

    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng đầu ngõ 12");
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(guiPhanAnh).not.toHaveBeenCalled();
    expect(khung.textContent).toContain(XA_PA.contact_name_missing);
    expect(khung.textContent).toContain(XA_PA.contact_phone_missing);

    await typeInto(doc().getElementById("xa-ho-ten")!, "Nguyễn Văn An");
    await typeInto(doc().getElementById("xa-dien-thoai")!, "0900000000");
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: { ...SENT, contact_unverified: true } });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(guiPhanAnh).toHaveBeenCalledTimes(1);
    const body = JSON.parse(vi.mocked(guiPhanAnh).mock.calls[0]![0].than) as Record<string, unknown>;
    expect(body).toMatchObject({ reporter_name: "Nguyễn Văn An", reporter_phone: "0900000000", anonymous: false });
    expect(khung.textContent).toContain(XA_PA.done_no_notice);
    await go();
  });

  it("429 `unverified_daily_limit` → the server's sentence in the footer, words kept, the next send a NEW attempt", async () => {
    const { khung, go } = await mount();
    await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng");
    await typeInto(doc().getElementById("xa-ho-ten")!, "Nguyễn Văn An");
    await typeInto(doc().getElementById("xa-dien-thoai")!, "0900000000");
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "unverified-daily-limit", message: "Câu của máy chủ." });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(khung.textContent).toContain("Câu của máy chủ.");
    expect(buttonsNamed(khung, GUI.nut_gui_lai)).toEqual([]);
    expect(boxText("xa-noi-dung")).toBe("Rác tồn đọng");
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    const [first, second] = vi.mocked(guiPhanAnh).mock.calls.map((c) => c[0]);
    expect(second!.khoa).not.toBe(first!.khoa);
    await go();
  });
});

/* ═════════════════ 09/10/2026 (owner, final) — the sender on a VERIFIED session: one line, or the name box alone ═════════════════ */

describe("app xã — phiên đã xác thực số: (a) một dòng tên Zalo, (c) chỉ ô họ tên; không ô số, không 'Sửa'", () => {
  const NAME = "Nguyễn Văn An";
  const TYPED = "Trần Thị Bình";
  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;

  beforeEach(() => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm", phone_verified: true };
    vi.mocked(guiPhanAnh).mockReset();
    vi.mocked(citizenReportFields).mockReset();
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
  });

  type SendProps = Partial<Parameters<typeof CommuneSendScreen>[0]>;
  const screen = (props: SendProps) =>
    createElement(CommuneSendScreen, {
      ten_xa: "Xã Thử Nghiệm",
      ho_ten: NAME,
      verifiedPhone: true,
      onBack: () => {},
      onSessionLost: () => {},
      onSent: () => {},
      onOpenPetition: () => {},
      ...props,
    });

  /** Mounted with a handle to render it again with new props — a name that arrives late is a new prop. */
  async function mountSend(props: SendProps, pickField = true) {
    const khung = doc().createElement("div");
    doc().body.appendChild(khung);
    const goc = taoGoc(khung as unknown as Element);
    const render = async (p: SendProps) => {
      await act(async () => {
        goc.render(createElement(StrictMode, null, screen(p)));
      });
      await act(async () => {
        await new Promise((r) => setTimeout(r, 0));
      });
    };
    await render(props);
    if (pickField) await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    return { khung, render, go: () => act(() => goc.unmount()) };
  }

  const sentBody = (i = 0) => JSON.parse(vi.mocked(guiPhanAnh).mock.calls[i]![0].than) as Record<string, unknown>;
  const anonymousSwitch = (khung: PhanTuGia) => tatCaPhanTu(khung).find((p) => p.getAttribute("role") === "switch")!;
  /** No button anywhere on the form that edits the sender — the old "Sửa" is gone, not hidden. */
  const noEditButton = (khung: PhanTuGia) => expect(buttonsNamed(khung, "Sửa")).toEqual([]);

  async function writeAndSend(khung: PhanTuGia) {
    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng đầu ngõ 12");
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
  }

  it("(a) verified + a Zalo name: one read-only line, no name box, no number box, no 'Sửa'", async () => {
    expect(SERVER_ATTACHES_SESSION_PHONE).toBe(true);
    const { khung, go } = await mountSend({});
    expect(khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    expect(doc().getElementById("xa-ho-ten")).toBeNull();
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    noEditButton(khung);
    await go();
  });

  it("(a) sends the Zalo name and an EMPTY number — the server attaches the verified one", async () => {
    const { khung, go } = await mountSend({ nameAtEntry: { kind: "settled", name: NAME }, ho_ten: NAME });
    await writeAndSend(khung);
    expect(guiPhanAnh).toHaveBeenCalledTimes(1);
    expect(sentBody()).toMatchObject({ reporter_name: NAME, reporter_phone: "", anonymous: false });
    await go();
  });

  it("(c) verified, the name declined: ONLY a required name box and the verified-number line; no number box", async () => {
    const { khung, go } = await mountSend({ ho_ten: null, nameAtEntry: { kind: "settled", name: null } });
    const box = doc().getElementById("xa-ho-ten")!;
    expect(box).not.toBeNull();
    expect(box.getAttribute("aria-required")).toBe("true");
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    expect(khung.textContent).toContain(XA_PA.sender_verified_phone);
    expect(khung.textContent).not.toContain(XA_PA.sender_summary_with_phone(""));
    noEditButton(khung);

    // An empty name is refused before anything leaves, with what to do next.
    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng đầu ngõ 12");
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(guiPhanAnh).not.toHaveBeenCalled();
    expect(khung.textContent).toContain(XA_PA.thieu_nguoi_gui);

    await typeInto(doc().getElementById("xa-ho-ten")!, TYPED);
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(guiPhanAnh).toHaveBeenCalledTimes(1);
    expect(sentBody()).toMatchObject({ reporter_name: TYPED, reporter_phone: "", anonymous: false });
    await go();
  });

  it("a restored draft holding a number does NOT carry it into (a) or (c); (a) sends the Zalo name, (c) the typed one", async () => {
    const DRAFT = {
      linh_vuc: FIELDS[0].code,
      noi_dung: "Rác tồn đọng đầu ngõ 12",
      dia_chi: "",
      ho_ten: TYPED,
      dien_thoai: "0900000000",
      an_danh: false,
    };
    const store = { load: () => DRAFT, save: vi.fn(), clear: vi.fn() };

    const a = await mountSend({ draftStore: store }, false);
    await press(buttonsNamed(a.khung, XA_PA.draft_resume)[0]!);
    expect(a.khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    await press(buttonsNamed(a.khung, GUI.tieu_de)[0]!);
    expect(sentBody(0)).toMatchObject({ reporter_name: NAME, reporter_phone: "" });
    await a.go();

    const c = await mountSend({ draftStore: store, ho_ten: null, nameAtEntry: { kind: "settled", name: null } }, false);
    await press(buttonsNamed(c.khung, XA_PA.draft_resume)[0]!);
    expect(boxText("xa-ho-ten")).toBe(TYPED);
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    vi.mocked(guiPhanAnh).mockResolvedValueOnce({ kieu: "xong", phieu: SENT });
    await press(buttonsNamed(c.khung, GUI.tieu_de)[0]!);
    expect(sentBody(1)).toMatchObject({ reporter_name: TYPED, reporter_phone: "" });
    // The draft saved while writing carries no number either.
    for (const [saved] of store.save.mock.calls) expect((saved as { dien_thoai: string }).dien_thoai).toBe("");
    await c.go();
  });

  it("the mode is FIXED once the name settled: a name arriving later never swaps (c) into (a)", async () => {
    const { khung, render, go } = await mountSend({ ho_ten: null, nameAtEntry: { kind: "settled", name: null } });
    await typeInto(doc().getElementById("xa-ho-ten")!, TYPED);
    await render({ ho_ten: NAME, nameAtEntry: { kind: "settled", name: NAME } });
    expect(boxText("xa-ho-ten")).toBe(TYPED);
    expect(khung.textContent).toContain(XA_PA.sender_verified_phone);
    expect(khung.textContent).not.toContain(XA_PA.sender_summary_with_phone(NAME));
    await go();
  });

  it("still being asked: no box at all and no sending until it settles — then (a) or (c), once", async () => {
    const asking = await mountSend({ ho_ten: null, nameAtEntry: { kind: "asking" } });
    expect(asking.khung.textContent).toContain(XA_PA.sender_pending);
    expect(doc().getElementById("xa-ho-ten")).toBeNull();
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng đầu ngõ 12");
    expect(buttonsNamed(asking.khung, GUI.tieu_de)[0]!.hasAttribute("disabled")).toBe(true);
    await asking.render({ ho_ten: NAME, nameAtEntry: { kind: "settled", name: NAME } });
    expect(asking.khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    expect(asking.khung.textContent).not.toContain(XA_PA.sender_pending);
    await asking.go();

    const declined = await mountSend({ ho_ten: null, nameAtEntry: { kind: "checking" } });
    await declined.render({ ho_ten: null, nameAtEntry: { kind: "settled", name: null } });
    expect(doc().getElementById("xa-ho-ten")).not.toBeNull();
    expect(declined.khung.textContent).toContain(XA_PA.sender_verified_phone);
    await declined.go();
  });

  it("'Gửi ẩn danh' still hides the sender in (a) and (c), and the body says anonymous", async () => {
    for (const props of [{}, { ho_ten: null, nameAtEntry: { kind: "settled", name: null } } as SendProps]) {
      const { khung, go } = await mountSend(props);
      await press(anonymousSwitch(khung));
      expect(doc().getElementById("xa-ho-ten")).toBeNull();
      expect(doc().getElementById("xa-dien-thoai")).toBeNull();
      expect(khung.textContent).not.toContain(XA_PA.sender_summary_with_phone(NAME));
      expect(khung.textContent).not.toContain(XA_PA.sender_verified_phone);
      await writeAndSend(khung);
      expect(sentBody(vi.mocked(guiPhanAnh).mock.calls.length - 1)).toMatchObject({
        reporter_name: "",
        reporter_phone: "",
        anonymous: true,
      });
      await go();
    }
  });

  it("not (a)/(c): no verified session (or unknown), or the server switch off — the name and number boxes, as before", async () => {
    for (const props of [{ verifiedPhone: undefined }, { verifiedPhone: false }, { sessionPhoneAttached: false }]) {
      const { khung, go } = await mountSend(props);
      expect(doc().getElementById("xa-ho-ten"), JSON.stringify(props)).not.toBeNull();
      expect(doc().getElementById("xa-dien-thoai"), JSON.stringify(props)).not.toBeNull();
      expect(khung.textContent).not.toContain(XA_PA.sender_summary_with_phone(NAME));
      noEditButton(khung);
      await go();
    }
  });

  it("(b) unchanged: the typed-contact path keeps both boxes required, whatever the Zalo name", async () => {
    const { khung, go } = await mountSend({ typedContact: true, verifiedPhone: false });
    expect(doc().getElementById("xa-ho-ten")).not.toBeNull();
    expect(doc().getElementById("xa-dien-thoai")!.getAttribute("aria-required")).toBe("true");
    expect(khung.textContent).not.toContain(XA_PA.sender_summary_with_phone(NAME));
    expect(khung.textContent).not.toContain(XA_PA.sender_verified_phone);
    await go();
  });
  it("the description box: three lines, grows with the text; the shared app's box keeps six lines", async () => {
    const { go } = await mountSend({});
    const box = doc().getElementById("xa-noi-dung")!;
    expect(box.getAttribute("class")).toContain("cd-o__nhap--grow");
    expect(box.getAttribute("rows")).toBe("3");
    // The shim has no layout: give the box the measures a browser would, then type.
    Object.defineProperties(box, {
      scrollHeight: { get: () => 168 },
      offsetHeight: { get: () => 100 },
      clientHeight: { get: () => 96 },
    });
    await typeInto(box, "Dòng một\nDòng hai\nDòng ba\nDòng bốn\nDòng năm");
    expect(box.style["height"]).toBe("172px");
    await go();

    const shared = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    await press(tatCaPhanTu(shared.khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    await press(buttonsNamed(shared.khung, GUI.nut_tiep)[0]!);
    const sharedBox = doc().getElementById("cd-noi-dung")!;
    expect(sharedBox.getAttribute("rows")).toBe("6");
    expect(sharedBox.getAttribute("class")).not.toContain("cd-o__nhap--grow");
    await shared.go();
  });
});

/* ═════════════════ 09/10/2026 (owner, final) — the Zalo name is asked in the SAME step as the number, at most once ═════════════════ */

describe("app xã — cổng gửi phản ánh: hỏi họ tên cùng bước xác nhận số, nhiều nhất một lần mỗi lần mở", () => {
  const COMMUNE = "Xã Thử Nghiệm";
  const NAME = "Nguyễn Văn An";
  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;

  beforeEach(() => {
    // The REAL session store: the gate opens the session through it, the screens read it back.
    trang.phien = undefined;
    datPhienViGov(null);
    vi.mocked(guiPhanAnh).mockReset();
    vi.mocked(citizenReportFields).mockReset();
    vi.mocked(citizenReportFields).mockResolvedValue({ kieu: "xong", fields: FIELDS });
    vi.mocked(traXaTheoTenMien).mockResolvedValue({
      kieu: "xong",
      gia_tri: [{ ten: COMMUNE, tinh: "Tỉnh Thử Nghiệm" }],
    } as Awaited<ReturnType<typeof traXaTheoTenMien>>);
    // Everything else the home screen reads (news, profile, banners) answers "nothing here".
    fetch_gia.mockImplementation(async () => ({ status: 404, ok: false, json: async () => ({}) }));
  });

  afterEach(() => {
    datPhienViGov(null);
  });

  /** The shell's name bridge: answers `check` and `ask` as told, and records every call. */
  function nameBridge(check: KetQuaLayTen, ask: KetQuaLayTen) {
    const calls: NameRequestMode[] = [];
    const bridge = vi.fn(async (mode: NameRequestMode) => {
      calls.push(mode);
      return mode === "check" ? check : ask;
    });
    return { bridge, calls };
  }

  const openSession = vi.fn<OpenCommuneAppSession>(async () => ({
    kieu: "xong",
    token: "tok-thu-nghiem",
    ten_xa: COMMUNE,
    da_xac_thuc_so: true,
  }));

  async function mountApp(bridge: (mode: NameRequestMode) => Promise<KetQuaLayTen>) {
    openSession.mockClear();
    return gan(createElement(TrangXa, { ten_mien: "thu.vigov.example", lay_ten: bridge, openSession }));
  }

  const sendTile = (khung: PhanTuGia) => buttonsNamed(khung, XA_GIAO_DIEN.o_gui)[0]!;

  /** Pick the first field: the sender block lives on the writing step. */
  async function toWritingStep(khung: PhanTuGia) {
    await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
  }

  /** From the writing step back to home: "Quay lại" twice, then "Huỷ bỏ" in the cancel question. */
  async function leaveSendScreen(khung: PhanTuGia) {
    await press(buttonsNamed(khung, QUAY_LAI)[0]!);
    await press(buttonsNamed(khung, QUAY_LAI)[0]!);
    await press(buttonsNamed(khung, XA_TN.huy_bo)[0]!);
  }

  it("name not settled: the explanation names BOTH; one tap → number, then name; (a); never asked again", async () => {
    const { bridge, calls } = nameBridge({ kieu: "tu-choi" }, { kieu: "xong", ho_ten: NAME });
    const { khung, go } = await mountApp(bridge);
    expect(calls).toEqual(["check"]);

    await press(sendTile(khung));
    // Policy 3.3.4: the purpose of BOTH dialogs is said before either opens; nothing asked yet.
    expect(khung.textContent).toContain(COMMUNE_APP_SESSION.why_with_name);
    expect(khung.textContent).toContain(COMMUNE_APP_SESSION.zalo_asks_with_name);
    expect(khung.textContent).not.toContain(COMMUNE_APP_SESSION.zalo_asks);
    expect(openSession).not.toHaveBeenCalled();
    expect(calls).toEqual(["check"]);

    await press(buttonsNamed(khung, PHONE_VERIFICATION.allow)[0]!);
    expect(openSession).toHaveBeenCalledTimes(1);
    expect(calls).toEqual(["check", "ask"]);
    await toWritingStep(khung);
    expect(khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    expect(doc().getElementById("xa-ho-ten")).toBeNull();
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();

    // A second send in the same open: the session exists, the name is settled — no gate, no dialog.
    await leaveSendScreen(khung);
    await press(sendTile(khung));
    await toWritingStep(khung);
    expect(calls).toEqual(["check", "ask"]);
    expect(openSession).toHaveBeenCalledTimes(1);
    expect(khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    await go();
  });

  it("the name declined in Zalo's dialog: (c) — the name box alone — and it is never asked again in this open", async () => {
    const { bridge, calls } = nameBridge({ kieu: "tu-choi" }, { kieu: "tu-choi" });
    const { khung, go } = await mountApp(bridge);
    await press(sendTile(khung));
    await press(buttonsNamed(khung, PHONE_VERIFICATION.allow)[0]!);
    await toWritingStep(khung);
    expect(calls).toEqual(["check", "ask"]);
    expect(doc().getElementById("xa-ho-ten")!.getAttribute("aria-required")).toBe("true");
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    expect(khung.textContent).toContain(XA_PA.sender_verified_phone);

    await leaveSendScreen(khung);
    await press(sendTile(khung));
    await toWritingStep(khung);
    expect(calls).toEqual(["check", "ask"]);
    expect(khung.textContent).toContain(XA_PA.sender_verified_phone);
    await go();
  });

  it("name already settled at open: the gate speaks of the number only and asks Zalo for nothing else; (a)", async () => {
    const { bridge, calls } = nameBridge({ kieu: "xong", ho_ten: NAME }, { kieu: "xong", ho_ten: "never asked" });
    const { khung, go } = await mountApp(bridge);
    await press(sendTile(khung));
    expect(khung.textContent).toContain(COMMUNE_APP_SESSION.zalo_asks);
    expect(khung.textContent).toContain(PHONE_VERIFICATION.why);
    expect(khung.textContent).not.toContain(COMMUNE_APP_SESSION.zalo_asks_with_name);
    await press(buttonsNamed(khung, PHONE_VERIFICATION.allow)[0]!);
    await toWritingStep(khung);
    expect(calls).toEqual(["check"]);
    expect(khung.textContent).toContain(XA_PA.sender_summary_with_phone(NAME));
    await go();
  });

  it("name declined on the home card: the gate does not ask it again; (c)", async () => {
    const { bridge, calls } = nameBridge({ kieu: "tu-choi" }, { kieu: "xong", ho_ten: "never asked" });
    const { khung, go } = await mountApp(bridge);
    await press(buttonsNamed(khung, XA_TN.name_card_decline)[0]!);
    await press(sendTile(khung));
    expect(khung.textContent).not.toContain(COMMUNE_APP_SESSION.zalo_asks_with_name);
    await press(buttonsNamed(khung, PHONE_VERIFICATION.allow)[0]!);
    await toWritingStep(khung);
    expect(calls).toEqual(["check"]);
    expect(doc().getElementById("xa-ho-ten")).not.toBeNull();
    expect(doc().getElementById("xa-dien-thoai")).toBeNull();
    await go();
  });

  it("the number refused: the name is not asked either — nothing for it to serve", async () => {
    const { bridge, calls } = nameBridge({ kieu: "tu-choi" }, { kieu: "xong", ho_ten: NAME });
    const { khung, go } = await mountApp(bridge);
    openSession.mockResolvedValueOnce({ kieu: "tu-choi" });
    await press(sendTile(khung));
    await press(buttonsNamed(khung, PHONE_VERIFICATION.allow)[0]!);
    expect(openSession).toHaveBeenCalledTimes(1);
    expect(calls).toEqual(["check"]);
    await go();
  });
});

/* ═════════════════ ADR 0083 — ACCOUNTLESS SEND, mounted for real, through the REAL client and a fake `fetch` ═════════════════ */

describe("app xã — gửi không tài khoản (ADR 0083): cùng biểu mẫu, không ảnh, không phiên, cùng khoá khi gửi lại", () => {
  const DOMAIN = "xa-thu-nghiem.vigov.example";
  const doc = () => (globalThis as unknown as { document: TaiLieuGia }).document;
  type Call = { url: string; method: string; headers: Record<string, string>; body: string | undefined };
  let calls: Call[];
  /** The POST answers, in order; the GET of the field list always answers the catalogue. */
  let postAnswers: Array<{ status: number; body: unknown }>;
  const locate = vi.fn();

  beforeEach(() => {
    // A session EXISTS on this phone: the accountless calls must still carry no bearer (ADR 0083 STOP #5).
    trang.phien = { token: "tok-must-not-leave", ten_xa: "Xã Thử Nghiệm", phone_verified: false };
    calls = [];
    postAnswers = [];
    fetch_gia.mockImplementation(async (url: string, init: { method: string; headers: Record<string, string>; body?: string }) => {
      calls.push({ url, method: init.method, headers: init.headers, body: init.body });
      const answer =
        init.method === "GET" ? { status: 200, body: { items: FIELDS } } : (postAnswers.shift() ?? { status: 500, body: {} });
      return { status: answer.status, ok: answer.status < 300, json: async () => answer.body };
    });
  });

  const mount = () =>
    gan(
      createElement(CommuneSendScreen, {
        ten_xa: "Xã Thử Nghiệm",
        ho_ten: null,
        accountless: { domain: DOMAIN },
        // Injected on purpose: the accountless path must NOT offer photos or a location button even when the
        // shell provides them (the location exchange needs the access token Zalo refused).
        pickScenePhotos: vi.fn(),
        getSceneLocation: locate,
        onBack: () => {},
        onSessionLost: () => {
          throw new Error("the accountless path has no session to lose");
        },
        onSent: () => {
          throw new Error("the accountless path never reports a session petition");
        },
        onOpenPetition: () => {},
      }),
    );

  async function fill(khung: PhanTuGia) {
    await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    await typeInto(doc().getElementById("xa-noi-dung")!, "Rác tồn đọng đầu ngõ 12");
    await typeInto(doc().getElementById("xa-ho-ten")!, "Nguyễn Văn An");
    await typeInto(doc().getElementById("xa-dien-thoai")!, "0900000000");
  }

  it("the field list comes from the PUBLIC route by domain; no photo buttons; the cost is said before sending", async () => {
    const { khung, go } = await mount();
    expect(citizenReportFields).not.toHaveBeenCalled();
    expect(calls).toHaveLength(1);
    const fields = new URL(calls[0]!.url);
    expect(fields.pathname).toBe("/api/v1/public-citizen-report-fields");
    expect(fields.searchParams.get("host")).toBe(DOMAIN);
    expect(calls[0]!.headers["Authorization"]).toBeUndefined();
    expect(khung.textContent).toContain(FIELDS[0].label);
    await press(tatCaPhanTu(khung).filter((p) => p.getAttribute("role") === "radio")[0]!);
    expect(khung.textContent).toContain(ACCOUNTLESS.form_note);
    expect(khung.textContent).toContain(ACCOUNTLESS.no_photos);
    expect(buttonsNamed(khung, SCENE_PHOTOS.take)).toEqual([]);
    expect(buttonsNamed(khung, SCENE_PHOTOS.pick)).toEqual([]);
    // No location button; the address box stays, and the hint says to write the place there.
    expect(buttonsNamed(khung, XA_TN.vi_tri_nut)).toEqual([]);
    expect(khung.textContent).not.toContain(XA_TN.vi_tri_nut);
    expect(doc().getElementById("xa-dia-chi")).not.toBeNull();
    expect(khung.textContent).toContain(XA_PA.location_without_button);
    expect(locate).not.toHaveBeenCalled();
    // Name and number required, anonymous off and disabled — the typed-contact rules (ADR 0083 #7).
    const toggle = tatCaPhanTu(khung).find((p) => p.getAttribute("role") === "switch")!;
    expect(toggle.hasAttribute("disabled")).toBe(true);
    expect(doc().getElementById("xa-dien-thoai")!.getAttribute("aria-required")).toBe("true");
    // The commune is read again at the last step (README non-negotiable #5).
    expect(khung.textContent).toContain(XA_PA.gui_toi("Xã Thử Nghiệm"));
    await go();
  });

  it("429 `rate_limited` → the plain sentence, words kept; 'Gửi lại' re-sends the SAME key and body; 201 → the code", async () => {
    const { khung, go } = await mount();
    await fill(khung);
    postAnswers.push({ status: 429, body: { code: "rate_limited", message: "x" } });
    postAnswers.push({
      status: 201,
      body: { code: "PB9M3K7Q2XTD", status: "da-tiep-nhan", acknowledge_due: null, resolve_due: null },
    });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(khung.textContent).toContain(LOI_GUI["rate-limited"].cau);
    expect(khung.textContent).not.toContain("rate_limited");
    expect(boxText("xa-noi-dung")).toBe("Rác tồn đọng đầu ngõ 12");
    await press(buttonsNamed(khung, GUI.nut_gui_lai)[0]!);

    const posts = calls.filter((c) => c.method === "POST");
    expect(posts).toHaveLength(2);
    for (const c of posts) {
      expect(new URL(c.url).pathname).toBe("/api/v1/public-citizen-reports");
      expect(new URL(c.url).search).toBe("");
      expect(c.headers["Authorization"]).toBeUndefined();
      expect(c.headers["Idempotency-Key"]).toMatch(/^[0-9a-f]{32}$/);
    }
    expect(posts[1]!.headers["Idempotency-Key"]).toBe(posts[0]!.headers["Idempotency-Key"]);
    expect(posts[1]!.body).toBe(posts[0]!.body);
    const body = JSON.parse(posts[0]!.body!) as Record<string, unknown>;
    expect(body).toEqual({
      content: "Rác tồn đọng đầu ngõ 12",
      address: "",
      reporter_name: "Nguyễn Văn An",
      reporter_phone: "0900000000",
      anonymous: false,
      field: FIELDS[0].code,
      host: DOMAIN,
    });
    expect(body).not.toHaveProperty("lat");
    expect(body).not.toHaveProperty("lng");
    expect(guiPhanAnh).not.toHaveBeenCalled();
    // The done screen: the code, and the sentence that no notification will ever come.
    expect(khung.textContent).toContain("#PB9M3K7Q2XTD");
    expect(khung.textContent).toContain(ACCOUNTLESS.done_notice);
    expect(buttonsNamed(khung, ACCOUNTLESS.follow)).toHaveLength(1);
    await go();
  });

  it("429 `commune_daily_limit` → its own sentence pointing to the reception desk; the draft stays", async () => {
    const { khung, go } = await mount();
    await fill(khung);
    postAnswers.push({ status: 429, body: { code: "commune_daily_limit" } });
    await press(buttonsNamed(khung, GUI.tieu_de)[0]!);
    expect(khung.textContent).toContain(LOI_GUI["commune-daily-limit"].cau);
    expect(LOI_GUI["commune-daily-limit"].cau).toContain("Bộ phận tiếp nhận của Ủy ban nhân dân xã");
    expect(boxText("xa-ho-ten")).toBe("Nguyễn Văn An");
    await go();
  });
});
