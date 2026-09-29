import { act, createElement, type ReactElement, StrictMode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * "CHƯA CÓ PHIÊN ViGov THÌ MÀN KHÔNG GỌI MẠNG" — kiểm khi màn ĐƯỢC GẮN THẬT và hiệu ứng CHẠY THẬT.
 *
 * VÌ SAO CẦN TỆP NÀY: `citizen.test.tsx` dựng màn bằng `renderToStaticMarkup`, và đường ấy KHÔNG BAO
 * GIỜ chạy `useEffect`. Nhánh chặn trong hiệu ứng (`MyReportsScreen.tsx` `session === null` trong
 * `useEffect`, `ReportLookupScreen.tsx` cùng chỗ) vì thế chưa từng được một ca nào chạm tới: xoá vế
 * `session === null` đi thì mọi ca cũ vẫn xanh.
 *
 * VÌ SAO MỘT DOM TỰ DỰNG TRONG TỆP NÀY, KHÔNG PHẢI jsdom: kho không có jsdom / happy-dom /
 * @testing-library, và thêm một phụ thuộc là việc phải hỏi. React 19 `react-dom/client` chỉ cần một
 * nhúm phương thức nút (`createElement`, `appendChild`, `setAttribute`, …) để gắn một cây KHÔNG có ô
 * nhập nào — đúng cây của "kênh chưa mở". Giới hạn: shim này không mô phỏng sự kiện; ca nào cần bấm
 * thì cần một DOM thật. Mọi `console.error` của React bị bắt và làm ca đỏ, nên shim thiếu gì thì ca
 * nói ra chứ không xanh sai.
 *
 * VÌ SAO HAI GIÁN ĐIỆP, KHÔNG PHẢI MỘT: `api/vigov-client.ts` TỰ chặn trước `fetch` khi không có phiên.
 * Nên "`fetch` không được gọi" đúng cả khi màn bỏ mất nhánh chặn của nó — lớp dưới đỡ hộ. Gián điệp
 * trên hàm gọi API (`myReports`, `lookupReport`, `submitReport`) mới bắt được chính nhánh của MÀN;
 * gián điệp `fetch` giữ lời hứa cả tầng.
 *
 * NGUỒN PHIÊN LÀ NGUỒN THẬT trong mọi ca "không phiên": `vi.mock` bên dưới CHUYỂN THẲNG tới
 * `getVigovSession` thật trừ khi một ca đối chứng đặt `page.session`. Mã sản phẩm không có khe nào để
 * đưa phiên vào; việc thay thế chỉ sống trong tệp test này (cùng cách `api/vigov-client.test.tsx` làm).
 */

const page = vi.hoisted(() => ({
  /** `undefined` = dùng nguồn phiên THẬT. Chỉ ca đối chứng đặt giá trị. */
  session: undefined as { token: string; commune_name: string } | undefined,
}));

vi.mock("../api/vigov-session", async (importOriginal) => {
  const base = await importOriginal<typeof import("../api/vigov-session")>();
  return { getVigovSession: () => (page.session === undefined ? base.getVigovSession() : page.session) };
});

vi.mock("../api/vigov-client", async (importOriginal) => {
  const base = await importOriginal<typeof import("../api/vigov-client")>();
  return {
    ...base,
    submitReport: vi.fn(base.submitReport),
    lookupReport: vi.fn(base.lookupReport),
    myReports: vi.fn(base.myReports),
  };
});

import { submitReport, myReports, lookupReport } from "../api/vigov-client";
import { getVigovSession } from "../api/vigov-session";

import { SubmitReportScreen } from "./SubmitReportScreen";
import { MY_REPORTS, CHANNEL_NOT_OPEN, PHONE_VERIFICATION, BACK } from "./copy";
import { MyReportsScreen } from "./MyReportsScreen";
import { ReportLookupScreen } from "./ReportLookupScreen";

// ---------------------------------------------------------------------------------------------
// DOM tối thiểu — đủ cho `react-dom/client` gắn, cập nhật và gỡ một cây tĩnh.
// ---------------------------------------------------------------------------------------------

const HTML_NS = "http://www.w3.org/1999/xhtml";

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
  insertBefore(child: FakeNode, previous: FakeNode | null) {
    if (previous === null) return this.appendChild(child);
    child.parentNode?.removeChild(child);
    const i = this.childNodes.indexOf(previous);
    if (i < 0) throw new Error("insertBefore: nút tham chiếu không phải con");
    this.childNodes.splice(i, 0, child);
    child.parentNode = this;
    return child;
  }
  removeChild(child: FakeNode) {
    const i = this.childNodes.indexOf(child);
    if (i < 0) throw new Error("removeChild: không phải con");
    this.childNodes.splice(i, 1);
    child.parentNode = null;
    return child;
  }
  contains(other: FakeNode | null): boolean {
    for (let n = other; n !== null; n = n.parentNode) if (n === this) return true;
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
  readonly namespaceURI = HTML_NS;
  readonly attributes = new Map<string, string>();
  readonly style: Record<string, string> = {};
  onclick: unknown = null;
  constructor(
    readonly tagName: string,
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
    return new FakeElement(name.toUpperCase(), this);
  }
  createTextNode(text: string) {
    return new FakeText(text, this);
  }
}

function allElements(node: FakeNode): FakeElement[] {
  return node.childNodes.flatMap((c) => (c instanceof FakeElement ? [c, ...allElements(c)] : []));
}

// `react-dom/client` quyết định "có DOM không" LÚC NẠP MÔ-ĐUN — nên dựng `window`/`document` trước,
// rồi mới `import()` nó.
let createRootFn: typeof import("react-dom/client").createRoot;
const saved = { window: (globalThis as Record<string, unknown>).window, document: (globalThis as Record<string, unknown>).document };

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
  createRootFn = (await import("react-dom/client")).createRoot;
});

