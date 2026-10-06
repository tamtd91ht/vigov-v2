import { describe, expect, it } from "vitest";

import {
  ALLOCATION_NO_AMOUNT,
  ALLOCATION_NO_SOURCE,
  allocationBlocksSave,
  allocationLines,
  sameAllocations,
  summarizeAllocations,
  type AllocationRow,
  CANH_BAO_SUA_VE_NHAP,
  CAU_KHONG_CO_GI_DOI,
  CAU_SO_TIEN_KHONG_DOC_DUOC,
  CAU_SO_TIEN_PHAI_DUONG,
  CAU_SO_TIEN_VUOT_CHINH_XAC,
  CAU_THIEU_HANG_MUC,
  CAU_KHOI_CONG_SAU_HOAN_THANH,
  CAU_THIEU_MA_DU_AN,
  CAU_THIEU_NGAY_CHI,
  CAU_THIEU_NOI_DUNG,
  CHUNG_TU_DA_KHOA,
  CHUNG_TU_DA_XAC_NHAN,
  CHUNG_TU_KE_TOAN_NHAP,
  DAU_GACH,
  docSoTien,
  FORM_CHUNG_TU_TRONG,
  FORM_DU_AN_TRONG,
  allocationOptionLabel,
  initialFundingSource,
  lopTrangThaiChungTu,
  MISSING_FUNDING_SOURCE,
  nhanMocKhoa,
  nhanTrangThaiChungTu,
  thaoTacChungTu,
  thanSuaChungTu,
  thanSuaDuAn,
  thanThemChungTu,
  thanThemDuAn,
  type GiaTriFormChungTu,
  type GiaTriFormDuAn,
} from "./nhan-ghi-giai-ngan";

/**
 * Những QUYẾT ĐỊNH của phần ghi phân hệ Giải ngân, kiểm ở dạng hàm thuần.
 *
 * NHÓM ĐẮT NHẤT Ở ĐÂY LÀ `docSoTien` VÀ HAI HÀM DELTA:
 *
 *   - `docSoTien` là chỗ duy nhất một con số tiền đi từ chữ sang số. Một chuỗi mười tám chữ số đi
 *     qua `Number(...)` cho ra một con số KHÁC trong im lặng, và con số khác ấy được ghi vào hồ sơ
 *     lưu trữ mà không có gì đỏ ở bất kỳ đâu.
 *   - Hai hàm delta quyết định `PATCH` có được gửi hay không. Một `PATCH` thừa ở tuyến chứng từ
 *     BÓC MẤT chữ xác nhận của lãnh đạo (ADR 0036), nên "không đổi gì thì không gửi" không phải
 *     tối ưu, nó là một quy tắc nghiệp vụ.
 */

const CHUNG_TU_MAU: GiaTriFormChungTu = {
  ngayChi: "2026-09-07",
  soTien: "30000000",
  noiDung: "Thanh toán đợt 3",
  doiTac: "Công ty ABC",
  soChungTu: "CT-2026-0912",
  fundingSourceId: "",
};

/** A project with no allocation line: no source select, no source sent. */
const NO_LINES: readonly string[] = [];
/** A project drawing on two sources. */
const TWO_LINES: readonly string[] = ["S1", "S2"];

const DU_AN_MAU: GiaTriFormDuAn = {
  ma: "DA01",
  hangMucID: "01JHM1",
  ten: "Bê tông hoá đường trục chính thôn Hà Lam",
  moTa: "Tuyến 1,2 km",
  keHoachVon: "7500000000",
  tongMucDuyet: "9000000000",
  ngayKhoiCong: "2026-03-01",
  ngayHoanThanh: "2026-11-30",
  thoiHanGiaiNgan: "2026-12-31",
  allocations: [],
};

