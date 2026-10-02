import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
} from "@/lib/quyen";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_deNghiChoDuyetRa,
  petitions_deNghiLuiHanRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
} from "@/lib/api/schema.gen";

import {
  BANG_NHAN_MAC_DINH,
  CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
  CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
  CANH_BAO_HAN_MOT_LAN,
  NEW_TASK_DUE_PREFILLED_NOTE,
  CAU_THIEU_QUYEN_DUYET_GIA_HAN,
  CAU_THIEU_QUYEN_DUYET_HOAN_THANH,
  DECISION_NOTE_LABEL,
  TASK_EXTENSIONS_EMPTY,
  TASK_EXTENSIONS_LOADING,
  TASK_EXTENSIONS_TITLE,
  extensionBlockNote,
  hienDongHangCho,
  CHUA_PHAN_CONG,
  CHI_TIET_THIEU_VAN_BAN,
  CHU_THICH_HAI_O_TICK,
  DANG_TAI_VAN_BAN,
  GHI_CHU_LUI_HAN,
  ID_HANG_CHO,
  KHOA_SUA_DANG_TAI,
  KHOA_SUA_LOI,
  KHOA_SUA_NHOM_LA,
  KHONG_DOC_DUOC_VAN_BAN,
  KHONG_SO,
  DUE_EDIT_NOTE,
  NHAN_NUT_SUA,
  NHAT_KY_RONG,
  O_TRONG,
  PHAN_CHUA_DUNG,
  SO_RONG,
  TIEU_DE_KHOI_VAN_BAN,
  CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN,
  NHAN_NUT_TRA_LAI,
  REOPEN_BUTTON,
  REOPEN_REASON_LABEL,
  reopenNote,
  SAP_XEP_MAC_DINH,
  cauLoiDanhBaLanhDao,
  ghiChuTraLai,
  mocCuoiNgay,
  ngayChoONhap,
  quyetDinhDuyetLuiHan,
  quyenNhiemVu,
  type QuyenNhiemVu,
  type SapXepSo,
} from "./nhan-nhiem-vu";
import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import { duongDanHangChoLuiHan, duongDanSoNhiemVu } from "@/lib/api/nhiem-vu";
import { TaskExtensionList, taskExtensionsQuery } from "./task-extension-block";
import {
  BangNhiemVu,
  ChiTietNhiemVu,
  FormGiaoViec,
  FormSuaKhoiVanBan,
  KhoiChuaDung,
  KhoiLuiHan,
  KhoiTraLai,
  bamSapXep,
  chuyenDrawer,
  type DanhMucNhiemVu,
  type DrawerNhiemVu,
  type TrangThaiTai,
} from "./so-nhiem-vu";
import { KhoiNhatKyNhiemVu, type TaiNhatKyNhiemVu } from "./nhat-ky-nhiem-vu";
import { serverTransitions } from "./task-transitions.fixture";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM ADR 0038, và nó là nhóm không ai nhìn thấy trong lúc
 * phát triển: người viết mã luôn tự đặt mình làm lãnh đạo giao việc trong dữ liệu thử, nên nút
 * duyệt luôn hiện. Điều phải đúng là chuyện ngược lại — một lãnh đạo KHÁC, hoặc một nhiệm vụ chưa
 * ghi lãnh đạo nào, KHÔNG được thấy nút ấy, và phải đọc được VÌ SAO.
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ `renderToStaticMarkup` thoát `"` thành `&quot;` và `&` thành `&amp;`, nên một phép
 * `not.toContain` với chuỗi thô sẽ XANH kể cả khi chữ ấy đang nằm chình ình trên trang — tức là
 * canh đúng con số không. Đã đo ở `features/thu-chi/bang-thu-chi.test.tsx`.
 */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

const LANH_DAO = "CB-2026-7K3M9Q";
const NGUOI_KHAC = "CB-2026-0P4X1Z";

/**
 * A row as the server returns it. `allowed_transitions` follows `status` (the server's list,
 * `task-transitions.fixture.ts`) unless the case sets it — the drawer draws only what the row carries.
 */
function nhiemVu(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    allowed_transitions: serverTransitions(sua.status ?? "dang-thuc-hien"),
    updated_at: "2026-06-01T02:00:00Z",
    type: "theo-van-ban",
    bloc: "khoi-dang",
    priority: "cao",
    title: "Báo cáo tổng kết việc thực hiện chủ trương của Bộ Chính trị về công tác cán bộ",
    description: "",
    status: "dang-thuc-hien",
    source: "ket-luan-hop",
    source_id: "01JKETLUAN",
    unit: "01JBOPHAN",
    assignee: "CB-2026-3H8N2W",
    assigner: LANH_DAO,
    due_at: "2026-06-20T23:59:59+07:00",
    original_due_at: "2026-06-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    created_by: "CB-2026-VANTHU",
    created_at: "2026-06-01T02:00:00Z",
    ...sua,
  };
}

const BO_PHAN: identity_boPhanRa[] = [
  { id: "01JBOPHAN", code: "vp-dang-uy", name: "VĂN PHÒNG ĐẢNG ỦY", parent_id: "", order: 0, staff_count: 0 },
];

const DANH_MUC: DanhMucNhiemVu = {
  loai: [
    {
      id: "01JLOAI",
      code: "theo-van-ban",
      label: "Theo văn bản",
      is_default: true,
      active: true,
      order: 1,
      source: "he-thong",
      tier: 1,
    },
  ],
  mucUuTien: [
    {
      id: "01JUUTIEN",
      code: "cao",
      label: "Cao",
      is_default: false,
      active: true,
      order: 2,
      source: "he-thong",
      tier: 1,
    },
  ],
  khoi: [
    {
      id: "01JKHOI",
      code: "khoi-dang",
      label: "Khối Đảng",
      is_default: false,
      active: true,
      order: 1,
      source: "don-vi",
      tier: 1,
    },
  ],
  boPhan: BO_PHAN,
};

/** Xã có loại mặc định là `co-ban` — để vẽ được nhánh §7.3 mà không cần sự kiện DOM. */
const DANH_MUC_CO_BAN: DanhMucNhiemVu = {
  ...DANH_MUC,
  loai: [
    {
      id: "01JLOAICOBAN",
      code: "co-ban",
      label: "Nhiệm vụ cơ bản",
      is_default: true,
      active: true,
      order: 2,
      source: "he-thong",
      tier: 1,
    },
  ],
};

/** Danh bạ chọn người giả — mã cán bộ giả, không số điện thoại, không email (đúng hợp đồng). */
const DANH_BA: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: LANH_DAO, full_name: "Trần Văn Lãnh", position: "Chủ tịch", department_id: "" },
      {
        code: NGUOI_KHAC,
        full_name: "Nguyễn Thị Thực",
        position: "",
        department_id: "01JBOPHAN",
      },
    ],
  },
};

const TEN_BO_PHAN = new Map(BO_PHAN.map((b) => [b.id, b.name]));

/** 23/09/2026 — ba tháng sau hạn 20/6, đúng bối cảnh `Trễ 87 ngày` của đặc tả. */
const BAY_GIO = new Date("2026-09-15T03:00:00Z");

const KHONG_GOI = (): Promise<KetQua<petitions_deNghiLuiHanRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

const KHONG_SUA = (): Promise<KetQua<petitions_nhiemVuRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

type TaiVanBan = TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;

/** Pass-2 drawer props (#6 #10 #11) that no test in this group exercises — inert. */
const PASS2_DRAWER_PROPS = {
  extensionRefreshKey: "0",
  onExtensionDecided: () => {},
  openTask: () => {},
  openTaskByCode: KHONG_SUA,
  saveParent: KHONG_SUA,
  addChild: null,
  reassign: KHONG_SUA,
} as const;

/**
 * Đủ năm khoá ghi `task.*`. Các nhóm canh HÌNH DẠNG màn (vòng đời, lùi hạn, văn bản) dùng nó; nhóm
 * `cổng nút theo khoá` ở cuối tệp canh chiều NGƯỢC LẠI — thiếu từng khoá một.
 */
const DU_QUYEN: QuyenNhiemVu = quyenNhiemVu([
  QUYEN_TAO_NHIEM_VU,
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
]);

function veChiTiet(
  sua: Partial<petitions_nhiemVuRa> = {},
  maNguoiDangNhap = LANH_DAO,
  vanBan: TaiVanBan = { pha: "dangTai" },
  quyen: QuyenNhiemVu = DU_QUYEN,
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null = null,
): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={nhiemVu(sua)}
      vanBan={vanBan}
      danhMuc={DANH_MUC}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={TEN_BO_PHAN}
      danhBa={danhBa}
      bayGio={BAY_GIO}
      maNguoiDangNhap={maNguoiDangNhap}
      quyen={quyen}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      doiTrangThai={() => {}}
      xoa={() => {}}
      guiDeNghiLuiHan={KHONG_GOI}
      quyetDinh={KHONG_GOI}
      suaKhoiVanBan={KHONG_SUA}
      docLaiChiTiet={KHONG_SUA}
      {...PASS2_DRAWER_PROPS}
    />,
  );
}

/** Chỉ dấu KHÔNG THỂ NHẦM của khối quyết định lùi hạn. */
const NUT_DUYET = "Duyệt lùi hạn";

