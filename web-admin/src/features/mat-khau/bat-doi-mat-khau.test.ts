import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

import {
  DUONG_DAN_DOI_MAT_KHAU,
  dangBiBatDoi,
  duongDanBatDoiMatKhau,
} from "./bat-doi-mat-khau";

/**
 * Đường bắt đổi mật khẩu ở lần đăng nhập đầu.
 *
 * HAI CHIỀU SAI ĐỀU PHẢI ĐỎ, và chiều thứ hai là chiều không ai thử:
 *
 *   - cờ bật mà không đưa đi đâu → cán bộ bấm quanh và nhận 403 ở mọi nơi, không hiểu vì sao.
 *   - đưa đi trong khi CHƯA BIẾT → trang bị cướp ngay dưới tay người đang đọc, hoặc tệ hơn, một
 *     vòng lặp chuyển hướng ngay trên chính màn đổi mật khẩu.
 */

function phien(batDoi: boolean): KetQua<identity_phienHienTaiRa> {
  return {
    ok: true,
    duLieu: {
      sid: "01J00000000000000000000SID",
      expires_at: "2026-09-22T20:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn A", position: "Chuyên viên" },
      role: null,
      permissions: [],
      must_change_password: batDoi,
    },
  };
}

describe("quyết định đưa người dùng tới màn đổi mật khẩu", () => {
  it("cờ bật, đang ở một màn khác → đưa tới màn đổi mật khẩu", () => {
    expect(duongDanBatDoiMatKhau(phien(true), "/cau-hinh")).toBe(DUONG_DAN_DOI_MAT_KHAU);
    expect(DUONG_DAN_DOI_MAT_KHAU).toBe("/doi-mat-khau");
  });

  it("CHƯA ĐỌC XONG phiên thì không đưa đi đâu cả", () => {
    // `null` là "chưa biết", không phải "cờ tắt". Đẩy đi lúc này là đẩy đi dựa trên một điều
    // chưa biết là đúng hay sai.
    expect(duongDanBatDoiMatKhau(null, "/cau-hinh")).toBeNull();
  });

  it("không đọc được phiên thì KHÔNG suy ra gì — không đẩy về đâu", () => {
    // 403 `password_change_required` khác hẳn 401: phiên vẫn hợp lệ. Ở ca không đọc được phiên
    // thì việc của tệp này không phải đoán; câu của máy chủ đã hiện ở cổng quyền.
    const hong: KetQua<identity_phienHienTaiRa> = {
      ok: false,
      thongBao: "Phiên làm việc đã hết hạn.",
    };
    expect(duongDanBatDoiMatKhau(hong, "/cau-hinh")).toBeNull();
  });

  it("cờ tắt thì không đưa đi đâu cả", () => {
    expect(duongDanBatDoiMatKhau(phien(false), "/cau-hinh")).toBeNull();
  });

  it("đã đứng ở màn đổi mật khẩu thì KHÔNG tự đẩy mình đi — kể cả khi đường có gạch đuôi", () => {
    // Vòng lặp không lối ra là hỏng nặng hơn hẳn việc không chuyển hướng: cán bộ không vào nổi
    // chính màn hình gỡ được cờ.
    expect(duongDanBatDoiMatKhau(phien(true), "/doi-mat-khau")).toBeNull();
    expect(duongDanBatDoiMatKhau(phien(true), "/doi-mat-khau/")).toBeNull();
  });

  it("không có bản sao nào của danh sách ba tuyến máy chủ cho phép", () => {
    // Máy chủ giữ danh sách ấy và so nó với chính các hằng `mux.Handle` nó đăng ký
    // (`service-identity/internal/http/middleware.go`). Một bản thứ hai ở client sẽ trôi khỏi
    // bản ấy. Ở đây kiểm bằng hành vi: hai tuyến KHÁC trong danh sách được phép của máy chủ —
    // `sessions/current` và đăng xuất — không hề là ngoại lệ của hàm này; chúng là lời gọi API,
    // không phải màn hình, nên hàm này không biết tới chúng và không cần biết.
    expect(duongDanBatDoiMatKhau(phien(true), "/")).toBe(DUONG_DAN_DOI_MAT_KHAU);
    expect(duongDanBatDoiMatKhau(phien(true), "/giai-ngan")).toBe(DUONG_DAN_DOI_MAT_KHAU);
  });
});

describe("cờ bắt đổi đang bật hay không", () => {
  it("chỉ bật khi đọc xong, đọc được, và máy chủ nói là bật", () => {
    expect(dangBiBatDoi(phien(true))).toBe(true);
    expect(dangBiBatDoi(phien(false))).toBe(false);
    expect(dangBiBatDoi(null)).toBe(false);
    expect(dangBiBatDoi({ ok: false, thongBao: "Phiên làm việc đã hết hạn." })).toBe(false);
  });
});
