import type { ReactElement, ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

/**
 * `/nguoi-dung` and `/nguoi-dung/phan-quyen` — the two screens that were tabs of `/cau-hinh` until
 * 05/10/2026. What is pinned here is that the move kept the gate: each page renders the tab's own
 * component, so an account without the key gets the same refusal the tab gave, and the staff list /
 * matrix is never mounted (so it never calls a route that would only 403).
 *
 * The two heavy screens are stubs: their own tests cover them. Only the gate around them is under test.
 */

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
  PhienProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/cau-hinh-xa", () => ({
  CauHinhXaProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/lib/cau-hinh-xa-hien-thi", () => ({ phanHienThi: () => ({}) }));
vi.mock("@/lib/tenant.server", () => ({
  layCauHinhXa: async () => ({}),
  communePageMetadata: async (screen: string) => ({ title: screen }),
}));
// The header carries the navigation since the text sidebar was replaced (ADR 0068 §Sửa đổi 05/10/2026 lần 2).
vi.mock("@/components/dau-trang", () => ({ DauTrang: () => <header>HEADER</header> }));
vi.mock("@/features/cau-hinh/danh-ba-can-bo", () => ({
  DanhBaCanBo: ({ active }: { active?: boolean }) => <section>STAFF-ACCOUNTS active={String(active)}</section>,
}));
vi.mock("@/features/cau-hinh/ma-tran-phan-quyen", () => ({
  MaTranPhanQuyen: ({ choSua }: { choSua: boolean }) => <section>ROLE-MATRIX edit={String(choSua)}</section>,
}));

const UsersPage = (await import("./page")).default;
const RolePermissionsPage = (await import("./phan-quyen/page")).default;

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: [...permissions],
    },
  } as PhienDaDoc;
}

async function render(page: () => Promise<ReactElement>): Promise<string> {
  return renderToStaticMarkup(await page());
}

afterEach(() => {
  fakeSession = null;
});

describe("/nguoi-dung", () => {
  it("`admin.user` → the staff-account screen, mounted active (catalogues read on arrival, ND-01/02)", async () => {
    fakeSession = sessionWith(["admin.user"]);
    const html = await render(UsersPage);
    expect(html).toMatch(/<h1[^>]*>Người dùng<\/h1>/);
    expect(html).toContain("STAFF-ACCOUNTS active=true");
    expect(html).toContain("HEADER");
    // The prototype's header action (ADR 0068 lần 5) — the import opens a dialog of the list.
    expect(html).toMatch(/<button[^>]*>.*Nhập từ Excel<\/button>/);
  });

  it("DENIED: no `admin.user` (even with `admin.role`, `admin.user.delete`) → refusal, list never mounted", async () => {
    fakeSession = sessionWith(["admin.role", "admin.user.delete", "admin.users", "ADMIN.USER"]);
    const html = await render(UsersPage);
    expect(html).not.toContain("STAFF-ACCOUNTS");
    expect(html).toContain("không có quyền quản lý người dùng");
    // The header stays (the page still says where you are), but without its write action.
    expect(html).toMatch(/<h1[^>]*>Người dùng<\/h1>/);
    expect(html).not.toContain("Nhập từ Excel");
  });

  it("session not read yet → neither the list nor a refusal", async () => {
    fakeSession = null;
    const html = await render(UsersPage);
    expect(html).not.toContain("STAFF-ACCOUNTS");
    expect(html).not.toContain("không có quyền");
    expect(html).toContain("Đang kiểm tra quyền truy cập");
    expect(html).not.toContain("Nhập từ Excel");
  });

  it("session unreadable → the server's sentence, list never mounted (fail closed)", async () => {
    fakeSession = { ok: false, thongBao: "Phiên làm việc đã hết hạn" } as PhienDaDoc;
    const html = await render(UsersPage);
    expect(html).not.toContain("STAFF-ACCOUNTS");
    expect(html).toContain("Phiên làm việc đã hết hạn");
  });
});

describe("/nguoi-dung/phan-quyen", () => {
  it("`admin.role` → the matrix, editable", async () => {
    fakeSession = sessionWith(["admin.role"]);
    const html = await render(RolePermissionsPage);
    expect(html).toMatch(/<h1[^>]*>Phân quyền<\/h1>/);
    expect(html).toContain("ROLE-MATRIX edit=true");
  });

  it("P2: header is the prototype's — mb-5, h1 navy 22px bold, subtitle VERBATIM (owner 08/10/2026)", async () => {
    fakeSession = sessionWith(["admin.role"]);
    const html = await render(RolePermissionsPage);
    expect(html).toMatch(/<header class="[^"]*mb-5[^"]*">/);
    expect(html).toMatch(/<h1 class="[^"]*text-\[22px\][^"]*font-bold[^"]*text-navy[^"]*">Phân quyền<\/h1>/);
    expect(html).toContain(
      ">Mỗi vai trò làm được những gì. Đổi ở đây là đổi cho mọi cán bộ đang giữ vai trò đó, ngay lần đăng nhập sau của họ.<",
    );
    // No extra column gap between header and body: the header's own 20px is the only spacing.
    expect(html).toMatch(/<\/header><div class="min-w-0">/);
  });

  it("P4: session not read yet → the three-bar skeleton, neither matrix nor refusal", async () => {
    fakeSession = null;
    const html = await render(RolePermissionsPage);
    expect(html).not.toContain("ROLE-MATRIX");
    expect(html).not.toContain("không có quyền");
    expect(html).toMatch(/<div class="space-y-2">(<span[^>]*class="[^"]*h-11 w-full[^"]*"><\/span>){3}<\/div>/);
  });

  it("P3 DENIED: no `admin.role` (even with `admin.user`, look-alikes) → the prototype's paragraph, matrix never mounted", async () => {
    fakeSession = sessionWith(["admin.user", "admin.roles", "ADMIN.ROLE", "admin.audit"]);
    const html = await render(RolePermissionsPage);
    expect(html).not.toContain("ROLE-MATRIX");
    expect(html).toContain(
      '<p class="border-line text-ink-muted rounded-card border bg-white p-6 text-[13px]">Tài khoản của bạn không có quyền quản lý phân quyền. Liên hệ Chánh Văn phòng hoặc quản trị viên của đơn vị nếu cần.</p>',
    );
  });

  it("session unreadable → the server's sentence, matrix never mounted (fail closed)", async () => {
    fakeSession = { ok: false, thongBao: "Phiên làm việc đã hết hạn" } as PhienDaDoc;
    const html = await render(RolePermissionsPage);
    expect(html).not.toContain("ROLE-MATRIX");
    expect(html).toContain("Phiên làm việc đã hết hạn");
  });
});