describe("ADR 0038 — lớp hai chạy TRÊN MÀN, không chỉ trong hàm thuần", () => {
  it("không phải lãnh đạo giao việc: KHÔNG có nút duyệt, và câu từ chối nói rõ vì sao", () => {
    const html = veChiTiet({}, NGUOI_KHAC);

    // Canh bằng chính nhãn nút. Câu từ chối KHÔNG chứa chuỗi ấy, nên phép `not.toContain` này
    // không thể xanh vì lý do sai.
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("nhiệm vụ CHƯA GHI lãnh đạo giao việc: không ai duyệt được, kể cả người đang xem", () => {
    // Đây là chỗ một dòng thiếu gây hỏng rộng nhất: không có phép kiểm chuỗi rỗng thì `"" === ""`
    // là đúng, và MỌI tài khoản duyệt được MỌI đề nghị trên MỌI nhiệm vụ chưa ghi lãnh đạo.
    const html = veChiTiet({ assigner: "" }, "");

    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC));
  });

  it("phiên chưa đọc được (không có mã cán bộ): FAIL CLOSED, không mở nút", () => {
    const html = veChiTiet({}, "");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("một ULID KHÔNG khớp một mã cán bộ — hai loại định danh khác nhau", () => {
    // Cả hai đều là chuỗi khác rỗng trông rất hợp lý, nên phép so sai KHÔNG làm đỏ gì ngoài ca này.
    const html = veChiTiet({}, "01JBGQ3M4K5N6P7Q8R9S0T1U2V");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("ĐÚNG lãnh đạo giao việc: drawer có khối đề nghị CỦA CHÍNH nhiệm vụ này, không còn chỉ đường", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (TASK-03 lượt web 2, #11): tuyến nay nhận `task=NV19`, nên drawer
    // đọc đề nghị của chính việc này thay vì chỉ đường tới hàng chờ đầu sổ.
    const html = veChiTiet({}, LANH_DAO);
    expect(html).toContain(nhuTrongHTML(TASK_EXTENSIONS_TITLE));
    expect(html).toContain(nhuTrongHTML(TASK_EXTENSIONS_LOADING));
    expect(html).not.toContain(`href="#${ID_HANG_CHO}"`);
    // Still loading ⇒ no row ⇒ no button yet.
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    // Và KHÔNG hiện câu "không phải lãnh đạo" — người này ĐÚNG là lãnh đạo giao việc.
    expect(html).not.toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("(#11) khối đề nghị: cùng đường gọi với hàng chờ — `task=` của đúng một nhiệm vụ", () => {
    expect(duongDanHangChoLuiHan(taskExtensionsQuery("NV19"))).toBe(
      "/api/v1/task-extensions?task=NV19&limit=20",
    );
    // No `approver=me`: a request the viewer cannot decide is still shown, read-only.
    expect(duongDanHangChoLuiHan(taskExtensionsQuery("NV19"))).not.toContain("approver");
  });

  describe("(#11) đề nghị đang chờ trong drawer — nút Duyệt / Từ chối theo ĐÚNG cổng của hàng chờ", () => {
    const row: petitions_deNghiChoDuyetRa = {
      id: "01JDENGHI",
      task_code: "NV19",
      task_title: "Báo cáo tổng kết",
      task_due_at: "2026-06-20T23:59:59+07:00",
      task_assigner: LANH_DAO,
      new_due_at: "2026-07-20T23:59:59+07:00",
      reason: "Chờ số liệu của thôn",
      requested_by: NGUOI_KHAC,
      requested_at: "2026-06-18T02:00:00Z",
    };
    function list(
      session: string,
      canApprove: boolean,
      rows: readonly petitions_deNghiChoDuyetRa[] = [row],
      assigner = LANH_DAO,
    ): string {
      return renderToStaticMarkup(
        <TaskExtensionList
          load={{ phase: "done", rows: rows.map((r) => ({ ...r, task_assigner: assigner })) }}
          assigner={assigner}
          directory={null}
          sessionStaffCode={session}
          canApproveExtension={canApprove}
          deciding={null}
          rowError={null}
          notes={{}}
          setNote={() => {}}
          decide={() => {}}
        />,
      );
    }

    it("đúng lãnh đạo giao việc + `task.extend`: đề nghị hiện, HAI nút và ô ghi chú tuỳ chọn", () => {
      const html = list(LANH_DAO, true);
      expect(html).toContain("Chờ số liệu của thôn");
      expect(html).toContain('aria-label="Duyệt lùi hạn NV19"');
      expect(html).toContain('aria-label="Từ chối lùi hạn NV19"');
      expect(html).toContain(nhuTrongHTML(DECISION_NOTE_LABEL));
      expect(html).not.toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
    });

    it("người KHÁC: đề nghị vẫn hiện CHỈ ĐỌC, không nút, và câu nói vì sao", () => {
      const html = list(NGUOI_KHAC, true);
      expect(html).toContain("Chờ số liệu của thôn");
      expect(html).not.toContain(NUT_DUYET);
      expect(html).not.toContain("Từ chối lùi hạn");
      expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
    });

    it("đúng người nhưng THIẾU `task.extend`: chỉ đọc, kèm câu thiếu quyền — lớp một đóng", () => {
      const html = list(LANH_DAO, false);
      expect(html).not.toContain(NUT_DUYET);
      expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_GIA_HAN));
    });

    it("phiên chưa đọc được và nhiệm vụ không ghi lãnh đạo: FAIL CLOSED, `\"\" === \"\"` không mở nút", () => {
      expect(list("", true)).not.toContain(NUT_DUYET);
      const html = list("", true, [row], "");
      expect(html).not.toContain(NUT_DUYET);
      expect(html).toContain(nhuTrongHTML(CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC));
    });

    it("gate is the queue's own `hienDongHangCho` — same answer for every session", () => {
      for (const [session, canApprove] of [
        [LANH_DAO, true],
        [LANH_DAO, false],
        [NGUOI_KHAC, true],
        ["", true],
      ] as const) {
        const queueSaysButtons = hienDongHangCho(row, null, session, canApprove).cauChan === null;
        expect(list(session, canApprove).includes(NUT_DUYET)).toBe(queueSaysButtons);
      }
    });

    it("đọc HỎNG: câu máy chủ nguyên văn, KHÔNG nói `không có đề nghị nào`", () => {
      const cau = "không đủ quyền: thiếu task.read";
      const html = renderToStaticMarkup(
        <TaskExtensionList
          load={{ phase: "error", message: cau }}
          assigner={LANH_DAO}
          directory={null}
          sessionStaffCode={LANH_DAO}
          canApproveExtension
          deciding={null}
          rowError={null}
          notes={{}}
          setNote={() => {}}
          decide={() => {}}
        />,
      );
      expect(html).toContain(`role="alert">${nhuTrongHTML(cau)}</p>`);
      expect(html).not.toContain(nhuTrongHTML(TASK_EXTENSIONS_EMPTY));
    });

    it("`extensionBlockNote`: thiếu `task.extend` chỉ đáng nói khi CÓ đề nghị", () => {
      const thieu = quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, false);
      expect(extensionBlockNote(thieu, false)).toBeNull();
      expect(extensionBlockNote(thieu, true)).toBe(CAU_THIEU_QUYEN_DUYET_GIA_HAN);
      expect(extensionBlockNote(quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, true), true)).toBeNull();
    });
  });

  it("ô gửi đề nghị KHÔNG còn nút quyết định nào của riêng nó — một cổng, một chỗ", () => {
    const html = renderToStaticMarkup(
      <KhoiLuiHan
        coQuyenDeNghi
        hanHienTai="2026-06-20T23:59:59+07:00"
        dangGui={false}
        guiDeNghi={KHONG_GOI}
      />,
    );
    expect(html).toContain("Gửi đề nghị lùi hạn");
    expect(html).not.toContain(NUT_DUYET);
    expect(html).not.toContain("Từ chối");
  });

  it("ô đề nghị lùi hạn LUÔN hiện với người đang làm việc — KHÔNG bị gắn sau khoá duyệt", () => {
    // VẾ CHỊU LỰC. Bài này đỏ đúng vào ngày ai đó "gộp cho gọn" hai nửa của khối lùi hạn — thao
    // tác trông hợp lý, và lấy mất khả năng XIN lùi hạn của mọi cán bộ không phải lãnh đạo.
    for (const ai of [LANH_DAO, NGUOI_KHAC, ""]) {
      const html = veChiTiet({}, ai);
      expect(html).toContain('id="han-moi-lui-han"');
      expect(html).toContain("Gửi đề nghị lùi hạn");
      expect(html).toContain(nhuTrongHTML(GHI_CHU_LUI_HAN));
    }
  });

  it("nhiệm vụ KHÔNG CÓ HẠN: không vẽ ô đề nghị lùi hạn — không có gì để lùi", () => {
    const html = veChiTiet({ due_at: null, original_due_at: null });
    expect(html).not.toContain('id="han-moi-lui-han"');
  });
});

describe("vòng đời — chỉ vẽ bước máy chủ liệt kê", () => {
  it("`dang-thuc-hien`: Chờ duyệt, Hoàn thành (thẳng, không cần `task.approve` — ADR 0065 NV1), Tạm dừng", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (lần hai): bài này ghim "KHÔNG có lối nhảy cóc sang
    // `hoan-thanh`" theo chuỗi chặt §6. Chủ đầu tư chọn bảng require: `dang-thuc-hien` → `hoan-thanh`
    // có thật (`nhiem_vu.go:100`), và danh sách nay đến từ máy chủ trên chính dòng (3b2330b).
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: bài này từng canh ba lối, kể cả `Chuyển sang Chuyển tiếp`.
    // Chủ đầu tư quyết định Chuyển tiếp là giao CÙNG nhiệm vụ cho nơi khác (khối §5.7,
    // `POST …/assignment`), và `…/status` nay trả 400 cho đích ấy (764bb92) — một nút ở đây là một
    // nút chắc chắn hỏng.
    const html = veChiTiet({ status: "dang-thuc-hien" });
    expect(html).toContain("Chuyển sang Chờ duyệt");
    expect(html).toContain("Chuyển sang Tạm dừng");
    expect(html).not.toContain("Chuyển sang Chuyển tiếp");
    expect(html).toContain("Chuyển sang Hoàn thành");
  });

  it("`hoan-thanh`: không nút thường nào — lối ra duy nhất là ô mở lại có lý do", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: bài này ghim `hoan-thanh` là ngõ cụt ("không có lối ra"). Máy
    // chủ nay liệt kê bước mở lại (`nhiem_vu.go:104`); nó đi qua ô lý do, không qua hàng nút.
    const html = veChiTiet({ status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" });
    expect(html).not.toContain("Chuyển sang");
    expect(html).not.toContain("không liệt kê lối ra nào");
    expect(html).toContain('id="ly-do-mo-lai"');
  });

  it("bước `hoan-thanh` VẪN HIỆN dù có thể còn việc con — máy chủ mới là nơi liệt kê mã", () => {
    // Màn hình KHÔNG biết nhiệm vụ có việc con hay không (phản hồi không mang số ấy). Ẩn nút đi
    // "cho chắc" là lấy mất đúng câu từ chối mang danh sách mã mà cán bộ cần đọc.
    const html = veChiTiet({ status: "cho-duyet" });
    expect(html).toContain("Chuyển sang Hoàn thành");
  });
});

describe("câu từ chối của máy chủ vẽ THẲNG, không nuốt thành 'có lỗi xảy ra'", () => {
  it("danh sách mã việc con đi nguyên văn ra trang", () => {
    const cau =
      "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới hoàn thành việc cha";
    const html = renderToStaticMarkup(
      <ChiTietNhiemVu
        nhiemVu={nhiemVu({ status: "cho-duyet" })}
        vanBan={{ pha: "dangTai" }}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maNguoiDangNhap={LANH_DAO}
        quyen={DU_QUYEN}
        dangGui={false}
        loiGhi={cau}
        dong={() => {}}
        doiTrangThai={() => {}}
        xoa={() => {}}
        guiDeNghiLuiHan={KHONG_GOI}
        quyetDinh={KHONG_GOI}
        suaKhoiVanBan={KHONG_SUA}
        docLaiChiTiet={KHONG_SUA}
        {...PASS2_DRAWER_PROPS}
      />,
    );
    expect(html).toContain(nhuTrongHTML(cau));
    expect(html).toContain("NV20");
    expect(html).toContain("NV21");
    expect(html).toContain("NV22");
    // Và KHÔNG có một câu chung chung nào thay thế nó.
    expect(html).not.toContain("Có lỗi xảy ra");
  });

  it("409 `parent_completed` (ADR 0065 NV2): câu nêu mã việc cha ra nguyên văn, kèm nút mở đúng việc cha", () => {
    const cau = "việc cha NV19 đã hoàn thành — mở lại việc cha trước rồi mới mở lại việc con";
    const html = renderToStaticMarkup(
      <ChiTietNhiemVu
        nhiemVu={nhiemVu({ status: "hoan-thanh", parent: "NV19", completed_at: "2026-06-25T02:00:00Z" })}
        vanBan={{ pha: "dangTai" }}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maNguoiDangNhap={LANH_DAO}
        quyen={DU_QUYEN}
        dangGui={false}
        loiGhi={cau}
        dong={() => {}}
        doiTrangThai={() => {}}
        xoa={() => {}}
        guiDeNghiLuiHan={KHONG_GOI}
        quyetDinh={KHONG_GOI}
        suaKhoiVanBan={KHONG_SUA}
        docLaiChiTiet={KHONG_SUA}
        {...PASS2_DRAWER_PROPS}
      />,
    );
    expect(html).toMatch(new RegExp(`role="alert">${nhuTrongHTML(cau)}<`));
    // The one act that unblocks the refusal sits in the same drawer: `ParentTaskField` opens the
    // parent by its register code — no second copy of the parent's code parsed out of the sentence.
    expect(html).toMatch(/Mở việc cha (<!-- -->)?NV19/);
    expect(html).not.toContain("Có lỗi xảy ra");
  });

  it("ô lý do xoá là BẮT BUỘC — nút xoá tắt khi chưa gõ lý do", () => {
    const html = veChiTiet();
    expect(html).toContain('id="ly-do-xoa-nhiem-vu"');
    expect(html).toContain("Xoá nhiệm vụ</button>");
    // `disabled` có mặt vì ô lý do rỗng: xoá mà không ghi lý do là một hồ sơ mất vết (luật 7).
    expect(html).toMatch(/disabled=""[^>]*>Xoá nhiệm vụ|Xoá nhiệm vụ/);
  });
});

describe("bảng danh sách §4.2", () => {
  it("phần trễ TÁCH RIÊNG để tô đỏ, không ghép sẵn vào chuỗi ngày", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain("20/6/2026");
    expect(html).toContain('class="nhan-lech"');
    expect(html).toContain("trễ 86 ngày");
    // Dòng phụ nguồn giao, §4.2.
    expect(html).toContain("Từ kết luận họp");
  });

  it("chưa phân công là một TRẠNG THÁI THẬT, không phải dấu gạch", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu({ assignee: "" })]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(CHUA_PHAN_CONG));
  });

  it("chip `Hoàn thành trễ hạn` so với HẠN BAN ĐẦU, không với hạn hiện tại", () => {
    // Một lần lùi hạn được duyệt sẽ tự xoá dấu vết của chính nó khỏi báo cáo nếu so với `due_at`.
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[
          nhiemVu({
            status: "hoan-thanh",
            // Hạn hiện tại đã được lùi tới tháng 8; hạn ban đầu vẫn là 20/6.
            due_at: "2026-08-30T23:59:59+07:00",
            original_due_at: "2026-06-20T23:59:59+07:00",
            completed_at: "2026-08-25T02:00:00Z",
          }),
        ]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain("Hoàn thành trễ hạn");
  });

  it("KHÔNG vẽ ô tick chọn hàng loạt — `Xoá đã chọn` không có tuyến nào", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).not.toContain('type="checkbox"');
  });

  it("sổ rỗng hiện câu trạng thái rỗng, không hiện một bảng không có dòng nào", () => {
    const html = renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(SO_RONG));
    expect(html).not.toContain("<table");
  });
});

