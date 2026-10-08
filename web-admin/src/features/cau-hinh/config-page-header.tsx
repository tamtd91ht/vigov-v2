/**
 * The words under the title — the prototype's own (`ConfigWorkspace.tsx`, ADR 0068 lần 5).
 */
export const CONFIG_SUBTITLE =
  "Sơ đồ tổ chức, địa bàn dân cư, danh mục nghiệp vụ và thời hạn xử lý của đơn vị. Tài khoản và " +
  "phân quyền nằm ở menu Người dùng.";

/**
 * Header of `/cau-hinh`, as spec `02-khung-trang.md` writes it: the h1 and one line of subtitle in an
 * `mb-6` block — 24px to the tab strip where other pages leave 20px — with NO icon and NO button on
 * the right. Drawn here rather than through `PageHeader`, whose `mb-3` and `leading-tight` are right
 * for every other page and are not changed for this one.
 *
 * `m-0` / `mb-0`: Tailwind's preflight is off in this app, so an `<h1>` and a `<p>` still carry the
 * browser's margins, which the prototype's reset removes.
 *
 * Shared with the dev preview (`app/(dev)/xem-thu/cau-hinh/page.tsx`) so the screenshot shows this
 * exact header and the subtitle is written once.
 */
export function ConfigPageHeader() {
  return (
    <div className="mb-6">
      <h1 className="text-navy m-0 text-[22px] font-bold">Cấu hình hệ thống</h1>
      <p className="text-ink-muted mt-1 mb-0 text-[13px]">{CONFIG_SUBTITLE}</p>
    </div>
  );
}
