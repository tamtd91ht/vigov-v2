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
  };
});

import { guiPhanAnh, phanAnhCuaToi, traCuuPhieu } from "../api/goi-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { GuiPhanAnhScreen } from "./GuiPhanAnhScreen";
import { CUA_TOI, KENH_CHUA_MO, QUAY_LAI } from "./noi-dung";
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
  addEventListener() {}
  removeEventListener() {}
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
}

class TaiLieuGia extends NutGia {
  readonly documentElement: PhanTuGia;
  readonly body: PhanTuGia;
  activeElement: PhanTuGia | null = null;
  defaultView: unknown = null;
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
}

function tatCaPhanTu(nut: NutGia): PhanTuGia[] {
  return nut.childNodes.flatMap((c) => (c instanceof PhanTuGia ? [c, ...tatCaPhanTu(c)] : []));
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

  it("Gửi phản ánh: không nút gửi, không gọi `guiPhanAnh`, không `fetch`", async () => {
    const { khung, go } = await gan(createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }));
    khangDinhKenhChuaMo(khung);
    expect(guiPhanAnh).not.toHaveBeenCalled();
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

  it("Tra cứu có mã điền sẵn gọi `traCuuPhieu(mã)` một lần", async () => {
    trang.phien = { token: "tok-thu-nghiem", ten_xa: "Xã Thử Nghiệm" };
    vi.mocked(traCuuPhieu).mockResolvedValueOnce({ kieu: "khong-thay" });
    const { go } = await gan(createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD" }));
    expect(vi.mocked(traCuuPhieu).mock.calls).toEqual([["PA7K2QX9M4TD"]]);
    await go();
  });
});
