import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { baiTinCuaXa, danhBaCanBoXa, traXaTheoTenMien, tinCuaXa } from "./api/goi-vigov";
import {
  chiaDoan,
  docBaiTin,
  docDanhBa,
  docTrangTinXa,
  docXa,
  DUONG_DAN_DANH_BA,
  DUONG_DAN_TIN_XA,
  DUONG_DAN_XA,
} from "./api/hop-dong-cong-khai";
import { type KetQuaMoPhien, moPhienSauXacNhan, type YeuCauMoPhien } from "./api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "./api/phien-vigov";
import { dichGoi, sauKhiTaiDanhBa, TheCanBo, ThanDanhBa } from "./man/DanhBaCanBoScreen";
import { KenhCongDan } from "./man/KenhCongDan";
import { CHUA_DANG_NHAP_XA, CUA_TOI, DANH_BA, GUI, TIN_XA, TRA_CUU, XAC_NHAN_XA } from "./man/noi-dung";
import {
  BaiTin,
  batDauTaiTin,
  sauKhiTaiBai,
  sauKhiTaiTin,
  ThanBaiTin,
  ThanTinXa,
  TIN_DAU,
} from "./man/TinTucXaScreen";
import { buocSauMoPhien, buocSauTraXa, ManXacNhanXa } from "./man/XacNhanXa";

/**
 * NỬA NHÀ NƯỚC NÓI CHUYỆN VỚI MÁY CHỦ — màn xác nhận xã, mở phiên qua hàm tiêm vào, tin tức và danh
 * bạ của xã. Dựng bằng `react-dom/server` và hàm thuần (không có DOM để bấm): mỗi bước của màn là
 * một hàm `buoc…`/`sauKhi…` được kiểm thẳng, và mỗi lời gọi mạng đi qua `fetch` giả.
 *
 * Tên miền, tên xã, họ tên và số điện thoại ở đây đều GIẢ. Số điện thoại là số giả đã thống nhất.
 */

const TEN_MIEN = "xa-vi-du.vigov.example";

type LoiGoi = { dia_chi: string; tuy_chon: RequestInit };
let loi_goi: LoiGoi[] = [];

function traLoi(status: number, than: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => than };
}