describe("drawer §5 — hai hạn cạnh nhau và hai ô tick", () => {
  it("HẠN XỬ LÝ và HẠN BAN ĐẦU cùng ra trang — đó là toàn bộ điểm của hai cột", () => {
    const html = veChiTiet({
      due_at: "2026-08-30T23:59:59+07:00",
      original_due_at: "2026-06-20T23:59:59+07:00",
    });
    expect(html).toContain("Hạn ban đầu");
    expect(html).toContain("20/6/2026");
    expect(html).toContain("30/8/2026");
  });

  it("chú thích BẮT BUỘC của hai ô tick phê duyệt có mặt", () => {
    expect(veChiTiet()).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("thời gian đã ở trạng thái hiện DẤU GẠCH, không hiện số 0", () => {
    // Mốc đổi trạng thái gần nhất nằm trong nhật ký, và dải bước chưa đọc nhật ký để tính nó. Một
    // số 0 ở đây đọc ra là "vừa chuyển xong", đúng điều ngược lại với "không biết".
    const html = veChiTiet();
    expect(html).toContain(O_TRONG);
    expect(html).not.toContain("0 ngày 0 giờ");
  });

  it("câu giải thích trạng thái hiện tại §5.2 có mặt", () => {
    expect(veChiTiet({ status: "moi-giao" })).toContain(
      nhuTrongHTML("Đã giao nhưng người nhận chưa bấm tiếp nhận."),
    );
  });

  it("`04-bien-ban-hop.md` §7.4 — nhiệm vụ tách từ kết luận có LIÊN KẾT NGƯỢC về biên bản gốc", () => {
    const html = veChiTiet({
      meeting_id: "01JBB1",
      meeting_title: "Giao ban tháng 8",
      conclusion_no: 3,
    });
    expect(html).toContain("Từ kết luận số 3 — Giao ban tháng 8");
    expect(html).toContain('href="/nhiem-vu/bien-ban#bien-ban-01JBB1"');
  });

  it("không có `meeting_id` thì KHÔNG có liên kết về biên bản nào", () => {
    // Bản mẫu mặc định mang `source: "ket-luan-hop"` — vẫn không liên kết khi máy chủ không nối được.
    const html = veChiTiet();
    expect(html).not.toContain("/nhiem-vu/bien-ban");
    expect(html).not.toContain("Từ kết luận số");
  });
});

describe("form Giao việc mới §7", () => {
  it("`Tự sinh mã` MẶC ĐỊNH BẬT, và ô mã chỉ hiện khi tắt nó", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain('id="giao-tu-sinh-ma"');
    expect(html).toContain('checked=""');
    // Mã tự nhập ẩn khi đang tự sinh — một ô mã bỏ trống kèm `auto_code: false` là 400.
    expect(html).not.toContain('id="giao-ma"');
  });

  it("CẢNH BÁO hạn chỉ đặt được MỘT LẦN đứng cạnh ô ngày", () => {
    // `han_ban_dau` lấy cùng mốc lúc INSERT và trigger `nhiem_vu_bat_bien` từ chối mọi lần ghi
    // lại. Một nhiệm vụ tạo ra không hạn thì không bao giờ có hạn nữa.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain('id="giao-han"');
    expect(html).toContain(nhuTrongHTML(CANH_BAO_HAN_MOT_LAN));
  });

  it("hạn ĐIỀN SẴN +7 ngày lúc 17:00 (ADR 0065 NV6), ô giờ bắt buộc khi có ngày, câu nói sửa được", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-30T03:00:00Z")); // Wed 30/09/2026 10:00 ICT
    try {
      const html = renderToStaticMarkup(
        <FormGiaoViec
          danhMuc={DANH_MUC}
          danhBa={DANH_BA}
          danhBaLanhDao={DANH_BA}
          dangGui={false}
          loi={null}
          huy={() => {}}
          giaoViec={() => {}}
        />,
      );
      expect(html).toMatch(/<input id="giao-han"[^>]*type="date"[^>]*value="2026-10-07"/);
      expect(html).toMatch(/<input id="giao-han-gio"[^>]*type="time"[^>]*required=""[^>]*value="17:00"/);
      expect(html).toContain(nhuTrongHTML(NEW_TASK_DUE_PREFILLED_NOTE));
      expect(html).not.toContain("23:59");
    } finally {
      vi.useRealTimers();
    }
  });

  it("ô `Lãnh đạo giao việc` nói ra hệ quả ADR 0038 của việc bỏ trống", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain("không ai duyệt được đề nghị lùi hạn");
  });

  it("màn Nhiệm vụ, loại `theo-van-ban`: vẽ BA danh sách văn bản §7.2, mỗi nhóm một nút thêm", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 24/09/2026 (TASK-02): bài này từng canh "form CHƯA vẽ ba danh sách" và
    // tự ghi là phải đỏ vào ngày chúng được dựng. Hôm nay là ngày ấy.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        coDanhSachVanBan
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain("Văn bản cấp trên giao");
    expect(html).toContain(nhuTrongHTML("Văn bản chỉ đạo của Đảng uỷ"));
    expect(html).toContain("Văn bản sản phẩm đầu ra");
    expect(html.split("+ Thêm văn bản</button>").length - 1).toBe(3);
    expect(html).toContain('id="giao-them-van-ban-cap-tren-giao"');
    expect(html).toContain("Nội dung nhiệm vụ / Trích yếu văn bản");
    // ADR 0065 NV5: no separate lead unit / monitor — they ARE `Đơn vị thực hiện` / `Người thực hiện`.
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
    expect(html).not.toContain("Cơ quan chủ trì");
    expect(html).not.toContain("theo dõi");
    expect(html).toContain('<label for="giao-bo-phan">Đơn vị thực hiện</label>');
    expect(html).toContain("Người thực hiện");
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026 (TASK-04): `POST /api/v1/tasks` nay nhận `note`, nên ô
    // `Ghi chú` §7.2 có mặt — có nhãn, sau ba danh sách, dừng ở cùng giới hạn với form `✎ Sửa`.
    expect(html).toContain('<label for="giao-ghi-chu">Ghi chú</label>');
    expect(html).toMatch(/<textarea id="giao-ghi-chu"[^>]*maxLength="5000"/);
    expect(html.indexOf('id="giao-ghi-chu"')).toBeGreaterThan(
      html.indexOf('id="giao-them-van-ban-san-pham-dau-ra"'),
    );
    expect(html).not.toContain("không nhận ghi chú");
  });

  it("loại `co-ban`: ô tiêu đề thành `Tên nhiệm vụ`; ba danh sách BIẾN MẤT", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC_CO_BAN}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        coDanhSachVanBan
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(html).toContain(">Tên nhiệm vụ</label>");
    expect(html).not.toContain("Trích yếu văn bản");
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
    expect(html).not.toContain('id="giao-ghi-chu"');
  });

  it("màn Biên bản (không truyền prop): loại `theo-van-ban` mà KHÔNG có ba danh sách", () => {
    // Đúng cách `features/bien-ban/so-bien-ban.tsx` gọi form: không có `coDanhSachVanBan`.
    // `petitions.tachKetLuanVao` không có `documents` — vẽ ba danh sách ở đó là để cán bộ gõ văn
    // bản rồi thấy chúng mất. Hai ô `lead_unit`/`monitor` đã gỡ ở mọi màn (ADR 0065 NV5).
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
        tieuDeCoSan="Kết luận giả của cuộc họp"
      />,
    );
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
    // `petitions.tachKetLuanVao` không có `note`: ô ấy ở đây là chữ gõ vào rồi mất.
    expect(html).not.toContain('id="giao-ghi-chu"');
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
  });
});

describe("ô ngày ↔ mốc của hợp đồng", () => {
  it("ngày thành mốc CUỐI NGÀY, không phải 00:00", () => {
    // 00:00 làm nhiệm vụ "hạn 20/6" quá hạn suốt cả ngày 20/6 — đọc lên là sai.
    expect(mocCuoiNgay("2026-06-20")).toBe("2026-06-20T23:59:59+07:00");
  });

  it("mốc về ô ngày cắt theo MÚI GIỜ VIỆT NAM, không cắt mười ký tự đầu", () => {
    // `2026-06-20T00:30:00+07:00` lưu ở UTC là `2026-06-19T17:30:00Z`; phép cắt chuỗi cho ra
    // ngày 19 — một hạn lệch một ngày.
    expect(ngayChoONhap("2026-06-19T17:30:00Z")).toBe("2026-06-20");
    expect(ngayChoONhap(null)).toBe("");
  });
});