describe("docSoTien — tiền vào hệ thống bằng đúng con số người ta gõ", () => {
  it("chữ số thuần, và dấu chấm phân cách hàng nghìn là cách viết thông thường", () => {
    expect(docSoTien("30000000")).toEqual({ loai: "so", dong: 30000000 });
    expect(docSoTien("100.000.000")).toEqual({ loai: "so", dong: 100000000 });
    expect(docSoTien("  7 500 000 000 ")).toEqual({ loai: "so", dong: 7500000000 });
    expect(docSoTien("0")).toEqual({ loai: "so", dong: 0 });
  });

  it("DẤU PHẨY KHÔNG ĐƯỢC BỎ — ở vi-VN nó là dấu thập phân", () => {
    // "1,5" lặng lẽ thành 15 là một khoản chi sai gấp mười lần.
    expect(docSoTien("1,5")).toEqual({ loai: "khongPhaiSo" });
    expect(docSoTien("1.5e9")).toEqual({ loai: "khongPhaiSo" });
    expect(docSoTien("-30000")).toEqual({ loai: "khongPhaiSo" });
    expect(docSoTien("ba mươi triệu")).toEqual({ loai: "khongPhaiSo" });
  });

  it("ô trống là ca RIÊNG, không phải số 0", () => {
    expect(docSoTien("")).toEqual({ loai: "trong" });
    expect(docSoTien("   ")).toEqual({ loai: "trong" });
  });

  it("VƯỢT SỐ NGUYÊN AN TOÀN thì TỪ CHỐI, không làm tròn im lặng", () => {
    // Trần của máy chủ là 10^17 đồng, số nguyên an toàn của JS dừng ở 2^53-1 ≈ 9,007×10^15.
    expect(docSoTien("9007199254740991")).toEqual({ loai: "so", dong: 9007199254740991 });
    expect(docSoTien("9007199254740993")).toEqual({ loai: "vuotChinhXac" });
    expect(docSoTien("100000000000000000")).toEqual({ loai: "vuotChinhXac" });
  });
});

describe("thaoTacChungTu — vòng đời là một dây xích", () => {
  it("`Kế toán nhập`: sửa · gỡ · xác nhận. KHÔNG khoá, KHÔNG mở khoá", () => {
    expect(thaoTacChungTu(CHUNG_TU_KE_TOAN_NHAP)).toEqual({
      sua: true,
      go: true,
      xacNhan: true,
      khoa: false,
      moKhoa: false,
    });
  });

  it("`Đã xác nhận`: sửa (về nháp) · gỡ · khoá. KHÔNG xác nhận lại", () => {
    expect(thaoTacChungTu(CHUNG_TU_DA_XAC_NHAN)).toEqual({
      sua: true,
      go: true,
      xacNhan: false,
      khoa: true,
      moKhoa: false,
    });
  });

  it("`Đã khoá`: CHỈ mở khoá — không sửa, không gỡ", () => {
    expect(thaoTacChungTu(CHUNG_TU_DA_KHOA)).toEqual({
      sua: false,
      go: false,
      xacNhan: false,
      khoa: false,
      moKhoa: true,
    });
  });

  it("trạng thái LẠ thì ĐÓNG HẾT — fail closed", () => {
    // Một trạng thái thứ tư máy chủ thêm vào mai này không được thừa hưởng bộ nút của trạng thái
    // nào cả (luật 1, cấm #1).
    expect(thaoTacChungTu("da-quyet-toan")).toEqual({
      sua: false,
      go: false,
      xacNhan: false,
      khoa: false,
      moKhoa: false,
    });
    expect(thaoTacChungTu("")).toEqual({
      sua: false,
      go: false,
      xacNhan: false,
      khoa: false,
      moKhoa: false,
    });
  });
});

