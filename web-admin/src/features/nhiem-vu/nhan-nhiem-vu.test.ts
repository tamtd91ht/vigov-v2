import { describe, expect, it } from "vitest";

import {
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  KHONG_SO,
  MOI_NHOM_VAN_BAN,
  MOI_TRANG_THAI,
  chiaNhomVanBan,
  coKhoiVanBanChiDao,
  dongVanBan,
  ngayVanBan,
  nhanNhomVanBan,
  O_TRONG,
  PHAN_CHUA_DUNG,
  TRANG_THAI_CHINH,
  TRANG_THAI_RE_NHANH,
  chuyenSangDuoc,
  hoanThanhTreHan,
  ketThuc,
  laTrangThaiNhiemVu,
  nhanBoDem,
  nhanCotKanban,
  nhanHanThe,
  nhanNgay,
  nhanNguonGiao,
  nhanTrangThai,
  oHan,
  quyetDinhDuyetLuiHan,
  tinhTrangHan,
  canhBaoVanBan,
  nhanNutGoVanBan,
  nhanOTieuDe,
  thanGiaoViec,
  type DongVanBanNhap,
  type FormGiaoViecNhap,
} from "./nhan-nhiem-vu";

/**
 * Ca đáng lo nhất của tệp này KHÔNG phải ca định dạng ngày. Nó là `quyetDinhDuyetLuiHan` — cổng
 * quyền hai lớp của ADR 0038 — và nó hỏng theo đúng kiểu không ai nhìn thấy:
 *
 *   1. Bỏ lớp `task.extend` ⇒ nút hiện cho người không có khoá. Màn trông vẫn đúng với lãnh đạo.
 *   2. Bỏ phép kiểm chuỗi rỗng ⇒ `"" === ""` là đúng, và MỌI tài khoản cầm `task.extend` duyệt
 *      được MỌI đề nghị trên MỌI nhiệm vụ chưa ghi lãnh đạo giao việc. Lỗi rộng nhất, sinh ra từ
 *      chỗ thiếu hẹp nhất.
 *   3. So mã cán bộ với một id nội bộ ⇒ không bao giờ khớp, tính năng đơn giản là không chạy, và
 *      không có gì đỏ vì cả hai đều là chuỗi khác rỗng trông rất hợp lý (luật 6, bất biến 8).
 */

describe("ADR 0038 — nút duyệt đề nghị lùi hạn", () => {
  const LANH_DAO = "CB-2026-7K3M9Q";
  const NGUOI_KHAC = "CB-2026-0P4X1Z";

  it("HIỆN chỉ khi có `task.extend` VÀ là đúng người ghi trên bản ghi", () => {
    expect(quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, true)).toEqual({ hien: true });
  });

  it("LỚP MỘT — thiếu `task.extend` thì ẩn, kể cả khi đúng là người được ghi", () => {
    // Cổng của tuyến là `authz.RequirePermission("task.extend")`. Người được ghi trên bản ghi mà
    // không có khoá vẫn nhận 403 trước khi chạm tới quy tắc ADR 0038 — hai lớp, không thay nhau.
    expect(quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, false)).toEqual({
      hien: false,
      vi: "thieu-quyen",
    });
  });

  it("LỚP HAI — có `task.extend` nhưng KHÔNG phải người được ghi thì ẩn", () => {
    // Đây là lớp mà một khoá quyền không diễn đạt được: luật 5 kiểm
    // `(tenant_id, role, permission)` và không có chiều "bản ghi nào". Thiếu nó thì mọi lãnh đạo
    // cầm khoá duyệt được mọi nhiệm vụ của cả xã.
    expect(quyetDinhDuyetLuiHan(NGUOI_KHAC, LANH_DAO, true)).toEqual({
      hien: false,
      vi: "khong-phai-lanh-dao-giao-viec",
      thongBao: CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
    });
  });

  it("HAI CHUỖI RỖNG KHÔNG KHỚP NHAU — nhiệm vụ chưa ghi lãnh đạo thì KHÔNG ai duyệt", () => {
    // ADR 0038 để ngỏ câu này và trả lời là TỪ CHỐI, cố ý KHÔNG lùi về `nguoi_tao_ma`: người
    // đánh máy hộ chính là người không được quyết. Thiếu nhánh này thì `"" === ""` mở cổng cho
    // mọi tài khoản cầm khoá, trên mọi nhiệm vụ chưa ghi lãnh đạo.
    expect(quyetDinhDuyetLuiHan("", "", true)).toEqual({
      hien: false,
      vi: "chua-ghi-lanh-dao-giao-viec",
      thongBao: CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
    });
    expect(quyetDinhDuyetLuiHan(LANH_DAO, "", true)).toEqual({
      hien: false,
      vi: "chua-ghi-lanh-dao-giao-viec",
      thongBao: CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
    });
  });

  it("phiên không có mã cán bộ thì ẩn — FAIL CLOSED, không so được thì không mở", () => {
    expect(quyetDinhDuyetLuiHan("", LANH_DAO, true)).toEqual({
      hien: false,
      vi: "khong-phai-lanh-dao-giao-viec",
      thongBao: CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
    });
  });

  it("một ULID KHÔNG khớp một mã cán bộ — hai loại định danh khác nhau", () => {
    // Cả hai đều là chuỗi khác rỗng trông hợp lý, nên phép so sai KHÔNG làm đỏ bất cứ thứ gì
    // ngoài ca này.
    expect(quyetDinhDuyetLuiHan("01JBGQ3M4K5N6P7Q8R9S0T1U2V", LANH_DAO, true).hien).toBe(false);
  });
});

