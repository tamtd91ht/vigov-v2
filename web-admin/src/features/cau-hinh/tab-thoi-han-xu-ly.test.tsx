import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachSLARa,
  identity_dongSLARa,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { RESOLVE_HOURS_ERROR } from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan, slaFieldLabelReadDecision } from "./quyen-tab";
import { banTuDong } from "./sua-thoi-han";
import { ManThoiHanXuLy, type DuLieuTab, type InlineEdit, type ThaoTacThoiHan } from "./tab-thoi-han-xu-ly";

/**
 * Tab "Thời hạn xử lý" since ADR 0079 D2 holds the SLA table only; the three calendar tables and their
 * checks moved to `working-calendar-tab.test.tsx` with their code.
 *
 * WHAT THIS FILE GUARDS — each is a one-line edit away from breaking with nothing else turning red:
 *
 * 1. KHỐI CẢNH BÁO CÓ MẶT KHI XÃ CHƯA KHAI XONG, VÀ NÓI RA HẬU QUẢ — and is ABSENT once configured.
 * 2. "+ Thêm thời hạn cho một lĩnh vực" and "Xoá thời hạn riêng" are only disabled "?" placeholders
 *    (ADR 0068 §14, ADR 0026 stop condition #2) — no clickable button to a route that does not exist.
 * 3. The banner states the unit "giờ làm việc" — the cells say only "{n} giờ" (spec 08), so the banner
 *    is now the one place the unit is said — and never a hardcoded SLA figure (rule 10 forbidden #3).
 * 4. Nothing beyond the prototype (owner 08/10/2026): no re-seed button once rows exist, no footnote.
 * 5. Editing is IN PLACE (spec 08) and its refusals show in place, the local one and the server one apart.
 */

const KHONG_LAM_GI: ThaoTacThoiHan = {
  gieoThoiHan: () => {},
  suaThoiHan: () => {},
};

const DONG_SLA: identity_dongSLARa = {
  id: "01J0000000000000000000SLA",
  work_kind: "phan-anh",
  field: "an-ninh-trat-tu",
  is_default: false,
  acknowledge_hours: 2,
  resolve_hours: 16,
  due_soon_hours: 4,
  escalate_leader_hours: 8,
  escalate_president_hours: 16,
  unassigned_hold_hours: 8,
};

const DEFAULT_ROW: identity_dongSLARa = {
  ...DONG_SLA,
  id: "01J0000000000000000000DEF",
  field: "",
  is_default: true,
};

function ok<T>(duLieu: T): KetQua<T> {
  return { ok: true, duLieu };
}

function phienVoi(quyen: string[]): KetQua<identity_phienHienTaiRa> {
  return ok<identity_phienHienTaiRa>({
    sid: "01J000000000000000000SID",
    expires_at: "2026-09-26T12:00:00Z",
    staff: { code: "CB001", full_name: "Cán bộ thử", position: "Chuyên viên" },
    role: null,
    permissions: quyen,
    must_change_password: false,
  });
}

const SLA_CO_DONG = ok<identity_danhSachSLARa>({ items: [DONG_SLA], problems: [] });
const SLA_RONG = ok<identity_danhSachSLARa>({ items: [], problems: [] });

function rowsOf(...items: identity_dongSLARa[]): KetQua<identity_danhSachSLARa> {
  return ok<identity_danhSachSLARa>({ items, problems: [] });
}

function editOf(row: identity_dongSLARa, more: Partial<InlineEdit> = {}): InlineEdit {
  return {
    rowId: row.id,
    draft: banTuDong(row),
    localError: "",
    serverError: "",
    busy: false,
    setDraft: () => {},
    onSave: () => {},
    onCancel: () => {},
    ...more,
  };
}

function ve(
  du: Partial<DuLieuTab> = {},
  them: {
    coQuyenGhi?: boolean;
    fieldLabels?: ReadonlyMap<string, string>;
    edit?: InlineEdit | null;
    seedError?: string;
  } = {},
) {
  return renderToStaticMarkup(
    <ManThoiHanXuLy
      du={{
        thoiHan: SLA_CO_DONG,
        ...du,
      }}
      coQuyenGhi={them.coQuyenGhi ?? true}
      fieldLabels={them.fieldLabels}
      thaoTac={KHONG_LAM_GI}
      loiMayChuNgoaiForm={them.seedError ?? ""}
      dangGui={false}
      edit={them.edit ?? null}
    />,
  );
}

