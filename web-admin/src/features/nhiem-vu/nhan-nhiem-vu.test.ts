import { describe, expect, it } from "vitest";

import type {
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
} from "@/lib/api/schema.gen";

import {
  CHUA_GIAO_BO_PHAN,
  CHUA_PHAN_CONG,
  danhBaChoNhatKy,
  gopTrangNhatKy,
  hienDongNhatKy,
  nhanNguoiNhatKy,
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
  hasTransition,
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
  CODE_EDIT_NOTE,
  DUE_EDIT_NOTE,
  DUE_TIME_LOADING,
  DUE_TIME_NO_CALENDAR,
  DUE_TIME_NO_SHIFT,
  changedSince,
  defaultDueTime,
  dueAtFromInputs,
  dueTimeFilledNote,
  dueTimeHint,
  timeForInput,
  canhBaoSua,
  formSuaTuChiTiet,
  lyDoKhoaSua,
  thanSuaNhiemVu,
  vanBanDaDoiOMayChu,
  canDocLaiTruocKhiLuu,
  CHI_TIET_THIEU_VAN_BAN,
  loiSauKhiDocLai,
  VAN_BAN_VUA_BI_DOI,
  DANG_TAI_DANH_BA,
  DE_BO_PHAN_TU_PHAN_CONG,
  MOI_NGUOI_THUC_HIEN_NHAN,
  cauLoiDanhBaGiaoViec,
  cauLoiDanhBaLoc,
  docDanhBaChonNguoi,
  nhanTrongOChonCanBo,
  duongDanTuLoc,
  locTuDuongDan,
  mucUuTienMacDinh,
  nhanCanBoDrawer,
  nhanCanBoNgan,
  SAP_XEP_MAC_DINH,
  ariaSapXep,
  bamCotSapXep,
  canMoveTask,
  canWriteLogEntry,
  logEntryNote,
  isReopen,
  lacksApprovalFor,
  laBuocTraLai,
  reasonMove,
  transitionNeedsApproval,
  transitionNeedsReason,
  quyenNhiemVu,
  sapXepDayDu,
  NO_PRIORITY_LAST_NOTE,
  yeuCauTraLai,
  type DongVanBanNhap,
  type DongVanBanSua,
  type FormGiaoViecNhap,
  type FormSuaNhiemVu,
} from "./nhan-nhiem-vu";
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU } from "@/lib/quyen";

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

  it("vòng đời là DANH SÁCH CỦA MÁY CHỦ trên dòng — màn hình không giữ bản sao nào", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: các ca ở đây từng ghim bản sao `CHUYEN_DUOC` (§6 chuỗi chặt,
    // `tam-dung` về bốn trạng thái, `hoan-thanh` ngõ cụt). Máy chủ nay trả `allowed_transitions` trên
    // mỗi dòng (3b2330b) theo bảng require; màn hình chỉ đọc nó.
    const row = { allowed_transitions: ["cho-duyet", "hoan-thanh", "tam-dung"] };
    expect(hasTransition(row, "hoan-thanh")).toBe(true);
    expect(hasTransition(row, "da-tiep-nhan")).toBe(false);
    expect(hasTransition({ allowed_transitions: [] }, "tam-dung")).toBe(false);
  });

  it("bước trả lại là một CẶP (từ, sang) — bước thuận `da-tiep-nhan` → `dang-thuc-hien` KHÔNG phải", () => {
    expect(laBuocTraLai("cho-duyet", "dang-thuc-hien")).toBe(true);
    expect(laBuocTraLai("da-tiep-nhan", "dang-thuc-hien")).toBe(false);
    expect(laBuocTraLai("tam-dung", "dang-thuc-hien")).toBe(false);
    expect(laBuocTraLai("cho-duyet", "hoan-thanh")).toBe(false);
  });

  it("lý do trả lại: rỗng hoặc toàn khoảng trắng ⇒ không gửi; có chữ ⇒ đích `dang-thuc-hien`, đã cắt", () => {
    // Máy chủ cắt khoảng trắng rồi mới kiểm (`KiemLyDoTraLai`), nên `"   "` là 400 y như `""`.
    expect(yeuCauTraLai("")).toBeNull();
    expect(yeuCauTraLai("   \n\t ")).toBeNull();
    expect(yeuCauTraLai("  Thiếu biên bản nghiệm thu \n")).toEqual({
      trangThai: "dang-thuc-hien",
      ghiChu: "Thiếu biên bản nghiệm thu",
    });
  });

  it("cần `task.approve`: vào `hoan-thanh`, mở lại, trả lại — đúng `NeedsApproval` của máy chủ", () => {
    expect(transitionNeedsApproval("dang-thuc-hien", "hoan-thanh")).toBe(true);
    expect(transitionNeedsApproval("cho-duyet", "hoan-thanh")).toBe(true);
    expect(transitionNeedsApproval("hoan-thanh", "dang-thuc-hien")).toBe(true);
    expect(transitionNeedsApproval("cho-duyet", "dang-thuc-hien")).toBe(true);
    expect(transitionNeedsApproval("da-tiep-nhan", "dang-thuc-hien")).toBe(false);
    expect(transitionNeedsApproval("tam-dung", "dang-thuc-hien")).toBe(false);
    expect(transitionNeedsApproval("dang-thuc-hien", "cho-duyet")).toBe(false);
  });

  it("mở lại là một CẶP (hoan-thanh → dang-thuc-hien), và nó cần lý do như bước trả lại", () => {
    expect(isReopen("hoan-thanh", "dang-thuc-hien")).toBe(true);
    expect(isReopen("tam-dung", "dang-thuc-hien")).toBe(false);
    expect(transitionNeedsReason("hoan-thanh", "dang-thuc-hien")).toBe(true);
    expect(transitionNeedsReason("cho-duyet", "dang-thuc-hien")).toBe(true);
    expect(transitionNeedsReason("da-tiep-nhan", "dang-thuc-hien")).toBe(false);
  });

  const NGUOI_THUC_HIEN = "CB-2026-3H8N2W";
  const NGUOI_KHAC = "CB-2026-0P4X1Z";
  const DU = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
  const CHI_CAP_NHAT = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);
  const CHI_DUYET = quyenNhiemVu([QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
  const KHONG = quyenNhiemVu([]);
  const choDuyet = {
    status: "cho-duyet",
    assignee: NGUOI_THUC_HIEN,
    allowed_transitions: ["hoan-thanh", "dang-thuc-hien"],
  };
  const daXong = { status: "hoan-thanh", assignee: NGUOI_THUC_HIEN, allowed_transitions: ["dang-thuc-hien"] };

  it("cổng dòng: `task.update` HOẶC đúng người thực hiện; phiên rỗng không khớp gì", () => {
    expect(canMoveTask(CHI_CAP_NHAT, choDuyet, NGUOI_KHAC)).toBe(true);
    expect(canMoveTask(KHONG, choDuyet, NGUOI_THUC_HIEN)).toBe(true);
    expect(canMoveTask(KHONG, choDuyet, NGUOI_KHAC)).toBe(false);
    expect(canMoveTask(CHI_DUYET, choDuyet, NGUOI_KHAC)).toBe(false);
    expect(canMoveTask(KHONG, { assignee: "" }, "")).toBe(false);
  });

  it("ô lý do: trả lại ở `cho-duyet`, mở lại ở `hoan-thanh` — chỉ khi đi được bước ấy", () => {
    expect(reasonMove(choDuyet, DU, NGUOI_KHAC)).toBe("return");
    expect(reasonMove(daXong, DU, NGUOI_KHAC)).toBe("reopen");
    // Người thực hiện có `task.approve` mà không có `task.update`: vẫn đi được (ea55113).
    expect(reasonMove(daXong, CHI_DUYET, NGUOI_THUC_HIEN)).toBe("reopen");
    // Thiếu `task.approve`.
    expect(reasonMove(choDuyet, CHI_CAP_NHAT, NGUOI_KHAC)).toBeNull();
    expect(reasonMove(daXong, CHI_CAP_NHAT, NGUOI_THUC_HIEN)).toBeNull();
    // Có khoá mà không phải người của dòng, và không có `task.update`.
    expect(reasonMove(daXong, CHI_DUYET, NGUOI_KHAC)).toBeNull();
    // Máy chủ không liệt kê bước ấy ⇒ không có ô.
    expect(reasonMove({ ...daXong, allowed_transitions: [] }, DU, NGUOI_KHAC)).toBeNull();
    expect(reasonMove({ ...choDuyet, status: "da-tiep-nhan" }, DU, NGUOI_KHAC)).toBeNull();
  });

  it("câu thiếu quyền duyệt: chỉ khi đi được dòng, máy chủ liệt kê bước cần duyệt, và thiếu khoá", () => {
    expect(lacksApprovalFor(choDuyet, CHI_CAP_NHAT, NGUOI_KHAC)).toBe(true);
    expect(lacksApprovalFor(daXong, KHONG, NGUOI_THUC_HIEN)).toBe(true);
    expect(lacksApprovalFor(choDuyet, DU, NGUOI_KHAC)).toBe(false);
    // Không đi được dòng thì không nói chuyện quyền duyệt — cả khối đã ẩn.
    expect(lacksApprovalFor(choDuyet, KHONG, NGUOI_KHAC)).toBe(false);
    expect(
      lacksApprovalFor({ status: "da-tiep-nhan", assignee: "", allowed_transitions: ["dang-thuc-hien", "tam-dung"] }, CHI_CAP_NHAT, NGUOI_KHAC),
    ).toBe(false);
  });

  it("`hoan-thanh` là \"đã xong\" THEO TÊN; `chuyen-tiep` cũ không còn là ngõ cụt", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: `ketThuc` từng là "không có lối ra", nên `chuyen-tiep` là ngõ cụt.
    // Máy chủ: hỏi `hoan-thanh` theo tên (`nhiem_vu.go:78-83`); dòng `chuyen-tiep` cũ đi tiếp được.
    expect(ketThuc("hoan-thanh")).toBe(true);
    expect(ketThuc("chuyen-tiep")).toBe(false);
    expect(ketThuc("tam-dung")).toBe(false);
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

  it("hai bộ lọc máy chủ từng TỪ CHỐI đã rời danh sách — nay được phục vụ (W5, 3a4e60f)", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5): ca này ghim hai mục `Liên quan đến tôi` và `Sắp đến hạn`
    // CÓ MẶT. Máy chủ nay phục vụ `scope=related` và `soon=true`, và màn hình đã vẽ cả hai.
    const moiLyDo = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(moiLyDo).not.toContain("Liên quan đến tôi");
    expect(moiLyDo).not.toContain("Sắp đến hạn");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: bài này từng canh chữ `task.assign` và `chuyen-tiep` — hai mục
    // "Ô đổi bộ phận / người thực hiện" và "`Chuyển tiếp` … phần máy chủ đang làm". Cả hai nay đã
    // dựng (khối §5.7, `POST …/assignment`), nên chúng rời danh sách; còn nằm đó là đẩy người sau
    // đi dựng lại thứ đã có.
    expect(moiLyDo).not.toContain("task.assign");
    expect(moiLyDo).not.toContain("Phần máy chủ cho nghĩa ấy đang làm");
  });

  it("ba ô chọn cán bộ ĐÃ DỰNG (TASK-05): mục cũ rời danh sách, chỉ còn mục ô tìm theo tên", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026: mục "Ô chọn `Người thực hiện` · …" từng giải thích vì sao ba
    // ô là ô gõ mã (danh bạ đứng sau `admin.user`). Ba ô nay đổ từ `GET /api/v1/staff-directory`.
    // Một mục còn nằm đó sau khi đã dựng là mục đẩy người sau đi dựng lại thứ đã có.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Ô chọn `Người thực hiện`"))).toBeUndefined();
    const moiLyDo = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(moiLyDo).not.toContain("admin.user");
    expect(moiLyDo).not.toContain("ô gõ mã cán bộ");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (TASK-02 lượt web 1): ô tìm theo tên đã dựng
    // (`components/staff-combobox.tsx`), nên mục `Gõ tên để tìm…` cũng rời danh sách.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.includes("Gõ tên để tìm…"))).toBeUndefined();
  });

  it("giao lại §5.7 (28/09/2026): đúng BA mục rời — 10 → 7 — còn lại nguyên", () => {
    // A literal count on purpose. Pass 1 went 16 → 14 (#12, #14). Pass 2 removed exactly four
    // entries — #6 chip/children, #10 add child/move parent, #11 decide in the drawer, #15 real
    // column counts — and REWROTE (did not remove) the sort entry (#13), which keeps `Tên việc` and
    // `Ưu tiên`.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (giao lại, `task.assign`): 10 → 7. Exactly three entries left,
    // all built by the §5.7 block — the unit/assignee box, `Chuyển tiếp`, and editing lead unit /
    // monitor. An 8 means one was left behind; a 6 means an unrelated entry was lost.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): 7 → 6 — `Sửa Hạn hoàn thành` was built.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3b): 6 → 5 — the sort entry was built (backend P9).
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5): 5 → 3 — `Liên quan đến tôi` and `Sắp đến hạn` were built.
    expect(PHAN_CHUA_DUNG.length).toBe(3);
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join(" | ");
    expect(ten).not.toContain("việc con");
    expect(ten).not.toContain("VIỆC CON");
    expect(ten).not.toContain("ngay trong drawer");
    expect(ten).not.toContain("SỐ LƯỢNG THẬT");
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Sắp xếp theo"))).toBeUndefined();
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): the entry "Sửa `Hạn hoàn thành`" (kept on purpose by the
    // owner's earlier decision) is gone — the owner adopted require 93cff7f and the form now edits it.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Sửa `Hạn hoàn thành`"))).toBeUndefined();
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: the three entries the §5.7 block built are gone.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("`Chuyển tiếp`"))).toBeUndefined();
    expect(ten).not.toContain("Ô đổi bộ phận / người thực hiện");
    expect(ten).not.toContain("Cơ quan chủ trì tham mưu");
  });

  it("hai mục về văn bản chỉ đạo nói ĐÚNG thứ còn thiếu hôm nay, không còn nói bảng chưa có", () => {
    // SỬA CÓ CHỦ Ý 24/09/2026: hai mục này từng viết "bảng `nhiem_vu_van_ban` chưa tồn tại". Bảng
    // đã có từ migration 0009; một lý do sai trên màn là lý do đẩy người sau đi dựng lại thứ đã có.
    //
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (TASK-03): mục `Nút ✎ Sửa của khối SỔ THEO DÕI…` đã dựng xong
    // nên BIẾN KHỎI danh sách.
    //
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026 (TASK-04): mục `Ô Ghi chú ở form Giao việc mới` cũng biến khỏi
    // danh sách — `POST /api/v1/tasks` nay nhận `note` (0a41e48) và form tạo đã có ô ấy.
    const mucSua = PHAN_CHUA_DUNG.find((p) => p.ten.includes("✎ Sửa"));
    const mucGhiChu = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Ghi chú` ở form `Giao việc mới`"));
    const muc43 = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Sổ theo dõi` (§4.3)"));
    expect(mucSua).toBeUndefined();
    expect(mucGhiChu).toBeUndefined();
    expect(muc43).toBeDefined();
    expect(muc43?.viSao).not.toContain("CHƯA TỒN TẠI");
    expect(muc43?.viSao).not.toContain("chưa tồn tại");
    expect(PHAN_CHUA_DUNG.map((p) => p.viSao).join(" ")).not.toContain("không nhận `note`");
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
    ghiChu: "",
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

describe("thân `Giao việc mới` — ô `Ghi chú` §7.2", () => {
  it("gõ ghi chú ⇒ `note` lên dây, đã cắt khoảng trắng", () => {
    const than = thanGiaoViec(formDay({ ghiChu: "  Ghi chú giả khi giao việc  " }), {
      coDanhSachVanBan: true,
    });
    expect(than.note).toBe("Ghi chú giả khi giao việc");
  });

  it("bỏ trống hoặc toàn khoảng trắng ⇒ `note` VẮNG MẶT, không gửi `\"\"`", () => {
    expect(thanGiaoViec(formDay(), { coDanhSachVanBan: true })).not.toHaveProperty("note");
    const toanKhoangTrang = formDay({ ghiChu: "  \n\t  " });
    expect(thanGiaoViec(toanKhoangTrang, { coDanhSachVanBan: true })).not.toHaveProperty("note");
  });

  it("loại `co-ban` (§7.3 bỏ ô) và màn Biên bản (tuyến không có `note`) ⇒ KHÔNG gửi `note`", () => {
    const f = formDay({ ghiChu: "Ghi chú giả" });
    expect(thanGiaoViec({ ...f, loai: "co-ban" }, { coDanhSachVanBan: true })).not.toHaveProperty(
      "note",
    );
    expect(thanGiaoViec(f, { coDanhSachVanBan: false })).not.toHaveProperty("note");
  });
});

describe("thân `Giao việc mới` — ba ô cán bộ lên ĐÚNG trường, bằng MÃ `CB-…`", () => {
  // ĐẮT NẾU SAI, VÀ IM LẶNG: `assigner` là lãnh đạo giao việc — theo ADR 0038 là NGƯỜI DUY NHẤT duyệt
  // được đề nghị lùi hạn, và `PATCH` cố ý không sửa được nó. Tráo `assignee` ↔ `assigner` là người
  // thực hiện tự duyệt lùi hạn cho chính mình, còn lãnh đạo mất quyền — máy chủ lưu bất kỳ mã nào nó
  // nhận, nên không bài kiểm nào khác đỏ. Ba mã KHÁC NHAU để một phép tráo không thể trùng hợp đúng.
  const THUC_HIEN = "CB-2026-GIATH1";
  const LANH_DAO = "CB-2026-GIALD2";
  const CHUYEN_VIEN = "CB-2026-GIACV3";

  it("`Người thực hiện` ⇒ `assignee`, `Lãnh đạo giao việc` ⇒ `assigner`, `Chuyên viên` ⇒ `monitor`", () => {
    const than = thanGiaoViec(
      formDay({
        nguoiThucHien: ` ${THUC_HIEN} `,
        lanhDaoGiaoViec: ` ${LANH_DAO} `,
        chuyenVien: CHUYEN_VIEN,
      }),
      { coDanhSachVanBan: true },
    );
    expect(than.assignee).toBe(THUC_HIEN);
    expect(than.assigner).toBe(LANH_DAO);
    expect(than.monitor).toBe(CHUYEN_VIEN);
  });

  it("bỏ trống hoặc toàn khoảng trắng ⇒ trường VẮNG MẶT, không gửi `\"\"`", () => {
    const than = thanGiaoViec(formDay({ nguoiThucHien: "", lanhDaoGiaoViec: "  " }), {
      coDanhSachVanBan: true,
    });
    expect(than).not.toHaveProperty("assignee");
    expect(than).not.toHaveProperty("assigner");
  });

  it("màn Biên bản (Tách kết luận) dùng cùng hàm: lãnh đạo giao việc vẫn lên `assigner`", () => {
    const than = thanGiaoViec(formDay({ lanhDaoGiaoViec: LANH_DAO }), { coDanhSachVanBan: false });
    expect(than.assigner).toBe(LANH_DAO);
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
    child_count: 0,
    allowed_transitions: [],
    updated_at: "2026-06-01T02:00:00Z",
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
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): every body now also carries `expected_updated_at` — the
    // `updated_at` of the task as the form opened it (d2ed15e) — so a concurrent write is a 409.
    expect(thanSuaNhiemVu({ ...formGoc(), ghiChu: " Ghi chú giả mới " }, chiTiet(), VB_GOC)).toEqual({
      note: "Ghi chú giả mới",
      expected_updated_at: "2026-06-01T02:00:00Z",
    });
    expect(thanSuaNhiemVu({ ...formGoc(), capTrenCongNhan: true }, chiTiet(), VB_GOC)).toEqual({
      superior_acknowledged: true,
      expected_updated_at: "2026-06-01T02:00:00Z",
    });
    expect(thanSuaNhiemVu({ ...formGoc(), tomTatKetQua: "Đã xong giả" }, chiTiet(), VB_GOC)).toEqual({
      result_summary: "Đã xong giả",
      expected_updated_at: "2026-06-01T02:00:00Z",
    });
  });

  it("xoá trắng ghi chú là MỘT THAY ĐỔI — gửi chuỗi rỗng, không bỏ qua", () => {
    const f = { ...formSuaTuChiTiet(chiTiet({ note: "Cũ" }), VB_GOC), ghiChu: "" };
    expect(thanSuaNhiemVu(f, chiTiet({ note: "Cũ" }), VB_GOC)).toEqual({ note: "", expected_updated_at: "2026-06-01T02:00:00Z" });
  });

  it("không đụng tới văn bản ⇒ `documents` VẮNG MẶT", () => {
    const than = thanSuaNhiemVu({ ...formGoc(), tieuDe: "Tiêu đề giả mới" }, chiTiet(), VB_GOC);
    expect(than).toEqual({ title: "Tiêu đề giả mới", expected_updated_at: "2026-06-01T02:00:00Z" });
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
    expect(thanSuaNhiemVu({ ...formGoc(), vanBan: [] }, chiTiet(), VB_GOC)).toEqual({
      documents: [],
      expected_updated_at: "2026-06-01T02:00:00Z",
    });
  });

  it("KHÔNG đổi mã, KHÔNG đổi hạn ⇒ không có `code`, `due_at`; không bao giờ `assigner`, `lead_unit`, `monitor`, `position`", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): `code` và `due_at` từng "không bao giờ" có. Nay chúng đi
    // khi cán bộ ĐỔI chúng (ca riêng bên dưới); ca này ghim rằng không đổi thì chúng vắng mặt.
    const f: FormSuaNhiemVu = {
      ...formGoc(),
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

  it("Hạn nay sửa được (không còn mục chưa dựng); Cơ quan chủ trì nay CHỈ đường sang §5.7", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): ca này ghim câu `LY_DO_KHONG_SUA_HAN` trong phần chưa dựng.
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("Hạn hoàn thành"))).toBe(false);
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (quyết định của người dùng): câu này từng nói "`PATCH` không
    // nhận `lead_unit` và `monitor`" và đứng trong phần chưa dựng. Hai trường ấy nay sửa ở khối
    // Giao việc, chuyển việc (`task.assign`), nên câu chỉ đường sang đó và rời phần chưa dựng.
    expect(PHAN_CHUA_DUNG.some((p) => p.viSao === LY_DO_KHONG_SUA_CHU_TRI)).toBe(false);
    expect(LY_DO_KHONG_SUA_CHU_TRI).toContain("“Giao việc, chuyển việc”");
    expect(LY_DO_KHONG_SUA_CHU_TRI).not.toContain("không nhận");
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
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): the body now also carries the lock token (d2ed15e). The
    // re-read stays: it keeps the typed text and names the cause before the server's 409 would.
    expect(than).toEqual({ documents: [], expected_updated_at: "2026-06-01T02:00:00Z" });
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

describe("danh bạ chọn người cho ba ô chọn cán bộ — BA pha, không hai", () => {
  const CB = { code: "CB-00001", full_name: "Cán bộ giả", position: "", department_id: "" };

  it("chưa đọc xong (`null`) ⇒ đang tải, không lỗi, không lựa chọn nào", () => {
    expect(docDanhBaChonNguoi(null)).toEqual({ ds: [], dangTai: true, loi: null });
  });

  it("đọc hỏng ⇒ KHÔNG đang tải, câu máy chủ NGUYÊN VĂN, không lựa chọn nào — không đoán ai", () => {
    expect(docDanhBaChonNguoi({ ok: false, thongBao: "Bạn chưa đăng nhập." })).toEqual({
      ds: [],
      dangTai: false,
      loi: "Bạn chưa đăng nhập.",
    });
  });

  it("đọc được ⇒ đúng `items` máy chủ trả, mã là mã nghiệp vụ", () => {
    const db = docDanhBaChonNguoi({ ok: true, duLieu: { items: [CB] } });
    expect(db).toEqual({ ds: [CB], dangTai: false, loi: null });
    expect(db.ds[0]?.code).toMatch(/^CB-/);
  });

  it("đọc được mà RỖNG khác đang tải: ô không khoá, lựa chọn rỗng mang nghĩa của ô", () => {
    const rong = docDanhBaChonNguoi({ ok: true, duLieu: { items: [] } });
    expect(rong.dangTai).toBe(false);
    expect(nhanTrongOChonCanBo(rong, DE_BO_PHAN_TU_PHAN_CONG)).toBe(DE_BO_PHAN_TU_PHAN_CONG);
  });

  it("chữ lựa chọn rỗng: đang tải thì nói đang tải; hỏng thì vẫn là nghĩa của ô", () => {
    expect(nhanTrongOChonCanBo(docDanhBaChonNguoi(null), MOI_NGUOI_THUC_HIEN_NHAN)).toBe(
      DANG_TAI_DANH_BA,
    );
    const hong = docDanhBaChonNguoi({ ok: false, thongBao: "x" });
    expect(nhanTrongOChonCanBo(hong, MOI_NGUOI_THUC_HIEN_NHAN)).toBe(MOI_NGUOI_THUC_HIEN_NHAN);
    expect(DE_BO_PHAN_TU_PHAN_CONG).toBe("— Để bộ phận tự phân công —");
  });

  it("câu lỗi của ô lọc mang câu máy chủ và nói ô còn lại gì", () => {
    const cau = cauLoiDanhBaLoc("Máy chủ bận.");
    expect(cau).toContain("Máy chủ bận.");
    expect(cau).toContain(MOI_NGUOI_THUC_HIEN_NHAN);
  });

  it("câu lỗi của form Giao việc NÓI HỆ QUẢ: lãnh đạo giao việc không ghi lại được sau khi tạo", () => {
    const cau = cauLoiDanhBaGiaoViec("Máy chủ bận.");
    expect(cau).toContain("Máy chủ bận.");
    expect(cau).toContain("lãnh đạo giao việc không ghi lại được sau khi tạo");
  });
});

describe("§5.9 Nhật ký & Trao đổi — nửa đọc", () => {
  const CB = "CB-00311";
  const DANH_BA = new Map([
    [CB, { code: CB, full_name: "Nguyễn Văn A", position: "Chuyên viên", department_id: "" }],
    ["CB-00999", { code: "CB-00999", full_name: "", position: "", department_id: "" }],
  ]);
  const TEN_BO_PHAN = new Map([["bp-vpdu", "VĂN PHÒNG ĐẢNG ỦY"]]);

  function dong(sua: Partial<petitions_nhatKyNhiemVuRa> = {}): petitions_nhatKyNhiemVuRa {
    return {
      id: "nknv-1",
      at: "2026-09-09T07:20:00Z",
      actor_code: CB,
      status: "dang-thuc-hien",
      unit: "",
      assignee: "",
      note: "Bắt đầu thực hiện.",
      ...sua,
    };
  }

  it("người ghi: có họ tên thì `Họ tên (mã)` — MÃ VẪN CÒN trên dòng", () => {
    expect(nhanNguoiNhatKy(CB, DANH_BA)).toBe("Nguyễn Văn A (CB-00311)");
  });

  it("danh bạ chưa có / không có người ấy / họ tên rỗng → hiện MÃ, không bao giờ ô trống", () => {
    expect(nhanNguoiNhatKy(CB, null)).toBe(CB);
    expect(nhanNguoiNhatKy("CB-00001", DANH_BA)).toBe("CB-00001");
    expect(nhanNguoiNhatKy("CB-00999", DANH_BA)).toBe("CB-00999");
    expect(nhanNguoiNhatKy("", DANH_BA)).toBe(O_TRONG);
  });

  it("danh bạ đang tải hoặc tải hỏng → `null` (dòng hiện mã); đọc được → bảng tra theo mã", () => {
    expect(danhBaChoNhatKy(null)).toBeNull();
    expect(danhBaChoNhatKy({ ok: false, thongBao: "Máy chủ bận." })).toBeNull();
    const bang = danhBaChoNhatKy({ ok: true, duLieu: { items: [...DANH_BA.values()] } });
    expect(bang?.get(CB)?.full_name).toBe("Nguyễn Văn A");
  });

  it("dòng → chữ: giờ Việt Nam, nhãn trạng thái CỦA XÃ, ghi chú nguyên văn", () => {
    const bangXa = {
      ...BANG_NHAN_MAC_DINH,
      nhan: { ...BANG_NHAN_MAC_DINH.nhan, "dang-thuc-hien": "Đang làm" },
    };
    const h = hienDongNhatKy(dong(), bangXa, DANH_BA, TEN_BO_PHAN);
    expect(h.thoiDiem).toContain("14:20");
    expect(h.thoiDiem).toContain("09/09/2026");
    expect(h.luc).toBe("2026-09-09T07:20:00Z");
    expect(h.trangThai).toBe("Đang làm");
    expect(h.nguoi).toBe("Nguyễn Văn A (CB-00311)");
    expect(h.ghiChu).toBe("Bắt đầu thực hiện.");
  });

  it("dòng KHÔNG đổi phân công (unit và assignee rỗng) thì không có dòng bộ phận/phụ trách", () => {
    expect(hienDongNhatKy(dong(), BANG_NHAN_MAC_DINH, DANH_BA, TEN_BO_PHAN).phanCong).toBeNull();
  });

  it("dòng đổi phân công: tên bộ phận + người phụ trách; vế rỗng nói trạng thái thật", () => {
    expect(
      hienDongNhatKy(dong({ unit: "bp-vpdu", assignee: CB }), BANG_NHAN_MAC_DINH, DANH_BA, TEN_BO_PHAN)
        .phanCong,
    ).toBe("VĂN PHÒNG ĐẢNG ỦY · Nguyễn Văn A (CB-00311)");
    expect(
      hienDongNhatKy(dong({ unit: "bp-vpdu" }), BANG_NHAN_MAC_DINH, DANH_BA, TEN_BO_PHAN).phanCong,
    ).toBe(`VĂN PHÒNG ĐẢNG ỦY · ${CHUA_PHAN_CONG}`);
    expect(
      hienDongNhatKy(dong({ assignee: CB }), BANG_NHAN_MAC_DINH, null, TEN_BO_PHAN).phanCong,
    ).toBe(`${CHUA_GIAO_BO_PHAN} · ${CB}`);
    // Bộ phận không có trong danh mục: hiện id, không bịa tên và không để trống.
    expect(
      hienDongNhatKy(dong({ unit: "bp-la" }), BANG_NHAN_MAC_DINH, null, TEN_BO_PHAN).phanCong,
    ).toBe(`bp-la · ${CHUA_PHAN_CONG}`);
  });

  it("gộp trang: nối theo thứ tự máy chủ, BỎ dòng trùng `id`, giữ bản đã có", () => {
    const a = dong({ id: "a" });
    const b = dong({ id: "b", note: "cũ" });
    const bLai = dong({ id: "b", note: "trôi sang trang sau" });
    const c = dong({ id: "c" });
    const ra = gopTrangNhatKy([a, b], [bLai, c]);
    expect(ra.map((d) => d.id)).toEqual(["a", "b", "c"]);
    expect(ra[1]?.note).toBe("cũ");
    // Trùng ngay TRONG trang mới cũng chỉ một lần.
    expect(gopTrangNhatKy([], [c, c]).map((d) => d.id)).toEqual(["c"]);
  });

  it("PHAN_CHUA_DUNG: mục cũ `Nhật ký & Trao đổi` đã rời; chỉ còn ô ghi tay — `Tiếp tục` đã dựng", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026 (TASK-06): khối nhật ký nay đọc được. Mục cũ nói "hợp đồng
    // không có tuyến nhật ký nào" — một lý do sai trên màn là lý do đẩy người sau đi dựng lại
    // thứ đã có.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W1): `Tiếp tục` sau tạm dừng rời mục này — màn hình vẽ đúng
    // danh sách máy chủ trả (`allowed_transitions`), luật "trạng thái trước lúc dừng" đã bỏ.
    expect(
      PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("Nhật ký & Trao đổi (§5.9)")),
    ).toBeUndefined();
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W2): ô ghi tay nay GHI được (60011e8); mục chỉ còn `📎 Đính kèm`,
    // thứ máy chủ không nhận. Mục cũ nói "chờ luật người đang giữ việc" — luật ấy đã có.
    const muc = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Ghi nhật ký (§5.9)"));
    expect(muc).toBeDefined();
    expect(muc?.ten).toContain("📎 Đính kèm");
    expect(PHAN_CHUA_DUNG.some((p) => `${p.ten} ${p.viSao}`.includes("Tiếp tục"))).toBe(false);
    expect(muc?.viSao).toContain("/log-entries");
    expect(muc?.viSao).not.toContain("người đang giữ việc");
    expect(PHAN_CHUA_DUNG.map((p) => p.viSao).join(" ")).not.toContain(
      "không có tuyến nhật ký nào",
    );
  });
});

