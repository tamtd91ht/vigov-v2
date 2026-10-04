import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

/**
 * The topbar's "Đổi mật khẩu" link is the only voluntary way into `/doi-mat-khau` — without it the
 * page opens only when the server forces a change. It belongs to the signed-in person, so it is
 * drawn only once the session is read, and it carries no identity of its own.
 */
const H = vi.hoisted(() => ({ phien: null as PhienDaDoc }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
// The bell needs the App Router context; it is not what this file tests.
vi.mock("./notification-bell", () => ({ NotificationBell: () => null }));

const { CauHinhXaProvider } = await import("./cau-hinh-xa");
const { DauTrang } = await import("./dau-trang");

const READ_SESSION = {
  ok: true,
  duLieu: { staff: { full_name: "Nguyễn Văn Hùng", position: "Chuyên viên" }, role: null, permissions: [] },
} as unknown as PhienDaDoc;

function render(phien: PhienDaDoc) {
  H.phien = phien;
  return renderToStaticMarkup(
    <CauHinhXaProvider giaTri={{ displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" }}>
      <DauTrang />
    </CauHinhXaProvider>,
  );
}

describe("DauTrang — change-password link", () => {
  it("with a read session: links to /doi-mat-khau, with no identity in the URL", () => {
    const html = render(READ_SESSION);
    expect(html).toMatch(/<a [^>]*href="\/doi-mat-khau"[^>]*>/);
    expect(html).toContain("Đổi mật khẩu");
    expect(html).not.toMatch(/href="\/doi-mat-khau[?#/]/);
  });

  it("before the session is read, or when it cannot be read: no link", () => {
    expect(render(null)).not.toContain('href="/doi-mat-khau"');
    expect(render({ ok: false } as unknown as PhienDaDoc)).not.toContain('href="/doi-mat-khau"');
  });
});
