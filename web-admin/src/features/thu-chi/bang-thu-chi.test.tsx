import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type {
  finance_bangDayDuRa,
  finance_chiSoNamRa,
  finance_cotRa,
  finance_cotVao,
  finance_danhSachDotRa,
  finance_dongRa,
  finance_tomTatRa,
} from "@/lib/api/schema.gen";

import {
  BangDayDu,
  DongKhoanMuc,
  FormDoiCachTinh,
  FormGoKemLyDo,
  LINE_ORDER_TITLE,
  LineOrderDialog,
  SheetSelectionBar,
  TheChiSoNam,
  TheTomTat,
} from "./bang-thu-chi";
import { FormGhiDot, HopDotThuChi, NoiDungHopDot } from "./dot-thu-chi";
import { BudgetSheetHeaderActions } from "./header-actions";
import {
  changeColumnType,
  ColumnFieldsets,
  dungThanTaoBang,
  LapBang,
  percentOperandError,
  removeColumnAt,
} from "./lap-bang";
import {
  boCotKhoiDiem,
  CANH_BAO_GO_BANG,
  cauQuyDoi,
  CAU_THIEU_QUYEN_XEM,
  cellValueDraft,
  cellValueEdit,
  docSoNhap,
  donViCuaBang,
  DOT_TRONG,
  GHI_CHU_CHENH_LECH,
  GOI_Y_DOI_DON_VI,
  MO_TA_HOP_DOT,
  NHAN_CHENH_LECH,
  nhanDatDongTong,
  nhanGoKhoanMuc,
  nhanNutDot,
  nhanSuaO,
  nhanThemCon,
  NHAN_SUA_TEN,
  NO_REPORT_YET,
  PARENT_SUMS_CHILDREN,
  PHAN_CHUA_DUNG,
} from "./nhan-thu-chi";
import { FormSuaBang } from "./sua-bang";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — bổ cho các ca kiểm module thuần, vốn chỉ canh
 * quyết định bên trong hàm. Đã đo trên màn Danh mục: bôi trắng câu quan trọng nhất của một màn
 * hình vẫn để lại toàn bộ bài kiểm xanh và `tsc` sạch.
 *
 * NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ NHÓM **BỊ TỪ CHỐI**, không phải nhóm đủ quyền: tài khoản của
 * người viết mã luôn có cả ba khoá, nên nhánh thiếu quyền là nhánh không ai nhìn thấy trong lúc
 * phát triển. Và chỗ dễ gắn cổng nhầm nhất được kiểm riêng: **GỠ** và **ĐÁNH DẤU DÒNG TỔNG** đứng
 * sau `budget.confirm`, KHÔNG sau `budget.update`.
 */

const COT: finance_cotRa[] = [
  { id: "C1", name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam", numerator_column_id: null, denominator_column_id: null },
  { id: "C2", name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach", numerator_column_id: null, denominator_column_id: null },
  {
    id: "C3",
    name: "So sánh TH/DT (%)",
    order: 3,
    type: "phan_tram",
    formula: "Chi ngân sách / Dự toán năm × 100",
    numerator_column_id: "C2",
    denominator_column_id: "C1",
  },
];

function dong(sua: Partial<finance_dongRa> = {}): finance_dongRa {
  return {
    id: "A",
    no: "A",
    name: "CHI NGÂN SÁCH NHÀ NƯỚC",
    order: 1,
    method: "children",
    level: 0,
    is_headline: false,
    // ĐỒNG, như máy chủ gửi: 5.502.660 và 3.401.673,3 triệu.
    values: { C1: 5502660000000, C2: 3401673300000 },
    ...sua,
  };
}

const TOM_TAT: finance_tomTatRa = {
  headline_line_id: "A",
  cells: [
    { column_id: "C1", name: "Dự toán năm", role: "du-toan-nam", value: 3794740000000 },
    { column_id: "C2", name: "Chi ngân sách", role: "chi-ngan-sach", value: 3463459200000 },
  ],
  indicator: { name: "Chi đạt dự toán", basis_points: 9130 },
};

function bang(sua: Partial<finance_bangDayDuRa> = {}): finance_bangDayDuRa {
  return {
    sheet: {
      id: "01JBANG",
      code: "NS-2026-CHI-01",
      year: 2026,
      kind: "chi",
      revision: 1,
      title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026",
      unit: "trieu-dong",
      unit_label: "Triệu đồng",
      cumulative_to: "2026-08-25",
    },
    columns: COT,
    lines: [
      dong(),
      dong({ id: "I", no: "I", name: "Chi đầu tư phát triển", parent_id: "A", method: "manual" }),
    ],
    summary: TOM_TAT,
    ...sua,
  };
}

/**
 * Nhãn a11y như nó THẬT SỰ nằm trong chuỗi HTML.
 *
 * ⚠ HÀM NÀY ĐẾN TỪ MỘT LẦN XANH SAI ĐÃ ĐO, không từ sự cẩn thận: `nhanDatDongTong` sinh ra
 * `Đặt "X" làm con số tổng`, và `renderToStaticMarkup` thoát dấu ngoặc kép thành `&quot;` khi ghi
 * vào thuộc tính. Nên một phép so với chuỗi THÔ đi qua `not.toContain` sẽ XANH kể cả khi cái nút ấy
 * đang nằm chình ình trên trang — tức là ba bài kiểm cổng quyền quan trọng nhất của tệp này từng
 * canh đúng con số không. Mọi phép so nhãn a11y ở dưới đi qua hàm này.
 */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** Bảng vẽ với một bộ quyền. `thaoTac` rỗng: tệp này canh cái GÌ hiện ra, không canh lời gọi. */
function veBang(coGhi: boolean, coXacNhan: boolean, duLieu = bang()): string {
  return renderToStaticMarkup(
    <BangDayDu
      sheetLock={null}
      duLieu={duLieu}
      thuGon={new Set()}
      datThuGon={() => {}}
      coGhi={coGhi}
      coXacNhan={coXacNhan}
      dangGui={false}
      saveLine={async () => null}
      openLineOrder={() => {}}
      moThem={() => {}}
      moGoDong={() => {}}
      datTong={() => {}}
      moCachTinh={() => {}}
      moDot={() => {}}
    />,
  );
}

/** The selection bar (year · sheets · sheet actions) with a set of permissions. */
function renderSelectionBar(
  canRecord: boolean,
  canConfirm: boolean,
  sheetLock: string | null = null,
  sheetState: "loading" | "ready" | "missing" = "ready",
): string {
  return renderToStaticMarkup(
    <SheetSelectionBar
      year={2026}
      anchorYear={2026}
      onYearChange={() => {}}
      kind="chi"
      onKindChange={() => {}}
      sheetState={sheetState}
      canRecord={canRecord}
      canConfirm={canConfirm}
      sheetLock={sheetLock}
      busy={false}
      onCreate={() => {}}
      onEdit={() => {}}
      onRemove={() => {}}
      onImported={() => {}}
    />,
  );
}

/** Một dòng lá vẽ riêng, chỉ đọc, trên bộ cột `COT`. */
function renderLine(line: finance_dongRa): string {
  return renderToStaticMarkup(
    <table>
      <tbody>
        <DongKhoanMuc
          sheetLock={null}
          hien={{ dong: line, cap: 0, coCon: false, moRong: false }}
          cot={COT}
          donVi={donViCuaBang(bang().sheet)}
          dongTongId=""
          coGhi={false}
          coXacNhan={false}
          dangGui={false}
          moRongDoi={() => {}}
          saveLine={async () => null}
          openLineOrder={() => {}}
          them={() => {}}
          go={() => {}}
          datTong={() => {}}
          doiCachTinh={() => {}}
          moDot={() => {}}
        />
      </tbody>
    </table>,
  );
}

describe("CỔNG QUYỀN — nhánh bị từ chối", () => {
  it("thiếu `budget.read`: cả màn không dựng, và câu từ chối gọi ĐÚNG TÊN khoá", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen quyetDinh={{ hien: false, vi: "khong-du-quyen" }} cauThieuQuyen={CAU_THIEU_QUYEN_XEM}>
        {veBangJSX()}
      </KhungQuyen>,
    );

    expect(html).toContain("budget.read");
    expect(html).toContain(CAU_THIEU_QUYEN_XEM);
    // Không một dòng khoản mục nào ra tới trang.
    expect(html).not.toContain("CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026");
  });

  it("KHÔNG đọc được quyền (phiên hết hạn) cũng là ẨN — 'chưa rõ' không được xử như 'có'", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen
        quyetDinh={{ hien: false, vi: "khong-doc-duoc", thongBao: "Phiên làm việc đã hết hạn." }}
        cauThieuQuyen={CAU_THIEU_QUYEN_XEM}
      >
        {veBangJSX()}
      </KhungQuyen>,
    );

    expect(html).toContain("Phiên làm việc đã hết hạn.");
    expect(html).not.toContain("CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026");
  });

  it("chỉ đọc (không `budget.update`, không `budget.confirm`): KHÔNG nút ghi nào ra trang", () => {
    const html = veBang(false, false);

    expect(html).not.toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(NHAN_SUA_TEN));
    expect(html).not.toContain("Thêm khoản mục cấp cao nhất");
    expect(html).not.toContain("Gỡ bảng");
  });

  it("tài khoản chỉ đọc vẫn ĐỌC ĐƯỢC dòng nào đang là con số tổng", () => {
    // Đó là thông tin ai xem bảng cũng cần, kể cả người không đổi được nó — ẩn luôn ngôi sao sẽ
    // làm người đọc không biết ba con số tóm tắt phía trên lấy từ dòng nào.
    // Presentational pin (ADR 0068 §5): the `★` glyph became a lucide `Star`; the mark is now found
    // by its tooltip, which is also its accessible name. Still: shown to a read-only account, and
    // not as the button that changes it.
    const html = veBang(false, false);
    expect(html).toContain('title="Dòng đang là con số tổng"');
    expect(html).not.toContain('aria-pressed=');
  });
});