function datFetch(...phan_hoi: Array<ReturnType<typeof traLoi> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (dia_chi: string, tuy_chon: RequestInit) => {
    loi_goi.push({ dia_chi, tuy_chon });
    const p = phan_hoi[Math.min(i++, phan_hoi.length - 1)]!;
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, " ")
    .replace(/\s+/g, " ");

beforeEach(() => {
  loi_goi = [];
  datPhienViGov(null);
});

afterEach(() => {
  vi.unstubAllGlobals();
  datPhienViGov(null);
});

// ---------------------------------------------------------------------------------------------

describe("tra xã theo tên miền — `GET identity /api/v1/communes?host=`", () => {
  it("GET đúng host của identity, `host` mã hoá, KHÔNG bearer, KHÔNG thân", async () => {
    datFetch(traLoi(200, { items: [{ name: "Xã Thử Nghiệm", province: "Tỉnh Ví Dụ" }] }));
    const kq = await traXaTheoTenMien(TEN_MIEN);
    expect(kq).toEqual({ kieu: "xong", gia_tri: [{ ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" }] });
    expect(loi_goi).toHaveLength(1);
    const g = loi_goi[0]!;
    expect(g.dia_chi).toBe(`https://identity.api.vigov.vn${DUONG_DAN_XA}?host=${TEN_MIEN}`);
    expect(g.tuy_chon.method).toBe("GET");
    expect(g.tuy_chon.body).toBeUndefined();
    expect(Object.keys(g.tuy_chon.headers as Record<string, string>)).toEqual(["Accept"]);
  });

  it("KHÔNG gọi mạng khi tên miền rỗng hoặc sai khuôn", async () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);
    for (const d of ["", "localhost", "a b.vn", "xa.vn/duong", "xa.vn?host=khac"]) {
      expect(await traXaTheoTenMien(d), d).toEqual({ kieu: "khong-hop-le" });
      expect(await danhBaCanBoXa(d), d).toEqual({ kieu: "khong-hop-le" });
      expect(await tinCuaXa(d, ""), d).toEqual({ kieu: "khong-hop-le" });
      expect(await baiTinCuaXa(d, "t1"), d).toEqual({ kieu: "khong-hop-le" });
    }
    expect(fetch_gia).not.toHaveBeenCalled();
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh", async () => {
    const CA: Array<[ReturnType<typeof traLoi> | Error, string]> = [
      [traLoi(400, { error: "x" }), "khong-hop-le"],
      [traLoi(503, {}), "tam-ngung"],
      [traLoi(500, {}), "loi-may-chu"],
      [traLoi(401, {}), "loi-may-chu"],
      [traLoi(200, { items: [{ name: 1 }] }), "loi-may-chu"],
      [new Error("mat mang"), "loi-mang"],
    ];
    for (const [phan_hoi, kieu] of CA) {
      datFetch(phan_hoi);
      expect((await traXaTheoTenMien(TEN_MIEN)).kieu).toBe(kieu);
    }
  });

  it("parser: rỗng là rỗng, sai kiểu là `null`", () => {
    expect(docXa({ items: [] })).toEqual([]);
    expect(docXa({})).toBeNull();
    expect(docXa({ items: [{ name: "a" }] })).toBeNull();
    expect(docXa(null)).toBeNull();
  });
});

describe("màn xác nhận xã — tên từ máy chủ, fail closed ở mọi nhánh khác", () => {
  const XA = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" };

  it("máy chủ trả MỘT xã, nguồn qr → hỏi 'Làm việc với <tên>, <tỉnh>'", () => {
    const b = buocSauTraXa({ kieu: "xong", gia_tri: [XA] }, "qr");
    expect(b).toEqual({ trang: { kieu: "hoi", xa: XA } });
    if (!("trang" in b)) throw new Error("không tới bước hỏi");
    const chu = textOf(
      renderToStaticMarkup(
        createElement(ManXacNhanXa, { trang: b.trang, nguon: "qr", onXacNhan: () => {}, onKhongPhai: () => {} }),
      ),
    );
    expect(chu).toContain("làm việc với Xã Thử Nghiệm, Tỉnh Ví Dụ");
    expect(chu).toContain("Bạn cần liên hệ với xã này?");
    expect(chu).not.toContain("vigov.example");
  });

  it("rỗng · nhiều hơn một xã · tên rỗng · 400 → về giới thiệu với câu 'mã QR chưa dẫn tới xã nào'", () => {
    for (const kq of [
      { kieu: "xong", gia_tri: [] },
      { kieu: "xong", gia_tri: [XA, { ten: "Xã Khác", tinh: "Tỉnh Khác" }] },
      { kieu: "xong", gia_tri: [{ ten: "  ", tinh: "Tỉnh Ví Dụ" }] },
      { kieu: "khong-hop-le" },
    ] as const) {
      expect(buocSauTraXa(kq, "qr"), JSON.stringify(kq)).toEqual({
        ket_thuc: { kieu: "ve-gioi-thieu", cau: XAC_NHAN_XA.khong_thay },
      });
    }
  });

  it("503 · 500 · mất mạng → về giới thiệu với câu 'chưa kết nối được'", () => {
    for (const kieu of ["tam-ngung", "loi-may-chu", "loi-mang", "chua-cau-hinh"] as const) {
      expect(buocSauTraXa({ kieu }, "qr"), kieu).toEqual({
        ket_thuc: { kieu: "ve-gioi-thieu", cau: XAC_NHAN_XA.chua_ket_noi },
      });
    }
  });

  it("nguồn không tin được thì không hỏi, kể cả khi máy chủ trả một xã", () => {
    expect("ket_thuc" in buocSauTraXa({ kieu: "xong", gia_tri: [XA] }, "share")).toBe(true);
  });

  it("bước chờ: một câu `role=status`, không tên xã, không nút", () => {
    const html = renderToStaticMarkup(
      createElement(ManXacNhanXa, {
        trang: { kieu: "dang-tra" },
        nguon: "qr",
        onXacNhan: () => {},
        onKhongPhai: () => {},
      }),
    );
    expect(html).toContain('role="status"');
    expect(textOf(html)).toContain(XAC_NHAN_XA.dang_tra);
    expect(html).not.toMatch(/<button\b/);
  });

  it("đang mở phiên: hai nút KHÔNG bấm được, câu chờ được đọc ra", () => {
    const html = renderToStaticMarkup(
      createElement(ManXacNhanXa, { trang: { kieu: "dang-mo", xa: XA }, nguon: "qr", onXacNhan: () => {}, onKhongPhai: () => {} }),
    );
    expect(html.match(/<button[^>]*disabled/g) ?? []).toHaveLength(2);
    expect(textOf(html)).toContain(XAC_NHAN_XA.dang_mo);
  });

  it("các câu của lớp khám phá không mang mã lỗi, tên dịch vụ hay tên miền", () => {
    for (const cau of Object.values(XAC_NHAN_XA)) {
      expect(cau).not.toMatch(/\b[45]\d\d\b|error|identity|vigov|host|token/i);
      expect(cau.length).toBeGreaterThan(20);
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("xác nhận → mở phiên qua hàm TIÊM VÀO", () => {
  it("gọi hàm tiêm với ĐÚNG tên miền và `communeConfirmed: true`, rồi phiên có trong bộ nhớ", async () => {
    const goi: YeuCauMoPhien[] = [];
    const mo = async (yc: YeuCauMoPhien): Promise<KetQuaMoPhien> => {
      goi.push(yc);
      return { kieu: "xong", token: "tok-vigov-thu", ten_xa: "Xã Của Phiên", ten_mien: null };
    };
    expect(layPhienViGov()).toBeNull();
    const kq = await moPhienSauXacNhan(mo, TEN_MIEN);
    expect(goi).toEqual([{ communeHostHint: TEN_MIEN, communeConfirmed: true }]);
    // Bearer KHÔNG đi ngược lên màn hình — chỉ tên xã (và tên miền chính của phiên, nếu có).
    expect(kq).toEqual({ kieu: "da-mo", ten_xa: "Xã Của Phiên", ten_mien: null });
    expect(layPhienViGov()).toEqual({ token: "tok-vigov-thu", ten_xa: "Xã Của Phiên" });
  });

  it("tên miền chính của phiên đi lên màn hình khi đúng khuôn; sai khuôn thì `null` — kiểm lại ở nửa này", async () => {
    const voi = (ten_mien: string | null) =>
      moPhienSauXacNhan(async () => ({ kieu: "xong", token: "tok", ten_xa: "Xã Của Phiên", ten_mien }), TEN_MIEN);
    expect(await voi("xa-khac.vigov.example")).toEqual({
      kieu: "da-mo",
      ten_xa: "Xã Của Phiên",
      ten_mien: "xa-khac.vigov.example",
    });
    for (const sai of ["", "localhost", "https://xa.vn", "Xa.Vn", "xa.vn?host=khac"]) {
      expect(await voi(sai), sai).toMatchObject({ kieu: "da-mo", ten_mien: null });
    }
    // Tên miền KHÔNG vào nguồn phiên: bearer và tên xã là tất cả những gì `phien-vigov.ts` giữ.
    expect(layPhienViGov()).toEqual({ token: "tok", ten_xa: "Xã Của Phiên" });
  });

  it("tên xã của phiên thắng tên màn xác nhận đã hiện (ADR 0047 §Trả lời mục 4)", () => {
    expect(
      buocSauMoPhien({ ten: "Tên Đã Hiện", tinh: "T" }, { kieu: "da-mo", ten_xa: "Tên Của Phiên", ten_mien: TEN_MIEN }),
    ).toEqual({
      ket_thuc: { kieu: "da-mo", ten_xa: "Tên Của Phiên", ten_mien: TEN_MIEN },
    });
  });

  it("cầu tắt · phiên không bearer · phiên không tên xã → KHÔNG có phiên, nhưng xã ĐÃ xác nhận: mở phần công khai", async () => {
    const xa = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" };
    for (const tra of [
      { kieu: "chua-mo" },
      { kieu: "xong", token: "", ten_xa: "Xã Của Phiên", ten_mien: null },
      { kieu: "xong", token: "tok", ten_xa: " ", ten_mien: TEN_MIEN },
    ] as const) {
      const kq = await moPhienSauXacNhan(async () => tra, TEN_MIEN);
      expect(kq, JSON.stringify(tra)).toEqual({ kieu: "chua-mo" });
      expect(layPhienViGov()).toBeNull();
      // Tên và tỉnh là của `/communes` — thứ công dân vừa đọc và bấm xác nhận — không dựng từ tham số.
      expect(buocSauMoPhien(xa, kq)).toEqual({ ket_thuc: { kieu: "xac-nhan-khong-phien", xa } });
    }
  });

  it("hàm tiêm ném lỗi hoặc báo thử lại → ở lại màn xác nhận với câu 'bấm lần nữa'", async () => {
    const kq = await moPhienSauXacNhan(async () => {
      throw new Error("x");
    }, TEN_MIEN);
    expect(kq).toEqual({ kieu: "thu-lai" });
    expect(layPhienViGov()).toBeNull();
    const xa = { ten: "a", tinh: "b" };
    expect(buocSauMoPhien(xa, kq)).toEqual({ trang: { kieu: "hoi", xa, cau_loi: XAC_NHAN_XA.thu_lai } });
  });

  it("ngoài Zalo → không phiên, nhưng xã đã xác nhận: phần công khai vẫn mở", () => {
    const xa = { ten: "a", tinh: "b" };
    expect(buocSauMoPhien(xa, { kieu: "ngoai-zalo" })).toEqual({ ket_thuc: { kieu: "xac-nhan-khong-phien", xa } });
  });
});

// ---------------------------------------------------------------------------------------------

describe("kênh công dân: hai lối vào công khai chỉ có khi đã biết tên miền xã", () => {
  it("không tên miền → không có 'Tin tức của xã', không có 'Danh bạ cán bộ xã'", () => {
    for (const ten_mien of [undefined, null]) {
      const html = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {}, ten_mien }));
      expect(html).not.toContain(TIN_XA.tieu_de);
      expect(html).not.toContain(DANH_BA.tieu_de);
    }
  });

  it("có tên miền → hai nút, mỗi nút là một `cd-nut` 48px", () => {
    const html = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {}, ten_mien: TEN_MIEN }));
    expect(html).toContain(`<button type="button" class="cd-nut">${TIN_XA.tieu_de}</button>`);
    expect(html).toContain(`<button type="button" class="cd-nut">${DANH_BA.tieu_de}</button>`);
    expect(html).not.toContain(TEN_MIEN);
  });
});

describe("kênh công dân KHÔNG phiên — nói ra, không im lặng", () => {
  it("chưa có phiên → câu `role=status` TRƯỚC ba lối phản ánh; ba lối vẫn còn", () => {
    const html = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {}, ten_mien: TEN_MIEN }));
    const chu = textOf(html);
    expect(html).toMatch(/<p class="cd-loi" role="status">/);
    expect(chu).toContain(CHUA_DANG_NHAP_XA.cau);
    expect(chu).toContain(CHUA_DANG_NHAP_XA.con_lai);
    // Câu đứng TRƯỚC nút đầu tiên: người dân đọc nó trước khi bấm.
    expect(html.indexOf(CHUA_DANG_NHAP_XA.cau)).toBeLessThan(html.indexOf(GUI.tieu_de));
    for (const nhan of [GUI.tieu_de, CUA_TOI.tieu_de, TRA_CUU.tieu_de]) {
      expect(html).toContain(`<button type="button" class="cd-nut">${nhan}</button>`);
    }
  });

  it("không tên miền → không hứa 'tin tức và danh bạ ở dưới'", () => {
    const chu = textOf(renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {}, ten_mien: null })));
    expect(chu).toContain(CHUA_DANG_NHAP_XA.cau);
    expect(chu).not.toContain(CHUA_DANG_NHAP_XA.con_lai);
  });

  it("CÓ phiên → không có câu 'chưa đăng nhập được'", () => {
    datPhienViGov({ token: "tok", ten_xa: "Xã Của Phiên" });
    const html = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {}, ten_mien: TEN_MIEN }));
    expect(textOf(html)).not.toContain(CHUA_DANG_NHAP_XA.cau);
    expect(html).toContain(TIN_XA.tieu_de);
  });

  it("câu ấy nói việc làm tiếp, không mã lỗi, không tên dịch vụ", () => {
    for (const cau of Object.values(CHUA_DANG_NHAP_XA)) {
      expect(cau).not.toMatch(/\b[45]\d\d\b|error|identity|vigov|host|token|phiên/i);
    }
    expect(CHUA_DANG_NHAP_XA.cau).toMatch(/Bộ phận tiếp nhận|gọi điện thoại/);
  });
});

