/**
 * The decision half of two things on the Phân quyền tab (ADR 0055), kept out of the `.tsx` so the
 * cases nobody sees on a working screen have tests:
 *
 *   1. the button "Tạo tám vai trò mẫu" and what it says before and after (`POST /api/v1/roles/defaults`);
 *   2. the warning shown when a key the templates deliberately never grant — `feedback.classify`,
 *      `feedback.unmask` — is held by NO role but the default administrator.
 *
 * WHY THE WARNING EXISTS (ADR 0055 §Cái giá): a commune that uses only the templates has exactly one
 * role able to classify citizen reports, and classifying is what FIXES the resolve deadline
 * (ADR 0028). A commune that forgets to grant it sees its figures degrade with no visible cause.
 * The warning is the only way the consequence of "no template grants it" reaches the screen.
 */

import type { identity_nhomQuyenRa, identity_seedRoleTemplatesOut, identity_vaiTroCotRa } from "@/lib/api/schema.gen";
import { CITIZEN_REPORT_UNMASK_PERMISSION, QUYEN_PHAN_LOAI_PHAN_ANH } from "@/lib/quyen";

import { oDaCap } from "./ma-tran-quyen";
import type { BangDaCap } from "./ma-tran-quyen";

/**
 * `vai_tro.ma` of the default administrator seeded at the commune's first login
 * (`service-identity/internal/store/quan_tri_mac_dinh.go:33`, ADR 0046). It holds every key, so a
 * key it alone holds is a key no working role holds — which is exactly what ADR 0055 asks to flag.
 */
export const DEFAULT_ADMIN_ROLE_CODE = "quan-tri-he-thong";

/**
 * The keys the warning watches, each with the consequence said in plain words. Only these two:
 * they are the two ADR 0030 forbids granting automatically. `admin.user.delete` is also never in a
 * template, but a commune where nobody may delete staff rows loses nothing silently.
 */
const WATCHED: readonly { readonly code: string; readonly consequence: string }[] = [
  {
    code: QUYEN_PHAN_LOAI_PHAN_ANH,
    consequence:
      "phiếu phản ánh sẽ không có ai ngoài tài khoản quản trị hệ thống phân loại được — phiếu chưa " +
      "phân loại vẫn bị tính thời hạn và có thể thành trễ hạn.",
  },
  {
    code: CITIZEN_REPORT_UNMASK_PERMISSION,
    consequence:
      "ngoài tài khoản quản trị hệ thống, không ai xem được đầy đủ họ tên và số điện thoại người gửi " +
      "phản ánh khi cần liên hệ lại.",
  },
];

export type UnheldWarning = { readonly code: string; readonly label: string; readonly sentence: string };

/**
 * Warnings for the matrix AS SAVED — the caller passes the server's grants, never the unsaved
 * ticks: a warning that vanishes on a tick nobody saved is a warning that lied.
 *
 * A watched key with NO ROW in the matrix is skipped, not warned about: the catalogue is the
 * server's, and a key it does not list is not one this screen can tell anybody to grant.
 */
export function unheldPermissionWarnings(
  groups: readonly identity_nhomQuyenRa[],
  roles: readonly identity_vaiTroCotRa[],
  granted: BangDaCap,
): readonly UnheldWarning[] {
  const labels = new Map<string, string>();
  for (const g of groups) for (const p of g.permissions) labels.set(p.code, p.label);

  const out: UnheldWarning[] = [];
  for (const w of WATCHED) {
    const label = labels.get(w.code);
    if (label === undefined) continue;
    const heldByWorkingRole = roles.some(
      (r) => r.code !== DEFAULT_ADMIN_ROLE_CODE && oDaCap(granted, r.id, w.code),
    );
    if (heldByWorkingRole) continue;
    out.push({
      code: w.code,
      label,
      sentence: `Chưa vai trò nào (ngoài Quản trị hệ thống) có quyền "${label}" (${w.code}) — ${w.consequence}`,
    });
  }
  return out;
}

/* ---- the seed button -------------------------------------------------------------------------- */

