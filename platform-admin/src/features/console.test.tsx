import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { ACCOUNT_ITEMS, Sidebar, SidebarView } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";
import type { CommuneDetail, MiniApp } from "@/lib/api";
import { canCreateCommune, canManageCommune, canManageDomains, canManageMiniApps } from "@/lib/permissions";

import { ChangePasswordDone, CHANGE_PASSWORD_DONE } from "./account/change-password-form";
import { CommuneDetailBody, DEACTIVATE_WARNING, NAME_CORRECTION_NOTICE } from "./communes/commune-detail";
import { CommuneListToolbar, CommuneTable } from "./communes/commune-list";
import {
  MANUAL_STEPS,
  MiniAppOutcomeNotices,
  MiniAppSection,
  REMOVAL_IS_PERMANENT,
  RemoveWarning,
  SECRET_NOT_RETIRED,
  SecretStatusText,
  UNBOUND_SECRETS_TITLE,
  type MiniAppOutcome,
} from "./communes/mini-app-section";
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

const APP: MiniApp = {
  app_id: "4096",
  mode: "rieng",
  active: true,
  created_at: "2026-10-01T03:00:00Z",
  created_by: "VH-00001",
  secret: { status: "da_dat", set_at: "2026-10-02T03:30:00Z", set_by: "VH-00002" },
};

const COMMUNE: CommuneDetail = {
  id: "01J0000000000000000000000A",
  name: "Xã Kiểm Thử",
  province: "Tỉnh Kiểm Thử",
  active: true,
  domains: ["chinh.example.vn", "phu.example.vn"],
  mini_apps: [APP],
  unbound_secrets: [],
};

const noop = () => {};

/** After a removal the detail no longer lists the App ID (soft delete, ADR 0070 §Sửa đổi #2). */
const NO_RUNNING_APP: CommuneDetail = {
  ...COMMUNE,
  mini_apps: [],
};

/** A pre-amendment row the server might still send: turned off, not removed. Shown, never actionable. */
const INACTIVE_ROW: CommuneDetail = {
  ...COMMUNE,
  mini_apps: [{ ...APP, active: false }],
};

const WITH_UNBOUND: CommuneDetail = {
  ...COMMUNE,
  unbound_secrets: [{ app_id: "2048", secret: { status: "da_dat", set_at: "2026-09-30T03:00:00Z", set_by: "VH-00003" } }],
};

const REPLACED_NOT_RETIRED: MiniAppOutcome = {
  kind: "replace",
  appId: "4096",
  newAppId: "8192",
  newSecretSet: false,
  secretRetired: false,
  retirementError: "identity_unavailable",
  reason: "Zalo cấp App ID mới",
};

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
      <CommuneDetailBody commune={NO_RUNNING_APP} permissionKeys={ALL} onChanged={noop} onMiniAppAttached={noop} />,
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

