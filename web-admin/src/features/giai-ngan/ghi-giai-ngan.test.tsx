import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { finance_chungTuRa, finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";

import { BangChungTu, FormChungTu, giaTriTuChungTu, KhoiChungTu, type ProjectVoucherList } from "./chung-tu-du-an";
import { FormDuAn, giaTriTuDuAn, KhoiThemDuAn, ProjectHeaderActions, type FundingCatalogue } from "./ghi-du-an";
import {
  CANH_BAO_SUA_VE_NHAP,
  CAU_THIEU_QUYEN_GHI,
  CAU_THIEU_QUYEN_XAC_NHAN,
  CHUNG_TU_DA_KHOA,
  CHUNG_TU_DA_XAC_NHAN,
  CHUNG_TU_KE_TOAN_NHAP,
  FORM_CHUNG_TU_TRONG,
  FORM_DU_AN_TRONG,
  PHAN_CHUA_DUNG_GHI,
} from "./nhan-ghi-giai-ngan";
import { BangDanhSach } from "./bang-du-an";
import { AttentionIssuesPending, DisbursementHeaderActions, ProjectRecordTabs } from "./pending-parts";

/** The server's list of a project with no voucher. */
const EMPTY_LIST: ProjectVoucherList = { phase: "ready", items: [], count: 0 };

/** The funding catalogue of a commune that declared no source yet. */
const NO_SOURCES: FundingCatalogue = { phase: "ready", items: [] };

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — bổ cho `nhan-ghi-giai-ngan.test.ts`, vốn chỉ
 * canh quyết định bên trong hàm.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * NHÓM QUAN TRỌNG NHẤT CỦA TỆP NÀY LÀ RANH GIỚI `budget.update` / `budget.confirm`.
 *
 * Tài khoản của người viết mã luôn có cả hai khoá, nên nhánh "chỉ có quyền nhập liệu" là nhánh
 * KHÔNG AI NHÌN THẤY trong lúc phát triển — và nó là nhánh phải đúng. Gộp hai khoá làm một không
 * làm đỏ một bài kiểm chức năng nào, không làm đỏ `tsc`, và không có gì trên màn hình nói ra: máy
 * chủ vẫn 403 đúng lúc bấm, tức là sau khi cán bộ đã tin là mình làm được việc ấy.
 *
 * Nên bốn nút `budget.confirm` (Xác nhận · Khoá · Mở khoá · Gỡ) được kiểm RIÊNG với một tài khoản
 * chỉ có `budget.update`, và ngược lại.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

/**
 * Chuỗi như nó THẬT SỰ nằm trong HTML.
 *
 * ⚠ HÀM NÀY ĐẾN TỪ MỘT LẦN XANH SAI ĐÃ ĐO Ở MÀN THU - CHI, không từ sự cẩn thận:
 * `renderToStaticMarkup` thoát `"` thành `&quot;` và `'` thành `&#x27;`, nên một phép so với chuỗi
 * THÔ đi qua `not.toContain` sẽ XANH kể cả khi cái nút ấy đang nằm chình ình trên trang.
 */
function nhuTrongHTML(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
}

function chungTu(sua: Partial<finance_chungTuRa> = {}): finance_chungTuRa {
  return {
    id: "01JCT1",
    project_id: "01JDA1",
    payment_date: "2026-09-07",
    amount: 30000000,
    description: "Thanh toán đợt 3",
    status: CHUNG_TU_KE_TOAN_NHAP,
    entered_by: "CB-00123",
    unlock_count: 0,
    ...sua,
  };
}

const NGAY = "07/09/2026";
// Presentational pins (ADR 0068 §5): the `✎` / `🗑` glyphs left the accessible names when they became
// lucide icons; the names still carry the action and the voucher's date, so each still names ONE
// button and the permission assertions below are unchanged.
const NHAN_SUA = `Sửa chứng từ ngày ${NGAY}`;
const NHAN_XAC_NHAN = `Xác nhận chứng từ ngày ${NGAY}`;
const NHAN_KHOA = `Khoá chứng từ ngày ${NGAY}`;
const NHAN_MO_KHOA = `Mở khoá chứng từ ngày ${NGAY}`;
const NHAN_GO = `Gỡ chứng từ ngày ${NGAY}`;

function veBang(coGhi: boolean, coXacNhan: boolean, ds: readonly finance_chungTuRa[]): string {
  return renderToStaticMarkup(
    <BangChungTu
      ds={ds}
      coGhi={coGhi}
      coXacNhan={coXacNhan}
      dangGui={false}
      moSua={() => {}}
      moGo={() => {}}
      moMoKhoa={() => {}}
      xacNhan={() => {}}
      khoa={() => {}}
    />,
  );
}

const DU_AN: finance_duAnRa = {
  id: "01JDA1",
  code: "DA-2026-be-tong-hoa-duong-ngo-xo-2",
  year: 2026,
  category_id: "01JHM1",
  name: "Bê tông hóa đường ngõ xóm tổ 6",
  planned_amount: 100000000,
  approved_amount: 100000000,
  disbursed_amount: 90000000,
  remaining_amount: 10000000,
  disbursed_ratio: 9000,
  delay_score: null,
  is_delayed: false,
  disbursement_deadline: "2026-12-31",
  delay_threshold: 1000,
  delay_threshold_source: "mac-dinh",
};

const HANG_MUC: finance_hangMucRa[] = [
  {
    id: "01JHM1",
    code: "chuyen-tiep",
    label: "Các công trình chuyển tiếp",
    is_default: true,
    active: true,
    order: 1,
    source: "he-thong",
    tier: 1,
  },
];

describe("BANG CHỨNG TỪ — ranh giới budget.update / budget.confirm", () => {
  it("CHỈ ĐỌC (không khoá nào): KHÔNG nút ghi nào ra trang, nhưng CON SỐ vẫn hiện", () => {
    const html = veBang(false, false, [chungTu()]);

    expect(html).not.toContain(nhuTrongHTML(NHAN_SUA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_XAC_NHAN));
    expect(html).not.toContain(nhuTrongHTML(NHAN_KHOA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_GO));
    // Người không ghi được vẫn phải ĐỌC được chứng từ và trạng thái của nó.
    expect(html).toContain("30.000.000 đ");
    expect(html).toContain("Kế toán nhập");
  });

  it("`budget.update` MỘT MÌNH mở SỬA và KHÔNG mở xác nhận · khoá · gỡ", () => {
    // BÀI KIỂM ĐẮT NHẤT CỦA TỆP. Xác nhận là hành vi của người chịu trách nhiệm, gỡ lấy một khoản
    // tiền ra khỏi tổng lãnh đạo đã đọc — cả hai đứng sau `budget.confirm` ở máy chủ. Gắn chúng vào
    // quyền nhập liệu là xoá mất ranh giới xã dựng ra giữa hai con người.
    const html = veBang(true, false, [chungTu()]);

    expect(html).toContain(nhuTrongHTML(NHAN_SUA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_XAC_NHAN));
    expect(html).not.toContain(nhuTrongHTML(NHAN_KHOA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_GO));
  });

  it("`budget.confirm` MỘT MÌNH mở xác nhận và gỡ, KHÔNG mở sửa", () => {
    const html = veBang(false, true, [chungTu()]);

    expect(html).toContain(nhuTrongHTML(NHAN_XAC_NHAN));
    expect(html).toContain(nhuTrongHTML(NHAN_GO));
    expect(html).not.toContain(nhuTrongHTML(NHAN_SUA));
  });
});

describe("BANG CHỨNG TỪ — vòng đời quyết định nút nào có nghĩa", () => {
  it("`Kế toán nhập`: có Xác nhận, KHÔNG có Khoá (chưa xác nhận thì chưa khoá được)", () => {
    const html = veBang(true, true, [chungTu()]);

    expect(html).toContain(nhuTrongHTML(NHAN_XAC_NHAN));
    expect(html).not.toContain(nhuTrongHTML(NHAN_KHOA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_MO_KHOA));
  });

  it("`Đã xác nhận`: có Khoá và vẫn có Sửa, KHÔNG có Xác nhận lại", () => {
    const html = veBang(true, true, [chungTu({ status: CHUNG_TU_DA_XAC_NHAN })]);

    expect(html).toContain(nhuTrongHTML(NHAN_KHOA));
    expect(html).toContain(nhuTrongHTML(NHAN_SUA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_XAC_NHAN));
  });

  it("`Đã khoá`: CHỈ Mở khoá — không sửa, không gỡ, dù có đủ cả hai khoá quyền", () => {
    const html = veBang(true, true, [
      chungTu({ status: CHUNG_TU_DA_KHOA, locked_at: "2026-09-22T07:05:00Z", unlock_count: 2 }),
    ]);

    expect(html).toContain(nhuTrongHTML(NHAN_MO_KHOA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_SUA));
    expect(html).not.toContain(nhuTrongHTML(NHAN_GO));
    // Mốc khoá in theo giờ Việt Nam dù máy chạy `TZ=UTC`, và số lần mở khoá là một CON SỐ có nghĩa.
    expect(html).toContain("14:05 22/09/2026");
    expect(html).toContain("Đã mở khoá 2 lần");
  });

  it("trạng thái LẠ: không nút nào, kể cả với đủ quyền — fail closed đi ra tới trang", () => {
    const html = veBang(true, true, [chungTu({ status: "da-quyet-toan" })]);

    for (const nhan of [NHAN_SUA, NHAN_XAC_NHAN, NHAN_KHOA, NHAN_MO_KHOA, NHAN_GO]) {
      expect(html).not.toContain(nhuTrongHTML(nhan));
    }
    // Vẫn hiện nguyên chuỗi máy chủ gửi, để người đọc thấy có gì đó lệch.
    expect(html).toContain("da-quyet-toan");
  });

  it("bảng rỗng là danh sách CỦA MÁY CHỦ: nói thẳng dự án chưa có chứng từ, không còn câu 'phiên làm việc'", () => {
    const html = veBang(true, true, []);

    expect(html).toContain("Dự án này chưa có chứng từ giải ngân nào.");
    expect(html).not.toContain("phiên làm việc");
    expect(html).not.toContain("chưa xem lại được");
  });

  it("cột NGUỒN VỐN: tên nguồn máy chủ trả, hoặc '—' — không còn dấu '?'", () => {
    const html = veBang(true, true, [
      chungTu({ id: "A", funding_source_name: "Ngân sách tỉnh" }),
      chungTu({ id: "B" }),
    ]);

    expect(html).toContain("<td class=\"whitespace-normal\">Ngân sách tỉnh</td>");
    expect(html).toContain("<td class=\"whitespace-normal\">—</td>");
    expect(html).toContain('<th scope="col">Nguồn vốn</th>');
    expect(html).not.toContain("data-pending");
  });
});

describe("BIỂU MẪU CHỨNG TỪ — cảnh báo ADR 0036", () => {
  it("sửa một chứng từ ĐÃ XÁC NHẬN: cảnh báo 'về Kế toán nhập' hiện NGAY TRÊN các ô", () => {
    const html = renderToStaticMarkup(
      <FormChungTu
        allocations={[]}
        tieuDeForm="Sửa chứng từ"
        giaTriDau={giaTriTuChungTu(chungTu({ status: CHUNG_TU_DA_XAC_NHAN }))}
        trangThaiHienTai={CHUNG_TU_DA_XAC_NHAN}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain(nhuTrongHTML(CANH_BAO_SUA_VE_NHAP));
  });

  it("thêm mới hoặc sửa chứng từ CHƯA xác nhận thì KHÔNG hiện cảnh báo ấy", () => {
    const them = renderToStaticMarkup(
      <FormChungTu
        allocations={[]}
        tieuDeForm="Ghi nhận khoản chi"
        giaTriDau={FORM_CHUNG_TU_TRONG}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    const suaNhap = renderToStaticMarkup(
      <FormChungTu
        allocations={[]}
        tieuDeForm="Sửa chứng từ"
        giaTriDau={giaTriTuChungTu(chungTu())}
        trangThaiHienTai={CHUNG_TU_KE_TOAN_NHAP}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    // Câu cảnh báo phải đúng NGỮ CẢNH của nó: hiện ở mọi nơi thì người ta thôi đọc nó.
    expect(them).not.toContain(nhuTrongHTML(CANH_BAO_SUA_VE_NHAP));
    expect(suaNhap).not.toContain(nhuTrongHTML(CANH_BAO_SUA_VE_NHAP));
  });

  it("KHÔNG có ô `Trạng thái` ở biểu mẫu — vòng đời không do client đặt", () => {
    const html = renderToStaticMarkup(
      <FormChungTu
        allocations={[]}
        tieuDeForm="Ghi nhận khoản chi"
        giaTriDau={FORM_CHUNG_TU_TRONG}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).not.toContain('name="trang-thai');
    expect(html).not.toContain("da-khoa");
  });
});

/**
 * The three project buttons by their VISIBLE WORDS, as text between tags. Presentational pins (ADR
 * 0068 §5): the `+` / `✎` / `🗑` glyphs became lucide icons in front of the same words. Matched as
 * `>Words<` so the words inside a refusal sentence or a form title can never satisfy the check.
 */
const NUT_THEM_DU_AN = ">Thêm dự án<";
const NUT_SUA_DU_AN = ">Sửa dự án<";
const NUT_GO_DU_AN = ">Gỡ dự án<";

describe("NÚT THÊM DỰ ÁN — cổng budget.update", () => {
  it("thiếu `budget.update`: KHÔNG có nút, và không gì khác thay chỗ nó", () => {
    // Presentational change (ADR 0068 lần 5): the prototype draws nothing for an account without the
    // key (`canRecord`), so the refusal sentence left the header. The permission is still named, by
    // its Phân quyền NAME, in the empty list (`nhanNamRong`).
    const html = renderToStaticMarkup(
      <KhoiThemDuAn nam={2026} danhMuc={HANG_MUC} coGhi={false} daGhiXong={() => {}} />,
    );

    expect(html).toBe("");
  });

  it("có `budget.update`: nút hiện, và hộp thoại CHƯA mở", () => {
    const html = renderToStaticMarkup(
      <KhoiThemDuAn nam={2026} danhMuc={HANG_MUC} coGhi daGhiXong={() => {}} />,
    );

    expect(html).toContain(NUT_THEM_DU_AN);
    expect(html).toContain('aria-haspopup="dialog"');
    expect(html).not.toContain("<dialog");
  });
});

/** Header buttons of the project card for `budget.update` held or not. */
function renderProjectButtons(canRecord: boolean, editing = false): string {
  return renderToStaticMarkup(
    <ProjectHeaderActions
      coGhi={canRecord}
      editing={editing}
      onToggleEdit={() => {}}
      onRemove={() => {}}
    />,
  );
}

describe("SỬA / GỠ DỰ ÁN — cùng cổng budget.update (quyết định 06/10/2026)", () => {
  it("`budget.update`: có Sửa dự án VÀ Gỡ dự án", () => {
    const html = renderProjectButtons(true);
    expect(html).toContain(NUT_SUA_DU_AN);
    expect(html).toContain(NUT_GO_DU_AN);
  });

  it("thiếu `budget.update` (kể cả khi giữ `budget.confirm`): không nút nào", () => {
    expect(renderProjectButtons(false)).toBe("");
  });

  it("đang sửa: nút đọc 'Đang sửa' và báo đang mở", () => {
    const html = renderProjectButtons(true, true);
    expect(html).toContain(">Đang sửa<");
    expect(html).toContain('aria-expanded="true"');
  });
});

describe("TAB CHỨNG TỪ — câu từ chối gọi đúng TÊN quyền", () => {
  it("thiếu cả hai khoá: hai câu, bằng tên trên màn Phân quyền, không bằng khoá máy", () => {
    const html = renderToStaticMarkup(
      <KhoiChungTu
        duAnID={DU_AN.id}
        allocations={[]}
        vouchers={EMPTY_LIST}
        coGhi={false}
        coXacNhan={false}
        daGhiXong={() => {}}
      />,
    );
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_GHI));
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_XAC_NHAN));
    expect(html).not.toContain(">Ghi nhận khoản chi<");
    expect(html).not.toContain("budget.update");
    expect(html).not.toContain("budget.confirm");
  });

  it("có `budget.update`: nút Ghi nhận khoản chi hiện", () => {
    const html = renderToStaticMarkup(
      <KhoiChungTu duAnID={DU_AN.id} allocations={[]} vouchers={EMPTY_LIST} coGhi coXacNhan={false} daGhiXong={() => {}} />,
    );
    expect(html).toContain(">Ghi nhận khoản chi<");
  });
});

