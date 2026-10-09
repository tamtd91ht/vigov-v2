import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachSLARa,
  identity_dongSLARa,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { SlaFieldCatalogues, SlaFieldOption } from "@/lib/api/thoi-han-xu-ly";

import {
  FIELD_REQUIRED_ERROR,
  PETITION_FIELDS_NOT_READABLE,
  RESOLVE_HOURS_ERROR,
} from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan, slaFieldLabelReadDecision } from "./quyen-tab";
import { banTuDong, newAddDraft } from "./sua-thoi-han";
import {
  ManThoiHanXuLy,
  type AddForm,
  type DuLieuTab,
  type InlineEdit,
  type InlineRemove,
  type ThaoTacThoiHan,
} from "./tab-thoi-han-xu-ly";

/**
 * Tab "Thời hạn xử lý" since ADR 0079 D2 holds the SLA table only; the three calendar tables and their
 * checks moved to `working-calendar-tab.test.tsx` with their code.
 *
 * WHAT THIS FILE GUARDS — each is a one-line edit away from breaking with nothing else turning red:
 *
 * 1. KHỐI CẢNH BÁO CÓ MẶT KHI XÃ CHƯA KHAI XONG, VÀ NÓI RA HẬU QUẢ — and is ABSENT once configured.
 * 2. "+ Thêm thời hạn cho một lĩnh vực" and "Xoá thời hạn riêng" are LIVE (ADR 0079 lô 2 Q4): the add
 *    row offers only the kind's own ACTIVE list, never `don-thu`; removal asks a reason (rule 7) and
 *    never appears on the default row.
 * 3. The banner states the unit "giờ làm việc" — the cells say only "{n} giờ" (spec 08), so the banner
 *    is now the one place the unit is said — and never a hardcoded SLA figure (rule 10 forbidden #3).
 * 4. Nothing beyond the prototype (owner 08/10/2026): no re-seed button once rows exist, no footnote.
 * 5. Editing is IN PLACE (spec 08) and its refusals show in place, the local one and the server one apart.
 */

const KHONG_LAM_GI: ThaoTacThoiHan = {
  gieoThoiHan: () => {},
  suaThoiHan: () => {},
  toggleAdd: () => {},
  startRemove: () => {},
};

const option = (code: string, label: string, active = true): SlaFieldOption => ({ code, label, active });

/** The three lists as a commune would have them: one retired entry in each, to prove it is filtered. */
const CATALOGUES: SlaFieldCatalogues = {
  "phan-anh": { ok: true, duLieu: [option("an-ninh-trat-tu", "An ninh, trật tự"), option("cu", "Lĩnh vực cũ", false)] },
  "van-ban-den": { ok: true, duLieu: [option("cong-van", "Công văn"), option("to-trinh-cu", "Tờ trình cũ", false)] },
  "nhiem-vu": { ok: true, duLieu: [option("khan", "Khẩn"), option("thuong", "Thường")] },
};

function addOf(more: Partial<AddForm> = {}): AddForm {
  return {
    draft: newAddDraft(),
    localError: "",
    serverError: "",
    busy: false,
    setDraft: () => {},
    onSubmit: () => {},
    onCancel: () => {},
    ...more,
  };
}

function removeOf(row: identity_dongSLARa, more: Partial<InlineRemove> = {}): InlineRemove {
  return {
    rowId: row.id,
    reason: "",
    localError: "",
    serverError: "",
    busy: false,
    setReason: () => {},
    onConfirm: () => {},
    onCancel: () => {},
    ...more,
  };
}

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
    catalogues?: SlaFieldCatalogues;
    edit?: InlineEdit | null;
    add?: AddForm | null;
    remove?: InlineRemove | null;
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
      catalogues={them.catalogues}
      thaoTac={KHONG_LAM_GI}
      loiMayChuNgoaiForm={them.seedError ?? ""}
      dangGui={false}
      edit={them.edit ?? null}
      add={them.add ?? null}
      remove={them.remove ?? null}
    />,
  );
}