/** One line, rendered alone, with or without `budget.update`. */
function renderLineAs(line: finance_dongRa, coGhi: boolean): string {
  return renderToStaticMarkup(
    <table>
      <tbody>
        <DongKhoanMuc
          sheetLock={null}
          hien={{ dong: line, cap: 0, coCon: false, moRong: false }}
          cot={COT}
          donVi={donViCuaBang(bang().sheet)}
          dongTongId=""
          coGhi={coGhi}
          coXacNhan={false}
          dangGui={false}
          moRongDoi={() => {}}
          saveLine={async () => null}
          openLineOrder={() => {}}
          them={() => {}}
          go={() => {}}
          datTong={() => {}}
          doiCachTinh={() => {}}
          moDot={() => {}}
        />
      </tbody>
    </table>,
  );
}

describe("Ô SỐ BẤM ĐỂ SỬA TẠI CHỖ (§4.1, NS-01)", () => {
  const MANUAL = dong({ id: "I", name: "Chi đầu tư phát triển", method: "manual", values: { C1: 5502660000000 } });

  it("dòng Nhập trực tiếp + `budget.update`: mỗi ô số là một nút, tên nói cột, khoản mục và con số", () => {
    const html = renderLineAs(MANUAL, true);
    expect(html).toContain(nhuTrongHTML(nhanSuaO("Dự toán năm", "Chi đầu tư phát triển", "5.502.660")));
    expect(html).toContain(nhuTrongHTML(nhanSuaO("Chi ngân sách", "Chi đầu tư phát triển", "—")));
    // The % column is computed by the server, never a button.
    expect(html).not.toContain("Sửa số So sánh");
  });

  it("THIẾU `budget.update`: không ô số nào là nút — vẫn hiện con số", () => {
    const html = renderLineAs(MANUAL, false);
    expect(html).not.toContain("Sửa số ");
    expect(html).toContain("5.502.660");
  });

  it("dòng Cộng theo đợt hay cộng con: không ô số nào là nút (máy chủ từ chối số gõ tay)", () => {
    for (const method of ["entries", "children"]) {
      // `Sửa số <cột>` — not the row's `Sửa số thứ tự…` button, which every editable row has.
      const html = renderLineAs({ ...MANUAL, method }, true);
      expect(html).not.toContain("Sửa số Dự toán năm");
      expect(html).not.toContain("Sửa số Chi ngân sách");
    }
  });
});

