// @vitest-environment jsdom
//
// jsdom: the matrix (`ma-tran-phan-quyen.tsx`) reads the API in an effect and saves on a click — the
// loading state, what the page shows once data is in, and the toast on a save are behaviour, not
// markup. The markup of the table itself is pinned in `ma-tran-phan-quyen.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { identity_cotPhanQuyenRa, identity_maTranQuyenRa } from "@/lib/api/schema.gen";

const H = vi.hoisted(() => ({ read: vi.fn(), save: vi.fn() }));
vi.mock("@/lib/api/phan-quyen", () => ({ layMaTranQuyen: H.read, luuPhanQuyenVaiTro: H.save }));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

const { MaTranPhanQuyen } = await import("./ma-tran-phan-quyen");

// `feedback.classify` held by nobody but the default admin: the old screen printed an ADR 0055
// warning for exactly this shape, and the owner removed it from this page (08/10/2026).
const MATRIX: identity_maTranQuyenRa = {
  groups: [
    {
      name: "Phản ánh",
      permissions: [
        { code: "feedback.classify", label: "Phân loại phản ánh" },
        { code: "feedback.unmask", label: "Xem đầy đủ người gửi" },
      ],
    },
  ],
  roles: [
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
  T.success.mockReset();
  T.error.mockReset();
  H.read.mockReset();
  H.save.mockReset();
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

function button(el: HTMLElement, label: string): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
  expect(b, label).not.toBeNull();
  return b!;
}

describe("Phân quyền matrix — prototype behaviour (card B)", () => {
  it("P5: while the matrix loads — three 44px skeleton bars in a space-y-2 stack, nothing else", async () => {
    H.read.mockReturnValue(new Promise(() => {}));
    const el = await mount();
    const stack = el.querySelector(".space-y-2");
    expect(stack).not.toBeNull();
    expect(stack!.querySelectorAll(".h-11.w-full")).toHaveLength(3);
    expect(el.textContent).not.toContain("Bấm vào ô");
    expect(el.querySelector("table")).toBeNull();
  });

  it("P6: the hint, verbatim, with Lưu in bold — and no seed panel, no unheld-permission warning", async () => {
    const el = await mount();
    const hint = el.querySelector("p.text-ink-muted.text-\\[12px\\]");
    expect(hint?.innerHTML).toBe(
      "Bấm vào ô để bật hoặc tắt quyền, sau đó bấm <b>Lưu</b> ở đầu cột của vai trò đó.",
    );
    expect(el.textContent).not.toContain("Tạo tám vai trò mẫu");
    expect(el.textContent).not.toContain("Chưa vai trò nào");
    expect(el.querySelector("table")).not.toBeNull();
  });

  it("read-only: no hint at all (prototype prints it only when the account can edit)", async () => {
    const el = await mount(false);
    expect(el.textContent).not.toContain("Bấm vào ô");
    // The old visible read-only note is gone (the sr-only table caption still says "chỉ để xem").
    expect(el.textContent).not.toContain("Tài khoản của bạn không có quyền phân quyền");
    expect(el.querySelector("table")).not.toBeNull();
  });

  it("P27: a saved column raises the prototype's success toast with the role name", async () => {
    const el = await mount();
    H.save.mockResolvedValue({
      ok: true,
      duLieu: { role_id: "vt-1", permissions: ["feedback.classify"] } as identity_cotPhanQuyenRa,
    });
    act(() => button(el, "Chuyên viên — Phân loại phản ánh").click());
    act(() => button(el, "Lưu phân quyền của vai trò Chuyên viên").click());
    await settle();
    expect(H.save).toHaveBeenCalledWith("vt-1", { permissions: ["feedback.classify"] });
    expect(T.success).toHaveBeenCalledWith("Đã lưu quyền của vai trò Chuyên viên.");
    expect(el.querySelector('button[aria-label="Lưu phân quyền của vai trò Chuyên viên"]')).toBeNull();
  });

  it("P27: a refused save keeps the server's reason inline under the column, no success toast", async () => {
    const el = await mount();
    const reason = "Bạn không thể cấp một quyền mà chính bạn không giữ.";
    H.save.mockResolvedValue({ ok: false, thongBao: reason });
    act(() => button(el, "Chuyên viên — Phân loại phản ánh").click());
    act(() => button(el, "Lưu phân quyền của vai trò Chuyên viên").click());
    await settle();
    expect(T.success).not.toHaveBeenCalled();
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(reason);
  });

  it("columns read A→Z by name, the system administrator (by its code) LAST — user 09/10/2026", async () => {
    H.read.mockResolvedValue({
      ok: true,
      duLieu: {
        ...MATRIX,
        roles: [
          { id: "vt-0", code: "quan-tri-he-thong", name: "Quản trị hệ thống", is_leader: false, staff_count: 1, active_account_count: 1 },
          ...MATRIX.roles,
          { id: "vt-3", code: "ke-toan", name: "Kế toán", is_leader: false, staff_count: 1, active_account_count: 1 },
        ],
      },
    } satisfies KetQua<identity_maTranQuyenRa>);
    const el = await mount();
    const heads = [...el.querySelectorAll("thead th .text-navy.font-semibold")].map((d) => d.textContent);
    expect(heads).toEqual(["Chuyên viên", "Kế toán", "Phó Chủ tịch", "Quản trị hệ thống"]);
  });
});
