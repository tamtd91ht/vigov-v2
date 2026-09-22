import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { finance_danhSachDuAnRa, finance_duAnRa } from "@/lib/api/schema.gen";

import { BangDanhSach } from "./bang-du-an";
import { ThongTinDuAn } from "./chi-tiet-du-an";
import { CANH_BAO_KHONG_PHAI_KE_TOAN, nhanTyLeGiaiNgan } from "./nhan-du-an";

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

    expect(html).toContain("100.000.000 đ");
    expect(html).toContain("90,00%");
  });

  it("dự án chậm: chip điểm chậm CÓ MẶT, nguyên văn", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach duLieu={danhSach([duAn({ delay_score: 3136, is_delayed: true })])} danhMuc={[]} />,
    );

    expect(html).toContain("Chậm 31,36 điểm");
    expect(html).toContain("chip-cham");
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

  it("giải ngân vượt kế hoạch: số còn lại ÂM ra tới trang, không bị kẹp về 0", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={danhSach([duAn({ disbursed_amount: 110000000, remaining_amount: -10000000 })])}
        danhMuc={[]}
      />,
    );

    expect(html).toContain("-10.000.000 đ");
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
    const html = renderToStaticMarkup(<ThongTinDuAn duAn={duAn()} />);
    expect(html).not.toContain(CANH_BAO_KHONG_PHAI_KE_TOAN);
  });

  it("hai mốc ngày đứng RIÊNG: thời hạn giải ngân không thay được ngày hoàn thành", () => {
    // §9 nói thẳng: công trình xong tháng 3 vẫn có thể phải giải ngân trước 31/12. Gộp hai dòng
    // là mất đúng mốc bị hỏi khi quyết toán.
    const html = renderToStaticMarkup(
      <ThongTinDuAn duAn={duAn({ completion_date: "2026-03-20", disbursement_deadline: "2026-12-31" })} />,
    );

    expect(html).toContain("Thời hạn giải ngân");
    expect(html).toContain("Ngày hoàn thành");
    expect(html).toContain("20/03/2026");
    expect(html).toContain("31/12/2026");
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
