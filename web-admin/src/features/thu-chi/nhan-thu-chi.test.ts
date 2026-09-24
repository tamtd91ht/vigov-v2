import { describe, expect, it } from "vitest";

import type { finance_dongRa } from "@/lib/api/schema.gen";

import {
  boCotKhoiDiem,
  docSoNhap,
  dongPhuTieuDe,
  dungCay,
  moiDongCoCon,
  nhanBoDem,
  nhanCachTinh,
  nhanChiSo,
  nhanNgayLuyKe,
  nhanPhanVan,
  nhanSoTien,
  nhanSoTienChiSo,
  nhanTab,
  O_TRONG,
  phangCay,
  suaDuocOSo,
} from "./nhan-thu-chi";

function dong(sua: Partial<finance_dongRa> = {}): finance_dongRa {
  return {
    id: "01JDONG",
    no: "A",
    name: "CHI NGÂN SÁCH NHÀ NƯỚC",
    order: 1,
    method: "manual",
    level: 0,
    is_headline: false,
    values: {},
    ...sua,
  };
}

describe("định dạng số", () => {
  it("ô trống hiện dấu gạch, KHÔNG BAO GIỜ số 0", () => {
    // §9 quy tắc 4. `0` là một khẳng định ("đã bố trí, chưa chi đồng nào"); `—` là "chưa ai ghi".
    expect(nhanSoTien(null)).toBe(O_TRONG);
    expect(nhanSoTien(0)).not.toBe(O_TRONG);
  });

  it("IN THẲNG SỐ MÁY CHỦ TRẢ, không chia cho một hệ số nào", () => {
    // Câu khách CHƯA CHỐT: `unit` là chuỗi tự do ("Triệu đồng") và hợp đồng không kèm hệ số quy
    // đổi. Chia cho 1e6 mỗi khi chuỗi tình cờ đọc là "Triệu đồng" làm một xã ghi `unit` khác đi
    // thấy số lệch MỘT TRIỆU LẦN, và không bài kiểm nào đỏ.
    expect(nhanSoTien(3794740)).toBe("3.794.740");
    expect(nhanSoTien(3463459.2)).toBe("3.463.459,2");
  });

  it("số âm hiện nguyên là số âm", () => {
    expect(nhanSoTien(-10000)).toBe("-10.000");
  });

  it("`basis_points` là PHẦN VẠN: 10811 ⇒ 108,11%", () => {
    expect(nhanPhanVan(10811)).toBe("108,11%");
    expect(nhanPhanVan(9130)).toBe("91,30%");
  });

  it("phần trăm `null` KHÔNG thành 0%", () => {
    expect(nhanPhanVan(null)).toBe(O_TRONG);
  });
});

describe("chỉ số vắng số thì nói ra CÂU CỦA MÁY CHỦ", () => {
  it("không có `basis_points` thì hiện `unavailable_reason` nguyên văn, không hiện 0%", () => {
    const cau = "ngan_sach: bảng chưa có dòng nào được đánh dấu là dòng tổng";
    expect(nhanChiSo({ name: "Chi đạt dự toán", basis_points: null, unavailable_reason: cau })).toBe(
      cau,
    );
  });

  it("số tiền vắng cũng vậy — `Cân đối thu - chi` không bao giờ in thành 0", () => {
    const cau = "ngan_sach: xã chưa có bảng ngân sách cho năm và loại này";
    expect(nhanSoTienChiSo({ amount: null, unavailable_reason: cau })).toBe(cau);
    expect(nhanSoTienChiSo({ amount: 0 })).toBe("0");
  });
});

describe("cách tính — HAI giá trị, và không ai chọn nó", () => {
  it("hai mã của máy chủ đọc thành hai nhãn của đặc tả", () => {
    expect(nhanCachTinh("manual")).toBe("Nhập trực tiếp");
    expect(nhanCachTinh("children")).toBe("Cộng khoản mục con");
  });

  it("`entries` KHÔNG có nhãn — chế độ ấy không tồn tại ở máy chủ", () => {
    // §4.2 vẽ ba chế độ; `dot_thu_chi` không có bảng và migration 0006 chỉ nhận hai. Dịch `entries`
    // thành "Cộng theo đợt" ở đây là gọi tên một tính năng luôn báo 0.
    expect(nhanCachTinh("entries")).toBe("entries");
  });

  it("chỉ dòng `manual` mới mở ô số", () => {
    expect(suaDuocOSo("manual")).toBe(true);
    expect(suaDuocOSo("children")).toBe(false);
  });
});