describe("Nhãn trạng thái", () => {
  it("ba nhãn đúng chữ đặc tả §8.2", () => {
    expect(nhanTrangThaiChungTu(CHUNG_TU_KE_TOAN_NHAP)).toBe("Kế toán nhập");
    expect(nhanTrangThaiChungTu(CHUNG_TU_DA_XAC_NHAN)).toBe("Đã xác nhận");
    expect(nhanTrangThaiChungTu(CHUNG_TU_DA_KHOA)).toBe("Đã khoá");
  });

  it("trạng thái lạ hiện NGUYÊN chuỗi máy chủ gửi", () => {
    expect(nhanTrangThaiChungTu("da-quyet-toan")).toBe("da-quyet-toan");
    expect(lopTrangThaiChungTu("da-quyet-toan")).toBe("chip");
  });

  it("ba trạng thái ba lớp KHÁC NHAU — không hai trạng thái nào nhìn giống nhau", () => {
    const lop = [CHUNG_TU_KE_TOAN_NHAP, CHUNG_TU_DA_XAC_NHAN, CHUNG_TU_DA_KHOA].map(
      lopTrangThaiChungTu,
    );
    expect(new Set(lop).size).toBe(3);
  });
});

describe("nhanMocKhoa — múi giờ GHIM", () => {
  it("in theo giờ Việt Nam dù máy chạy ở UTC", () => {
    // `vitest.config.mts` ghim `TZ=UTC` đúng để ca này đỏ nếu ai đó bỏ `timeZone` khỏi phép định
    // dạng: 2026-09-22T07:05:00Z là 14:05 ngày 22/09 ở Asia/Ho_Chi_Minh.
    expect(nhanMocKhoa("2026-09-22T07:05:00Z")).toBe("14:05 22/09/2026");
  });

  it("qua nửa đêm: mốc UTC tối 21/09 là RẠNG SÁNG 22/09 ở Việt Nam", () => {
    expect(nhanMocKhoa("2026-09-21T18:30:00Z")).toBe("01:30 22/09/2026");
  });

  it("vắng mặt là dấu gạch, chuỗi hỏng hiện nguyên văn", () => {
    expect(nhanMocKhoa(undefined)).toBe(DAU_GACH);
    expect(nhanMocKhoa("")).toBe(DAU_GACH);
    expect(nhanMocKhoa("hôm qua")).toBe("hôm qua");
  });
});

describe("thanThemChungTu", () => {
  it("dựng đủ trường, cắt khoảng trắng, và mang `project_id` của TRANG", () => {
    const kq = thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, noiDung: "  Thanh toán đợt 3  " }, NO_LINES);

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.than).toEqual({
      project_id: "01JDA1",
      payment_date: "2026-09-07",
      amount: 30000000,
      description: "Thanh toán đợt 3",
      counterparty: "Công ty ABC",
      voucher_no: "CT-2026-0912",
    });
  });

  it("ô tuỳ chọn để trống thì VẮNG MẶT, không gửi chuỗi rỗng", () => {
    const kq = thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, doiTac: "  ", soChungTu: "" }, NO_LINES);

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.than.counterparty).toBeUndefined();
    expect(kq.than.voucher_no).toBeUndefined();
  });

  it("KHÔNG BAO GIỜ mang `status`: vòng đời không do client đặt", () => {
    const kq = thanThemChungTu("01JDA1", CHUNG_TU_MAU, NO_LINES);

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(Object.keys(kq.than)).not.toContain("status");
  });

  it("từ chối ngày chi trống, nội dung trống, số tiền không đọc được, số tiền 0", () => {
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, ngayChi: "" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_THIEU_NGAY_CHI,
    });
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, noiDung: "   " }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_THIEU_NOI_DUNG,
    });
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, soTien: "1,5" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_SO_TIEN_KHONG_DOC_DUOC,
    });
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, soTien: "" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_SO_TIEN_KHONG_DOC_DUOC,
    });
    // `amount` phải lớn hơn 0 đồng ở máy chủ, và đây là câu nói ra điều đó trước.
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, soTien: "0" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_SO_TIEN_PHAI_DUONG,
    });
    expect(thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, soTien: "100000000000000000" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_SO_TIEN_VUOT_CHINH_XAC,
    });
  });

  it("biểu mẫu trống không dựng được thân nào", () => {
    expect(thanThemChungTu("01JDA1", FORM_CHUNG_TU_TRONG, NO_LINES).ok).toBe(false);
  });
});