describe("PHAN_CHUA_DUNG — Duyệt / Từ chối lùi hạn (TASK-07)", () => {
  it("mục cũ `của người khác` đã rời; chỉ còn phần drawer không tự tìm được đề nghị của chính nó", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026: hàng chờ `GET /api/v1/task-extensions` nay có và đã dựng ở mục
    // `Đề nghị lùi hạn chờ duyệt`. Mục cũ nói "hợp đồng KHÔNG có tuyến nào liệt kê đề nghị" — một
    // lý do sai trên màn là lý do đẩy người sau đi dựng lại thứ đã có.
    expect(
      PHAN_CHUA_DUNG.find((p) => p.ten.includes("đề nghị lùi hạn của người khác")),
    ).toBeUndefined();
    expect(PHAN_CHUA_DUNG.map((p) => p.viSao).join(" ")).not.toContain(
      "KHÔNG có tuyến nào liệt kê đề nghị",
    );
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (TASK-03 lượt web 2): tuyến nay nhận `task=NV19`, và drawer có
    // khối đề nghị đang chờ của chính nhiệm vụ ấy — mục "ngay trong drawer" rời danh sách.
    expect(PHAN_CHUA_DUNG.find((p) => p.ten.includes("ngay trong drawer"))).toBeUndefined();
    expect(PHAN_CHUA_DUNG.map((p) => p.viSao).join(" ")).not.toContain(
      "KHÔNG lọc được theo nhiệm vụ",
    );
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHÓM A (27/09/2026)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("câu `chưa ghi lãnh đạo giao việc` nói ĐÚNG điều làm được", () => {
  it("không còn bảo `bổ sung` — `PATCH` không nhận `assigner`, nên lời khuyên ấy không làm theo được", () => {
    expect(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC).not.toContain("hãy bổ sung");
    expect(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC).toContain("chỉ ghi được lúc giao việc");
    expect(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC).toContain("không bổ sung được");
  });
});

describe("tên cán bộ ở ô chật — họ tên, mã lạ là mã, rỗng là câu của ô", () => {
  const DANH_BA = new Map([
    ["CB-1", { code: "CB-1", full_name: "Lê Văn Một", position: "", department_id: "" }],
    ["CB-2", { code: "CB-2", full_name: "", position: "", department_id: "" }],
  ]);

  it("ba nhánh, không nhánh nào ra chuỗi rỗng cho một mã có thật", () => {
    expect(nhanCanBoNgan("CB-1", DANH_BA, CHUA_PHAN_CONG)).toBe("Lê Văn Một");
    // Danh bạ có dòng nhưng họ tên rỗng: vẫn là mã — ô trống đọc ra là "chưa giao cho ai".
    expect(nhanCanBoNgan("CB-2", DANH_BA, CHUA_PHAN_CONG)).toBe("CB-2");
    expect(nhanCanBoNgan("CB-9", DANH_BA, CHUA_PHAN_CONG)).toBe("CB-9");
    expect(nhanCanBoNgan("CB-1", null, CHUA_PHAN_CONG)).toBe("CB-1");
    expect(nhanCanBoNgan("", DANH_BA, CHUA_PHAN_CONG)).toBe(CHUA_PHAN_CONG);
  });

  it("drawer giữ MÃ cạnh họ tên — chỗ đối chiếu hồ sơ", () => {
    expect(nhanCanBoDrawer("CB-1", DANH_BA, O_TRONG)).toBe("Lê Văn Một (CB-1)");
    expect(nhanCanBoDrawer("CB-9", DANH_BA, O_TRONG)).toBe("CB-9");
    expect(nhanCanBoDrawer("", DANH_BA, O_TRONG)).toBe(O_TRONG);
  });
});

describe("§7.1 — `mucUuTienMacDinh`", () => {
  const m = (code: string, is_default: boolean, active = true) => ({
    id: code,
    code,
    label: code,
    is_default,
    active,
    order: 1,
    source: "he-thong",
    tier: 1,
  });

  it("dòng xã đặt mặc định, đang dùng — không phải dòng tên `Thường`", () => {
    expect(mucUuTienMacDinh([m("thuong", false), m("cao", true)])).toBe("cao");
  });

  it("không có dòng mặc định, hoặc dòng ấy đã ngừng dùng: `\"\"` (= Chưa xác định)", () => {
    expect(mucUuTienMacDinh([m("thuong", false)])).toBe("");
    expect(mucUuTienMacDinh([m("thuong", true, false)])).toBe("");
    expect(mucUuTienMacDinh([])).toBe("");
  });
});

describe("§3 — bộ lọc ↔ đường dẫn", () => {
  const DAY_DU = {
    phamVi: "mine" as const,
    trangThai: "cho-duyet",
    nguonGiao: "van-ban-den",
    loai: "theo-van-ban",
    khoi: "khoi-dang",
    mucUuTien: "khan",
    boPhanID: "01JBOPHAN",
    nguoiThucHienMa: "CB-2026-3H8N2W",
    chiTreHan: true as const,
  };

  it("đi rồi về: ghi ra đường dẫn rồi đọc lại được ĐÚNG bộ lọc ấy", () => {
    expect(locTuDuongDan(duongDanTuLoc(DAY_DU))).toEqual(DAY_DU);
    expect(locTuDuongDan(`?${duongDanTuLoc(DAY_DU)}`)).toEqual(DAY_DU);
  });

  it("tên tham số là tên của tuyến `GET /api/v1/tasks` — đường dẫn chia sẻ đọc lên đúng câu hỏi", () => {
    const t = new URLSearchParams(duongDanTuLoc(DAY_DU));
    expect([...t.keys()].sort()).toEqual(
      ["assignee", "bloc", "late", "priority", "scope", "source", "status", "type", "unit"].sort(),
    );
  });

  it("KHÔNG BAO GIỜ đưa chữ tìm lên thanh địa chỉ (luật 3, cấm #4)", () => {
    const chuoi = duongDanTuLoc({ ...DAY_DU, tim: "Nguyễn Văn A 0900000000" });
    expect(chuoi).not.toContain("q=");
    expect(decodeURIComponent(chuoi)).not.toContain("Nguyễn");
    expect(locTuDuongDan("?q=Nguy%E1%BB%85n")).toEqual({});
  });

  it("không lọc gì: chuỗi rỗng; `Toàn xã` và ô tick bỏ trống thì tham số vắng mặt hẳn", () => {
    expect(duongDanTuLoc({})).toBe("");
    expect(duongDanTuLoc({ phamVi: "all", chiTreHan: false })).toBe("");
  });

  it("tham số lạ bị bỏ qua; giá trị máy chủ trả 400 không được lọt vào", () => {
    // Một đường dẫn gõ sai không được biến quyển sổ thành một trang lỗi.
    expect(
      locTuDuongDan(
        // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5): `scope=related` is a real value now — it moved to the
        // round-trip case below. `soon=72` is still refused: only `true` exists.
        "?status=xong-roi&source=zalo&late=false&scope=nobody&soon=72&che_do_xem=kanban&foo=1",
      ),
    ).toEqual({});
    expect(locTuDuongDan("?late=TRUE")).toEqual({});
  });

  it("mã tự do rỗng hoặc dài bất thường bị bỏ, không gửi lên máy chủ", () => {
    expect(locTuDuongDan("?assignee=&unit=%20%20")).toEqual({});
    expect(locTuDuongDan(`?type=${"x".repeat(101)}`)).toEqual({});
    expect(locTuDuongDan("?type=%20co-ban%20")).toEqual({ loai: "co-ban" });
  });

  it("sắp xếp đi rồi về qua đường dẫn — `sort`/`order`, đúng tên tuyến", () => {
    const coSapXep = { ...DAY_DU, sapXep: "code" as const, chieu: "asc" as const };
    expect(locTuDuongDan(duongDanTuLoc(coSapXep))).toEqual(coSapXep);
    const t = new URLSearchParams(duongDanTuLoc(coSapXep));
    expect(t.get("sort")).toBe("code");
    expect(t.get("order")).toBe("asc");
    // Con trỏ KHÔNG lên đường dẫn — nó gắn với đúng một cách sắp và sống vài giây.
    expect([...t.keys()]).not.toContain("cursor");
  });

  it("`sort`/`order` lạ bị bỏ — máy chủ trả 400 cho chúng (`core/page/page.go:336-355`)", () => {
    // `due_at` IS a server column since 28/09/2026 (#13) — only the bad `order` is dropped.
    expect(locTuDuongDan("?sort=due_at&order=up")).toEqual({ sapXep: "due_at" });
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3b): `title` and `priority` were dropped here as unknown columns.
    // The server sorts by both now (backend P9); a genuinely unknown column is still dropped.
    expect(locTuDuongDan("?sort=title&order=up")).toEqual({ sapXep: "title" });
    expect(locTuDuongDan("?sort=title")).toEqual({ sapXep: "title" });
    expect(locTuDuongDan("?sort=priority")).toEqual({ sapXep: "priority" });
    expect(locTuDuongDan("?sort=tieu_de")).toEqual({});
    expect(locTuDuongDan("?order=desc")).toEqual({ chieu: "desc" });
  });
});

describe("§4.2 — sắp xếp bảng Danh sách", () => {
  it("mặc định là mặc định của máy chủ: `created_at` giảm dần; thiếu vế nào lấy vế ấy", () => {
    expect(sapXepDayDu({})).toEqual({ cot: "created_at", chieu: "desc" });
    // `page.NewAllowlist(page.Desc, …)`: chiều mặc định là GIẢM cho mọi cột, kể cả `code`.
    expect(sapXepDayDu({ sapXep: "code" })).toEqual({ cot: "code", chieu: "desc" });
    expect(sapXepDayDu({ chieu: "asc" })).toEqual({ cot: "created_at", chieu: "asc" });
  });

  it("bấm cùng cột thì đảo chiều; bấm cột khác thì theo chiều tự nhiên của cột ấy", () => {
    const md = SAP_XEP_MAC_DINH;
    expect(bamCotSapXep(md, "created_at")).toEqual({ cot: "created_at", chieu: "asc" });
    // Mã: NV01, NV02… — thứ tự của chính quyển sổ.
    expect(bamCotSapXep(md, "code")).toEqual({ cot: "code", chieu: "asc" });
    expect(bamCotSapXep({ cot: "code", chieu: "asc" }, "code")).toEqual({ cot: "code", chieu: "desc" });
    expect(bamCotSapXep({ cot: "code", chieu: "asc" }, "created_at")).toEqual({
      cot: "created_at",
      chieu: "desc",
    });
  });

  it("(#13) `Hạn` sắp được: bấm lần đầu là hạn SỚM NHẤT trước, bấm lại thì đảo", () => {
    expect(bamCotSapXep(SAP_XEP_MAC_DINH, "due_at")).toEqual({ cot: "due_at", chieu: "asc" });
    expect(bamCotSapXep({ cot: "due_at", chieu: "asc" }, "due_at")).toEqual({
      cot: "due_at",
      chieu: "desc",
    });
    expect(ariaSapXep({ cot: "due_at", chieu: "desc" }, "due_at")).toBe("descending");
    // Round-trips through the address bar like the other two columns.
    const t = new URLSearchParams(duongDanTuLoc({ sapXep: "due_at", chieu: "asc" }));
    expect(t.get("sort")).toBe("due_at");
    expect(locTuDuongDan(`?${t.toString()}`)).toEqual({ sapXep: "due_at", chieu: "asc" });
  });

  it("`Tên việc` và `Ưu tiên` sắp được: bấm lần đầu TĂNG (A→Z; đúng thứ tự danh mục của xã), bấm lại thì đảo", () => {
    expect(bamCotSapXep(SAP_XEP_MAC_DINH, "title")).toEqual({ cot: "title", chieu: "asc" });
    expect(bamCotSapXep(SAP_XEP_MAC_DINH, "priority")).toEqual({ cot: "priority", chieu: "asc" });
    expect(bamCotSapXep({ cot: "priority", chieu: "asc" }, "priority")).toEqual({
      cot: "priority",
      chieu: "desc",
    });
    for (const cot of ["title", "priority"] as const) {
      const t = new URLSearchParams(duongDanTuLoc({ sapXep: cot, chieu: "desc" }));
      expect(t.get("sort")).toBe(cot);
      expect(locTuDuongDan(`?${t.toString()}`)).toEqual({ sapXep: cot, chieu: "desc" });
    }
    expect(NO_PRIORITY_LAST_NOTE).toContain("thứ tự xã xếp danh mục mức ưu tiên");
    expect(NO_PRIORITY_LAST_NOTE).toContain("luôn nằm cuối");
  });

  it("`aria-sort` chỉ ở cột đang sắp, đúng chiều", () => {
    expect(ariaSapXep({ cot: "code", chieu: "asc" }, "code")).toBe("ascending");
    expect(ariaSapXep({ cot: "code", chieu: "desc" }, "code")).toBe("descending");
    expect(ariaSapXep({ cot: "code", chieu: "asc" }, "created_at")).toBe("none");
  });
});

describe("ô ghi tay §5.9 — ai thấy, và thân gửi đi", () => {
  const TASK = {
    assignee: "CB-2026-THUCHIEN",
    monitor: "CB-2026-THEODOI",
    assigner: "CB-2026-LANHDAO",
    created_by: "CB-2026-NGUOITAO",
  };
  const KHONG = quyenNhiemVu([]);

  it("ĐƯỢC — `task.update`, hoặc người thực hiện / theo dõi / giao việc / tạo (máy chủ `TaskWorkRightFor`)", () => {
    expect(canWriteLogEntry(quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]), TASK, "CB-2026-NGUOIKHAC")).toBe(true);
    for (const ma of Object.values(TASK)) expect(canWriteLogEntry(KHONG, TASK, ma)).toBe(true);
  });

  it("BỊ TỪ CHỐI — người ngoài, kể cả cầm `task.approve`; phiên rỗng trên việc thiếu người theo dõi", () => {
    expect(canWriteLogEntry(KHONG, TASK, "CB-2026-NGUOIKHAC")).toBe(false);
    expect(canWriteLogEntry(quyenNhiemVu([QUYEN_DUYET_HOAN_THANH_NHIEM_VU]), TASK, "CB-2026-NGUOIKHAC")).toBe(
      false,
    );
    // `"" === ""` would open the form on every task with an empty field to every unread session.
    expect(canWriteLogEntry(KHONG, { ...TASK, monitor: "" }, "")).toBe(false);
  });

  it("nội dung: cắt khoảng trắng như máy chủ; rỗng ⇒ `null` (nút khoá)", () => {
    expect(logEntryNote("")).toBeNull();
    expect(logEntryNote("  \n\t ")).toBeNull();
    expect(logEntryNote("  Đã gửi công văn \n")).toBe("Đã gửi công văn");
  });
});