describe("bảy trạng thái — §6", () => {
  it("năm chính + hai rẽ nhánh, không trùng, không thiếu", () => {
    expect(TRANG_THAI_CHINH).toHaveLength(5);
    expect(TRANG_THAI_RE_NHANH).toEqual(["tam-dung", "chuyen-tiep"]);
    expect(new Set(MOI_TRANG_THAI).size).toBe(7);
  });

  it("`moi-giao` trên Kanban gọi là `Chưa thực hiện`, ở chỗ khác là `Mới giao`", () => {
    // §6 ghi thẳng sự lệch này. Dùng một nhãn cho cả hai chỗ là làm sai một trong hai màn.
    expect(nhanCotKanban("moi-giao")).toBe("Chưa thực hiện");
    expect(nhanTrangThai("moi-giao")).toBe("Mới giao");
  });

  it("mã lạ hiện NGUYÊN VĂN, không thành dấu gạch", () => {
    // Một trạng thái mới ở máy chủ mà màn hình vẽ thành `—` là hồ sơ trông như chưa có trạng thái.
    expect(laTrangThaiNhiemVu("da-ban-giao")).toBe(false);
    expect(nhanTrangThai("da-ban-giao")).toBe("da-ban-giao");
  });

  it("vòng đời: chỉ có bước §6 vẽ ra", () => {
    expect(chuyenSangDuoc("moi-giao", "da-tiep-nhan")).toBe(true);
    expect(chuyenSangDuoc("cho-duyet", "hoan-thanh")).toBe(true);
    // Nhảy cóc không có trong §6.
    expect(chuyenSangDuoc("moi-giao", "hoan-thanh")).toBe(false);
    // `tam-dung` và `chuyen-tiep` rẽ được từ bốn trạng thái chính, không từ `hoan-thanh`.
    expect(chuyenSangDuoc("dang-thuc-hien", "tam-dung")).toBe(true);
    expect(chuyenSangDuoc("hoan-thanh", "tam-dung")).toBe(false);
  });

  it("`hoan-thanh` và `chuyen-tiep` là hai ngõ cụt", () => {
    expect(ketThuc("hoan-thanh")).toBe(true);
    expect(ketThuc("chuyen-tiep")).toBe(true);
    expect(ketThuc("tam-dung")).toBe(false);
  });

  it("`tam-dung` quay về được bốn trạng thái chính — lỏng CÓ CHỦ Ý", () => {
    // Chỉ ĐÚNG MỘT trong bốn là hợp lệ thật: trạng thái ngay trước lúc dừng. Máy chủ tìm nó trong
    // nhật ký; màn hình không có tuyến nhật ký nào nên hiện cả bốn và để máy chủ từ chối ba lối sai.
    for (const t of ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet"]) {
      expect(chuyenSangDuoc("tam-dung", t)).toBe(true);
    }
    expect(chuyenSangDuoc("tam-dung", "hoan-thanh")).toBe(false);
  });
});