// ---------------------------------------------------------------------------------------------

const TIN_RA = {
  id: "tin-01",
  title: "Lịch tiêm chủng tháng mười",
  summary: "Trạm y tế thông báo lịch tiêm.",
  published_on: "2026-09-27",
  category_name: "Y tế",
};

describe("tin của xã — lời gọi và đọc trang", () => {
  it("trang đầu: GET đúng host của comms, chỉ `host`; trang sau: thêm `cursor` NGUYÊN VĂN", async () => {
    datFetch(traLoi(200, { items: [TIN_RA], next_cursor: "c+/=&1", has_more: true }));
    const kq = await tinCuaXa(TEN_MIEN, "");
    expect(kq.kieu).toBe("xong");
    expect(loi_goi[0]!.dia_chi).toBe(`https://comms.api.vigov.vn${DUONG_DAN_TIN_XA}?host=${TEN_MIEN}`);
    expect(Object.keys(loi_goi[0]!.tuy_chon.headers as Record<string, string>)).toEqual(["Accept"]);

    await tinCuaXa(TEN_MIEN, "c+/=&1");
    const url = new URL(loi_goi[1]!.dia_chi);
    expect(url.searchParams.get("host")).toBe(TEN_MIEN);
    expect(url.searchParams.get("cursor")).toBe("c+/=&1");
    expect([...url.searchParams.keys()].sort()).toEqual(["cursor", "host"]);
  });

  it("chi tiết: `id` mã hoá trong đường dẫn, 404 là MỘT nhánh", async () => {
    datFetch(traLoi(404, { error: "khong thay" }));
    expect(await baiTinCuaXa(TEN_MIEN, "a/b")).toEqual({ kieu: "khong-thay" });
    expect(loi_goi[0]!.dia_chi).toBe(`https://comms.api.vigov.vn${DUONG_DAN_TIN_XA}/a%2Fb?host=${TEN_MIEN}`);
  });

  it("parser: `has_more` mà không có con trỏ là sai khuôn; một dòng hỏng làm hỏng cả trang", () => {
    expect(docTrangTinXa({ items: [TIN_RA], next_cursor: "", has_more: false })).toEqual({
      // `type: null` — this fixture has no `type` (an older server): absent is not malformed.
      muc: [{ id: "tin-01", tieu_de: TIN_RA.title, tom_tat: TIN_RA.summary, ngay_dang: "2026-09-27", chuyen_muc: "Y tế", type: null }],
      con_tro: "",
      con_nua: false,
    });
    expect(docTrangTinXa({ items: [TIN_RA], next_cursor: "", has_more: true })).toBeNull();
    expect(docTrangTinXa({ items: [TIN_RA, { ...TIN_RA, id: 3 }], next_cursor: "", has_more: false })).toBeNull();
    expect(docBaiTin(TIN_RA)).toBeNull(); // chi tiết bắt buộc có `body`
    expect(docBaiTin({ ...TIN_RA, body: "Đoạn một." })?.noi_dung).toBe("Đoạn một.");
  });

  it("'Xem thêm' NỐI trang sau vào cuối và bỏ tin trùng", () => {
    const t1 = { id: "1", tieu_de: "a", tom_tat: "", ngay_dang: "2026-09-27", chuyen_muc: "", type: null };
    const t2 = { ...t1, id: "2" };
    const sau1 = sauKhiTaiTin(TIN_DAU, { kieu: "xong", gia_tri: { muc: [t1], con_tro: "c1", con_nua: true } });
    const sau2 = sauKhiTaiTin(batDauTaiTin(sau1), {
      kieu: "xong",
      gia_tri: { muc: [t1, t2], con_tro: "", con_nua: false },
    });
    expect(sau2.muc.map((t) => t.id)).toEqual(["1", "2"]);
    expect(sau2.con_nua).toBe(false);
    const html = renderToStaticMarkup(createElement(ThanTinXa, { ds: sau1, onMo: () => {}, onTai: () => {} }));
    expect(html).toContain(TIN_XA.nut_xem_them);
    expect(textOf(html)).toContain("Ngày đăng: 27/09/2026");
  });

  it("lỗi giữ danh sách đã có; tên miền bị từ chối thì không mời Thử lại", () => {
    const co = sauKhiTaiTin(TIN_DAU, {
      kieu: "xong",
      gia_tri: { muc: [{ id: "1", tieu_de: "a", tom_tat: "", ngay_dang: "", chuyen_muc: "", type: null }], con_tro: "c", con_nua: true },
    });
    const loi = sauKhiTaiTin(batDauTaiTin(co), { kieu: "loi-mang" });
    expect(loi.muc).toHaveLength(1);
    expect(renderToStaticMarkup(createElement(ThanTinXa, { ds: loi, onMo: () => {}, onTai: () => {} }))).toContain(
      TIN_XA.nut_thu_lai,
    );
    const tu_choi = sauKhiTaiTin(TIN_DAU, { kieu: "khong-hop-le" });
    const html = renderToStaticMarkup(createElement(ThanTinXa, { ds: tu_choi, onMo: () => {}, onTai: () => {} }));
    expect(html).toContain(TIN_XA.khong_hop_le);
    expect(html).not.toContain(TIN_XA.nut_thu_lai);
  });
});