describe("Mini App change / remove / secret (ADR 0070, §Sửa đổi 05/10/2026)", () => {
  const MINI_CONTROLS = ["Đổi App ID", "Gỡ khỏi xã", "Đặt/đổi khoá bí mật", "Thu hồi khoá"];

  it("without ops.mini_app.manage: no Mini App control at all", () => {
    for (const keys of [[], [TENANT, DOMAIN]]) {
      for (const commune of [COMMUNE, NO_RUNNING_APP, INACTIVE_ROW, WITH_UNBOUND]) {
        const html = renderToString(
          <CommuneDetailBody commune={commune} permissionKeys={keys} onChanged={noop} onMiniAppAttached={noop} />,
        );
        for (const c of [...MINI_CONTROLS, "Gắn Mini App"]) expect(html).not.toContain(c);
      }
    }
  });

  it("a running own app offers change, secret and remove — and hides the attach form", () => {
    const html = renderToString(<MiniAppSection commune={COMMUNE} allowed onChanged={noop} onMiniAppAttached={noop} />);
    for (const c of ["Đổi App ID", "Gỡ khỏi xã", "Đặt/đổi khoá bí mật"]) expect(html).toContain(c);
    expect(html).not.toContain("Gắn Mini App");
  });

  it("there is no reactivation anywhere: no Bật lại, whatever rows the server sends", () => {
    for (const commune of [COMMUNE, NO_RUNNING_APP, INACTIVE_ROW, WITH_UNBOUND]) {
      const html = renderToString(<MiniAppSection commune={commune} allowed onChanged={noop} onMiniAppAttached={noop} />);
      expect(html).not.toContain("Bật lại");
    }
  });

  it("a row that is not running is shown without any control", () => {
    const html = renderToString(<MiniAppSection commune={INACTIVE_ROW} allowed onChanged={noop} onMiniAppAttached={noop} />);
    expect(html).toContain("4096");
    for (const c of ["Đổi App ID", "Gỡ khỏi xã", "Đặt/đổi khoá bí mật"]) expect(html).not.toContain(c);
  });

  it("no App ID listed: the empty state and the attach form", () => {
    const html = renderToString(<MiniAppSection commune={NO_RUNNING_APP} allowed onChanged={noop} onMiniAppAttached={noop} />);
    expect(html).toContain("Xã chưa gắn Mini App riêng nào.");
    expect(html).toContain("Gắn Mini App");
  });

  it("the removal dialog says the App ID is removed permanently and cannot be re-attached", () => {
    const html = renderToString(<RemoveWarning appId="4096" />);
    expect(html).toContain(REMOVAL_IS_PERMANENT);
    expect(html).toContain("không gắn lại được nữa, kể cả cho chính xã này");
    expect(html).not.toMatch(/bật lại|Bật lại/);
  });

  it("the remove outcome says permanent, never 'tắt, không xoá'", () => {
    const html = renderToString(
      <MiniAppOutcomeNotices communeId={COMMUNE.id} outcome={{ kind: "remove", appId: "4096", secretRetired: true, reason: "x" }} onUpdate={noop} />,
    );
    expect(html).toContain("vĩnh viễn");
    expect(html).not.toContain("không xoá");
  });

  it("the shared app carries no control", () => {
    const shared = { ...COMMUNE, mini_apps: [{ ...APP, mode: "chung" }] };
    const html = renderToString(<MiniAppSection commune={shared} allowed onChanged={noop} onMiniAppAttached={noop} />);
    for (const c of MINI_CONTROLS) expect(html).not.toContain(c);
  });

  it("secret_retired false shows the not-retired notice with Thu hồi lại, plus the manual steps", () => {
    const html = renderToString(<MiniAppOutcomeNotices communeId={COMMUNE.id} outcome={REPLACED_NOT_RETIRED} onUpdate={noop} />);
    expect(html).toContain(SECRET_NOT_RETIRED);
    expect(html).toContain("Thu hồi lại");
    expect(html).toContain("ZALO_MINIAPP_COMMUNE_APP_SECRETS");
    for (const s of MANUAL_STEPS) expect(html).toContain(s);
    expect(html).toContain("Đặt khoá bí mật cho App ID 8192");
  });

  it("a retired secret shows no retry", () => {
    const html = renderToString(
      <MiniAppOutcomeNotices
        communeId={COMMUNE.id}
        outcome={{ ...REPLACED_NOT_RETIRED, secretRetired: true, newSecretSet: true }}
        onUpdate={noop}
      />,
    );
    expect(html).not.toContain(SECRET_NOT_RETIRED);
    expect(html).not.toContain("Thu hồi lại");
    expect(html).not.toContain("Đặt khoá bí mật cho App ID 8192");
  });
});

describe("the secret is write-only: status in words, never a value (ADR 0070 §Sửa đổi #3)", () => {
  it("da_dat reads 'Đã đặt lúc … bởi …' in the business time zone", () => {
    const html = renderToString(<SecretStatusText secret={{ status: "da_dat", set_at: "2026-10-02T03:30:00Z", set_by: "VH-00002" }} />);
    expect(html).toContain("Đã đặt lúc ");
    expect(html).toContain("bởi VH-00002");
    expect(html).toContain("10:30"); // 03:30Z = 10:30 +07:00
  });

  it("chua_dat warns that citizens cannot sign in", () => {
    const html = renderToString(<SecretStatusText secret={{ status: "chua_dat" }} />);
    expect(html).toContain("Chưa đặt");
    expect(html).toContain("Dân chưa đăng nhập được qua app này.");
  });

  it("khong_ro says the auth system did not answer — never 'Chưa đặt'", () => {
    const html = renderToString(<SecretStatusText secret={{ status: "khong_ro" }} />);
    expect(html).toContain("Không rõ (hệ thống xác thực chưa trả lời)");
    expect(html).not.toContain("Chưa đặt");
  });

  it("an attach answer (no status) is not guessed", () => {
    const html = renderToString(<SecretStatusText />);
    expect(html).not.toMatch(/Chưa đặt|Đã đặt/);
  });

  it("the table has a Khoá bí mật column, and no row shows anything secret-shaped", () => {
    const html = renderToString(<MiniAppSection commune={COMMUNE} allowed onChanged={noop} onMiniAppAttached={noop} />);
    expect(html).toContain("Khoá bí mật");
    expect(html).toContain("bởi VH-00002");
    expect(html).not.toMatch(/version|fingerprint/i);
  });
});

