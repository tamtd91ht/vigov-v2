import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { finance_danhSachDuAnRa, finance_duAnRa } from "@/lib/api/schema.gen";

import { BangDanhSach } from "./bang-du-an";
import { ThongTinDuAn } from "./chi-tiet-du-an";
import * as projectLabels from "./nhan-du-an";
import { nhanTyLeGiaiNgan } from "./nhan-du-an";
import { ScopeNotice } from "./scope-notice";

/** A commune-reworded `budget.scope_notice` — deliberately NOT the software's default wording. */
const COMMUNE_WORDING = "Số liệu trên màn này chỉ để điều hành, không thay sổ kế toán của xã.";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — bổ cho các ca kiểm module thuần, vốn chỉ canh
 * quyết định bên trong hàm. Đã đo trên màn Danh mục: bôi trắng câu quan trọng nhất của một màn
 * hình vẫn để lại 167 ca xanh và `tsc` sạch.
 */

function duAn(sua: Partial<finance_duAnRa> = {}): finance_duAnRa {
  return {
    id: "01JDUAN1",
    code: "DA-2026-be-tong-hoa-duong-ngo-xo-2",
    year: 2026,
    category_id: "",
    name: "Bê tông hoá đường ngõ xóm tổ 6",
    planned_amount: 100000000,
    approved_amount: 100000000,
    disbursed_amount: 90000000,
    remaining_amount: 10000000,
    disbursed_ratio: 9000,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    // Ngưỡng cảnh báo chậm nay của TỪNG XÃ, đọc từ `cau_hinh_giai_ngan` chứ không còn là hằng số
    // trong kho mã nhà cung cấp (luật 1 bất biến 10, câu mở #31 chốt 22/09/2026). `…_source` nói
    // con số ấy đến từ đâu — `mac_dinh` khi xã chưa khai — nên một màn hình đọc được nó phân biệt
    // được "xã đã chọn 10" với "chưa ai chọn gì nên lấy 10".
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...sua,
  };
}

function danhSach(items: finance_duAnRa[]): finance_danhSachDuAnRa {
  return { items, year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" };
}

describe("bảng dự án kết xuất ra trang", () => {
  it("số tiền và tỷ lệ ra tới trang ở dạng đã định dạng, không phải số thô", () => {
    const html = renderToStaticMarkup(<BangDanhSach duLieu={danhSach([duAn()])} danhMuc={[]} />);

    // G6 (user decision 07/10/2026): the list prints the prototype's short form; the full đồng stays
    // in the cell (hover title + visually-hidden text), no longer as the visible figure.
    expect(html).toContain(">100 triệu<");
    expect(html).toContain("100.000.000 đ");
    expect(html).toContain(">90%<");
  });

  it("dự án chậm: chip điểm chậm CÓ MẶT, nguyên văn", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach duLieu={danhSach([duAn({ delay_score: 3136, is_delayed: true })])} danhMuc={[]} />,
    );

    // Spec 02 §8: lowercase "chậm x điểm" with the triangle, small red words under the name.
    expect(html).toContain("chậm 31,36 điểm");
    expect(html).toContain("lucide-triangle-alert");
    // The prototype's red left edge + faint red fill on the row. Only a late project paints red here.
    expect(html).toContain("border-l-danger");
    expect(html).toContain("bg-danger/3");
    expect(html).toContain("text-danger");
  });

  it("dự án bám sát tiến độ KHÔNG mang dấu chậm — viền đỏ và chữ 'Chậm' chỉ dành cho dự án máy chủ báo chậm", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={danhSach([duAn({ delay_score: 500, is_delayed: false }), duAn({ id: "01JDUAN2", disbursed_ratio: null })])}
        danhMuc={[]}
      />,
    );

    // The prototype's list says nothing for an on-track project ("Bám sát tiến độ" is on the
    // project page); a project with no capital still says so in words. The bar's colour is the 80/50/30
    // tier since 07/10/2026 (`progress-tone.ts`) — 90% here, so green; the LATE signal is the edge.
    expect(html).toContain("Chưa bố trí vốn");
    expect(html).not.toContain("border-l-danger");
    expect(html).not.toContain("text-danger");
    expect(html).not.toContain("bg-danger");
  });

  it("dự án chưa bố trí vốn KHÔNG bao giờ hiện thành 0%", () => {
    // Hiện 0% cho một dự án chưa ai bố trí vốn là báo cáo nó như dự án tệ nhất của xã.
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={danhSach([duAn({ disbursed_ratio: null, delay_score: null })])}
        danhMuc={[]}
      />,
    );

    expect(html).toContain(nhanTyLeGiaiNgan(null));
    expect(html).not.toContain("0,00%");
  });

  it("giải ngân vượt kế hoạch: tỷ lệ TRÊN 100% ra tới bảng, không bị kẹp", () => {
    // The list has no "Còn lại" column (prototype); the over-plan shows as the ratio, the bar is
    // capped at full width but the words keep the real figure. The negative remainder: project page.
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={danhSach([
          duAn({ disbursed_amount: 110000000, remaining_amount: -10000000, disbursed_ratio: 11000 }),
        ])}
        danhMuc={[]}
      />,
    );

    expect(html).toContain(">110%<");
    expect(html).toContain("width:100%");
  });

  it("spec 02 §8: NO category sub-line under the name (the group row names it)", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={danhSach([duAn({ category_id: "01JHM1" })])}
        danhMuc={[
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
        ]}
      />,
    );
    expect(html).not.toContain("Các công trình chuyển tiếp");
  });

  it("năm trên bảng lấy từ PHẢN HỒI, không từ ô chọn", () => {
    // Một phản hồi không nói nó thuộc năm nào thì không phân biệt được với phản hồi của năm khác.
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={{ items: [duAn()], year: 2024, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }}
        danhMuc={[]}
      />,
    );

    expect(html).toContain("2024");
  });

  it("đường dẫn chi tiết mã hoá id, đúng đường dẫn con của đặc tả", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach duLieu={danhSach([duAn({ id: "a/b" })])} danhMuc={[]} />,
    );

    expect(html).toContain("/giai-ngan/du-an/a%2Fb");
  });
});