describe("phần chưa dựng được — ra tới màn hình, không giấu trong chú thích mã", () => {
  it("mọi mục có mặt, kể cả ba chỗ phát hiện trong lượt này", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(nhuTrongHTML(p.ten));
    }
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: bài này từng canh chữ "KHÔNG phát ra `id`" — lý do Thêm việc
    // con không gửi được. Từ ad7f821 `parent` nhận MÃ SỔ, khối Nhiệm vụ con đã dựng
    // (`child-tasks.tsx`), nên mục ấy rời danh sách.
    expect(html).not.toContain("KHÔNG phát ra `id`");
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026 (nhóm A, mục 7): bài này từng canh chữ `task.extend` — nó đến từ
    // mục "CỔNG QUYỀN Ở GIAO DIỆN cho bảy khoá `task.*`". Cổng nay đã dựng (`quyenNhiemVu`), nên mục
    // ấy rời danh sách; một mục còn nằm đó sau khi đã dựng là mục đẩy người sau đi dựng lại.
    expect(html).not.toContain("CỔNG QUYỀN Ở GIAO DIỆN");
    expect(html).not.toContain("chưa có hằng nào cho chúng");
    // ĐỔI CÓ CHỦ Ý 27/09/2026 (TASK-05): bài này từng canh chữ `admin.user` — lý do ba ô cán bộ là
    // ô gõ mã. Ba ô nay đổ từ `GET /api/v1/staff-directory`, nên mục ấy rời danh sách; phần còn
    // thiếu thu lại đúng một điều — ô tìm theo tên đặc tả vẽ.
    expect(html).not.toContain("admin.user");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (TASK-02 lượt web 1): ô tìm theo tên nay đã dựng, nên mục
    // cuối cùng của nhóm ấy cũng rời danh sách.
    expect(html).not.toContain("Gõ tên để tìm…");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BA Ô CHỌN CÁN BỘ CỦA FORM GIAO VIỆC — đổ từ danh bạ chọn người, GỬI MÃ `CB-…`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Đoạn HTML của MỘT `<select>` theo `id`, để phép so không xanh nhờ một ô khác. */
function oChon(html: string, id: string): string {
  const dau = html.indexOf(`<select id="${id}"`);
  expect(dau).toBeGreaterThanOrEqual(0);
  return html.slice(dau, html.indexOf("</select>", dau));
}

/** The listbox of ONE `StaffCombobox`, by input id — so a match cannot come from another box. */
function comboboxListbox(html: string, id: string): string {
  const start = html.indexOf(`<ul id="${id}-danh-sach"`);
  expect(start).toBeGreaterThanOrEqual(0);
  return html.slice(start, html.indexOf("</ul>", start));
}

function veForm(danhBa: KetQua<identity_danhBaChonNguoiRa> | null): string {
  return renderToStaticMarkup(
    <FormGiaoViec
      danhMuc={DANH_MUC}
      danhBa={danhBa}
      // Cùng câu trả lời cho ô lãnh đạo: các ca dưới canh BA PHA của danh bạ, không canh bộ lọc
      // quyền — bộ lọc có nhóm riêng (`ô Lãnh đạo giao việc — chỉ người cầm quyền duyệt gia hạn`).
      danhBaLanhDao={danhBa}
      dangGui={false}
      loi={null}
      huy={() => {}}
      giaoViec={() => {}}
    />,
  );
}

describe("form Giao việc — ô chọn cán bộ", () => {
  it("giá trị mỗi lựa chọn là MÃ NGHIỆP VỤ `CB-…`, chữ hiện là họ tên · chức vụ", () => {
    const html = veForm(DANH_BA);
    for (const id of ["giao-nguoi-thuc-hien", "giao-lanh-dao"]) {
      const o = oChon(html, id);
      expect(o).toContain(`value="${LANH_DAO}"`);
      expect(o).toContain(`value="${NGUOI_KHAC}"`);
      expect(o).toContain(">Trần Văn Lãnh · Chủ tịch</option>");
      expect(o).not.toContain("disabled");
    }
    // Có nhãn gắn đúng ô — ô chọn không nhãn là ô trình đọc màn hình đọc thành "hộp chọn".
    expect(html).toContain('for="giao-lanh-dao"');
    // Ô gõ mã cũ đã đi.
    expect(html).not.toContain('placeholder="CB-…"');
  });

  it("danh bạ ĐỌC HỎNG: câu lỗi nguyên văn máy chủ, `role=\"alert\"`, ô chỉ còn lựa chọn trống", () => {
    const html = veForm({ ok: false, thongBao: "Máy chủ danh bạ đang bảo trì." });
    expect(html).toContain('role="alert"');
    expect(html).toContain("Máy chủ danh bạ đang bảo trì.");
    const o = oChon(html, "giao-lanh-dao");
    expect(o.split("<option").length - 1).toBe(1);
    expect(o).toContain('value=""');
    expect(o).not.toContain("CB-");
  });

  it("danh bạ CHƯA ĐỌC XONG: ô khoá, nói đang tải, và nút Giao việc khoá", () => {
    const html = veForm(null);
    const o = oChon(html, "giao-nguoi-thuc-hien");
    expect(o).toContain("disabled");
    expect(o).toContain("Đang tải danh bạ cán bộ…");
    expect(html).not.toContain('role="alert"');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §5.4 — KHỐI "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO", CHỈ ĐỌC
 *
 * Ca nặng nhất ở đây là ca KHÔNG ai thấy lúc phát triển: dòng của sổ vắng `documents` có chủ ý, và
 * một drawer vẽ từ dòng ấy sẽ hiện ba nhóm `—` — "nhiệm vụ này không có văn bản" — cho một nhiệm
 * vụ có ba văn bản. Mọi bài dưới đây canh để câu ấy chỉ xuất hiện khi TUYẾN CHI TIẾT đã nói thế.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

function vb(sua: Partial<petitions_nhiemVuVanBanRa> = {}): petitions_nhiemVuVanBanRa {
  return {
    id: "01JVANBAN1",
    group: "cap-tren-giao",
    reference: "1742-CV/BTCTU",
    date: "2026-06-09",
    summary: "Công văn của Ban Tổ chức Thành uỷ",
    position: 1,
    ...sua,
  };
}

const BA_VAN_BAN: petitions_nhiemVuVanBanRa[] = [
  vb(),
  vb({
    id: "01JVANBAN2",
    group: "san-pham-dau-ra",
    reference: "324-BC/ĐU",
    date: "2026-06-15",
    summary: "Báo cáo của Ban Thường vụ Đảng uỷ",
    position: 1,
  }),
];

/** Dòng của sổ như tuyến `GET /api/v1/tasks` trả: KHÔNG có trường `documents`. */
function dongSo(sua: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  const n = nhiemVu(sua);
  delete n.documents;
  return n;
}

const NHAN_BA_NHOM = [
  "Văn bản cấp trên giao",
  "Văn bản chỉ đạo của Đảng uỷ",
  "Văn bản sản phẩm đầu ra",
];

function veTuDrawer(d: DrawerNhiemVu | null): string {
  if (d === null) throw new Error("drawer phải đang mở");
  return veChiTiet(d.nhiemVu, LANH_DAO, d.vanBan);
}

describe("§5.4 — drawer đọc TUYẾN CHI TIẾT, không đọc dòng của sổ", () => {
  it("mở từ một dòng sổ (vắng `documents`): khối ĐANG TẢI, KHÔNG bao giờ là ba nhóm rỗng", () => {
    const d = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    expect(d?.vanBan).toEqual({ pha: "dangTai" });
    expect(d?.luotDoc).toBe(1);

    const html = veTuDrawer(d);
    expect(html).toContain(nhuTrongHTML(DANG_TAI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
  });

  it("`mo` không tin `documents` của dòng được bấm, kể cả khi dòng ấy mang một mảng", () => {
    // Nguồn duy nhất của khối là tuyến chi tiết. Một ngày tuyến sổ đổi hình dạng thì drawer vẫn
    // đọc lại, thay vì âm thầm vẽ thứ tuyến sổ chưa từng hứa.
    const d = chuyenDrawer(null, { loai: "mo", nhiemVu: nhiemVu({ documents: [] }) });
    expect(d?.vanBan).toEqual({ pha: "dangTai" });
  });

  it("chi tiết về: ba nhóm hiện đủ, theo đúng nhãn §5.4, và trường vô hướng lấy theo chi tiết", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN, note: "Ghi chú từ chi tiết" }) },
    });
    expect(d?.vanBan).toEqual({ pha: "xong", duLieu: BA_VAN_BAN });
    expect(d?.nhiemVu.note).toBe("Ghi chú từ chi tiết");

    const html = veTuDrawer(d);
    expect(html).toContain(nhuTrongHTML(TIEU_DE_KHOI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).toContain(nhuTrongHTML(nhan));
    expect(html).toContain("1742-CV/BTCTU · 9/6/2026");
    expect(html).toContain(nhuTrongHTML("Công văn của Ban Tổ chức Thành uỷ"));
    expect(html).toContain("324-BC/ĐU · 15/6/2026");
    // Nhóm giữa rỗng thật — máy chủ ĐÃ nói thế — nên nó là dấu gạch.
    expect(html).toContain("Văn bản chỉ đạo của Đảng uỷ</dt><dd>—</dd>");
  });

  it("đọc chi tiết HỎNG: câu lỗi hiện ra, và KHÔNG có nhóm nào — không một danh sách rỗng", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: false, thongBao: "Không tìm thấy nhiệm vụ." },
    });
    expect(d?.vanBan).toEqual({ pha: "loi", thongBao: "Không tìm thấy nhiệm vụ." });

    const html = veTuDrawer(d);
    expect(html).toContain('role="alert"');
    expect(html).toContain(nhuTrongHTML(KHONG_DOC_DUOC_VAN_BAN));
    expect(html).toContain(nhuTrongHTML("Không tìm thấy nhiệm vụ."));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
  });

  it("chi tiết về mà THIẾU `documents`: hợp đồng bị vỡ — báo lỗi, không đọc thành rỗng", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const d = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: dongSo() },
    });
    expect(d?.vanBan).toEqual({ pha: "loi", thongBao: CHI_TIET_THIEU_VAN_BAN });
  });

  it("câu trả lời của một lượt ĐÃ CŨ, hoặc của nhiệm vụ khác, bị bỏ", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const cu = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 0,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    expect(cu).toBe(mo);
    const khac = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV20",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ code: "NV20", documents: BA_VAN_BAN }) },
    });
    expect(khac).toBe(mo);
  });
});

describe("§5.4 — một lần đổi trạng thái KHÔNG làm rơi khối văn bản", () => {
  function daDocXong(): DrawerNhiemVu | null {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    return chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
  }

  it("phản hồi `…/status` (vắng `documents`): trạng thái mới, văn bản CŨ vẫn hiện, và đọc lại", () => {
    const truoc = daDocXong();
    const sau = chuyenDrawer(truoc, {
      loai: "ghiXong",
      nhiemVu: dongSo({ status: "cho-duyet" }),
    });
    expect(sau?.nhiemVu.status).toBe("cho-duyet");
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: BA_VAN_BAN });
    // Lượt tăng ⇒ hiệu ứng đọc lại chi tiết.
    expect(sau?.luotDoc).toBe((truoc?.luotDoc ?? 0) + 1);

    const html = veTuDrawer(sau);
    expect(html).toContain("1742-CV/BTCTU · 9/6/2026");
    expect(html).not.toContain(nhuTrongHTML(DANG_TAI_VAN_BAN));
  });

  it("lượt đọc GỬI TRƯỚC lần ghi mà về SAU bị bỏ — không đè trạng thái cũ lên trạng thái mới", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const sau = chuyenDrawer(mo, { loai: "ghiXong", nhiemVu: dongSo({ status: "cho-duyet" }) });
    const muon = chuyenDrawer(sau, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ status: "dang-thuc-hien", documents: BA_VAN_BAN }) },
    });
    expect(muon?.nhiemVu.status).toBe("cho-duyet");
  });

  it("phản hồi mang mảng (tuyến tạo): lấy thẳng, không phải chờ", () => {
    const d = chuyenDrawer(null, {
      loai: "ghiXong",
      nhiemVu: nhiemVu({ code: "NV34", documents: [] }),
    });
    expect(d?.vanBan).toEqual({ pha: "xong", duLieu: [] });
  });

  it("`ghiXong` của MỘT NHIỆM VỤ KHÁC (vắng `documents`): KHÔNG mượn văn bản của việc đang mở", () => {
    // Giữ khối cũ chỉ đúng khi cùng mã. Mượn nó sang việc khác là vẽ văn bản của NV19 dưới tên
    // NV34 — và mở luôn `✎ Sửa` trên một tập không phải của NV34.
    const truoc = daDocXong();
    const sau = chuyenDrawer(truoc, { loai: "ghiXong", nhiemVu: dongSo({ code: "NV34" }) });
    expect(sau?.nhiemVu.code).toBe("NV34");
    expect(sau?.vanBan).toEqual({ pha: "dangTai" });

    const html = veTuDrawer(sau);
    expect(html).not.toContain("1742-CV/BTCTU");
    expect(theNutSua(html)).toContain('disabled=""');
  });

  it("đóng drawer là hết trạng thái", () => {
    expect(chuyenDrawer(daDocXong(), { loai: "dong" })).toBeNull();
  });
});

describe("§5.4 — chỉ với loại `Theo văn bản`, và câu chữ của từng dòng", () => {
  const XONG: TaiVanBan = { pha: "xong", duLieu: BA_VAN_BAN };

  it("loại `co-ban`: KHÔNG vẽ khối, kể cả khi đã có văn bản trong tay", () => {
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, XONG);
    expect(html).not.toContain(nhuTrongHTML(TIEU_DE_KHOI_VAN_BAN));
    for (const nhan of NHAN_BA_NHOM) expect(html).not.toContain(nhuTrongHTML(nhan));
    expect(html).not.toContain("1742-CV/BTCTU");
    // Các trường §5.4 vô hướng vẫn giữ nguyên, cùng chú thích bắt buộc.
    expect(html).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("loại `theo-van-ban` với cùng dữ liệu: khối CÓ — bài trên không xanh vì lý do sai", () => {
    expect(veChiTiet({}, LANH_DAO, XONG)).toContain("1742-CV/BTCTU");
  });

  it("số ký hiệu rỗng ⇒ `Không số`; ngày rỗng ⇒ bỏ hẳn phần ngày", () => {
    const html = veChiTiet({}, LANH_DAO, {
      pha: "xong",
      duLieu: [vb({ reference: "", date: "" })],
    });
    expect(html).toContain(`<li>${KHONG_SO}<span`);
    expect(html).not.toContain(`${KHONG_SO} · `);
  });

  it("chi tiết trả mảng RỖNG: lúc này ba nhóm `—` là đúng — máy chủ đã nói không có dòng nào", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: [] });
    for (const nhan of NHAN_BA_NHOM) {
      expect(html).toContain(`${nhuTrongHTML(nhan)}</dt><dd>—</dd>`);
    }
  });

  it("thứ tự trong nhóm là thứ tự MÁY CHỦ GỬI, không sắp lại theo `position`", () => {
    const html = veChiTiet({}, LANH_DAO, {
      pha: "xong",
      duLieu: [
        vb({ id: "a", reference: "90-TB/TU", position: 3 }),
        vb({ id: "b", reference: "12-CV/UBND", position: 1 }),
      ],
    });
    expect(html.indexOf("90-TB/TU")).toBeGreaterThan(-1);
    expect(html.indexOf("90-TB/TU")).toBeLessThan(html.indexOf("12-CV/UBND"));
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §5.4 — NÚT `✎ SỬA`
 *
 * Ca nặng nhất là ca bị TỪ CHỐI, không phải ca mở được: `documents` trên PATCH là thay cả tập, nên
 * một nút `✎ Sửa` bấm được lúc khối còn đang tải là một nút gỡ mất mọi văn bản cán bộ chưa thấy.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Thẻ mở của nút `✎ Sửa`, hoặc `null` khi trang không có nút ấy. */
function theNutSua(html: string): string | null {
  const k = /<button[^>]*aria-label="Sửa sổ theo dõi văn bản chỉ đạo"[^>]*>/.exec(html);
  return k === null ? null : k[0];
}

describe("§5.4 — nút `✎ Sửa`: chỉ `Theo văn bản`, và KHOÁ khi chưa đọc đủ văn bản", () => {
  it("`theo-van-ban`, khối đã đọc xong: nút có mặt và BẤM ĐƯỢC", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN });
    const nut = theNutSua(html);
    expect(nut).not.toBeNull();
    expect(nut).not.toContain("disabled");
    // ADR 0068: `✎` is a lucide icon now; the visible word is `Sửa` (NHAN_NUT_SUA, shared with
    // the Nội dung screen, keeps its glyph there). The accessible name is the aria-label above.
    expect(html).toMatch(/aria-label="Sửa sổ theo dõi văn bản chỉ đạo"><svg[^>]*aria-hidden="true"[^>]*>.*?<\/svg>Sửa<\/button>/);
  });

  it("loại `co-ban`: KHÔNG có nút, kể cả khi đã có văn bản trong tay", () => {
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN });
    expect(theNutSua(html)).toBeNull();
    expect(html).not.toContain(NHAN_NUT_SUA);
    expect(html).not.toContain("aria-label=\"Sửa sổ theo dõi văn bản chỉ đạo\"");
  });

  it("khối ĐANG TẢI: nút KHOÁ, và lý do khoá ra tới trang", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "dangTai" });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(theNutSua(html)).toContain('aria-describedby="ly-do-khoa-sua-van-ban"');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_DANG_TAI));
  });

  it("khối đọc HỎNG: nút KHOÁ, và lý do khoá ra tới trang", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "loi", thongBao: "Không tìm thấy nhiệm vụ." });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_LOI));
  });

  it("khối ĐÃ ĐỌC XONG nhưng có một mã nhóm lạ: nút VẪN KHOÁ, lý do ra trang, dòng lạ vẫn hiện", () => {
    // Pha `xong` là pha duy nhất hai ca trên không phủ: một nút chỉ khoá theo pha sẽ mở ở đây, và
    // lần lưu đầu tiên gỡ mất dòng form không có chỗ vẽ.
    const la = vb({ id: "01JVANBANLA", group: "nhom-moi-gia", reference: "77-TB/GIA" });
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: [...BA_VAN_BAN, la] });
    expect(theNutSua(html)).toContain('disabled=""');
    expect(html).toContain(nhuTrongHTML(KHOA_SUA_NHOM_LA));
    expect(html).toContain("77-TB/GIA");
  });
});

