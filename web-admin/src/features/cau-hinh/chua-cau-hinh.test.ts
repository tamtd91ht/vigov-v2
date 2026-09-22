import { describe, expect, it } from "vitest";

import { khoiCanhBao, tinhTrangBang } from "./chua-cau-hinh";

/**
 * Phép quyết định đứng sau khối cảnh báo của tab "Thời hạn xử lý".
 *
 * VẾ CHỊU LỰC Ở ĐÂY LÀ VẾ PHỦ ĐỊNH — "hai bảng đã có dòng thì KHÔNG hiện". Một lời báo động cũng
 * hiện ở xã đã cấu hình xong là một lời báo động người ta học cách bỏ qua, rồi bỏ qua nốt lần nó
 * đúng. Vế khẳng định thì ai cũng thử; vế phủ định là vế người ta quên.
 */

const CO_DONG = { ok: true as const, duLieu: { items: [{ id: "x" }], problems: [] } };
const RONG = { ok: true as const, duLieu: { items: [], problems: [] } };
const HONG = { ok: false as const, thongBao: "Bạn không có quyền thực hiện thao tác này." };

describe("tinhTrangBang — ba trạng thái, không hai", () => {
  it("chưa đọc xong (`null`) là CHƯA BIẾT, không phải rỗng", () => {
    // Gộp hai thứ này lại thì khối cảnh báo nhấp nháy ở mọi lần mở trang của một xã đã đủ cấu
    // hình — và một lời báo động xuất hiện rồi biến mất là lời báo động không ai còn tin.
    expect(tinhTrangBang(null)).toBe("chuaBiet");
  });

  it("đọc hỏng (403, mạng hỏng) cũng là CHƯA BIẾT", () => {
    expect(tinhTrangBang(HONG)).toBe("chuaBiet");
  });

  it("đọc được và không có dòng nào là TRỐNG", () => {
    expect(tinhTrangBang(RONG)).toBe("trong");
  });

  it("đọc được và có dòng là ĐÃ KHAI", () => {
    expect(tinhTrangBang(CO_DONG)).toBe("daKhai");
  });
});

describe("khoiCanhBao — hiện khi BẤT KỲ bảng nào rỗng", () => {
  it("thời hạn rỗng, lịch đã có → HIỆN, và nêu đúng bảng đang thiếu", () => {
    expect(khoiCanhBao("trong", "daKhai")).toEqual({
      hien: true,
      thieuThoiHan: true,
      thieuLichTuan: false,
    });
  });

  it("lịch rỗng, thời hạn đã có → VẪN HIỆN", () => {
    // Điều kiện `&&` ở chỗ này sẽ tạo ra đúng trạng thái im lặng nguy hiểm nhất: xã đã gieo thời
    // hạn, tưởng xong, mà `ResolveDeadlines` vẫn từ chối vì không đếm được một giờ làm việc nào.
    expect(khoiCanhBao("daKhai", "trong")).toEqual({
      hien: true,
      thieuThoiHan: false,
      thieuLichTuan: true,
    });
  });

  it("cả hai rỗng → hiện, nêu cả hai", () => {
    expect(khoiCanhBao("trong", "trong")).toEqual({
      hien: true,
      thieuThoiHan: true,
      thieuLichTuan: true,
    });
  });

  it("CẢ HAI ĐÃ CÓ DÒNG → VẮNG MẶT", () => {
    expect(khoiCanhBao("daKhai", "daKhai")).toEqual({ hien: false });
  });

  it("chưa biết thì KHÔNG kêu — khối này là một lời khẳng định, không phải một cánh cổng", () => {
    // Khẳng định "đơn vị chưa khai xong" sau một lượt đọc hỏng là nói sai với một cơ quan nhà
    // nước, và câu sai ấy đẩy cán bộ đi gieo cho một bảng có thể đang đầy đủ. Khi đọc hỏng, thứ
    // phải hiện là CÂU CỦA MÁY CHỦ, và bảng tương ứng hiện nó. Phép đóng-khi-không-chắc thật sự
    // vẫn nguyên ở máy chủ: không cấu hình thì tuyến tiếp nhận từ chối, bất kể màn hình nói gì.
    expect(khoiCanhBao("chuaBiet", "chuaBiet")).toEqual({ hien: false });
    expect(khoiCanhBao("chuaBiet", "daKhai")).toEqual({ hien: false });
  });
});
