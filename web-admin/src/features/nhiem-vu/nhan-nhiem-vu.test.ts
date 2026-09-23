import { describe, expect, it } from "vitest";

import {
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  MOI_TRANG_THAI,
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
});
