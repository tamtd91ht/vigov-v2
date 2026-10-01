import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { ACCOUNT_ITEMS, Sidebar } from "@/components/sidebar";
import type { CommuneDetail } from "@/lib/api";
import { canCreateCommune, canManageCommune, canManageDomains, canManageMiniApps } from "@/lib/permissions";

import { ChangePasswordDone, CHANGE_PASSWORD_DONE } from "./account/change-password-form";
import { CommuneDetailBody, DEACTIVATE_WARNING, NAME_CORRECTION_NOTICE } from "./communes/commune-detail";
import { CommuneListToolbar, CommuneTable } from "./communes/commune-list";
import {
  CREATE_FORBIDDEN_NOTICE,
  CreateCommuneGate,
  EMPTY_COMMUNE_NOTICE,
  FIRST_ADMIN_NOTICE,
} from "./communes/create-commune-form";

const TENANT = "ops.tenant.manage";
const DOMAIN = "ops.domain.manage";
const MINI_APP = "ops.mini_app.manage";
const ALL = [TENANT, DOMAIN, MINI_APP];

const COMMUNE: CommuneDetail = {
  id: "01J0000000000000000000000A",
  name: "Xã Kiểm Thử",
  province: "Tỉnh Kiểm Thử",
  active: true,
  domains: ["chinh.example.vn", "phu.example.vn"],
  mini_apps: [
    { app_id: "4096", mode: "rieng", active: true, created_at: "2026-10-01T03:00:00Z", created_by: "VH-00001" },
  ],
};

const noop = () => {};

