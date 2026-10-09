// @vitest-environment jsdom
//
// jsdom for this file: a press, a key, a toast, a request in flight are events — a markup string
// cannot show them. The markup itself is pinned by `so-bien-ban.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

import { SoBienBan } from "./so-bien-ban";

const api = vi.hoisted(() => ({
  laySoBienBan: vi.fn(),
  layBienBan: vi.fn(),
  taoBienBan: vi.fn(),
  suaBienBan: vi.fn(),
  themKetLuan: vi.fn(),
  suaKetLuan: vi.fn(),
  xoaKetLuan: vi.fn(),
  xoaBienBan: vi.fn(),
  kyBienBan: vi.fn(),
  danhDauKhongPhatSinh: vi.fn(),
  boDauKhongPhatSinh: vi.fn(),
  layNhiemVuCuaKetLuan: vi.fn(),
  tachKetLuanThanhNhiemVu: vi.fn(),
}));

vi.mock("@/lib/api/bien-ban", () => api);
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => ({ ok: true, duLieu: { permissions: [] } }),
}));
const EMPTY = { ok: true, duLieu: { items: [] } };
vi.mock("@/lib/api/danh-ba-chon-nguoi", () => ({ layDanhBaChonNguoi: () => Promise.resolve(EMPTY) }));
vi.mock("@/lib/api/danh-muc", () => ({ layDanhMucBoPhan: () => Promise.resolve(EMPTY) }));
vi.mock("@/lib/api/danh-muc-nghiep-vu", () => ({
  layKhoiNhiemVu: () => Promise.resolve(EMPTY),
  layLoaiNhiemVu: () => Promise.resolve(EMPTY),
  layMucUuTienNhiemVu: () => Promise.resolve(EMPTY),
}));
vi.mock("@/lib/api/trang-thai-nhiem-vu", () => ({ layTrangThaiNhiemVu: () => Promise.resolve(EMPTY) }));
// The split dialog is `FormGiaoViec`, shared with Nhiệm vụ and pinned there. Here only its hand-off
// matters: what this screen does once the form asks to create the task.
vi.mock("@/features/nhiem-vu/so-nhiem-vu", () => ({
  FormGiaoViec: ({ giaoViec }: { giaoViec: (body: unknown, key: string) => void }) => (
    <button type="button" onClick={() => giaoViec({ title: "Việc tách" }, "key-1")}>
      stub-assign-task
    </button>
  ),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function conclusion(patch: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
  return {
    id: "k1",
    ordinal: 1,
    content: "Kết luận một.",
    task_count: 0,
    task_done_count: 0,
    status: "chua-giao",
    no_task: false,
    created_at: "2026-08-05T02:00:00Z",
    ...patch,
  };
}

function meeting(patch: Partial<petitions_bienBanRa> = {}): petitions_bienBanRa {
  return {
    id: "01JBB1",
    title: "Giao ban tháng 8",
    held_on: "2026-08-05",
    reference_no: "",
    location: "",
    chaired_by: "",
    conclusions: [conclusion()],
    task_count: 0,
    task_done_count: 0,
    status: "du-thao",
    conclusion_count: 1,
    conclusion_done_count: 0,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-08-05T02:00:00Z",
    ...patch,
  };
}

function ok<T>(data: T): Promise<KetQua<T>> {
  return Promise.resolve({ ok: true, duLieu: data });
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  api.laySoBienBan.mockImplementation(() => ok({ items: [meeting()], has_more: false, next_cursor: "" }));
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.clearAllMocks();
});

async function settle(): Promise<void> {
  for (let i = 0; i < 8; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<SoBienBan />));
  await settle();
  return host;
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement {
  const b = [...el.querySelectorAll<HTMLButtonElement>("button")].find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function typeInto(el: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function press(el: Element, k: string, shiftKey = false): void {
  act(() => {
    el.dispatchEvent(new KeyboardEvent("keydown", { key: k, shiftKey, bubbles: true, cancelable: true }));
  });
}

function submitOf(textareaId: string, page: ParentNode): Element {
  return page.querySelector(`#${textareaId}`)!.closest("form")!.querySelector('button[type="submit"]')!;
}

describe("Nhập biên bản — lỗi dưới ô, toast khi lưu xong", () => {
  it("bấm Lưu khi thiếu tên và ngày: hai câu lỗi dưới hai ô, KHÔNG gửi gì", async () => {
    const page = await mount();
    act(() => buttonByText(page, "Nhập biên bản").click());
    const dialog = page.querySelector("dialog")!;
    act(() => buttonByText(dialog, "Lưu biên bản").click());

    const alerts = [...dialog.querySelectorAll('[role="alert"]')].map((a) => a.textContent);
    expect(alerts).toEqual(["Cần tên cuộc họp.", "Cần ngày họp."]);
    expect(dialog.querySelector("#ten-cuoc-hop")!.getAttribute("aria-invalid")).toBe("true");
    expect(api.taoBienBan).not.toHaveBeenCalled();
  });

  it("đủ tên và ngày: gửi, toast “Đã lưu biên bản”, đóng hộp", async () => {
    api.taoBienBan.mockImplementation(() => ok(meeting({ id: "01JBB2" })));
    const page = await mount();
    act(() => buttonByText(page, "Nhập biên bản").click());
    const dialog = page.querySelector("dialog")!;
    typeInto(dialog.querySelector<HTMLInputElement>("#ten-cuoc-hop")!, "Giao ban tuần 34");
    typeInto(dialog.querySelector<HTMLInputElement>("#ngay-hop")!, "2026-08-20");
    act(() => buttonByText(dialog, "Lưu biên bản").click());
    await settle();

    expect(api.taoBienBan).toHaveBeenCalledTimes(1);
    expect(api.taoBienBan.mock.calls[0]![0]).toMatchObject({ title: "Giao ban tuần 34", held_on: "2026-08-20" });
    expect(toast.success).toHaveBeenCalledWith("Đã lưu biên bản");
    expect(page.querySelector("dialog")).toBeNull();
  });

  it("máy chủ từ chối: câu của máy chủ NGUYÊN VĂN trong hộp, không toast, hộp ở lại", async () => {
    api.taoBienBan.mockImplementation(() =>
      Promise.resolve({ ok: false, thongBao: "biên bản họp: thiếu ngày họp" }),
    );
    const page = await mount();
    act(() => buttonByText(page, "Nhập biên bản").click());
    const dialog = page.querySelector("dialog")!;
    typeInto(dialog.querySelector<HTMLInputElement>("#ten-cuoc-hop")!, "Giao ban");
    typeInto(dialog.querySelector<HTMLInputElement>("#ngay-hop")!, "2026-08-20");
    act(() => buttonByText(dialog, "Lưu biên bản").click());
    await settle();

    expect(page.querySelector("dialog")!.textContent).toContain("biên bản họp: thiếu ngày họp");
    expect(toast.success).not.toHaveBeenCalled();
  });
});

describe("Thêm kết luận — Loader trên ĐÚNG thẻ đang gửi", () => {
  it("đang gửi: nút của thẻ ấy quay Loader thay Plus; xong thì trở lại", async () => {
    let finish: (kq: KetQua<petitions_ketLuanRa>) => void = () => {};
    api.themKetLuan.mockImplementation(() => new Promise((r) => (finish = r)));
    api.laySoBienBan.mockImplementation(() =>
      ok({
        items: [meeting(), meeting({ id: "01JBB2", title: "Giao ban tháng 9" })],
        has_more: false,
        next_cursor: "",
      }),
    );
    const page = await mount();
    const box = page.querySelector<HTMLTextAreaElement>("#them-ket-luan-01JBB1")!;
    typeInto(box, "Kết luận mới.");
    act(() => box.form!.requestSubmit());

    expect(submitOf("them-ket-luan-01JBB1", page).querySelector("svg")!.getAttribute("class")).toContain(
      "animate-spin",
    );
    expect(submitOf("them-ket-luan-01JBB2", page).querySelector("svg")!.getAttribute("class")).not.toContain(
      "animate-spin",
    );

    await act(async () => finish({ ok: true, duLieu: conclusion({ id: "k9", ordinal: 2 }) }));
    await settle();
    expect(submitOf("them-ket-luan-01JBB1", page).querySelector("svg")!.getAttribute("class")).not.toContain(
      "animate-spin",
    );
  });
});

describe("Tách thành nhiệm vụ — toast khi xong", () => {
  it("tách xong: toast “Đã tách thành nhiệm vụ”, hộp đóng, không câu báo nào trên dòng", async () => {
    api.tachKetLuanThanhNhiemVu.mockImplementation(() => ok({ code: "NV12" }));
    const page = await mount();
    act(() => buttonByText(page, "Tách thành nhiệm vụ").click());
    act(() => buttonByText(page, "stub-assign-task").click());
    await settle();

    expect(api.tachKetLuanThanhNhiemVu).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith("Đã tách thành nhiệm vụ");
    expect(page.textContent).not.toContain("stub-assign-task");
    expect(page.textContent).not.toContain("NV12");
  });
});

describe("Sửa kết luận tại chỗ — Enter lưu, Shift+Enter xuống dòng, Esc huỷ", () => {
  it("Esc huỷ: ô nhập biến mất, câu trở lại, không gửi", async () => {
    const page = await mount();
    act(() => page.querySelector<HTMLButtonElement>('[aria-label="Sửa kết luận số 1"]')!.click());
    const box = page.querySelector<HTMLTextAreaElement>("#sua-ket-luan-k1")!;
    expect(document.activeElement).toBe(box);
    press(box, "Escape");

    expect(page.querySelector("#sua-ket-luan-k1")).toBeNull();
    expect(page.textContent).toContain("Kết luận một.");
    expect(api.suaKetLuan).not.toHaveBeenCalled();
  });

  it("Shift+Enter không gửi; Enter gửi đúng câu đã sửa qua tuyến PATCH cũ", async () => {
    api.suaKetLuan.mockImplementation(() => ok(null));
    const page = await mount();
    act(() => page.querySelector<HTMLButtonElement>('[aria-label="Sửa kết luận số 1"]')!.click());
    const box = page.querySelector<HTMLTextAreaElement>("#sua-ket-luan-k1")!;
    typeInto(box, "Kết luận một, đã sửa.");
    press(box, "Enter", true);
    expect(api.suaKetLuan).not.toHaveBeenCalled();
    press(box, "Enter");
    await settle();

    expect(api.suaKetLuan).toHaveBeenCalledTimes(1);
    expect(api.suaKetLuan.mock.calls[0]![2]).toBe("Kết luận một, đã sửa.");
  });

  it("máy chủ từ chối: câu NGUYÊN VĂN trên dòng ấy, ô nhập giữ chữ đã gõ", async () => {
    api.suaKetLuan.mockImplementation(() =>
      Promise.resolve({ ok: false, thongBao: "kết luận đã được tách — không sửa được" }),
    );
    const page = await mount();
    act(() => page.querySelector<HTMLButtonElement>('[aria-label="Sửa kết luận số 1"]')!.click());
    const box = page.querySelector<HTMLTextAreaElement>("#sua-ket-luan-k1")!;
    typeInto(box, "Sửa lần hai.");
    press(box, "Enter");
    await settle();

    expect(page.querySelector('[role="alert"]')!.textContent).toBe("kết luận đã được tách — không sửa được");
    expect(page.querySelector<HTMLTextAreaElement>("#sua-ket-luan-k1")!.value).toBe("Sửa lần hai.");
  });
});

describe("Gỡ — hộp thoại có ô lý do bắt buộc", () => {
  it("Gỡ biên bản mở một <dialog>; lý do trống thì nút gỡ tắt; gõ lý do rồi gỡ là gửi đúng lý do", async () => {
    api.xoaBienBan.mockImplementation(() => ok(null));
    const page = await mount();
    act(() => buttonByText(page, "Gỡ biên bản").click());
    const dialog = page.querySelector("dialog")!;
    const submit = dialog.querySelector<HTMLButtonElement>('button[type="submit"]')!;
    expect(submit.disabled).toBe(true);
    typeInto(dialog.querySelector<HTMLTextAreaElement>("textarea")!, "Nhập trùng");
    act(() => submit.click());
    await settle();

    expect(api.xoaBienBan).toHaveBeenCalledWith("01JBB1", "Nhập trùng");
    expect(page.querySelector("dialog")).toBeNull();
  });
});
