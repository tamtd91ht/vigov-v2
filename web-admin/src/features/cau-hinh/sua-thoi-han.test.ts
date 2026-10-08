import { describe, expect, it } from "vitest";

import type { identity_dongSLARa } from "@/lib/api/schema.gen";

import { LOI_KHONG_DOI_GI, LOI_SO_GIO_LA, PRESIDENT_BELOW_LEADER_ERROR, RESOLVE_HOURS_ERROR } from "./nhan-thoi-han";
import { banTuDong, soanSua } from "./sua-thoi-han";

/**
 * `PATCH /api/v1/sla/{id}` đọc "không nhắc tới" là "giữ nguyên". Tệp này canh đúng một điều, và
 * nó là điều không màn hình nào cho thấy: **thân yêu cầu mang ĐÚNG những cột đã đổi**.
 *
 * Gửi cả sáu cột trông vô hại và sai ở chỗ khó thấy nhất: hai quản trị viên sửa hai cột khác nhau
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
  unassigned_hold_hours: 8,
};

/** The same row with the sixth figure unset — "do not report". */
const DONG_KHONG_BAO: identity_dongSLARa = { ...DONG, unassigned_hold_hours: null };

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
    // cùng một thân — nếu không, mọi lần mở biểu mẫu rồi bấm Lưu đều là một lần ghi đè sáu cột.
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

  it("Xử lý xong bằng 0 hoặc âm → câu riêng của spec 08, 'Thời hạn xử lý phải lớn hơn 0 giờ.'", () => {
    // REGRESSION: before spec 08 this box got the general LOI_SO_GIO_LA like every other.
    for (const bad of ["0", "-4"]) {
      expect(soanSua(DONG, { ...banTuDong(DONG), resolve_hours: bad }), bad).toEqual({
        ok: false,
        loi: RESOLVE_HOURS_ERROR,
      });
    }
    expect(RESOLVE_HOURS_ERROR).toBe("Thời hạn xử lý phải lớn hơn 0 giờ.");
  });

  it("Xử lý xong trống, chữ hay số lẻ → vẫn câu chung: không phải lỗi 'nhỏ hơn 0'", () => {
    for (const bad of ["", "?", "1.5", "16 giờ"]) {
      expect(soanSua(DONG, { ...banTuDong(DONG), resolve_hours: bad }), bad).toEqual({
        ok: false,
        loi: LOI_SO_GIO_LA,
      });
    }
  });

  it("ô không đọc được của trình duyệt ('?') ở cột thứ sáu bị TỪ CHỐI, không thành 'tắt báo'", () => {
    // `type="number"` reports "" for "-" or "1e"; the tab stores "?" instead (`readNumberBox`),
    // because "" in this column means `null` = do not report.
    expect(soanSua(DONG, { ...banTuDong(DONG), unassigned_hold_hours: "?" })).toEqual({
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
  it("nạp đủ sáu ô, không ô nào trống khi dòng đã đặt đủ", () => {
    expect(banTuDong(DONG)).toEqual({
      acknowledge_hours: "2",
      resolve_hours: "16",
      due_soon_hours: "4",
      escalate_leader_hours: "8",
      escalate_president_hours: "16",
      unassigned_hold_hours: "8",
    });
  });

  it("ngưỡng giữ việc CHƯA ĐẶT thì ô trống — không điền một con số thay cho 'không báo'", () => {
    // NULL là lựa chọn của xã (ADR 0029 §Bổ sung 29/09). Điền sẵn 8 vào ô là để một lần bấm Lưu vì
    // sửa cột khác lặng lẽ BẬT một lời báo xã đã chọn tắt.
    expect(banTuDong(DONG_KHONG_BAO).unassigned_hold_hours).toBe("");
  });
});

describe("soanSua — ngưỡng giữ việc chưa giao có ba trạng thái", () => {
  it("không đụng tới ô ấy thì khoá ấy KHÔNG đi lên", () => {
    const kq = soanSua(DONG, { ...banTuDong(DONG), resolve_hours: "24" });
    expect(kq.ok && Object.keys(kq.than)).toEqual(["resolve_hours"]);
  });

  it("xoá trắng một ngưỡng đang có → gửi `null` (tắt báo), không từ chối như năm cột kia", () => {
    const kq = soanSua(DONG, { ...banTuDong(DONG), unassigned_hold_hours: "  " });
    expect(kq).toEqual({ ok: true, than: { unassigned_hold_hours: null } });
  });

  it("đặt ngưỡng cho một dòng đang không báo → gửi con số", () => {
    const kq = soanSua(DONG_KHONG_BAO, { ...banTuDong(DONG_KHONG_BAO), unassigned_hold_hours: "16" });
    expect(kq).toEqual({ ok: true, than: { unassigned_hold_hours: 16 } });
  });

  it("dòng đang không báo, ô vẫn trống → không có gì để gửi", () => {
    expect(soanSua(DONG_KHONG_BAO, banTuDong(DONG_KHONG_BAO))).toEqual({
      ok: false,
      loi: LOI_KHONG_DOI_GI,
    });
  });

  it("số 0 và chữ bị từ chối — 0 không phải cách nói 'không báo', và NaN sẽ thành `null` = tắt báo", () => {
    for (const hong of ["0", "-1", "8 giờ", "1.5"]) {
      expect(soanSua(DONG, { ...banTuDong(DONG), unassigned_hold_hours: hong }), hong).toEqual({
        ok: false,
        loi: LOI_SO_GIO_LA,
      });
    }
  });
});

describe("soanSua — Báo Chủ tịch không nhỏ hơn Báo lãnh đạo trực tiếp", () => {
  it("hạ Báo Chủ tịch xuống dưới Báo lãnh đạo → từ chối tại chỗ, nói rõ quy tắc", () => {
    const kq = soanSua(DONG, { ...banTuDong(DONG), escalate_president_hours: "4" });
    expect(kq).toEqual({ ok: false, loi: PRESIDENT_BELOW_LEADER_ERROR });
  });

  it("chỉ nâng Báo lãnh đạo lên trên Báo Chủ tịch cũng bị từ chối — kiểm trên DÒNG SAU KHI SỬA", () => {
    // Máy chủ kiểm dòng kết quả, không kiểm riêng ô vừa gõ. Kiểm riêng ô vừa gõ ở đây thì lần sửa
    // này đi lên và nhận 400 — đúng lượt vòng mà phép kiểm tại chỗ tồn tại để tránh.
    const kq = soanSua(DONG, { ...banTuDong(DONG), escalate_leader_hours: "24" });
    expect(kq).toEqual({ ok: false, loi: PRESIDENT_BELOW_LEADER_ERROR });
  });

  it("bằng nhau thì được — quy tắc là Y ≥ X, không phải Y > X", () => {
    const kq = soanSua(DONG, { ...banTuDong(DONG), escalate_president_hours: "8" });
    expect(kq).toEqual({ ok: true, than: { escalate_president_hours: 8 } });
  });

  it("nâng cả hai một lúc cho đúng thứ tự thì được", () => {
    const kq = soanSua(DONG, {
      ...banTuDong(DONG),
      escalate_leader_hours: "24",
      escalate_president_hours: "48",
    });
    expect(kq).toEqual({ ok: true, than: { escalate_leader_hours: 24, escalate_president_hours: 48 } });
  });
});