describe("§5.4 — phản hồi PATCH cập nhật drawer", () => {
  it("`ghiXong` với phản hồi PATCH (mang `documents`): trường vô hướng VÀ khối văn bản lấy theo phản hồi", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const truoc = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    const conMot = [BA_VAN_BAN[1] as petitions_nhiemVuVanBanRa];
    const sau = chuyenDrawer(truoc, {
      loai: "ghiXong",
      nhiemVu: nhiemVu({ note: "Ghi chú giả sau khi sửa", leader_approved: true, documents: conMot }),
    });
    expect(sau?.nhiemVu.note).toBe("Ghi chú giả sau khi sửa");
    expect(sau?.nhiemVu.leader_approved).toBe(true);
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: conMot });
    // Lượt tăng ⇒ đọc lại chi tiết, như mọi lần ghi khác.
    expect(sau?.luotDoc).toBe((truoc?.luotDoc ?? 0) + 1);

    const html = veTuDrawer(sau);
    expect(html).not.toContain("1742-CV/BTCTU");
    expect(html).toContain("324-BC/ĐU · 15/6/2026");
  });

  it("phản hồi PATCH gỡ hết văn bản (`documents: []`): khối là ba nhóm `—`, không phải khối cũ", () => {
    const mo = chuyenDrawer(null, { loai: "mo", nhiemVu: dongSo() });
    const truoc = chuyenDrawer(mo, {
      loai: "chiTietVe",
      ma: "NV19",
      luotDoc: 1,
      kq: { ok: true, duLieu: nhiemVu({ documents: BA_VAN_BAN }) },
    });
    const sau = chuyenDrawer(truoc, { loai: "ghiXong", nhiemVu: nhiemVu({ documents: [] }) });
    expect(sau?.vanBan).toEqual({ pha: "xong", duLieu: [] });
  });
});

describe("§5.4 — form `✎ Sửa`", () => {
  function veForm(sua: Partial<petitions_nhiemVuRa> = {}): string {
    return renderToStaticMarkup(
      <FormSuaKhoiVanBan
        nhiemVu={nhiemVu(sua)}
        vanBan={BA_VAN_BAN}
        luu={KHONG_SUA}
        docLai={KHONG_SUA}
        xong={() => {}}
      />,
    );
  }

  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): ca này ghim "Mã, Hạn HIỆN mà KHÔNG có ô nhập". Chủ đầu tư
  // khi ấy cho sửa cả hai (3c3525f, f27fd6e). ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026 (ADR 0065 NV3, NV5): mã đã
  // cấp KHÔNG sửa nữa (máy chủ trả 400), và Cơ quan chủ trì / Chuyên viên không còn là trường riêng.
  it("Hạn (ngày + giờ) là Ô NHẬP có nhãn và câu giải thích; KHÔNG có ô Mã, KHÔNG có Cơ quan chủ trì / Chuyên viên", () => {
    const html = veForm();
    expect(html).not.toContain('id="sua-ma-nhiem-vu"');
    expect(html).not.toContain(">Mã nhiệm vụ</label>");
    expect(html).not.toContain("Cơ quan chủ trì");
    expect(html).not.toContain("Chuyên viên");
    expect(html).toContain("<legend>Hạn xử lý</legend>");
    expect(html).toMatch(/<input id="sua-han-ngay"[^>]*type="date"[^>]*value="2026-06-20"/);
    // The time is REQUIRED once a date is there — the server defaults no hour.
    expect(html).toMatch(/<input id="sua-han-gio"[^>]*type="time"[^>]*required=""[^>]*value="23:59"/);
    expect(html).toContain(nhuTrongHTML(DUE_EDIT_NOTE));
    expect(html).not.toContain("<select");
    // One date field for the deadline, plus one per document.
    expect(html.split('type="date"').length - 1).toBe(BA_VAN_BAN.length + 1);
  });

  it("việc CHƯA CÓ HẠN: hai ô trống, ô giờ CHƯA bắt buộc khi chưa chọn ngày; không có giờ gõ cứng", () => {
    const html = veForm({ due_at: null, original_due_at: null });
    expect(html).toMatch(/<input id="sua-han-ngay"[^>]*value=""/);
    expect(html).toMatch(/<input id="sua-han-gio"[^>]*value=""/);
    expect(html).not.toMatch(/<input id="sua-han-gio"[^>]*required=""/);
    expect(html).not.toContain("17:00");
  });

  it("dây nối: lịch làm việc đọc từ `layLichLamViec`, giờ điền sẵn chỉ vào ô TRỐNG, lỗi lưu đọc lại (đọc mã)", () => {
    // No DOM: the effect and the change handler never run here. Pinned by source instead.
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain("layLichLamViec().then((r) => {");
    expect(src).toContain(
      'cu.dueTime === "" && calendar.pha === "xong" ? defaultDueTime(calendar.shifts, date) : cu.dueTime',
    );
    expect(src).toContain("setStaleNote(changedSince(await docLai(), goc.nhiemVu.updated_at));");
    expect(src).toContain('guiDrawer({ loai: "docLai", ma: code });');
    expect(src).not.toMatch(/dueTime: "1\d:\d\d"/);
  });

  it("năm ô sửa được có nhãn thật, và chú thích BẮT BUỘC của hai ô tick vẫn hiện", () => {
    const html = veForm();
    expect(html).toContain('<label for="sua-tieu-de">Nội dung nhiệm vụ / Trích yếu văn bản</label>');
    expect(html).toContain('id="sua-tom-tat-ket-qua"');
    expect(html).toContain('<label for="sua-ghi-chu">Ghi chú</label>');
    expect(html).toContain('id="sua-lanh-dao-phe-duyet"');
    expect(html).toContain('id="sua-cap-tren-cong-nhan"');
    expect(html).toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("mọi dòng đã có hiện ra để sửa, dùng CÙNG ô của form tạo; không có lối chuyển nhóm", () => {
    const html = veForm();
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN1"');
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN2"');
    expect(html).toContain("Công văn của Ban Tổ chức Thành uỷ");
    expect(html).toContain('value="1742-CV/BTCTU"');
    expect(html).toContain('value="2026-06-09"');
    expect(html.split("+ Thêm văn bản</button>").length - 1).toBe(3);
    // Id không đụng form tạo khi hai form cùng mở.
    expect(html).not.toContain('id="giao-');
  });

  it("vừa mở, chưa đổi gì: nút `Lưu` KHOÁ và câu nói vì sao", () => {
    const html = veForm();
    expect(html).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Lưu<\/button>/);
    expect(html).toContain("Chưa có gì thay đổi để lưu.");
    expect(html).toContain(">Huỷ</button>");
  });
});

