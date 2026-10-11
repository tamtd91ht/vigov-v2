import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_XOA_DONG_DANH_BA } from "@/lib/quyen";

import {
  CAU_CO_TAI_KHOAN,
  GIAI_THICH_XOA,
  duocXoaTheoPhien,
  yeuCauXoa,
} from "./xoa-dong";

/**
 * Xoá một dòng nhập trùng (#10). Hai điều chịu lực:
 *
 *   1. Dòng có tài khoản đăng nhập dừng ở client; mọi dòng khác gửi được, KHÔNG cần lý do
 *      (bug sheet row 27, owner decision 10/10/2026 — the server records a fixed sentence).
 *   2. Nút chỉ hiện khi phiên đã đọc xong VÀ có `admin.user.delete` — `admin.user` một mình không đủ.
 */

function canBo(ghiDe: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    department_id: "",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: false,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...ghiDe,
  };
}

describe("yeuCauXoa — chỉ dòng có tài khoản bị chặn tại chỗ", () => {
  it("dòng không có tài khoản → gửi được, không hỏi lý do", () => {
    expect(yeuCauXoa(canBo())).toBeNull();
  });

  it("dòng CÓ tài khoản → câu giải thích, không gửi", () => {
    expect(yeuCauXoa(canBo({ has_account: true }))).toBe(CAU_CO_TAI_KHOAN);
    expect(CAU_CO_TAI_KHOAN).toContain("khoá tài khoản");
  });
});

describe("khoá quyền của nút 🗑", () => {
  const phien = (permissions: string[]) =>
    ({ ok: true, duLieu: { permissions } }) as unknown as Parameters<typeof duocXoaTheoPhien>[0];

  it("là `admin.user.delete`", () => {
    expect(QUYEN_XOA_DONG_DANH_BA).toBe("admin.user.delete");
  });

  it("chưa đọc phiên, đọc hỏng, hay chỉ có `admin.user` → KHÔNG vẽ nút (ca bị từ chối)", () => {
    expect(duocXoaTheoPhien(null)).toBe(false);
    expect(duocXoaTheoPhien({ ok: false, thongBao: "Phiên hết hạn." })).toBe(false);
    expect(duocXoaTheoPhien(phien(["admin.user"]))).toBe(false);
    // So khớp CHÍNH XÁC, không tiền tố: `admin.user` không mở `admin.user.delete`, và ngược lại.
    expect(duocXoaTheoPhien(phien(["admin.user.del"]))).toBe(false);
  });

  it("có `admin.user.delete` → vẽ nút", () => {
    expect(duocXoaTheoPhien(phien(["admin.user", "admin.user.delete"]))).toBe(true);
  });
});

describe("câu giải thích theo prototype", () => {
  it("nói người này khuất khỏi danh bạ và Mini App, bản ghi vẫn được giữ, không có nút hoàn tác", () => {
    expect(GIAI_THICH_XOA).toContain("khuất khỏi danh bạ và khỏi Mini App");
    expect(GIAI_THICH_XOA).toContain("Bản ghi vẫn được giữ trong hệ thống");
    expect(GIAI_THICH_XOA).toContain("không có nút hoàn tác");
    expect(GIAI_THICH_XOA).not.toMatch(/lý do/i);
  });
});
