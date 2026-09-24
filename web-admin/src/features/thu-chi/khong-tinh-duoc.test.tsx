import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type {
  finance_bangDayDuRa,
  finance_chiSoNamRa,
  finance_cotRa,
  finance_danhSachDotRa,
  finance_dongRa,
} from "@/lib/api/schema.gen";

import { BangDayDu, DongKhoanMuc, FormSuaDong, TheChiSoNam, TheTomTat } from "./bang-thu-chi";
import { NoiDungHopDot } from "./dot-thu-chi";
import {
  donViCuaBang,
  dungGiaSuaDong,
  lyDoKhongTinh,
  O_KHONG_TINH_DUOC,
  O_TRONG,
} from "./nhan-thu-chi";

/**
 * "KHÔNG TÍNH ĐƯỢC" KHÁC "TRỐNG" — trên cây, thẻ tóm tắt, thẻ chỉ số, danh sách đợt, và ô sửa.
 *
 * Máy chủ gửi `null` cho cả hai (56d3224); câu lý do là thứ DUY NHẤT phân biệt. Trước bản vá này mọi
 * `null` vẽ thành `—`, nên một tổng tràn số trông y hệt một ô xã chưa khai — một câu sai về một con số
 * công, và mọi bài kiểm vẫn xanh. Mỗi ca dưới đây đặt HAI ô cạnh nhau (một trống, một không tính được)
 * để một bản vẽ gộp hai trạng thái làm một phải đỏ.
 */

// Câu thật của máy chủ (`domain.ErrTongVuotMuc`, `ErrGiaTriDaLuuVuotMuc`, `ErrTongDotVuotMuc`).
const CAU_TONG =
  "ngan_sach: tổng cộng ra vượt mức một con số ngân sách hiển thị chính xác được — không tính được; kiểm tra các số quá lớn ở các dòng bên dưới";
const CAU_DA_LUU =
  "ngan_sach: số đang lưu vượt mức một con số ngân sách hiển thị chính xác được — gõ lại số đúng hoặc gỡ đợt ghi nhầm";

const COT: finance_cotRa[] = [
  { id: "C1", name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam" },
  { id: "C2", name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach" },
];

/** Ô C1 KHÔNG TÍNH ĐƯỢC, ô C2 TRỐNG. */
function dongHaiTrangThai(sua: Partial<finance_dongRa> = {}): finance_dongRa {
  return {
    id: "I",
    parent_id: "A",
    no: "I",
    name: "Chi đầu tư phát triển",
    order: 1,
    method: "manual",
    level: 1,
    is_headline: false,
    values: { C1: null, C2: null },
    unavailable_reasons: { C1: CAU_DA_LUU },
    ...sua,
  };
}

function bang(dong: finance_dongRa[]): finance_bangDayDuRa {
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
    },
    columns: COT,
    lines: dong,
    summary: {
      headline_line_id: "A",
      cells: [
        { column_id: "C1", name: "Dự toán năm", value: null, unavailable_reason: CAU_TONG },
        { column_id: "C2", name: "Chi ngân sách", value: null },
      ],
      indicator: { name: "Chi đạt dự toán", basis_points: null },
    },
  };
}

/** Như `renderToStaticMarkup` thoát ký tự trong thuộc tính — so chuỗi thô sẽ xanh sai. */
function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** Nội dung các `<td>` của một chuỗi HTML, theo thứ tự. */
function cacO(html: string): string[] {
  return [...html.matchAll(/<td[^>]*>(.*?)<\/td>/g)].map((m) => m[1] ?? "");
}

describe("câu lý do: có mặt mới là 'không tính được'", () => {
  it("vắng, rỗng hay toàn khoảng trắng là ô bình thường", () => {
    expect(lyDoKhongTinh(undefined)).toBeNull();
    expect(lyDoKhongTinh("")).toBeNull();
    expect(lyDoKhongTinh("   ")).toBeNull();
    expect(lyDoKhongTinh(CAU_TONG)).toBe(CAU_TONG);
  });
});

