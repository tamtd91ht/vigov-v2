import { describe, expect, it } from "vitest";

import { cauBoQua, cauGieoTuan, nhanLinhVuc, nhanLoaiViec, nhanSoGio } from "./nhan-thoi-han";

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

describe("nhanLoaiViec", () => {
  it("dịch ba mã của ràng buộc CHECK sang tiếng Việt hành chính", () => {
    expect(nhanLoaiViec("van-ban-den")).toBe("Văn bản đến");
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

  it("dòng có lĩnh vực hiện MÃ THÔ — web không giữ bảng tra tên lĩnh vực nào", () => {
    // Danh mục `Lĩnh vực phản ánh` chưa có chủ (câu mở #4, ADR 0024) và chưa có tuyến nào phát ra
    // nhãn của nó. Một bảng tra gõ tay ở đây là bản sao thứ hai của một danh mục chưa ai sở hữu.
    expect(nhanLinhVuc("an-ninh-trat-tu", false)).toBe("an-ninh-trat-tu");
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