describe("đọc một ô số vừa gõ — ba kết quả, không hai", () => {
  it("ô để trống nghĩa là XOÁ TRẮNG, không phải lỗi và không phải số 0", () => {
    expect(docSoNhap("")).toEqual({ loai: "trong" });
    expect(docSoNhap("   ")).toEqual({ loai: "trong" });
  });

  it("số nguyên đọc được, kể cả số âm", () => {
    expect(docSoNhap("977310")).toEqual({ loai: "so", gia: 977310 });
    expect(docSoNhap("-5")).toEqual({ loai: "so", gia: -5 });
  });

  it("chữ và số lẻ là LỖI, không bị làm tròn thầm và không bị đọc thành ô trống", () => {
    // Giá trị trên dây là `int64` đồng. Một lần gõ hỏng bị đọc thành "trống" sẽ lặng lẽ xoá một
    // con số ngân sách đang có.
    expect(docSoNhap("mười")).toEqual({ loai: "loi" });
    expect(docSoNhap("1,5")).toEqual({ loai: "loi" });
    expect(docSoNhap("1.5")).toEqual({ loai: "loi" });
  });
});

describe("dựng cây khoản mục", () => {
  const phang: finance_dongRa[] = [
    dong({ id: "A", name: "A", parent_id: undefined }),
    dong({ id: "I", name: "I", parent_id: "A" }),
    dong({ id: "1.1", name: "1.1", parent_id: "I" }),
    dong({ id: "B", name: "B", parent_id: undefined }),
  ];

  it("dựng đúng hình cây theo `parent_id`", () => {
    const cay = dungCay(phang);
    expect(cay.map((n) => n.dong.id)).toEqual(["A", "B"]);
    expect(cay[0]?.con.map((n) => n.dong.id)).toEqual(["I"]);
    expect(cay[0]?.con[0]?.con.map((n) => n.dong.id)).toEqual(["1.1"]);
  });

  it("KHÔNG DÒNG NÀO BIẾN MẤT: dòng trỏ tới cha không có trong tập được treo ở gốc", () => {
    // Một khoản mục rơi khỏi màn hình là một con số biến mất khỏi báo cáo, và không có gì trên màn
    // hình nói ra rằng nó đã biến mất.
    const cay = dungCay([...phang, dong({ id: "MO_COI", parent_id: "KHONG_CO" })]);
    const moiID = phangCay(cay, new Set()).map((d) => d.dong.id);
    expect(moiID).toContain("MO_COI");
    expect(moiID).toHaveLength(5);
  });

  it("vòng lặp cha - con KHÔNG làm treo tab, và cũng không nuốt dòng nào", () => {
    const vong = [
      dong({ id: "X", parent_id: "Y" }),
      dong({ id: "Y", parent_id: "X" }),
    ];
    const moiID = phangCay(dungCay(vong), new Set()).map((d) => d.dong.id);
    expect(moiID.slice().sort()).toEqual(["X", "Y"]);
  });

  it("thu gọn một dòng thì con của nó KHÔNG có mặt, và bộ đếm nói đúng con số", () => {
    const cay = dungCay(phang);
    const hien = phangCay(cay, new Set(["A"]));
    expect(hien.map((d) => d.dong.id)).toEqual(["A", "B"]);
    expect(nhanBoDem(hien.length, phang.length)).toBe("đang hiện 2/4 khoản mục");
  });

  it("`› Chỉ xem mục lớn` thu gọn đúng những dòng CÓ con", () => {
    expect([...moiDongCoCon(phang)].sort()).toEqual(["A", "I"]);
  });

  it("độ sâu vẽ ra đếm từ cây, nên dòng mồ côi treo ở gốc có cấp 0", () => {
    const hien = phangCay(dungCay([...phang, dong({ id: "MO_COI", parent_id: "KHONG_CO", level: 7 })]), new Set());
    expect(hien.find((d) => d.dong.id === "MO_COI")?.cap).toBe(0);
    expect(hien.find((d) => d.dong.id === "1.1")?.cap).toBe(2);
  });
});