export const SEED_BUTTON = "Tạo tám vai trò mẫu";
export const SEED_CONFIRM_BUTTON = "Đồng ý tạo";
export const SEED_CANCEL_BUTTON = "Huỷ";
export const SEED_SENDING = "Đang tạo vai trò mẫu…";

/**
 * Said BEFORE the click, because the three rules it states are the ones a person would otherwise
 * fear or assume wrongly: "will it overwrite my ticks?" (no), "will it bring back the role I
 * deleted?" (no), "why was I refused?" (you must hold every key it grants).
 *
 * NO ROLE NAMES, NO KEY COUNT: the list is the server's (`domain.RoleTemplates`); a copy here would
 * be wrong the day the customer changes one line of ADR 0055 §4.
 */
export const SEED_CONFIRM_TEXT =
  "Hệ thống sẽ tạo các vai trò mẫu mà đơn vị chưa có, mỗi vai trò kèm sẵn bộ quyền mẫu. " +
  "Vai trò đơn vị đã có thì giữ nguyên — kể cả các quyền đã tick hoặc đã bỏ tick. " +
  "Vai trò đơn vị đã xoá thì không được tạo lại. " +
  "Chỉ người đang giữ đủ mọi quyền mà bộ mẫu cấp mới tạo được. " +
  "Không vai trò mẫu nào có quyền Phân loại phản ánh và Xem đầy đủ người gửi — hãy tick hai quyền " +
  "này cho người phụ trách sau khi tạo.";

export const SEED_HINT =
  `Bấm "${SEED_BUTTON}" để tạo sẵn các vai trò thường dùng ở cấp xã, mỗi vai trò kèm bộ quyền mẫu, ` +
  "rồi điều chỉnh quyền của từng vai trò trong bảng.";

/** Said only while it is TRUE — see `seedHint`. */
export const ONLY_ADMIN_ROLE_HINT = "Hiện đơn vị chỉ có vai trò Quản trị hệ thống.";

/**
 * The hint above the seed button. The "only the administrator role" sentence is a statement of fact
 * about this commune, so it is said only when the matrix just read shows exactly that one role —
 * printed unconditionally it read as false on every commune that already had its roles (PQ-01).
 * `roles === null` (matrix not read yet, or unreadable) is "do not know": the sentence is omitted.
 */
export function seedHint(roles: readonly Pick<identity_vaiTroCotRa, "code">[] | null): string {
  const onlyAdmin = roles !== null && roles.length === 1 && roles[0]?.code === DEFAULT_ADMIN_ROLE_CODE;
  return onlyAdmin ? `${ONLY_ADMIN_ROLE_HINT} ${SEED_HINT}` : SEED_HINT;
}

export type SeedSummaryBlock = { readonly heading: string; readonly names: readonly string[] };

/**
 * The three lists of a run, as headed blocks — empty lists omitted. When nothing was created the
 * first line says so plainly: a run that created nothing is a normal answer on a second click, and
 * must not read as a failure.
 */
export function seedSummary(r: identity_seedRoleTemplatesOut): {
  readonly lead: string;
  readonly blocks: readonly SeedSummaryBlock[];
} {
  const names = (xs: readonly { name: string }[]) => xs.map((x) => x.name);
  const blocks: SeedSummaryBlock[] = [];
  if (r.created.length > 0) blocks.push({ heading: "Đã tạo", names: names(r.created) });
  if (r.skipped_existing.length > 0) {
    blocks.push({ heading: "Đơn vị đã có — giữ nguyên, không đổi quyền", names: names(r.skipped_existing) });
  }
  if (r.skipped_deleted.length > 0) {
    blocks.push({ heading: "Đơn vị đã xoá trước đây — không tạo lại", names: names(r.skipped_deleted) });
  }
  const lead =
    r.created.length > 0
      ? `Đã tạo ${r.created.length} vai trò mẫu. Bảng phân quyền đã được tải lại.`
      : "Không tạo thêm vai trò nào: đơn vị đã có hoặc đã xoá mọi vai trò mẫu.";
  return { lead, blocks };
}
