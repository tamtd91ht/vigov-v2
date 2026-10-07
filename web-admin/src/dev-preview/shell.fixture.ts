import { flattenMenu, NHOM_MENU } from "@/components/muc-menu";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_GHI_NGAN_SACH,
  QUYEN_QUAN_LY_DANH_MUC,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XAC_NHAN_NGAN_SACH,
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  TASK_ASSIGN_PERMISSION,
} from "@/lib/quyen";

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
export function fullMenuPermissions(base: readonly string[] = PREVIEW_PERMISSIONS): string[] {
  const keys = flattenMenu(NHOM_MENU).flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa]));
  return [...new Set([...base, ...keys])];
}

/**
 * The Nhiệm vụ preview's keys — every `task.*` key the screen gates a control on (`quyenNhiemVu`) plus
 * `task.read` for the menu item. Real `quyen` keys, through their constants (rule 5, invariant 3c).
 */
export const PREVIEW_TASK_PERMISSIONS: readonly string[] = [
  QUYEN_XEM_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  TASK_ASSIGN_PERMISSION,
];

/** Who the session is. The accountant opens Giải ngân; the leader opens Nhiệm vụ. */
export type PreviewPerson = "ke-toan" | "lanh-dao";

const PEOPLE: Record<PreviewPerson, Pick<identity_phienHienTaiRa, "staff" | "role">> = {
  "ke-toan": {
    staff: { code: "CB-00001", full_name: "Cán bộ A", position: "Kế toán ngân sách" },
    role: { code: "ke-toan", name: "Kế toán", is_leader: false },
  },
  // Cán bộ C of `PREVIEW_STAFF` — the `assigner` of the Nhiệm vụ fixture, so the pending extension
  // request shows its Duyệt / Từ chối (ADR 0038's second layer compares this code with the row).
  "lanh-dao": {
    staff: { code: "CB-00003", full_name: "Cán bộ C", position: "Phó Chủ tịch UBND" },
    role: { code: "lanh-dao", name: "Lãnh đạo", is_leader: true },
  },
};

export function previewSession(permissions: readonly string[], person: PreviewPerson = "ke-toan"): identity_phienHienTaiRa {
  return {
    sid: "xem-thu",
    expires_at: "2099-01-01T00:00:00Z",
    ...PEOPLE[person],
    permissions: [...permissions],
    must_change_password: false,
  };
}
