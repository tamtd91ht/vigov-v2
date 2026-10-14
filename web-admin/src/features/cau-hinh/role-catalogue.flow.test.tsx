// @vitest-environment jsdom
//
// The role-list actions on the Phân quyền screen (owner decision 15/10/2026): who sees them, which
// column offers them, what each dialog sends, that a success re-reads the matrix, and that a refusal
// stays in the dialog as the server's sentence. Same harness as `role-permission-matrix.flow.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { identity_maTranQuyenRa, identity_seedRoleTemplatesOut } from "@/lib/api/schema.gen";

const H = vi.hoisted(() => ({
  read: vi.fn(),
  save: vi.fn(),
  seed: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("@/lib/api/phan-quyen", () => ({ layMaTranQuyen: H.read, luuPhanQuyenVaiTro: H.save }));
vi.mock("@/lib/api/role-templates", () => ({ seedRoleTemplates: H.seed }));
vi.mock("@/lib/api/roles", () => ({ createRole: H.create, updateRole: H.update, deleteRole: H.remove }));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

const { MaTranPhanQuyen } = await import("./ma-tran-phan-quyen");
const { ROLE_TEMPLATE_NAMES, roleNameProblem, seedResultMessage } = await import("./role-catalogue");

const MATRIX: identity_maTranQuyenRa = {
  groups: [{ name: "Phản ánh", permissions: [{ code: "feedback.classify", label: "Phân loại phản ánh" }] }],
  roles: [
    { id: "vt-0", code: "quan-tri-he-thong", name: "Quản trị hệ thống", is_leader: false, staff_count: 1, active_account_count: 1 },
    { id: "vt-1", code: "chuyen-vien", name: "Chuyên viên", is_leader: false, staff_count: 3, active_account_count: 2 },
    { id: "vt-2", code: "pho-chu-tich", name: "Phó Chủ tịch", is_leader: true, staff_count: 1, active_account_count: 1 },
  ],
  grants: [],
};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  for (const f of Object.values(H)) f.mockReset();
  T.success.mockReset();
  H.read.mockResolvedValue({ ok: true, duLieu: MATRIX } satisfies KetQua<identity_maTranQuyenRa>);
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

async function settle() {
  for (let i = 0; i < 6; i++) await act(async () => {});
}

async function mount(canEdit = true): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<MaTranPhanQuyen choSua={canEdit} maVaiTroCuaToi={null} />));
  await settle();
  return host;
}

function byText(el: HTMLElement, text: string): HTMLButtonElement | null {
  return [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text) ?? null;
}