describe("thẻ tiêu đề báo cáo", () => {
  it("ngày luỹ kế in KHÔNG đệm số 0, đúng chữ §2 in ra", () => {
    expect(nhanNgayLuyKe("2026-08-25")).toBe("25/8/2026");
  });

  it("chuỗi không đúng khuôn hợp đồng hiện NGUYÊN VĂN, không đoán thành một ngày khác", () => {
    expect(nhanNgayLuyKe("25/8/2026")).toBe("25/8/2026");
  });

  it("dòng phụ gộp đơn vị tính, mốc luỹ kế và số khoản mục", () => {
    const bang = {
      id: "01JBANG",
      code: "NS-2026-CHI-01",
      year: 2026,
      kind: "chi",
      revision: 1,
      title: "BÁO CÁO CHI NGÂN SÁCH",
      unit: "Triệu đồng",
      unit_label: "Triệu đồng",
      cumulative_to: "2026-08-25",
    };
    expect(dongPhuTieuDe(bang, 59)).toBe("Đơn vị tính: Triệu đồng · Luỹ kế đến 25/8/2026 · 59 khoản mục");
  });

  it("chưa có mốc luỹ kế thì KHÔNG in một mốc rỗng", () => {
    const bang = {
      id: "01JBANG",
      code: "NS-2026-THU-01",
      year: 2026,
      kind: "thu",
      revision: 1,
      title: "THU NGÂN SÁCH",
      unit: "Triệu đồng",
      unit_label: "Triệu đồng",
    };
    expect(dongPhuTieuDe(bang, 52)).toBe("Đơn vị tính: Triệu đồng · 52 khoản mục");
  });
});

describe("nhãn tab và bộ cột khởi điểm", () => {
  it("nhãn tab mang NĂM ĐANG CHỌN, đúng §2", () => {
    expect(nhanTab("chi", 2026)).toBe("Chi ngân sách 2026");
    expect(nhanTab("thu", 2025)).toBe("Thu ngân sách 2025");
  });

  it("tên cột của bảng thu dựng TỪ năm, không phải một chuỗi có sẵn số 2026", () => {
    expect(boCotKhoiDiem("thu", 2027)[0]?.name).toBe("Dự toán 2027 TP giao");
  });

  it("cột `so` mang vai trò và KHÔNG mang công thức; cột `phan_tram` thì ngược lại", () => {
    // `KiemTraCot` từ chối cả hai chiều: `ErrThuaCongThuc` trên cột số, `ErrVaiTroTrenCotPhanTram`
    // trên cột phần trăm.
    for (const c of boCotKhoiDiem("chi", 2026)) {
      if (c.type === "so") {
        expect(c.formula).toBeUndefined();
      } else {
        expect(c.role).toBeUndefined();
        expect(c.formula).not.toBe("");
      }
    }
  });

  it("bảng chi có đủ HAI vai trò của chỉ số `Chi đạt dự toán`", () => {
    // Bảng lập thiếu vai trò là bảng mà chỉ số trống suốt năm (ADR 0035 §A).
    const vaiTro = boCotKhoiDiem("chi", 2026).map((c) => c.role);
    expect(vaiTro).toContain("du-toan-nam");
    expect(vaiTro).toContain("chi-ngan-sach");
  });

  it("bảng thu có đủ BỐN vai trò, kể cả `thu-xa-huong` mà `Cân đối thu - chi` đọc từ đó", () => {
    const vaiTro = boCotKhoiDiem("thu", 2026).map((c) => c.role);
    expect(vaiTro).toContain("du-toan-tp-giao");
    expect(vaiTro).toContain("du-toan-xa-giao");
    expect(vaiTro).toContain("thu-nsnn");
    expect(vaiTro).toContain("thu-xa-huong");
  });
});