describe("tin của xã — thân tin là VĂN BẢN, không bao giờ là HTML", () => {
  const BAI = {
    id: "tin-01",
    tieu_de: "<b>Tiêu đề</b>",
    tom_tat: "",
    ngay_dang: "2026-09-27",
    chuyen_muc: "Y tế",
    type: null,
    noi_dung: 'Đoạn một, dòng một.\nDòng hai.\n\n<script>alert("x")</script>\n\n\n\n<img src=x onerror=alert(1)>',
  };

  it("chia đoạn theo dòng trống, bỏ đoạn rỗng, giữ dòng đơn", () => {
    expect(chiaDoan(BAI.noi_dung)).toEqual([
      "Đoạn một, dòng một.\nDòng hai.",
      '<script>alert("x")</script>',
      "<img src=x onerror=alert(1)>",
    ]);
    expect(chiaDoan("a\r\n\r\nb")).toEqual(["a", "b"]);
    expect(chiaDoan("   \n\n  ")).toEqual([]);
  });

  it("`<script>` và `<img>` trong thân hiện thành CHỮ, không thành thẻ", () => {
    const html = renderToStaticMarkup(createElement(BaiTin, { bai: BAI }));
    expect(html).not.toMatch(/<script\b/i);
    expect(html).not.toMatch(/<img\b/i);
    expect(html).not.toMatch(/<b>/);
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).toContain("&lt;b&gt;Tiêu đề&lt;/b&gt;");
    // Ba đoạn, ba `<p>` thân tin.
    expect(html.match(/class="cd-tin__doan"/g) ?? []).toHaveLength(3);
  });

  it("không tệp nào của nửa nhà nước dùng `dangerouslySetInnerHTML` hay `innerHTML`", () => {
    const RAW = import.meta.glob("./**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    // Bỏ chú thích: tệp màn tin GIẢI THÍCH bằng lời vì sao nó không dùng API ấy.
    const boChuThich = (ma: string) => ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
    const san_xuat = Object.entries(RAW)
      .filter(([p]) => !p.includes(".test."))
      .map(([p, ma]) => [p, boChuThich(ma)] as const);
    expect(san_xuat.length).toBeGreaterThan(10);
    expect(san_xuat.filter(([, ma]) => /dangerouslySetInnerHTML|innerHTML/.test(ma)).map(([p]) => p)).toEqual([]);
  });

  it("chi tiết: 404 nói việc cần làm; lỗi mạng mời Thử lại", () => {
    expect(textOf(renderToStaticMarkup(createElement(ThanBaiTin, { trang: sauKhiTaiBai({ kieu: "khong-thay" }), onTai: () => {} })))).toContain(
      TIN_XA.khong_thay,
    );
    const loi = renderToStaticMarkup(createElement(ThanBaiTin, { trang: sauKhiTaiBai({ kieu: "loi-mang" }), onTai: () => {} }));
    expect(loi).toContain('role="alert"');
    expect(loi).toContain(TIN_XA.nut_thu_lai);
  });
});

