import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import { KhoiHangChoLuiHan, type TaiHangCho } from "./hang-cho-lui-han";
import {
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_AI_DUYET_DUOC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  CAU_THIEU_QUYEN_DUYET_GIA_HAN,
  DANG_TAI_HANG_CHO,
  ID_HANG_CHO,
  LOC_HANG_CHO_MAC_DINH,
  NHAN_LOC_CHO_TOI,
  NHAN_XEM_THEM_HANG_CHO,
  O_TRONG,
  boDeNghiDaQuyet,
  cauHangChoRong,
  gopTrangNhatKy,
  hienDongHangCho,
  thamSoHangCho,
  type LocHangCho,
} from "./nhan-nhiem-vu";
import { chuyenDrawer, type DrawerNhiemVu } from "./so-nhiem-vu";

/**
 * Hàng chờ duyệt lùi hạn §5.8 (TASK-07).
 *
 * CA ĐÁNG LO NHẤT LÀ DÒNG KHÔNG GHI LÃNH ĐẠO: `task_assigner` rỗng và một phiên chưa đọc được (mã
 * rỗng) KHÔNG được khớp nhau — `"" === ""` là đúng, và bỏ phép kiểm ấy là vẽ hai nút cho mọi người
 * trên mọi dòng không ai duyệt được. Máy chủ vẫn chặn; nhưng một màn hình mời bấm vào thứ chắc chắn
 * bị từ chối là một màn hình nói sai về ADR 0038.
 */

/** Chuỗi như nó THẬT SỰ nằm trong HTML — xem `so-nhiem-vu.test.tsx`. */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

const LANH_DAO = "CB-2026-7K3M9Q";
const NGUOI_KHAC = "CB-2026-0P4X1Z";
const NGUOI_DE_NGHI = "CB-2026-3H8N2W";

const DANH_BA: DanhBaTheoMa = new Map([
  [LANH_DAO, { code: LANH_DAO, full_name: "Trần Văn Lãnh", position: "Chủ tịch", department_id: "" }],
  [
    NGUOI_DE_NGHI,
    { code: NGUOI_DE_NGHI, full_name: "Nguyễn Thị Thực", position: "", department_id: "" },
  ],
]);

function deNghi(sua: Partial<petitions_deNghiChoDuyetRa> = {}): petitions_deNghiChoDuyetRa {
  return {
    id: "01JDENGHI0001",
    task_code: "NV19",
    task_title: "Báo cáo tổng kết công tác cán bộ",
    task_due_at: "2026-06-20T23:59:59+07:00",
    task_assigner: LANH_DAO,
    new_due_at: "2026-07-15T23:59:59+07:00",
    reason: "Chờ số liệu của hai thôn",
    requested_by: NGUOI_DE_NGHI,
    requested_at: "2026-06-18T02:20:00Z",
    ...sua,
  };
}

function ve(
  tai: TaiHangCho,
  tuyChon: {
    loc?: LocHangCho;
    maNguoiDangNhap?: string;
    /** `task.extend` của phiên. Mặc định CÓ — ca thiếu khoá truyền `false` tường minh. */
    coQuyenDuyetGiaHan?: boolean;
    loiDong?: { id: string; thongBao: string } | null;
    loiThem?: string | null;
    dangQuyet?: string | null;
  } = {},
): string {
  return renderToStaticMarkup(
    <KhoiHangChoLuiHan
      loc={tuyChon.loc ?? LOC_HANG_CHO_MAC_DINH}
      datLoc={() => {}}
      tai={tai}
      danhBa={DANH_BA}
      maNguoiDangNhap={tuyChon.maNguoiDangNhap ?? LANH_DAO}
      coQuyenDuyetGiaHan={tuyChon.coQuyenDuyetGiaHan ?? true}
      dangQuyet={tuyChon.dangQuyet ?? null}
      loiDong={tuyChon.loiDong ?? null}
      loiMo={null}
      loiThem={tuyChon.loiThem ?? null}
      dangTaiThem={false}
      ghiChu={{}}
      datGhiChu={() => {}}
      quyetDinh={() => {}}
      moNhiemVu={() => {}}
      xemThem={() => {}}
    />,
  );
}