describe("voucher funding source — decided by the project's allocation lines (06/10/2026)", () => {
  it("project with lines: source REQUIRED on add, refused before the server's 409", () => {
    expect(thanThemChungTu("01JDA1", CHUNG_TU_MAU, TWO_LINES)).toEqual({ ok: false, cau: MISSING_FUNDING_SOURCE });
    const kq = thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, fundingSourceId: "S2" }, TWO_LINES);
    expect(kq.ok && kq.than.funding_source_id).toBe("S2");
  });

  it("project with NO line: nothing sent, even if a value lingers", () => {
    const kq = thanThemChungTu("01JDA1", { ...CHUNG_TU_MAU, fundingSourceId: "S1" }, NO_LINES);
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect("funding_source_id" in kq.than && kq.than.funding_source_id !== undefined).toBe(false);
    expect(JSON.stringify(kq.than)).not.toContain("funding_source_id");
  });

  it("PATCH omits an UNCHANGED source — even a legacy empty one on a project with lines", () => {
    const filed = { ...CHUNG_TU_MAU, fundingSourceId: "S1" };
    expect(thanSuaChungTu(filed, { ...filed, noiDung: "Đợt 4" }, TWO_LINES)).toEqual({
      ok: true,
      than: { description: "Đợt 4" },
    });
    // Entered before the project declared sources: still editable without being forced onto one.
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, noiDung: "Đợt 4" }, TWO_LINES)).toEqual({
      ok: true,
      than: { description: "Đợt 4" },
    });
  });

  it("PATCH sends a CHANGED source, and refuses clearing it on a project with lines", () => {
    const filed = { ...CHUNG_TU_MAU, fundingSourceId: "S1" };
    expect(thanSuaChungTu(filed, { ...filed, fundingSourceId: "S2" }, TWO_LINES)).toEqual({
      ok: true,
      than: { funding_source_id: "S2" },
    });
    expect(thanSuaChungTu(filed, { ...filed, fundingSourceId: "" }, TWO_LINES)).toEqual({
      ok: false,
      cau: MISSING_FUNDING_SOURCE,
    });
  });

  it("option label: name — còn (allocated − drawn), short form; overdrawn stays negative", () => {
    const line = { funding_source_id: "S1", name: "Ngân sách tỉnh", amount: 1_500_000_000, disbursed_amount: 300_000_000, disbursed_ratio: 2000 };
    expect(allocationOptionLabel(line)).toBe("Ngân sách tỉnh — còn 1,2 tỷ đồng");
    expect(allocationOptionLabel({ ...line, disbursed_amount: 1_600_000_000 })).toBe("Ngân sách tỉnh — còn -100 triệu đồng");
  });

  it("preselected only when the project has exactly one line", () => {
    const line = { funding_source_id: "S1", name: "A", amount: 1, disbursed_amount: 0, disbursed_ratio: 0 };
    expect(initialFundingSource([line])).toBe("S1");
    expect(initialFundingSource([line, { ...line, funding_source_id: "S2" }])).toBe("");
    expect(initialFundingSource([])).toBe("");
  });
});

describe("thanSuaChungTu — CHỈ những ô thật sự đổi", () => {
  it("không đổi gì thì KHÔNG gửi `PATCH` nào", () => {
    // Một `PATCH` thừa vẫn là một lần ghi, và ở tuyến này nó có thể bóc chữ xác nhận của lãnh đạo.
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_KHONG_CO_GI_DOI,
    });
  });

  it("đổi một ô thì thân CHỈ có ô ấy", () => {
    const kq = thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, noiDung: "Thanh toán đợt 4" }, NO_LINES);

    expect(kq).toEqual({ ok: true, than: { description: "Thanh toán đợt 4" } });
  });

  it("xoá trắng ô đối tác GỬI `\"\"` — đó là một lần sửa có thật", () => {
    const kq = thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, doiTac: "" }, NO_LINES);

    expect(kq).toEqual({ ok: true, than: { counterparty: "" } });
  });

  it("số tiền đổi thì đi qua `docSoTien`, và ca vượt chính xác bị chặn", () => {
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, soTien: "12.000.000" }, NO_LINES)).toEqual({
      ok: true,
      than: { amount: 12000000 },
    });
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, soTien: "999999999999999999" }, NO_LINES)).toEqual(
      { ok: false, cau: CAU_SO_TIEN_VUOT_CHINH_XAC },
    );
  });

  it("xoá trắng ngày chi hoặc nội dung bị TỪ CHỐI — cả hai bắt buộc ở máy chủ", () => {
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, ngayChi: "" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_THIEU_NGAY_CHI,
    });
    expect(thanSuaChungTu(CHUNG_TU_MAU, { ...CHUNG_TU_MAU, noiDung: "" }, NO_LINES)).toEqual({
      ok: false,
      cau: CAU_THIEU_NOI_DUNG,
    });
  });
});