/** Every `<input>` tag of the markup. */
function inputs(html: string): string[] {
  return html.match(/<input[^>]*>/g) ?? [];
}

describe("khối 'đơn vị chưa khai xong'", () => {
  it("bảng thời hạn RỖNG → khối có mặt và nói ra hậu quả", () => {
    const html = ve({ thoiHan: SLA_RONG });

    expect(html).toContain("Đơn vị chưa khai xong phần bắt buộc");
    // HẬU QUẢ, không phải tình trạng. "Chưa có dữ liệu" đọc ra là một việc để hôm khác.
    expect(html).toContain("chưa vào sổ được văn bản đến");
    expect(html).toContain("chưa nhận được phản ánh của người dân");
    expect(html).toContain("Bảng Thời hạn xử lý đang trống");
  });

  it("khối mang ĐÚNG nút gieo của bảng thời hạn, và chỉ nó — lịch tuần ở tab Lịch làm việc (ADR 0079 D2)", () => {
    const html = ve({ thoiHan: SLA_RONG });

    // Câu `problems` của máy chủ gọi đích danh nhãn nút này.
    expect(html).toContain("Gieo thời hạn mặc định");
    expect(html).not.toContain("Gieo giờ làm việc mặc định");
    expect(html).not.toContain("Giờ làm việc trong tuần đang trống");
  });

  it("bảng đã có dòng → khối VẮNG MẶT", () => {
    const html = ve();

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).not.toContain("chưa vào sổ được văn bản đến");
  });

  it("bảng đã có dòng → KHÔNG có nút gieo lại — đúng prototype (chủ dự án 08/10/2026)", () => {
    // REGRESSION (round 1 #7): a secondary "Gieo thời hạn mặc định" stood beside the Add button.
    expect(ve()).not.toContain("Gieo thời hạn mặc định");
  });

  it("chưa đọc xong → khối VẮNG MẶT, trang nói đang tải, và không có bảng", () => {
    const html = ve({ thoiHan: null });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Đang tải bảng thời hạn xử lý…");
    expect(html).not.toContain("<table");
  });

  it("đọc hỏng → KHÔNG khẳng định xã chưa khai; hiện NGUYÊN câu máy chủ", () => {
    const html = ve({
      thoiHan: { ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." },
    });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Bạn không có quyền thực hiện thao tác này.");
  });

  it("gieo bị từ chối → câu máy chủ hiện TẠI CHỖ", () => {
    const html = ve({}, { seedError: "Máy chủ từ chối gieo." });
    expect(html).toMatch(/<p role="alert"[^>]*>Máy chủ từ chối gieo\.<\/p>/);
  });
});

describe("hộp giải thích (spec 08)", () => {
  it("nói ĐƠN VỊ 'giờ làm việc' in đậm — ô bảng chỉ còn '{n} giờ', nên đây là chỗ duy nhất nói nó", () => {
    expect(ve()).toContain("Thời hạn tính theo <b>giờ làm việc</b>, không tính ngày nghỉ và ngày lễ.");
  });

  it("KHÔNG có câu 'Mặc định 72 giờ, tức ba ngày' — một con số SLA ghi cứng (luật 10 cấm #3)", () => {
    // REGRESSION: the old banner rendered `DAN_THOI_HAN_2`, which ended with exactly this sentence.
    const html = ve();
    expect(html).not.toContain("72 giờ");
    expect(html).not.toContain("tức ba ngày");
  });

  it("câu áp dụng nói đúng điều hệ thống làm: hạn đã đặt giữ nguyên (luật 10 bất biến 2, ADR 0028)", () => {
    // The spec's "chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu" is false here: a citizen's petition
    // received before the save gets `han_xu_ly_xong` when its field is settled — possibly after.
    const html = ve();
    expect(html).toContain("Thay đổi chỉ áp dụng cho hạn đặt sau thời điểm lưu; hạn đã đặt cho hồ sơ giữ nguyên.");
    expect(html).not.toContain("hồ sơ tiếp nhận sau thời điểm lưu");
  });

  it("cột Sắp đến hạn khi còn: ba công dụng, tên cột in đậm", () => {
    expect(ve()).toContain(
      "Cột <b>Sắp đến hạn khi còn</b> quyết định cả ba: lúc nào gửi lời nhắc, ô lọc “Sắp đến hạn” trên màn " +
        "nhiệm vụ lấy ra việc nào, và con số trong thông báo ở chuông.",
    );
  });
});