describe("✎ Sửa — Mã nhiệm vụ và Hạn xử lý (W3: 3c3525f, f27fd6e, d2ed15e)", () => {
  const TOKEN = "2026-06-01T02:00:00Z";

  it("form mở với mã và hạn của nhiệm vụ — ngày VÀ giờ theo múi giờ Việt Nam", () => {
    const f = formGoc();
    expect(f.code).toBe("NV19");
    expect(f.dueDate).toBe("2026-06-20");
    expect(f.dueTime).toBe("23:59");
    // 00:30 ở +07 là 17:30 UTC hôm trước: đọc theo UTC sẽ ra sai cả ngày lẫn giờ.
    expect(timeForInput("2026-06-19T17:30:00Z")).toBe("00:30");
    expect(timeForInput(null)).toBe("");
    expect(timeForInput("không phải ngày")).toBe("");
  });

  it("đổi mã ⇒ `code` đã cắt khoảng trắng; gõ lại đúng mã cũ ⇒ không có gì để lưu", () => {
    expect(thanSuaNhiemVu({ ...formGoc(), code: "  NV19A " }, chiTiet(), VB_GOC)).toEqual({
      code: "NV19A",
      expected_updated_at: TOKEN,
    });
    expect(thanSuaNhiemVu({ ...formGoc(), code: " NV19 " }, chiTiet(), VB_GOC)).toBeNull();
  });

  it("đổi hạn ⇒ `due_at` là MỘT MỐC `+07:00` ghép từ ngày và giờ", () => {
    expect(dueAtFromInputs("2026-07-01", "16:30")).toBe("2026-07-01T16:30:00+07:00");
    expect(thanSuaNhiemVu({ ...formGoc(), dueDate: "2026-07-01", dueTime: "16:30" }, chiTiet(), VB_GOC)).toEqual({
      due_at: "2026-07-01T16:30:00+07:00",
      expected_updated_at: TOKEN,
    });
    // Chỉ đổi giờ cũng là một lần sửa hạn.
    expect(thanSuaNhiemVu({ ...formGoc(), dueTime: "17:00" }, chiTiet(), VB_GOC)?.due_at).toBe(
      "2026-06-20T17:00:00+07:00",
    );
  });

  it("KHÔNG đụng tới hạn ⇒ không gửi `23:59:00` thay cho `23:59:59` đã lưu", () => {
    // The stored second is invisible in an HH:MM field; re-composing it would be a false correction.
    const than = thanSuaNhiemVu({ ...formGoc(), ghiChu: "x" }, chiTiet(), VB_GOC);
    expect(than).not.toHaveProperty("due_at");
  });

  it("việc CHƯA CÓ HẠN: đặt hạn lần đầu được; để trống thì không gửi gì", () => {
    const khongHan = chiTiet({ due_at: null, original_due_at: null });
    const f = formSuaTuChiTiet(khongHan, VB_GOC);
    expect([f.dueDate, f.dueTime]).toEqual(["", ""]);
    expect(thanSuaNhiemVu(f, khongHan, VB_GOC)).toBeNull();
    expect(canhBaoSua(f, khongHan)).toBeNull();
    expect(thanSuaNhiemVu({ ...f, dueDate: "2026-07-01", dueTime: "17:00" }, khongHan, VB_GOC)?.due_at).toBe(
      "2026-07-01T17:00:00+07:00",
    );
  });

  it("chặn Lưu: mã trống; ngày mà thiếu giờ; giờ mà thiếu ngày; xoá hạn đang có", () => {
    expect(canhBaoSua({ ...formGoc(), code: "  " }, chiTiet())).toContain("Mã nhiệm vụ");
    expect(canhBaoSua({ ...formGoc(), dueTime: "" }, chiTiet())).toBe("Nhập giờ của hạn xử lý.");
    expect(canhBaoSua({ ...formGoc(), dueDate: "" }, chiTiet())).toBe("Chọn ngày của hạn xử lý.");
    // `due_at: null` means "leave it" on the server: an emptied deadline would look saved and not be.
    expect(canhBaoSua({ ...formGoc(), dueDate: "", dueTime: "" }, chiTiet())).toContain("không xoá được");
    expect(canhBaoSua(formGoc(), chiTiet())).toBeNull();
  });

  it("câu dưới hai ô nói rõ: SỬA hạn không phải lùi hạn, và khi nào hạn ban đầu đổi theo", () => {
    expect(DUE_EDIT_NOTE).toContain("không phải lùi hạn");
    expect(DUE_EDIT_NOTE).toContain("chưa từng được duyệt lùi hạn thì hạn ban đầu đổi theo");
    expect(DUE_EDIT_NOTE).toContain("chỉ hạn xử lý đổi");
    expect(CODE_EDIT_NOTE).toContain("không bao giờ cấp cho việc khác");
  });
});

