import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: () => "/xa" }));

import { Sidebar } from "@/components/sidebar";
import { listTenants, NotWiredError, signIn } from "@/lib/api";

import { SignInForm } from "./auth/sign-in-form";
import { TENANT_LIST_NOT_LOADED, TenantListEmpty } from "./tenants/tenant-list-empty";

describe("api stubs throw, never fake success", () => {
  it("signIn", async () => {
    await expect(
      signIn({ email: "a@example.gov.vn", password: "x", secondFactor: { kind: "totp", code: "000000" } }),
    ).rejects.toBeInstanceOf(NotWiredError);
  });

  it("listTenants", async () => {
    await expect(listTenants()).rejects.toBeInstanceOf(NotWiredError);
  });
});

describe("sign-in form", () => {
  const html = renderToString(<SignInForm />);

  it("has email, password and the TOTP field by default, with the recovery-code toggle", () => {
    expect(html).toContain("Thư điện tử");
    expect(html).toContain("Mật khẩu");
    expect(html).toContain("Mã xác thực");
    expect(html).toContain('autoComplete="one-time-code"');
    expect(html).toContain("Dùng mã khôi phục");
  });

  it("carries no demo wording", () => {
    expect(html.toLowerCase()).not.toMatch(/demo|trải nghiệm|dùng thử/);
  });
});

describe("console shell", () => {
  it("sidebar lists Danh sách xã and marks it current", () => {
    const html = renderToString(<Sidebar />);
    expect(html).toContain("Danh sách xã");
    expect(html).toContain('href="/xa"');
    expect(html).toContain('aria-current="page"');
  });

  it("the empty state says NOT LOADED, never that there are no communes", () => {
    const html = renderToString(<TenantListEmpty />);
    expect(html).toContain(TENANT_LIST_NOT_LOADED);
    expect(html).not.toMatch(/Chưa có xã nào|Không có xã/);
  });
});
