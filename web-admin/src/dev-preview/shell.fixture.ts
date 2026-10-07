import { flattenMenu, NHOM_MENU } from "@/components/muc-menu";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";
import { QUYEN_GHI_NGAN_SACH, QUYEN_QUAN_LY_DANH_MUC, QUYEN_XAC_NHAN_NGAN_SACH, QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

/**
 * FIXTURE — the dev-only screenshot preview's commune and session (`preview-gate.ts`). Nothing here is a
 * real commune, a real person or a real key: the commune is spec 01's own sample ("Xã Thăng Bình" /
 * "Thành phố Đà Nẵng"), the person "Cán bộ A" (rule 3: no real name in source).
 *
 * WHY A COMMUNE IS WRITTEN DOWN HERE AT ALL, when rule 1 says the commune comes from `Host` and nothing
 * else: this file feeds ONLY pages that `notFound()` in every production build and that call no API —
 * the preview has no commune to resolve and no data to isolate. A real page never imports it.
 */
export const PREVIEW_COMMUNE: CauHinhXaHienThi = {
  displayName: "Xã Thăng Bình",
  parentAuthority: "Thành phố Đà Nẵng",
  logoUrl: "",
  webAdminBannerUrl: "",
};

/** The keys the task card asked for: read, record and confirm the budget, and manage its catalogue. */
export const PREVIEW_PERMISSIONS: readonly string[] = [
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_GHI_NGAN_SACH,
  QUYEN_XAC_NHAN_NGAN_SACH,
  QUYEN_QUAN_LY_DANH_MUC,
];

/**
 * `?menu=day-du`: every key a menu item asks for, so the sidebar shows the prototype's full menu for a
 * side-by-side screenshot. Derived from `NHOM_MENU`, never listed by hand, so it follows the menu.
 */
export function fullMenuPermissions(): string[] {
  const keys = flattenMenu(NHOM_MENU).flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa]));
  return [...new Set([...PREVIEW_PERMISSIONS, ...keys])];
}

export function previewSession(permissions: readonly string[]): identity_phienHienTaiRa {
  return {
    sid: "xem-thu",
    expires_at: "2099-01-01T00:00:00Z",
    staff: { code: "CB-00001", full_name: "Cán bộ A", position: "Kế toán ngân sách" },
    role: { code: "ke-toan", name: "Kế toán", is_leader: false },
    permissions: [...permissions],
    must_change_password: false,
  };
}