describe("§5.9 Nhật ký & Trao đổi — khối trong drawer", () => {
  const CB = "CB-2026-3H8N2W";
  const DANH_BA_THEO_MA = new Map([
    [CB, { code: CB, full_name: "Trần Thị B", position: "", department_id: "" }],
  ]);

  function veNhatKy(tai: TaiNhatKyNhiemVu, loiThem: string | null = null): string {
    return renderToStaticMarkup(
      <KhoiNhatKyNhiemVu
        maNhiemVu="NV19"
        tai={tai}
        nhanTT={BANG_NHAN_MAC_DINH}
        danhBa={DANH_BA_THEO_MA}
        tenBoPhan={TEN_BO_PHAN}
        loiThem={loiThem}
        dangTaiThem={false}
        xemThem={() => {}}
      />,
    );
  }

  it("drawer có khối, và lúc mở nó ĐANG TẢI (role=status) — chưa nói rỗng khi chưa biết", () => {
    const html = veChiTiet();
    expect(html).toContain("Nhật ký &amp; Trao đổi");
    expect(html).toContain('role="status">Đang tải nhật ký…');
    expect(html).not.toContain(nhuTrongHTML(NHAT_KY_RONG));
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W2): ca này ghim "KHÔNG có ô ghi tay — tuyến ghi chưa dựng". Tuyến
  // nay có (60011e8), nên ô hiện cho người được ghi và VẮNG cho người không được — cả hai chiều dưới.
  it("CÓ ô ghi tay cho người cầm `task.update`: nhãn, gợi ý §5.9, nút khoá khi trống, và `📎 Đính kèm`", () => {
    const html = veChiTiet();
    expect(html).toContain('<label for="ghi-nhat-ky-NV19">Ghi vào nhật ký của nhiệm vụ</label>');
    expect(html).toContain('placeholder="Đã làm được gì, còn vướng gì…"');
    expect(html).toMatch(/<textarea id="ghi-nhat-ky-NV19"[^>]*maxLength="5000"/);
    // ADR 0068: `➤` is a decorative lucide icon before the word now.
    expect(html).toMatch(/<button type="submit" class="nut-chinh" disabled=""><svg[^>]*aria-hidden="true"[^>]*>.*?<\/svg>Ghi nhật ký<\/button>/);
    expect(html).toContain("Dòng đã ghi không sửa, không xoá được");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (A4): this pinned "no 📎" — the server had no file store. It has
    // one now (ADR 0052, b37ec2d); the picker is part of the form, behind the same gate.
    // ADR 0068: `📎` is a lucide icon now; the label still says `Đính kèm`.
    expect(html).toMatch(/<label for="ghi-nhat-ky-NV19-dinh-kem" class="nut-phu"><svg[^>]*>.*?<\/svg>Đính kèm/);
    expect(html).toMatch(/<input id="ghi-nhat-ky-NV19-dinh-kem"[^>]*type="file"[^>]*multiple=""/);
    // The form sits INSIDE the §5.9 block, above the timeline.
    const khoi = html.slice(html.indexOf('aria-labelledby="tieu-de-nhat-ky-nhiem-vu-NV19"'));
    expect(khoi.indexOf("ghi-nhat-ky-NV19")).toBeLessThan(khoi.indexOf("Đang tải nhật ký"));
  });

  it("CÓ ô cho người liên quan KHÔNG có khoá: người thực hiện, giao việc, tạo", () => {
    for (const ma of ["CB-2026-3H8N2W", LANH_DAO, "CB-2026-VANTHU"]) {
      const html = veChiTiet({}, ma, { pha: "dangTai" }, quyenNhiemVu([]));
      expect(html).toContain('id="ghi-nhat-ky-NV19"');
    }
  });

  it("KHÔNG có ô — người ngoài không khoá; phiên chưa đọc trên việc chưa có người giao việc", () => {
    expect(veChiTiet({}, NGUOI_KHAC, { pha: "dangTai" }, quyenNhiemVu([]))).not.toContain(
      "Ghi nhật ký</button>",
    );
    const html = veChiTiet({ assigner: "" }, "", { pha: "dangTai" }, quyenNhiemVu([]));
    expect(html).not.toContain('id="ghi-nhat-ky-NV19"');
    // The timeline itself still reads.
    expect(html).toContain("Nhật ký &amp; Trao đổi");
  });

  it("sau 201: nhật ký ĐỌC LẠI (khoá đọc mang bộ đếm), khoá chống trùng thay mới — dây nối (đọc mã)", () => {
    // No DOM here, so `onSubmit` cannot run. What is pinned: the read key includes the counter the
    // form bumps; the key is replaced only on success and reused otherwise.
    const src = readFileSync(fileURLToPath(new URL("./nhat-ky-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain("const khoaDoc = `${maNhiemVu}|${lanLamMoi}|${written}`;");
    expect(src).toContain("onWritten={() => setWritten((n) => n + 1)}");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (A4): the entry now carries the STORED attachment ids.
    expect(src).toContain("addTaskLogEntry(taskCode, note, key, storedIds(files.items))");
    const ok = src.indexOf("setRefusal(null);");
    expect(src.indexOf("setKey(crypto.randomUUID());")).toBeGreaterThan(ok);
    expect(src.indexOf("setRefusal(r.thongBao);")).toBeLessThan(ok);
  });

  it("rỗng: câu §5.9 nguyên văn, không danh sách, không `Xem thêm`", () => {
    const html = veNhatKy({ pha: "xong", dong: [], conNua: false });
    expect(html).toContain(nhuTrongHTML(NHAT_KY_RONG));
    expect(html).not.toContain("<ol");
    expect(html).not.toContain("Xem thêm");
  });

  it("có dòng: thời điểm, họ tên kèm mã, nhãn trạng thái, bộ phận/phụ trách, ghi chú", () => {
    const html = veNhatKy({
      pha: "xong",
      dong: [
        {
          id: "nknv-2",
          at: "2026-09-09T07:20:00Z",
          actor_code: CB,
          status: "dang-thuc-hien",
          unit: "01JBOPHAN",
          assignee: CB,
          note: "Giao lại cho <b>văn phòng</b>.",
          attachments: [],
        },
        {
          id: "nknv-1",
          at: "2026-09-01T02:00:00Z",
          actor_code: "CB-00007",
          status: "moi-giao",
          unit: "",
          assignee: "",
          note: "Tạo nhiệm vụ.",
          attachments: [],
        },
      ],
      conNua: true,
    });
    expect(html).toContain('dateTime="2026-09-09T07:20:00Z"');
    expect(html).toContain("14:20");
    expect(html).toContain("Trần Thị B (CB-2026-3H8N2W)");
    // Người không có trong danh bạ: chỉ mã.
    expect(html).toContain("<strong>CB-00007</strong>");
    expect(html).toContain("Đang thực hiện");
    expect(html).toContain("Mới giao");
    expect(html).toContain("VĂN PHÒNG ĐẢNG ỦY · Trần Thị B (CB-2026-3H8N2W)");
    // Ghi chú là TEXT, không phải HTML: thẻ trong đó bị thoát.
    expect(html).toContain("&lt;b&gt;văn phòng&lt;/b&gt;");
    expect(html).not.toContain("<b>văn phòng</b>");
    // Mới nhất trước — đúng thứ tự máy chủ trả.
    expect(html.indexOf("Giao lại")).toBeLessThan(html.indexOf("Tạo nhiệm vụ."));
    expect(html).toContain("Xem thêm</button>");
  });

  it("đọc hỏng: câu máy chủ NGUYÊN VĂN, role=alert — và KHÔNG nói `Chưa có ghi chép nào.`", () => {
    const html = veNhatKy({ pha: "loi", thongBao: "Không tìm thấy nhiệm vụ." });
    expect(html).toContain('role="alert">Không tìm thấy nhiệm vụ.');
    expect(html).not.toContain(nhuTrongHTML(NHAT_KY_RONG));
    expect(html).not.toContain("Xem thêm");
  });

  it("`Xem thêm` hỏng: câu máy chủ hiện, các dòng đã tải vẫn ở lại", () => {
    const html = veNhatKy(
      {
        pha: "xong",
        dong: [
          {
            id: "a",
            at: "2026-09-09T07:20:00Z",
            actor_code: CB,
            status: "moi-giao",
            unit: "",
            assignee: "",
            note: "Dòng đã có.",
            attachments: [],
          },
        ],
        conNua: true,
      },
      "Con trỏ không hợp lệ.",
    );
    expect(html).toContain("Dòng đã có.");
    expect(html).toContain('role="alert">Con trỏ không hợp lệ.');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHÓM A (27/09/2026) — họ tên thay mã, hàng quá hạn, mức ưu tiên mặc định, cổng nút theo khoá
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Một mã có thật trên bản ghi nhưng KHÔNG có trong danh bạ — người đã nghỉ, tài khoản bị khoá. */
const NGUOI_DA_NGHI = "CB-2019-NGHIHUU";

describe("họ tên thay mã `CB-…` — danh bạ đọc MỘT LẦN, mã lạ vẫn hiện mã", () => {
  const DANH_BA_MA = new Map(
    (DANH_BA.ok ? DANH_BA.duLieu.items : []).map((cb) => [cb.code, cb] as const),
  );

  function veBang(assignee: string, danhBa: typeof DANH_BA_MA | null): string {
    return renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu({ assignee })]}
        danhMuc={DANH_MUC}
        danhBa={danhBa}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
  }

  it("cột `Người thực hiện`: họ tên khi danh bạ có, KHÔNG còn mã trần", () => {
    const html = veBang(NGUOI_KHAC, DANH_BA_MA);
    expect(html).toContain("<td>Nguyễn Thị Thực</td>");
    expect(html).not.toContain(`<td>${NGUOI_KHAC}</td>`);
  });

  it("mã KHÔNG có trong danh bạ: hiện MÃ, không bao giờ để trống", () => {
    // VẾ CHỊU LỰC. Một ô trống đọc ra là "chưa giao cho ai" — với đúng người đã làm việc ấy.
    const html = veBang(NGUOI_DA_NGHI, DANH_BA_MA);
    expect(html).toContain(`<td>${NGUOI_DA_NGHI}</td>`);
    expect(html).not.toContain("<td></td>");
  });

  it("danh bạ chưa về (hoặc hỏng): hiện MÃ; chưa phân công vẫn là `Chưa phân công`", () => {
    expect(veBang(NGUOI_KHAC, null)).toContain(`<td>${NGUOI_KHAC}</td>`);
    expect(veBang("", DANH_BA_MA)).toContain(nhuTrongHTML(CHUA_PHAN_CONG));
  });

  it("drawer: người thực hiện, lãnh đạo giao việc — `Họ tên (CB-…)`, mã lạ là mã", () => {
    const html = veChiTiet(
      { assignee: NGUOI_KHAC, assigner: NGUOI_DA_NGHI },
      LANH_DAO,
      { pha: "dangTai" },
      DU_QUYEN,
      DANH_BA,
    );
    expect(html).toContain(`Nguyễn Thị Thực (${NGUOI_KHAC})`);
    expect(html).toContain(`<dd>${NGUOI_DA_NGHI}</dd>`);
    const khac = veChiTiet({ assigner: LANH_DAO }, LANH_DAO, { pha: "dangTai" }, DU_QUYEN, DANH_BA);
    expect(khac).toContain(`<dd>Trần Văn Lãnh (${LANH_DAO})</dd>`);
  });

  it("drawer KHÔNG còn `Cơ quan chủ trì tham mưu` / `Chuyên viên theo dõi` (ADR 0065 NV5)", () => {
    const html = veChiTiet({ assignee: NGUOI_KHAC }, LANH_DAO, { pha: "dangTai" }, DU_QUYEN, DANH_BA);
    expect(html).not.toContain("Cơ quan chủ trì");
    expect(html).not.toContain("Chuyên viên theo dõi");
    expect(html).not.toContain("Chuyên viên Văn phòng");
  });
});

describe("§4.2 — hàng quá hạn tô nền hồng rất nhạt, SUY RA từ hạn", () => {
  function veMotDong(sua: Partial<petitions_nhiemVuRa>): string {
    return renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu(sua)]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        sapXep={SAP_XEP_MAC_DINH}
        doiSapXep={() => {}}
        moNhiemVu={() => {}}
      />,
    );
  }

  it("quá hạn: nền hồng, VÀ chữ `(trễ N ngày)` cùng dòng — màu không đứng một mình", () => {
    const html = veMotDong({});
    // LỚP, không còn giá trị inline — `.dong-qua-han` ở `globals.css`.
    expect(html).toMatch(/<tr data-tre-han="" class="dong-qua-han">/);
    expect(html).not.toContain("style=");
    expect(html).toContain("(trễ 86 ngày)");
  });

  it("chưa tới hạn, và không có hạn: không tô", () => {
    for (const sua of [
      { due_at: "2026-12-20T23:59:59+07:00" },
      { due_at: null, original_due_at: null },
    ]) {
      const html = veMotDong(sua);
      expect(html).not.toContain("dong-qua-han");
      expect(html).not.toContain("data-tre-han");
    }
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TASK-04 (27/09/2026) — trả lại để làm tiếp · ô lãnh đạo chỉ gợi người cầm `task.extend` ·
 * sắp xếp bảng Danh sách
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("`Trả lại để làm tiếp` — chỉ ở `cho-duyet`, chỉ với `task.approve`, lý do bắt buộc", () => {
  const Q_DUYET = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
  const Q_CAP_NHAT = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);
  const NHAN_NUT = nhuTrongHTML(NHAN_NUT_TRA_LAI);

  it("`cho-duyet` + `task.approve`: ô lý do bắt buộc, có nhãn, và nút KHOÁ khi ô còn trống", () => {
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_KHAC, { pha: "dangTai" }, Q_DUYET);
    expect(html).toContain(NHAN_NUT);
    expect(html).toContain('<label for="ly-do-tra-lai">Lý do trả lại (bắt buộc)</label>');
    expect(html).toMatch(/<textarea id="ly-do-tra-lai"[^>]*required=""/);
    expect(html).toMatch(/<button type="submit" class="nut-phu" disabled="">Trả lại để làm tiếp<\/button>/);
    // KHÔNG có nút thường `Chuyển sang Đang thực hiện` — cú bấm ấy sẽ gửi lý do rỗng, tức 400.
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
    // Câu giải thích lấy tên trạng thái đích từ bảng nhãn, không gõ cứng.
    expect(html).toContain(nhuTrongHTML(ghiChuTraLai(BANG_NHAN_MAC_DINH)));
  });

  it("`cho-duyet`, THIẾU `task.approve`: không có ô trả lại — và câu nói vì sao", () => {
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_KHAC, { pha: "dangTai" }, Q_CAP_NHAT);
    expect(html).not.toContain(NHAN_NUT);
    expect(html).not.toContain('id="ly-do-tra-lai"');
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("phiên chưa đọc được: không có ô trả lại — fail closed", () => {
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_KHAC, { pha: "dangTai" }, quyenNhiemVu(null));
    expect(html).not.toContain(NHAN_NUT);
  });

  it("trạng thái khác `cho-duyet`: không có ô trả lại, kể cả với đủ khoá", () => {
    for (const status of ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "tam-dung", "hoan-thanh"]) {
      const html = veChiTiet({ status }, NGUOI_KHAC, { pha: "dangTai" }, Q_DUYET);
      expect(html).not.toContain(NHAN_NUT);
      expect(html).not.toContain('id="ly-do-tra-lai"');
    }
  });

  it("bước THUẬN `da-tiep-nhan` → `dang-thuc-hien` vẫn là một nút thường, không đòi lý do", () => {
    const html = veChiTiet({ status: "da-tiep-nhan" }, NGUOI_KHAC, { pha: "dangTai" }, Q_CAP_NHAT);
    expect(html).toContain("Chuyển sang Đang thực hiện");
    expect(html).not.toContain('id="ly-do-tra-lai"');
  });

  it("thành phần `KhoiTraLai` vẽ riêng: đúng một ô, một nút khoá lúc đầu", () => {
    const html = renderToStaticMarkup(
      <KhoiTraLai kind="return" nhanTT={BANG_NHAN_MAC_DINH} dangGui={false} gui={() => {}} />,
    );
    expect(html.split('id="ly-do-tra-lai"').length - 1).toBe(1);
    expect(html).toContain('maxLength="5000"');
    expect(html).toContain('disabled=""');
  });

  // ⚠ LẦN BẤM GỬI KHÔNG ĐƯỢC CANH Ở ĐÂY: môi trường kiểm là Node không DOM, nên `onSubmit` của
  // `KhoiTraLai` không chạy được. Hai mắt xích hai bên nó có bài riêng — `yeuCauTraLai` (lý do → đích
  // `dang-thuc-hien` + `note` đã cắt, `nhan-nhiem-vu.test.ts`) và thân `{status, note}` của
  // `doiTrangThaiNhiemVu` (`lib/api/nhiem-vu.test.ts`). Dây nối giữa là một dòng
  // `gui(yeuCau.trangThai, yeuCau.ghiChu)` không bài nào chạy qua.

  it("câu từ chối của máy chủ (403/400) ra NGUYÊN VĂN trong drawer", () => {
    const cau = "Trả lại để làm tiếp phải ghi lý do — người thực hiện cần biết còn thiếu gì.";
    const html = renderToStaticMarkup(
      <ChiTietNhiemVu
        nhiemVu={nhiemVu({ status: "cho-duyet" })}
        vanBan={{ pha: "dangTai" }}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maNguoiDangNhap={NGUOI_KHAC}
        quyen={Q_DUYET}
        dangGui={false}
        loiGhi={cau}
        dong={() => {}}
        doiTrangThai={() => {}}
        xoa={() => {}}
        guiDeNghiLuiHan={KHONG_GOI}
        quyetDinh={KHONG_GOI}
        suaKhoiVanBan={KHONG_SUA}
        docLaiChiTiet={KHONG_SUA}
        {...PASS2_DRAWER_PROPS}
      />,
    );
    expect(html).toContain(`role="alert">${nhuTrongHTML(cau)}</p>`);
  });
});

describe("ô `Lãnh đạo giao việc` — chỉ người cầm quyền duyệt gia hạn", () => {
  const CHI_LANH_DAO: KetQua<identity_danhBaChonNguoiRa> = {
    ok: true,
    duLieu: {
      items: [{ code: LANH_DAO, full_name: "Trần Văn Lãnh", position: "Chủ tịch", department_id: "" }],
    },
  };

  function veHaiDanhBa(
    danhBa: KetQua<identity_danhBaChonNguoiRa> | null,
    danhBaLanhDao: KetQua<identity_danhBaChonNguoiRa> | null,
    loi: string | null = null,
  ): string {
    return renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={danhBa}
        danhBaLanhDao={danhBaLanhDao}
        dangGui={false}
        loi={loi}
        huy={() => {}}
        giaoViec={() => {}}
        // Tiêu đề điền sẵn: tiêu đề trống cũng khoá nút gửi, và khi ấy hai ca "nút khoá / nút mở"
        // dưới đây sẽ xanh hoặc đỏ vì tiêu đề chứ không vì danh bạ.
        tieuDeCoSan="Việc giả của bài kiểm"
      />,
    );
  }

  it("ô lãnh đạo đổ từ danh bạ ĐÃ LỌC; ô người thực hiện vẫn đổ từ danh bạ cả xã", () => {
    const html = veHaiDanhBa(DANH_BA, CHI_LANH_DAO);
    const lanhDao = oChon(html, "giao-lanh-dao");
    expect(lanhDao).toContain(`value="${LANH_DAO}"`);
    expect(lanhDao).not.toContain(`value="${NGUOI_KHAC}"`);
    expect(oChon(html, "giao-nguoi-thuc-hien")).toContain(`value="${NGUOI_KHAC}"`);
  });

  it("danh bạ đã lọc RỖNG: một câu nói không ai có quyền — KHÔNG vẽ ô chọn rỗng", () => {
    const html = veHaiDanhBa(DANH_BA, { ok: true, duLieu: { items: [] } });
    expect(html).not.toContain('<select id="giao-lanh-dao"');
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN));
    // Không rỗng: nút gửi không bị khoá vì chuyện này — giao việc vẫn phải làm được.
    expect(html).toMatch(/<button type="submit" class="nut-chinh">Giao việc<\/button>/);
  });

  it("danh bạ đã lọc ĐỌC HỎNG: câu máy chủ nguyên văn, `role=\"alert\"`, ô chỉ còn lựa chọn trống", () => {
    const html = veHaiDanhBa(DANH_BA, { ok: false, thongBao: "Khoá quyền dùng để lọc không hợp lệ." });
    expect(html).toContain(nhuTrongHTML(cauLoiDanhBaLanhDao("Khoá quyền dùng để lọc không hợp lệ.")));
    const o = oChon(html, "giao-lanh-dao");
    expect(o.split("<option").length - 1).toBe(1);
    // Và câu "không ai có quyền" KHÔNG hiện — đọc hỏng không phải là "không có ai".
    expect(html).not.toContain(nhuTrongHTML(CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN));
  });

  it("danh bạ đã lọc CÒN ĐANG TẢI: ô khoá và nút Giao việc khoá — lãnh đạo không ghi lại được sau", () => {
    const html = veHaiDanhBa(DANH_BA, null);
    expect(oChon(html, "giao-lanh-dao")).toContain("disabled");
    expect(html).toMatch(/<button type="submit" class="nut-chinh" disabled="">Giao việc<\/button>/);
  });

  it("màn Nhiệm vụ (`staffSearch`): ô GÕ TÊN của lãnh đạo vẫn chỉ gợi người cầm `task.extend`", () => {
    // TASK-02 lượt web 1 (28/09/2026). Đổi loại ô KHÔNG được đổi nguồn: ô tìm đổ từ `db` cả xã sẽ
    // để gõ ra một người không bao giờ duyệt được lùi hạn — và cột ấy không sửa lại được sau.
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={CHI_LANH_DAO}
        staffSearch
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    const approvers = comboboxListbox(html, "giao-lanh-dao");
    expect(approvers).toContain(`data-value="${LANH_DAO}"`);
    expect(approvers).not.toContain(`data-value="${NGUOI_KHAC}"`);
    expect(comboboxListbox(html, "giao-nguoi-thuc-hien")).toContain(`data-value="${NGUOI_KHAC}"`);
    // Không còn `<select>` nào cho hai ô cán bộ trên màn Nhiệm vụ.
    for (const id of ["giao-nguoi-thuc-hien", "giao-lanh-dao"]) {
      expect(html).not.toContain(`<select id="${id}"`);
    }
  });

  it("máy chủ từ chối lãnh đạo (400) hoặc không kiểm được (503): câu NGUYÊN VĂN trên form", () => {
    for (const cau of [
      "Người được chọn làm lãnh đạo giao việc không hợp lệ. Hãy chọn người khác trong danh sách.",
      "Chưa kiểm tra được lãnh đạo giao việc nên nhiệm vụ CHƯA được tạo. Vui lòng thử lại sau ít phút.",
    ]) {
      const html = veHaiDanhBa(DANH_BA, CHI_LANH_DAO, cau);
      expect(html).toContain(`role="alert">${nhuTrongHTML(cau)}</p>`);
    }
  });
});

describe("§4.2 — tiêu đề sắp được: Mã, Tên việc, Ngày giao, Ưu tiên và Hạn", () => {
  function veBangSapXep(sapXep: SapXepSo): string {
    return renderToStaticMarkup(
      <BangNhiemVu
        nhiemVu={[nhiemVu()]}
        danhMuc={DANH_MUC}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={TEN_BO_PHAN}
        bayGio={BAY_GIO}
        maDangMo={null}
        moNhiemVu={() => {}}
        sapXep={sapXep}
        doiSapXep={() => {}}
      />,
    );
  }

  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: was "đúng HAI nút sắp" with `Hạn` as plain text. The server sorts
  // by `due_at` since ad7f821, so `Hạn` gets the third button.
  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3b): was "đúng BA", with `Tên việc` plain. Backend P9 sorts by
  // `title` and `priority` too — five buttons; the other columns stay plain text.
  it("đúng NĂM nút sắp, `aria-sort` đúng chiều ở cột đang sắp, `none` ở các cột kia", () => {
    const html = veBangSapXep({ cot: "code", chieu: "asc" });
    expect(html.split('class="nut-sap-xep"').length - 1).toBe(5);
    expect(html).toContain('<th scope="col" aria-sort="ascending"><button type="button" class="nut-sap-xep">Mã ↑</button></th>');
    expect(html).toContain('aria-sort="none"><button type="button" class="nut-sap-xep">Tên việc ⇅</button>');
    expect(html).toContain('aria-sort="none"><button type="button" class="nut-sap-xep">Ngày giao ⇅</button>');
    expect(html).toContain('aria-sort="none"><button type="button" class="nut-sap-xep">Ưu tiên ⇅</button>');
    expect(html).toContain('aria-sort="none"><button type="button" class="nut-sap-xep">Hạn ⇅</button>');
    // Các cột còn lại là chữ thường — không mũi tên nào hứa một cách sắp máy chủ không có.
    expect(html).toContain('<th scope="col">Người thực hiện</th>');
    expect(html).toContain('<th scope="col">Trạng thái</th>');
  });

  it("đang sắp theo Ưu tiên giảm dần: mũi tên xuống ở đúng cột ấy; câu `chưa có mức ưu tiên nằm cuối` có sẵn", () => {
    const html = veBangSapXep({ cot: "priority", chieu: "desc" });
    expect(html).toContain('aria-sort="descending"><button type="button" class="nut-sap-xep">Ưu tiên ↓</button>');
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain('<p className="ghi-chu">{NO_PRIORITY_LAST_NOTE}</p>');
  });

  it("mặc định (`created_at` giảm dần): mũi tên xuống ở Ngày giao; ô ngày hiện ngày giao", () => {
    const html = veBangSapXep(SAP_XEP_MAC_DINH);
    expect(html).toContain("Ngày giao ↓");
    expect(html).toContain('<time dateTime="2026-06-01T02:00:00Z">1/6/2026</time>');
  });

  it("đổi cách sắp là VỀ TRANG ĐẦU — con trỏ cũ thuộc cách sắp cũ, máy chủ trả 400", () => {
    // Đang ở trang 3 theo mặc định. Con trỏ giữ `sort`/`order` bên trong (`core/page/page.go:456`).
    const loc = { trangThai: "cho-duyet" };
    const moi = bamSapXep(loc, "code");
    expect(moi.nganXep).toEqual(TRANG_DAU);
    expect(moi.loc).toEqual({ trangThai: "cho-duyet", sapXep: "code", chieu: "asc" });
    // Lượt đọc kế tiếp mang cách sắp mới và KHÔNG mang con trỏ nào.
    const duong = duongDanSoNhiemVu({ ...moi.loc, cursor: moi.nganXep.hienTai, limit: 20 });
    expect(duong).toBe("/api/v1/tasks?status=cho-duyet&sort=code&order=asc&limit=20");
    // Bấm lại đúng cột: đảo chiều, vẫn về trang đầu.
    expect(bamSapXep(moi.loc, "code")).toEqual({
      loc: { trangThai: "cho-duyet", sapXep: "code", chieu: "desc" },
      nganXep: TRANG_DAU,
    });
  });
});

describe("§7.1 — mức ưu tiên mặc định lấy từ DANH MỤC CỦA XÃ, không gõ cứng `Thường`", () => {
  const muc = (code: string, label: string, is_default: boolean, active = true) => ({
    id: `01J${code}`,
    code,
    label,
    is_default,
    active,
    order: 1,
    source: "he-thong",
    tier: 1,
  });

  function oUuTien(mucUuTien: DanhMucNhiemVu["mucUuTien"]): string {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={{ ...DANH_MUC, mucUuTien }}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    return oChon(html, "giao-uu-tien");
  }

  it("xã đặt `binh-thuong` làm mặc định: ô chọn sẵn đúng dòng ấy", () => {
    const o = oUuTien([muc("khan", "Khẩn", false), muc("binh-thuong", "Bình thường", true)]);
    expect(o).toContain('<option value="binh-thuong" selected="">Bình thường</option>');
    expect(o).not.toMatch(/<option value="" selected="">/);
  });

  it("xã CHƯA đặt dòng mặc định nào: ô đứng ở `— Chưa xác định —`", () => {
    const o = oUuTien([muc("khan", "Khẩn", false), muc("cao", "Cao", false)]);
    expect(o).toMatch(/<option value="" selected="">/);
  });

  it("dòng mặc định đã NGỪNG dùng: không chọn sẵn thứ xã đã bỏ", () => {
    const o = oUuTien([muc("thuong", "Thường", true, false), muc("cao", "Cao", false)]);
    expect(o).toMatch(/<option value="" selected="">/);
  });
});

describe("cổng nút theo khoá `task.*` — CA BỊ TỪ CHỐI, không chỉ ca được phép", () => {
  it("phiên chưa đọc được (`null`): MỌI cổng đóng — fail closed", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: thêm cổng thứ sáu `reassign` (`task.assign`, khối §5.7).
    expect(quyenNhiemVu(null)).toEqual({
      giaoViec: false,
      capNhat: false,
      duyetHoanThanh: false,
      xoa: false,
      duyetGiaHan: false,
      reassign: false,
    });
  });

  it("so CHÍNH XÁC từng khoá: `task.read` không mở nút ghi nào, không có phép khớp `task.*`", () => {
    const q = quyenNhiemVu(["task.read", "task.*", "task"]);
    expect(Object.values(q).every((v) => v === false)).toBe(true);
  });

  it("`duyetHoanThanh` là `task.approve` MỘT MÌNH — cổng dòng là việc của `canMoveTask`", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: ca này ghim `false` (cổng tuyến là `task.update`). Từ ea55113 cổng
    // tuyến `…/status` là `task.read` và dòng nhận NGƯỜI THỰC HIỆN không cần `task.update`, nên người
    // thực hiện cầm `task.approve` duyệt được. Người không phải người thực hiện, thiếu `task.update`,
    // vẫn không thấy khối — xem nhóm "người thực hiện" bên dưới.
    expect(quyenNhiemVu([QUYEN_DUYET_HOAN_THANH_NHIEM_VU]).duyetHoanThanh).toBe(true);
  });

  it("chỉ đọc: không khối chuyển trạng thái, không ô đề nghị lùi hạn, không xoá, không ✎ Sửa", () => {
    const html = veChiTiet(
      { status: "dang-thuc-hien" },
      NGUOI_KHAC,
      { pha: "xong", duLieu: [] },
      quyenNhiemVu([]),
    );
    expect(html).not.toContain("Chuyển sang");
    expect(html).not.toContain("<h4>Chuyển trạng thái</h4>");
    expect(html).not.toContain('id="han-moi-lui-han"');
    expect(html).not.toContain('id="ly-do-xoa-nhiem-vu"');
    // ADR 0068: the button no longer shows the `✎ Sửa` glyph text, so absence is checked on the
    // button itself — a not-contains on the old text would pass whatever this page drew.
    expect(theNutSua(html)).toBeNull();
    // Phần ĐỌC vẫn nguyên: hạn, khối văn bản, nhật ký.
    expect(html).toContain("Hạn ban đầu");
    expect(html).toContain(nhuTrongHTML(TIEU_DE_KHOI_VAN_BAN));
    expect(html).toContain("Nhật ký &amp; Trao đổi");
  });

  it("có `task.update`, thiếu `task.approve`: ở `dang-thuc-hien` Hoàn thành CÓ, không câu thiếu quyền", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026 (ADR 0065 NV1, 90a17153): ca này ghim bước Hoàn thành ẨN ở
    // `dang-thuc-hien` khi thiếu `task.approve`. Bước duyệt nay tuỳ chọn: hoàn thành thẳng không cần khoá.
    const html = veChiTiet(
      { status: "dang-thuc-hien" },
      NGUOI_KHAC,
      { pha: "dangTai" },
      quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]),
    );
    expect(html).toContain("Chuyển sang Tạm dừng");
    expect(html).toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("BỊ TỪ CHỐI — có `task.update`, thiếu `task.approve`, ở `cho-duyet`: không duyệt, không trả lại, kèm câu vì sao", () => {
    const html = veChiTiet(
      { status: "cho-duyet" },
      NGUOI_KHAC,
      { pha: "dangTai" },
      quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]),
    );
    expect(html).not.toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain('id="ly-do-tra-lai"');
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("đủ `task.update` + `task.approve`: bước Hoàn thành có, câu thiếu quyền KHÔNG hiện", () => {
    const html = veChiTiet(
      { status: "cho-duyet" },
      NGUOI_KHAC,
      { pha: "dangTai" },
      quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]),
    );
    expect(html).toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("ĐÚNG lãnh đạo giao việc nhưng THIẾU `task.extend`: không mời duyệt — lớp một đóng", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "dangTai" }, quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]));
    expect(html).not.toContain(`href="#${ID_HANG_CHO}"`);
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    // ADR 0038: ô XIN lùi hạn không đứng sau khoá duyệt — người cầm `task.update` vẫn gửi được.
    expect(html).toContain('id="han-moi-lui-han"');
  });

  it("có `task.extend` nhưng KHÔNG phải lãnh đạo ghi trên bản ghi: lớp hai vẫn chặn", () => {
    // Khoá thật KHÔNG thay được phép so mã của ADR 0038 — lớp hai chạy SAU lớp khoá, không thay nó.
    const html = veChiTiet({}, NGUOI_KHAC, { pha: "dangTai" }, DU_QUYEN);
    expect(html).not.toContain(`href="#${ID_HANG_CHO}"`);
    expect(html).toContain(nhuTrongHTML(CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC));
  });

  it("`task.delete` một mình: chỉ ô xoá hiện", () => {
    const html = veChiTiet({}, NGUOI_KHAC, { pha: "dangTai" }, quyenNhiemVu([QUYEN_XOA_NHIEM_VU]));
    expect(html).toContain('id="ly-do-xoa-nhiem-vu"');
    expect(html).not.toContain("Chuyển sang");
    expect(html).not.toContain('id="han-moi-lui-han"');
  });

  it("`+ Giao việc mới` đứng sau `task.create` — không suy ra từ `task.update`", () => {
    expect(quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]).giaoViec).toBe(false);
    expect(quyenNhiemVu([QUYEN_TAO_NHIEM_VU]).giaoViec).toBe(true);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * W1 (28/09/2026) — danh sách bước của MÁY CHỦ, người thực hiện tự đổi trạng thái, mở lại
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("khối Chuyển trạng thái — người thực hiện, danh sách máy chủ, mở lại", () => {
  const NGUOI_THUC_HIEN = "CB-2026-3H8N2W"; // `assignee` of the default row
  const KHONG_KHOA = quyenNhiemVu([]);
  const CHI_DUYET = quyenNhiemVu([QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
  const Q_CAP_NHAT = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);

  it("ĐƯỢC — người thực hiện, KHÔNG có `task.update`: khối hiện, đúng các bước máy chủ liệt kê", () => {
    const html = veChiTiet({ status: "dang-thuc-hien" }, NGUOI_THUC_HIEN, { pha: "dangTai" }, KHONG_KHOA);
    expect(html).toContain("<h4>Chuyển trạng thái</h4>");
    expect(html).toContain("Chuyển sang Chờ duyệt");
    expect(html).toContain("Chuyển sang Tạm dừng");
    // ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026 (ADR 0065 NV1): người thực hiện hoàn thành thẳng, không cần
    // `task.approve` — máy chủ quyết lại trên dòng (và vẫn đòi mọi việc con đã xong).
    expect(html).toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("BỊ TỪ CHỐI — người thực hiện ở `cho-duyet`, thiếu `task.approve`: không tự duyệt, không trả lại", () => {
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_THUC_HIEN, { pha: "dangTai" }, KHONG_KHOA);
    expect(html).toContain("<h4>Chuyển trạng thái</h4>");
    expect(html).not.toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain('id="ly-do-tra-lai"');
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("BỊ TỪ CHỐI — không phải người thực hiện, không `task.update`: không khối, kể cả có `task.approve`", () => {
    for (const q of [KHONG_KHOA, CHI_DUYET]) {
      const html = veChiTiet({ status: "dang-thuc-hien" }, NGUOI_KHAC, { pha: "dangTai" }, q);
      expect(html).not.toContain("<h4>Chuyển trạng thái</h4>");
      expect(html).not.toContain("Chuyển sang");
      expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
    }
  });

  it("BỊ TỪ CHỐI — phiên chưa đọc (mã rỗng) trên việc CHƯA phân công: không khớp, không khối", () => {
    const html = veChiTiet({ status: "dang-thuc-hien", assignee: "" }, "", { pha: "dangTai" }, KHONG_KHOA);
    expect(html).not.toContain("<h4>Chuyển trạng thái</h4>");
  });

  it("chỉ vẽ bước MÁY CHỦ liệt kê: danh sách rỗng ⇒ không nút nào, và câu nói vì sao", () => {
    const html = veChiTiet({ status: "dang-thuc-hien", allowed_transitions: [] }, NGUOI_KHAC, { pha: "dangTai" }, DU_QUYEN);
    expect(html).not.toContain("Chuyển sang");
    expect(html).toContain("Máy chủ không liệt kê lối ra nào khỏi trạng thái này.");
  });

  it("`Tiếp tục` sau tạm dừng: đúng ba bước máy chủ trả, không còn `Chờ duyệt`", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: màn hình từng hiện bốn lối (kể cả `cho-duyet`) và để máy chủ
    // từ chối ba lối sai theo luật "trạng thái trước lúc dừng". Luật ấy đã bỏ; danh sách là của máy chủ.
    const html = veChiTiet({ status: "tam-dung" }, NGUOI_KHAC, { pha: "dangTai" }, Q_CAP_NHAT);
    expect(html).toContain("Chuyển sang Mới giao");
    expect(html).toContain("Chuyển sang Đã tiếp nhận");
    expect(html).toContain("Chuyển sang Đang thực hiện");
    expect(html).not.toContain("Chuyển sang Chờ duyệt");
  });

  it("`hoan-thanh` + `task.approve`: ô MỞ LẠI với lý do bắt buộc, nút khoá khi trống — không nút thường", () => {
    const html = veChiTiet(
      { status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" },
      NGUOI_KHAC,
      { pha: "dangTai" },
      DU_QUYEN,
    );
    expect(html).toContain(`<h4>${REOPEN_BUTTON}</h4>`);
    expect(html).toContain(`<label for="ly-do-mo-lai">${REOPEN_REASON_LABEL}</label>`);
    expect(html).toMatch(/<textarea id="ly-do-mo-lai"[^>]*required=""/);
    expect(html).toMatch(new RegExp(`<button type="submit" class="nut-phu" disabled="">${REOPEN_BUTTON}</button>`));
    expect(html).toContain(nhuTrongHTML(reopenNote(BANG_NHAN_MAC_DINH)));
    // Không phải bước trả lại, và không phải một nút `Chuyển sang …` gửi lý do rỗng.
    expect(html).not.toContain('id="ly-do-tra-lai"');
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
  });

  it("`hoan-thanh`, THIẾU `task.approve`: không ô mở lại — và câu nói vì sao", () => {
    const html = veChiTiet({ status: "hoan-thanh" }, NGUOI_KHAC, { pha: "dangTai" }, Q_CAP_NHAT);
    expect(html).not.toContain('id="ly-do-mo-lai"');
    expect(html).not.toContain(REOPEN_BUTTON);
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("người thực hiện cầm `task.approve` (không `task.update`): mở lại được việc của mình", () => {
    const html = veChiTiet({ status: "hoan-thanh" }, NGUOI_THUC_HIEN, { pha: "dangTai" }, CHI_DUYET);
    expect(html).toContain('id="ly-do-mo-lai"');
  });
});

describe("W5 — tab `Liên quan đến tôi` và ô `Sắp đến hạn` trên hàng lọc", () => {
  const SRC = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("ba tab phạm vi và ô tick mới có mặt, gửi đúng giá trị (đọc mã: không có DOM)", () => {
    expect(SRC).toContain('onClick={() => datLoc({ ...loc, phamVi: "related" })}');
    expect(SRC).toContain("{SCOPE_RELATED_LABEL}");
    expect(SRC).toContain("onChange={(e) => datLoc({ ...loc, dueSoon: e.target.checked ? true : undefined })}");
    expect(SRC).toContain("{DUE_SOON_FILTER_LABEL}");
    // The Kanban counts are read with the SAME `loc` as the cards (`getTaskCounts(loc)` shares the
    // filter builder with the list — see lib/api/nhiem-vu.test.ts).
    expect(SRC).toContain("getTaskCounts(loc).then(");
  });
});