function byLabel(el: HTMLElement, label: string): HTMLButtonElement | null {
  return el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

function dialog(): HTMLDialogElement {
  const d = document.querySelector("dialog");
  expect(d).not.toBeNull();
  return d!;
}

/** Types into a React-controlled field: the native setter, then the event React listens to. */
function type(field: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(field, value);
  field.dispatchEvent(new Event("input", { bubbles: true }));
}

async function submit(d: HTMLDialogElement) {
  act(() => d.querySelector("form")!.requestSubmit());
  await settle();
}

describe("role-list actions — who sees them", () => {
  it("an account that can edit sees Thêm vai trò and Thêm vai trò mẫu", async () => {
    const el = await mount(true);
    expect(byText(el, "Thêm vai trò")).not.toBeNull();
    expect(byText(el, "Thêm vai trò mẫu")).not.toBeNull();
  });

  it("DENIED: a read-only account sees no add button and no column action", async () => {
    const el = await mount(false);
    expect(byText(el, "Thêm vai trò")).toBeNull();
    expect(byText(el, "Thêm vai trò mẫu")).toBeNull();
    expect(el.querySelector('button[aria-label^="Đổi tên vai trò"]')).toBeNull();
    expect(el.querySelector('button[aria-label^="Xoá vai trò"]')).toBeNull();
  });

  it("the system-administrator column has neither Đổi tên nor Xoá; the others have both", async () => {
    const el = await mount();
    expect(byLabel(el, "Đổi tên vai trò Quản trị hệ thống")).toBeNull();
    expect(byLabel(el, "Xoá vai trò Quản trị hệ thống")).toBeNull();
    expect(byLabel(el, "Đổi tên vai trò Chuyên viên")).not.toBeNull();
    expect(byLabel(el, "Xoá vai trò Phó Chủ tịch")).not.toBeNull();
  });

  it("an empty commune (no role yet) still offers both add buttons", async () => {
    H.read.mockResolvedValue({ ok: true, duLieu: { ...MATRIX, roles: [] } });
    const el = await mount();
    expect(byText(el, "Thêm vai trò mẫu")).not.toBeNull();
    expect(byText(el, "Thêm vai trò")).not.toBeNull();
  });

  it("unsaved ticks disable every role action, so a reload cannot drop them", async () => {
    const el = await mount();
    act(() => byLabel(el, "Chuyên viên — Phân loại phản ánh")!.click());
    expect(byText(el, "Thêm vai trò")!.disabled).toBe(true);
    expect(byText(el, "Thêm vai trò mẫu")!.disabled).toBe(true);
    expect(byLabel(el, "Xoá vai trò Phó Chủ tịch")!.disabled).toBe(true);
  });
});

describe("Thêm vai trò mẫu", () => {
  it("lists the eight template names, sends the seed, counts the answer, and re-reads the matrix", async () => {
    const out: identity_seedRoleTemplatesOut = {
      created: [{ code: "ke-toan", name: "Kế toán" }],
      skipped_existing: [{ code: "chuyen-vien", name: "Chuyên viên chuyên môn" }],
      skipped_deleted: [],
    };
    H.seed.mockResolvedValue({ ok: true, duLieu: out });
    const el = await mount();
    act(() => byText(el, "Thêm vai trò mẫu")!.click());
    const d = dialog();
    for (const n of ROLE_TEMPLATE_NAMES) expect(d.textContent).toContain(n);
    expect(ROLE_TEMPLATE_NAMES).toHaveLength(8);
    expect(d.textContent).toContain("không được khôi phục");
    await submit(d);
    expect(H.seed).toHaveBeenCalledTimes(1);
    expect(T.success).toHaveBeenCalledWith("Đã thêm 1 vai trò mẫu. Giữ nguyên 1 vai trò đã có.");
    expect(H.read).toHaveBeenCalledTimes(2);
    expect(document.querySelector("dialog")).toBeNull();
  });

  it("403 permission_escalation: the server's sentence stays in the dialog, no reload", async () => {
    const sentence = "Tài khoản của bạn còn thiếu: budget.confirm.";
    H.seed.mockResolvedValue({ ok: false, thongBao: sentence });
    const el = await mount();
    act(() => byText(el, "Thêm vai trò mẫu")!.click());
    await submit(dialog());
    expect(dialog().querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(H.read).toHaveBeenCalledTimes(1);
    expect(T.success).not.toHaveBeenCalled();
  });
});

describe("Thêm vai trò", () => {
  it("sends the trimmed name and the leader flag, then re-reads the matrix", async () => {
    H.create.mockResolvedValue({ ok: true, duLieu: { id: "vt-9", code: "ke-toan", name: "Kế toán", is_leader: true } });
    const el = await mount();
    act(() => byText(el, "Thêm vai trò")!.click());
    const d = dialog();
    expect(d.textContent).toContain("Tích quyền cho vai trò ngay trên bảng sau khi thêm");
    act(() => type(d.querySelector<HTMLInputElement>('input[name="name"]')!, "  Kế toán "));
    act(() => d.querySelector<HTMLInputElement>('input[name="is_leader"]')!.click());
    await submit(d);
    expect(H.create).toHaveBeenCalledWith({ name: "Kế toán", is_leader: true });
    expect(T.success).toHaveBeenCalledWith("Đã thêm vai trò Kế toán.");
    expect(H.read).toHaveBeenCalledTimes(2);
  });

  it("an empty name is stopped under the field, nothing sent", async () => {
    const el = await mount();
    act(() => byText(el, "Thêm vai trò")!.click());
    await submit(dialog());
    expect(H.create).not.toHaveBeenCalled();
    const input = dialog().querySelector<HTMLInputElement>('input[name="name"]')!;
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(document.getElementById(input.getAttribute("aria-describedby")!)?.textContent).toBe("Nhập tên vai trò.");
  });

  it("409 duplicate_name lands under Tên vai trò, verbatim", async () => {
    const sentence = "Đơn vị đã có vai trò mang tên này.";
    H.create.mockResolvedValue({ ok: false, thongBao: sentence, code: "duplicate_name" });
    const el = await mount();
    act(() => byText(el, "Thêm vai trò")!.click());
    act(() => type(dialog().querySelector<HTMLInputElement>('input[name="name"]')!, "Chuyên viên"));
    await submit(dialog());
    const input = dialog().querySelector<HTMLInputElement>('input[name="name"]')!;
    expect(document.getElementById(input.getAttribute("aria-describedby")!)?.textContent).toBe(sentence);
    expect(H.read).toHaveBeenCalledTimes(1);
  });
});

describe("Đổi tên / Xoá vai trò", () => {
  it("Đổi tên opens prefilled and PUTs the role's id", async () => {
    H.update.mockResolvedValue({ ok: true, duLieu: { id: "vt-2", code: "pho-chu-tich", name: "Phó Chủ tịch UBND", is_leader: true } });
    const el = await mount();
    act(() => byLabel(el, "Đổi tên vai trò Phó Chủ tịch")!.click());
    const d = dialog();
    const input = d.querySelector<HTMLInputElement>('input[name="name"]')!;
    expect(input.value).toBe("Phó Chủ tịch");
    expect(d.querySelector<HTMLInputElement>('input[name="is_leader"]')!.checked).toBe(true);
    act(() => type(input, "Phó Chủ tịch UBND"));
    await submit(d);
    expect(H.update).toHaveBeenCalledWith("vt-2", { name: "Phó Chủ tịch UBND", is_leader: true });
    expect(H.read).toHaveBeenCalledTimes(2);
  });

  it("Xoá requires a reason, then sends it; success re-reads the matrix", async () => {
    H.remove.mockResolvedValue({ ok: true, duLieu: null });
    const el = await mount();
    act(() => byLabel(el, "Xoá vai trò Chuyên viên")!.click());
    await submit(dialog());
    expect(H.remove).not.toHaveBeenCalled();
    expect(dialog().textContent).toContain("Nhập lý do xoá vai trò.");
    act(() => type(dialog().querySelector<HTMLTextAreaElement>('textarea[name="reason"]')!, "Gộp chức danh"));
    await submit(dialog());
    expect(H.remove).toHaveBeenCalledWith("vt-1", "Gộp chức danh");
    expect(T.success).toHaveBeenCalledWith("Đã xoá vai trò Chuyên viên.");
    expect(H.read).toHaveBeenCalledTimes(2);
  });

  it("409 role_in_use: the server's sentence, verbatim, at the foot of the form — not under the reason", async () => {
    const sentence = "Còn 3 cán bộ đang giữ vai trò này. Chuyển họ sang vai trò khác trước khi xoá.";
    H.remove.mockResolvedValue({ ok: false, thongBao: sentence, code: "role_in_use" });
    const el = await mount();
    act(() => byLabel(el, "Xoá vai trò Chuyên viên")!.click());
    const area = dialog().querySelector<HTMLTextAreaElement>('textarea[name="reason"]')!;
    act(() => type(area, "Gộp chức danh"));
    await submit(dialog());
    const alerts = [...dialog().querySelectorAll('[role="alert"]')].map((a) => a.textContent);
    expect(alerts).toEqual([sentence]);
    expect(area.getAttribute("aria-invalid")).toBe("false");
    expect(H.read).toHaveBeenCalledTimes(1);
  });

  it("422 invalid_reason lands under Lý do xoá", async () => {
    const sentence = "Lý do xoá không hợp lệ.";
    H.remove.mockResolvedValue({ ok: false, thongBao: sentence, code: "invalid_reason" });
    const el = await mount();
    act(() => byLabel(el, "Xoá vai trò Chuyên viên")!.click());
    const area = dialog().querySelector<HTMLTextAreaElement>('textarea[name="reason"]')!;
    act(() => type(area, "x"));
    await submit(dialog());
    expect(area.getAttribute("aria-invalid")).toBe("true");
    expect(document.getElementById(area.getAttribute("aria-describedby")!)?.textContent).toBe(sentence);
  });
});

describe("role-catalogue words", () => {
  it("name check: empty, 100 characters accepted, 101 refused", () => {
    expect(roleNameProblem("   ")).toBe("Nhập tên vai trò.");
    expect(roleNameProblem("a".repeat(100))).toBe("");
    expect(roleNameProblem("a".repeat(101))).toBe("Tên vai trò dài tối đa 100 ký tự.");
  });

  it("seed result counts what the server answered", () => {
    expect(
      seedResultMessage({ created: [], skipped_existing: [], skipped_deleted: [{ code: "ke-toan", name: "Kế toán" }] }),
    ).toBe("Đã thêm 0 vai trò mẫu. Không khôi phục 1 vai trò đã xoá.");
  });
});