describe("trang chi tiết dự án kết xuất ra trang", () => {
  it("banner 'không phải phần mềm kế toán' KHÔNG nằm trong khối chi tiết — nó ở đầu màn", () => {
    // Canh đúng chỗ: khối chi tiết chỉ nói về dự án. Banner là phạm vi của cả màn hình và được
    // dựng ở `ChiTietDuAn`, không lặp lại bên trong từng khối.
    const html = renderToStaticMarkup(<ThongTinDuAn duAn={duAn({ scope_notice: COMMUNE_WORDING })} />);
    expect(html).not.toContain(COMMUNE_WORDING);
  });

  it("spec 07 figure grid: SIX cells; `Thời gian thực hiện` is start → completion, never the deadline", () => {
    // The disbursement deadline is not a cell of the project card (spec 07 §Body 2); it stays on the
    // list and in the edit form. The completion date is therefore never shown as if it were it.
    const html = renderToStaticMarkup(
      <ThongTinDuAn duAn={duAn({ completion_date: "2026-03-20", disbursement_deadline: "2026-12-31" })} />,
    );

    expect(html.match(/<dt /g)).toHaveLength(6);
    expect(html).toContain("Thời gian thực hiện");
    expect(html).toContain("Chưa đặt → 20/3/2026");
    expect(html).not.toContain("31/12/2026");
  });

  it("giải ngân vượt kế hoạch: số còn lại ÂM ra tới trang, không bị kẹp về 0", () => {
    const html = renderToStaticMarkup(
      <ThongTinDuAn duAn={duAn({ disbursed_amount: 110000000, remaining_amount: -10000000 })} />,
    );
    expect(html).toContain("-10.000.000 đ");
  });

  it("ngày chưa đặt nói ra bằng chữ, không để ô trống", () => {
    const html = renderToStaticMarkup(<ThongTinDuAn duAn={duAn()} />);
    expect(html).toContain("Chưa đặt");
  });

  it("KHÔNG in id nội bộ của bộ phận hay cán bộ phụ trách lên màn hình", () => {
    // Hợp đồng chỉ trả ID; in một chuỗi ULID cho cán bộ đọc không nói với ai điều gì, và tra nó
    // thành tên người là việc của tuyến khác dưới quyền khác.
    const html = renderToStaticMarkup(
      <ThongTinDuAn duAn={duAn({ org_unit_id: "01JBOPHAN", assignee_id: "01JCANBO" })} />,
    );

    expect(html).not.toContain("01JBOPHAN");
    expect(html).not.toContain("01JCANBO");
  });
});

describe("banner phạm vi (`scope_notice`) — câu của máy chủ, không câu của web", () => {
  it("hiện NGUYÊN VĂN câu máy chủ gửi, kể cả khi xã đã sửa lời", () => {
    const html = renderToStaticMarkup(<ScopeNotice text={COMMUNE_WORDING} />);
    // Presentational pin (ADR 0068 §5): the markup is now the neutral `Notice`, so the exact-string
    // pin became "the sentence is there, verbatim, once — and it is not announced as an alert".
    expect(html.split(COMMUNE_WORDING)).toHaveLength(2);
    expect(html).not.toContain('role="alert"');
  });

  it("vắng hoặc trống thì KHÔNG hiện gì — không tự bịa câu dự phòng", () => {
    expect(renderToStaticMarkup(<ScopeNotice text={undefined} />)).toBe("");
    expect(renderToStaticMarkup(<ScopeNotice text="   " />)).toBe("");
  });

  it("không còn hằng câu banner ở client — bản sao thứ hai của lời hệ thống `budget.scope_notice`", () => {
    // Hằng cũ `CANH_BAO_KHONG_PHAI_KE_TOAN` giữ lời mặc định; xã sửa lời thì màn này vẫn hiện câu cũ.
    expect(Object.keys(projectLabels)).not.toContain("CANH_BAO_KHONG_PHAI_KE_TOAN");
  });

  it("`scope_notice` là trường của hợp đồng, ở cả danh sách lẫn từng dự án", () => {
    const list: finance_danhSachDuAnRa = { ...danhSach([]), scope_notice: COMMUNE_WORDING };
    const project: finance_duAnRa = duAn({ scope_notice: COMMUNE_WORDING });
    expect(renderToStaticMarkup(<ScopeNotice text={list.scope_notice} />)).toContain(COMMUNE_WORDING);
    expect(renderToStaticMarkup(<ScopeNotice text={project.scope_notice} />)).toContain(COMMUNE_WORDING);
  });
});
