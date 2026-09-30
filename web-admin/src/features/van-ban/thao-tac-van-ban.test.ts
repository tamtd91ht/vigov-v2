import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_THIEU_BO_PHAN, LOI_THIEU_LY_DO_CHUYEN, LOI_THIEU_LY_DO_GO } from "./nhan-van-ban";
import { guiChuyenVanBan, guiGoVanBanDen, guiGoVanBanDi } from "./thao-tac-van-ban";

/**
 * MỘT ĐIỀU DUY NHẤT, VÀ NÓ ĐƯỢC ĐO BẰNG SỐ LẦN `fetch` CHẠY: thiếu một ô bắt buộc thì KHÔNG có lời
 * gọi mạng nào đi ra.
 *
 * Vì sao phải đếm chứ không chỉ xem câu trả lời: một phép kiểm chỉ hỏi "có trả về `ok: false` hay
 * không" vẫn xanh khi lời gọi ĐÃ đi ra rồi bị máy chủ từ chối — mà đó đúng là thứ khác nhau. Lần
 * gọi ấy mang ý định chuyển một hồ sơ nhà nước mà không nói được vì sao, và nó đi vào nhật ký truy
 * cập của máy chủ.
 *
 * Ba hàm dưới đây là ĐƯỜNG ĐI DUY NHẤT từ màn hình tới ba tuyến ấy (`so-van-ban-den.tsx`,
 * `so-van-ban-di.tsx`), nên gỡ phép kiểm ra khỏi chúng là làm đỏ đúng những ca này.
 */

function demFetch() {
  const gia = vi.fn(async () => new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("chuyển xử lý — lý do rỗng thì chặn tại client, KHÔNG gọi mạng", () => {
  it("lý do rỗng: không một lần `fetch` nào, và câu nói rõ vì sao lý do là bắt buộc", async () => {
    const gia = demFetch();
    const kq = await guiChuyenVanBan("01JVB", { denBoPhan: "01JBOPHAN", canBoXuLy: "", lyDo: "" });

    expect(gia).not.toHaveBeenCalled();
    expect(kq).toEqual({ ok: false, thongBao: LOI_THIEU_LY_DO_CHUYEN });
    // Câu ấy phải nói ra HỆ QUẢ, không chỉ nói "bắt buộc": dòng lịch sử không sửa được sau khi ghi.
    expect(LOI_THIEU_LY_DO_CHUYEN).toMatch(/KHÔNG sửa được/);
  });

  it("lý do TOÀN DẤU CÁCH cũng là rỗng — `required` của trình duyệt không bắt được ca này", async () => {
    const gia = demFetch();
    const kq = await guiChuyenVanBan("01JVB", { denBoPhan: "01JBOPHAN", canBoXuLy: "", lyDo: "   " });

    expect(gia).not.toHaveBeenCalled();
    expect(kq.ok).toBe(false);
  });

  it("chưa chọn bộ phận: cũng không gọi mạng, và câu KHÁC câu thiếu lý do", async () => {
    const gia = demFetch();
    const kq = await guiChuyenVanBan("01JVB", { denBoPhan: "", canBoXuLy: "", lyDo: "có lý do" });

    expect(gia).not.toHaveBeenCalled();
    expect(kq).toEqual({ ok: false, thongBao: LOI_THIEU_BO_PHAN });
    // Hai ô thiếu là hai câu khác nhau: một câu chung buộc cán bộ tự dò xem mình còn thiếu ô nào.
    expect(LOI_THIEU_BO_PHAN).not.toBe(LOI_THIEU_LY_DO_CHUYEN);
  });

  it("đủ hai ô: CÓ gọi mạng, đúng một lần, và lý do đã cắt khoảng trắng hai đầu", async () => {
    // VẾ ĐỐI XỨNG — không có nó thì một hàm luôn từ chối mọi thứ cũng làm ba ca trên xanh hết.
    const gia = vi.fn(async () => new Response(JSON.stringify({ id: "01JVB" }), { status: 200 }));
    vi.stubGlobal("fetch", gia);

    await guiChuyenVanBan("01JVB", {
      denBoPhan: "01JBOPHAN",
      canBoXuLy: "",
      lyDo: "  thuộc thẩm quyền bộ phận Địa chính  ",
    });

    expect(gia).toHaveBeenCalledTimes(1);
    const [, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(tuyChon.body))).toEqual({
      to_unit: "01JBOPHAN",
      assignee: "",
      reason: "thuộc thẩm quyền bộ phận Địa chính",
    });
  });
});

describe("gỡ khỏi sổ — lý do rỗng thì chặn tại client, ở CẢ HAI quyển sổ", () => {
  it("sổ văn bản đến: không gọi mạng khi lý do rỗng", async () => {
    const gia = demFetch();
    const kq = await guiGoVanBanDen("01JVB", "  ");

    expect(gia).not.toHaveBeenCalled();
    expect(kq).toEqual({ ok: false, thongBao: LOI_THIEU_LY_DO_GO });
  });

  it("sổ văn bản đi: không gọi mạng khi lý do rỗng", async () => {
    const gia = demFetch();
    const kq = await guiGoVanBanDi("01JVB", "");

    expect(gia).not.toHaveBeenCalled();
    expect(kq).toEqual({ ok: false, thongBao: LOI_THIEU_LY_DO_GO });
  });

  it("có lý do: CÓ gọi mạng, đúng một lần, và lý do đi trong thân", async () => {
    const gia = demFetch();
    const kq = await guiGoVanBanDen("01JVB", " nhập trùng với số 12/2026 ");

    expect(gia).toHaveBeenCalledTimes(1);
    expect(kq).toEqual({ ok: true, duLieu: null });
    const [, tuyChon] = gia.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(tuyChon.body))).toEqual({ reason: "nhập trùng với số 12/2026" });
  });
});
