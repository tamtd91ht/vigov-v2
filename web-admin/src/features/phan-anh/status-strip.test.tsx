// @vitest-environment jsdom
//
// jsdom for this file: owner decision D1 is BEHAVIOUR — pressing a chip opens its act, pressing a
// blocked chip does nothing, `Huỷ` folds the composer. The search box filters 300ms after the last key,
// and `?id=` opens one petition's drawer. A markup string can show none of these.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { congThaoTac } from "./nhan-phieu";
import { ChiTietPhieu, SoPhanAnh } from "./so-phan-anh";

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => ({ ok: true, duLieu: { permissions: ["feedback.read"] } }),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

const PETITION: petitions_phieuPhanAnhRa = {
  code: "PA-2026-0021",
  channel: "zalo-mini-app",
  status: "dang-xu-ly",
  field: "rac-thai",
  field_label: "Rác thải – Vệ sinh môi trường",
  content: "Rác tồn đọng ở đầu ngõ.",
  address: "Đầu ngõ thôn Hà Lam",
  reporter_name: "",
  reporter_phone: "",
  anonymous: true,
  clock_from: "2026-09-09T07:20:00Z",
  booked_at: "2026-09-09T07:21:00Z",
  acknowledge_due: "2026-09-09T09:20:00Z",
  resolve_due: "2026-09-10T09:20:00Z",
  classify_due: "2026-09-09T11:20:00Z",
  unit: "",
  assignee: "",
  result: "",
  public: false,
};

function drawer(p: petitions_phieuPhanAnhRa, cong: ReturnType<typeof congThaoTac>, tienTrangThai = vi.fn()) {
  return mount(
    <ChiTietPhieu
      phieu={p}
      bayGio={new Date("2026-09-09T08:00:00Z")}
      cong={cong}
      tenBoPhan={new Map()}
      boPhan={[]}
      danhBa={null}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={tienTrangThai}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
    />,
  );
}

/** The strip chip whose first line is `label`. */
function chip(el: HTMLElement, label: string): HTMLButtonElement {
  const b = [...el.querySelectorAll<HTMLButtonElement>("li > button")].find(
    (x) => x.firstElementChild?.textContent === label,
  );
  if (b === undefined) throw new Error(`no chip ${label}`);
  return b;
}

const composer = (el: HTMLElement) => el.querySelector('[role="group"][aria-label^="Chuyển sang"]');

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
});

describe("status strip — pressing a chip (owner decision D1)", () => {
  it("a pressable chip opens ITS act under the strip; `Huỷ` folds it; `Xác nhận` sends that act", () => {
    const tien = vi.fn();
    const el = drawer(PETITION, congThaoTac(false, false, false), tien);
    expect(composer(el)).toBeNull();

    act(() => chip(el, "Đã xử lý").click());
    expect(composer(el)?.textContent).toContain("Chuyển sang “Đã xử lý”");
    expect(chip(el, "Đã xử lý").getAttribute("aria-expanded")).toBe("true");
    expect(el.querySelector("#ghi-chu-tien")).not.toBeNull();

    const cancel = [...composer(el)!.querySelectorAll("button")].find((b) => b.textContent === "Huỷ");
    act(() => cancel?.click());
    expect(composer(el)).toBeNull();

    act(() => chip(el, "Đã xử lý").click());
    const form = composer(el)!.querySelector("form")!;
    act(() => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(tien).toHaveBeenCalledTimes(1);
  });

  it("DENIED — a blocked chip opens nothing: no key for the act, or not a move from here", () => {
    const el = drawer({ ...PETITION, status: "cho-dan-xac-nhan" }, congThaoTac(true, true, false));
    const close = chip(el, "Đã đóng");
    expect(close.getAttribute("aria-disabled")).toBe("true");
    expect(close.title).toContain("feedback.resolve");
    act(() => close.click());
    expect(composer(el)).toBeNull();
    expect(el.querySelector("#ket-qua-xu-ly")).toBeNull();

    act(() => chip(el, "Đã tiếp nhận").click());
    expect(composer(el)).toBeNull();
    expect(el.textContent).toContain("Bạn không có quyền đổi trạng thái phiếu này.");
  });
});

describe("search as you type — 300ms after the last key (row 10)", () => {
  it("one list read with `q` after the pause, none before; the words never reach the address bar", () => {
    vi.useFakeTimers();
    const fetchSpy = vi.fn((_url: string) => new Promise<Response>(() => {}));
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(<SoPhanAnh />);
    const box = el.querySelector<HTMLInputElement>("#tim-phan-anh")!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    act(() => {
      setter?.call(box, "ngõ 5");
      box.dispatchEvent(new Event("input", { bubbles: true }));
    });
    // The LIST read; the count route (`Danh sách (n)`) takes the same filters and is checked apart.
    const withQ = () =>
      fetchSpy.mock.calls.filter(([u]) => String(u).startsWith("/api/v1/citizen-reports?") && String(u).includes("q="));
    const countWithQ = () =>
      fetchSpy.mock.calls.filter(([u]) => String(u).startsWith("/api/v1/citizen-report-counts?") && String(u).includes("q="));
    act(() => vi.advanceTimersByTime(299));
    expect(withQ()).toEqual([]);
    expect(countWithQ()).toEqual([]);
    act(() => vi.advanceTimersByTime(1));
    expect(withQ().length).toBe(1);
    expect(String(withQ()[0]?.[0])).toContain("q=ng%C3%B5+5");
    expect(countWithQ().length).toBe(1);
    expect(window.location.search).not.toContain("ng");
    expect(window.location.href).not.toContain("ng%C3%B5");
  });
});

describe("`?id=` deep link (row 1)", () => {
  it("reads THAT petition through the detail route and opens its drawer", async () => {
    const fetchSpy = vi.fn((url: string) =>
      String(url).endsWith("/api/v1/citizen-reports/PA-2026-0021")
        ? Promise.resolve(new Response(JSON.stringify(PETITION), { status: 200, headers: { "Content-Type": "application/json" } }))
        : new Promise<Response>(() => {}),
    );
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(<SoPhanAnh openCode="PA-2026-0021" />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(fetchSpy.mock.calls.some(([u]) => String(u).endsWith("/api/v1/citizen-reports/PA-2026-0021"))).toBe(true);
    expect(el.querySelector("dialog")?.textContent).toContain("PA-2026-0021");
  });
});