// ---------------------------------------------------------------------------------------------

const CAN_BO_RA = {
  full_name: "Nguyễn Văn Thử",
  position: "Công chức Văn phòng",
  department_name: "Văn phòng Ủy ban",
  phone: "0900.000 000",
  mobile: "0900000000",
  has_zalo: true,
};

describe("danh bạ cán bộ xã", () => {
  it("GET đúng host của identity, chỉ `host`, KHÔNG bearer", async () => {
    datFetch(traLoi(200, { items: [CAN_BO_RA] }));
    const kq = await danhBaCanBoXa(TEN_MIEN);
    expect(kq.kieu).toBe("xong");
    expect(loi_goi[0]!.dia_chi).toBe(`https://identity.api.vigov.vn${DUONG_DAN_DANH_BA}?host=${TEN_MIEN}`);
    expect(Object.keys(loi_goi[0]!.tuy_chon.headers as Record<string, string>)).toEqual(["Accept"]);
  });

  it("hiện họ tên, chức vụ, bộ phận, và hai liên kết `tel:` đã làm sạch", () => {
    const cb = docDanhBa({ items: [CAN_BO_RA] })![0]!;
    const html = renderToStaticMarkup(createElement(TheCanBo, { cb }));
    const chu = textOf(html);
    expect(chu).toContain("Nguyễn Văn Thử");
    expect(chu).toContain(`${DANH_BA.chuc_vu}: Công chức Văn phòng`);
    expect(chu).toContain(`${DANH_BA.bo_phan}: Văn phòng Ủy ban`);
    expect(html.match(/href="tel:0900000000"/g) ?? []).toHaveLength(2);
    expect(chu).toContain(DANH_BA.goi("0900.000 000"));
    expect(chu).toContain(DANH_BA.co_zalo);
    expect(html).toContain('class="cd-goi"');
  });

  it("dấu Zalo CHỈ khi `has_zalo`; bộ phận rỗng không để một dòng trống; số rỗng không có liên kết", () => {
    const cb = docDanhBa({ items: [{ ...CAN_BO_RA, has_zalo: false, department_name: "", mobile: "" }] })![0]!;
    const html = renderToStaticMarkup(createElement(TheCanBo, { cb }));
    expect(textOf(html)).not.toContain(DANH_BA.co_zalo);
    expect(textOf(html)).not.toContain(DANH_BA.bo_phan);
    expect(textOf(html)).not.toContain(DANH_BA.di_dong);
    expect(html.match(/href="tel:/g) ?? []).toHaveLength(1);
  });

  it("`dichGoi`: giữ chữ số và một `+` đầu, không bao giờ ra một `tel:` mang ký tự lạ", () => {
    expect(dichGoi("0900.000 000")).toBe("tel:0900000000");
    expect(dichGoi("+84 900 000 000")).toBe("tel:+84900000000");
    expect(dichGoi("javascript:alert(1)")).toBeNull();
    expect(dichGoi("  ")).toBeNull();
    expect(dichGoi("09")).toBeNull();
  });

  it("parser: sai kiểu một trường là `null`", () => {
    expect(docDanhBa({ items: [{ ...CAN_BO_RA, has_zalo: "co" }] })).toBeNull();
    expect(docDanhBa({ items: [] })).toEqual([]);
  });

  it("rỗng: một câu nói việc làm được; lỗi: câu + Thử lại; tên miền bị từ chối: không Thử lại", () => {
    const ve = (trang: Parameters<typeof ThanDanhBa>[0]["trang"]) =>
      renderToStaticMarkup(createElement(ThanDanhBa, { trang, onTai: () => {} }));
    expect(ve(sauKhiTaiDanhBa({ kieu: "xong", gia_tri: [] }))).toContain(DANH_BA.trong);
    const mang = ve(sauKhiTaiDanhBa({ kieu: "loi-mang" }));
    expect(mang).toContain(DANH_BA.loi_mang);
    expect(mang).toContain(DANH_BA.nut_thu_lai);
    expect(ve(sauKhiTaiDanhBa({ kieu: "tam-ngung" }))).toContain(DANH_BA.loi_may_chu);
    const tu_choi = ve(sauKhiTaiDanhBa({ kieu: "khong-hop-le" }));
    expect(tu_choi).toContain(DANH_BA.khong_hop_le);
    expect(tu_choi).not.toContain(DANH_BA.nut_thu_lai);
  });
});