describe("hạn xử lý — SUY RA, không đọc từ một cột", () => {
  const BAY_GIO = new Date("2026-09-23T10:00:00+07:00");

  it("không có hạn là một TRẠNG THÁI THẬT, không phải số 0", () => {
    expect(tinhTrangHan(null, BAY_GIO)).toEqual({ loai: "khong-han" });
    expect(nhanHanThe(null, BAY_GIO)).toBe(`Hạn ${O_TRONG}`);
    expect(oHan(null, BAY_GIO)).toEqual({ ngay: O_TRONG, phanTre: "" });
  });

  it("quá hạn đếm ra số ngày; còn hạn thì in ngày", () => {
    expect(nhanHanThe("2026-06-28T10:00:00+07:00", BAY_GIO)).toBe("Trễ 87 ngày");
    expect(nhanHanThe("2026-12-20T10:00:00+07:00", BAY_GIO)).toBe("Hạn 20/12/2026");
  });

  it("`Trễ 0 ngày` KHÔNG BAO GIỜ được in ra", () => {
    // Việc vừa quá hạn nửa tiếng vẫn là việc đã trễ, và `Trễ 0 ngày` đọc ra là "chưa trễ" —
    // đúng điều ngược lại.
    expect(nhanHanThe("2026-09-23T09:30:00+07:00", BAY_GIO)).toBe("Trễ dưới 1 ngày");
    expect(oHan("2026-09-23T09:30:00+07:00", BAY_GIO).phanTre).toBe("(trễ dưới 1 ngày)");
  });

  it("ô hạn ở bảng tách RIÊNG phần trễ để tầng vẽ tô đỏ — §4.2", () => {
    // Ghép sẵn rồi cắt lại bằng biểu thức chính quy ở tầng vẽ là dựng một phép phân tích trên
    // chính chuỗi mình vừa tạo.
    expect(oHan("2026-09-10T10:00:00+07:00", BAY_GIO)).toEqual({
      ngay: "10/9/2026",
      phanTre: "(trễ 13 ngày)",
    });
    expect(oHan("2026-12-20T10:00:00+07:00", BAY_GIO).phanTre).toBe("");
  });

  it("ngày in theo múi giờ Việt Nam, không theo múi giờ của máy chạy", () => {
    // `due_at` là `date-time`. Không ghim múi giờ thì một hạn 23:30 giờ Việt Nam in ra ngày hôm
    // trước ở mọi máy đặt múi giờ phía tây — một cam kết lệch một ngày.
    expect(nhanNgay("2026-06-20T23:30:00+07:00")).toBe("20/6/2026");
  });

  it("chuỗi không đọc được hiện NGUYÊN VĂN, không `Invalid Date` và không dấu gạch", () => {
    expect(nhanNgay("hai mươi tháng sáu")).toBe("hai mươi tháng sáu");
  });
});