/** The one `<input>` tag carrying `name="…"`, whatever the attribute order. */
function inputNamed(html: string, name: string): string {
  return inputs(html).find((b) => b.includes(`name="${name}"`)) ?? "";
}

/** The `<option>`s of one select, as `value|text`. */
function optionsOf(html: string, selectId: string): string[] {
  const select = html.match(new RegExp(`<select[^>]*id="${selectId}"[^>]*>(.*?)</select>`))?.[1] ?? "";
  return [...select.matchAll(/<option value="([^"]*)"[^>]*>([^<]*)<\/option>/g)].map((m) => `${m[1]}|${m[2]}`);
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

  it("câu cuối 'Mặc định {N} giờ.' lấy N từ dòng mặc định phản ánh CỦA XÃ — không ghi cứng (người dùng 09/10/2026, luật 10 cấm #3)", () => {
    const petitionDefault = { ...DEFAULT_ROW, due_soon_hours: 40 };
    const documentDefault = { ...DEFAULT_ROW, id: "01J0000000000000000000DOC", work_kind: "van-ban-den", due_soon_hours: 99 };
    const html = ve({ thoiHan: rowsOf(documentDefault, DONG_SLA, petitionDefault) });
    expect(html).toContain("con số trong thông báo ở chuông. Mặc định 40 giờ.</span>");
    // Not the prototype's figure, not another kind's default, not a field row's (DONG_SLA has 4).
    expect(html).not.toContain("72 giờ");
    expect(html).not.toContain("Mặc định 99 giờ");
    expect(html).not.toContain("Mặc định 4 giờ");
    // A day count would need the commune's working calendar, which this tab does not read.
    expect(html).not.toContain("tức ba ngày");
  });

  it("chưa có dòng mặc định phản ánh → không có câu 'Mặc định … giờ' (không bịa số)", () => {
    const html = ve({ thoiHan: rowsOf(DONG_SLA) });
    expect(html).toContain("con số trong thông báo ở chuông.</span>");
    expect(html).not.toMatch(/Mặc định \d+ giờ/);
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
  it("nút 'Thêm thời hạn cho một lĩnh vực' BẤM ĐƯỢC, không còn dấu '?' (ADR 0079 lô 2 Q4)", () => {
    const html = ve();

    expect(html).not.toContain(pendingMarkerLabel("Thêm thời hạn cho một lĩnh vực"));
    const button = html.match(/<button[^>]*>(?:(?!<\/button>).)*Thêm thời hạn cho một lĩnh vực<\/button>/)?.[0] ?? "";
    expect(button).not.toBe("");
    expect(button).not.toContain('disabled=""');
    expect(button).toContain('aria-expanded="false"');
    expect(html).not.toContain("lĩnh vực mới");
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

  describe("nút gieo cạnh câu 'thiếu dòng mặc định' (ADR 0079 lô 5 Q14)", () => {
    const cau =
      "Loại việc “phan-anh” có dòng riêng nhưng thiếu dòng mặc định, nên lĩnh vực nào không có " +
      "dòng riêng sẽ không tính được hạn.";
    const missing = (...kinds: string[]) =>
      ok<identity_danhSachSLARa>({
        items: [DONG_SLA],
        problems: kinds.map((k) => ({ kind: "missing_default_row", work_kind: k, message: cau })),
      });
    /** The block holding the sentence, up to the table. */
    const block = (html: string) => html.slice(html.indexOf(cau), html.indexOf("<table"));

    it("máy chủ báo thiếu dòng mặc định → nút 'Gieo thời hạn mặc định' đứng NGAY CẠNH câu ấy", () => {
      const html = ve({ thoiHan: missing("phan-anh") });
      const near = block(html);

      expect(near).toMatch(/<button[^>]*type="button"[^>]*>(?:(?!<\/button>).)*Gieo thời hạn mặc định<\/button>/);
      expect(html.match(/Gieo thời hạn mặc định/g)).toHaveLength(1);
    });

    it("hai loại việc cùng thiếu → vẫn MỘT nút: một lần gieo thêm đủ mọi dòng mặc định còn thiếu", () => {
      const html = ve({ thoiHan: missing("phan-anh", "nhiem-vu") });
      expect(html.match(/Gieo thời hạn mặc định<\/button>/g)).toHaveLength(1);
    });

    it("đang gửi → nút khoá", () => {
      const html = renderToStaticMarkup(
        <ManThoiHanXuLy
          du={{ thoiHan: missing("phan-anh") }}
          coQuyenGhi
          thaoTac={KHONG_LAM_GI}
          loiMayChuNgoaiForm=""
          dangGui
        />,
      );
      expect(html).toMatch(/<button[^>]*disabled=""[^>]*>(?:(?!<\/button>).)*Gieo thời hạn mặc định<\/button>/);
    });

    it("CA BỊ TỪ CHỐI: thiếu `admin.sla` → câu vẫn hiện, KHÔNG có nút", () => {
      const html = ve({ thoiHan: missing("phan-anh") }, { coQuyenGhi: false });
      expect(html).toContain(cau);
      expect(html).not.toContain("Gieo thời hạn mặc định");
    });

    it("vấn đề loại khác (không phải missing_default_row) → KHÔNG có nút", () => {
      const html = ve({
        thoiHan: ok<identity_danhSachSLARa>({
          items: [DONG_SLA],
          problems: [{ kind: "something_else", work_kind: "", message: "Cấu hình có vấn đề." }],
        }),
      });
      expect(html).toContain("Cấu hình có vấn đề.");
      expect(html).not.toContain("Gieo thời hạn mặc định");
    });
  });

  it("dòng mặc định nói rõ nó là mặc định, chữ mờ", () => {
    expect(ve({ thoiHan: rowsOf(DEFAULT_ROW) })).toContain(
      '<td class="whitespace-normal! text-ink-muted">Mặc định cho mọi lĩnh vực</td>',
    );
  });

  it("dòng `don-thu` đọc là 'Đơn thư', chữ navy đậm, không hiện mã thô", () => {
    const html = ve({ thoiHan: rowsOf({ ...DEFAULT_ROW, work_kind: "don-thu" }) });

    expect(html).toContain('<td class="text-navy font-medium">Đơn thư</td>');
    expect(html).not.toContain("don-thu");
  });

  it("nhịp dọc space-y-3 (`SlaTable.tsx:67`); đầu cột MỘT DÒNG với độ rộng cố định, chỉ ô Lĩnh vực xuống dòng (người dùng, 09/10/2026)", () => {
    const html = ve();
    expect(html).toMatch(/^<section class="space-y-3"/);
    expect(html).not.toContain("[&amp;_th]:whitespace-normal");
    expect(html).toContain('<th scope="col" class="w-[96px]">Tiếp nhận</th>');
    expect(html).toContain('<th scope="col" class="w-[96px]">Xử lý xong</th>');
    expect(html).toContain('<th scope="col" class="w-[152px]">Sắp đến hạn khi còn</th>');
    expect(html).toContain('<th scope="col" class="w-[172px]">Báo lãnh đạo trực tiếp</th>');
    expect(html).toContain('<th scope="col" class="w-[104px]">Báo Chủ tịch</th>');
    expect(html).toContain('<th scope="col" class="w-[148px]">Giữ chưa phân công</th>');
    // No catalogue passed → the raw code, in the one cell allowed to wrap.
    expect(html).toContain('<td class="whitespace-normal!">an-ninh-trat-tu</td>');
  });

  it("thứ tự dòng: Văn bản đến → Đơn thư → Phản ánh (Mặc định đầu, rồi nhãn A→Z) → Nhiệm vụ (người dùng, 09/10/2026)", () => {
    const row = (id: string, work_kind: string, field: string): identity_dongSLARa => ({
      ...DONG_SLA,
      id,
      work_kind,
      field,
      is_default: field === "",
    });
    const catalogues: SlaFieldCatalogues = {
      ...CATALOGUES,
      "phan-anh": {
        ok: true,
        duLieu: [option("rac-thai", "Rác thải – Vệ sinh môi trường"), option("an-ninh", "An ninh trật tự"), option("dien", "Điện")],
      },
    };
    // Server order deliberately scrambled.
    const html = ve(
      {
        thoiHan: rowsOf(
          row("01", "nhiem-vu", ""),
          row("02", "phan-anh", "rac-thai"),
          row("03", "don-thu", ""),
          row("04", "phan-anh", "dien"),
          row("05", "phan-anh", ""),
          row("06", "van-ban-den", ""),
          row("07", "phan-anh", "an-ninh"),
        ),
      },
      { catalogues },
    );
    const cells = [...html.matchAll(/<tr><td class="text-navy font-medium">([^<]+)<\/td><td[^>]*>([^<]+)<\/td>/g)].map(
      (m) => `${m[1]} | ${m[2]}`,
    );
    expect(cells).toEqual([
      "Văn bản đến | Mặc định cho mọi lĩnh vực",
      "Đơn thư | Mặc định cho mọi lĩnh vực",
      "Phản ánh của người dân | Mặc định cho mọi lĩnh vực",
      "Phản ánh của người dân | An ninh trật tự",
      // Vietnamese collation: "Điện" sorts after "D…" and before "R…".
      "Phản ánh của người dân | Điện",
      "Phản ánh của người dân | Rác thải – Vệ sinh môi trường",
      "Nhiệm vụ | Mặc định cho mọi lĩnh vực",
    ]);
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
    expect(html).not.toContain('title="Xoá thời hạn riêng"');
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

describe("xoá thời hạn riêng (ADR 0079 lô 2 Q4)", () => {
  it("dòng có lĩnh vực: nút thùng rác BẤM ĐƯỢC, không còn dấu '?'", () => {
    const html = ve();

    expect(html).not.toContain(pendingMarkerLabel("Xoá thời hạn riêng"));
    const trash = html.match(/<button[^>]*title="Xoá thời hạn riêng"[^>]*>/)?.[0] ?? "";
    expect(trash).not.toBe("");
    expect(trash).not.toContain('disabled=""');
    expect(trash).toContain('aria-label="Xoá thời hạn riêng Phản ánh của người dân — an-ninh-trat-tu"');
  });

  it("dòng mặc định: KHÔNG có nút xoá — mọi lĩnh vực chưa có dòng riêng dựa vào nó", () => {
    const html = ve({ thoiHan: rowsOf(DEFAULT_ROW) });
    expect(html).not.toContain("Xoá thời hạn riêng");
    expect(html).toContain('title="Sửa thời hạn"');
  });

  it("bước lý do: ô lý do + Xoá + Huỷ trong ô thao tác, thay bút và thùng rác của dòng ấy", () => {
    const html = ve({ thoiHan: rowsOf(DONG_SLA, DEFAULT_ROW) }, { remove: removeOf(DONG_SLA) });

    expect(inputNamed(html, "reason")).toContain('placeholder="Lý do xoá"');
    expect(html).toContain('aria-label="Lý do xoá — Phản ánh của người dân — an-ninh-trat-tu"');
    expect(html).toMatch(/<button[^>]*>Xoá<\/button>/);
    expect(html).toMatch(/<button[^>]*>Huỷ<\/button>/);
    expect(html).not.toContain('title="Xoá thời hạn riêng"');
    // The other row keeps its pencil; the figures stay text, not boxes.
    expect(html).toContain('aria-label="Sửa thời hạn Phản ánh của người dân — Mặc định cho mọi lĩnh vực"');
    expect(inputs(html)).toHaveLength(1);
  });

  it("thiếu lý do và câu máy chủ (409 dòng mặc định) hiện ngay dưới dòng, tách hai câu", () => {
    const server = "Không xoá được thời hạn mặc định — mọi lĩnh vực chưa có quy định riêng đều dựa vào nó.";
    const html = ve({}, { remove: removeOf(DONG_SLA, { localError: "Hãy nêu lý do xoá.", serverError: server }) });

    expect(html).toContain('<p role="alert" class="text-danger m-0 text-[12.5px]">Hãy nêu lý do xoá.</p>');
    expect(html).toContain(`<p role="alert" class="text-danger m-0 text-[12.5px]">${server}</p>`);
  });

  it("một `remove` trỏ vào dòng mặc định vẫn không vẽ bước xoá", () => {
    const html = ve({ thoiHan: rowsOf(DEFAULT_ROW) }, { remove: removeOf(DEFAULT_ROW) });
    expect(html).not.toContain('name="reason"');
  });

  it("đang xoá: nút báo bận, ô lý do khoá", () => {
    const html = ve({}, { remove: removeOf(DONG_SLA, { busy: true, reason: "Trùng" }) });
    expect(html).toContain("Đang xoá…");
    expect(inputNamed(html, "reason")).toContain('disabled=""');
  });
});

describe("hàng thêm thời hạn cho một lĩnh vực (spec 08)", () => {
  it("hàng xám ConfigFormRow, lưới 14rem_1fr_8rem_8rem_auto từ sm; nút Thêm báo đang mở", () => {
    const html = ve({}, { add: addOf(), catalogues: CATALOGUES });

    expect(html).toMatch(/<form[^>]*class="[^"]*bg-background[^"]*sm:grid-cols-\[14rem_1fr_8rem_8rem_auto\]/);
    expect(html).toContain('aria-expanded="true"');
    expect(html).toContain('aria-controls="sla-add-row"');
    expect(html).toContain('id="sla-add-row"');
  });

  it("Loại việc: đúng ba loại, KHÔNG có Đơn thư (ADR 0079 lô 3)", () => {
    expect(optionsOf(ve({}, { add: addOf() }), "sla-add-kind")).toEqual([
      "phan-anh|Phản ánh của người dân",
      "van-ban-den|Văn bản đến",
      "nhiem-vu|Nhiệm vụ",
    ]);
  });

  it("Lĩnh vực: '— Chọn lĩnh vực —' rồi các mục CÒN DÙNG của đúng danh mục loại việc", () => {
    const petition = optionsOf(ve({}, { add: addOf(), catalogues: CATALOGUES }), "sla-add-field");
    expect(petition).toEqual(["|— Chọn lĩnh vực —", "an-ninh-trat-tu|An ninh, trật tự"]);

    const document = optionsOf(
      ve({}, { add: addOf({ draft: { ...newAddDraft(), workKind: "van-ban-den" } }), catalogues: CATALOGUES }),
      "sla-add-field",
    );
    expect(document).toEqual(["|— Chọn lĩnh vực —", "cong-van|Công văn"]);

    const task = optionsOf(
      ve({}, { add: addOf({ draft: { ...newAddDraft(), workKind: "nhiem-vu" } }), catalogues: CATALOGUES }),
      "sla-add-field",
    );
    // Scale order kept as the server gives it.
    expect(task).toEqual(["|— Chọn lĩnh vực —", "khan|Khẩn", "thuong|Thường"]);
  });

  it("Tiếp nhận 8, Xử lý xong 48 khi mở; nút Thêm / Huỷ", () => {
    const html = ve({}, { add: addOf() });

    expect(inputNamed(html, "acknowledge_hours")).toContain('value="8"');
    const resolve = inputNamed(html, "resolve_hours");
    expect(resolve).toContain('value="48"');
    expect(resolve).toContain('min="1"');
    expect(html).toContain("Tiếp nhận (giờ)");
    expect(html).toContain("Xử lý xong (giờ)");
    expect(html).toMatch(/<button[^>]*type="submit"[^>]*>Thêm<\/button>/);
  });

  it("lỗi tại chỗ và câu máy chủ hiện TRONG hàng thêm, tách hai câu", () => {
    const server = "Lĩnh vực này đã có thời hạn riêng — hãy sửa dòng sẵn có.";
    const html = ve({}, { add: addOf({ localError: FIELD_REQUIRED_ERROR, serverError: server }) });

    expect(html).toContain(`<p role="alert" class="text-danger col-span-full m-0 text-[12.5px]">${FIELD_REQUIRED_ERROR}</p>`);
    expect(html).toContain(`<p role="alert" class="text-danger col-span-full m-0 text-[12.5px]">${server}</p>`);
    expect(html).toMatch(/<select[^>]*id="sla-add-field"[^>]*aria-invalid="true"/);
  });

  it("danh mục không đọc được (thiếu `admin.lookup`) → nói lý do, không để một ô chọn rỗng câm", () => {
    const html = ve(
      {},
      { add: addOf(), catalogues: { ...CATALOGUES, "phan-anh": { ok: false, thongBao: PETITION_FIELDS_NOT_READABLE } } },
    );
    expect(html).toContain(PETITION_FIELDS_NOT_READABLE);
    expect(optionsOf(html, "sla-add-field")).toEqual(["|— Chọn lĩnh vực —"]);
  });

  it("CA BỊ TỪ CHỐI: thiếu `admin.sla` thì một `add` lọt vào vẫn không vẽ hàng thêm", () => {
    const html = ve({}, { coQuyenGhi: false, add: addOf() });
    expect(html).not.toContain("<form");
    expect(html).not.toContain("Lĩnh vực áp dụng");
  });

  it("đang gửi: nút báo 'Đang thêm…', mọi ô khoá", () => {
    const html = ve({}, { add: addOf({ busy: true }) });
    expect(html).toContain("Đang thêm…");
    for (const b of inputs(html)) expect(b).toContain('disabled=""');
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
    const html = ve({}, { catalogues: CATALOGUES });
    expect(html).toContain("An ninh, trật tự");
    expect(html).not.toContain(">an-ninh-trat-tu<");
    expect(html).toContain('aria-label="Sửa thời hạn Phản ánh của người dân — An ninh, trật tự"');
    expect(html).not.toMatch(/câu mở #4|câu hỏi mở #4/);
  });

  it("không đọc được nhãn → hiện nguyên mã", () => {
    expect(ve()).toContain(">an-ninh-trat-tu<");
  });

  it("dòng văn bản đến và nhiệm vụ hiện NHÃN loại văn bản / mức ưu tiên, không mã thô", () => {
    const html = ve(
      {
        thoiHan: rowsOf(
          { ...DONG_SLA, id: "01J00000000000000000000VB", work_kind: "van-ban-den", field: "cong-van" },
          { ...DONG_SLA, id: "01J00000000000000000000NV", work_kind: "nhiem-vu", field: "khan" },
        ),
      },
      { catalogues: CATALOGUES },
    );
    expect(html).toContain('<td class="whitespace-normal!">Công văn</td>');
    expect(html).toContain('<td class="whitespace-normal!">Khẩn</td>');
    expect(html).not.toContain(">cong-van<");
    expect(html).not.toContain(">khan<");
  });

  it("nhãn tra theo ĐÚNG loại việc của dòng: một mã trùng ở danh mục khác không mượn nhãn", () => {
    const html = ve(
      { thoiHan: rowsOf({ ...DONG_SLA, work_kind: "nhiem-vu", field: "an-ninh-trat-tu" }) },
      { catalogues: CATALOGUES },
    );
    expect(html).toContain(">an-ninh-trat-tu<");
    expect(html).not.toContain("An ninh, trật tự");
  });

  it("slaFieldLabelReadDecision: chỉ admin.lookup mới đọc danh mục lĩnh vực; admin.sla thôi thì không", () => {
    expect(slaFieldLabelReadDecision(phienVoi(["admin.sla", "admin.lookup"])).hien).toBe(true);
    expect(slaFieldLabelReadDecision(phienVoi(["admin.sla"])).hien).toBe(false);
  });
});