/** Chỉ dấu không thể nhầm của hai nút: nhãn truy cập mang mã nhiệm vụ. */
const NUT_DUYET_NV19 = 'aria-label="Duyệt lùi hạn NV19"';
const NUT_TU_CHOI_NV19 = 'aria-label="Từ chối lùi hạn NV19"';

describe("bộ lọc — mặc định `Chờ tôi duyệt`, và tham số đi lên đúng hình", () => {
  it("mặc định là `cua-toi` (quyết định 27/09/2026)", () => {
    expect(LOC_HANG_CHO_MAC_DINH).toBe("cua-toi");
  });

  it("`cua-toi` ⇒ `approver: \"me\"`; `toan-xa` ⇒ VẮNG MẶT, không phải chuỗi rỗng", () => {
    expect(thamSoHangCho("cua-toi")).toEqual({ approver: "me" });
    expect(thamSoHangCho("toan-xa")).toEqual({});
    expect("approver" in thamSoHangCho("toan-xa")).toBe(false);
  });

  it("nút lọc đang chọn mang `aria-pressed`, và đọc được bằng chữ Sổ tay lãnh đạo", () => {
    const html = ve({ pha: "xong", dong: [], conNua: false });
    // ADR 0068: drawn as a segmented control — classes and a decorative icon sit between the
    // attribute and the word. Still ONE pressed button, and it is `Chờ tôi duyệt`.
    expect(html.match(/aria-pressed="true"/g)?.length).toBe(1);
    expect(html).toMatch(new RegExp(`aria-pressed="true">(?:<svg[^>]*aria-hidden="true"[^>]*>.*?</svg>)?${NHAN_LOC_CHO_TOI}</button>`));
    expect(html).toContain(`id="${ID_HANG_CHO}"`);
  });
});

describe("dòng hàng chờ → chữ", () => {
  it("hạn đang có vs hạn đề nghị theo giờ Việt Nam; người đề nghị và lãnh đạo có họ tên KÈM mã", () => {
    const h = hienDongHangCho(deNghi(), DANH_BA, LANH_DAO, true);
    expect(h.hanHienTai).toBe("20/6/2026");
    expect(h.hanDeNghi).toBe("15/7/2026");
    expect(h.nguoiDeNghi).toBe(`Nguyễn Thị Thực (${NGUOI_DE_NGHI})`);
    expect(h.lanhDao).toBe(`Trần Văn Lãnh (${LANH_DAO})`);
    expect(h.cauChan).toBeNull();
  });

  it("danh bạ chưa có: hiện MÃ, không bịa tên; nhiệm vụ không có hạn: dấu gạch", () => {
    const h = hienDongHangCho(deNghi({ task_due_at: null }), null, LANH_DAO, true);
    expect(h.nguoiDeNghi).toBe(NGUOI_DE_NGHI);
    expect(h.hanHienTai).toBe(O_TRONG);
  });

  it("không ghi lãnh đạo: câu `không ai duyệt được`, KHÔNG phải câu bảo bổ sung", () => {
    const h = hienDongHangCho(deNghi({ task_assigner: "" }), DANH_BA, LANH_DAO, true);
    expect(h.cauChan).toBe(CAU_KHONG_AI_DUYET_DUOC);
    expect(h.cauChan).not.toBe(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC);
    expect(h.lanhDao).toBe(O_TRONG);
  });

  it("không ghi lãnh đạo VÀ phiên chưa đọc được: hai chuỗi rỗng KHÔNG khớp nhau", () => {
    expect(hienDongHangCho(deNghi({ task_assigner: "" }), DANH_BA, "", true).cauChan).toBe(
      CAU_KHONG_AI_DUYET_DUOC,
    );
  });

  it("lãnh đạo khác (bộ lọc Toàn xã): câu lớp hai ADR 0038, không có nút", () => {
    expect(hienDongHangCho(deNghi(), DANH_BA, NGUOI_KHAC, true).cauChan).toBe(
      CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
    );
  });

  it("ĐÚNG lãnh đạo nhưng THIẾU `task.extend`: không nút, câu nói thiếu quyền — không phải câu lớp hai", () => {
    // Lớp một đóng thì tuyến trả 403 trước khi chạm tới quy tắc ADR 0038, nên câu đúng là câu về
    // QUYỀN. Câu "chỉ lãnh đạo giao việc…" ở đây sẽ nói sai với chính người được ghi trên bản ghi.
    const h = hienDongHangCho(deNghi(), DANH_BA, LANH_DAO, false);
    expect(h.cauChan).toBe(CAU_THIEU_QUYEN_DUYET_GIA_HAN);
  });
});