describe("chip `Hoàn thành trễ hạn`", () => {
  it("so với HẠN BAN ĐẦU, không so với hạn hiện tại", () => {
    // §11 quy tắc 3: tỷ lệ đúng hạn tính trên `ngay_hoan_thanh ≤ han_ban_dau`, và §5.6 nói hạn
    // ban đầu KHÔNG đổi khi gia hạn. So với `due_at` thì mọi việc được duyệt lùi hạn đều thành
    // "đúng hạn" — một lần lùi hạn tự xoá dấu vết của chính nó khỏi báo cáo.
    const hanBanDau = "2026-06-20T17:00:00+07:00";
    expect(hoanThanhTreHan("2026-06-25T09:00:00+07:00", hanBanDau)).toBe(true);
    expect(hoanThanhTreHan("2026-06-19T09:00:00+07:00", hanBanDau)).toBe(false);
  });

  it("chưa hoàn thành, hoặc không có hạn ban đầu, thì KHÔNG phải trễ", () => {
    expect(hoanThanhTreHan(null, "2026-06-20T17:00:00+07:00")).toBe(false);
    expect(hoanThanhTreHan("2026-06-25T09:00:00+07:00", null)).toBe(false);
  });
});

describe("câu chữ chung", () => {
  it("bốn nguồn giao lấy nguyên văn §3", () => {
    expect(nhanNguonGiao("van-ban-den")).toBe("Từ văn bản đến");
    expect(nhanNguonGiao("ket-luan-hop")).toBe("Từ kết luận họp");
    expect(nhanNguonGiao("nguon-la")).toBe("nguon-la");
  });

  it("chân trang đếm đúng chữ §2", () => {
    expect(nhanBoDem(28)).toBe("Hiển thị 28 nhiệm vụ.");
  });
});

