import { describe, expect, it } from "vitest";

import {
  cauBoQua,
  cauGieoTuan,
  hoursCellLabel,
  nhanLinhVuc,
  nhanLoaiViec,
  nhanSoGio,
} from "./nhan-thoi-han";

/**
 * Câu chữ của tab Thời hạn xử lý. Bốn quyết định dưới đây trông như chuyện trình bày và không
 * phải: mỗi cái đều đổi điều cán bộ HIỂU về một con số của cơ quan mình.
 */

describe("nhanSoGio — đơn vị luôn đi kèm", () => {
  it("nói rõ 'giờ làm việc', không phải 'giờ'", () => {
    // 40 giờ làm việc là trọn một tuần. Đọc nhầm thành giờ đồng hồ là nới hoặc siết một cam kết
    // với người dân theo hệ số tám, mà không có gì báo lỗi (ADR 0007, luật 10 bất biến 4).
    expect(nhanSoGio(40)).toBe("40 giờ làm việc");
    expect(nhanSoGio(2)).toBe("2 giờ làm việc");
  });
});

describe("hoursCellLabel — ô có thể chưa đặt", () => {
  it("null đọc là 'Không báo', con số vẫn kèm đơn vị", () => {
    expect(hoursCellLabel(null)).toBe("Không báo");
    expect(hoursCellLabel(8)).toBe("8 giờ làm việc");
  });
});

describe("nhanLoaiViec", () => {
  it("dịch bốn mã của ràng buộc CHECK sang tiếng Việt hành chính", () => {
    expect(nhanLoaiViec("van-ban-den")).toBe("Văn bản đến");
    expect(nhanLoaiViec("don-thu")).toBe("Đơn thư");
    expect(nhanLoaiViec("phan-anh")).toBe("Phản ánh của người dân");
    expect(nhanLoaiViec("nhiem-vu")).toBe("Nhiệm vụ");
  });

  it("mã lạ hiện NGUYÊN MÃ, không hiện một cái tên đoán ra", () => {
    // Một loại việc lạ trong bảng nghĩa là CSDL không còn đúng lược đồ mã này được dựng theo. Che
    // nó bằng một cái tên đẹp là xoá đúng dấu vết người vận hành cần.
    expect(nhanLoaiViec("mot-loai-la")).toBe("mot-loai-la");
  });
});

describe("nhanLinhVuc", () => {
  it("dòng mặc định nói rõ nó áp cho mọi lĩnh vực", () => {
    expect(nhanLinhVuc("", true)).toBe("Mặc định cho mọi lĩnh vực");
  });

  it("có nhãn của xã (đọc từ petitions) → hiện nhãn, không hiện mã", () => {
    const labels = new Map([["an-ninh-trat-tu", "An ninh, trật tự"]]);
    expect(nhanLinhVuc("an-ninh-trat-tu", false, labels)).toBe("An ninh, trật tự");
  });

  it("không có nhãn (đọc hỏng, thiếu admin.lookup, mã lạ, nhãn rỗng) → hiện NGUYÊN MÃ, không đoán", () => {
    expect(nhanLinhVuc("an-ninh-trat-tu", false)).toBe("an-ninh-trat-tu");
    expect(nhanLinhVuc("an-ninh-trat-tu", false, new Map([["cap-thoat-nuoc", "Cấp thoát nước"]]))).toBe(
      "an-ninh-trat-tu",
    );
    expect(nhanLinhVuc("an-ninh-trat-tu", false, new Map([["an-ninh-trat-tu", ""]]))).toBe("an-ninh-trat-tu");
  });
});

describe("cauBoQua — `skipped` khác `kept`", () => {
  it("bỏ qua 0 dòng thì KHÔNG có câu nào", () => {
    // Một dòng "bỏ qua 0" là một lời báo động giả, và báo động giả lặp lại làm hỏng mọi lời báo
    // động thật trên cùng màn hình.
    expect(cauBoQua(0)).toBe("");
    expect(cauGieoTuan(10, 0, 0)).not.toContain("Bỏ qua");
  });

  it("bỏ qua vài dòng thì nói ra, và nói là phải đi xem lại", () => {
    // `kept` là "đơn vị đã có rồi"; `skipped` là "chúng tôi KHÔNG ghi, vì ghi vào sẽ làm hỏng
    // lịch". Gộp hai con số là giấu đúng con số cần người xem.
    expect(cauBoQua(2)).toContain("Bỏ qua 2 dòng");
    expect(cauGieoTuan(8, 0, 2)).toContain("hãy xem lại các dòng đang có");
  });
});
