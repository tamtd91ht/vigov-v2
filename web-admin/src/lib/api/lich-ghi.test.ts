import { afterEach, describe, expect, it, vi } from "vitest";

import {
  gieoNgayNghiLeMacDinh,
  gieoTuanMacDinh,
  suaCaLamViec,
  themCaLamViec,
  xoaCaLamViec,
  xoaNgayNghiLe,
} from "./lich-ghi";

/**
 * Mười một tuyến ghi lịch, nhìn từ phía **dây**. Ba điều tệp này canh đều không hiện ra trên màn
 * hình, và cả ba đều là những thứ một lần sửa MỘT DÒNG phá được:
 *
 *   1. `reason` đi trong THÂN của `DELETE` — không có nó thì xoá mềm mất cột `delete_reason`,
 *      tức mất câu trả lời khi đoàn kiểm tra hỏi vì sao lịch của xã đổi (luật 7, bất biến 1).
 *   2. `PATCH` chỉ mang những trường đã đổi — "không nhắc tới" là "giữ nguyên".
 *   3. Gieo ngày nghỉ lễ PHẢI nêu năm; "năm nay" không phải mặc định client được phép chọn.
 */

function ghiGia(ma: number, than: unknown) {
  const gia = vi.fn(
    async () =>
      new Response(than === null ? null : JSON.stringify(than), {
        status: ma,
        headers: than === null ? {} : { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

const CA = { id: "01J00000000000000000000CA", weekday: 1, start: "07:30:00", end: "11:30:00", note: "Buổi sáng" };

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/v1/working-hours", () => {
  it("gửi đúng bốn trường của hợp đồng, chờ 201", async () => {
    const gia = ghiGia(201, CA);
    await themCaLamViec({ weekday: 1, start: "07:30", end: "11:30", note: "Buổi sáng" });

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/working-hours");
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      weekday: 1,
      start: "07:30",
      end: "11:30",
      note: "Buổi sáng",
    });
  });

  it("409 'giờ mở ca đã bị chiếm' đi ra NGUYÊN VĂN", async () => {
    // Câu này là toàn bộ điểm của tuyến: `UNIQUE (tenant_id, thu, bat_dau)` tính cả dòng ĐÃ XOÁ,
    // nên một ca không còn trên màn hình vẫn chắn đường. Viết lại câu ấy ở client là dựng bản sao
    // thứ hai của một quy tắc nghiệp vụ, và bản sao ấy trôi mà không ai thấy.
    const cau =
      "Xã đã có một ca bắt đầu đúng giờ này trong thứ này — kể cả khi ca đó đã bị xoá. " +
      "Hãy chọn giờ bắt đầu khác, hoặc sửa ca đang có.";
    ghiGia(409, { code: "session_start_taken", message: cau, trace_id: "01JTRACE" });

    const kq = await themCaLamViec({ weekday: 1, start: "07:30", end: "11:30", note: "" });
    expect(kq).toEqual({ ok: false, thongBao: cau });
  });
});

describe("PATCH /api/v1/working-hours/{id}", () => {
  it("chỉ mang trường đã đổi — trường vắng nghĩa là giữ nguyên", async () => {
    const gia = ghiGia(200, CA);
    await suaCaLamViec(CA.id, { start: "07:00" });

    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toEqual({ start: "07:00" });
  });

  it("id vào đường dẫn ĐÃ MÃ HOÁ", async () => {
    const gia = ghiGia(200, CA);
    await suaCaLamViec("a/b", { note: "x" });

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/working-hours/a%2Fb");
  });
});

describe("DELETE — lý do là BẮT BUỘC và đi trong thân", () => {
  it("ca làm việc: `reason` trong thân, chờ 204 không thân", async () => {
    const gia = ghiGia(204, null);
    const kq = await xoaCaLamViec(CA.id, "Xã đổi giờ làm từ 01/10");

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("DELETE");
    expect(duongDan).toBe(`/api/v1/working-hours/${CA.id}`);
    expect(JSON.parse(String(tuyChon.body))).toEqual({ reason: "Xã đổi giờ làm từ 01/10" });
    // 204 KHÔNG CÓ THÂN: một hàm luôn gọi `.json()` sẽ biến lần xoá thành công thành "không đọc
    // được".
    expect(kq).toEqual({ ok: true, duLieu: null });
  });

  it("ngày nghỉ lễ: lý do KHÔNG đi vào query string", async () => {
    // Một câu tự do do người gõ về một hồ sơ nhà nước nằm trong query string sẽ đi vào mọi access
    // log và mọi bộ đệm proxy.
    const gia = ghiGia(204, null);
    await xoaNgayNghiLe("01J0000000000000000000LE", "Khai nhầm ngày");

    const { duongDan } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/public-holidays/01J0000000000000000000LE");
    expect(duongDan).not.toContain("?");
  });
});

describe("hai tuyến gieo lịch", () => {
  it("gieo tuần làm việc: KHÔNG thân, KHÔNG `Content-Type` — hợp đồng khai `than: never`", async () => {
    const gia = ghiGia(200, { seeded: 10, kept: 0, skipped: 0 });
    await gieoTuanMacDinh();

    const { tuyChon, header } = loiGoi(gia, 0);
    expect(tuyChon.body).toBeUndefined();
    expect(header.get("Content-Type")).toBeNull();
  });

  it("gieo ngày nghỉ lễ: NÊU RÕ NĂM trong thân", async () => {
    // VẾ CHỊU LỰC. Máy chủ cố ý không lấy "năm nay" làm mặc định: đúng 0h ngày 01/01 một client
    // không gửi tham số sẽ gieo sang một năm khác mà không dòng mã nào thay đổi.
    const gia = ghiGia(200, { seeded: 4, kept: 0, skipped: 0 });
    await gieoNgayNghiLeMacDinh(2026);

    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toEqual({ year: 2026 });
  });

  it("gieo ngày nghỉ lễ đọc được cả ba con số, gồm `skipped`", async () => {
    // `skipped` KHÁC `kept`, và màn hình nợ người đọc hai câu khác nhau: `kept` là "đơn vị đã có
    // rồi", `skipped` là "chúng tôi KHÔNG ghi, vì ghi vào sẽ làm hỏng lịch".
    ghiGia(200, { seeded: 2, kept: 1, skipped: 1 });
    const kq = await gieoNgayNghiLeMacDinh(2026);

    expect(kq).toEqual({ ok: true, duLieu: { seeded: 2, kept: 1, skipped: 1 } });
  });
});