describe("gộp trang và bỏ dòng đã quyết", () => {
  it("`Xem thêm` nối theo thứ tự máy chủ, BỎ dòng trùng `id`, giữ bản đã có", () => {
    const a = deNghi({ id: "a" });
    const b = deNghi({ id: "b", reason: "cũ" });
    const bLai = deNghi({ id: "b", reason: "trôi sang trang sau" });
    const c = deNghi({ id: "c" });
    const ra = gopTrangNhatKy([a, b], [bLai, c]);
    expect(ra.map((d) => d.id)).toEqual(["a", "b", "c"]);
    expect(ra[1]?.reason).toBe("cũ");
  });

  it("quyết định xong: đúng một dòng rời, thứ tự còn lại giữ nguyên", () => {
    const ds = [deNghi({ id: "a" }), deNghi({ id: "b" }), deNghi({ id: "c" })];
    expect(boDeNghiDaQuyet(ds, "b").map((d) => d.id)).toEqual(["a", "c"]);
    expect(boDeNghiDaQuyet(ds, "khong-co").map((d) => d.id)).toEqual(["a", "b", "c"]);
  });
});

describe("mục hàng chờ ra tới trang", () => {
  it("dòng của đúng lãnh đạo: mã + tiêu đề mở drawer, hai hạn, lý do, người đề nghị, HAI nút có nhãn", () => {
    const html = ve({ pha: "xong", dong: [deNghi()], conNua: false });
    expect(html).toContain('aria-label="Mở NV19: Báo cáo tổng kết công tác cán bộ"');
    expect(html).toContain("20/6/2026");
    expect(html).toContain("15/7/2026");
    expect(html).toContain("Chờ số liệu của hai thôn");
    expect(html).toContain(`Nguyễn Thị Thực (${NGUOI_DE_NGHI})`);
    expect(html).toContain('dateTime="2026-06-18T02:20:00Z"');
    expect(html).toContain(NUT_DUYET_NV19);
    expect(html).toContain(NUT_TU_CHOI_NV19);
    expect(html).toContain('for="ghi-chu-quyet-dinh-01JDENGHI0001"');
  });

  it("dòng KHÔNG ghi lãnh đạo: câu nói không ai duyệt được, KHÔNG có nút nào", () => {
    const html = ve({ pha: "xong", dong: [deNghi({ task_assigner: "" })], conNua: false });
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_AI_DUYET_DUOC));
    expect(html).not.toContain(NUT_DUYET_NV19);
    expect(html).not.toContain(NUT_TU_CHOI_NV19);
  });

  it("CA BỊ TỪ CHỐI: người khác xem Toàn xã — không nút, câu lớp hai ADR 0038", () => {
    const html = ve(
      { pha: "xong", dong: [deNghi()], conNua: false },
      { loc: "toan-xa", maNguoiDangNhap: NGUOI_KHAC },
    );
    expect(html).not.toContain(NUT_DUYET_NV19);
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("CA BỊ TỪ CHỐI: đúng lãnh đạo, phiên THIẾU `task.extend` — không nút, câu thiếu quyền", () => {
    const html = ve(
      { pha: "xong", dong: [deNghi()], conNua: false },
      { coQuyenDuyetGiaHan: false },
    );
    expect(html).not.toContain(NUT_DUYET_NV19);
    expect(html).not.toContain(NUT_TU_CHOI_NV19);
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_GIA_HAN));
  });

  it("máy chủ từ chối quyết định: câu NGUYÊN VĂN dưới đúng dòng ấy, `role=\"alert\"`", () => {
    const cau = "Chỉ lãnh đạo giao việc ghi trên nhiệm vụ mới được duyệt — tài khoản thiếu task.extend";
    const html = ve(
      { pha: "xong", dong: [deNghi({ id: "x" }), deNghi({ id: "y", task_code: "NV20" })], conNua: false },
      { loiDong: { id: "y", thongBao: cau } },
    );
    expect(html).toContain(`role="alert">${nhuTrongHTML(cau)}</p>`);
    // Đúng MỘT lần — không lan sang dòng kia.
    expect(html.split(nhuTrongHTML(cau)).length - 1).toBe(1);
    // Câu nằm SAU nút của NV20, tức trong dòng NV20.
    expect(html.indexOf(nhuTrongHTML(cau))).toBeGreaterThan(html.indexOf("Duyệt lùi hạn NV20"));
  });

  it("đang gửi một quyết định: mọi nút quyết định khoá — không gửi hai lần song song", () => {
    const html = ve({ pha: "xong", dong: [deNghi()], conNua: false }, { dangQuyet: "01JDENGHI0001" });
    expect(html).toContain("Đang gửi…");
    expect(html).toMatch(/disabled=""[^>]*aria-label="Duyệt lùi hạn NV19"/);
  });

  it("rỗng: HAI câu khác nhau cho hai bộ lọc", () => {
    expect(ve({ pha: "xong", dong: [], conNua: false })).toContain(cauHangChoRong("cua-toi"));
    expect(ve({ pha: "xong", dong: [], conNua: false }, { loc: "toan-xa" })).toContain(
      cauHangChoRong("toan-xa"),
    );
    expect(cauHangChoRong("cua-toi")).not.toBe(cauHangChoRong("toan-xa"));
  });

  it("đang tải: `role=\"status\"`, không vẽ câu rỗng", () => {
    const html = ve({ pha: "dangTai" });
    expect(html).toContain(`role="status">${DANG_TAI_HANG_CHO}`);
    expect(html).not.toContain(cauHangChoRong("cua-toi"));
  });

  it("tải hỏng: câu máy chủ nguyên văn, `role=\"alert\"`, KHÔNG vẽ câu rỗng", () => {
    const html = ve({ pha: "loi", thongBao: "approver chỉ nhận me" });
    expect(html).toContain('role="alert">approver chỉ nhận me</p>');
    expect(html).not.toContain(cauHangChoRong("cua-toi"));
  });

  it("`Xem thêm` chỉ khi còn trang; lỗi tải thêm hiện nguyên văn", () => {
    expect(ve({ pha: "xong", dong: [deNghi()], conNua: true })).toContain(NHAN_XEM_THEM_HANG_CHO);
    expect(ve({ pha: "xong", dong: [deNghi()], conNua: false })).not.toContain(
      NHAN_XEM_THEM_HANG_CHO,
    );
    expect(
      ve({ pha: "xong", dong: [deNghi()], conNua: true }, { loiThem: "con trỏ không hợp lệ" }),
    ).toContain('role="alert">con trỏ không hợp lệ</p>');
  });
});

describe("chuyenDrawer — `docLai` sau một quyết định ở hàng chờ", () => {
  const nv = { code: "NV19" } as petitions_nhiemVuRa;
  const s: DrawerNhiemVu = { nhiemVu: nv, vanBan: { pha: "xong", duLieu: [] }, luotDoc: 3 };

  it("đúng mã đang mở: tăng lượt đọc, GIỮ nguyên phần đang hiện", () => {
    const ra = chuyenDrawer(s, { loai: "docLai", ma: "NV19" });
    expect(ra?.luotDoc).toBe(4);
    expect(ra?.vanBan).toBe(s.vanBan);
  });

  it("mã khác hoặc drawer đóng: không làm gì", () => {
    expect(chuyenDrawer(s, { loai: "docLai", ma: "NV20" })).toBe(s);
    expect(chuyenDrawer(null, { loai: "docLai", ma: "NV19" })).toBeNull();
  });
});