describe("CỔNG QUYỀN — chỗ dễ gắn nhầm nhất", () => {
  it("`budget.update` MỘT MÌNH KHÔNG mở nút GỠ và KHÔNG mở ngôi sao", () => {
    // Đây là bài kiểm đắt nhất của cả màn. Gỡ một khoản mục đổi ngay con số lãnh đạo đã đọc, và
    // đánh sao đổi DÒNG mà mọi ô tóm tắt cùng hai chỉ số đọc từ đó — cả hai đứng sau
    // `budget.confirm` ở máy chủ (`routes.go:909`, `:943`). Gắn chúng vào quyền nhập liệu là mở
    // một thao tác xác nhận cho người chỉ được nhập số, và không có gì trên màn hình nói ra.
    const html = veBang(true, false);

    expect(html).toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).toContain(nhuTrongHTML(NHAN_SUA_TEN));
    expect(html).toContain("Thêm khoản mục cấp cao nhất");

    expect(html).not.toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain("Gỡ bảng");
  });

  it("`budget.confirm` mở GỠ và ngôi sao, với nhãn a11y nguyên văn đặc tả", () => {
    const html = veBang(true, true);

    expect(html).toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).toContain(nhuTrongHTML(nhanGoKhoanMuc("CHI NGÂN SÁCH NHÀ NƯỚC")));
  });

  it("thanh chọn bảng: `Gỡ` (bảng) đứng sau `budget.confirm`, KHÔNG sau `budget.update`", () => {
    // The sheet's removal moved to the selection bar, as in the prototype (ADR 0068 lần 5).
    expect(renderSelectionBar(true, false)).not.toContain('aria-label="Gỡ bảng"');
    expect(renderSelectionBar(false, false)).not.toContain('aria-label="Gỡ bảng"');
    expect(renderSelectionBar(false, true)).toContain('aria-label="Gỡ bảng"');
  });

  it("thanh chọn bảng: chưa có bảng thì `Lập bảng` (budget.update), không `Sửa` hay `Gỡ`", () => {
    const html = renderSelectionBar(true, true, null, "missing");
    expect(html).toContain(">Lập bảng<");
    expect(html).not.toContain("Sửa thông tin bảng");
    expect(html).not.toContain('aria-label="Gỡ bảng"');
    expect(renderSelectionBar(false, true, null, "missing")).not.toContain(">Lập bảng<");
  });

  it("`budget.confirm` MỘT MÌNH không mở nút thêm và không mở sửa tên", () => {
    const html = veBang(false, true);

    expect(html).toContain(nhuTrongHTML(nhanDatDongTong("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain(nhuTrongHTML(nhanThemCon("CHI NGÂN SÁCH NHÀ NƯỚC")));
    expect(html).not.toContain("Thêm khoản mục cấp cao nhất");
  });
});

describe("con số ra tới trang", () => {
  it("máy chủ gửi ĐỒNG, bảng triệu đồng in 3.401.673,3 — không in số đồng thô cạnh nhãn triệu", () => {
    // Trước quyết định 25/09/2026 trang in `3.401.673.300.000` cạnh chữ "Triệu đồng": lệch 10⁶ lần.
    const html = veBang(false, false);

    expect(html).toContain("5.502.660");
    expect(html).toContain("3.401.673,3");
    expect(html).toContain("3.463.459,2");
    expect(html).not.toContain("3.401.673.300.000");
    expect(html).toContain(cauQuyDoi(donViCuaBang(bang().sheet)));
    expect(html).toContain("Đơn vị tính: Triệu đồng");
  });

  it("đơn vị cũ chưa ánh xạ được: in ĐỒNG thô, nhãn 'đồng', và câu cảnh báo NỔI BẬT", () => {
    const canhBao =
      "Đơn vị tính đang lưu không thuộc danh sách đồng / nghìn đồng / triệu đồng — chọn lại đơn vị cho bảng.";
    const sheet = { ...bang().sheet, unit: "", unit_label: "Tr.đồng", unit_warning: canhBao };
    const html = veBang(false, false, bang({ sheet }));

    expect(html).toContain('role="alert"');
    expect(html).toContain(canhBao);
    expect(html).toContain("Tr.đồng");
    expect(html).toContain("Đơn vị tính: đồng");
    expect(html).toContain("3.401.673.300.000");
    expect(html).not.toContain("3.401.673,3");
  });

  it("ô phần trăm in ĐÚNG phần vạn máy chủ gửi — không tự chia `values` — làm tròn một chữ số lẻ", () => {
    // `values` của dòng cho 3.401.673,3 / 5.502.660 = 61,8%. Máy chủ gửi 9127 CÓ CHỦ Ý khác đi: nếu
    // trang tự chia thì nó ra 61,8% và ca này đỏ. One decimal, as the prototype (owner, 09/10/2026).
    const html = renderLine(dong({ method: "manual", percent_basis_points: { C3: 9127 } }));

    expect(html).toContain(">91,3%<");
    expect(html).not.toContain("91,27%");
    expect(html).not.toContain("61,8");
    expect(html).not.toContain("Không tính được");
  });

  it("số trong bảng làm tròn MỘT chữ số lẻ theo đơn vị của bảng; ô sửa vẫn điền con số CHÍNH XÁC", () => {
    // 1.234.567,89 triệu đồng — prints 1.234.567,9; the box the officer opens holds the exact digits.
    const line = dong({ id: "I", name: "Chi đầu tư phát triển", method: "manual", values: { C1: 1234567890000, C2: null } });
    const html = renderLine(line);
    expect(html).toContain(">1.234.567,9<");
    expect(html).not.toContain("1.234.567,89");
    expect(cellValueDraft(line, "C1", "trieu-dong")).toBe("1.234.567,89");
  });

  it("ô phần trăm null kèm câu: dấu 'Không tính được' và NGUYÊN câu máy chủ, không phải 0%", () => {
    const cau = "ngan_sach: mẫu số bằng 0 — không tính tỷ lệ";
    const html = renderLine(
      dong({ method: "manual", percent_basis_points: { C3: null }, unavailable_reasons: { C3: cau } }),
    );

    expect(html).toContain("Không tính được");
    expect(html).toContain(cau);
    expect(html).not.toContain("0,00%");
  });

  it("cột `%` cũ chưa rõ tử số / mẫu số: ô nói câu của máy chủ, và dưới bảng có MỘT khối ở mức cột", () => {
    const cau =
      "ngan_sach: cột phần trăm này tạo trước khi hệ thống lưu rõ cột tử số và mẫu số, và công thức cũ không đọc được chắc chắn — không tính được; không đoán từ công thức";
    const legacyColumn: finance_cotRa = {
      ...COT[2]!,
      formula: "col_4 / col_2 * 100",
      numerator_column_id: null,
      denominator_column_id: null,
    };
    const lines = bang().lines.map((l) => ({
      ...l,
      percent_basis_points: { C3: null },
      unavailable_reasons: { C3: cau },
    }));
    const html = veBang(false, false, bang({ columns: [COT[0]!, COT[1]!, legacyColumn], lines }));

    expect(html).toContain(cau);
    expect(html).toContain("Cột phần trăm chưa tính được tỷ lệ (1)");
    expect(html).toContain("col_4 / col_2 * 100");
    // Ô `%` không tính được KHÔNG chen vào danh sách ô TIỀN không tính được.
    expect(html).not.toContain("Ô không tính được con số");
  });

  it("tiêu đề cột `%` chỉ hiện NHÃN, không hiện chuỗi công thức (bảng lỗi khách hàng dòng 52)", () => {
    expect(veBang(false, false)).not.toContain("Chi ngân sách / Dự toán năm × 100");
  });

  it("dòng không mang `percent_basis_points` (bản đồ vắng): ô `%` là DẤU GẠCH, không phải 0%", () => {
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <DongKhoanMuc
            sheetLock={null}
            hien={{ dong: dong({ method: "manual" }), cap: 0, coCon: false, moRong: false }}
            cot={COT}
            donVi={donViCuaBang(bang().sheet)}
            dongTongId=""
            coGhi={false}
            coXacNhan={false}
            dangGui={false}
            moRongDoi={() => {}}
            saveLine={async () => null}
            openLineOrder={() => {}}
            them={() => {}}
            go={() => {}}
            datTong={() => {}}
            doiCachTinh={() => {}}
            moDot={() => {}}
          />
        </tbody>
      </table>,
    );

    expect(html).toContain("—");
    expect(html).not.toContain("61,8");
    expect(html).not.toContain("0,00%");
  });

  it("thẻ tóm tắt KHÔNG đoán dòng tổng: chưa ai đánh dấu thì hiện CÂU của máy chủ, không hiện 0", () => {
    // §5 quy tắc 5 nói "mặc định dòng đầu tiên", và nửa ấy là cạm bẫy: bảng thu có hai dòng cấp
    // cao lồng nhau, bảng chi có `Tổng số` đứng NGANG HÀNG A…E (ADR 0035 §A).
    const cau = "ngan_sach: bảng chưa có dòng nào được đánh dấu là dòng tổng";
    const html = renderToStaticMarkup(
      <TheTomTat
        bang={bang().sheet}
        tomTat={{ unavailable_reason: cau, cells: [], indicator: { name: "Chi đạt dự toán", basis_points: null } }}
        columns={COT}
        soKhoanMuc={59}
        donVi={donViCuaBang(bang().sheet)}
      />,
    );

    expect(html).toContain(cau);
    expect(html).not.toContain("0%");
  });

  it("thẻ tóm tắt: MỘT ô cho MỖI cột của bảng, kể cả cột %, nhãn là tên cột, số theo đơn vị của bảng", () => {
    const headline = dong({ percent_basis_points: { C3: 9127 } });
    const html = renderToStaticMarkup(
      <TheTomTat
        bang={bang().sheet}
        tomTat={TOM_TAT}
        columns={COT}
        headline={headline}
        soKhoanMuc={2}
        donVi={donViCuaBang(bang().sheet)}
      />,
    );
    const labels = [...html.matchAll(/<dt[^>]*>(.*?)<\/dt>/g)].map((m) => m[1]);
    const values = [...html.matchAll(/<dd[^>]*>(.*?)<\/dd>/g)].map((m) => m[1]);
    // Three columns, three tiles — the server's indicator is no longer a fourth tile here.
    expect(labels).toEqual(["Dự toán năm", "Chi ngân sách", "So sánh TH/DT (%)"]);
    // In the SHEET'S unit (triệu đồng), one decimal — not `compactDong` ("3,8 nghìn tỷ").
    expect(values).toEqual(["3.794.740", "3.463.459,2", "91,3%"]);
    expect(html).not.toContain("tỷ");
    // The exact figure stays on hover.
    expect(html).toContain('title="3.463.459,2 triệu đồng"');
  });
});