describe("permission hints — denied cases first", () => {
  it("Tạo xã needs BOTH ops.tenant.manage and ops.domain.manage", () => {
    expect(canCreateCommune([])).toBe(false);
    expect(canCreateCommune([TENANT])).toBe(false);
    expect(canCreateCommune([DOMAIN])).toBe(false);
    expect(canCreateCommune([TENANT, DOMAIN])).toBe(true);
  });

  it("the list hides Tạo xã without both keys, and shows it with both", () => {
    for (const keys of [[], [TENANT], [DOMAIN], [MINI_APP]]) {
      expect(renderToString(<CommuneListToolbar permissionKeys={keys} />)).not.toContain("Tạo xã");
    }
    const html = renderToString(<CommuneListToolbar permissionKeys={[TENANT, DOMAIN]} />);
    expect(html).toContain("Tạo xã");
    expect(html).toContain('href="/xa/moi"');
  });

  it("/xa/moi without the keys says so instead of showing a form", () => {
    const html = renderToString(<CreateCommuneGate ready permissionKeys={[TENANT]} />);
    expect(html).toContain(CREATE_FORBIDDEN_NOTICE);
    expect(html).not.toContain("<form");
  });

  it("detail without any key shows the record and NO write control", () => {
    const html = renderToString(
      <CommuneDetailBody commune={COMMUNE} permissionKeys={[]} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(html).toContain("chinh.example.vn");
    expect(html).toContain("4096");
    for (const control of ["Sửa lỗi gõ", "Ngừng hoạt động xã", "Thêm tên miền", "Đặt làm tên miền chính", "Gắn Mini App"]) {
      expect(html).not.toContain(control);
    }
  });

  it("each detail control follows its own key", () => {
    expect(canManageDomains([TENANT])).toBe(false);
    expect(canManageCommune([DOMAIN])).toBe(false);
    expect(canManageMiniApps([TENANT, DOMAIN])).toBe(false);

    const domainsOnly = renderToString(
      <CommuneDetailBody commune={COMMUNE} permissionKeys={[DOMAIN]} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(domainsOnly).toContain("Thêm tên miền");
    expect(domainsOnly).toContain("Đặt làm tên miền chính");
    expect(domainsOnly).not.toContain("Sửa lỗi gõ");
    expect(domainsOnly).not.toContain("Gắn Mini App");
  });

  it("with every key: all wave-1 controls, and still no remove / repoint domain", () => {
    const html = renderToString(
      <CommuneDetailBody commune={COMMUNE} permissionKeys={ALL} onChanged={noop} onMiniAppAttached={noop} />,
    );
    for (const control of ["Sửa lỗi gõ trong tên xã", "Ngừng hoạt động xã", "Thêm tên miền", "Đặt làm tên miền chính", "Gắn Mini App"]) {
      expect(html).toContain(control);
    }
    expect(html).not.toMatch(/Gỡ tên miền|Xoá tên miền|Xóa tên miền|Trỏ tên miền|Chuyển tên miền/);
  });

  it("an inactive commune offers re-activation, not deactivation", () => {
    const html = renderToString(
      <CommuneDetailBody commune={{ ...COMMUNE, active: false }} permissionKeys={ALL} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(html).toContain("Bật hoạt động trở lại");
    expect(html).not.toContain("Ngừng hoạt động xã");
    expect(html).toContain("Ngừng hoạt động"); // the status badge, in text
  });
});

describe("wording that IS the behaviour", () => {
  it("the create form says the commune starts empty and how the first admin signs in", () => {
    const html = renderToString(<CreateCommuneGate ready permissionKeys={[TENANT, DOMAIN]} />);
    expect(html).toContain(EMPTY_COMMUNE_NOTICE);
    expect(html).toContain(FIRST_ADMIN_NOTICE);
    expect(EMPTY_COMMUNE_NOTICE).toMatch(/vai trò.*cán bộ.*thời hạn xử lý.*giờ làm việc.*ngày nghỉ lễ/);
    expect(FIRST_ADMIN_NOTICE).toContain("“admin”");
    expect(FIRST_ADMIN_NOTICE).not.toMatch(/IDENTITY_|SEED/);
    expect(html).toContain('method="post"');
  });

  it("the name dialog is a typing correction, not a renaming of an administrative unit", () => {
    expect(NAME_CORRECTION_NOTICE).toContain("sửa lỗi gõ");
    expect(NAME_CORRECTION_NOTICE).toContain("không phải đổi tên đơn vị hành chính");
  });

  it("deactivating warns the sites stop within about 30 seconds and nothing is deleted", () => {
    expect(DEACTIVATE_WARNING).toContain("30 giây");
    expect(DEACTIVATE_WARNING).toContain("giữ nguyên");
  });

  it("after a password change the screen says every session ended", () => {
    expect(renderToString(<ChangePasswordDone />)).toContain(CHANGE_PASSWORD_DONE);
  });
});

describe("list and shell", () => {
  it("the table shows name, province, status in text, primary domain and the count of the others", () => {
    const html = renderToString(<CommuneTable items={[COMMUNE]} />);
    expect(html).toContain("Xã Kiểm Thử");
    expect(html).toContain("Tỉnh Kiểm Thử");
    expect(html).toContain("Đang hoạt động");
    expect(html).toContain("chinh.example.vn");
    expect(html).not.toContain("phu.example.vn");
    expect(html).toContain(`href="/xa/${COMMUNE.id}"`);
  });

  it("an empty registry says it is empty — only once it has loaded", () => {
    expect(renderToString(<CommuneTable items={[]} />)).toContain("Sổ xã chưa có xã nào.");
  });

  it("the sidebar carries the account menu and a sign-out button", () => {
    const html = renderToString(<Sidebar />);
    expect(html).toContain("Danh sách xã");
    expect(html).toContain('aria-current="page"');
    for (const item of ACCOUNT_ITEMS) expect(html).toContain(`href="${item.href}"`);
    expect(html).toContain("Đổi mật khẩu");
    expect(html).toContain("Tạo lại mã khôi phục");
    expect(html).toContain("Đăng xuất");
  });
});

describe("nothing credential-shaped is persisted by this app", () => {
  // Source scan: a storage API in the console would be one place a password, a TOTP secret or a
  // recovery code could outlive the page. Tests are excluded — they name the APIs to forbid them.
  function sources(dir: string): string[] {
    return readdirSync(dir).flatMap((name) => {
      const p = join(dir, name);
      if (statSync(p).isDirectory()) return sources(p);
      return /\.(ts|tsx)$/.test(name) && !/\.test\.tsx?$/.test(name) ? [p] : [];
    });
  }

  it("no localStorage, sessionStorage, indexedDB or document.cookie anywhere in src", () => {
    const offenders = sources(fileURLToPath(new URL("..", import.meta.url))).filter((f) =>
      /\b(localStorage|sessionStorage|indexedDB|document\.cookie)\b/.test(readFileSync(f, "utf8")),
    );
    expect(offenders).toEqual([]);
  });
});
