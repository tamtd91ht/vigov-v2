import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_XOA_DONG_DANH_BA } from "@/lib/quyen";

import {
  CAU_CO_TAI_KHOAN,
  CAU_LY_DO_QUA_DAI,
  CAU_THIEU_LY_DO,
  GIAI_THICH_XOA,
  duocXoaTheoPhien,
  yeuCauXoa,
} from "./xoa-dong";

/**
 * Xoá một dòng nhập trùng (#10). Hai điều chịu lực:
 *
 *   1. KHÔNG CÓ YÊU CẦU XOÁ NÀO THIẾU LÝ DO — rỗng, toàn khoảng trắng, quá dài đều dừng ở client.
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

describe("yeuCauXoa — không yêu cầu nào thiếu lý do", () => {
  it("lý do rỗng hay toàn khoảng trắng → câu từ chối, không có lý do để gửi", () => {
    expect(yeuCauXoa(canBo(), "")).toEqual({ loi: CAU_THIEU_LY_DO });
    expect(yeuCauXoa(canBo(), "   \n ")).toEqual({ loi: CAU_THIEU_LY_DO });
  });

  it("lý do 501 ký tự → câu từ chối; 500 ký tự có dấu → nhận", () => {
    expect(yeuCauXoa(canBo(), "ễ".repeat(501))).toEqual({ loi: CAU_LY_DO_QUA_DAI });
    expect("lyDo" in yeuCauXoa(canBo(), "ễ".repeat(500))).toBe(true);
  });

  it("lý do hợp lệ → đã cắt hai đầu", () => {
    expect(yeuCauXoa(canBo(), "  nhập trùng  ")).toEqual({
      lyDo: { loai: "hopLe", lyDo: "nhập trùng" },
    });
  });

  it("dòng CÓ tài khoản → câu giải thích, không gửi — kể cả khi lý do hợp lệ", () => {
    expect(yeuCauXoa(canBo({ has_account: true }), "nhập trùng")).toEqual({
      loi: CAU_CO_TAI_KHOAN,
    });
    expect(CAU_CO_TAI_KHOAN).toContain("khoá tài khoản");
  });

  it("câu giới hạn nói con số", () => {
    expect(CAU_LY_DO_QUA_DAI).toContain("500");
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

describe("câu giải thích nói đủ bốn điều TRƯỚC khi bấm", () => {
  it("chỉ cho dòng nhập trùng · nghỉ hưu/chuyển công tác thì KHOÁ · dòng có tài khoản không xoá · mã không cấp lại · lý do được lưu", () => {
    expect(GIAI_THICH_XOA).toContain("hai lần do nhầm");
    expect(GIAI_THICH_XOA).toContain("nghỉ hưu hoặc chuyển công tác thì phải khoá tài khoản");
    expect(GIAI_THICH_XOA).toContain("tài khoản đăng nhập không xoá được");
    expect(GIAI_THICH_XOA).toContain("không bao giờ được cấp lại");
    expect(GIAI_THICH_XOA).toContain("lý do xoá được lưu");
  });
});
