import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import type { finance_hangMucRa } from "@/lib/api/schema.gen";

import {
  hangMucDuAn,
  lopHangMuc,
  lopTienDo,
  nhanHangMuc,
  nhanNgay,
  nhanNguongCham,
  nhanTien,
  nhanTienDo,
  nhanTyLeGiaiNgan,
  tienDoDuAn,
} from "./nhan-du-an";

describe("số tiền", () => {
  it("định dạng vi-VN kèm đơn vị đồng", () => {
    expect(nhanTien(100000000)).toBe("100.000.000 đ");
  });

  it("SỐ ÂM HIỆN NGUYÊN LÀ SỐ ÂM — giải ngân vượt kế hoạch phải nhìn thấy được", () => {
    // Kẹp về 0 là giấu một lần chi vượt kế hoạch, đúng con số có người cần nhìn thấy (§13 #2).
    expect(nhanTien(-10000000)).toBe("-10.000.000 đ");
  });

  it("0 đ là một khẳng định, và nó hiện ra thành 0 đ", () => {
    expect(nhanTien(0)).toBe("0 đ");
  });

  it("giá trị không phải số hữu hạn nói ra bằng câu KHÁC, không phải 'NaN đ'", () => {
    expect(nhanTien(Number.NaN)).toBe("Không đọc được");
  });
});

describe("tỷ lệ giải ngân", () => {
  it("đơn vị là phần vạn: 1033 đọc thành 10,33%", () => {
    expect(nhanTyLeGiaiNgan(1033)).toBe("10,33%");
  });

  it("trên 100% thì HIỆN, không chặn (§13 #2)", () => {
    expect(nhanTyLeGiaiNgan(17650)).toBe("176,50%");
  });

  it("`null` KHÔNG hiện thành 0% — hai câu khác hẳn nhau", () => {
    // `null` = chưa bố trí vốn, không có mẫu số để chia. `0%` = đã bố trí mà chưa chi đồng nào.
    // Hiện 0% cho dự án chưa ai bố trí vốn là báo cáo nó như dự án tệ nhất của xã.
    expect(nhanTyLeGiaiNgan(null)).toBe("Chưa bố trí vốn");
    expect(nhanTyLeGiaiNgan(0)).toBe("0,00%");
  });
});

describe("chip tiến độ", () => {
  it("đang chậm: hiện điểm chậm theo đúng đơn vị đặc tả in ra", () => {
    const t = tienDoDuAn(3136, true);
    expect(nhanTienDo(t)).toBe("Chậm 31,36 điểm");
    expect(lopTienDo(t)).toBe("chip chip-cham");
  });

  it("bám sát: chữ của đặc tả §8, không phải một dấu tích", () => {
    expect(nhanTienDo(tienDoDuAn(500, false))).toBe("Bám sát tiến độ");
  });

  it("chưa bố trí vốn: KHÔNG hiện 'chậm 0 điểm' và cũng không để ô trống", () => {
    // `delay_score` null kéo theo `is_delayed` false ở máy chủ ("dự án chưa bố trí vốn thì không
    // chậm — đó là trạng thái kế toán, không phải sự chậm trễ", `domain.LaCham`).
    const t = tienDoDuAn(null, false);
    expect(nhanTienDo(t)).toBe("Chưa bố trí vốn");
    expect(lopTienDo(t)).toBe("chip chip-ngung");
  });

  it("ba ca là ba lớp CSS khác nhau — màu đi theo LOẠI, không theo câu chữ", () => {
    const lop = [
      lopTienDo(tienDoDuAn(null, false)),
      lopTienDo(tienDoDuAn(0, false)),
      lopTienDo(tienDoDuAn(3136, true)),
    ];
    expect(new Set(lop).size).toBe(3);
  });
});