afterAll(() => {
  Object.assign(globalThis, { window: saved.window, document: saved.document, IS_REACT_ACT_ENVIRONMENT: undefined });
});

let fake_fetch: ReturnType<typeof vi.fn>;
let react_errors: unknown[][];

beforeEach(() => {
  page.session = undefined;
  vi.mocked(submitReport).mockClear();
  vi.mocked(lookupReport).mockClear();
  vi.mocked(myReports).mockClear();
  fake_fetch = vi.fn();
  vi.stubGlobal("fetch", fake_fetch);
  react_errors = [];
  vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => void react_errors.push(a));
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.mocked(console.error).mockRestore();
  // Shim thiếu gì, React sẽ nói qua `console.error` — và ca không được xanh trên một cây gắn hỏng.
  expect(react_errors).toEqual([]);
});

/**
 * Gắn thật trong StrictMode (hiệu ứng chạy HAI lần ở chế độ dev), chờ mọi hiệu ứng và mọi promise
 * chúng khởi động, rồi trả cây và hàm gỡ.
 */
async function mount(screen: ReactElement) {
  const doc = (globalThis as unknown as { document: FakeDocument }).document;
  const frame = doc.createElement("div");
  doc.body.appendChild(frame);
  const root = createRootFn(frame as unknown as Element);
  await act(async () => {
    root.render(createElement(StrictMode, null, screen));
  });
  // Một lượt nữa: `load("")` / `lookup()` là async — kết quả của chúng rơi vào lượt sau.
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  return {
    frame,
    unmount: () => act(() => root.unmount()),
  };
}

function assertChannelNotOpen(frame: FakeElement) {
  const text = frame.textContent;
  expect(text).toContain(CHANNEL_NOT_OPEN.title);
  expect(text).not.toContain(MY_REPORTS.loading);
  expect(text).not.toContain(MY_REPORTS.empty);
  const tag_names = allElements(frame).map((p) => p.tagName);
  expect(tag_names).not.toContain("INPUT");
  expect(tag_names).not.toContain("TEXTAREA");
  expect(tag_names).not.toContain("FORM");
  // Nút DUY NHẤT là "Quay lại": không có nút gửi / tra / xem thêm nào để một cú bấm đi ra mạng.
  const buttons = allElements(frame).filter((p) => p.tagName === "BUTTON");
  expect(buttons.map((n) => n.textContent)).toEqual([BACK]);
}

describe("nguồn phiên THẬT (null) — màn gắn thật, hiệu ứng chạy, không một lời gọi nào đi ra", () => {
  it("tiền đề: nguồn phiên của tệp này đúng là nguồn thật và trả `null`", () => {
    expect(getVigovSession()).toBeNull();
  });

  it("Phản ánh của tôi: hiệu ứng tải trang đầu KHÔNG gọi `myReports`, không `fetch`", async () => {
    const { frame, unmount } = await mount(
      createElement(MyReportsScreen, { onBack: () => {}, onOpenReport: () => {}, onSubmitReport: () => {} }),
    );
    assertChannelNotOpen(frame);
    expect(myReports).not.toHaveBeenCalled();
    expect(fake_fetch).not.toHaveBeenCalled();
    await unmount();
  });

  it("Tra cứu có mã điền sẵn: hiệu ứng tra ngay KHÔNG gọi `lookupReport`, không `fetch`", async () => {
    const { frame, unmount } = await mount(
      createElement(ReportLookupScreen, { onBack: () => {}, initial_code: "PA7K2QX9M4TD" }),
    );
    assertChannelNotOpen(frame);
    expect(lookupReport).not.toHaveBeenCalled();
    expect(fake_fetch).not.toHaveBeenCalled();
    await unmount();
  });

  it("Tra cứu không mã: không ô nhập, không nút tra, không gọi gì", async () => {
    const { frame, unmount } = await mount(createElement(ReportLookupScreen, { onBack: () => {} }));
    assertChannelNotOpen(frame);
    expect(lookupReport).not.toHaveBeenCalled();
    expect(fake_fetch).not.toHaveBeenCalled();
    await unmount();
  });

  it("Gửi phản ánh: không nút gửi, không gọi `submitReport`, không `fetch`", async () => {
    const { frame, unmount } = await mount(createElement(SubmitReportScreen, { onBack: () => {} }));
    assertChannelNotOpen(frame);
    expect(submitReport).not.toHaveBeenCalled();
    expect(fake_fetch).not.toHaveBeenCalled();
    await unmount();
  });
});