describe("bảng thời hạn xử lý", () => {
  it("thêm thời hạn cho một lĩnh vực: CHỈ là chỗ giữ vô hiệu có dấu '?' (ADR 0068 §14), không một nút bấm được", () => {
    const html = ve();

    expect(html).toContain(pendingMarkerLabel("Thêm thời hạn cho một lĩnh vực"));
    const buttons = html.match(/<button[^>]*>(?:(?!<\/button>).)*Thêm thời hạn(?:(?!<\/button>).)*<\/button>/g) ?? [];
    expect(buttons.length).toBeGreaterThan(0);
    for (const b of buttons) expect(b).toContain('disabled=""');
    expect(html).not.toContain("lĩnh vực mới");
    expect(html).not.toContain("Thêm lĩnh vực");
  });

  it("ô số nói '{n} giờ', Xử lý xong in đậm, ba cột báo nói 'sau {n} giờ'", () => {
    const html = ve();

    expect(html).toContain("<td>2 giờ</td>");
    expect(html).toContain('<td class="font-semibold">16 giờ</td>');
    expect(html).toContain("<td>4 giờ</td>");
    expect(html).toContain("<td>sau 8 giờ</td>");
    expect(html).toContain("<td>sau 16 giờ</td>");
    // Cells no longer repeat the unit (spec 08); the banner says it.
    expect(html).not.toContain("16 giờ làm việc");
  });

  it("thứ tự cột theo spec, cột Giữ chưa phân công đứng trước cột thao tác", () => {
    const heads = (ve().match(/<th[^>]*>(?:(?!<\/th>).)*<\/th>/g) ?? []).map((h) => h.replace(/<[^>]+>/g, ""));
    expect(heads).toEqual([
      "Loại việc",
      "Lĩnh vực",
      "Tiếp nhận",
      "Xử lý xong",
      "Sắp đến hạn khi còn",
      "Báo lãnh đạo trực tiếp",
      "Báo Chủ tịch",
      "Giữ chưa phân công",
      "Thao tác",
    ]);
  });

  it("`problems` của máy chủ hiện ra NGUYÊN VĂN, không nuốt", () => {
    const cau =
      "Loại việc “phan-anh” có dòng riêng nhưng thiếu dòng mặc định, nên lĩnh vực nào không có " +
      "dòng riêng sẽ không tính được hạn.";
    const html = ve({
      thoiHan: ok<identity_danhSachSLARa>({
        items: [DONG_SLA],
        problems: [{ kind: "missing_default_row", work_kind: "phan-anh", message: cau }],
      }),
    });

    expect(html).toContain(cau);
  });

  it("dòng mặc định nói rõ nó là mặc định, chữ mờ", () => {
    expect(ve({ thoiHan: rowsOf(DEFAULT_ROW) })).toContain('<td class="text-ink-muted">Mặc định cho mọi lĩnh vực</td>');
  });

  it("dòng `don-thu` đọc là 'Đơn thư', chữ navy đậm, không hiện mã thô", () => {
    const html = ve({ thoiHan: rowsOf({ ...DEFAULT_ROW, work_kind: "don-thu" }) });

    expect(html).toContain('<td class="text-navy font-medium">Đơn thư</td>');
    expect(html).not.toContain("don-thu");
  });

  it("nhịp dọc space-y-3 (`SlaTable.tsx:67`); đầu cột được xuống dòng để bảng vừa khung, không đẩy cột thao tác ra ngoài", () => {
    const html = ve();
    expect(html).toMatch(/^<section class="space-y-3"/);
    expect(html).toContain('<div class="[&amp;_th]:min-w-[4.5rem] [&amp;_th]:whitespace-normal"><div role="region"');
  });

  it("không còn tên lớp CSS cũ nào", () => {
    const html = ve({}, { edit: editOf(DONG_SLA) });
    for (const legacy of ["tab-thoi-han", "form-danh-muc", "o-nhap", "o-thao-tac", "bang-cuon", "ma-muc", "thong-bao-loi"]) {
      expect(html, legacy).not.toContain(legacy);
    }
  });
});

