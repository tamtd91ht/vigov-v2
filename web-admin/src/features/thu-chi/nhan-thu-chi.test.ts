import { describe, expect, it } from "vitest";

import type { finance_bangRa, finance_cotRa, finance_dongRa } from "@/lib/api/schema.gen";

import {
  boCotKhoiDiem,
  canhBaoDoiCachTinh,
  cauDieuKienDot,
  cauQuyDoi,
  chonDuocCachTinh,
  docSoNhap,
  donViCuaBang,
  dongPhuTieuDe,
  dongSangChuoi,
  dungThanDot,
  dungThanSuaBang,
  khoaSauLanGhi,
  NHAN_CHENH_LECH,
  PHAN_CHUA_DUNG,
  tieuDeHopDot,
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

function bangRa(sua: Partial<finance_bangRa> = {}): finance_bangRa {
  return {
    id: "01JBANG",
    code: "NS-2026-CHI-01",
    year: 2026,
    kind: "chi",
    revision: 1,
    title: "BÁO CÁO CHI NGÂN SÁCH",
    unit: "trieu-dong",
    unit_label: "Triệu đồng",
    cumulative_to: "2026-08-25",
    ...sua,
  };
}

describe("định dạng số — máy chủ gửi ĐỒNG, màn hình chia theo đơn vị của bảng", () => {
  it("ô trống hiện dấu gạch, KHÔNG BAO GIỜ số 0", () => {
    // §9 quy tắc 4. `0` là một khẳng định ("đã bố trí, chưa chi đồng nào"); `—` là "chưa ai ghi".
    expect(nhanSoTien(null, "trieu-dong")).toBe(O_TRONG);
    expect(nhanSoTien(0, "trieu-dong")).toBe("0");
  });

  it("3463459200000 đồng in thành 3.463.459,2 triệu — đúng chữ §2 in, không phải số đồng thô", () => {
    // Trước quyết định 25/09/2026 màn hình in số đồng thô cạnh nhãn "Triệu đồng": lệch 10⁶ lần.
    expect(nhanSoTien(3463459200000, "trieu-dong")).toBe("3.463.459,2");
    expect(nhanSoTien(3463459200000, "nghin-dong")).toBe("3.463.459.200");
    expect(nhanSoTien(3463459200000, "dong")).toBe("3.463.459.200.000");
  });

  it("hiển thị CHÍNH XÁC tới từng đồng, bỏ số 0 cuối, không làm tròn về hai chữ số", () => {
    expect(nhanSoTien(3794740000000, "trieu-dong")).toBe("3.794.740");
    expect(nhanSoTien(1234567, "trieu-dong")).toBe("1,234567");
    expect(nhanSoTien(1, "trieu-dong")).toBe("0,000001");
    expect(nhanSoTien(1500, "nghin-dong")).toBe("1,5");
  });

  it("số âm hiện nguyên là số âm", () => {
    expect(nhanSoTien(-10000000000, "trieu-dong")).toBe("-10.000");
    expect(nhanSoTien(-500000, "trieu-dong")).toBe("-0,5");
  });

  it("giá trị không phải số nguyên đồng là hợp đồng hỏng: nói ra, không làm tròn", () => {
    expect(nhanSoTien(3463459.2, "dong")).toBe("Không đọc được");
    expect(nhanSoTien(Number.NaN, "dong")).toBe("Không đọc được");
  });

  it("`basis_points` là PHẦN VẠN: 10811 ⇒ 108,11% — tỷ lệ KHÔNG bị đơn vị tính chạm tới", () => {
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

  it("số tiền vắng cũng vậy — con số chênh lệch không bao giờ in thành 0", () => {
    const cau = "ngan_sach: xã chưa có bảng ngân sách cho năm và loại này";
    expect(nhanSoTienChiSo({ amount: null, unavailable_reason: cau }, "dong")).toBe(cau);
    expect(nhanSoTienChiSo({ amount: 0 }, "dong")).toBe("0");
  });
});

describe("đơn vị của một bảng", () => {
  it("mã hợp lệ và không cảnh báo: dùng mã ấy và nhãn máy chủ gửi", () => {
    expect(donViCuaBang(bangRa())).toEqual({
      ma: "trieu-dong",
      nhan: "Triệu đồng",
      canhBao: null,
      nhanCu: null,
    });
  });

  it("có `unit_warning`: KHÔNG quy đổi — in đồng, nhãn 'đồng', và mang nguyên câu cảnh báo", () => {
    // Chia theo một hệ số đoán từ chữ cũ là hiện mọi con số lệch hàng nghìn lần.
    const canhBao = "Đơn vị tính đang lưu không thuộc danh sách — chọn lại đơn vị cho bảng.";
    const dv = donViCuaBang(bangRa({ unit: "", unit_label: "Tr.đ", unit_warning: canhBao }));
    expect(dv.ma).toBe("dong");
    expect(dv.nhan).toBe("đồng");
    expect(dv.canhBao).toBe(canhBao);
    expect(dv.nhanCu).toBe("Tr.đ");
    expect(nhanSoTien(3463459200000, dv.ma)).toBe("3.463.459.200.000");
  });

  it("mã lạ không kèm cảnh báo vẫn FAIL CLOSED về đồng, và tự nói ra", () => {
    const dv = donViCuaBang(bangRa({ unit: "ty-dong", unit_label: "Tỷ đồng" }));
    expect(dv.ma).toBe("dong");
    expect(dv.canhBao).not.toBeNull();
  });

  it("câu quy đổi nói đúng hệ số chia", () => {
    expect(cauQuyDoi(donViCuaBang(bangRa()))).toContain("1.000.000");
    expect(cauQuyDoi(donViCuaBang(bangRa({ unit: "dong", unit_label: "Đồng" })))).toContain(
      "không quy đổi",
    );
  });
});

describe("đọc một ô tiền vừa gõ — theo đơn vị của bảng, ra số nguyên đồng", () => {
  it("ô để trống nghĩa là XOÁ TRẮNG, không phải lỗi và không phải số 0", () => {
    expect(docSoNhap("", "trieu-dong")).toEqual({ loai: "trong" });
    expect(docSoNhap("   ", "trieu-dong")).toEqual({ loai: "trong" });
  });

  it('"3.463.459,2" triệu ⇄ 3463459200000 đồng, CẢ HAI CHIỀU', () => {
    expect(docSoNhap("3.463.459,2", "trieu-dong")).toEqual({ loai: "so", gia: 3463459200000 });
    expect(docSoNhap("3463459,2", "trieu-dong")).toEqual({ loai: "so", gia: 3463459200000 });
    expect(dongSangChuoi(3463459200000, "trieu-dong")).toBe("3.463.459,2");
  });

  it("KHÔNG CÓ SỐ THỰC trong phép quy đổi: những giá trị số thực làm lệch vẫn ra đúng từng đồng", () => {
    // Bằng chứng: 1.005 * 1e6 === 1004999.9999999999 và 1.015 * 1000 === 1014.9999999999999 trong JS.
    expect(1.005 * 1e6).not.toBe(1005000);
    expect(docSoNhap("1,005", "trieu-dong")).toEqual({ loai: "so", gia: 1005000 });
    expect(docSoNhap("1,015", "nghin-dong")).toEqual({ loai: "so", gia: 1015 });
    expect(docSoNhap("0,1", "trieu-dong")).toEqual({ loai: "so", gia: 100000 });
    // Lớn, sát trần số nguyên an toàn, với đủ sáu chữ số lẻ.
    expect(docSoNhap("9.007.199,254740", "trieu-dong")).toEqual({ loai: "so", gia: 9007199254740 });
  });

  it("mọi giá trị đồng đi ra màn hình rồi quay lại là ĐÚNG giá trị ấy", () => {
    for (const dong of [0, 1, 999, 1005000, 3463459200000, -1234567, 9007199254740991]) {
      for (const dv of ["dong", "nghin-dong", "trieu-dong"] as const) {
        expect(docSoNhap(dongSangChuoi(dong, dv), dv)).toEqual({ loai: "so", gia: dong });
      }
    }
  });

  it("từ chối nhiều chữ số lẻ hơn đơn vị cho phép — không làm tròn thầm", () => {
    expect(docSoNhap("1,0000001", "trieu-dong").loai).toBe("loi");
    expect(docSoNhap("1,1234", "nghin-dong").loai).toBe("loi");
    expect(docSoNhap("1,5", "dong").loai).toBe("loi");
    // Đủ số chữ số thì nhận.
    expect(docSoNhap("1,000001", "trieu-dong")).toEqual({ loai: "so", gia: 1000001 });
    expect(docSoNhap("1,123", "nghin-dong")).toEqual({ loai: "so", gia: 1123 });
  });

  it("số âm đọc được", () => {
    expect(docSoNhap("-5", "dong")).toEqual({ loai: "so", gia: -5 });
    expect(docSoNhap("-1.234,5", "trieu-dong")).toEqual({ loai: "so", gia: -1234500000 });
    expect(docSoNhap("-0", "dong")).toEqual({ loai: "so", gia: 0 });
  });

  it("chữ, dấu chấm thập phân kiểu Anh, và nhóm hàng nghìn sai là LỖI kèm câu nói vì sao", () => {
    // `1.5` không được đoán thành một phẩy năm: ở `vi-VN` dấu chấm ngăn hàng nghìn.
    for (const tho of ["mười", "1.5", "1,2,3", "12.34", "1 000", "+5", "1e6", ",5"]) {
      const kq = docSoNhap(tho, "trieu-dong");
      expect(kq.loai).toBe("loi");
      if (kq.loai === "loi") expect(kq.viSao).not.toBe("");
    }
  });

  it("vượt trần số nguyên an toàn là LỖI, không phải một số bị làm tròn", () => {
    expect(docSoNhap("9.007.199.254,740992", "trieu-dong").loai).toBe("loi");
  });
});

describe("cách tính — ba chế độ, CHỈ HAI là lựa chọn", () => {
  it("ba mã của máy chủ đọc thành ba nhãn của đặc tả §4.2", () => {
    expect(nhanCachTinh("manual")).toBe("Nhập trực tiếp");
    expect(nhanCachTinh("entries")).toBe("Cộng theo đợt");
    expect(nhanCachTinh("children")).toBe("Cộng khoản mục con");
  });

  it("ô chọn CHỈ có trên dòng lá: dòng có con, hay dòng máy chủ nói là `children`, không có", () => {
    expect(chonDuocCachTinh("manual", false)).toBe(true);
    expect(chonDuocCachTinh("entries", false)).toBe(true);
    expect(chonDuocCachTinh("manual", true)).toBe(false);
    expect(chonDuocCachTinh("children", false)).toBe(false);
    expect(chonDuocCachTinh("children", true)).toBe(false);
  });

  it("chuyển entries → manual CẢNH BÁO rằng tổng các đợt được chép vào ô số", () => {
    expect(canhBaoDoiCachTinh("manual")).toContain("chép tổng các đợt");
    expect(canhBaoDoiCachTinh("entries")).toContain("không hiện nữa");
  });

  it("chỉ dòng `manual` mới mở ô số — `entries` và `children` là chỉ đọc", () => {
    expect(suaDuocOSo("manual")).toBe(true);
    expect(suaDuocOSo("entries")).toBe(false);
    expect(suaDuocOSo("children")).toBe(false);
  });
});

describe("hộp các đợt — dựng thân POST", () => {
  const COT: finance_cotRa[] = [
    { id: "C1", name: "Dự toán năm", order: 1, type: "so" },
    { id: "C2", name: "Chi ngân sách", order: 2, type: "so" },
    { id: "C3", name: "So sánh (%)", order: 3, type: "phan_tram", formula: "col_2 / col_1 * 100" },
  ];

  function nhap(sua: Partial<Parameters<typeof dungThanDot>[0]> = {}) {
    return {
      ngay: "2026-09-25",
      noiDung: "Thu tiền sử dụng đất đợt 2",
      doiTac: "",
      soChungTu: "",
      gia: { C2: "1,005", C3: "50" },
      ...sua,
    };
  }

  it("CHỈ cột số, đã quy đổi sang đồng; cột trống là null; cột % không bao giờ đi lên", () => {
    const kq = dungThanDot(nhap(), COT, "trieu-dong");
    expect(kq).toEqual({
      ok: true,
      than: {
        date: "2026-09-25",
        content: "Thu tiền sử dụng đất đợt 2",
        values: { C1: null, C2: 1005000 },
      },
    });
  });

  it("trường chữ tuỳ chọn để trống thì KHÔNG có mặt trong thân", () => {
    const kq = dungThanDot(nhap(), COT, "trieu-dong");
    expect(kq.ok && "counterparty" in kq.than).toBe(false);
    expect(kq.ok && "document_no" in kq.than).toBe(false);
  });

  it("có đơn vị, cá nhân và số chứng từ thì mang lên, đã cắt khoảng trắng", () => {
    const kq = dungThanDot(nhap({ doiTac: " Công ty A ", soChungTu: " PT-12 " }), COT, "trieu-dong");
    expect(kq.ok && kq.than.counterparty).toBe("Công ty A");
    expect(kq.ok && kq.than.document_no).toBe("PT-12");
  });

  it("phải có ít nhất một số tiền", () => {
    expect(dungThanDot(nhap({ gia: {} }), COT, "trieu-dong").ok).toBe(false);
  });

  it("một ô tiền hỏng chặn cả lần ghi và gọi đúng tên cột", () => {
    const kq = dungThanDot(nhap({ gia: { C1: "1.5" } }), COT, "trieu-dong");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("Dự toán năm");
  });

  it("chặn trường vượt độ dài máy chủ đặt: nội dung 1000, đơn vị cá nhân 300, số chứng từ 100", () => {
    expect(dungThanDot(nhap({ noiDung: "a".repeat(1001) }), COT, "dong").ok).toBe(false);
    expect(dungThanDot(nhap({ noiDung: "a".repeat(1000) }), COT, "dong").ok).toBe(false); // C2 "1,005" lẻ với đồng
    expect(dungThanDot(nhap({ noiDung: "a".repeat(1000), gia: { C1: "5" } }), COT, "dong").ok).toBe(true);
    expect(dungThanDot(nhap({ doiTac: "a".repeat(301), gia: { C1: "5" } }), COT, "dong").ok).toBe(false);
    expect(dungThanDot(nhap({ soChungTu: "a".repeat(101), gia: { C1: "5" } }), COT, "dong").ok).toBe(false);
  });

  it("thiếu ngày hoặc thiếu nội dung là lỗi", () => {
    expect(dungThanDot(nhap({ ngay: "" }), COT, "trieu-dong").ok).toBe(false);
    expect(dungThanDot(nhap({ noiDung: "   " }), COT, "trieu-dong").ok).toBe(false);
  });

  it("khoá chống trùng: GIỮ khi gửi lại sau lỗi, THAY MỚI sau một lần thành công", () => {
    let dem = 0;
    const sinh = () => `khoa-${++dem}`;
    expect(khoaSauLanGhi("khoa-dau", false, sinh)).toBe("khoa-dau");
    expect(khoaSauLanGhi("khoa-dau", false, sinh)).toBe("khoa-dau");
    expect(dem).toBe(0);
    expect(khoaSauLanGhi("khoa-dau", true, sinh)).toBe("khoa-1");
  });

  it("tiêu đề hộp là tên khoản mục VIẾT HOA, kể cả chữ Việt", () => {
    expect(tieuDeHopDot("Thu tiền sử dụng đất")).toBe("THU TIỀN SỬ DỤNG ĐẤT");
  });

  it("hộp nói ra rằng đợt chỉ được cộng khi Cách tính là Cộng theo đợt", () => {
    expect(cauDieuKienDot("manual")).toContain("Cộng theo đợt");
    expect(cauDieuKienDot("manual")).toContain("chưa được cộng");
    expect(cauDieuKienDot("entries")).not.toContain("chưa được cộng");
  });
});

describe("sửa thông tin bảng — CHỈ những trường thật sự đổi", () => {
  it("đổi tiêu đề thì thân chỉ có `title`", () => {
    expect(
      dungThanSuaBang(bangRa(), { tieuDe: "TIÊU ĐỀ MỚI", luyKe: "2026-08-25", donVi: "trieu-dong" }),
    ).toEqual({ ok: true, than: { title: "TIÊU ĐỀ MỚI" } });
  });

  it("để trống luỹ kế khi bảng đang có mốc là `\"\"` — BỎ mốc", () => {
    expect(
      dungThanSuaBang(bangRa(), { tieuDe: "BÁO CÁO CHI NGÂN SÁCH", luyKe: "", donVi: "trieu-dong" }),
    ).toEqual({ ok: true, than: { cumulative_to: "" } });
  });

  it("đổi đơn vị gửi MÃ, không gửi nhãn", () => {
    expect(
      dungThanSuaBang(bangRa(), {
        tieuDe: "BÁO CÁO CHI NGÂN SÁCH",
        luyKe: "2026-08-25",
        donVi: "nghin-dong",
      }),
    ).toEqual({ ok: true, than: { unit: "nghin-dong" } });
    expect(
      dungThanSuaBang(bangRa(), { tieuDe: "X", luyKe: "", donVi: "Triệu đồng" }).ok,
    ).toBe(false);
  });

  it("không bao giờ mang `year`, `kind`, `code` hay `columns` — máy chủ trả 400 cho cả bốn", () => {
    const kq = dungThanSuaBang(bangRa(), { tieuDe: "Y", luyKe: "2026-09-30", donVi: "dong" });
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(Object.keys(kq.than).sort()).toEqual(["cumulative_to", "title", "unit"]);
  });

  it("không đổi gì thì không gửi", () => {
    expect(
      dungThanSuaBang(bangRa(), {
        tieuDe: "BÁO CÁO CHI NGÂN SÁCH",
        luyKe: "2026-08-25",
        donVi: "trieu-dong",
      }).ok,
    ).toBe(false);
  });

  it("tiêu đề trống là lỗi", () => {
    expect(dungThanSuaBang(bangRa(), { tieuDe: "  ", luyKe: "", donVi: "dong" }).ok).toBe(false);
  });
});

describe("thẻ chỉ số và danh sách phần chưa dựng", () => {
  it("con số chênh lệch mang ĐÚNG tên khách chốt, không phải cân đối, bội chi hay thâm hụt", () => {
    expect(NHAN_CHENH_LECH).toBe("Chênh lệch thu – chi luỹ kế");
    expect(NHAN_CHENH_LECH).not.toMatch(/bội chi|thâm hụt|cân đối/i);
  });

  it("danh sách chưa dựng KHÔNG còn đợt, Cách tính, hay sửa Luỹ kế — ba thứ ấy đã dựng", () => {
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join(" | ");
    expect(ten).not.toMatch(/đợt/i);
    expect(ten).not.toMatch(/Cách tính/);
    expect(ten).not.toMatch(/Luỹ kế/);
    expect(ten).toMatch(/Excel/);
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
      unit: "trieu-dong",
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
      unit: "trieu-dong",
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