describe("phần chưa dựng được", () => {
  it("mỗi mục có TÊN và LÝ DO — không mục nào để trống lý do", () => {
    // Một mục không nói vì sao là một mục người sau đọc thành "chưa làm tới", rồi dựng nó lên và
    // gặp lại đúng bức tường cũ.
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toBe("");
      expect(p.viSao.length).toBeGreaterThan(40);
    }
  });

  it("hai chỗ máy chủ TỪ CHỐI bộ lọc đều có mặt — không vẽ ô tick chắc chắn hỏng", () => {
    const moiLyDo = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(moiLyDo).toContain("Liên quan đến tôi");
    expect(moiLyDo).toContain("Sắp đến hạn");
    expect(moiLyDo).toContain("task.assign");
    expect(moiLyDo).toContain("chuyen-tiep");
  });

  it("hai mục về văn bản chỉ đạo nói ĐÚNG thứ còn thiếu hôm nay, không còn nói bảng chưa có", () => {
    // SỬA CÓ CHỦ Ý 24/09/2026: hai mục này từng viết "bảng `nhiem_vu_van_ban` chưa tồn tại". Bảng
    // đã có từ migration 0009; một lý do sai trên màn là lý do đẩy người sau đi dựng lại thứ đã có.
    const muc54 = PHAN_CHUA_DUNG.find((p) => p.ten.includes("SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO"));
    const muc43 = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Sổ theo dõi` (§4.3)"));
    expect(muc54).toBeDefined();
    expect(muc43).toBeDefined();
    for (const p of [muc54, muc43]) {
      expect(p?.viSao).not.toContain("CHƯA TỒN TẠI");
      expect(p?.viSao).not.toContain("chưa tồn tại");
    }
    expect(muc54?.ten).toContain("✎ Sửa");
    expect(muc54?.viSao).toContain("GET /api/v1/tasks/{ma}");
    expect(muc43?.viSao).toContain("`GET /api/v1/tasks`");
    expect(muc43?.viSao).toContain("documents");
    expect(muc43?.viSao).toContain("xuat-so-theo-doi");
  });
});

describe("§5.4 — câu chữ của khối văn bản chỉ đạo", () => {
  it("chỉ loại `theo-van-ban` có khối", () => {
    expect(coKhoiVanBanChiDao("theo-van-ban")).toBe(true);
    expect(coKhoiVanBanChiDao("co-ban")).toBe(false);
    expect(coKhoiVanBanChiDao("")).toBe(false);
  });

  it("ba nhãn nhóm NGUYÊN VĂN §5.4, đúng thứ tự; mã lạ hiện nguyên văn", () => {
    expect(MOI_NHOM_VAN_BAN.map(nhanNhomVanBan)).toEqual([
      "Văn bản cấp trên giao",
      "Văn bản chỉ đạo của Đảng uỷ",
      "Văn bản sản phẩm đầu ra",
    ]);
    expect(nhanNhomVanBan("nhom-la")).toBe("nhom-la");
  });

  it("ngày văn bản không đệm số 0, không lệch ngày dưới TZ=UTC; sai khuôn hiện nguyên văn", () => {
    expect(ngayVanBan("2026-06-09")).toBe("9/6/2026");
    expect(ngayVanBan("2026-11-30")).toBe("30/11/2026");
    expect(ngayVanBan("")).toBe("");
    expect(ngayVanBan("09/06/2026")).toBe("09/06/2026");
  });

  it("dòng văn bản: `{số} · {ngày}`, `Không số` khi thiếu số, bỏ phần ngày khi thiếu ngày", () => {
    expect(dongVanBan("90-TB/TU", "2026-11-30")).toBe("90-TB/TU · 30/11/2026");
    expect(dongVanBan("", "2026-11-30")).toBe(`${KHONG_SO} · 30/11/2026`);
    expect(dongVanBan("90-TB/TU", "")).toBe("90-TB/TU");
    expect(dongVanBan("", "")).toBe(KHONG_SO);
    expect(KHONG_SO).toBe("Không số");
  });

  it("chia nhóm: luôn đủ ba nhóm, giữ thứ tự máy chủ, và KHÔNG bỏ rơi một mã nhóm lạ", () => {
    const dong = (id: string, group: string, position: number) => ({
      id,
      group,
      reference: id,
      date: "",
      summary: "Trích yếu",
      position,
    });
    const nhom = chiaNhomVanBan([
      dong("c", "san-pham-dau-ra", 5),
      dong("a", "cap-tren-giao", 2),
      dong("b", "cap-tren-giao", 1),
      dong("x", "nhom-moi", 1),
    ]);
    expect(nhom.map((n) => n.ma)).toEqual([
      "cap-tren-giao",
      "chi-dao-dang-uy",
      "san-pham-dau-ra",
      "nhom-moi",
    ]);
    expect(nhom[0]?.vanBan.map((v) => v.id)).toEqual(["a", "b"]);
    expect(nhom[1]?.vanBan).toEqual([]);
    expect(nhom[3]?.nhan).toBe("nhom-moi");
    expect(chiaNhomVanBan([]).map((n) => n.vanBan.length)).toEqual([0, 0, 0]);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * FORM `GIAO VIỆC MỚI` §7.2 / §7.3 — THÂN YÊU CẦU
 *
 * Ca đắt nhất ở đây là ca KHÔNG NHÌN THẤY: cán bộ gõ cơ quan chủ trì và ba văn bản, rồi đổi sang
 * `Nhiệm vụ cơ bản`. Các ô biến khỏi màn — và nếu chúng vẫn lên dây thì bản ghi (không xoá cứng
 * được) mang thứ người giao việc tin là đã bỏ. Mọi dữ liệu dưới đây là dữ liệu giả.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

function dongVB(sua: Partial<DongVanBanNhap> & Pick<DongVanBanNhap, "khoa" | "nhom">): DongVanBanNhap {
  return { trichYeu: "", soKyHieu: "", ngay: "", ...sua };
}

/** Form ĐÃ GÕ ĐỦ mọi ô, kể cả ba ô chỉ `Theo văn bản` mới có. */
function formDay(sua: Partial<FormGiaoViecNhap> = {}): FormGiaoViecNhap {
  return {
    tuSinhMa: true,
    ma: "",
    loai: "theo-van-ban",
    khoi: "",
    tieuDe: "  Báo cáo sơ kết công tác tháng 9  ",
    moTa: "",
    mucUuTien: "",
    boPhan: "",
    nguoiThucHien: "",
    lanhDaoGiaoViec: "",
    coQuanChuTri: "01JBOPHANGIA",
    chuyenVien: "CB-2026-GIA001",
    han: "",
    vanBan: [
      // CỐ Ý XEN KẼ NHÓM theo thứ tự bấm: thân phải gom theo nhóm §5.4 mà giữ thứ tự trong nhóm.
      dongVB({ khoa: "a", nhom: "san-pham-dau-ra", trichYeu: "Báo cáo giả số một" }),
      dongVB({
        khoa: "b",
        nhom: "cap-tren-giao",
        trichYeu: "  Thông báo giả về ý kiến chỉ đạo  ",
        soKyHieu: " 90-TB/GIA ",
        ngay: "2026-01-30",
      }),
      dongVB({ khoa: "c", nhom: "cap-tren-giao", trichYeu: "Kế hoạch giả thứ hai", soKyHieu: "   " }),
      dongVB({ khoa: "d", nhom: "chi-dao-dang-uy", trichYeu: "Công văn giả", ngay: "2026-06-15" }),
    ],
    ...sua,
  };
}

describe("§7.2 / §7.3 — nhãn ô tiêu đề theo loại", () => {
  it("`theo-van-ban` ⇒ `Nội dung nhiệm vụ / Trích yếu văn bản`; loại khác ⇒ `Tên nhiệm vụ`", () => {
    expect(nhanOTieuDe("theo-van-ban")).toBe("Nội dung nhiệm vụ / Trích yếu văn bản");
    expect(nhanOTieuDe("co-ban")).toBe("Tên nhiệm vụ");
    expect(nhanOTieuDe("")).toBe("Tên nhiệm vụ");
  });
});

describe("thân `Giao việc mới` — ba nhóm văn bản", () => {
  it("`theo-van-ban`: `documents` mang ĐÚNG group/summary/reference/date, gom theo nhóm, giữ thứ tự trên màn", () => {
    const than = thanGiaoViec(formDay(), { coDanhSachVanBan: true });
    expect(than.documents).toEqual([
      {
        group: "cap-tren-giao",
        summary: "Thông báo giả về ý kiến chỉ đạo",
        reference: "90-TB/GIA",
        date: "2026-01-30",
      },
      { group: "cap-tren-giao", summary: "Kế hoạch giả thứ hai" },
      { group: "chi-dao-dang-uy", summary: "Công văn giả", date: "2026-06-15" },
      { group: "san-pham-dau-ra", summary: "Báo cáo giả số một" },
    ]);
    expect(than.lead_unit).toBe("01JBOPHANGIA");
    expect(than.monitor).toBe("CB-2026-GIA001");
    expect(than.title).toBe("Báo cáo sơ kết công tác tháng 9");
  });

  it("ô tuỳ chọn bỏ trống KHÔNG thành rác: không `reference: \"\"`, không `date: \"\"`, không `id`/`position`", () => {
    const than = thanGiaoViec(formDay(), { coDanhSachVanBan: true });
    for (const d of than.documents ?? []) {
      expect(Object.keys(d).every((k) => ["group", "summary", "reference", "date"].includes(k))).toBe(true);
      expect(Object.values(d)).not.toContain("");
    }
    // Dòng `c` gõ toàn khoảng trắng vào ô số ⇒ vắng hẳn.
    expect(than.documents?.[1]).not.toHaveProperty("reference");
    expect(than.documents?.[1]).not.toHaveProperty("date");
  });

  it("ngày văn bản đi NGUYÊN `YYYY-MM-DD`, không bao giờ là một mốc RFC 3339", () => {
    // Máy chủ TỪ CHỐI RFC 3339 có chủ ý (`nhiem_vu_ghi.go:335-337`): múi giờ trình duyệt không
    // được quyết văn bản ký ngày nào. Ghim TZ=UTC nên một phép đi vòng qua `Date` sẽ lộ ra.
    const than = thanGiaoViec(formDay(), { coDanhSachVanBan: true });
    const ngay = (than.documents ?? []).flatMap((d) => (d.date === undefined ? [] : [d.date]));
    expect(ngay).toEqual(["2026-01-30", "2026-06-15"]);
    for (const n of ngay) expect(n).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("KHÔNG có dòng nào ⇒ `documents` vắng mặt, không gửi mảng rỗng", () => {
    expect(thanGiaoViec(formDay({ vanBan: [] }), { coDanhSachVanBan: true })).not.toHaveProperty(
      "documents",
    );
  });

  it("gỡ một dòng giữa ⇒ đúng dòng ấy biến khỏi thân, các dòng còn lại giữ thứ tự", () => {
    const f = formDay();
    const sauKhiGo = formDay({ vanBan: f.vanBan.filter((d) => d.khoa !== "b") });
    const than = thanGiaoViec(sauKhiGo, { coDanhSachVanBan: true });
    expect(than.documents?.map((d) => d.summary)).toEqual([
      "Kế hoạch giả thứ hai",
      "Công văn giả",
      "Báo cáo giả số một",
    ]);
  });
});

describe("thân `Giao việc mới` — trường ĐANG ẨN không lên dây", () => {
  it("`co-ban` SAU KHI đã gõ cơ quan chủ trì, chuyên viên và ba văn bản: không `documents`/`lead_unit`/`monitor`", () => {
    // Đúng thao tác thật: gõ đủ ở `Theo văn bản` rồi mới đổi loại. State vẫn giữ chữ đã gõ (đổi
    // lại thì hiện lại) — nên chỗ cắt PHẢI là hàm dựng thân này.
    const than = thanGiaoViec(formDay({ loai: "co-ban" }), { coDanhSachVanBan: true }) as Record<
      string,
      unknown
    >;
    expect(than).not.toHaveProperty("documents");
    expect(than).not.toHaveProperty("lead_unit");
    expect(than).not.toHaveProperty("monitor");
    expect(than.type).toBe("co-ban");
  });

  it("màn Biên bản (`coDanhSachVanBan: false`) KHÔNG gửi `documents` kể cả với loại `theo-van-ban`", () => {
    // `petitions.tachKetLuanVao` không có `documents`. Cơ quan chủ trì và chuyên viên thì CÓ, nên
    // hai ô ấy vẫn đi — hành vi màn Biên bản không đổi.
    const than = thanGiaoViec(formDay(), { coDanhSachVanBan: false });
    expect(than).not.toHaveProperty("documents");
    expect(than.lead_unit).toBe("01JBOPHANGIA");
    expect(than.monitor).toBe("CB-2026-GIA001");
  });
});

describe("chặn nút `Giao việc` vì ba danh sách", () => {
  it("dòng chưa có trích yếu CHẶN và nói đúng dòng nào — không bị lọc bỏ lặng lẽ", () => {
    const cau = canhBaoVanBan([
      dongVB({ khoa: "a", nhom: "chi-dao-dang-uy", trichYeu: "Có nội dung" }),
      dongVB({ khoa: "b", nhom: "chi-dao-dang-uy", trichYeu: "   " }),
    ]);
    expect(cau).toContain("Văn bản thứ 2 của nhóm Văn bản chỉ đạo của Đảng uỷ");
    expect(canhBaoVanBan(formDay().vanBan)).toBeNull();
    expect(canhBaoVanBan([])).toBeNull();
  });

  it("quá 100 dòng một lần ⇒ chặn, nói con số (giới hạn của máy chủ)", () => {
    const nhieu = Array.from({ length: 101 }, (_, i) =>
      dongVB({ khoa: String(i), nhom: "cap-tren-giao", trichYeu: "x" }),
    );
    expect(canhBaoVanBan(nhieu)).toContain("tối đa 100");
    expect(canhBaoVanBan(nhieu.slice(0, 100))).toBeNull();
  });

  it("tên nút `✕` nói dòng thứ mấy của nhóm nào", () => {
    expect(nhanNutGoVanBan("san-pham-dau-ra", 3)).toBe(
      "Gỡ văn bản thứ 3 khỏi nhóm Văn bản sản phẩm đầu ra",
    );
  });
});
