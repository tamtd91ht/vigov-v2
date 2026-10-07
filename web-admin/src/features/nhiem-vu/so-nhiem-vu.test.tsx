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
  GHI_CHU_LANH_DAO_GIAO_VIEC,
  GHI_CHU_TU_SINH_MA,
  MO_TA_FORM_GIAO_VIEC,
  EXTENSION_NO_DUE,
  TASK_PROGRESS_PENDING,
  TASK_TYPE_MISSING,
  TASK_TYPE_PLACEHOLDER,
  cardHolderText,
  childFormNote,
  ghiChuKanbanReNhanh,
  nhanNguonGiao,
  CAU_THIEU_QUYEN_DUYET_GIA_HAN,
  CAU_THIEU_QUYEN_DUYET_HOAN_THANH,
  DECISION_NOTE_LABEL,
  TASK_EXTENSIONS_EMPTY,
  TASK_EXTENSIONS_LOADING,
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
  NHAN_NUT_SUA,
  NHAT_KY_RONG,
  O_TRONG,
  PHAM_VI_CUA_TOI,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  SCOPE_RELATED_LABEL,
  SO_RONG,
  TIEU_DE_KHOI_VAN_BAN,
  CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN,
  CAU_KHONG_AI_DUYET_DUOC,
  NHAN_NUT_TRA_LAI,
  REOPEN_BUTTON,
  SAP_XEP_MAC_DINH,
  cauLoiDanhBaLanhDao,
  mocCuoiNgay,
  ngayChoONhap,
  basicTaskEditBody,
  formSuaTuChiTiet,
  quyetDinhDuyetLuiHan,
  quyenNhiemVu,
  type QuyenNhiemVu,
  type SapXepSo,
} from "./nhan-nhiem-vu";
import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import { duongDanHangChoLuiHan, duongDanSoNhiemVu } from "@/lib/api/nhiem-vu";
import {
  EXTENSION_REASON_PLACEHOLDER,
  EXTENSION_SEND,
  EXTENSION_TITLE,
  EXTENSION_WAITING,
  TaskExtensionView,
  taskExtensionsQuery,
} from "./task-extension-block";
import { PERSON_PICKER_PLACEHOLDER } from "./task-person-picker";
import { INPUT_CLASS, LABEL_CLASS } from "./task-spec";
import {
  BangNhiemVu,
  ChiTietNhiemVu,
  CREATE_TASK_BODY_CLASS,
  CREATE_TASK_DIALOG_CLASS,
  FormGiaoViec,
  FormSuaKhoiVanBan,
  HangLoc,
  KhoiChuaDung,
  LEADER_EMPTY_LABEL,
  LEADER_HINT_SPEC,
  TASK_DELETE_BUTTON,
  TASK_INFO_EDIT_LABEL,
  bamSapXep,
  chuyenDrawer,
  progressFactText,
  type DanhMucNhiemVu,
  type DrawerNhiemVu,
  type TrangThaiTai,
} from "./so-nhiem-vu";
import { KhoiNhatKyNhiemVu, type TaiNhatKyNhiemVu } from "./nhat-ky-nhiem-vu";
import { NOT_SENT, serverTransitions } from "./task-transitions.fixture";
import {
  STATUS_MOVE_DENIED,
  STATUS_NO_EXIT,
  chipMove,
  reasonMoveName,
} from "./task-status-pipeline";

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
    extension_count: 0,
    pending_extension: false,
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
      requires_directive: true,
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
      requires_directive: false,
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
      dong={() => {}}
      doiTrangThai={NOT_SENT}
      xoa={NOT_SENT}
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
  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 07 §6f, owner decision #2 lần 2): the drawer's extension
  // section draws ONE state — the pending request (from `GET /task-extensions?task=`) or the form to
  // ask. Before the read returns there is no row, so the drawer itself never shows a decide button;
  // the gate cases run on `TaskExtensionView` below, where a row is present.
  it("không phải lãnh đạo giao việc: KHÔNG có nút duyệt trong drawer", () => {
    const html = veChiTiet({}, NGUOI_KHAC);
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("nhiệm vụ CHƯA GHI lãnh đạo giao việc: không ai duyệt được, kể cả người đang xem", () => {
    // `"" === ""` is true: without the empty-string check EVERY account could decide every request
    // on every task with no recorded assigner.
    const html = veChiTiet({ assigner: "" }, "");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("phiên chưa đọc được (không có mã cán bộ): FAIL CLOSED, không mở nút", () => {
    const html = veChiTiet({}, "");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("một ULID KHÔNG khớp một mã cán bộ — hai loại định danh khác nhau", () => {
    const html = veChiTiet({}, "01JBGQ3M4K5N6P7Q8R9S0T1U2V");
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
  });

  it("drawer có khối `Đề nghị lùi hạn` của CHÍNH nhiệm vụ này, đang tải — không chỉ đường tới hàng chờ", () => {
    const html = veChiTiet({}, LANH_DAO);
    expect(html).toContain(`>${EXTENSION_TITLE}</h3>`);
    expect(html).toContain(nhuTrongHTML(TASK_EXTENSIONS_LOADING));
    expect(html).not.toContain(`href="#${ID_HANG_CHO}"`);
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
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
    function extensionView(
      p: Partial<Parameters<typeof TaskExtensionView>[0]> = {},
    ): string {
      return renderToStaticMarkup(
        <TaskExtensionView
          load={{ phase: "done", rows: [row] }}
          dueAt="2026-06-20T23:59:59+07:00"
          directory={null}
          sessionStaffCode={LANH_DAO}
          canApproveExtension
          canRequest
          deciding={false}
          note=""
          setNote={() => {}}
          decide={() => {}}
          newDue=""
          setNewDue={() => {}}
          reason=""
          setReason={() => {}}
          sending={false}
          send={() => {}}
          {...p}
        />,
      );
    }
    function list(session: string, canApprove: boolean, assigner = LANH_DAO): string {
      return extensionView({
        load: { phase: "done", rows: [{ ...row, task_assigner: assigner }] },
        sessionStaffCode: session,
        canApproveExtension: canApprove,
      });
    }

    it("đúng lãnh đạo giao việc + `task.extend`: hộp cam `Xin lùi hạn tới …`, lý do, HAI nút và ô ghi chú tuỳ chọn", () => {
      const html = list(LANH_DAO, true);
      expect(html).toContain('class="border-tangerine/30 bg-tangerine/6 rounded-[10px] border p-3"');
      expect(html).toContain(">Xin lùi hạn tới 20/7/2026</p>");
      expect(html).toContain("Chờ số liệu của thôn");
      expect(html).toContain('aria-label="Duyệt lùi hạn NV19"');
      expect(html).toContain('aria-label="Từ chối lùi hạn NV19"');
      expect(html).toContain(nhuTrongHTML(DECISION_NOTE_LABEL));
      // A pending request REPLACES the form (spec 07 §6f: one state at a time).
      expect(html).not.toContain('id="han-moi-lui-han"');
    });

    it("DENIED — người KHÁC: đề nghị vẫn hiện CHỈ ĐỌC, không nút, `Đang chờ lãnh đạo duyệt.`", () => {
      const html = list(NGUOI_KHAC, true);
      expect(html).toContain("Chờ số liệu của thôn");
      expect(html).not.toContain(NUT_DUYET);
      expect(html).not.toContain("Từ chối lùi hạn");
      expect(html).toContain(`>${EXTENSION_WAITING}</p>`);
    });

    it("DENIED — đúng người nhưng THIẾU `task.extend`: chỉ đọc, kèm câu thiếu quyền — lớp một đóng", () => {
      const html = list(LANH_DAO, false);
      expect(html).not.toContain(NUT_DUYET);
      expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_GIA_HAN));
    });

    it("DENIED — phiên chưa đọc được và nhiệm vụ không ghi lãnh đạo: FAIL CLOSED, `\"\" === \"\"` không mở nút", () => {
      expect(list("", true)).not.toContain(NUT_DUYET);
      const html = list("", true, "");
      expect(html).not.toContain(NUT_DUYET);
      expect(html).toContain(nhuTrongHTML(CAU_KHONG_AI_DUYET_DUOC));
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

    it("đọc HỎNG: câu máy chủ nguyên văn, KHÔNG nói `không có đề nghị nào`, không vẽ form gửi", () => {
      const cau = "không đủ quyền: thiếu task.read";
      const html = extensionView({ load: { phase: "error", message: cau } });
      expect(html).toContain(`role="alert">${nhuTrongHTML(cau)}</p>`);
      expect(html).not.toContain(nhuTrongHTML(TASK_EXTENSIONS_EMPTY));
      expect(html).not.toContain('id="han-moi-lui-han"');
    });

    it("không có đề nghị + `task.update`: form hỏi `Hạn mới` / `Lý do` + `Gửi đề nghị lùi hạn` — KHÔNG nút quyết định nào", () => {
      const html = extensionView({ load: { phase: "done", rows: [] } });
      expect(html).toContain('id="han-moi-lui-han"');
      expect(html).toContain('type="datetime-local"');
      expect(html).toContain(`placeholder="${EXTENSION_REASON_PLACEHOLDER}"`);
      expect(html).toContain(`${EXTENSION_SEND}</button>`);
      expect(html).toContain(nhuTrongHTML(GHI_CHU_LUI_HAN));
      expect(html).not.toContain(NUT_DUYET);
      expect(html).not.toContain(">Từ chối<");
    });

    it("ô đề nghị lùi hạn LUÔN hiện với người đang làm việc — KHÔNG bị gắn sau khoá duyệt", () => {
      // VẾ CHỊU LỰC: "merging the two halves" would take asking away from every non-leader.
      for (const [session, canApprove] of [
        [LANH_DAO, true],
        [NGUOI_KHAC, false],
        ["", false],
      ] as const) {
        const html = extensionView({
          load: { phase: "done", rows: [] },
          sessionStaffCode: session,
          canApproveExtension: canApprove,
        });
        expect(html).toContain('id="han-moi-lui-han"');
      }
    });

    it("DENIED — không `task.update` và không đề nghị nào: không vẽ gì cả (không form rỗng)", () => {
      expect(extensionView({ load: { phase: "done", rows: [] }, canRequest: false })).toBe("");
      expect(extensionView({ load: { phase: "loading" }, canRequest: false })).toBe("");
    });

    it("nhiệm vụ KHÔNG CÓ HẠN: không form — một câu nói vì sao", () => {
      const html = extensionView({ load: { phase: "done", rows: [] }, dueAt: null });
      expect(html).not.toContain('id="han-moi-lui-han"');
      expect(html).toContain(nhuTrongHTML(EXTENSION_NO_DUE));
    });

    it("`extensionBlockNote`: thiếu `task.extend` chỉ đáng nói khi CÓ đề nghị", () => {
      const thieu = quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, false);
      expect(extensionBlockNote(thieu, false)).toBeNull();
      expect(extensionBlockNote(thieu, true)).toBe(CAU_THIEU_QUYEN_DUYET_GIA_HAN);
      expect(extensionBlockNote(quyetDinhDuyetLuiHan(LANH_DAO, LANH_DAO, true), true)).toBeNull();
    });
  });

  it("nhiệm vụ KHÔNG CÓ HẠN: drawer không vẽ ô đề nghị lùi hạn — không có gì để lùi", () => {
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
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 07 §2): `Chờ duyệt` is drawn only while current; the move
    // into it stays on the Kanban (menu and drop) — `kanban-move.test.tsx`.
    expect(html).not.toContain("Chuyển sang Chờ duyệt");
    expect(html).toContain("Chuyển sang Tạm dừng");
    expect(html).not.toContain("Chuyển sang Chuyển tiếp");
    expect(html).toContain("Chuyển sang Hoàn thành");
  });

  it("`hoan-thanh`: không nút thường nào — lối ra duy nhất là ô mở lại có lý do", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: bài này ghim `hoan-thanh` là ngõ cụt ("không có lối ra"). Máy
    // chủ nay liệt kê bước mở lại (`nhiem_vu.go:104`); nó đi qua ô lý do, không qua hàng nút.
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (prototype pipeline): the reopen is the `Đang thực hiện` chip,
    // named for the act; its reason box opens on the press (DOM case in `task-detail-dialog.test.tsx`).
    const html = veChiTiet({ status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" });
    expect(html).not.toContain("Chuyển sang");
    expect(html).not.toContain("không liệt kê lối ra nào");
    expect(html).toContain(`aria-label="${reasonMoveName("reopen", "Đang thực hiện")}"`);
  });

  it("bước `hoan-thanh` VẪN HIỆN dù có thể còn việc con — máy chủ mới là nơi liệt kê mã", () => {
    // Màn hình KHÔNG biết nhiệm vụ có việc con hay không (phản hồi không mang số ấy). Ẩn nút đi
    // "cho chắc" là lấy mất đúng câu từ chối mang danh sách mã mà cán bộ cần đọc.
    const html = veChiTiet({ status: "cho-duyet" });
    expect(html).toContain("Chuyển sang Hoàn thành");
  });
});

describe("câu từ chối của máy chủ vẽ THẲNG, không nuốt thành 'có lỗi xảy ra'", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner #12): a refused status move is the ERROR TOAST with the
  // server's sentence verbatim — the child-code list and the `parent_completed` sentence included.
  // The drawer no longer carries an error line (`loiGhi`), and `Việc cha` left the drawer (owner #6),
  // so the `Mở việc cha` button beside the 409 is gone with it. The DOM case (`task-detail-dialog`)
  // pins `toast.error(<sentence>)`; here the source is read so no wrapper can reword it.
  it("a refused status move: `toast.error(r.thongBao)` — verbatim, never a generic sentence", () => {
    const src = readFileSync(fileURLToPath(new URL("./task-status-pipeline.tsx", import.meta.url)), "utf8");
    expect(src).toContain("toast.error(r.thongBao);");
    expect(src).not.toContain("Có lỗi xảy ra");
  });

  it("xoá: một nút ở ĐẦU hộp chi tiết mở hộp xác nhận — ô lý do chưa vẽ khi chưa bấm", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner #7): no rail; the header's `Xoá` opens the existing confirm. The mandatory reason and the disabled button are pinned in a DOM case
    // (`task-detail-dialog.test.tsx`), where the dialog can be opened.
    const html = veChiTiet();
    expect(html).toMatch(new RegExp(`<button[^>]*aria-label="${TASK_DELETE_BUTTON}"[^>]*aria-haspopup="dialog"`));
    expect(html).not.toContain('id="ly-do-xoa-nhiem-vu"');
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
    // Spec 03 §2: the whole Hạn cell is red on a late row, the days said in words beside the date.
    expect(html).toMatch(/<td class="[^"]*text-danger font-semibold">20\/6\/2026 \(trễ 86 ngày\)<\/td>/);
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
    // The `Thời hạn` card (prototype) gives both with their time — the time is part of a deadline.
    expect(html).toContain(nhuTrongHTML(nhanThoiDiem("2026-06-20T23:59:59+07:00")));
    expect(html).toContain(nhuTrongHTML(nhanThoiDiem("2026-08-30T23:59:59+07:00")));
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
  it("`Tự sinh mã` MẶC ĐỊNH BẬT; ô mã đứng cùng hàng nhưng KHOÁ và RỖNG khi đang tự sinh", () => {
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
    // 06/10/2026 (prototype): the code box sits beside the tick, DISABLED and empty while the server
    // numbers the task — nothing typed can be sent then (`thanGiaoViec` drops it with `tuSinhMa`).
    const codeBox = /<input id="giao-ma"[^>]*>/.exec(html)?.[0] ?? "";
    expect(codeBox).toContain('disabled=""');
    expect(codeBox).toContain('placeholder="Hệ thống sẽ tự sinh"');
    expect(codeBox).not.toMatch(/value="[^"]+"/);
    expect(html.indexOf('id="giao-ma"')).toBeLessThan(html.indexOf('id="giao-tu-sinh-ma"'));
  });

  it("07/10: ô hạn KHÔNG có dòng chú thích nào (prototype không có) — và không câu 'chỉ đặt một lần'", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026: ca này từng ghim câu "Bỏ trống thì nhiệm vụ chưa có hạn…" và
    // "Điền sẵn 7 ngày nữa…" dưới ô hạn. Người dùng: theo prototype, không thêm gì — prototype không
    // có dòng nào dưới [Hạn | Mức ưu tiên]. Câu cũ "không đặt được về sau" vẫn sai (ADR 0065 NV4).
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
    expect(html).not.toContain("Bỏ trống thì nhiệm vụ chưa có hạn");
    expect(html).not.toContain("Điền sẵn");
    expect(html).not.toContain("Việc con có hạn riêng");
    // The row's next sibling is not a help paragraph.
    expect(html).not.toMatch(/id="giao-uu-tien"[\s\S]*?<\/select><\/div><\/div><p class="ghi-chu/);
    expect(html).not.toContain("chỉ đặt được một lần");
    expect(html).not.toContain("không đặt được về sau");
  });

  it("NV-01: xã KHÔNG có loại mặc định — ô loại đứng ở `— Chọn loại —`; nút KHÔNG khoá, chưa đỏ gì trước lần bấm", () => {
    // Trước: không có dòng trống, trình duyệt hiện loại ĐẦU TIÊN trong khi giá trị là "" — nút
    // `Giao việc` mờ mà không nói vì sao (báo cáo kiểm thử 05/10/2026).
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026: ca này ghim "nút khoá KÈM lý do" (hộp hồng trên nút). Người dùng:
    // bấm thì báo lỗi DƯỚI Ô và đưa tiêu điểm tới ô thiếu, như prototype — xem
    // `create-task-form.interaction.test.tsx`. Ở đây chỉ còn trạng thái TRƯỚC lần bấm.
    const typeRow = (code: string, label: string) => ({
      id: `01J${code}`,
      code,
      label,
      is_default: false,
      active: true,
      order: 1,
      source: "he-thong",
      tier: 1,
      requires_directive: code === "theo-van-ban",
    });
    const noDefaultType: DanhMucNhiemVu = {
      ...DANH_MUC,
      loai: [typeRow("theo-van-ban", "Theo văn bản"), typeRow("co-ban", "Nhiệm vụ cơ bản")],
    };
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={noDefaultType}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
        tieuDeCoSan="Tên việc đã điền"
      />,
    );
    const o = oChon(html, "giao-loai");
    expect(o).toContain(`<option value="" selected="">${TASK_TYPE_PLACEHOLDER}</option>`);
    // Không tự chọn dòng đầu thay cán bộ — mặc định là lựa chọn của danh mục xã.
    expect(o).not.toMatch(/<option value="theo-van-ban" selected="">/);
    expect(o).not.toContain("aria-invalid");
    expect(o).not.toContain("aria-describedby");
    expect(html).not.toContain(TASK_TYPE_MISSING);
    expect(html).toMatch(/<button type="submit" class="nut-chinh">Giao việc<\/button>/);
  });

  it("07/10: loại mặc định đã NGỪNG dùng thì không chọn sẵn — `— Chọn loại —`", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={{
          ...DANH_MUC,
          loai: [
            { ...DANH_MUC.loai[0]!, active: false },
            { ...DANH_MUC_CO_BAN.loai[0]!, is_default: false },
          ],
        }}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    const o = oChon(html, "giao-loai");
    expect(o).toContain(`<option value="" selected="">${TASK_TYPE_PLACEHOLDER}</option>`);
    expect(o).not.toMatch(/value="(theo-van-ban|co-ban)" selected=""/);
  });

  it("07/10: loại mặc định `theo-van-ban` → form mở ra đã có `Văn bản sản phẩm đầu ra` và `Ghi chú`", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={{ ...DANH_MUC, loai: [{ ...DANH_MUC_CO_BAN.loai[0]!, is_default: false }, DANH_MUC.loai[0]!] }}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        coDanhSachVanBan
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(oChon(html, "giao-loai")).toContain('<option value="theo-van-ban" selected="">');
    expect(html).toContain('id="giao-nhom-van-ban-san-pham-dau-ra"');
    expect(html).toMatch(/<label for="giao-ghi-chu"[^>]*>Ghi chú<\/label>/);
  });

  it("NV-01: xã CÓ loại mặc định — không dòng trống, không câu lý do", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
        tieuDeCoSan="Tên việc đã điền"
      />,
    );
    const o = oChon(html, "giao-loai");
    expect(o).toContain('<option value="theo-van-ban" selected="">');
    expect(o).not.toContain(TASK_TYPE_PLACEHOLDER);
    expect(html).not.toContain(TASK_TYPE_MISSING);
  });

  it("hạn ĐIỀN SẴN +7 ngày lúc 17:00 (ADR 0065 NV6) trong MỘT ô datetime-local, không ô `Giờ` riêng", () => {
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
      expect(html).toMatch(/<input id="giao-han"[^>]*type="datetime-local"[^>]*value="2026-10-07T17:00"/);
      expect(html).not.toContain('id="giao-han-gio"');
      expect(html).not.toContain('type="time"');
      expect(html).not.toContain(">Giờ</label>");
      expect(html).not.toContain("23:59");
      // Prototype row: [Hạn hoàn thành | Mức ưu tiên] in two halves.
      expect(html).toMatch(/<div class="grid grid-cols-1 gap-3 sm:grid-cols-2"><div><label for="giao-han"[^>]*>Hạn hoàn thành<\/label>/);
    } finally {
      vi.useRealTimers();
    }
  });

  it("ô `Lãnh đạo giao việc` nói ra hệ quả ADR 0038 của việc bỏ trống — và không hứa chuông/thư", () => {
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
    // 07/10: the prototype's "qua chuông và qua thư" is NOT drawn — filing an extension request sends
    // no notice (`DeNghiLuiHan`, service-petitions/internal/app/nhiem_vu.go).
    expect(GHI_CHU_LANH_DAO_GIAO_VIEC).toBe("Đề nghị lùi hạn sẽ gửi tới người này.");
    expect(html).toContain(nhuTrongHTML(GHI_CHU_LANH_DAO_GIAO_VIEC));
    expect(html).not.toContain("chuông");
    expect(html).not.toContain("qua thư");
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
    expect(html.split("Thêm văn bản</button>").length - 1).toBe(3);
    expect(html).toContain('id="giao-them-van-ban-cap-tren-giao"');
    expect(html).toContain("Nội dung nhiệm vụ / Trích yếu văn bản");
    // ADR 0065 NV5: no SEPARATE lead unit / monitor fields — they ARE the unit and the assignee. Since
    // 06/10/2026 (prototype) a `Theo văn bản` task shows those two fields under the tracking book's
    // names; same ids, same values sent.
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
    expect(html).toMatch(/<label for="giao-bo-phan"[^>]*>Cơ quan chủ trì tham mưu \(cơ quan thực hiện\)<\/label>/);
    expect(html).toContain("Chuyên viên tham mưu / theo dõi (người thực hiện)");
    // The prototype's order: upper + party documents BEFORE the deadline, output AFTER it.
    const at = (s: string) => html.indexOf(s);
    expect(at('id="giao-them-van-ban-chi-dao-dang-uy"')).toBeLessThan(at('id="giao-han"'));
    expect(at('id="giao-han"')).toBeLessThan(at('id="giao-them-van-ban-san-pham-dau-ra"'));
    // ĐỔI CHIỀU CÓ CHỦ Ý 27/09/2026 (TASK-04): `POST /api/v1/tasks` nay nhận `note`, nên ô
    // `Ghi chú` §7.2 có mặt — có nhãn, sau ba danh sách, dừng ở cùng giới hạn với form `✎ Sửa`.
    expect(html).toMatch(/<label for="giao-ghi-chu"[^>]*>Ghi chú<\/label>/);
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
    // 07/10/2026: the prototype's required mark follows the words (`Field required`).
    expect(html).toContain('>Tên nhiệm vụ<span class="text-danger ml-1" aria-hidden="true">*</span></label>');
    expect(html).not.toContain("Trích yếu văn bản");
    expect(html).not.toContain('id="giao-co-quan-chu-tri"');
    expect(html).not.toContain('id="giao-chuyen-vien"');
    expect(html).not.toContain("Thêm văn bản");
    expect(html).not.toContain("Văn bản cấp trên giao");
    expect(html).not.toContain('id="giao-ghi-chu"');
  });

  it("`dialog` (06/10/2026, prototype): a named modal, 500px; the prototype's field order", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        dialog
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
    expect(html).toMatch(/^<dialog aria-labelledby="tieu-de-giao-viec-moi" aria-modal="true" class="[^"]*max-w-\[500px\]/);
    expect(html).toContain('<h2 id="tieu-de-giao-viec-moi"');
    const at = (s: string) => html.indexOf(s);
    const order = [
      'id="giao-loai"',
      'id="giao-khoi"',
      'id="giao-ma"',
      'id="giao-tu-sinh-ma"',
      'id="giao-tieu-de"',
      'id="giao-mo-ta"',
      'id="giao-bo-phan"',
      'id="giao-nguoi-thuc-hien"',
      'id="giao-lanh-dao"',
      'id="giao-han"',
      'id="giao-uu-tien"',
      ">Huỷ</button>",
      ">Giao việc</button>",
    ];
    for (let i = 1; i < order.length; i++) expect(at(order[i - 1]!)).toBeLessThan(at(order[i]!));
    expect(html).toMatch(/<label for="giao-bo-phan"[^>]*>Đơn vị thực hiện<\/label>/);
  });

  it("`dialog` + `Theo văn bản`: the wide 800px box (three document lists)", () => {
    const html = renderToStaticMarkup(
      <FormGiaoViec
        dialog
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
    expect(html).toMatch(/^<dialog [^>]*class="[^"]*max-w-\[800px\]/);
  });

  // THE NHIỆM VỤ SCREEN'S DIALOG (`taskScreen`, spec 06, owner 07/10/2026 #2 #9). Each assertion names
  // the spec rule it follows; Biên bản / Phản ánh (no `taskScreen`) keep their rules — cases above.
  const dialogMarkup = (danhMuc: DanhMucNhiemVu) =>
    renderToStaticMarkup(
      <FormGiaoViec
        dialog
        taskScreen
        danhMuc={danhMuc}
        danhBa={DANH_BA}
        danhBaLanhDao={DANH_BA}
        coDanhSachVanBan
        staffSearch
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
  const dialogClass = (html: string) => /^<dialog [^>]*class="([^"]*)"/.exec(html)?.[1]?.split(" ") ?? [];
  const typeRow = (code: string, label: string, is_default: boolean, active = true, order = 1) => ({
    id: `01J${code}`,
    code,
    label,
    is_default,
    active,
    order,
    source: "he-thong",
    tier: 1,
    requires_directive: code === "theo-van-ban",
  });

  it("owner #9: 500px for a basic task, 800px for `Theo văn bản` — the width follows the TYPE", () => {
    expect(dialogClass(dialogMarkup(DANH_MUC_CO_BAN))).toContain("max-w-[500px]");
    expect(dialogClass(dialogMarkup(DANH_MUC_CO_BAN))).not.toContain("max-w-[800px]");
    expect(dialogClass(dialogMarkup(DANH_MUC))).toContain("max-w-[800px]");
    expect(dialogClass(dialogMarkup(DANH_MUC))).not.toContain("max-w-[500px]");
  });

  it("the dialog keeps clear of the top edge and pads like the prototype (p-4); `m-auto!` centres it", () => {
    for (const danhMuc of [DANH_MUC_CO_BAN, DANH_MUC]) {
      const cls = dialogClass(dialogMarkup(danhMuc));
      expect(cls).toContain("w-[calc(100vw-2rem)]");
      // The variant REPLACES `ModalDialog`'s defaults (cn → last utility wins), it does not stack.
      expect(cls).toContain("max-h-[calc(100dvh-6rem)]");
      expect(cls).not.toContain("max-h-[90dvh]");
      expect(cls).toContain("p-4");
      expect(cls).not.toContain("p-6");
      expect(cls).toContain("m-auto!");
      expect(cls.some((c) => /^(top|mt|my|inset|translate)-/.test(c))).toBe(false);
    }
    expect(CREATE_TASK_DIALOG_CLASS).toBe("max-h-[calc(100dvh-6rem)] p-4");
  });

  it("owner #2: the type starts on `is_default`, else on the FIRST active row — never `— Chọn loại —`", () => {
    const withDefault = dialogMarkup({
      ...DANH_MUC,
      loai: [typeRow("co-ban", "Nhiệm vụ cơ bản", false), typeRow("theo-van-ban", "Theo văn bản", true, true, 2)],
    });
    expect(oChon(withDefault, "giao-loai")).toContain('<option value="theo-van-ban" selected="">');
    const noDefault = dialogMarkup({
      ...DANH_MUC,
      loai: [typeRow("co-ban", "Nhiệm vụ cơ bản", false), typeRow("theo-van-ban", "Theo văn bản", false, true, 2)],
    });
    expect(oChon(noDefault, "giao-loai")).toContain('<option value="co-ban" selected="">');
    expect(noDefault).not.toContain(TASK_TYPE_PLACEHOLDER);
    // A retired default is not chosen, and a retired row is not offered at all.
    const retired = dialogMarkup({
      ...DANH_MUC,
      loai: [typeRow("theo-van-ban", "Theo văn bản", true, false), typeRow("co-ban", "Nhiệm vụ cơ bản", false, true, 2)],
    });
    expect(oChon(retired, "giao-loai")).toContain('<option value="co-ban" selected="">');
    expect(oChon(retired, "giao-loai")).not.toContain('value="theo-van-ban"');
  });

  it("spec 06 §5: a BASIC task has no unit / assignee fields (assigned in the detail); `Theo văn bản` has both", () => {
    const basic = dialogMarkup(DANH_MUC_CO_BAN);
    expect(basic).not.toContain('id="giao-bo-phan"');
    expect(basic).not.toContain('id="giao-nguoi-thuc-hien"');
    expect(basic).toContain('id="giao-lanh-dao"');
    const vb = dialogMarkup(DANH_MUC);
    expect(vb).toContain('id="giao-bo-phan"');
    expect(vb).toContain('id="giao-nguoi-thuc-hien"');
  });

  it("owner #2: `Lãnh đạo giao việc` empty line and hint are the spec's words, verbatim", () => {
    const html = dialogMarkup(DANH_MUC_CO_BAN);
    expect(LEADER_EMPTY_LABEL).toBe("— Người đang tạo nhiệm vụ —");
    expect(LEADER_HINT_SPEC).toBe("Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư.");
    expect(html).toContain(nhuTrongHTML(LEADER_HINT_SPEC));
    expect(comboboxListbox(html, "giao-lanh-dao")).toContain(nhuTrongHTML(LEADER_EMPTY_LABEL));
  });

  it("spec 06: `Mức ưu tiên` has no empty choice — the commune's default row, else its first", () => {
    const priority = (is_default: boolean) =>
      oChon(
        dialogMarkup({
          ...DANH_MUC_CO_BAN,
          mucUuTien: [
            { ...DANH_MUC.mucUuTien[0]!, code: "khan", label: "Khẩn", is_default: false },
            { ...DANH_MUC.mucUuTien[0]!, code: "thuong", label: "Thường", is_default },
          ],
        }),
        "giao-uu-tien",
      );
    expect(priority(true)).toContain('<option value="thuong" selected="">');
    expect(priority(false)).toContain('<option value="khan" selected="">');
    expect(priority(false)).not.toContain('<option value="">');
  });

  it("the page mounts the create dialog under `[&>*]:my-0` — the reason `m-auto!` exists", () => {
    const page = readFileSync(new URL("./so-nhiem-vu.tsx", import.meta.url), "utf8");
    const section = page.indexOf('className="man-nhiem-vu mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0"');
    expect(section).toBeGreaterThan(-1);
    expect(page.indexOf("<FormGiaoViec\n          dialog\n          taskScreen", section)).toBeGreaterThan(section);
    const modal = readFileSync(new URL("../../components/ui/modal-dialog.tsx", import.meta.url), "utf8");
    expect(modal).toContain('"m-auto! box-border');
  });

  it("controls: 16px below 768px (iOS zoom), 14px from 768px — one input class for every box", () => {
    expect(INPUT_CLASS.split(" ")).toContain("text-base");
    expect(INPUT_CLASS.split(" ")).toContain("md:text-sm");
    expect(INPUT_CLASS.split(" ")).toContain("h-9");
    expect(LABEL_CLASS).toBe("text-ink mb-1.5 block text-[13px] font-semibold");
    const html = dialogMarkup(DANH_MUC);
    expect(html).toContain(`<label for="giao-tieu-de" class="${LABEL_CLASS}">`);
  });

  it("staff boxes (spec 06 `UserCombobox`): ONE input with the ⇕ icon inside, the spec placeholder, a listbox", () => {
    const html = dialogMarkup(DANH_MUC);
    for (const id of ["giao-nguoi-thuc-hien", "giao-lanh-dao"]) {
      const input = /<input [^>]*>/.exec(html.slice(html.indexOf(`<input id="${id}"`)))?.[0] ?? "";
      expect(input).toContain('role="combobox"');
      expect(input).toContain(`aria-controls="${id}-danh-sach"`);
      expect(input).toContain(`placeholder="${PERSON_PICKER_PLACEHOLDER}"`);
      expect(html).toContain(`<ul id="${id}-danh-sach" role="listbox"`);
    }
    expect(PERSON_PICKER_PLACEHOLDER).toBe("Gõ tên để tìm…");
    expect(html).not.toContain('class="nut-phu nut-mo-danh-sach"');
    expect(html).not.toContain(">▾<");
    expect(html).toContain("lucide-chevrons-up-down");
  });

  it("the fields AND the buttons scroll — capped at the prototype's 70vh, header outside", () => {
    const html = dialogMarkup(DANH_MUC);
    const body = html.indexOf(`<div class="${CREATE_TASK_BODY_CLASS.replaceAll("&", "&amp;").replaceAll(">", "&gt;")}">`);
    expect(body).toBeGreaterThan(-1);
    expect(CREATE_TASK_BODY_CLASS).toContain("max-h-[70vh]");
    expect(CREATE_TASK_BODY_CLASS).toContain("overflow-y-auto");
    expect(html.indexOf('<h2 id="tieu-de-giao-viec-moi"')).toBeLessThan(body);
    // `Huỷ / Giao việc` are the LAST child of the scrolling area, after `Ghi chú`.
    expect(html.lastIndexOf(">Giao việc</button>")).toBeGreaterThan(html.lastIndexOf('id="giao-ghi-chu"'));
    expect(html).toMatch(/<\/textarea><\/div><\/div><div class="flex justify-end gap-2 pt-1">/);
    // After the form, only the dialog's own ✕ (ADR 0068 lần 6, prototype `dialog.tsx:83-95`).
    expect(html).toMatch(/>Giao việc<\/button><\/div><\/div><\/form><button [^>]*aria-label="Đóng"[^>]*>[\s\S]*?<\/button><\/dialog>$/);
  });

  it("`Theo văn bản` — the prototype's full field order, title marked required", () => {
    const html = dialogMarkup(DANH_MUC);
    const at = (s: string) => html.indexOf(s);
    const order = [
      'id="giao-loai"',
      'id="giao-khoi"',
      'id="giao-ma"',
      'id="giao-tu-sinh-ma"',
      '>Nội dung nhiệm vụ / Trích yếu văn bản<span class="text-danger ml-1" aria-hidden="true">*</span></label>',
      'id="giao-mo-ta"',
      '>Cơ quan chủ trì tham mưu (cơ quan thực hiện)</label>',
      'id="giao-nguoi-thuc-hien"',
      'id="giao-lanh-dao"',
      'id="giao-them-van-ban-cap-tren-giao"',
      'id="giao-them-van-ban-chi-dao-dang-uy"',
      'id="giao-han"',
      'id="giao-uu-tien"',
      'id="giao-them-van-ban-san-pham-dau-ra"',
      'id="giao-ghi-chu"',
      ">Huỷ</button>",
      ">Giao việc</button>",
    ];
    for (const s of order) expect(at(s), s).toBeGreaterThan(-1);
    for (let i = 1; i < order.length; i++) expect(at(order[i - 1]!), order[i]).toBeLessThan(at(order[i]!));
    // Only the title carries the mark, as in the prototype.
    expect(html.match(/<span class="text-danger ml-1" aria-hidden="true">\*<\/span>/g)).toHaveLength(1);
  });

  it("the screenshots' words, where our field exists (both task types)", () => {
    for (const danhMuc of [DANH_MUC, DANH_MUC_CO_BAN]) {
      const html = dialogMarkup(danhMuc);
      expect(html).toContain(nhuTrongHTML(MO_TA_FORM_GIAO_VIEC));
      expect(MO_TA_FORM_GIAO_VIEC).toBe(
        "Giao cho một bộ phận hoặc trực tiếp cho cán bộ. Giao cho bộ phận mà quá lâu chưa phân công " +
          "người thì hệ thống báo lên lãnh đạo.",
      );
      expect(html).toContain(nhuTrongHTML(GHI_CHU_TU_SINH_MA));
      expect(GHI_CHU_TU_SINH_MA).toBe(
        "Tự sinh sẽ cấp số tiếp theo trong dãy NV01, NV02… Nhập từ Excel cũng được đánh số tự động " +
          "theo dãy này.",
      );
    }
    const vb = dialogMarkup(DANH_MUC);
    expect(vb).toContain('<option value="" selected="">— Chọn cơ quan —</option>');
    expect(vb).toContain(
      nhuTrongHTML("Người này là người thực hiện chính: nhiệm vụ hiện trong mục “Giao cho tôi” của họ ngay khi lưu."),
    );
  });

  it("the submit is the spec's primary button; the directory still loading locks it (and only that)", () => {
    const ready = dialogMarkup(DANH_MUC);
    expect(ready).toMatch(/<button class="nut-chinh[^"]*" type="submit">Giao việc<\/button>/);
    const loading = renderToStaticMarkup(
      <FormGiaoViec dialog taskScreen danhMuc={DANH_MUC} danhBa={null} danhBaLanhDao={null} coDanhSachVanBan staffSearch dangGui={false} loi={null} huy={() => {}} giaoViec={() => {}} />,
    );
    expect(loading).toMatch(/<button class="nut-chinh[^"]*" type="submit" disabled="">Giao việc<\/button>/);
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

/** One document line of the read-only block: reference in bold, then ` · date`. */
function docLine(reference: string, date: string): string {
  return `<span class="font-semibold">${reference}</span><span class="text-ink-muted"> · ${date}</span>`;
}

/** An EMPTY group of the read-only block — its label, then a dash. */
function emptyGroup(label: string): string {
  return `${nhuTrongHTML(label)}</p><div class="m-0 text-[13px]">—</div>`;
}

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
    expect(html).toContain(docLine("1742-CV/BTCTU", "9/6/2026"));
    expect(html).toContain(nhuTrongHTML("Công văn của Ban Tổ chức Thành uỷ"));
    expect(html).toContain(docLine("324-BC/ĐU", "15/6/2026"));
    // Nhóm giữa rỗng thật — máy chủ ĐÃ nói thế — nên nó là dấu gạch.
    expect(html).toContain(emptyGroup("Văn bản chỉ đạo của Đảng uỷ"));
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
    expect(html).toContain(docLine("1742-CV/BTCTU", "9/6/2026"));
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
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 07 §6a): a basic task with no result, note or tick shows
    // `Thông tin nhiệm vụ` — the two ticks and their note belong to the directive block.
    expect(html).not.toContain(nhuTrongHTML(CHU_THICH_HAI_O_TICK));
  });

  it("loại `theo-van-ban` với cùng dữ liệu: khối CÓ — bài trên không xanh vì lý do sai", () => {
    expect(veChiTiet({}, LANH_DAO, XONG)).toContain("1742-CV/BTCTU");
  });

  it("số ký hiệu rỗng ⇒ `Không số`; ngày rỗng ⇒ bỏ hẳn phần ngày", () => {
    const html = veChiTiet({}, LANH_DAO, {
      pha: "xong",
      duLieu: [vb({ reference: "", date: "" })],
    });
    expect(html).toContain(`<span class="font-semibold">${KHONG_SO}</span><div`);
    expect(html).not.toContain(`${KHONG_SO}</span><span class="text-ink-muted"> · `);
  });

  it("chi tiết trả mảng RỖNG: lúc này ba nhóm `—` là đúng — máy chủ đã nói không có dòng nào", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: [] });
    for (const nhan of NHAN_BA_NHOM) {
      expect(html).toContain(emptyGroup(nhan));
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
    expect(nut).not.toContain('disabled=""');
    // ADR 0068: `✎` is a lucide icon now; the visible word is `Sửa` (NHAN_NUT_SUA, shared with
    // the Nội dung screen, keeps its glyph there). The accessible name is the aria-label above.
    expect(html).toMatch(/aria-label="Sửa sổ theo dõi văn bản chỉ đạo"[^>]*><svg[^>]*aria-hidden="true"[^>]*>.*?<\/svg>Sửa<\/button>/);
  });

  it("loại `co-ban`: KHÔNG có nút sửa khối văn bản, kể cả khi đã có văn bản trong tay", () => {
    // Its `✎ Sửa` is the information block's (title + deadline), never the document block's.
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN });
    expect(theNutSua(html)).toBeNull();
    expect(html).not.toContain(NHAN_NUT_SUA);
    expect(html).not.toContain("aria-label=\"Sửa sổ theo dõi văn bản chỉ đạo\"");
    expect(html).toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
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
    expect(html).toContain(docLine("324-BC/ĐU", "15/6/2026"));
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
        unitName="VĂN PHÒNG ĐẢNG ỦY"
        assigneeName="Nguyễn Thị Thực"
        luu={KHONG_SUA}
        docLai={KHONG_SUA}
        xong={() => {}}
      />,
    );
  }

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 07 §6a, owner #8 #10): the first row is [Mã | Tên | Hạn] — the
  // code SHOWN, never an input (ADR 0065 NV3); the deadline ONE `datetime-local` box. `Cơ quan chủ trì
  // tham mưu` / `Chuyên viên Văn phòng…` are shown READ-ONLY (the unit and the assignee, changed in
  // `Giao việc, chuyển việc` — ADR 0065 NV5: no separate field is sent).
  it("[Mã (text) | Tên | Hạn (datetime-local)]; unit and assignee shown read-only, never inputs", () => {
    const html = veForm();
    expect(html).not.toContain('id="sua-ma-nhiem-vu"');
    expect(html).not.toContain(">Mã nhiệm vụ</label>");
    expect(html).toMatch(/>Mã nhiệm vụ<\/p><p class="m-0 mt-2 text-\[13px\] font-semibold">NV19<\/p>/);
    expect(html).toMatch(/<input id="sua-han"[^>]*type="datetime-local"[^>]*value="2026-06-20T23:59"/);
    expect(html).toContain(`Cơ quan chủ trì tham mưu</p><div class="m-0 text-[13px]">VĂN PHÒNG ĐẢNG ỦY</div>`);
    expect(html).toContain(`Chuyên viên Văn phòng tham mưu / theo dõi</p><div class="m-0 text-[13px]">Nguyễn Thị Thực</div>`);
    expect(html).not.toContain("<select");
    // One date field per document — the deadline is the datetime box.
    expect(html.split('type="date"').length - 1).toBe(BA_VAN_BAN.length);
  });

  it("việc CHƯA CÓ HẠN: the deadline box is empty — no hour typed in for the clerk", () => {
    const html = veForm({ due_at: null, original_due_at: null });
    expect(html).toMatch(/<input id="sua-han"[^>]*value=""/);
    expect(html).not.toContain("17:00");
  });

  it("dây nối: a refusal re-reads and says the task moved on; `Lưu` with nothing changed just leaves (đọc mã)", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain("const stale = changedSince(await docLai(), goc.nhiemVu.updated_at);");
    expect(src).toContain("toast.error(stale ? `${kq.thongBao} ${TASK_CHANGED_NOTE}` : kq.thongBao);");
    expect(src).toContain("if (than === null) {\n      xong();\n      return;\n    }");
    expect(src).toContain('guiDrawer({ loai: "docLai", ma: code });');
    expect(src).not.toMatch(/dueTime: "1\d:\d\d"/);
  });

  it("the editable fields have real labels; the two ticks are NOT in the form (ticked in view, spec 07 §6a)", () => {
    const html = veForm();
    expect(html).toMatch(/<label for="sua-tieu-de"[^>]*>Nội dung nhiệm vụ \/ Trích yếu văn bản<\/label>/);
    expect(html).toContain('id="sua-tom-tat-ket-qua"');
    expect(html).toMatch(/<label for="sua-ghi-chu"[^>]*>Ghi chú<\/label>/);
    expect(html).not.toContain('id="sua-lanh-dao-phe-duyet"');
    expect(html).not.toContain('id="sua-cap-tren-cong-nhan"');
  });

  it("mọi dòng đã có hiện ra để sửa, dùng CÙNG ô của form tạo; không có lối chuyển nhóm", () => {
    const html = veForm();
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN1"');
    expect(html).toContain('id="sua-van-ban-id-01JVANBAN2"');
    expect(html).toContain("Công văn của Ban Tổ chức Thành uỷ");
    expect(html).toContain('value="1742-CV/BTCTU"');
    expect(html).toContain('value="2026-06-09"');
    expect(html.split("Thêm văn bản</button>").length - 1).toBe(3);
    // Id không đụng form tạo khi hai form cùng mở.
    expect(html).not.toContain('id="giao-');
  });

  it("`Huỷ` · `Lưu` (spec 07 §6a); `Lưu` is not locked by 'nothing changed' — it just leaves", () => {
    const html = veForm();
    expect(html).toMatch(/type="submit">Lưu<\/button>/);
    expect(html).toContain(">Huỷ</button>");
    expect(html).not.toContain("Chưa có gì thay đổi để lưu.");
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
    expect(html).toMatch(/role="status"[^>]*>Đang tải nhật ký…/);
    expect(html).not.toContain(nhuTrongHTML(NHAT_KY_RONG));
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W2): ca này ghim "KHÔNG có ô ghi tay — tuyến ghi chưa dựng". Tuyến
  // nay có (60011e8), nên ô hiện cho người được ghi và VẮNG cho người không được — cả hai chiều dưới.
  it("CÓ ô ghi tay cho người cầm `task.update`: nhãn (ẩn), gợi ý trong ô, nút khoá khi trống, và `Đính kèm`", () => {
    const html = veChiTiet();
    // Spec 08: no visible title over the box — the accessible name stays.
    expect(html).toContain('<label for="ghi-nhat-ky-NV19" class="an-thi-giac">Ghi vào nhật ký của nhiệm vụ</label>');
    expect(html).toContain('placeholder="Đã làm được gì, còn vướng gì…"');
    expect(html).toMatch(/<textarea id="ghi-nhat-ky-NV19"[^>]*maxLength="5000"/);
    expect(html).toMatch(/type="submit" disabled=""><svg[^>]*lucide-send[^>]*>.*?<\/svg>Ghi nhật ký<\/button>/);
    // `Đính kèm` is a button that opens the hidden native file input.
    expect(html).toMatch(/<svg[^>]*lucide-paperclip[^>]*>.*?<\/svg>Đính kèm<\/button>/);
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
    // Outcomes are toasts (spec 08): the refusal returns BEFORE the key is replaced.
    const refused = src.indexOf("toast.error(r.thongBao);");
    expect(refused).toBeGreaterThan(-1);
    expect(src.indexOf("setKey(crypto.randomUUID());")).toBeGreaterThan(refused);
    expect(src.indexOf("toast.success(LOG_ENTRY_DONE);")).toBeGreaterThan(src.indexOf("setKey(crypto.randomUUID());"));
  });

  it("rỗng: câu §5.9 nguyên văn, không danh sách, không `Xem thêm`", () => {
    const html = veNhatKy({ pha: "xong", dong: [], conNua: false });
    expect(html).toContain(nhuTrongHTML(NHAT_KY_RONG));
    expect(html).not.toContain("<ol");
    expect(html).not.toContain("Xem thêm");
  });

  it("có dòng: thời điểm, HỌ TÊN (prototype — không kèm mã), nhãn trạng thái, bàn giao, ghi chú", () => {
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
    expect(html).toContain('<b class="text-navy text-[12.5px]">Trần Thị B</b>');
    expect(html).not.toContain("Trần Thị B (CB-2026-3H8N2W)");
    // Người không có trong danh bạ: chỉ mã.
    expect(html).toContain(">CB-00007</b>");
    // Spec 08: a status pill only on the row that CHANGED it (vs. the older row loaded); the first
    // row ever (creation) gets none.
    expect(html).toContain(nhuTrongHTML("Chuyển sang “Đang thực hiện”"));
    expect(html).not.toContain(nhuTrongHTML("Chuyển sang “Mới giao”"));
    // The hand-over box: no OLDER assignment row loaded → the new holder only, NO arrow before it.
    expect(html).toContain('Bộ phận: </span><b class="text-navy font-semibold">VĂN PHÒNG ĐẢNG ỦY</b>');
    expect(html).toContain('Người thực hiện: </span><b class="text-navy font-semibold">Trần Thị B</b>');
    expect(html).not.toContain(": → ");
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
    expect(html).toContain(">Nguyễn Thị Thực</td>");
    expect(html).not.toContain(`>${NGUOI_KHAC}</td>`);
  });

  it("mã KHÔNG có trong danh bạ: hiện MÃ, không bao giờ để trống", () => {
    // VẾ CHỊU LỰC. Một ô trống đọc ra là "chưa giao cho ai" — với đúng người đã làm việc ấy.
    const html = veBang(NGUOI_DA_NGHI, DANH_BA_MA);
    expect(html).toContain(`>${NGUOI_DA_NGHI}</td>`);
    expect(html).not.toMatch(/<td[^>]*><\/td>/);
  });

  it("danh bạ chưa về (hoặc hỏng): hiện MÃ; chưa phân công vẫn là `Chưa phân công`", () => {
    expect(veBang(NGUOI_KHAC, null)).toContain(`>${NGUOI_KHAC}</td>`);
    expect(veBang("", DANH_BA_MA)).toContain(nhuTrongHTML(CHUA_PHAN_CONG));
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (main session, prototype `TaskDetailDrawer.tsx:407-417`): the name
  // alone; a code absent from the directory is still shown as the code.
  it("drawer: người thực hiện, lãnh đạo giao việc — HỌ TÊN, mã lạ là mã", () => {
    const html = veChiTiet(
      { assignee: NGUOI_KHAC, assigner: NGUOI_DA_NGHI },
      LANH_DAO,
      { pha: "dangTai" },
      DU_QUYEN,
      DANH_BA,
    );
    expect(html).toContain(">Nguyễn Thị Thực</p>");
    expect(html).not.toContain(`Nguyễn Thị Thực (${NGUOI_KHAC})`);
    // The assigner sits under the assignee in the fact row (prototype, 06/10/2026).
    expect(html).toContain(`Lãnh đạo giao việc: ${NGUOI_DA_NGHI}</p>`);
    const khac = veChiTiet({ assigner: LANH_DAO }, LANH_DAO, { pha: "dangTai" }, DU_QUYEN, DANH_BA);
    expect(khac).toContain(`Lãnh đạo giao việc: Trần Văn Lãnh</p>`);
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner #8, ADR 0076 lần 2): the directive block shows `Cơ quan chủ
  // trì tham mưu` and `Chuyên viên Văn phòng tham mưu / theo dõi` again — as DISPLAY of the same unit and
  // assignee (ADR 0065 NV5 still holds: no separate fields, nothing separate is sent).
  it("drawer: `Cơ quan chủ trì tham mưu` / `Chuyên viên Văn phòng…` show the SAME unit and assignee", () => {
    const html = veChiTiet({ assignee: NGUOI_KHAC }, LANH_DAO, { pha: "dangTai" }, DU_QUYEN, DANH_BA);
    expect(html).toContain(`Cơ quan chủ trì tham mưu</p><div class="m-0 text-[13px]">VĂN PHÒNG ĐẢNG ỦY</div>`);
    expect(html).toContain(
      // The short name, as the register's column (the code only when the directory lacks the person).
      `Chuyên viên Văn phòng tham mưu / theo dõi</p><div class="m-0 text-[13px]">Nguyễn Thị Thực</div>`,
    );
    // A basic task with nothing in the directive block does not show them.
    const basic = veChiTiet({ type: "co-ban", assignee: NGUOI_KHAC }, LANH_DAO, { pha: "dangTai" }, DU_QUYEN, DANH_BA);
    expect(basic).not.toContain("Chuyên viên Văn phòng");
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
    // A class, never an inline value (spec 03 §2: `bg-danger/6`).
    expect(html).toMatch(/<tr data-tre-han="" class="[^"]*\bbg-danger\/6\b[^"]*">/);
    expect(html).not.toContain("style=");
    expect(html).toContain("(trễ 86 ngày)");
  });

  it("chưa tới hạn, và không có hạn: không tô", () => {
    for (const sua of [
      { due_at: "2026-12-20T23:59:59+07:00" },
      { due_at: null, original_due_at: null },
    ]) {
      const html = veMotDong(sua);
      expect(html).not.toContain("bg-danger/6");
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

  it("`cho-duyet` + `task.approve`: chip `Đang thực hiện` mang tên hành vi TRẢ LẠI, không phải bước thường", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (prototype pipeline): the reason field, its label, `required` and
    // the disabled `Xác nhận` appear on the press — pinned in a DOM case (`task-detail-dialog.test.tsx`).
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_KHAC, { pha: "dangTai" }, Q_DUYET);
    expect(html).toContain(NHAN_NUT);
    expect(html).toContain(`aria-label="${reasonMoveName("return", "Đang thực hiện")}"`);
    // KHÔNG có nút thường `Chuyển sang Đang thực hiện` — cú bấm ấy sẽ gửi lý do rỗng, tức 400.
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
    expect(chipMove(nhiemVu({ status: "cho-duyet" }), Q_DUYET, NGUOI_KHAC, "dang-thuc-hien")).toEqual({
      target: "dang-thuc-hien",
      kind: "return",
    });
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

  it("`chipMove`: a chip opens ONLY the moves `clickableTransitions` / `reasonMove` allow", () => {
    // The pipeline's single gate. Plain moves are exactly the Kanban's list; the reason moves are
    // the return and the reopen; everything else — the current status, an unlisted step, a step
    // the account may not take — is text.
    const t = nhiemVu({ status: "dang-thuc-hien" });
    expect(chipMove(t, Q_CAP_NHAT, NGUOI_KHAC, "cho-duyet")).toEqual({ target: "cho-duyet", kind: "plain" });
    expect(chipMove(t, Q_CAP_NHAT, NGUOI_KHAC, "dang-thuc-hien")).toBeNull();
    expect(chipMove(t, Q_CAP_NHAT, NGUOI_KHAC, "moi-giao")).toBeNull();
    // `chuyen-tiep` is never a status move (`…/status` answers 400 for it).
    expect(chipMove(t, Q_DUYET, NGUOI_KHAC, "chuyen-tiep")).toBeNull();
    // DENIED: no `task.update` and not the assignee ⇒ no chip is a move, listed or not.
    for (const code of ["cho-duyet", "hoan-thanh", "tam-dung"]) {
      expect(chipMove(t, quyenNhiemVu([]), NGUOI_KHAC, code)).toBeNull();
    }
    // The reopen needs `task.approve` (ADR 0065 NV2).
    const done = nhiemVu({ status: "hoan-thanh" });
    expect(chipMove(done, Q_CAP_NHAT, NGUOI_KHAC, "dang-thuc-hien")).toBeNull();
    expect(chipMove(done, Q_DUYET, NGUOI_KHAC, "dang-thuc-hien")).toEqual({ target: "dang-thuc-hien", kind: "reopen" });
  });

  // The press and the send (reason required, trimmed, target `dang-thuc-hien`) run in a DOM in
  // `task-detail-dialog.test.tsx`; this file renders strings only.

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner #12): the server's 403/400 sentence is the error toast, verbatim
  // — `task-detail-dialog.test.tsx` presses the chip and reads `toast.error`.
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
  // ĐỔI CHIỀU CÓ CHỦ Ý 06/10/2026 (prototype columns): FOUR — `Ngày giao` left the table.
  it("đúng BỐN nút sắp, `aria-sort` đúng chiều ở cột đang sắp, `none` ở các cột kia", () => {
    const html = veBangSapXep({ cot: "code", chieu: "asc" });
    // Owner #5: server-side sort. A column the server cannot sort draws its control DISABLED with a
    // "?" (lần 2 #12) — seven arrows, three of them on disabled buttons.
    expect(html.split("lucide-arrow-up-down").length - 1).toBe(7);
    expect(html.split('<button type="button" disabled=""').length - 1).toBe(3);
    expect(html.split('aria-label="Sắp xếp theo cột này').length - 1).toBe(3);
    const sortable = [...html.matchAll(/<th scope="col" aria-sort="(\w+)"[^>]*><button[^>]*>([^<]+)</g)].map((m) => [m[2], m[1]]);
    expect(sortable).toEqual([
      ["Mã", "ascending"],
      ["Tên việc", "none"],
      ["Ưu tiên", "none"],
      ["Hạn", "none"],
    ]);
    expect(html).not.toContain("Ngày giao");
    // The active column's arrow is full strength; the others are faded.
    expect(html.split("size-3 opacity-100").length - 1).toBe(1);
    // The three others: no `aria-sort` (nothing was asked of the server for them).
    expect(html).toMatch(/<th scope="col" class="[^"]*"><span[^>]*><button type="button" disabled=""[^>]*>Người thực hiện</);
    expect(html).toMatch(/<th scope="col" class="[^"]*"><span[^>]*><button type="button" disabled=""[^>]*>Trạng thái</);
  });

  it("đang sắp theo Ưu tiên giảm dần: `aria-sort=\"descending\"` ở đúng cột ấy", () => {
    const html = veBangSapXep({ cot: "priority", chieu: "desc" });
    expect(html).toMatch(/aria-sort="descending"[^>]*><button[^>]*>Ưu tiên</);
    expect(html.match(/aria-sort="(ascending|descending)"/g)).toHaveLength(1);
  });

  it("prototype columns, in order: Mã · Tên việc · Người thực hiện · Bộ phận · Ưu tiên · Hạn · Trạng thái", () => {
    const html = veBangSapXep(SAP_XEP_MAC_DINH);
    const heads = [...html.matchAll(/<th scope="col"[^>]*>(?:<span[^>]*>)?(?:<button[^>]*>)?([^<]+)/g)].map((m) => m[1]!.trim());
    expect(heads).toEqual(["Mã", "Tên việc", "Người thực hiện", "Bộ phận", "Ưu tiên", "Hạn", "Trạng thái"]);
    // The default order (newest first) has no column of its own: no arrow anywhere.
    expect(html).not.toMatch(/aria-sort="(ascending|descending)"/);
    // The whole row opens the task; the code cell is the keyboard's button. No `Mở NV…` column.
    expect(html).not.toContain("Mở NV");
    expect(html).toMatch(/<td class="[^"]*"><button type="button"[^>]*aria-expanded="false">NV19<\/button><\/td>/);
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
    expect(html).toContain(nhuTrongHTML(STATUS_MOVE_DENIED));
    expect(html).not.toContain('id="han-moi-lui-han"');
    expect(html).not.toContain(`aria-label="${TASK_DELETE_BUTTON}"`);
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
    // ADR 0038: asking is not behind the decide key — the section is there for `task.update` (its
    // form shows once the read returns no pending request: `TaskExtensionView` cases above).
    expect(html).toContain(`>${EXTENSION_TITLE}</h3>`);
  });

  it("có `task.extend` nhưng KHÔNG phải lãnh đạo ghi trên bản ghi: lớp hai vẫn chặn", () => {
    // Khoá thật KHÔNG thay được phép so mã của ADR 0038 — lớp hai chạy SAU lớp khoá, không thay nó.
    const html = veChiTiet({}, NGUOI_KHAC, { pha: "dangTai" }, DU_QUYEN);
    expect(html).not.toContain(`href="#${ID_HANG_CHO}"`);
    expect(html).not.toContain(nhuTrongHTML(NUT_DUYET));
    // The row-level sentence is pinned on `TaskExtensionView` (`người KHÁC: … Đang chờ lãnh đạo duyệt.`).
  });

  it("`task.delete` một mình: chỉ nút xoá ở đầu hộp chi tiết hiện", () => {
    const html = veChiTiet({}, NGUOI_KHAC, { pha: "dangTai" }, quyenNhiemVu([QUYEN_XOA_NHIEM_VU]));
    expect(html).toContain(`aria-label="${TASK_DELETE_BUTTON}"`);
    expect(html).not.toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
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
    expect(html).not.toContain(nhuTrongHTML(STATUS_MOVE_DENIED));
    // `Chờ duyệt` is drawn only while current (spec 07 §2); the move into it is on the Kanban.
    expect(html).toContain("Chuyển sang Tạm dừng");
    // ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026 (ADR 0065 NV1): người thực hiện hoàn thành thẳng, không cần
    // `task.approve` — máy chủ quyết lại trên dòng (và vẫn đòi mọi việc con đã xong).
    expect(html).toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("BỊ TỪ CHỐI — người thực hiện ở `cho-duyet`, thiếu `task.approve`: không tự duyệt, không trả lại", () => {
    const html = veChiTiet({ status: "cho-duyet" }, NGUOI_THUC_HIEN, { pha: "dangTai" }, KHONG_KHOA);
    expect(html).not.toContain(nhuTrongHTML(STATUS_MOVE_DENIED));
    expect(html).not.toContain("Chuyển sang Hoàn thành");
    expect(html).not.toContain('id="ly-do-tra-lai"');
    expect(html).not.toContain("Chuyển sang Đang thực hiện");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
  });

  it("BỊ TỪ CHỐI — không phải người thực hiện, không `task.update`: không khối, kể cả có `task.approve`", () => {
    for (const q of [KHONG_KHOA, CHI_DUYET]) {
      const html = veChiTiet({ status: "dang-thuc-hien" }, NGUOI_KHAC, { pha: "dangTai" }, q);
      expect(html).toContain(nhuTrongHTML(STATUS_MOVE_DENIED));
      expect(html).not.toContain("Chuyển sang");
      expect(html).not.toContain(nhuTrongHTML(CAU_THIEU_QUYEN_DUYET_HOAN_THANH));
    }
  });

  it("BỊ TỪ CHỐI — phiên chưa đọc (mã rỗng) trên việc CHƯA phân công: không khớp, không khối", () => {
    const html = veChiTiet({ status: "dang-thuc-hien", assignee: "" }, "", { pha: "dangTai" }, KHONG_KHOA);
    expect(html).toContain(nhuTrongHTML(STATUS_MOVE_DENIED));
    expect(html).not.toContain("Chuyển sang");
  });

  it("chỉ vẽ bước MÁY CHỦ liệt kê: danh sách rỗng ⇒ không nút nào, và câu nói vì sao", () => {
    const html = veChiTiet({ status: "dang-thuc-hien", allowed_transitions: [] }, NGUOI_KHAC, { pha: "dangTai" }, DU_QUYEN);
    expect(html).not.toContain("Chuyển sang");
    expect(html).toContain(STATUS_NO_EXIT);
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

  it("`hoan-thanh` + `task.approve`: chip MỞ LẠI (lý do bắt buộc khi bấm) — không nút thường", () => {
    const html = veChiTiet(
      { status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" },
      NGUOI_KHAC,
      { pha: "dangTai" },
      DU_QUYEN,
    );
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (prototype pipeline): the reason box opens on the press — its
    // label, `required`, the disabled `Xác nhận` and `reopenNote` are pinned in a DOM case
    // (`task-detail-dialog.test.tsx`).
    expect(html).toContain(`aria-label="${reasonMoveName("reopen", "Đang thực hiện")}"`);
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
    expect(html).toContain(`aria-label="${reasonMoveName("reopen", "Đang thực hiện")}"`);
  });
});

describe("W5 — tab `Liên quan đến tôi` và ô `Sắp đến hạn` trên hàng lọc", () => {
  const SRC = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("the counts are read with the SAME filter as the rows (`viewLoc`) — on EVERY view", () => {
    // `getTaskCounts` shares the filter builder with the list — see lib/api/nhiem-vu.test.ts. Since
    // 07/10/2026 the footer total of all three views comes from it, never from the page length
    // (owner #5); Sổ theo dõi forces `Theo văn bản` into `viewLoc`, so its total counts its rows.
    expect(SRC).toContain("getTaskCounts(viewLoc).then(");
    expect(SRC).not.toContain("getTaskCounts(loc)");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * 06/10/2026 — ADR 0068 §Sửa đổi lần 5: the screen follows the prototype (`TaskWorkspace.tsx`)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("the filter row — ONE row in the prototype's order, no `Bộ lọc` panel", () => {
  const row = (loc: Parameters<typeof HangLoc>[0]["loc"] = {}, extra: Partial<Parameters<typeof HangLoc>[0]> = {}) =>
    renderToStaticMarkup(
      <HangLoc
        loc={loc}
        tim=""
        datTim={() => {}}
        datLoc={() => {}}
        danhMuc={DANH_MUC}
        danhBa={DANH_BA}
        {...extra}
      />,
    );

  it("scope first as a segmented group, then search → Bộ phận → Người thực hiện → Ưu tiên → Loại → Khối → Nguồn giao → two toggles", () => {
    const html = row();
    const at = (s: string) => html.indexOf(s);
    const order = [
      'role="group" aria-label="Phạm vi"',
      'id="tim-nhiem-vu"',
      'id="loc-bo-phan"',
      'id="loc-nguoi-thuc-hien"',
      'id="loc-uu-tien"',
      'id="loc-loai"',
      'id="loc-khoi"',
      'id="loc-nguon-giao"',
      'id="loc-qua-han"',
      'id="loc-sap-den-han"',
    ];
    for (let i = 0; i < order.length; i++) expect(at(order[i]!)).toBeGreaterThan(-1);
    for (let i = 1; i < order.length; i++) expect(at(order[i - 1]!)).toBeLessThan(at(order[i]!));
    expect(html).toContain(`>${PHAM_VI_TOAN_XA}</button>`);
    expect(html).toContain(`>${PHAM_VI_CUA_TOI}</button>`);
    expect(html).toContain(`>${SCOPE_RELATED_LABEL}</button>`);
  });

  it("DENIED shapes: no `Bộ lọc` button, no panel, no status select, no `Tìm` button", () => {
    const html = row();
    expect(html).not.toContain(">Bộ lọc<");
    expect(html).not.toContain('aria-controls="task-filters-more"');
    expect(html).not.toContain('id="loc-trang-thai"');
    expect(html).not.toContain(">Tìm</button>");
  });

  it("`Sổ theo dõi` hides `Loại`, as the prototype does", () => {
    expect(row({}, { hideType: true })).not.toContain('id="loc-loai"');
  });

  it("drill-down: every control drawn but disabled", () => {
    const html = row({}, { disabled: true });
    expect(html).toMatch(/<select id="loc-bo-phan" aria-label="Lọc theo bộ phận" disabled=""/);
    expect(html).toMatch(/id="loc-qua-han"[^>]*disabled=""/);
  });

  // `scope values` and the two exclusive toggles are clicks now (the row holds the search debounce,
  // an effect) — `task-filter-row.interaction.test.tsx`.

  it("page wiring: the selection bar and the view switch are the row's right end; drill-down banner under it", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    const page = src.slice(src.indexOf("export function SoNhiemVu("), src.indexOf("export function CanhBaoNhanTrangThai"));
    expect(page.indexOf("<HangLoc")).toBeGreaterThan(-1);
    expect(page.indexOf("<HangLoc")).toBeLessThan(page.indexOf("<DrillDownBanner"));
    expect(page).toContain("disabled={drillDownActive}");
    expect(page).toContain('hideType={viewMode === "so-theo-doi"}');
    // The on-page extension queue and the inline create form are gone (prototype).
    expect(page).not.toContain("HangChoLuiHan");
    expect(page).not.toContain("Đóng biểu mẫu giao việc");
    expect(page).toContain("<FormGiaoViec\n          dialog");
    // `+ Thêm việc con` opens the SAME dialog (stacked over the detail panel).
    expect(page).toContain("key={drawer.nhiemVu.code}\n                      dialog");
    // Phản ánh `Tạo nhiệm vụ` opens the SAME form as the prototype's dialog too (06/10/2026).
    const petition = readFileSync(fileURLToPath(new URL("../phan-anh/petition-task.tsx", import.meta.url)), "utf8");
    expect(petition).toMatch(/<FormGiaoViec\s+dialog/);
    // Biên bản `Tách thành nhiệm vụ` opens the SAME form as the prototype's dialog (06/10/2026).
    const meeting = readFileSync(fileURLToPath(new URL("../bien-ban/so-bien-ban.tsx", import.meta.url)), "utf8");
    expect(meeting).toMatch(/<FormGiaoViec\s+dialog/);
  });
});

describe("the detail — one view, the prototype's order", () => {
  it("status strip → fact grid (spec 07 §4: four cells) → left column → Nhật ký on the right", () => {
    const html = veChiTiet({ description: "Mô tả giả" });
    const at = (s: string) => html.indexOf(s);
    const facts = [...html.matchAll(/<p class="text-ink-muted m-0 mb-1 text-\[10.5px\] font-bold tracking-wide uppercase">([^<]+)<\/p>/g)].map(
      (m) => m[1],
    );
    expect(facts).toEqual([
      "Hạn xử lý",
      "Cơ quan thực hiện (chủ trì tham mưu)",
      "Người thực hiện (chuyên viên tham mưu)",
      "Mức ưu tiên",
    ]);
    expect(at('aria-label="Các bước của vòng đời nhiệm vụ"')).toBeLessThan(at(">Hạn xử lý</p>"));
    expect(at(">Mức ưu tiên</p>")).toBeLessThan(at('id="task-detail-info"'));
    expect(at('id="task-detail-info"')).toBeLessThan(at('id="task-detail-description"'));
    expect(at('id="task-detail-description"')).toBeLessThan(at('id="task-detail-deadlines"'));
    expect(at('id="task-detail-deadlines"')).toBeLessThan(at('id="task-detail-extensions"'));
    // The timeline is the right column, after the left one.
    expect(at('id="task-detail-extensions"')).toBeLessThan(at('aria-labelledby="tieu-de-nhat-ky-nhiem-vu-NV19"'));
  });

  it("no description: no empty `Mô tả` card", () => {
    expect(veChiTiet({ description: "" })).not.toContain('id="task-detail-description"');
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BÁO CÁO KIỂM THỬ 05/10/2026 — mục 3.9 (NV-04, NV-08, NV-09, NV-11, NV-13)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("NV-04 — hạn xử lý sửa/đặt được ở MỌI loại, sau cùng khoá `task.update`", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (user decision 1, prototype): the separate `Sửa hạn xử lý` /
  // `Đặt hạn xử lý` button is gone. The deadline is edited in the information block's inline
  // `✎ Sửa` — with the title for a basic task, inside the document form for `Theo văn bản`.
  const ONLY_UPDATE = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);
  const READ_ONLY = quyenNhiemVu(["task.read"]);

  it("loại `co-ban`, cầm `task.update`: khối `Thông tin nhiệm vụ` có `✎ Sửa`", () => {
    const html = veChiTiet({ type: "co-ban" }, LANH_DAO, { pha: "dangTai" }, ONLY_UPDATE);
    expect(html).toMatch(new RegExp(`aria-label="${TASK_INFO_EDIT_LABEL}"[^>]*><svg[^>]*>.*?</svg>Sửa</button>`));
  });

  it("việc tạo ra KHÔNG có hạn: vẫn sửa được (đặt hạn trong `✎ Sửa`), và khối lùi hạn chỉ tới đó", () => {
    const html = veChiTiet(
      { type: "co-ban", due_at: null, original_due_at: null },
      LANH_DAO,
      { pha: "dangTai" },
      ONLY_UPDATE,
    );
    expect(html).toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
    // No deadline ⇒ no request form (the sentence itself is pinned on `TaskExtensionView`).
    expect(html).not.toContain('id="han-moi-lui-han"');
    expect(html).not.toContain("Hạn chỉ đặt được một lần");
  });

  it("CA BỊ TỪ CHỐI — thiếu `task.update`: không có `✎ Sửa` nào", () => {
    const html = veChiTiet({ type: "co-ban", due_at: null }, LANH_DAO, { pha: "dangTai" }, READ_ONLY);
    expect(html).not.toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
    expect(theNutSua(html)).toBeNull();
  });

  it("loại `theo-van-ban`: một `✎ Sửa` — của khối văn bản, không có nút thứ hai", () => {
    const html = veChiTiet({}, LANH_DAO, { pha: "xong", duLieu: BA_VAN_BAN }, ONLY_UPDATE);
    expect(theNutSua(html)).not.toBeNull();
    expect(html).not.toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
  });

  it("`basicTaskEditBody`: chỉ tiêu đề và hạn đã đổi, kèm khoá lạc quan — KHÔNG BAO GIỜ `code`", () => {
    const opened = nhiemVu({ type: "co-ban", title: "Rà soát hộ nghèo", note: "ghi chú có dấu cách cuối " });
    const unchanged = formSuaTuChiTiet(opened, []);
    expect(basicTaskEditBody(unchanged, opened)).toBeNull();

    const body = basicTaskEditBody({ ...unchanged, tieuDe: "  Rà soát hộ nghèo quý IV  " }, opened);
    expect(body).toEqual({ title: "Rà soát hộ nghèo quý IV", expected_updated_at: opened.updated_at });
    // The note the form never showed is NOT re-sent, even though trimming it would differ.
    expect(body).not.toHaveProperty("note");
    expect(body).not.toHaveProperty("code");

    const due = basicTaskEditBody({ ...unchanged, dueDate: "2026-07-01", dueTime: "17:00" }, opened);
    expect(due).toEqual({ due_at: "2026-07-01T17:00:00+07:00", expected_updated_at: opened.updated_at });
  });
});

describe("NV-08 → spec 07 §4: the priority cell carries the STORED progress as its second line", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner, ADR 0076 lần 2: the drawer follows spec 07): NV-08 (05/10)
  // replaced `0% tiến độ ghi nhận` with a disabled box and a "?". Spec 07 §4 draws `{n}% tiến độ ghi
  // nhận` under the priority — the STORED figure, never computed here (sub-task weights are a
  // BACKEND DEPENDENCY). The disabled box and its "?" are gone.
  it("the fact grid's priority cell: `Cao`, then `40% tiến độ ghi nhận`", () => {
    const html = veChiTiet({ progress: 40 });
    expect(html).toContain(`>Mức ưu tiên</p><p class="text-navy m-0 text-[12.5px]">Cao</p>`);
    expect(html).toContain(`>${progressFactText(40)}</p>`);
    expect(progressFactText(0)).toBe("0% tiến độ ghi nhận");
    expect(html).not.toContain('id="chi-tiet-tien-do"');
    expect(html).not.toContain(nhuTrongHTML(TASK_PROGRESS_PENDING.ten));
  });
});

describe("NV-09 → hộp chi tiết lớn (ADR 0068 §Sửa đổi 05/10/2026): mọi lối mở đi qua MỘT lệnh, không còn cuộn", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 05/10/2026: these pinned the scroll-to-detail of NV-09. The detail is now a
  // dialog over the list, so there is nothing to scroll to; what must still hold is that every way
  // of opening dispatches the same `mo` — the dialog, its URL and its focus return all hang off it.
  // The behaviour itself (click → dialog + one pushed entry, `?task=` → dialog, close → focus back)
  // is exercised in a DOM in `task-detail-dialog.test.tsx`.
  const SRC = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("chỉ hai chỗ phát lệnh mở: hàm `openDrawer` và đường `?task=` lúc tải", () => {
    expect(SRC).toContain(
      'function openDrawer(n: petitions_nhiemVuRa) {\n    guiDrawer({ loai: "mo", nhiemVu: n });\n  }',
    );
    expect(SRC).toContain(
      '      guiDrawer({ loai: "mo", nhiemVu: kq.duLieu });\n    });\n    return () => {\n      cancelled = true;',
    );
    // A THIRD dispatch would be a way of opening that bypasses `openDrawer`.
    expect(SRC.split('guiDrawer({ loai: "mo"').length - 1).toBe(2);
    // The scroll is gone with the in-page block — a leftover would jump the page under the dialog.
    expect(SRC).not.toContain("scrollIntoView");
  });

  it("dòng bảng, thẻ Kanban, Sổ theo dõi, việc con và Back/Forward đều qua `openDrawer`", () => {
    // The three views and the child list hand `openDrawer` itself down (07/10/2026: the views'
    // wrappers went with the rewrite).
    expect(SRC.split("moNhiemVu={openDrawer}").length - 1).toBe(3);
    expect(SRC.split("openTask={openDrawer}").length - 1).toBe(1);
    // 2 → 1 on 07/10/2026: `Việc cha` (open by code) left the drawer (owner #6); Back/Forward remains.
    expect(SRC.split("openDrawer(kq.duLieu);").length - 1).toBe(1);
  });

  it("thanh địa chỉ theo MÃ ĐANG MỞ, ở một chỗ; chi tiết vẽ trong `LargeDialog`", () => {
    // With record tabs the address names the ACTIVE tab while the panel is shown (`shownCode`).
    expect(SRC.split("useTaskDialogUrl(shownCode,").length - 1).toBe(1);
    expect(SRC).toContain("<LargeDialog\n          titleId={TASK_DETAIL_TITLE_ID}");
  });
});

describe("NV-11 — thẻ Kanban nói bộ phận đang giữ việc; nhãn nguồn `truc-tiep` không gợi 'giao cho người'", () => {
  const UNITS = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);

  it("chưa phân công người: `{bộ phận} · Chưa phân công`", () => {
    expect(cardHolderText({ assignee: "", unit: "01JBOPHAN" }, null, UNITS)).toBe(
      `VĂN PHÒNG ĐẢNG ỦY · ${CHUA_PHAN_CONG}`,
    );
    // Bộ phận ngoài danh mục: hiện id, không bịa tên.
    expect(cardHolderText({ assignee: "", unit: "bp-la" }, null, UNITS)).toBe(`bp-la · ${CHUA_PHAN_CONG}`);
  });

  it("có người: họ tên/mã như cũ; không bộ phận lẫn người: `Chưa phân công`", () => {
    expect(cardHolderText({ assignee: "CB-1", unit: "01JBOPHAN" }, null, UNITS)).toBe("CB-1");
    expect(cardHolderText({ assignee: "", unit: "" }, null, UNITS)).toBe(CHUA_PHAN_CONG);
  });

  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner #2: the wording is the spec's, entirely): NV-11 (05/10)
  // renamed `truc-tiep` away from `Giao trực tiếp`; spec 10 uses `Giao trực tiếp`. The card's holder
  // line (above) still says the unit, so the reading NV-11 feared does not come back.
  it("nhãn nguồn `truc-tiep` là `Giao trực tiếp` (spec 10)", () => {
    expect(nhanNguonGiao("truc-tiep")).toBe("Giao trực tiếp");
  });
});

describe("NV-13 — không còn tham chiếu đặc tả hay câu kỹ thuật trên màn", () => {
  it("câu dưới Kanban không có `§`; câu việc con là câu hành chính", () => {
    expect(ghiChuKanbanReNhanh(BANG_NHAN_MAC_DINH)).not.toContain("§");
    expect(childFormNote("NV19")).toBe("Việc con của NV19.");
  });
});