describe("cột Giữ chưa phân công (migration 0016, ADR 0079 quyết định 3)", () => {
  it("con số hiện 'sau {n} giờ'", () => {
    expect(ve({ thoiHan: rowsOf({ ...DONG_SLA, unassigned_hold_hours: 12 }) })).toContain("<td>sau 12 giờ</td>");
  });

  it("CHƯA ĐẶT hiện '—' và vẫn nói 'Không báo' cho trình đọc màn hình — không một con số nào thay lựa chọn của xã", () => {
    const html = ve({ thoiHan: rowsOf({ ...DONG_SLA, unassigned_hold_hours: null }) });

    expect(html).toContain('<td><span aria-hidden="true">—</span><span class="sr-only">Không báo</span></td>');
    expect(html).not.toContain("null");
  });

  it("KHÔNG có ghi chú nào dưới bảng — đúng prototype (chủ dự án 08/10/2026)", () => {
    // REGRESSION (round 1 #15): a footnote under the table, absent from `SlaTable.tsx`.
    const html = ve();

    expect(html).not.toContain("đếm từ lúc việc đã quá hạn");
    expect(html).not.toContain("để trống là không báo");
    expect(html).not.toContain("CHƯA tự gửi");
    expect(html).not.toContain("sla-reporting-note");
    // The table is the last thing in the tab.
    expect(html).toMatch(/<\/table><\/div><\/div><\/section>$/);
  });
});

describe("sửa tại chỗ (spec 08)", () => {
  it("dòng đang sửa: sáu ô số h-8 w-20 nạp sẵn con số hiện tại; nút thành Lưu / Huỷ", () => {
    const html = ve({}, { edit: editOf(DONG_SLA) });
    const boxes = inputs(html);

    expect(boxes).toHaveLength(6);
    for (const b of boxes) {
      expect(b).toContain('type="number"');
      expect(b).toContain("h-8 w-20 text-[12.5px]");
    }
    expect(html).toMatch(/<input[^>]*name="resolve_hours"[^>]*value="16"/);
    expect(html).toMatch(/<button[^>]*>Lưu<\/button>/);
    expect(html).toMatch(/<button[^>]*>Huỷ<\/button>/);
    // The pencil and the trash of the row being edited are gone.
    expect(html).not.toContain('title="Sửa thời hạn"');
    expect(html).not.toContain(pendingMarkerLabel("Xoá thời hạn riêng"));
  });

  it("không ô nào trỏ tới một ghi chú đã bỏ (aria-describedby treo là id không tồn tại)", () => {
    const html = ve({}, { edit: editOf(DONG_SLA) });
    expect(inputs(html).filter((b) => b.includes("aria-describedby"))).toHaveLength(0);
  });

  it("chỉ DÒNG đang sửa thành ô nhập; dòng khác giữ chữ và nút bút", () => {
    const html = ve({ thoiHan: rowsOf(DONG_SLA, DEFAULT_ROW) }, { edit: editOf(DONG_SLA) });
    expect(inputs(html)).toHaveLength(6);
    expect(html).toContain('aria-label="Sửa thời hạn Phản ánh của người dân — Mặc định cho mọi lĩnh vực"');
  });

  it("lỗi tại chỗ và lỗi máy chủ hiện NGAY DƯỚI dòng, tách hai câu", () => {
    const serverSentence = "Số giờ báo Chủ tịch không được nhỏ hơn số giờ báo lãnh đạo trực tiếp.";
    const html = ve({}, { edit: editOf(DONG_SLA, { localError: RESOLVE_HOURS_ERROR, serverError: serverSentence }) });

    expect(html).toContain(`<p role="alert" class="text-danger m-0 text-[12.5px]">${RESOLVE_HOURS_ERROR}</p>`);
    expect(html).toContain(`<p role="alert" class="text-danger m-0 text-[12.5px]">${serverSentence}</p>`);
    expect(html).toContain('<td colSpan="9">');
  });

  it("đang lưu: nút Lưu báo bận, ô nhập khoá", () => {
    const html = ve({}, { edit: editOf(DONG_SLA, { busy: true }) });
    expect(html).toContain("Đang lưu…");
    for (const b of inputs(html)) expect(b).toContain('disabled=""');
  });
});