describe("cây khoản mục", () => {
  function veDong(d: finance_dongRa): string {
    return renderToStaticMarkup(
      <table>
        <tbody>
          <DongKhoanMuc
            hien={{ dong: d, cap: 1, coCon: false, moRong: false }}
            cot={COT}
            donVi={donViCuaBang(bang([d]).sheet)}
            dongTongId=""
            coGhi={false}
            coXacNhan={false}
            dangGui={false}
            moRongDoi={() => {}}
            moSua={() => {}}
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

  it("ô không tính được KHÔNG vẽ thành `—`; ô trống bên cạnh VẪN là `—`", () => {
    const o = cacO(veDong(dongHaiTrangThai()));
    // [TT, Nội dung, C1, C2, Cách tính, Thao tác]
    expect(o[2]).toContain(O_KHONG_TINH_DUOC);
    expect(o[2]).not.toBe(O_TRONG);
    expect(o[3]).toBe(O_TRONG);
    expect(o[3]).not.toContain(O_KHONG_TINH_DUOC);
  });

  it("câu lý do đọc được KHÔNG CHỈ bằng tooltip: có trong `title` VÀ trong chữ ẩn thị giác", () => {
    const o = cacO(veDong(dongHaiTrangThai()))[2] ?? "";
    expect(o).toContain(`title="${nhuTrongHTML(CAU_DA_LUU)}"`);
    expect(o).toMatch(new RegExp(`class="an-thi-giac">[^<]*${CAU_DA_LUU}`));
    // Dấu ⚠ là trang trí — trình đọc màn hình không đọc "cảnh báo" rồi mới đọc câu.
    expect(o).toContain('aria-hidden="true"');
  });

  it("dòng không mang `unavailable_reasons` thì không có dấu nào — không vẽ thừa", () => {
    const html = veDong(dongHaiTrangThai({ unavailable_reasons: undefined }));
    expect(html).not.toContain(O_KHONG_TINH_DUOC);
  });

  it("dưới bảng: danh sách HIỆN RÕ nơi và câu của từng ô — cho màn cảm ứng không rê chuột được", () => {
    const d = dongHaiTrangThai();
    const html = renderToStaticMarkup(
      <BangDayDu
        duLieu={bang([d])}
        thuGon={new Set()}
        datThuGon={() => {}}
        coGhi={false}
        coXacNhan={false}
        dangGui={false}
        dangSuaDong={null}
        moSua={() => {}}
        huySua={() => {}}
        luuSua={() => {}}
        moThem={() => {}}
        moGoDong={() => {}}
        datTong={() => {}}
        moGoBang={() => {}}
        moSuaBang={() => {}}
        moCachTinh={() => {}}
        moDot={() => {}}
      />,
    );
    expect(html).toContain("Ô không tính được con số (1)");
    expect(html).toContain("<strong>I. Chi đầu tư phát triển — Dự toán năm</strong>: " + CAU_DA_LUU);
  });

  it("không ô nào không tính được thì không có danh sách", () => {
    const d = dongHaiTrangThai({ unavailable_reasons: undefined });
    const html = renderToStaticMarkup(
      <BangDayDu
        duLieu={bang([d])}
        thuGon={new Set()}
        datThuGon={() => {}}
        coGhi={false}
        coXacNhan={false}
        dangGui={false}
        dangSuaDong={null}
        moSua={() => {}}
        huySua={() => {}}
        luuSua={() => {}}
        moThem={() => {}}
        moGoDong={() => {}}
        datTong={() => {}}
        moGoBang={() => {}}
        moSuaBang={() => {}}
        moCachTinh={() => {}}
        moDot={() => {}}
      />,
    );
    expect(html).not.toContain("Ô không tính được con số");
  });
});

describe("thẻ tóm tắt", () => {
  it("ô tóm tắt không tính được hiện dấu KÈM câu; ô tóm tắt trống vẫn là `—`", () => {
    const b = bang([dongHaiTrangThai()]);
    const html = renderToStaticMarkup(
      <TheTomTat bang={b.sheet} tomTat={b.summary} soKhoanMuc={1} donVi={donViCuaBang(b.sheet)} />,
    );
    const dd = [...html.matchAll(/<dd>(.*?)<\/dd>/g)].map((m) => m[1] ?? "");
    expect(dd[0]).toContain(O_KHONG_TINH_DUOC);
    expect(dd[0]).toContain(CAU_TONG);
    expect(dd[1]).toBe(O_TRONG);
  });
});

describe("thẻ chỉ số — tổng thu", () => {
  it("tổng thu có câu lý do: dấu + câu hiện rõ, KHÔNG '—' và không gắn chữ 'đồng'; tổng thu trống vẫn '—'", () => {
    const cau = "ngan_sach: bảng thu chưa có dòng nào được đánh dấu là dòng tổng";
    const chiSo: finance_chiSoNamRa = {
      year: 2026,
      revenue_achievement: { name: "Thu đạt dự toán", basis_points: 10811 },
      expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
      balance: { amount: 853304000000 },
      revenue_totals: [
        { column_id: "T1", name: "Thu ngân sách NSNN", value: null, unavailable_reason: cau },
        { column_id: "T2", name: "Thu ngân sách Thu xã hưởng", value: null },
      ],
    };
    const html = renderToStaticMarkup(<TheChiSoNam chiSo={chiSo} />);
    const dd = [...html.matchAll(/<dd>(.*?)<\/dd>/g)].map((m) => m[1] ?? "");
    // [Thu đạt, Chi đạt, Chênh lệch, T1, T2]
    expect(dd[3]).toContain(O_KHONG_TINH_DUOC);
    expect(dd[3]).toContain(cau);
    expect(dd[3]).not.toContain("đồng<");
    expect(dd[4]).toBe(O_TRONG);
  });
});

describe("danh sách đợt", () => {
  const DOT: finance_danhSachDotRa = {
    line_id: "I",
    method: "entries",
    entries: [
      {
        id: "D1",
        line_id: "I",
        date: "2026-09-20",
        content: "Thu tiền sử dụng đất đợt 2",
        values: { C1: null, C2: null },
        unavailable_reasons: { C1: CAU_DA_LUU },
      },
    ],
  };

  it("số tiền vượt trần của một đợt hiện dấu; ô trống cạnh bên vẫn '—'; danh sách dưới bảng nói ngày và cột", () => {
    const html = renderToStaticMarkup(
      <NoiDungHopDot
        method="entries"
        cot={COT}
        donVi={donViCuaBang(bang([]).sheet)}
        danhSach={{ pha: "xong", duLieu: DOT }}
        coXacNhan={false}
        dangGui={false}
        moGo={() => {}}
      />,
    );
    const o = cacO(html);
    // [Ngày, Nội dung, Đơn vị cá nhân, Số chứng từ, C1, C2]
    expect(o[4]).toContain(O_KHONG_TINH_DUOC);
    expect(o[4]).toContain(`title="${nhuTrongHTML(CAU_DA_LUU)}"`);
    expect(o[4]).toMatch(new RegExp(`class="an-thi-giac">[^<]*${CAU_DA_LUU}`));
    expect(o[5]).toBe(O_TRONG);
    expect(html).toContain("Đợt ngày 20/9/2026 — Dự toán năm</strong>: " + CAU_DA_LUU);
  });
});

describe("ô sửa KHÔNG lặng lẽ xoá một ô không tính được", () => {
  function veForm(d: finance_dongRa): string {
    return renderToStaticMarkup(
      <table>
        <tbody>
          <FormSuaDong
            dong={d}
            cot={COT}
            donVi={donViCuaBang(bang([d]).sheet)}
            soCotBang={6}
            dangGui={false}
            huy={() => {}}
            luu={() => {}}
          />
        </tbody>
      </table>,
    );
  }

  const giaTriO = (html: string, id: string) =>
    new RegExp(`name="gia:${id}"[^>]*value="([^"]*)"`).exec(html)?.[1];

  it("ô không tính được điền chữ 'Không tính được', KHÔNG điền rỗng; ô trống thật vẫn điền rỗng", () => {
    const html = veForm(dongHaiTrangThai());
    expect(giaTriO(html, "C1")).toBe(O_KHONG_TINH_DUOC);
    expect(giaTriO(html, "C2")).toBe("");
    // Câu lý do và cách giữ nguyên được NÓI RA, và gắn vào ô bằng `aria-describedby`.
    expect(html).toContain('aria-describedby="sua-gia-goi-y-I-C1"');
    expect(html).toContain(CAU_DA_LUU);
  });

  it("lưu mà KHÔNG chạm ô ấy: mã cột VẮNG MẶT trong `values` — máy chủ giữ nguyên con số", () => {
    const d = dongHaiTrangThai();
    const kq = dungGiaSuaDong({ C1: O_KHONG_TINH_DUOC, C2: "" }, COT, d.unavailable_reasons, "trieu-dong");
    expect(kq).toEqual({ ok: true, than: { C2: null } });
    // Chính xác là VẮNG, không phải `null` — `null` là xoá trắng.
    if (kq.ok) expect(kq.than).not.toHaveProperty("C1");
  });

  it("gõ số mới thì ghi số ấy; xoá trắng thì xoá — như mọi ô khác", () => {
    const d = dongHaiTrangThai();
    expect(dungGiaSuaDong({ C1: "1,5", C2: "" }, COT, d.unavailable_reasons, "trieu-dong")).toEqual({
      ok: true,
      than: { C1: 1500000, C2: null },
    });
    expect(dungGiaSuaDong({ C1: "", C2: "" }, COT, d.unavailable_reasons, "trieu-dong")).toEqual({
      ok: true,
      than: { C1: null, C2: null },
    });
  });

  it("chữ 'Không tính được' ở một ô KHÔNG có lý do thì bị từ chối, không bị bỏ qua", () => {
    // Chỉ ô máy chủ nói là không tính được mới được "để nguyên" bằng chữ ấy; ở ô khác nó là một lần
    // gõ hỏng và phải dừng lại, không lặng lẽ biến thành "không nhắc tới".
    const kq = dungGiaSuaDong({ C1: "", C2: O_KHONG_TINH_DUOC }, COT, { C1: CAU_DA_LUU }, "trieu-dong");
    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toContain('Ô "Chi ngân sách"');
  });
});