describe("giờ mặc định của hạn — giờ kết thúc ca cuối theo lịch làm việc CỦA XÃ", () => {
  // ISO weekday: 1 = Monday … 7 = Sunday. 2026-07-01 is a Wednesday (3), 2026-07-05 a Sunday (7).
  const LICH = [
    { weekday: 3, end: "11:30:00" },
    { weekday: 3, end: "17:00:00" },
    { weekday: 6, end: "11:30" },
  ];

  it("ca cuối của đúng thứ ấy — không phải một giờ gõ cứng", () => {
    expect(defaultDueTime(LICH, "2026-07-01")).toBe("17:00");
    expect(defaultDueTime(LICH, "2026-07-04")).toBe("11:30");
    // Another commune's calendar gives another hour from the same code.
    expect(defaultDueTime([{ weekday: 3, end: "16:30" }], "2026-07-01")).toBe("16:30");
  });

  it("Chủ nhật là 7 theo ISO, KHÔNG phải 0 — chỉ ngày này tách được hai cách đánh số", () => {
    // Measured: mapping JS `getUTCDay()` straight through (0 = Sunday) left every other case green,
    // because Monday–Saturday coincide under both numberings.
    expect(defaultDueTime([{ weekday: 7, end: "11:00" }], "2026-07-05")).toBe("11:00");
    expect(defaultDueTime([{ weekday: 0, end: "11:00" }], "2026-07-05")).toBe("");
  });

  it("thứ không có ca, ngày hỏng, lịch rỗng ⇒ `\"\"` (ô giờ trống và bắt buộc)", () => {
    expect(defaultDueTime(LICH, "2026-07-05")).toBe("");
    expect(defaultDueTime(LICH, "05/07/2026")).toBe("");
    expect(defaultDueTime([], "2026-07-01")).toBe("");
  });

  it("câu dưới ô giờ: đang đọc lịch / không đọc được / không có ca / đã điền sẵn — suy ra, không lưu", () => {
    const goc = chiTiet();
    const moi = { dueDate: "2026-07-01", dueTime: "" };
    expect(dueTimeHint({ pha: "dangTai" }, moi, goc)).toBe(DUE_TIME_LOADING);
    expect(dueTimeHint({ pha: "loi" }, moi, goc)).toBe(DUE_TIME_NO_CALENDAR);
    expect(dueTimeHint({ pha: "xong", shifts: LICH }, { dueDate: "2026-07-05", dueTime: "" }, goc)).toBe(
      DUE_TIME_NO_SHIFT,
    );
    const filled = { dueDate: "2026-07-01", dueTime: "17:00" };
    expect(dueTimeHint({ pha: "xong", shifts: LICH }, filled, goc)).toBe(dueTimeFilledNote("17:00"));
    // The clerk typed another hour: the "pre-filled" sentence goes.
    expect(dueTimeHint({ pha: "xong", shifts: LICH }, { ...filled, dueTime: "15:00" }, goc)).toBeNull();
    // Deadline untouched: nothing to say.
    expect(dueTimeHint({ pha: "xong", shifts: LICH }, { dueDate: "2026-06-20", dueTime: "23:59" }, goc)).toBeNull();
  });
});

describe("409 `task_changed` — phát hiện bằng DỮ LIỆU đọc lại, không bằng mã lỗi", () => {
  it("đọc lại có `updated_at` khác bản chụp ⇒ đã đổi; bằng nhau hoặc đọc hỏng ⇒ không nói vậy", () => {
    expect(changedSince({ ok: true, duLieu: { updated_at: "B" } }, "A")).toBe(true);
    expect(changedSince({ ok: true, duLieu: { updated_at: "A" } }, "A")).toBe(false);
    expect(changedSince({ ok: false, thongBao: "x" }, "A")).toBe(false);
  });
});

describe("W5 — `Liên quan đến tôi` và `Sắp đến hạn` đi rồi về qua đường dẫn", () => {
  it("`scope=related` và `soon=true` sống sót; chỉ đúng chữ `true`", () => {
    const loc = { phamVi: "related" as const, dueSoon: true as const };
    expect(locTuDuongDan(duongDanTuLoc(loc))).toEqual(loc);
    expect(locTuDuongDan("?scope=mine&soon=true")).toEqual({ phamVi: "mine", dueSoon: true });
    expect(locTuDuongDan("?soon=TRUE")).toEqual({});
    expect(duongDanTuLoc({ dueSoon: false })).toBe("");
  });
});
