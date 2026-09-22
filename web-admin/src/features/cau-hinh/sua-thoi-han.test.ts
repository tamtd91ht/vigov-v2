import { describe, expect, it } from "vitest";

import type { identity_dongSLARa } from "@/lib/api/schema.gen";

import { LOI_KHONG_DOI_GI, LOI_SO_GIO_LA } from "./nhan-thoi-han";
import { banTuDong, soanSua } from "./sua-thoi-han";

/**
 * `PATCH /api/v1/sla/{id}` đọc "không nhắc tới" là "giữ nguyên". Tệp này canh đúng một điều, và
 * nó là điều không màn hình nào cho thấy: **thân yêu cầu mang ĐÚNG những cột đã đổi**.
 *
 * Gửi cả năm cột trông vô hại và sai ở chỗ khó thấy nhất: hai quản trị viên sửa hai cột khác nhau
 * trên cùng một dòng thì người lưu sau lặng lẽ kéo bốn cột kia về giá trị mình đã đọc lúc mở màn
 * hình. Không gì báo lỗi, và thứ bị kéo lùi là một cam kết của xã với người dân.
 */

const DONG: identity_dongSLARa = {
  id: "01J0000000000000000000SLA",
  work_kind: "phan-anh",
  field: "an-ninh-trat-tu",
  is_default: false,
  acknowledge_hours: 2,
  resolve_hours: 16,
  due_soon_hours: 4,
  escalate_leader_hours: 8,
  escalate_president_hours: 16,
};

describe("soanSua — chỉ gửi những cột đã đổi", () => {
  it("đổi MỘT cột thì thân chỉ có MỘT khoá", () => {
    const kq = soanSua(DONG, { ...banTuDong(DONG), resolve_hours: "24" });
    expect(kq).toEqual({ ok: true, than: { resolve_hours: 24 } });
  });

  it("đổi hai cột thì thân có đúng hai khoá, không khoá nào khác", () => {
    const kq = soanSua(DONG, {
      ...banTuDong(DONG),
      acknowledge_hours: "4",
      escalate_president_hours: "32",
    });
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(Object.keys(kq.than).sort()).toEqual([
      "acknowledge_hours",
      "escalate_president_hours",
    ]);
    expect(kq.than.acknowledge_hours).toBe(4);
    expect(kq.than.escalate_president_hours).toBe(32);
  });

  it("gõ lại ĐÚNG con số đang có thì cột đó KHÔNG đi lên", () => {
    // Bản nháp được nạp sẵn giá trị hiện tại, nên "không sửa gì" và "gõ lại y nguyên" phải cho ra
    // cùng một thân — nếu không, mọi lần mở biểu mẫu rồi bấm Lưu đều là một lần ghi đè năm cột.
    const kq = soanSua(DONG, { ...banTuDong(DONG), resolve_hours: " 16 " });
    expect(kq).toEqual({ ok: false, loi: LOI_KHONG_DOI_GI });
  });

  it("không đổi gì thì TỪ CHỐI tại chỗ, không gửi `{}` lên để nhận 400", () => {
    expect(soanSua(DONG, banTuDong(DONG))).toEqual({ ok: false, loi: LOI_KHONG_DOI_GI });
  });

  it("gõ sai thành chữ thì TỪ CHỐI, không lặng lẽ thành 'không đổi'", () => {
    // VẾ CHỊU LỰC. `Number("12 giờ")` ra `NaN`, `JSON.stringify` biến `NaN` thành `null`, và
    // `null` vào một `*int` của Go là con trỏ rỗng — tức "không nhắc tới". Không có phép kiểm này
    // thì một lỗi gõ đi trọn đường ghi và màn hình báo "Đã lưu" cho một lần không lưu gì.
    const kq = soanSua(DONG, { ...banTuDong(DONG), resolve_hours: "24 giờ" });
    expect(kq).toEqual({ ok: false, loi: LOI_SO_GIO_LA });
  });

  it("ô bị xoá trắng KHÔNG đọc thành 'giữ nguyên'", () => {
    // Biểu mẫu mở ra đã có sẵn con số, nên một ô trống là một ý định — mà "bỏ hẳn một con số"
    // không có trên tuyến. Nói ra, đừng đoán.
    expect(soanSua(DONG, { ...banTuDong(DONG), due_soon_hours: "" })).toEqual({
      ok: false,
      loi: LOI_SO_GIO_LA,
    });
  });

  it("số 0 và số âm bị từ chối trước khi gửi", () => {
    expect(soanSua(DONG, { ...banTuDong(DONG), acknowledge_hours: "0" })).toEqual({
      ok: false,
      loi: LOI_SO_GIO_LA,
    });
    expect(soanSua(DONG, { ...banTuDong(DONG), acknowledge_hours: "-2" })).toEqual({
      ok: false,
      loi: LOI_SO_GIO_LA,
    });
  });
});

describe("banTuDong — biểu mẫu mở ra đã có sẵn con số hiện tại", () => {
  it("nạp đủ năm ô, không ô nào trống", () => {
    expect(banTuDong(DONG)).toEqual({
      acknowledge_hours: "2",
      resolve_hours: "16",
      due_soon_hours: "4",
      escalate_leader_hours: "8",
      escalate_president_hours: "16",
    });
  });
});