describe("ngày", () => {
  it("YYYY-MM-DD đọc thành dd/MM/yyyy", () => {
    expect(nhanNgay("2026-12-31")).toBe("31/12/2026");
  });

  it("KHÔNG dựng `Date` ở đâu trong module này", () => {
    // ─────────────────────────────────────────────────────────────────────────────────────
    // ĐỌC THẲNG MÃ NGUỒN, VÌ MỘT CA TEST GỌI HÀM KHÔNG BẮT ĐƯỢC LỖI NÀY.
    //
    // `nhanNgay("2026-12-31")` trả đúng "31/12/2026" ở CẢ hai cách viết — cắt chuỗi, hay đi qua
    // `new Date(...)` rồi định dạng — miễn là máy chạy test đặt múi giờ UTC+7. Nó chỉ sai trên
    // máy của một cán bộ đặt múi giờ khác, tức là sai trong sản xuất và không sai ở đây.
    //
    // Cho một NGÀY qua `new Date("2026-12-31")` là gán cho nó nửa đêm UTC; ở mọi máy phía tây
    // London nó hiện thành ngày 30. Một thời hạn giải ngân lùi một ngày là một con số sai đi
    // tiếp vào báo cáo gửi lên trên.
    // ─────────────────────────────────────────────────────────────────────────────────────
    const nguon = readFileSync(new URL("./nhan-du-an.ts", import.meta.url), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split("\n")
      .filter((d) => !/^\s*\/\//.test(d))
      .join("\n");

    expect(nguon).not.toMatch(/new\s+Date\s*\(/);
    expect(nguon).not.toMatch(/Date\s*\.\s*parse/);
  });

  it("chuỗi rỗng nghĩa là xã chưa đặt mốc ấy — nói ra, không để ô trống", () => {
    expect(nhanNgay("")).toBe("Chưa đặt");
  });

  it("chuỗi sai khuôn hiện NGUYÊN VĂN, không đoán", () => {
    expect(nhanNgay("31/12/2026")).toBe("31/12/2026");
    expect(nhanNgay("hong")).toBe("hong");
  });
});

describe("tra hạng mục kế hoạch vốn", () => {
  const danhMuc: finance_hangMucRa[] = [
    // `order`, `source`, `tier` có mặt vì hợp đồng đòi — chúng không tham gia phép tra ở đây, và
    // chính vì thế chúng được đặt ĐÚNG như máy chủ trả về chứ không phải giá trị tuỳ tiện: một
    // dữ liệu mẫu nói sai về tầng của một dòng là một dữ liệu mẫu sẽ bị chép sang bài test khác.
    {
      id: "01JHM1",
      code: "tra-no",
      label: "Vốn trả nợ",
      is_default: false,
      active: true,
      order: 1,
      source: "he-thong",
      tier: 2,
    },
    {
      id: "01JHM2",
      code: "xay-moi",
      label: "",
      is_default: false,
      active: false,
      order: 2,
      source: "don-vi",
      tier: 1,
    },
  ];

  it("tra được thì hiện nhãn", () => {
    expect(nhanHangMuc(hangMucDuAn("01JHM1", danhMuc))).toBe("Vốn trả nợ");
    expect(lopHangMuc(hangMucDuAn("01JHM1", danhMuc))).toBeUndefined();
  });

  it("dự án chưa gắn hạng mục KHÁC HẲN hạng mục không tra được", () => {
    // Gộp hai ca là báo sai: một bên là việc của cán bộ nhập liệu, bên kia là dữ liệu lệch mà
    // chỉ người quản trị sửa được.
    const chuaGan = hangMucDuAn("", danhMuc);
    const chiCoMa = hangMucDuAn("01JKHONGCO", danhMuc);

    expect(nhanHangMuc(chuaGan)).toBe("Chưa gắn hạng mục");
    expect(nhanHangMuc(chiCoMa)).toContain("01JKHONGCO");
    expect(lopHangMuc(chuaGan)).not.toBe(lopHangMuc(chiCoMa));
  });

  it("danh mục RỖNG (trạng thái hôm nay của mọi xã) thì hiện mã kèm lời giải thích", () => {
    // `GET /api/v1/capital-plan-categories` cố ý ship rỗng cho tới khi có bước khởi tạo xã.
    expect(nhanHangMuc(hangMucDuAn("01JHM1", []))).toBe(
      "01JHM1 (không tra được trong danh mục hạng mục)",
    );
  });

  it("nhãn rỗng trong danh mục cũng rơi vào ca 'chỉ có mã', không thành ô trống", () => {
    expect(nhanHangMuc(hangMucDuAn("01JHM2", danhMuc))).toContain("01JHM2");
  });
});

describe("ngưỡng cảnh báo chậm", () => {
  it("hiện ngưỡng MÁY CHỦ ĐÃ ÁP DỤNG, để người đọc biết chữ 'chậm' đang đo bằng gì", () => {
    expect(nhanNguongCham(1000)).toBe("Ngưỡng cảnh báo chậm: 10,00 điểm");
  });
});