describe("unbound secrets of removed App IDs", () => {
  it("listed with Thu hồi khoá when allowed", () => {
    const html = renderToString(<MiniAppSection commune={WITH_UNBOUND} allowed onChanged={noop} onMiniAppAttached={noop} />);
    expect(html).toContain(UNBOUND_SECRETS_TITLE);
    expect(html).toContain("2048");
    expect(html).toContain("Thu hồi khoá");
  });

  it("denied: listed, but no Thu hồi khoá", () => {
    const html = renderToString(
      <MiniAppSection commune={WITH_UNBOUND} allowed={false} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(html).toContain(UNBOUND_SECRETS_TITLE);
    expect(html).not.toContain("Thu hồi khoá");
  });

  it("[] shows nothing; null says it is unknown", () => {
    const none = renderToString(<MiniAppSection commune={COMMUNE} allowed onChanged={noop} onMiniAppAttached={noop} />);
    expect(none).not.toContain(UNBOUND_SECRETS_TITLE);
    expect(none).not.toContain("Chưa biết còn khoá");
    const unknown = renderToString(
      <MiniAppSection commune={{ ...COMMUNE, unbound_secrets: null }} allowed onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(unknown).not.toContain(UNBOUND_SECRETS_TITLE);
    expect(unknown).toContain("Chưa biết còn khoá bí mật nào của App ID đã gỡ");
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

  // The sign-out button moved from the sidebar to the topbar in the 02/10/2026 redesign (ADR 0068
  // look): where it is drawn is presentation; that the signed-in shell carries it is the behaviour.
  it("the sidebar carries the account menu, and the topbar the operator code and sign-out", () => {
    const html = renderToString(<SidebarView permissionKeys={[MINI_APP]} />);
    expect(html).toContain("Danh sách xã");
    expect(html).toContain('aria-current="page"');
    for (const item of ACCOUNT_ITEMS) expect(html).toContain(`href="${item.href}"`);
    expect(html).toContain("Đổi mật khẩu");
    expect(html).toContain("Tạo lại mã khôi phục");
    const top = renderToString(<Topbar />);
    expect(top).toContain("Người vận hành");
    expect(top).toContain("Đăng xuất");
  });
});

describe("the CSP holds: no screen draws a style attribute", () => {
  // The production document policy has `style-src 'self' 'nonce-…'` and no 'unsafe-inline'
  // (`lib/csp.ts`): a `style="…"` attribute in the server HTML is refused by the browser, silently,
  // and the element renders unstyled. The redesign draws with classes only — this pins it for
  // every component that renders to a string here (lucide icons included).
  it("shell, list, create form, detail and account screens render without style=", () => {
    const screens = [
      <Sidebar key="s" />,
      <Topbar key="t" />,
      <CommuneListToolbar key="lt" permissionKeys={ALL} />,
      <CommuneTable key="ct" items={[COMMUNE]} />,
      <CommuneTable key="ce" items={[]} />,
      <CreateCommuneGate key="cg" ready permissionKeys={[TENANT, DOMAIN]} />,
      <CreateCommuneGate key="cf" ready permissionKeys={[]} />,
      <CommuneDetailBody key="d" commune={COMMUNE} permissionKeys={ALL} onChanged={noop} onMiniAppAttached={noop} />,
      <ChangePasswordDone key="pd" />,
      <MiniAppSection key="ms" commune={COMMUNE} allowed onChanged={noop} onMiniAppAttached={noop} />,
      <MiniAppSection key="mi" commune={NO_RUNNING_APP} allowed onChanged={noop} onMiniAppAttached={noop} />,
      <MiniAppSection key="mu" commune={WITH_UNBOUND} allowed onChanged={noop} onMiniAppAttached={noop} />,
      <RemoveWarning key="rw" appId="4096" />,
      <MiniAppOutcomeNotices key="mo" communeId={COMMUNE.id} outcome={REPLACED_NOT_RETIRED} onUpdate={noop} />,
    ];
    for (const s of screens) expect(renderToString(s)).not.toContain("style=");
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
