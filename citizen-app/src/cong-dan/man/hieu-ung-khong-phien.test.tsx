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
  phien: undefined as { token: string; ten_xa: string } | undefined,
}));

vi.mock("../api/phien-vigov", async (importOriginal) => {
  const goc = await importOriginal<typeof import("../api/phien-vigov")>();
  return { layPhienViGov: () => (trang.phien === undefined ? goc.layPhienViGov() : trang.phien) };
});

vi.mock("../api/goi-vigov", async (importOriginal) => {
  const goc = await importOriginal<typeof import("../api/goi-vigov")>();
  return {
    ...goc,
    guiPhanAnh: vi.fn(goc.guiPhanAnh),
    traCuuPhieu: vi.fn(goc.traCuuPhieu),
    phanAnhCuaToi: vi.fn(goc.phanAnhCuaToi),
    citizenReportFields: vi.fn(goc.citizenReportFields),
  };
});

import { citizenReportFields, guiPhanAnh, phanAnhCuaToi, traCuuPhieu } from "../api/goi-vigov";
import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";
import { layPhienViGov } from "../api/phien-vigov";

import { GuiPhanAnhScreen, ID_DAU_BUOC } from "./GuiPhanAnhScreen";
import { CUA_TOI, GUI, KENH_CHUA_MO, LOI_GUI, PHONE_VERIFICATION, QUAY_LAI } from "./noi-dung";
import { PhanAnhCuaToiScreen } from "./PhanAnhCuaToiScreen";
import { TraCuuPhieuScreen } from "./TraCuuPhieuScreen";

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
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
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
