import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { finance_chungTuRa, finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";

import { BangChungTu, FormChungTu, giaTriTuChungTu } from "./chung-tu-du-an";
import { FormDuAn, giaTriTuDuAn, KhoiSuaXoaDuAn, KhoiThemDuAn } from "./ghi-du-an";
import { KhoiChuaDungGhi } from "./khoi-chua-dung-ghi";
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
const NHAN_SUA = `✎ Sửa chứng từ ngày ${NGAY}`;
const NHAN_XAC_NHAN = `Xác nhận chứng từ ngày ${NGAY}`;
const NHAN_KHOA = `Khoá chứng từ ngày ${NGAY}`;
const NHAN_MO_KHOA = `Mở khoá chứng từ ngày ${NGAY}`;
const NHAN_GO = `🗑 Gỡ chứng từ ngày ${NGAY}`;

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

  it("bảng rỗng NÓI RÕ vì sao nó rỗng — không để đọc thành 'dự án chưa chi đồng nào'", () => {
    const html = veBang(true, true, []);

    expect(html).toContain("phiên làm việc này");
    expect(html).toContain("chưa có tuyến đọc danh sách chứng từ");
  });
});

describe("BIỂU MẪU CHỨNG TỪ — cảnh báo ADR 0036", () => {
  it("sửa một chứng từ ĐÃ XÁC NHẬN: cảnh báo 'về Kế toán nhập' hiện NGAY TRÊN các ô", () => {
    const html = renderToStaticMarkup(
      <FormChungTu
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

describe("NÚT THÊM DỰ ÁN — cổng budget.update", () => {
  it("thiếu `budget.update`: KHÔNG có nút, và câu từ chối GỌI ĐÚNG TÊN KHOÁ", () => {
    const html = renderToStaticMarkup(
      <KhoiThemDuAn nam={2026} danhMuc={HANG_MUC} coGhi={false} daGhiXong={() => {}} />,
    );

    expect(html).not.toContain("+ Thêm dự án");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_GHI));
    expect(html).toContain("budget.update");
  });

  it("có `budget.update`: nút hiện", () => {
    const html = renderToStaticMarkup(
      <KhoiThemDuAn nam={2026} danhMuc={HANG_MUC} coGhi daGhiXong={() => {}} />,
    );

    expect(html).toContain("+ Thêm dự án");
  });
});

describe("SỬA / GỠ DỰ ÁN — hai cổng riêng", () => {
  it("`budget.update` MỘT MÌNH: có Sửa dự án, KHÔNG có Gỡ dự án", () => {
    const html = renderToStaticMarkup(
      <KhoiSuaXoaDuAn
        duAn={DU_AN}
        danhMuc={HANG_MUC}
        coGhi
        coXacNhan={false}
        daSuaXong={() => {}}
        daXoaXong={() => {}}
      />,
    );

    expect(html).toContain("✎ Sửa dự án");
    expect(html).not.toContain("🗑 Gỡ dự án");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_XAC_NHAN));
    expect(html).toContain("budget.confirm");
  });

  it("`budget.confirm` MỘT MÌNH: có Gỡ dự án, KHÔNG có Sửa dự án", () => {
    const html = renderToStaticMarkup(
      <KhoiSuaXoaDuAn
        duAn={DU_AN}
        danhMuc={HANG_MUC}
        coGhi={false}
        coXacNhan
        daSuaXong={() => {}}
        daXoaXong={() => {}}
      />,
    );

    expect(html).toContain("🗑 Gỡ dự án");
    expect(html).not.toContain("✎ Sửa dự án");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_GHI));
  });

  it("không khoá nào: cả hai câu từ chối, không nút nào", () => {
    const html = renderToStaticMarkup(
      <KhoiSuaXoaDuAn
        duAn={DU_AN}
        danhMuc={HANG_MUC}
        coGhi={false}
        coXacNhan={false}
        daSuaXong={() => {}}
        daXoaXong={() => {}}
      />,
    );

    expect(html).not.toContain("✎ Sửa dự án");
    expect(html).not.toContain("🗑 Gỡ dự án");
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_GHI));
    expect(html).toContain(nhuTrongHTML(CAU_THIEU_QUYEN_XAC_NHAN));
  });
});

describe("BIỂU MẪU DỰ ÁN", () => {
  it("danh mục hạng mục RỖNG: nói trước, và nút Lưu bị tắt", () => {
    const html = renderToStaticMarkup(
      <FormDuAn
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={[]}
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
    expect(html).toContain('<button type="submit" class="nut-chinh" disabled="">Lưu dự án</button>');
  });

  it("khi SỬA: mã dự án chỉ ĐỌC, không có ô nhập mã", () => {
    const html = renderToStaticMarkup(
      <FormDuAn
        tieuDeForm="Sửa dự án"
        giaTriDau={giaTriTuDuAn(DU_AN)}
        maChiDoc={DU_AN.code}
        danhMuc={HANG_MUC}
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
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={HANG_MUC}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );

    expect(html).toContain('id="ma-du-an"');
    expect(html).toContain("Mã đã cấp thì không cấp lại");
  });

  it("giá trị ban đầu của biểu mẫu SỬA là SỐ THÔ, không phải chuỗi đã định dạng", () => {
    // `100.000.000` trong ô nhập làm phép so "có đổi không" thấy khác và gửi một `PATCH` thừa.
    expect(giaTriTuDuAn(DU_AN).keHoachVon).toBe("100000000");
    expect(giaTriTuChungTu(chungTu()).soTien).toBe("30000000");
  });
});

describe("KHỐI PHẦN CHƯA DỰNG", () => {
  it("MỌI mục ra tới HTML — tên và lý do, từng mục một", () => {
    const html = renderToStaticMarkup(<KhoiChuaDungGhi />);

    for (const p of PHAN_CHUA_DUNG_GHI) {
      expect(html).toContain(nhuTrongHTML(p.ten));
      expect(html).toContain(nhuTrongHTML(p.viSao));
    }
    expect(html).toContain(`${PHAN_CHUA_DUNG_GHI.length} phần`);
  });

  it("mục đầu tiên nói ra điều nặng nhất: không có tuyến ĐỌC danh sách chứng từ", () => {
    const html = renderToStaticMarkup(<KhoiChuaDungGhi />);

    expect(html).toContain(nhuTrongHTML("KHÔNG " + "có tuyến nào đọc danh sách chứng từ"));
  });
});