describe("Câu cảnh báo ADR 0036", () => {
  it("nói rõ HAI hậu quả: về Kế toán nhập VÀ mất dấu người xác nhận", () => {
    expect(CANH_BAO_SUA_VE_NHAP).toContain("Kế toán nhập");
    expect(CANH_BAO_SUA_VE_NHAP).toContain("xác nhận");
  });
});

describe("thanThemDuAn", () => {
  it("dựng đủ trường và lấy NĂM từ ô chọn của màn, không từ đồng hồ", () => {
    const kq = thanThemDuAn(2026, DU_AN_MAU);

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.than).toEqual({
      code: "DA01",
      year: 2026,
      category_id: "01JHM1",
      name: "Bê tông hoá đường trục chính thôn Hà Lam",
      description: "Tuyến 1,2 km",
      planned_amount: 7500000000,
      approved_amount: 9000000000,
      start_date: "2026-03-01",
      completion_date: "2026-11-30",
      disbursement_deadline: "2026-12-31",
    });
  });

  it("tổng mức để trống thì VẮNG MẶT — §9: lấy bằng số tiền bố trí năm nay", () => {
    const kq = thanThemDuAn(2026, { ...DU_AN_MAU, tongMucDuyet: "" });

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.than.approved_amount).toBeUndefined();
  });

  it("thiếu mã · thiếu hạng mục · thiếu tên đều bị từ chối trước khi gửi", () => {
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, ma: "  " })).toEqual({
      ok: false,
      cau: CAU_THIEU_MA_DU_AN,
    });
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, hangMucID: "" })).toEqual({
      ok: false,
      cau: CAU_THIEU_HANG_MUC,
    });
    expect(thanThemDuAn(2026, FORM_DU_AN_TRONG).ok).toBe(false);
  });

  it("câu 'thiếu mã' nói rõ hệ thống CHƯA tự sinh mã", () => {
    // §9 vẽ ô `☑ Tự sinh mã`; máy chủ chưa có. Người dùng phải biết vì sao mã phải nhập tay — bằng
    // lời thường, không nhắc "bản thiết kế" (GN-04).
    expect(CAU_THIEU_MA_DU_AN).toContain("chưa tự sinh mã");
    expect(CAU_THIEU_MA_DU_AN).not.toMatch(/bản thiết kế/);
  });
});

describe("ngày khởi công không sau ngày hoàn thành (GN-03, mirror of the server's 0fa67247)", () => {
  const LOI = { ok: false, cau: CAU_KHOI_CONG_SAU_HOAN_THANH };

  it("thêm: khởi công sau hoàn thành → bị từ chối tại chỗ, không dựng thân", () => {
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, ngayKhoiCong: "2026-12-01", ngayHoanThanh: "2026-11-30" })).toEqual(LOI);
  });

  it("thêm: cùng ngày được; thiếu một trong hai ngày thì không có gì để so", () => {
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, ngayKhoiCong: "2026-11-30", ngayHoanThanh: "2026-11-30" }).ok).toBe(true);
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, ngayKhoiCong: "2026-12-01", ngayHoanThanh: "" }).ok).toBe(true);
    expect(thanThemDuAn(2026, { ...DU_AN_MAU, ngayKhoiCong: "", ngayHoanThanh: "2026-01-01" }).ok).toBe(true);
  });

  it("sửa: so cặp ngày SAU KHI SỬA — đổi một ngày cho vượt ngày còn lại cũng bị từ chối", () => {
    expect(thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, ngayHoanThanh: "2026-02-01" })).toEqual(LOI);
    expect(thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, ngayKhoiCong: "2026-12-01" })).toEqual(LOI);
    expect(thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, ngayHoanThanh: "2026-03-01" }).ok).toBe(true);
  });
});