/**
 * ĐỐI CHỨNG — chứng minh ba ca trên KHÔNG xanh vì hiệu ứng không bao giờ chạy trong khung gắn này.
 * Có phiên (giả, chỉ trong tệp này) thì cùng khung gắn ấy PHẢI thấy lời gọi. Nếu ca này đỏ, "không
 * gọi" ở trên chẳng chứng minh gì.
 */
describe("đối chứng: có phiên thì chính khung gắn này thấy hiệu ứng gọi đi — đúng một lần", () => {
  it("Phản ánh của tôi gọi `myReports('')` một lần dù StrictMode chạy hiệu ứng hai lần", async () => {
    page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    vi.mocked(myReports).mockResolvedValueOnce({ kind: "chua-co-phien" });
    const { frame, unmount } = await mount(
      createElement(MyReportsScreen, { onBack: () => {}, onOpenReport: () => {}, onSubmitReport: () => {} }),
    );
    expect(vi.mocked(myReports).mock.calls).toEqual([[""]]);
    // Máy chủ nói "chưa có phiên" → màn đóng kênh, không kẹt ở "đang tải".
    expect(frame.textContent).toContain(CHANNEL_NOT_OPEN.title);
    await unmount();
  });

  /**
   * 403 `chua_xac_thuc_so` NGAY LÚC GẮN (tải trang đầu / tra mã điền sẵn): màn HỎI, và chỉ hỏi — hàm mở
   * lại phiên kèm số (thứ bật hộp thoại xin số của Zalo) KHÔNG được gọi khi công dân chưa bấm gì.
   */
  it("403 `chua_xac_thuc_so` lúc gắn: khung hỏi số hiện ra, KHÔNG tự xin số, không gọi lại", async () => {
    page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    const reopen = vi.fn();
    vi.mocked(myReports).mockResolvedValueOnce({ kind: "can-xac-thuc-so" });
    const mine = await mount(
      createElement(MyReportsScreen, {
        onBack: () => {},
        onOpenReport: () => {},
        onSubmitReport: () => {},
        reopenWithPhone: reopen,
      }),
    );
    expect(mine.frame.textContent).toContain(PHONE_VERIFICATION.title);
    expect(mine.frame.textContent).toContain(PHONE_VERIFICATION.allow);
    // Không kẹt ở "đang tải", không nói "chưa gửi phản ánh nào" — chưa biết gì về danh sách.
    expect(mine.frame.textContent).not.toContain(MY_REPORTS.loading);
    expect(mine.frame.textContent).not.toContain(MY_REPORTS.empty);
    expect(vi.mocked(myReports)).toHaveBeenCalledTimes(1);
    await mine.unmount();

    vi.mocked(lookupReport).mockResolvedValueOnce({ kind: "can-xac-thuc-so" });
    const lookup = await mount(
      createElement(ReportLookupScreen, { onBack: () => {}, initial_code: "PA7K2QX9M4TD", reopenWithPhone: reopen }),
    );
    expect(lookup.frame.textContent).toContain(PHONE_VERIFICATION.title);
    expect(vi.mocked(lookupReport)).toHaveBeenCalledTimes(1);
    await lookup.unmount();

    expect(reopen).not.toHaveBeenCalled();
    expect(fake_fetch).not.toHaveBeenCalled();
  });

  it("Tra cứu có mã điền sẵn gọi `lookupReport(mã)` một lần", async () => {
    page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    vi.mocked(lookupReport).mockResolvedValueOnce({ kind: "khong-thay" });
    const { unmount } = await mount(createElement(ReportLookupScreen, { onBack: () => {}, initial_code: "PA7K2QX9M4TD" }));
    expect(vi.mocked(lookupReport).mock.calls).toEqual([["PA7K2QX9M4TD"]]);
    await unmount();
  });
});