describe("BIỂU MẪU DỰ ÁN", () => {
  it("danh mục hạng mục RỖNG: nói trước, và nút Lưu bị tắt", () => {
    const html = renderToStaticMarkup(
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={[]}
        fundingCatalogue={NO_SOURCES}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain("Danh mục hạng mục kế hoạch vốn của xã đang rỗng");
    // Gửi đi lúc ấy chỉ nhận 404 "không tìm thấy hạng mục kế hoạch vốn này trong xã". Phép so nhắm
    // ĐÚNG nút gửi chứ không tìm chữ `disabled` ở đâu đó — ô chọn hạng mục cũng đang bị tắt, nên
    // một phép so lỏng sẽ xanh kể cả khi nút Lưu vẫn bấm được.
    // Presentational pin (ADR 0068 §5): the button now carries the `Button` utilities after its
    // legacy `nut-chinh` class; what is asserted is still THE SUBMIT BUTTON, disabled.
    // The add form's submit reads "Thêm dự án", the edit form's "Lưu dự án" (prototype `:672`).
    expect(html).toMatch(/<button class="nut-chinh [^"]*" type="submit" disabled="">Thêm dự án<\/button>/);
  });

  it("khi SỬA: mã dự án chỉ ĐỌC, không có ô nhập mã", () => {
    const html = renderToStaticMarkup(
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Sửa dự án"
        giaTriDau={giaTriTuDuAn(DU_AN)}
        maChiDoc={DU_AN.code}
        danhMuc={HANG_MUC}
        fundingCatalogue={NO_SOURCES}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    // Mã đã cấp thì không đánh lại (luật 7, cấm #4) — một ô nhập ở đây là một lời mời sửa nó.
    expect(html).not.toContain('id="ma-du-an"');
    expect(html).toContain(DU_AN.code);
  });

  it("khi THÊM: có ô mã, và nói rõ mã đã cấp thì không cấp lại", () => {
    const html = renderToStaticMarkup(
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={HANG_MUC}
        fundingCatalogue={NO_SOURCES}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain('id="ma-du-an"');
    expect(html).toContain("kể cả bởi dự án đã rút khỏi danh sách");
    // §9: `Tự sinh mã` checked by default, the code box disabled and saying who fills it.
    expect(html).toMatch(/<input id="tu-sinh-ma-du-an" type="checkbox"[^>]* checked=""/);
    expect(html).toMatch(/<input id="ma-du-an"[^>]* disabled=""[^>]* placeholder="Hệ thống cấp khi lưu"/);
  });

  it("giá trị ban đầu của biểu mẫu SỬA là SỐ THÔ, không phải chuỗi đã định dạng", () => {
    // `100.000.000` trong ô nhập làm phép so "có đổi không" thấy khác và gửi một `PATCH` thừa.
    expect(giaTriTuDuAn(DU_AN).keHoachVon).toBe("100000000");
    expect(giaTriTuChungTu(chungTu()).soTien).toBe("30000000");
  });
});

/** Every "?" of the Giải ngân screens, server-rendered — the spots ADR 0068 §14 approved. */
function allPlaceholders(): string {
  return [
    renderToStaticMarkup(<DisbursementHeaderActions />),
    renderToStaticMarkup(<AttentionIssuesPending />),
    renderToStaticMarkup(
      <BangDanhSach
        duLieu={{ items: [DU_AN], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }}
        danhMuc={HANG_MUC}
      />,
    ),
    renderToStaticMarkup(<ProjectRecordTabs chart={null}>panel</ProjectRecordTabs>),
    renderToStaticMarkup(
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={HANG_MUC}
        fundingCatalogue={NO_SOURCES}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    ),
    veBang(true, true, [chungTu()]),
  ].join("\n");
}

describe("PHẦN CHƯA DỰNG — dấu '?' đúng vị trí đặc tả (ADR 0068 §14)", () => {
  it("MỌI mục có một dấu '?' trên màn, mang đúng tên của mục ấy", () => {
    const html = allPlaceholders();
    for (const p of PHAN_CHUA_DUNG_GHI) {
      expect(html).toContain(`aria-label="${nhuTrongHTML(pendingMarkerLabel(p.ten))}"`);
    }
    // The description opens only when "?" is pressed: no sentence is printed on the page.
    for (const p of PHAN_CHUA_DUNG_GHI) {
      expect(html).not.toContain(nhuTrongHTML(p.viSao));
    }
  });

  it("khối gập 'N phần chưa dựng' không còn trên màn", () => {
    const html = allPlaceholders();
    expect(html).not.toContain("phần của bản thiết kế chưa dựng được");
    expect(html).not.toContain("<details");
  });

  it("control giữ chỗ là control THẬT, VÔ HIỆU: nút, ô chọn, tab, ô đánh dấu", () => {
    const header = renderToStaticMarkup(<DisbursementHeaderActions />);
    // `Hạng mục` is LIVE (06/10/2026, `category-manager-dialog.tsx`): no placeholder left here.
    expect(header).not.toContain("Hạng mục");
    expect(header).toMatch(/<button[^>]* disabled=""[^>]*>.*Nhập giải ngân<\/button>/);

    // §7.1 `Chỉ dự án chậm` and `Gộp theo hạng mục` are LIVE since 06/10/2026 (`bang-du-an.tsx`).

    const tabs = renderToStaticMarkup(<ProjectRecordTabs chart={null}>panel</ProjectRecordTabs>);
    for (const name of ["Vướng mắc", "Trao đổi"]) {
      expect(tabs).toMatch(new RegExp(`role="tab" aria-selected="false"[^>]* disabled=""[^>]*>.*${name}<\/button>`));
    }
    // Biểu đồ is live (§8.3) but not the default; Chứng từ is selected and its panel holds the vouchers.
    const chartTab = /<button([^>]*)>(?:(?!<\/button>).)*Biểu đồ<\/button>/.exec(tabs);
    expect(chartTab).not.toBeNull();
    expect(chartTab![1]).toContain('role="tab"');
    expect(chartTab![1]).not.toContain("disabled");
    expect(tabs).toMatch(/role="tab" aria-selected="true"[^>]*>.*Chứng từ<\/button>/);
    expect(tabs).toContain('role="tabpanel"');
  });

  it("biểu mẫu SỬA dự án: không 'Tự sinh mã' (mã đã cấp), nhưng có nguồn vốn và đơn vị như prototype", () => {
    // ADR 0068 lần 5: the prototype's edit form is the add form minus the code (`BudgetItemForm.tsx
    // :318`), so the funding list and the unit / officer selects are there too. The funding list is
    // LIVE since 8245698b (no "?"); the unit / officer selects remain "?" placeholders.
    const html = renderToStaticMarkup(
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Sửa dự án"
        giaTriDau={giaTriTuDuAn(DU_AN)}
        maChiDoc={DU_AN.code}
        danhMuc={HANG_MUC}
        fundingCatalogue={NO_SOURCES}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    const marker = (ten: string) => `aria-label="${nhuTrongHTML(pendingMarkerLabel(ten))}"`;
    expect(html).not.toContain(marker("Tự sinh mã"));
    expect(html).not.toContain('id="tu-sinh-ma-du-an"');
    expect(html).not.toContain(marker("Thêm nguồn vốn cho dự án"));
    expect(html).toContain("data-funding-allocations");
    expect(html).toContain(marker("Đơn vị thực hiện và Cán bộ phụ trách"));
  });

  it("dòng phụ 'vướng mắc · nguy cơ' giữ chỗ KHÔNG in con số nào — '0 vướng mắc' sẽ đọc thành 'không có'", () => {
    const html = renderToStaticMarkup(<AttentionIssuesPending />);
    // The words on screen, not the class names (`gap-1.5`, `size-[18px]`).
    expect(html.replace(/<[^>]*>/g, "")).not.toMatch(/\d/);
    expect(html).toContain("data-pending");
  });
});
