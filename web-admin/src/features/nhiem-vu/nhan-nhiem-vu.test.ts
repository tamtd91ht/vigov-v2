import { describe, expect, it } from "vitest";

import type { petitions_nhiemVuRa, petitions_nhiemVuVanBanRa } from "@/lib/api/schema.gen";

import {
  BANG_NHAN_MAC_DINH,
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
  nhanHanThe,
  nhanNgay,
  nhanNguonGiao,
  nhanTrangThai,
  oHan,
  quyetDinhDuyetLuiHan,
  tinhTrangHan,
  canhBaoVanBan,
  cauTuKetLuan,
  nhanNutGoVanBan,
  nhanOTieuDe,
  thanGiaoViec,
  KHOA_SUA_DANG_TAI,
  KHOA_SUA_LOI,
  KHOA_SUA_NHOM_LA,
  LY_DO_KHONG_SUA_CHU_TRI,
  LY_DO_KHONG_SUA_HAN,
  canhBaoSua,
  formSuaTuChiTiet,
  lyDoKhoaSua,
  thanSuaNhiemVu,
  vanBanDaDoiOMayChu,
  canDocLaiTruocKhiLuu,
  CHI_TIET_THIEU_VAN_BAN,
  loiSauKhiDocLai,
  VAN_BAN_VUA_BI_DOI,
  type DongVanBanNhap,
  type DongVanBanSua,
  type FormGiaoViecNhap,
  type FormSuaNhiemVu,
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

  it("đường lui = đúng bảng mặc định của máy chủ: `moi-giao` là `Mới giao`, thứ tự vòng đời", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (#21): bài này từng canh nhãn Kanban riêng "Chưa thực hiện".
    // Máy chủ giao "Mới giao" làm mặc định và coi "Chưa thực hiện" là chữ XÃ tự đặt
    // (`nhan_trang_thai_nhiem_vu.go:57-59`); một bản nhãn Kanban thứ hai ở màn hình là bản sao
    // quyết định #21 bỏ đi.
    expect(nhanTrangThai(BANG_NHAN_MAC_DINH, "moi-giao")).toBe("Mới giao");
    expect(BANG_NHAN_MAC_DINH.thuTu).toEqual(MOI_TRANG_THAI);
  });

  it("mã lạ hiện NGUYÊN VĂN, không thành dấu gạch", () => {
    // Một trạng thái mới ở máy chủ mà màn hình vẽ thành `—` là hồ sơ trông như chưa có trạng thái.
    expect(laTrangThaiNhiemVu("da-ban-giao")).toBe(false);
    expect(nhanTrangThai(BANG_NHAN_MAC_DINH, "da-ban-giao")).toBe("da-ban-giao");
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

  it("liên kết ngược về biên bản gốc: số kết luận ĐÃ CẤP và tên cuộc họp", () => {
    const nv = { meeting_id: "01JBB1", meeting_title: "Giao ban tháng 8", conclusion_no: 3 };
    expect(cauTuKetLuan(nv as petitions_nhiemVuRa)).toBe(
      "Từ kết luận số 3 — Giao ban tháng 8",
    );
  });

  it("không có `meeting_id` thì KHÔNG có liên kết — kể cả khi `source` là kết luận họp", () => {
    // Máy chủ chỉ điền bộ ba `meeting_*` khi nối được về một biên bản còn đó; dựng liên kết từ
    // `source_id` là trỏ vào hư không.
    const nv = { source: "ket-luan-hop", source_id: "01JKL" };
    expect(cauTuKetLuan(nv as petitions_nhiemVuRa)).toBeNull();
  });

  it("thiếu số kết luận thì bỏ con số, không bịa một con số", () => {
    const nv = { meeting_id: "01JBB1", meeting_title: "Giao ban tháng 8" };
    expect(cauTuKetLuan(nv as petitions_nhiemVuRa)).toBe("Từ kết luận họp — Giao ban tháng 8");
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
    //
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (TASK-03): mục `Nút ✎ Sửa của khối SỔ THEO DÕI…` đã dựng xong
    // nên BIẾN KHỎI danh sách; phần còn thiếu của nó thu lại đúng một điều — ô Ghi chú ở form TẠO.
    const mucSua = PHAN_CHUA_DUNG.find((p) => p.ten.includes("✎ Sửa"));
    const mucGhiChu = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Ghi chú` ở form `Giao việc mới`"));
    const muc43 = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Sổ theo dõi` (§4.3)"));
    expect(mucSua).toBeUndefined();
    expect(mucGhiChu).toBeDefined();
    expect(muc43).toBeDefined();
    for (const p of [mucGhiChu, muc43]) {
      expect(p?.viSao).not.toContain("CHƯA TỒN TẠI");
      expect(p?.viSao).not.toContain("chưa tồn tại");
    }
    expect(mucGhiChu?.viSao).toContain("taoNhiemVuVao");
    expect(mucGhiChu?.viSao).toContain("✎ Sửa");
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

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NÚT `✎ SỬA` §5.4 — THÂN `PATCH /api/v1/tasks/{ma}`
 *
 * Ca đắt nhất ở đây là ca KHÔNG NHÌN THẤY: `documents` trên PATCH là THAY CẢ TẬP. Một dòng cũ lên
 * dây thiếu `id` là một văn bản bị xoá mềm rồi chép lại; `documents` gửi kèm khi cán bộ không đụng
 * tới khối là ghi đè lên bất cứ dòng nào người khác vừa thêm. Cả hai đều xanh trên màn hình.
 * Dữ liệu giả, mã cán bộ giả `CB-00001`.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

function chiTiet(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "theo-van-ban",
    bloc: "",
    priority: "",
    title: "Báo cáo giả về công tác cán bộ",
    description: "",
    status: "dang-thuc-hien",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "CB-00001",
    lead_unit: "01JBOPHANGIA",
    monitor: "CB-00001",
    due_at: "2026-06-20T23:59:59+07:00",
    original_due_at: "2026-06-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    created_by: "CB-00001",
    created_at: "2026-06-01T02:00:00Z",
    ...sua,
  };
}

const VB_GOC: petitions_nhiemVuVanBanRa[] = [
  {
    id: "01JVB1",
    group: "cap-tren-giao",
    reference: "1742-CV/GIA",
    date: "2026-06-09",
    summary: "Công văn giả của cấp trên",
    position: 1,
  },
  {
    id: "01JVB2",
    group: "san-pham-dau-ra",
    reference: "",
    date: "",
    summary: "Báo cáo giả đầu ra",
    position: 1,
  },
];

function formGoc(): FormSuaNhiemVu {
  return formSuaTuChiTiet(chiTiet(), VB_GOC);
}

describe("✎ Sửa — form bắt đầu từ chi tiết", () => {
  it("điền đủ năm ô và mọi dòng, dòng cũ mang `id` của nó", () => {
    const f = formSuaTuChiTiet(chiTiet({ note: "Ghi chú giả", leader_approved: true }), VB_GOC);
    expect(f.tieuDe).toBe("Báo cáo giả về công tác cán bộ");
    expect(f.ghiChu).toBe("Ghi chú giả");
    expect(f.lanhDaoPheDuyet).toBe(true);
    expect(f.vanBan.map((d) => d.id)).toEqual(["01JVB1", "01JVB2"]);
    expect(f.vanBan[0]).toMatchObject({
      nhom: "cap-tren-giao",
      trichYeu: "Công văn giả của cấp trên",
      soKyHieu: "1742-CV/GIA",
      ngay: "2026-06-09",
    });
  });
});

describe("✎ Sửa — thân PATCH chỉ mang thứ đã đổi", () => {
  it("form KHÔNG ĐỔI ⇒ `null` (nút Lưu khoá), kể cả khi chỉ gõ thêm khoảng trắng", () => {
    expect(thanSuaNhiemVu(formGoc(), chiTiet(), VB_GOC)).toBeNull();
    const f = { ...formGoc(), tieuDe: "  Báo cáo giả về công tác cán bộ  " };
    expect(thanSuaNhiemVu(f, chiTiet(), VB_GOC)).toBeNull();
  });

  it("đổi MỘT trường vô hướng ⇒ thân có ĐÚNG trường ấy", () => {
    expect(thanSuaNhiemVu({ ...formGoc(), ghiChu: " Ghi chú giả mới " }, chiTiet(), VB_GOC)).toEqual({
      note: "Ghi chú giả mới",
    });
    expect(thanSuaNhiemVu({ ...formGoc(), capTrenCongNhan: true }, chiTiet(), VB_GOC)).toEqual({
      superior_acknowledged: true,
    });
    expect(thanSuaNhiemVu({ ...formGoc(), tomTatKetQua: "Đã xong giả" }, chiTiet(), VB_GOC)).toEqual({
      result_summary: "Đã xong giả",
    });
  });

  it("xoá trắng ghi chú là MỘT THAY ĐỔI — gửi chuỗi rỗng, không bỏ qua", () => {
    const f = { ...formSuaTuChiTiet(chiTiet({ note: "Cũ" }), VB_GOC), ghiChu: "" };
    expect(thanSuaNhiemVu(f, chiTiet({ note: "Cũ" }), VB_GOC)).toEqual({ note: "" });
  });

  it("không đụng tới văn bản ⇒ `documents` VẮNG MẶT", () => {
    const than = thanSuaNhiemVu({ ...formGoc(), tieuDe: "Tiêu đề giả mới" }, chiTiet(), VB_GOC);
    expect(than).toEqual({ title: "Tiêu đề giả mới" });
    expect(than).not.toHaveProperty("documents");
  });

  it("sửa chữ một dòng ⇒ gửi LẠI MỌI dòng còn giữ, mỗi dòng cũ KÈM `id`", () => {
    const f = formGoc();
    const sua: FormSuaNhiemVu = {
      ...f,
      vanBan: f.vanBan.map((d) => (d.id === "01JVB2" ? { ...d, trichYeu: "Báo cáo giả đã sửa" } : d)),
    };
    expect(thanSuaNhiemVu(sua, chiTiet(), VB_GOC)?.documents).toEqual([
      {
        id: "01JVB1",
        group: "cap-tren-giao",
        summary: "Công văn giả của cấp trên",
        reference: "1742-CV/GIA",
        date: "2026-06-09",
      },
      { id: "01JVB2", group: "san-pham-dau-ra", summary: "Báo cáo giả đã sửa" },
    ]);
  });

  it("CHỈ xoá số ký hiệu của một dòng (kèm sửa ghi chú) ⇒ `documents` VẪN đi, dòng ấy thiếu `reference`", () => {
    // Không phát hiện được thay đổi này thì thân chỉ còn `note`: lưu "thành công", cán bộ tưởng đã
    // xoá số ký hiệu, còn sổ vẫn giữ số cũ. Máy chủ đọc `reference` vắng mặt trên dòng có `id` là
    // chuỗi rỗng (`SoSanhVanBan`, `domain/nhiem_vu_van_ban.go`), nên vắng mặt là đúng hình dạng.
    const f = formGoc();
    const than = thanSuaNhiemVu(
      {
        ...f,
        ghiChu: "Ghi chú giả",
        vanBan: f.vanBan.map((d) => (d.id === "01JVB1" ? { ...d, soKyHieu: "" } : d)),
      },
      chiTiet(),
      VB_GOC,
    );
    expect(than?.note).toBe("Ghi chú giả");
    const dong = than?.documents?.find((d) => d.id === "01JVB1");
    expect(dong).toEqual({
      id: "01JVB1",
      group: "cap-tren-giao",
      summary: "Công văn giả của cấp trên",
      date: "2026-06-09",
    });
  });

  it("gỡ một dòng ⇒ dòng ấy VẮNG khỏi thân, dòng kia vẫn đi kèm `id`", () => {
    const f = formGoc();
    const than = thanSuaNhiemVu(
      { ...f, vanBan: f.vanBan.filter((d) => d.id !== "01JVB1") },
      chiTiet(),
      VB_GOC,
    );
    expect(than?.documents?.map((d) => d.id)).toEqual(["01JVB2"]);
  });

  it("thêm một dòng ⇒ dòng mới KHÔNG có `id`, dòng cũ vẫn có", () => {
    const f = formGoc();
    const moi: DongVanBanSua = {
      khoa: "moi1",
      id: "",
      nhom: "chi-dao-dang-uy",
      trichYeu: "Công văn giả mới",
      soKyHieu: "",
      ngay: "2026-07-01",
    };
    const ds = thanSuaNhiemVu({ ...f, vanBan: [...f.vanBan, moi] }, chiTiet(), VB_GOC)?.documents ?? [];
    expect(ds).toHaveLength(3);
    const dongMoi = ds.find((d) => d.summary === "Công văn giả mới");
    expect(dongMoi).toEqual({ group: "chi-dao-dang-uy", summary: "Công văn giả mới", date: "2026-07-01" });
    expect(dongMoi).not.toHaveProperty("id");
    expect(ds.filter((d) => d.id !== undefined).map((d) => d.id)).toEqual(["01JVB1", "01JVB2"]);
  });

  it("gỡ HẾT mọi dòng ⇒ `documents: []`, không phải vắng mặt", () => {
    expect(thanSuaNhiemVu({ ...formGoc(), vanBan: [] }, chiTiet(), VB_GOC)).toEqual({ documents: [] });
  });

  it("không bao giờ có `code`, `due_at`, `assigner`, `lead_unit`, `monitor`, `position`", () => {
    const f: FormSuaNhiemVu = {
      tieuDe: "Tiêu đề giả khác",
      tomTatKetQua: "Kết quả giả",
      ghiChu: "Ghi chú giả",
      lanhDaoPheDuyet: true,
      capTrenCongNhan: true,
      vanBan: [],
    };
    const than = thanSuaNhiemVu(f, chiTiet(), VB_GOC) as Record<string, unknown>;
    for (const k of ["code", "due_at", "original_due_at", "assigner", "lead_unit", "monitor"]) {
      expect(than).not.toHaveProperty(k);
    }
    const coVanBan = thanSuaNhiemVu(
      { ...formGoc(), vanBan: formGoc().vanBan.slice(0, 1) },
      chiTiet(),
      VB_GOC,
    );
    expect(coVanBan?.documents).toHaveLength(1);
    for (const d of coVanBan?.documents ?? []) expect(d).not.toHaveProperty("position");
  });

  it("ngày văn bản đi NGUYÊN `YYYY-MM-DD`", () => {
    const f = formGoc();
    const than = thanSuaNhiemVu(
      { ...f, vanBan: f.vanBan.map((d) => ({ ...d, ngay: "2026-12-31" })) },
      chiTiet(),
      VB_GOC,
    );
    expect(than?.documents).toHaveLength(2);
    for (const d of than?.documents ?? []) expect(d.date).toBe("2026-12-31");
  });
});

describe("✎ Sửa — khi nào nút mở, khi nào Lưu khoá", () => {
  it("khối đang tải hoặc đọc hỏng ⇒ KHOÁ kèm lý do; đọc xong ⇒ mở", () => {
    expect(lyDoKhoaSua({ pha: "dangTai" })).toBe(KHOA_SUA_DANG_TAI);
    expect(lyDoKhoaSua({ pha: "loi" })).toBe(KHOA_SUA_LOI);
    expect(lyDoKhoaSua({ pha: "xong", duLieu: VB_GOC })).toBeNull();
    expect(lyDoKhoaSua({ pha: "xong", duLieu: [] })).toBeNull();
  });

  it("một mã nhóm lạ ⇒ KHOÁ: form không vẽ được dòng ấy, lưu lại là gỡ mất nó", () => {
    const la = { ...(VB_GOC[0] as petitions_nhiemVuVanBanRa), id: "01JVBLA", group: "nhom-moi" };
    expect(lyDoKhoaSua({ pha: "xong", duLieu: [...VB_GOC, la] })).toBe(KHOA_SUA_NHOM_LA);
  });

  it("tiêu đề xoá trắng, dòng thiếu trích yếu ⇒ chặn Lưu và nói vì sao", () => {
    expect(canhBaoSua({ ...formGoc(), tieuDe: "   " })).toContain("không được để trống");
    const f = formGoc();
    const cau = canhBaoSua({
      ...f,
      vanBan: f.vanBan.map((d) => (d.id === "01JVB2" ? { ...d, trichYeu: " " } : d)),
    });
    expect(cau).toContain("Văn bản thứ 1 của nhóm Văn bản sản phẩm đầu ra");
    expect(canhBaoSua(formGoc())).toBeNull();
  });

  it("ngày sai khuôn ⇒ chặn Lưu, không lặng lẽ bỏ ngày", () => {
    const f = formGoc();
    const cau = canhBaoSua({
      ...f,
      vanBan: f.vanBan.map((d) => (d.id === "01JVB1" ? { ...d, ngay: "09/06/2026" } : d)),
    });
    expect(cau).toContain("không đúng khuôn ngày");
  });

  it("lý do không sửa được Hạn và Cơ quan chủ trì là MỘT câu, dùng chung với phần chưa dựng", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.viSao === LY_DO_KHONG_SUA_HAN)).toBe(true);
    expect(PHAN_CHUA_DUNG.some((p) => p.viSao === LY_DO_KHONG_SUA_CHU_TRI)).toBe(true);
    expect(LY_DO_KHONG_SUA_CHU_TRI).toContain("lead_unit");
    expect(LY_DO_KHONG_SUA_CHU_TRI).toContain("monitor");
  });
});

describe("✎ Sửa — đọc lại trước khi lưu: không gỡ lặng lẽ văn bản người khác vừa thêm", () => {
  it("cùng một tập ⇒ KHÔNG đổi, được lưu", () => {
    expect(vanBanDaDoiOMayChu(VB_GOC, VB_GOC.map((v) => ({ ...v })))).toBe(false);
    expect(vanBanDaDoiOMayChu([], [])).toBe(false);
  });

  it("máy chủ có THÊM một dòng ⇒ ĐÃ ĐỔI — đúng ca lần lưu sẽ gỡ mất dòng ấy", () => {
    const them: petitions_nhiemVuVanBanRa = {
      id: "01JVB3",
      group: "chi-dao-dang-uy",
      reference: "",
      date: "",
      summary: "Công văn giả người khác vừa thêm",
      position: 1,
    };
    expect(vanBanDaDoiOMayChu(VB_GOC, [...VB_GOC, them])).toBe(true);
  });

  it("máy chủ GỠ một dòng, hoặc thay một dòng bằng dòng khác cùng số lượng ⇒ ĐÃ ĐỔI", () => {
    expect(vanBanDaDoiOMayChu(VB_GOC, VB_GOC.slice(0, 1))).toBe(true);
    const thay = [VB_GOC[0], { ...(VB_GOC[1] as petitions_nhiemVuVanBanRa), id: "01JVBKHAC" }];
    expect(vanBanDaDoiOMayChu(VB_GOC, thay as petitions_nhiemVuVanBanRa[])).toBe(true);
  });

  it("sửa trích yếu, số ký hiệu, ngày hoặc nhóm của một dòng ⇒ ĐÃ ĐỔI", () => {
    const sua = (k: Partial<petitions_nhiemVuVanBanRa>) =>
      VB_GOC.map((v) => (v.id === "01JVB1" ? { ...v, ...k } : v));
    expect(vanBanDaDoiOMayChu(VB_GOC, sua({ summary: "Trích yếu giả đã sửa" }))).toBe(true);
    expect(vanBanDaDoiOMayChu(VB_GOC, sua({ reference: "1743-CV/GIA" }))).toBe(true);
    expect(vanBanDaDoiOMayChu(VB_GOC, sua({ date: "2026-06-10" }))).toBe(true);
    expect(vanBanDaDoiOMayChu(VB_GOC, sua({ group: "san-pham-dau-ra" }))).toBe(true);
  });

  it("CHỈ khác thứ tự ⇒ coi là KHÔNG đổi (quyết định có chủ ý: thứ tự do sổ cấp, thân không mang nó)", () => {
    expect(vanBanDaDoiOMayChu(VB_GOC, [...VB_GOC].reverse())).toBe(false);
  });

  it("`position` đổi không tính — nó không nằm trong năm trường so", () => {
    expect(vanBanDaDoiOMayChu(VB_GOC, VB_GOC.map((v) => ({ ...v, position: v.position + 5 })))).toBe(
      false,
    );
  });

  it("thân CÓ `documents` (kể cả `[]`) ⇒ phải đọc lại; thân chỉ có trường vô hướng ⇒ KHÔNG", () => {
    const f = formGoc();
    const coVanBan = thanSuaNhiemVu({ ...f, vanBan: f.vanBan.slice(0, 1) }, chiTiet(), VB_GOC);
    const goHet = thanSuaNhiemVu({ ...f, vanBan: [] }, chiTiet(), VB_GOC);
    const voHuong = thanSuaNhiemVu({ ...f, ghiChu: "Ghi chú giả" }, chiTiet(), VB_GOC);
    expect(coVanBan && canDocLaiTruocKhiLuu(coVanBan)).toBe(true);
    expect(goHet && canDocLaiTruocKhiLuu(goHet)).toBe(true);
    expect(voHuong && canDocLaiTruocKhiLuu(voHuong)).toBe(false);
    expect(canDocLaiTruocKhiLuu({ documents: null })).toBe(false);
  });

  /*
   * `loiSauKhiDocLai` — quyết định của lần `Lưu` trong `FormSuaKhoiVanBan`, tách ra để kiểm không
   * cần DOM. Mỗi ca dưới đây là một PATCH thay cả tập sẽ chạy từ một tập máy chủ KHÔNG xác nhận được.
   */
  const DONG_NGUOI_KHAC: petitions_nhiemVuVanBanRa = {
    id: "01JVB9",
    group: "chi-dao-dang-uy",
    reference: "",
    date: "",
    summary: "Công văn giả người khác vừa thêm",
    position: 1,
  };

  it("đọc lại: cùng tập ⇒ `null` (gửi); tập khác ⇒ chặn bằng `VAN_BAN_VUA_BI_DOI`", () => {
    const ok = (docs: petitions_nhiemVuVanBanRa[]) =>
      ({ ok: true, duLieu: chiTiet({ documents: docs }) }) as const;
    expect(loiSauKhiDocLai(ok(VB_GOC.map((v) => ({ ...v }))), VB_GOC)).toBeNull();
    expect(loiSauKhiDocLai(ok([...VB_GOC, DONG_NGUOI_KHAC]), VB_GOC)).toBe(VAN_BAN_VUA_BI_DOI);
  });

  it("đọc lại HỎNG ⇒ chặn bằng câu máy chủ nguyên văn — không bao giờ `null`", () => {
    expect(loiSauKhiDocLai({ ok: false, thongBao: "Máy chủ giả đang bận." }, VB_GOC)).toBe(
      "Máy chủ giả đang bận.",
    );
  });

  it("đọc lại về mà THIẾU `documents` ⇒ chặn, không đọc thành tập rỗng", () => {
    // Bản chụp RỖNG là ca đắt: đọc `undefined` thành `[]` thì "giống bản chụp", lần lưu đi tiếp,
    // và mọi dòng người khác vừa thêm bị gỡ.
    const thieu = { ok: true, duLieu: chiTiet() } as const;
    expect(loiSauKhiDocLai(thieu, [])).toBe(CHI_TIET_THIEU_VAN_BAN);
    expect(loiSauKhiDocLai(thieu, VB_GOC)).toBe(CHI_TIET_THIEU_VAN_BAN);
  });

  it("trọn luồng `gỡ hết`: thân `[]` ⇒ phải đọc lại ⇒ người khác vừa thêm một dòng ⇒ KHÔNG gửi", () => {
    // Đây là ca một lần bấm xoá mềm cả dòng cán bộ chưa từng thấy nếu bất cứ mắt xích nào hỏng.
    const than = thanSuaNhiemVu({ ...formGoc(), vanBan: [] }, chiTiet(), VB_GOC);
    expect(than).toEqual({ documents: [] });
    expect(than !== null && canDocLaiTruocKhiLuu(than)).toBe(true);
    const docLai = { ok: true, duLieu: chiTiet({ documents: [...VB_GOC, DONG_NGUOI_KHAC] }) } as const;
    expect(loiSauKhiDocLai(docLai, VB_GOC)).toBe(VAN_BAN_VUA_BI_DOI);
  });

  it("câu báo nói rõ CHƯA LƯU GÌ và cán bộ phải làm gì", () => {
    expect(VAN_BAN_VUA_BI_DOI).toContain("vừa được người khác thay đổi");
    expect(VAN_BAN_VUA_BI_DOI).toContain("Chưa lưu gì");
    expect(VAN_BAN_VUA_BI_DOI).toContain("Huỷ");
  });
});