describe("xoá thời hạn riêng: chỗ giữ '?' (ADR 0079 #5)", () => {
  it("dòng có lĩnh vực: nút thùng rác VÔ HIỆU, có dấu '?'", () => {
    const html = ve();

    expect(html).toContain(pendingMarkerLabel("Xoá thời hạn riêng"));
    expect(html).toMatch(/<button[^>]*title="Xoá thời hạn riêng"[^>]*disabled=""/);
  });

  it("dòng mặc định: KHÔNG có nút xoá — mọi lĩnh vực chưa có dòng riêng dựa vào nó", () => {
    const html = ve({ thoiHan: rowsOf(DEFAULT_ROW) });
    expect(html).not.toContain("Xoá thời hạn riêng");
    expect(html).toContain('title="Sửa thời hạn"');
  });
});

describe("nút ghi đi theo `admin.sla`", () => {
  it("CA BỊ TỪ CHỐI: thiếu quyền ghi thì bảng vẫn đủ dòng, không bút, không thùng rác, không chỗ giữ Thêm, không gieo", () => {
    const html = ve({}, { coQuyenGhi: false });

    expect(html).toContain('<td class="font-semibold">16 giờ</td>');
    expect(html).not.toContain("Sửa thời hạn");
    expect(html).not.toContain("Xoá thời hạn riêng");
    expect(html).not.toContain("Thêm thời hạn");
    expect(html).not.toContain("Gieo thời hạn mặc định");
    expect(html).not.toContain("Thao tác");
  });

  it("CA BỊ TỪ CHỐI: một `edit` lọt vào khi thiếu quyền vẫn không vẽ ô nhập nào", () => {
    expect(inputs(ve({}, { coQuyenGhi: false, edit: editOf(DONG_SLA) }))).toHaveLength(0);
  });

  it("CHỈ có `admin.sla` (không `admin.lookup`, không khoá admin nào khác): dùng được tab", () => {
    const phien = phienVoi(["admin.sla"]);
    const quyet = quyetDinhGhiThoiHan(phien);
    expect(quyet).toEqual({ hien: true });

    const html = ve({}, { coQuyenGhi: quyet.hien });
    expect(html).toContain('aria-label="Sửa thời hạn');
  });

  it("không có `admin.sla` (dù có mọi khoá admin khác): KHÔNG một nút ghi nào", () => {
    const phien = phienVoi(["admin.lookup", "admin.org", "admin.user", "admin.role", "admin.audit"]);
    const quyet = quyetDinhGhiThoiHan(phien);
    expect(quyet).toEqual({ hien: false, vi: "khong-du-quyen" });

    const html = ve({}, { coQuyenGhi: quyet.hien });
    expect(html).not.toContain('aria-label="Sửa thời hạn');
  });
});

describe("cột Lĩnh vực (SLA-03)", () => {
  it("có nhãn của xã → hiện nhãn, cả trong tên nút Sửa; không còn nhắc câu hỏi mở #4", () => {
    const html = ve({}, { fieldLabels: new Map([["an-ninh-trat-tu", "An ninh, trật tự"]]) });
    expect(html).toContain("An ninh, trật tự");
    expect(html).not.toContain(">an-ninh-trat-tu<");
    expect(html).toContain('aria-label="Sửa thời hạn Phản ánh của người dân — An ninh, trật tự"');
    expect(html).not.toMatch(/câu mở #4|câu hỏi mở #4/);
  });

  it("không đọc được nhãn → hiện nguyên mã", () => {
    expect(ve()).toContain(">an-ninh-trat-tu<");
  });

  it("slaFieldLabelReadDecision: chỉ admin.lookup mới đọc danh mục lĩnh vực; admin.sla thôi thì không", () => {
    expect(slaFieldLabelReadDecision(phienVoi(["admin.sla", "admin.lookup"])).hien).toBe(true);
    expect(slaFieldLabelReadDecision(phienVoi(["admin.sla"])).hien).toBe(false);
  });
});