describe("thanSuaDuAn — CHỈ những ô thật sự đổi", () => {
  it("không đổi gì thì không gửi", () => {
    expect(thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU })).toEqual({
      ok: false,
      cau: CAU_KHONG_CO_GI_DOI,
    });
  });

  it("KHÔNG BAO GIỜ mang `code` hay `year`, kể cả khi ô mã bị sửa", () => {
    const kq = thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, ma: "DA99", ten: "Tên mới" });

    expect(kq).toEqual({ ok: true, than: { name: "Tên mới" } });
  });

  it("xoá trắng tổng mức được duyệt gửi 0 — quay về quy tắc §9", () => {
    const kq = thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, tongMucDuyet: "" });

    expect(kq).toEqual({ ok: true, than: { approved_amount: 0 } });
  });

  it("xoá trắng ngày gửi `\"\"` — máy chủ sở hữu quy tắc 'về NULL' và 'về 31/12'", () => {
    const kq = thanSuaDuAn(DU_AN_MAU, { ...DU_AN_MAU, ngayKhoiCong: "", thoiHanGiaiNgan: "" });

    expect(kq).toEqual({ ok: true, than: { start_date: "", disbursement_deadline: "" } });
  });
});

/* ── Phân bổ nguồn vốn §9 (8245698b) ─────────────────────────────────────────────────────────── */

function row(key: number, sourceId: string, amount: string): AllocationRow {
  return { key, sourceId, amount };
}

describe("summarizeAllocations — the live line under the funding list", () => {
  it("no rows: nothing to say", () => {
    expect(summarizeAllocations("100000000", [])).toEqual({ state: "none" });
  });

  it("thiếu · khớp · vượt against the plan typed above, amounts read like every money box", () => {
    expect(summarizeAllocations("100.000.000", [row(0, "S1", "60.000.000")])).toEqual({
      state: "short",
      allocated: 60000000,
      planned: 100000000,
      gap: 40000000,
    });
    expect(summarizeAllocations("100000000", [row(0, "S1", "60000000"), row(1, "S2", "40000000")])).toEqual({
      state: "match",
      allocated: 100000000,
      planned: 100000000,
    });
    expect(summarizeAllocations("100000000", [row(0, "S1", "60000000"), row(1, "S2", "50000000")])).toEqual({
      state: "over",
      allocated: 110000000,
      planned: 100000000,
      gap: 10000000,
    });
  });

  it("only `over` and `unsafe` block saving — under-allocation is a normal project", () => {
    expect(allocationBlocksSave(summarizeAllocations("100", [row(0, "S1", "101")]))).toBe(true);
    expect(allocationBlocksSave(summarizeAllocations("100", [row(0, "S1", "99")]))).toBe(false);
    expect(allocationBlocksSave(summarizeAllocations("100", [row(0, "S1", "100")]))).toBe(false);
    expect(allocationBlocksSave(summarizeAllocations("", [row(0, "S1", "100")]))).toBe(false);
    // Two amounts each exact, whose total a browser number cannot hold exactly: refused, never rounded.
    const big = String(Number.MAX_SAFE_INTEGER);
    expect(summarizeAllocations("1", [row(0, "S1", big), row(1, "S2", big)])).toEqual({ state: "unsafe" });
    expect(allocationBlocksSave({ state: "unsafe" })).toBe(true);
  });

  it("an empty plan is `noPlan`, not 'over' — the clerk may type the list first", () => {
    expect(summarizeAllocations("", [row(0, "S1", "5")])).toEqual({ state: "noPlan", allocated: 5 });
  });
});