describe("thẻ ba chỉ số của năm", () => {
  function chiSo(sua: Partial<finance_chiSoNamRa> = {}): finance_chiSoNamRa {
    return {
      year: 2026,
      revenue_achievement: { name: "Thu đạt dự toán", basis_points: 10811 },
      expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
      balance: { amount: 853304000000 },
      revenue_totals: [
        { column_id: "T1", name: "Thu ngân sách NSNN", role: "thu-nsnn", value: 4316764000000 },
        { column_id: "T2", name: "Thu ngân sách Thu xã hưởng", role: "thu-xa-huong", value: 3300800000000 },
      ],
      ...sua,
    };
  }

  it("`basis_points` là phần vạn: 10811 ra trang thành 108,11%", () => {
    expect(renderToStaticMarkup(<TheChiSoNam chiSo={chiSo()} />)).toContain("108,11%");
  });

  it("CẢ HAI số thu hiện ra, mỗi số gọi đúng tên (ADR 0035 §A)", () => {
    // Xã cần một số để báo cáo thu ngân sách, và một số để biết mình còn được giữ bao nhiêu. Màn
    // hình chỉ hiện một trong hai là trả lời một câu hỏi không ai đặt.
    const html = renderToStaticMarkup(<TheChiSoNam chiSo={chiSo()} />);
    expect(html).toContain("Thu ngân sách NSNN");
    expect(html).toContain("Thu ngân sách Thu xã hưởng");
  });

  it("xã chưa có bảng: thẻ VẪN Ở LẠI, nói câu của máy chủ, và KHÔNG hiện 0", () => {
    const cau = "xã chưa có bảng ngân sách cho năm và loại này";
    const html = renderToStaticMarkup(
      <TheChiSoNam
        chiSo={chiSo({
          revenue_achievement: { name: "Thu đạt dự toán", basis_points: null, unavailable_reason: cau },
          balance: { amount: null, unavailable_reason: cau },
          revenue_totals: [],
        })}
      />,
    );

    expect(html).toContain("Chỉ số ngân sách năm 2026");
    expect(html).toContain(cau);
    expect(html).not.toContain("0,00%");
  });
});

describe("cây khoản mục và thanh công cụ", () => {
  it("thu gọn một dòng thì con của nó KHÔNG ra trang, và bộ đếm nói đúng", () => {
    const html = renderToStaticMarkup(
      <BangDayDu
        sheetLock={null}
        duLieu={bang()}
        thuGon={new Set(["A"])}
        datThuGon={() => {}}
        coGhi={false}
        coXacNhan={false}
        dangGui={false}
        saveLine={async () => null}
        openLineOrder={() => {}}
        moThem={() => {}}
        moGoDong={() => {}}
        datTong={() => {}}
        moCachTinh={() => {}}
        moDot={() => {}}
      />,
    );

    expect(html).not.toContain("Chi đầu tư phát triển");
    expect(html).toContain("đang hiện 1/2 khoản mục");
  });

  it("mở hết thì đủ hai dòng", () => {
    expect(veBang(false, false)).toContain("đang hiện 2/2 khoản mục");
  });
});