describe("allocationLines — rows to the contract's lines", () => {
  it("field by field; the row `key` never reaches the server; 0 is a line the server accepts", () => {
    expect(allocationLines([row(7, "S1", "1.000"), row(9, "S2", "0")])).toEqual({
      ok: true,
      than: [
        { funding_source_id: "S1", amount: 1000 },
        { funding_source_id: "S2", amount: 0 },
      ],
    });
  });

  it("a row without a source, or without an amount, is named", () => {
    expect(allocationLines([row(0, "", "5")])).toEqual({ ok: false, cau: ALLOCATION_NO_SOURCE });
    expect(allocationLines([row(0, "S1", "")])).toEqual({ ok: false, cau: ALLOCATION_NO_AMOUNT });
    expect(allocationLines([row(0, "S1", "1,5")])).toEqual({ ok: false, cau: CAU_SO_TIEN_KHONG_DOC_DUOC });
  });
});

describe("sameAllocations — whether the set changed", () => {
  it("same sources and amounts in any order, amounts compared by value", () => {
    expect(sameAllocations([row(0, "S1", "100000000"), row(1, "S2", "5")], [row(4, "S2", "5"), row(5, "S1", "100.000.000")])).toBe(true);
    expect(sameAllocations([row(0, "S1", "5")], [row(0, "S1", "6")])).toBe(false);
    expect(sameAllocations([row(0, "S1", "5")], [row(0, "S2", "5")])).toBe(false);
    expect(sameAllocations([row(0, "S1", "5")], [])).toBe(false);
  });
});

describe("thanThemDuAn / thanSuaDuAn — funding_allocations", () => {
  const WITH_TWO: GiaTriFormDuAn = {
    ...DU_AN_MAU,
    allocations: [row(0, "S1", "5000000000"), row(1, "S2", "2500000000")],
  };

  it("thêm: lines sent when the list has rows, absent when it has none", () => {
    const sent = thanThemDuAn(2026, WITH_TWO);
    expect(sent.ok && sent.than.funding_allocations).toEqual([
      { funding_source_id: "S1", amount: 5000000000 },
      { funding_source_id: "S2", amount: 2500000000 },
    ]);
    const none = thanThemDuAn(2026, DU_AN_MAU);
    expect(none.ok).toBe(true);
    if (!none.ok) return;
    expect(none.than.funding_allocations).toBeUndefined();
  });

  it("thêm: over the plan is refused before sending, naming the overrun", () => {
    const kq = thanThemDuAn(2026, { ...WITH_TWO, keHoachVon: "7000000000" });
    expect(kq.ok).toBe(false);
    if (kq.ok) return;
    expect(kq.cau).toContain("vượt");
    expect(kq.cau).toContain("không lưu được");
    expect(kq.cau).toContain("500.000.000 đ");
  });

  it("sửa: an UNCHANGED set is OMITTED (absent = leave as is), even when re-entered in another order", () => {
    const reordered: GiaTriFormDuAn = {
      ...WITH_TWO,
      ten: "Tên mới",
      allocations: [row(5, "S2", "2.500.000.000"), row(6, "S1", "5000000000")],
    };
    expect(thanSuaDuAn(WITH_TWO, reordered)).toEqual({ ok: true, than: { name: "Tên mới" } });
  });

  it("sửa: a changed set is sent as the FULL list; removing every row sends []", () => {
    expect(thanSuaDuAn(WITH_TWO, { ...WITH_TWO, allocations: [row(0, "S1", "5000000000")] })).toEqual({
      ok: true,
      than: { funding_allocations: [{ funding_source_id: "S1", amount: 5000000000 }] },
    });
    expect(thanSuaDuAn(WITH_TWO, { ...WITH_TWO, allocations: [] })).toEqual({
      ok: true,
      than: { funding_allocations: [] },
    });
  });

  it("sửa: lowering the plan below what stays allocated is refused here too", () => {
    const kq = thanSuaDuAn(WITH_TWO, { ...WITH_TWO, keHoachVon: "1000" });
    expect(kq.ok).toBe(false);
  });
});