describe("biểu mẫu ghi", () => {
  it("hộp GỠ có Ô LÝ DO BẮT BUỘC, không chỉ một nút Đồng ý", () => {
    // §6 chỉ vẽ "hộp xác nhận". Máy chủ đòi `reason` trong thân (luật 7 bất biến 1), nên một hộp
    // không có ô lý do nhận 400 ở MỌI lần bấm.
    const html = renderToStaticMarkup(
      <FormGoKemLyDo
        tieuDe="Gỡ bảng X"
        canhBao={CANH_BAO_GO_BANG}
        dangGui={false}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain('name="reason"');
    expect(html).toContain("required");
    expect(html).toContain(CANH_BAO_GO_BANG);
  });

  it("ô số sửa tại chỗ điền sẵn THEO ĐƠN VỊ CỦA BẢNG — đọc ngược lại ra đúng số đồng cũ", () => {
    const line = dong({ method: "manual" });
    expect(cellValueDraft(line, "C2", "trieu-dong")).toBe("3.401.673,3");
    // Enter without typing sends nothing.
    expect(cellValueEdit(line, COT[1]!, "3.401.673,3", "trieu-dong")).toEqual({ kind: "unchanged" });
  });

  it("giá trị máy chủ gửi mà JS không đọc chính xác được: ô sửa điền NGUYÊN chữ số, không điền rỗng", () => {
    // Ô rỗng lúc lưu nghĩa là XOÁ TRẮNG ô ấy (`values[cot] = null`). Điền rỗng cho một giá trị đọc hỏng
    // thì một lần bấm vào ô rồi rời đi lặng lẽ xoá một con số ngân sách đang lưu. Chữ số thô bị
    // `docSoNhap` từ chối kèm một câu, nên lần lưu dừng lại và cán bộ thấy có chuyện.
    const line = dong({ method: "manual", values: { C1: 9007199254740994, C2: null } });
    expect(cellValueDraft(line, "C1", "trieu-dong")).toBe("9007199254740994");
    // Ô thật sự trống (máy chủ gửi `null`) thì vẫn điền rỗng — đó là "trống", không phải "hỏng".
    expect(cellValueDraft(line, "C2", "trieu-dong")).toBe("");
    // Và chữ số thô ấy KHÔNG lọt qua ô đọc như một con số (đơn vị bảng là triệu đồng).
    expect(docSoNhap("9007199254740994", "trieu-dong").loai).toBe("loi");
    expect(cellValueEdit(line, COT[0]!, "9007199254740994", "trieu-dong").kind).toBe("invalid");
  });

  it("ô số: gõ số mới gửi ĐÚNG MỘT cột, xoá trắng gửi null, gõ sai thì giữ ô mở với câu lý do", () => {
    const line = dong({ method: "manual" });
    expect(cellValueEdit(line, COT[0]!, "1,5", "trieu-dong")).toEqual({
      kind: "save",
      body: { values: { C1: 1500000 } },
    });
    expect(cellValueEdit(line, COT[0]!, "", "trieu-dong")).toEqual({ kind: "save", body: { values: { C1: null } } });
    const wrong = cellValueEdit(line, COT[0]!, "1.5", "trieu-dong");
    expect(wrong.kind).toBe("invalid");
  });
});

describe("TT và thứ tự hiển thị — hộp nhỏ riêng", () => {
  it("chỉ hai trường, điền sẵn giá trị của dòng; tiêu đề nói đúng việc", () => {
    const html = renderToStaticMarkup(
      <LineOrderDialog dong={dong({ no: "I", order: 3 })} dangGui={false} huy={() => {}} luu={() => {}} />,
    );
    expect(html).toContain(LINE_ORDER_TITLE);
    expect(html).toMatch(/name="no"[^>]*value="I"/);
    expect(html).toMatch(/name="order"[^>]*value="3"/);
    expect(html).not.toContain('name="name"');
    expect(html).not.toContain('name="gia:');
  });
});

describe("lập bảng — biểu mẫu cho xã không có tệp Excel", () => {
  it("KHÔNG vẽ ô chọn tệp nào: tệp đi qua `Nạp từ Excel`, biểu mẫu này chỉ lập bảng bằng tay", () => {
    const html = renderToStaticMarkup(
      <LapBang nam={2026} loai="chi" dangGui={false} datDangGui={() => {}} xong={() => {}} onClose={() => {}} />,
    );

    expect(html).toContain("Lập bảng chi ngân sách năm 2026");
    expect(html).not.toContain('type="file"');
    expect(html).not.toContain(".xlsx");
  });
});

describe("những phần đặc tả vẽ mà chưa dựng được", () => {
  it("không còn phần nào: `Nạp từ Excel` đã dựng (ADR 0081 #6) — nút THẬT, không vô hiệu, không dấu '?'", () => {
    const html = renderToStaticMarkup(<BudgetSheetHeaderActions year={2026} busy={false} onImported={() => {}} />);

    expect(PHAN_CHUA_DUNG).toHaveLength(0);
    expect(html).not.toContain(`aria-label="${pendingMarkerLabel("Nạp từ Excel")}"`);
    expect(html).toMatch(/<button[^>]*>.*Nạp từ Excel<\/button>/);
    expect(html).toContain('type="file" accept=".xlsx"');
    expect(html).not.toMatch(/<button[^>]* disabled=""[^>]*>.*Nạp từ Excel<\/button>/);
  });
});

describe("thẻ chỉ số — `Chênh lệch thu – chi luỹ kế`", () => {
  const CHI_SO: finance_chiSoNamRa = {
    year: 2026,
    revenue_achievement: { name: "Thu đạt dự toán", basis_points: 10811 },
    expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
    balance: { amount: 853304000000 },
    revenue_totals: [],
  };

  it("nhãn ĐÚNG chữ khách chốt, kèm ghi chú cách tính và câu 'chờ xác nhận'", () => {
    const html = renderToStaticMarkup(<TheChiSoNam chiSo={CHI_SO} />);

    expect(html).toContain(NHAN_CHENH_LECH);
    expect(html).toContain(nhuTrongHTML(GHI_CHU_CHENH_LECH));
    expect(html).toContain("chờ khách hàng xác nhận");
  });

  it("KHÔNG còn chữ cân đối, bội chi hay thâm hụt ở bất kỳ đâu trên thẻ", () => {
    const html = renderToStaticMarkup(<TheChiSoNam chiSo={CHI_SO} />);

    expect(html).not.toMatch(/Cân đối/i);
    expect(html).not.toMatch(/bội chi/i);
    expect(html).not.toMatch(/thâm hụt/i);
  });

  it("số tiền của thẻ in bằng ĐỒNG kèm chữ 'đồng' — thẻ đọc từ cả hai bảng", () => {
    expect(renderToStaticMarkup(<TheChiSoNam chiSo={CHI_SO} />)).toContain("853.304.000.000 đồng");
  });

  it("HAI số thu cũng in bằng đồng, và chênh lệch ÂM in dấu âm — không số nào mượn đơn vị của một bảng", () => {
    // Thẻ nằm ngoài hai tab: bảng thu có thể là triệu đồng trong khi bảng chi là đồng. Một số thu in
    // theo đơn vị của bảng thu đặt cạnh chênh lệch in theo đồng là hai con số lệch nhau 10⁶ lần trên
    // cùng một thẻ lãnh đạo đọc.
    const html = renderToStaticMarkup(
      <TheChiSoNam
        chiSo={{
          ...CHI_SO,
          balance: { amount: -1500000 },
          revenue_totals: [
            { column_id: "T1", name: "Thu ngân sách NSNN", role: "thu-nsnn", value: 4316764000000 },
            { column_id: "T2", name: "Thu ngân sách Thu xã hưởng", role: "thu-xa-huong", value: null },
          ],
        }}
      />,
    );

    expect(html).toContain("4.316.764.000.000 đồng");
    expect(html).toContain("-1.500.000 đồng");
    // Số thu vắng là `—`, không phải "— đồng" và không phải "0 đồng".
    expect(html).not.toContain("— đồng");
    expect(html).not.toContain(">0 đồng<");
  });
});

describe("cách tính — ô chọn trên dòng lá", () => {
  /** The opening `<select …>` tag and its options, by `aria-label`. */
  function selectByLabel(html: string, label: string): string {
    const at = html.indexOf(`aria-label="${nhuTrongHTML(label)}"`);
    expect(at).toBeGreaterThan(-1);
    const start = html.lastIndexOf("<select", at);
    return html.slice(start, html.indexOf("</select>", at));
  }

  it("ba lựa chọn; dòng có con: ô chọn VÔ HIỆU, hiện 'Cộng khoản mục con'; dòng lá: lựa chọn ấy vô hiệu", () => {
    const html = veBang(true, false);

    const parent = selectByLabel(html, "Cách tính của CHI NGÂN SÁCH NHÀ NƯỚC");
    expect(parent).toMatch(/^<select[^>]* disabled=""/);
    expect(parent).toContain(nhuTrongHTML(PARENT_SUMS_CHILDREN));
    expect(parent).toContain('<option value="children" selected="">Cộng khoản mục con</option>');

    const leaf = selectByLabel(html, "Cách tính của Chi đầu tư phát triển");
    expect(leaf).not.toMatch(/^<select[^>]* disabled=""/);
    expect(leaf).toContain('<option value="manual" selected="">Nhập trực tiếp</option>');
    expect(leaf).toContain('<option value="entries">Cộng theo đợt</option>');
    // Never sent (the server answers 400): shown, not choosable.
    expect(leaf).toContain('<option value="children" disabled="">Cộng khoản mục con</option>');
  });

  it("thiếu `budget.update`: KHÔNG ô chọn nào, cách tính hiện thành chữ", () => {
    const html = veBang(false, true);

    expect(html).not.toContain("<select");
    expect(html).toContain("Nhập trực tiếp");
  });

  it("chuyển entries → manual: hộp xác nhận nói tổng các đợt được chép vào ô số", () => {
    const html = renderToStaticMarkup(
      <FormDoiCachTinh ten="Thu phí" den="manual" dangGui={false} huy={() => {}} luu={() => {}} />,
    );

    expect(html).toContain("chép tổng các đợt");
    expect(html).toContain("Đổi cách tính");
  });

  it("chuyển manual → entries: hộp nói số gõ tay được giữ nhưng không hiện", () => {
    const html = renderToStaticMarkup(
      <FormDoiCachTinh ten="Thu phí" den="entries" dangGui={false} huy={() => {}} luu={() => {}} />,
    );

    expect(html).toContain("không hiện nữa");
  });
});

describe("hộp các đợt thu, chi (§5)", () => {
  const COT_SO: finance_cotRa[] = [COT[0]!, COT[1]!];
  const DON_VI = donViCuaBang(bang().sheet);

  const DOT: finance_danhSachDotRa = {
    line_id: "I",
    method: "entries",
    entries: [
      {
        id: "D2",
        line_id: "I",
        date: "2026-09-20",
        content: "Thu tiền sử dụng đất đợt 2",
        // ĐÃ CHE Ở MÁY CHỦ — màn hình in nguyên.
        counterparty: "N*** V** A",
        document_no: "PT-12",
        values: { C1: null, C2: 1005000 },
      },
    ],
  };

  function veNoiDung(coXacNhan: boolean, danhSach: finance_danhSachDotRa = DOT): string {
    return renderToStaticMarkup(
      <NoiDungHopDot
        closes={[]}
        sheetYear={2026}
        cot={COT_SO}
        donVi={DON_VI}
        danhSach={{ pha: "xong", duLieu: danhSach }}
        coXacNhan={coXacNhan}
        dangGui={false}
        moGo={() => {}}
      />,
    );
  }

  it("nút ⇄ trên MỌI dòng: dòng lá mở được (cả người chỉ đọc); dòng có con VÔ HIỆU và nói vì sao", () => {
    const html = veBang(false, false);

    const leafAt = html.indexOf(`aria-label="${nhuTrongHTML(nhanNutDot("Chi đầu tư phát triển"))}"`);
    expect(leafAt).toBeGreaterThan(-1);
    expect(html.slice(html.lastIndexOf("<button", leafAt), html.indexOf(">", leafAt))).not.toContain('disabled=""');
    // The parent's button is there, disabled, named by the reason — never as "Các đợt thu, chi của …".
    expect(html).not.toContain(nhuTrongHTML(nhanNutDot("CHI NGÂN SÁCH NHÀ NƯỚC")));
    const parentAt = html.indexOf(`aria-label="${nhuTrongHTML(PARENT_SUMS_CHILDREN)}"`);
    expect(parentAt).toBeGreaterThan(-1);
    expect(html.slice(html.lastIndexOf("<button", parentAt), html.indexOf(">", parentAt))).toContain('disabled=""');
  });

  it("tiêu đề là tên khoản mục NGUYÊN VĂN, câu mô tả nguyên văn §5 ngay dưới, không thêm dòng phụ", () => {
    const html = renderToStaticMarkup(
      <HopDotThuChi
        closes={[]}
        sheetYear={2026}
        khoanMucId="I"
        tenKhoanMuc="Chi đầu tư phát triển"
        cot={COT_SO}
        donVi={DON_VI}
        coGhi
        coXacNhan={false}
        dong={() => {}}
        daDoiSoLieu={() => {}}
      />,
    );

    expect(html).toContain(">Chi đầu tư phát triển</h2>");
    expect(html).not.toContain("CHI ĐẦU TƯ PHÁT TRIỂN");
    expect(html).toContain(MO_TA_HOP_DOT);
    // The two extra lines (summing condition, unit) and the list heading are gone (prototype).
    expect(html).not.toContain("chưa được cộng");
    expect(html).not.toContain("Số tiền theo đơn vị của bảng");
    expect(html).not.toContain("Các đợt đã ghi</h4>");
    // Money labels are the column's own label.
    expect(html).toContain(">Dự toán năm</label>");
    // A centred modal now, as the prototype's `FiscalEntriesDialog` (native `<dialog>`).
    expect(html).toContain("<dialog");
  });

  it("thiếu `budget.update`: không có biểu mẫu ghi đợt", () => {
    const html = renderToStaticMarkup(
      <HopDotThuChi
        closes={[]}
        sheetYear={2026}
        khoanMucId="I"
        tenKhoanMuc="Chi đầu tư phát triển"
        cot={COT_SO}
        donVi={DON_VI}
        coGhi={false}
        coXacNhan={false}
        dong={() => {}}
        daDoiSoLieu={() => {}}
      />,
    );

    // Presentational pin (ADR 0068 §5): "+ Ghi đợt" became a `Plus` icon + "Ghi đợt", matched as the
    // button's own text (`>Ghi đợt<`) so the title "Ghi một đợt" cannot satisfy it.
    expect(html).not.toContain(">Ghi đợt<");
  });

  it("danh sách rỗng hiện 'Chưa ghi đợt nào.' — một dòng chữ, không biểu tượng", () => {
    const html = veNoiDung(false, { line_id: "I", method: "entries", entries: [] });
    expect(html).toContain(`<p class="m-0 py-8 text-center text-[12.5px] text-ink-muted">${DOT_TRONG}</p>`);
    expect(html).not.toContain("<svg");
  });

  it("danh sách nằm trong khung cao tối đa 24rem, tự cuộn", () => {
    expect(veNoiDung(false)).toContain("max-h-[24rem]");
  });

  it("'Đơn vị, cá nhân' in NGUYÊN chuỗi máy chủ đã che — không thử khôi phục", () => {
    const html = veNoiDung(false);

    // Under the entry's content, as the prototype: "<đơn vị, cá nhân> · <số chứng từ>".
    expect(html).toContain("N*** V** A · PT-12");
  });

  it("số tiền của đợt quy đổi theo đơn vị của bảng, giống ô của bảng", () => {
    expect(veNoiDung(false)).toContain("1,005");
  });

  it("KHÔNG có `budget.confirm`: không nút gỡ đợt nào", () => {
    expect(veNoiDung(false)).not.toContain("Gỡ đợt");
  });

  it("có `budget.confirm`: nút gỡ đợt hiện — chỉ biểu tượng thùng rác đỏ, tên đọc được", () => {
    const html = veNoiDung(true);
    expect(html).toContain('aria-label="Gỡ đợt ngày 20/9/2026"');
    expect(html).not.toContain(">Gỡ đợt<");
  });

  it("biểu mẫu ghi đợt có ô tiền CHỈ cho cột số, là ô CHỮ, và bốn trường của §5", () => {
    const html = renderToStaticMarkup(<FormGhiDot cot={COT_SO} dangGui={false} gui={() => {}} />);

    expect(html).toContain('name="dot-gia:C1"');
    expect(html).toContain('name="dot-gia:C2"');
    expect(html).not.toContain('name="dot-gia:C3"');
    expect(html).not.toContain('type="number"');
    expect(html).toContain('name="counterparty"');
    expect(html).toContain('name="document_no"');
    expect(html).toContain("Thu tiền sử dụng đất đợt 2");
    expect(html).toContain(">Ghi đợt<");
  });

  it("hộp gỡ đợt dùng hộp gỡ có ô lý do BẮT BUỘC", () => {
    const html = renderToStaticMarkup(
      <FormGoKemLyDo
        idTruong="go-dot-reason"
        tieuDe="Gỡ đợt ngày 20/9/2026"
        canhBao="x"
        dangGui={false}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain('id="go-dot-reason"');
    expect(html).toContain('name="reason"');
    expect(html).toContain("required");
  });
});

describe("thanh chọn bảng — nút kỳ rời, như prototype", () => {
  /** The opening tag of the sheet button whose words are `text`. */
  function sheetButton(html: string, text: string): string {
    const at = html.indexOf(`>${text}</button>`);
    expect(at).toBeGreaterThan(-1);
    return html.slice(html.lastIndexOf("<button", at), at + 1);
  }

  it("kỳ đang chọn: nền navy chữ trắng, `aria-selected=true`; kỳ kia: viền nhạt chữ xám, `false`", () => {
    const html = renderSelectionBar(false, false);
    const chosen = sheetButton(html, "Chi ngân sách 2026");
    const other = sheetButton(html, "Thu ngân sách 2026");

    expect(chosen).toContain('aria-selected="true"');
    expect(chosen).toContain("border-navy bg-navy text-white");
    expect(other).toContain('aria-selected="false"');
    expect(other).toContain("border-line bg-surface text-ink-muted");
    for (const tag of [chosen, other]) {
      expect(tag).toContain("rounded-[8px] border border-solid px-3 py-1.5");
      expect(tag).toContain("text-[12.5px]");
      expect(tag).toContain('role="tab"');
    }
    // Separate buttons with a 6px gap — no grey segmented track that scrolls sideways.
    expect(html).toContain('role="tablist" aria-label="Chọn bảng thu hoặc bảng chi" class="flex flex-wrap gap-1.5"');
    expect(html).not.toContain("bg-muted p-[3px]");
  });

  it("`Gỡ` là nút viền chữ thường (outline), không nền đỏ; thứ tự Nạp · Sửa · Gỡ", () => {
    const html = renderSelectionBar(true, true);
    const at = html.indexOf('aria-label="Gỡ bảng"');
    const tag = html.slice(html.lastIndexOf("<button", at), html.indexOf(">", at));
    expect(tag).toContain("text-foreground");
    expect(tag).not.toContain("text-destructive");
    const order = ["Nạp từ Excel", "Sửa thông tin bảng", 'aria-label="Gỡ bảng"'].map((s) => html.indexOf(s));
    expect(order).toEqual([...order].sort((a, b) => a - b));
  });

  it("câu trạng thái rỗng là câu của prototype, nguyên văn", () => {
    expect(NO_REPORT_YET).toBe("Chưa có báo cáo nào. Nạp tệp Excel của Phòng Tài chính để bắt đầu.");
  });
});

describe("sửa thông tin bảng", () => {
  it("nút sửa bảng đứng sau `budget.update`, không sau `budget.confirm`", () => {
    expect(renderSelectionBar(true, false)).toContain("Sửa thông tin bảng");
    expect(renderSelectionBar(false, true)).not.toContain("Sửa thông tin bảng");
  });

  it("ba trường, đơn vị là Ô CHỌN ba mã, và câu gợi ý nói đổi đơn vị không đổi con số", () => {
    const html = renderToStaticMarkup(
      <FormSuaBang bang={bang().sheet} dangGui={false} huy={() => {}} luu={() => {}} />,
    );

    expect(html).toContain('name="title"');
    expect(html).toContain('name="cumulative_to"');
    expect(html).toContain('value="2026-08-25"');
    expect(html).toContain('<select id="sua-bang-unit" name="unit"');
    expect(html).toContain('value="dong"');
    expect(html).toContain('value="nghin-dong"');
    expect(html).toContain('value="trieu-dong" selected=""');
    expect(html).toContain(GOI_Y_DOI_DON_VI);
    expect(html).not.toContain('name="year"');
    expect(html).not.toContain('name="kind"');
  });

  it("bảng mang đơn vị cũ chưa ánh xạ: ô chọn KHÔNG chọn sẵn mã nào", () => {
    const sheet = { ...bang().sheet, unit: "", unit_label: "Tr.đ", unit_warning: "x" };
    const html = renderToStaticMarkup(
      <FormSuaBang bang={sheet} dangGui={false} huy={() => {}} luu={() => {}} />,
    );

    expect(html).toContain("— Chọn đơn vị tính —");
    expect(html).not.toContain('value="trieu-dong" selected=""');
  });
});

describe("lập bảng — đơn vị là một MÃ", () => {
  it("thân gửi `trieu-dong`, không phải chữ tự do", () => {
    const kq = dungThanTaoBang({
      nam: 2026,
      loai: "chi",
      tieuDe: "BÁO CÁO CHI",
      donVi: "trieu-dong",
      luyKe: "",
      cot: [{ name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam" }],
    });

    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.than.unit).toBe("trieu-dong");
    expect(kq.than.cumulative_to).toBeUndefined();
  });

  it("chữ tự do như 'Triệu đồng' bị TỪ CHỐI, không bị thay bằng một mã đoán", () => {
    const kq = dungThanTaoBang({
      nam: 2026,
      loai: "chi",
      tieuDe: "T",
      donVi: "Triệu đồng",
      luyKe: "",
      cot: [],
    });

    expect(kq.ok).toBe(false);
  });
});

describe("lập bảng — cột `%` chọn tử số và mẫu số", () => {
  function build(columns: readonly finance_cotVao[]) {
    return dungThanTaoBang({
      nam: 2026,
      loai: "chi",
      tieuDe: "BÁO CÁO CHI",
      donVi: "trieu-dong",
      luyKe: "",
      cot: columns,
    });
  }

  /** Các `<option>` của một ô chọn, theo `id`. */
  function selectOptions(html: string, id: string): string {
    return new RegExp(`id="${id}"[^>]*>(.*?)</select>`).exec(html)?.[1] ?? "";
  }

  it("hai ô chọn Tử số / Mẫu số liệt kê CỘT SỐ theo tên, chọn sẵn theo bộ khởi điểm; KHÔNG ô gõ công thức", () => {
    const html = renderToStaticMarkup(
      <ColumnFieldsets kind="chi" columns={boCotKhoiDiem("chi", 2026)} onChange={() => {}} />,
    );

    expect(html).toContain("Tử số");
    expect(html).toContain("Mẫu số");
    expect(html).not.toContain("cot-congthuc");
    expect(html).not.toContain("Công thức");

    const numerator = selectOptions(html, "cot-tuso-2");
    const denominator = selectOptions(html, "cot-mauso-2");
    // Chỉ hai cột số — cột `%` không phải lựa chọn.
    expect(numerator).toContain(">Dự toán năm<");
    expect(numerator).toContain(">Chi ngân sách<");
    expect(numerator).not.toContain("So sánh TH/DT");
    expect(numerator).toContain('<option value="1" selected="">Chi ngân sách</option>');
    expect(denominator).toContain('<option value="0" selected="">Dự toán năm</option>');
  });

  it("bộ cột khởi điểm gửi ĐÚNG vị trí toán hạng và không gửi `formula`", () => {
    for (const kind of ["chi", "thu"] as const) {
      const kq = dungThanTaoBang({
        nam: 2026,
        loai: kind,
        tieuDe: "T",
        donVi: "trieu-dong",
        luyKe: "",
        cot: boCotKhoiDiem(kind, 2026),
      });
      expect(kq.ok).toBe(true);
      if (!kq.ok) return;
      const percent = kq.than.columns.find((c) => c.type === "phan_tram");
      expect(percent?.formula).toBeUndefined();
      expect(percent?.numerator_index).toBe(kind === "chi" ? 1 : 2);
      expect(percent?.denominator_index).toBe(0);
      for (const c of kq.than.columns.filter((x) => x.type === "so")) {
        expect(c.numerator_index).toBeUndefined();
        expect(c.denominator_index).toBeUndefined();
      }
    }
  });

  it("thiếu một toán hạng: TỪ CHỐI ở client, gọi đúng tên cột", () => {
    const columns = boCotKhoiDiem("chi", 2026).map((c) =>
      c.type === "phan_tram" ? { ...c, denominator_index: null } : c,
    );
    const kq = build(columns);
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("So sánh TH/DT (%)");
  });

  it("tử số trùng mẫu số: TỪ CHỐI ở client", () => {
    const columns = boCotKhoiDiem("chi", 2026).map((c) =>
      c.type === "phan_tram" ? { ...c, numerator_index: 0, denominator_index: 0 } : c,
    );
    const kq = build(columns);
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("hai cột khác nhau");
  });

  it("toán hạng là một cột phần trăm: TỪ CHỐI ở client", () => {
    const columns: finance_cotVao[] = [
      ...boCotKhoiDiem("chi", 2026),
      { name: "Tỷ lệ 2", order: 4, type: "phan_tram", numerator_index: 2, denominator_index: 0 },
    ];
    const kq = build(columns);
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain("cột số");
  });

  it("bỏ một cột đứng TRƯỚC: toán hạng dời theo, và vị trí gửi đi vẫn trỏ đúng hai cột đã chọn", () => {
    const columns: finance_cotVao[] = [
      { name: "Cột thừa", order: 1, type: "so" },
      { name: "Dự toán năm", order: 2, type: "so" },
      { name: "Chi ngân sách", order: 3, type: "so" },
      { name: "Tỷ lệ", order: 4, type: "phan_tram", numerator_index: 2, denominator_index: 1 },
    ];
    const kq = build(removeColumnAt(columns, 0));
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    const sent = kq.than.columns;
    const percent = sent[2];
    expect(sent[percent?.numerator_index ?? -1]?.name).toBe("Chi ngân sách");
    expect(sent[percent?.denominator_index ?? -1]?.name).toBe("Dự toán năm");
  });

  it("bỏ ĐÚNG cột đang là toán hạng: toán hạng ấy xoá trắng, lần gửi bị chặn cho tới khi chọn lại", () => {
    const after = removeColumnAt(boCotKhoiDiem("chi", 2026), 0);
    const percent = after.find((c) => c.type === "phan_tram");
    expect(percent?.denominator_index).toBeNull();
    expect(percent?.numerator_index).toBe(0);
    expect(build(after).ok).toBe(false);
  });

  it("đổi cột toán hạng sang `%`: cột `%` dùng nó mất toán hạng ấy; đổi `%` sang số thì bỏ toán hạng", () => {
    const start = boCotKhoiDiem("chi", 2026);
    const changed = changeColumnType(start, 1, "phan_tram");
    expect(changed[2]?.numerator_index).toBeNull();
    expect(changed[2]?.denominator_index).toBe(0);
    expect(changed[1]?.role).toBeUndefined();

    const back = changeColumnType(start, 2, "so");
    expect(back[2]?.numerator_index).toBeUndefined();
    expect(back[2]?.denominator_index).toBeUndefined();
    expect(percentOperandError(back, 2)).toBeNull();
  });
});

/** Nội dung dựng bên trong cổng quyền ở hai ca từ chối — tách ra để hai ca không chép lại nhau. */
function veBangJSX() {
  return (
    <BangDayDu
      sheetLock={null}
      duLieu={bang()}
      thuGon={new Set()}
      datThuGon={() => {}}
      coGhi
      coXacNhan
      dangGui={false}
      saveLine={async () => null}
      openLineOrder={() => {}}
      moThem={() => {}}
      moGoDong={() => {}}
      datTong={() => {}}
      moCachTinh={() => {}}
      moDot={() => {}}
    />
  );
}
