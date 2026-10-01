/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { guiPhanAnh, phanAnhCuaToi, traCuuPhieu } from "./api/goi-vigov";
import { thanGuiPhanAnh } from "./api/hop-dong-phan-anh";
import { taoLanGui } from "./api/lan-gui";
import { layPhienViGov } from "./api/phien-vigov";
import { diaChiViGov } from "./api/dia-chi-vigov";
import type { PhieuCuaToi } from "./api/hop-dong-phan-anh";
import {
  BuocXacNhan,
  DangGui,
  GuiPhanAnhScreen,
  ID_DAU_BUOC,
  KetQuaGui,
  kiemPhanAnh,
  LoiGui,
  PHAN_ANH_TRONG,
} from "./man/GuiPhanAnhScreen";
import { KenhCongDan } from "./man/KenhCongDan";
import {
  CUA_TOI,
  KENH_CHUA_MO,
  GUI,
  LOI_GUI,
  nhanTrangThai,
  giaiThichTrangThai,
  TRA_CUU,
  TRANG_THAI,
  TRANG_THAI_CHUA_CO_NHAN,
} from "./man/noi-dung";
import * as NOI_DUNG from "./man/noi-dung";
import { PhanAnhCuaToiScreen } from "./man/PhanAnhCuaToiScreen";
import { thoiDiemVN } from "../lib/thoi-diem";
import { KetQuaTraCuu, TraCuuPhieuScreen } from "./man/TraCuuPhieuScreen";

/**
 * KÊNH CÔNG DÂN, VỚI NGUỒN PHIÊN THẬT — tức là ĐÓNG. Tệp này KHÔNG giả lập phiên: mọi ca ở đây chạy
 * đúng mã nằm trong bundle hôm nay. Các ca cần một phiên giả (khoá chống trùng, thân gửi đi,
 * 201/404) ở `api/goi-vigov.test.ts`, nơi `vi.mock` thay nguồn phiên cho RIÊNG tệp ấy.
 */

const RAW = import.meta.glob("./**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const boChuThich = (ma: string) =>
  ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

const SAN_XUAT = Object.entries(RAW)
  .filter(([p]) => !p.includes(".test."))
  .map(([path, code]) => ({ path, code: boChuThich(code) }));

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("nguồn phiên ViGov đóng — không một lời gọi mạng nào đi ra", () => {
  it("nguồn phiên trả `null` — dù host của `petitions` đã có (ADR 0046)", () => {
    expect(layPhienViGov()).toBeNull();
    // Host đã chốt; cổng DUY NHẤT còn giữ yêu cầu lại là phiên. Ca ngay dưới chứng minh điều đó.
    expect(diaChiViGov("petitions", "/api/v1/my-citizen-reports")).toBe(
      "https://petitions.api.vigov.vn/api/v1/my-citizen-reports",
    );
  });

  it("một tên dịch vụ ngoài bảng ra RỖNG, không ra `undefined/…` hay khoá của `Object.prototype`", () => {
    // `identity` từng đứng đầu danh sách này; nó vào bảng host ngày 27/09/2026 (màn xác nhận xã,
    // danh bạ). `platform` giữ chỗ ấy: một dịch vụ CÓ THẬT mà Mini App không bao giờ được gọi thẳng.
    for (const ten of ["platform", "toString", "__proto__", ""]) {
      expect(diaChiViGov(ten as "petitions", "/api/v1/x"), ten).toBe("");
    }
  });

  it("gửi và tra cứu dừng ở `chua-co-phien`, fetch KHÔNG được gọi", async () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);

    const lan = taoLanGui(thanGuiPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "Ổ gà trước cổng chợ" }));
    expect(await guiPhanAnh(lan)).toEqual({ kieu: "chua-co-phien" });
    expect(await traCuuPhieu("MA-THU-01")).toEqual({ kieu: "chua-co-phien" });
    expect(await traCuuPhieu("")).toEqual({ kieu: "chua-co-phien" });
    expect(await phanAnhCuaToi("")).toEqual({ kieu: "chua-co-phien" });
    expect(await phanAnhCuaToi("c1")).toEqual({ kieu: "chua-co-phien" });
    expect(fetch_gia).not.toHaveBeenCalled();
  });

  it("ba màn nói 'kênh chưa mở', không vẽ ô nhập nào, không gọi mạng", () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);

    for (const man of [
      createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }),
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {} }),
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {}, ma_ban_dau: "PA7K2QX9M4TD" }),
      createElement(PhanAnhCuaToiScreen, { onQuayLai: () => {}, onMoPhieu: () => {}, onGuiPhanAnh: () => {} }),
    ]) {
      const html = renderToStaticMarkup(man);
      expect(html).toContain(KENH_CHUA_MO.tieu_de);
      expect(html).not.toMatch(/<(input|textarea|form)[\s/>]/);
      // Không nút gửi nào khi kênh đóng.
      expect(html).not.toContain(GUI.nut_tiep);
      // Không đang tải, không "chưa gửi phản ánh nào" — chưa có phiên thì chưa biết gì cả.
      expect(html).not.toContain(CUA_TOI.dang_tai);
      expect(html).not.toContain(CUA_TOI.trong);
    }
    expect(fetch_gia).not.toHaveBeenCalled();
  });

  it("màn chọn việc có lối vào 'Phản ánh của tôi'", () => {
    const html = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {} }));
    expect(html).toContain(`<button type="button" class="cd-nut">${CUA_TOI.tieu_de}</button>`);
  });
});

// (Ca "`index.rong.ts` chỉ nhập KIỂU" đã bị xoá 27/09/2026 cùng bản rỗng ấy: bản dựng chỉ còn một
// và nó MANG kênh công dân — `bundle-for-zalo.test.ts` đo sự có mặt ấy trên bundle thật.)

describe("bearer chỉ đến từ `api/phien-vigov.ts` — kiểm tĩnh trên mã nguồn", () => {
  it("quét đúng cây mã thật của nửa nhà nước", () => {
    const duong = SAN_XUAT.map((f) => f.path);
    for (const tep of ["./api/phien-vigov.ts", "./api/goi-vigov.ts", "./api/hop-dong-phan-anh.ts"]) {
      expect(duong).toContain(tep);
    }
  });

  it("nguồn phiên KHÔNG nhập gì cả — không có đường nào để nó đọc một phiếu phiên khác", () => {
    const ma = SAN_XUAT.find((f) => f.path === "./api/phien-vigov.ts")!.code;
    expect(ma).not.toMatch(/\bimport\b|\brequire\s*\(/);
  });

  it("không tệp nào của nửa nhà nước chạm phiếu phiên hay client của `vihat-miniapp`", () => {
    // Khối đăng nhập (`features/dang-nhap/`, `kho-phien`) và `src/api/` là của backend THƯƠNG MẠI.
    // Phiếu phiên của nó KHÔNG phải phiên công dân ViGov (ADR 0032).
    const CAM = /["'`][^"'`]*(dang-nhap|kho-phien|goi-may-chu|hop-dong-yeu-cau|\.\.\/\.\.\/api\/|\.\.\/api\/dia-chi|tinh-nang|zmp-sdk)[^"'`]*["'`]/;
    const vi_pham = SAN_XUAT.filter((f) => CAM.test(f.code)).map((f) => f.path);
    expect(vi_pham).toEqual([]);
  });

  it("chữ `Bearer` và `Authorization` chỉ có trong tệp gọi mạng, và tệp ấy lấy token từ `layPhienViGov`", () => {
    const co_bearer = SAN_XUAT.filter((f) => /Bearer|Authorization/.test(f.code)).map((f) => f.path);
    expect(co_bearer).toEqual(["./api/goi-vigov.ts"]);

    const goi = SAN_XUAT.find((f) => f.path === "./api/goi-vigov.ts")!.code;
    expect(goi).toMatch(/from\s+"\.\/phien-vigov"/);
    expect(goi).toMatch(/layPhienViGov\(\)/);
    // Không hàm xuất ra nào nhận token qua tham số — một tham số là một khe nhét phiên khác vào.
    for (const khop of goi.matchAll(/export\s+async\s+function\s+(\w+)\s*\(([^)]*)\)/g)) {
      expect(khop[2], `${khop[1]} nhận một tham số giống phiên`).not.toMatch(/token|phien|bearer/i);
    }
  });

  it("phép kiểm tĩnh còn sống: nó bắt được một đường nhập phiếu phiên thương mại", () => {
    const CAM = /["'`][^"'`]*(dang-nhap|kho-phien)[^"'`]*["'`]/;
    expect(CAM.test('import { useKhoPhien } from "../../features/dang-nhap/kho-phien";')).toBe(true);
    expect(CAM.test('import { layPhienViGov } from "./phien-vigov";')).toBe(false);
  });
});

describe("chữ của màn hình", () => {
  it("giờ hiện theo +07, KHÔNG theo múi giờ của máy", () => {
    expect(thoiDiemVN("2026-09-24T01:30:00Z")).toBe("24/09/2026 08:30");
    // Qua nửa đêm giờ Việt Nam: ngày cũng phải sang.
    expect(thoiDiemVN("2026-09-24T20:15:00Z")).toBe("25/09/2026 03:15");
    expect(thoiDiemVN("2026-09-24T10:00:00+07:00")).toBe("24/09/2026 10:00");
    expect(thoiDiemVN("khong-phai-thoi-diem")).toBeNull();
  });

  it("đủ chín trạng thái của ADR 0027, người dân thấy bốn nhóm, và `da-tiep-nhan` có câu cho người dân", () => {
    expect(Object.keys(TRANG_THAI).sort()).toEqual(
      [
        "da-tiep-nhan",
        "dang-phan-loai",
        "da-chuyen-xu-ly",
        "dang-xu-ly",
        "da-xu-ly",
        "cho-dan-xac-nhan",
        "da-dong",
        "khong-tiep-nhan",
        "chuyen-cap-tren",
      ].sort(),
    );
    expect(nhanTrangThai("da-tiep-nhan")).toBe("Đã tiếp nhận");
    expect(giaiThichTrangThai("da-tiep-nhan")).toContain("chờ cán bộ");
    // Mã lạ: không hiện mã thô cho người dân.
    expect(nhanTrangThai("ma-la")).not.toContain("ma-la");
    // PINNED (ADR 0050 #5 in the shared app): an unknown code is the neutral sentence, NOT a guessed group.
    // "Đã đóng" on a ticket that may still be open tells the citizen the commune has stopped working on it.
    expect(nhanTrangThai("ma-la")).toBe(TRANG_THAI_CHUA_CO_NHAN);
    for (const label of ["Đã đóng", "Đã tiếp nhận", "Đang xử lý", "Đã xử lý xong"]) {
      expect(nhanTrangThai("ma-la")).not.toBe(label);
    }
    // Inherited keys are not codes: `groupOf` reads own keys only.
    expect(nhanTrangThai("toString")).toBe(TRANG_THAI_CHUA_CO_NHAN);
    // The nine staff labels that were merged away no longer reach the citizen.
    expect(nhanTrangThai("cho-dan-xac-nhan")).toBe("Đã xử lý xong");
    expect(nhanTrangThai("khong-tiep-nhan")).toBe("Đã đóng");
  });

  it("nội dung bắt buộc, độ dài theo máy chủ, và ẩn danh không tính họ tên", () => {
    expect(kiemPhanAnh(PHAN_ANH_TRONG)).toBe(GUI.thieu_noi_dung);
    expect(kiemPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "   \n " })).toBe(GUI.thieu_noi_dung);
    expect(kiemPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "Đèn đường hỏng" })).toBeNull();
    expect(kiemPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "ă".repeat(4001) })).not.toBeNull();
    expect(kiemPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "ă".repeat(4000) })).toBeNull();
    expect(
      kiemPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "x", ho_ten: "a".repeat(201), an_danh: true }),
    ).toBeNull();
  });

  it("không câu chữ nào của nửa nhà nước là một mã lỗi hay tên trường kỹ thuật", () => {
    const noi_dung = SAN_XUAT.find((f) => f.path === "./man/noi-dung.ts")!.code;
    const chuoi = [...noi_dung.matchAll(/"([^"\n]{12,})"/g)].map((m) => m[1]!);
    expect(chuoi.length).toBeGreaterThan(20);
    for (const c of chuoi) {
      expect(c, `câu có mã lỗi: ${c}`).not.toMatch(/\b(4\d\d|5\d\d)\b|error|content|reporter_/i);
    }
  });

  it("không câu nào dạy việc 'đổi xã' — một phiên, một xã, không có lối đổi (ADR 0044)", () => {
    // Đọc GIÁ TRỊ lúc chạy, không đọc mã nguồn: một câu dựng từ hàm vẫn bị quét.
    const gom = (v: unknown): string[] =>
      typeof v === "string"
        ? [v]
        : typeof v === "function"
          ? gom(v(...Array<string>(v.length).fill("mẫu")))
          : v && typeof v === "object"
            ? Object.values(v).flatMap(gom)
            : [];
    const DOI_XA = /đổi xã/i;
    const cau = gom(NOI_DUNG);
    expect(cau.length).toBeGreaterThan(40);
    expect(cau.filter((c) => DOI_XA.test(c))).toEqual([]);
    // Phép kiểm còn sống: câu cũ bị bắt.
    expect(DOI_XA.test("hãy quay lại và Đổi xã trước khi gửi")).toBe(true);
  });

  it("không `console.*` ở bất kỳ tệp nào của nửa nhà nước (luật 3) — trừ đúng một tệp chẩn đoán", () => {
    // The one exemption (owner, 01/10/2026): `api/connection-log.ts`, fixed-shape record, `--demo` only.
    // Its shape is pinned in `api/connection-log.test.ts`; widening this list is widening rule 3.
    const allowed = ["./api/connection-log.ts"];
    expect(
      SAN_XUAT.filter((f) => /\bconsole\s*\./.test(f.code) && !allowed.includes(f.path)).map((f) => f.path),
    ).toEqual([]);
    const log = SAN_XUAT.find((f) => f.path === allowed[0]);
    expect(log, "the exempted file moved — the exemption now guards nothing").toBeDefined();
    expect(log!.code.match(/\bconsole\s*\.\w+/g)).toEqual(["console.warn"]);
    expect(log!.code).toMatch(/if \(!DEMO_BUILD\) return;/);
  });
});

/**
 * TRÌNH ĐỌC MÀN HÌNH — kiểm trên markup từng bước. `renderToStaticMarkup` không chạy hiệu ứng nên
 * việc TIÊU ĐIỂM THẬT SỰ DỜI tới đầu bước KHÔNG được kiểm ở đây (cần bấm, mà khung gắn tự dựng của
 * `man/hieu-ung-khong-phien.test.tsx` không mô phỏng sự kiện). Ở đây chỉ chứng minh: mỗi bước CÓ chỗ
 * nhận tiêu điểm đúng `id` hiệu ứng tìm, và thông báo nằm đúng vùng.
 */
describe("trình đọc màn hình: đổi bước và kết quả được báo ra", () => {
  const PHIEU: PhieuCuaToi = {
    ma_tra_cuu: "PA7K2QX9M4TD",
    trang_thai: "da-tiep-nhan",
    linh_vuc: "",
    nhan_linh_vuc: "",
    noi_dung: "Đèn đường hỏng",
    dia_chi: "",
    ho_ten_da_che: "",
    dien_thoai_da_che: "",
    an_danh: true,
    goc_dem_han: "2026-09-24T01:30:00Z",
    han_tiep_nhan: null,
    han_xu_ly_xong: null,
    ket_qua: "",
    ly_do: "",
    co_quan_nhan: "",
    rating: null,
    rated_at: null,
  };

  /** Thẻ mở của phần tử mang `id` — rỗng khi không có. */
  const theMo = (html: string, id: string) => html.match(new RegExp(`<[a-z0-9]+[^>]* id="${id}"[^>]*>`))?.[0] ?? "";

  it("mỗi bước có đúng một chỗ nhận tiêu điểm, `tabindex=\"-1\"`, mang `id` hiệu ứng tìm", () => {
    const buoc: Array<[keyof typeof ID_DAU_BUOC, string]> = [
      ["xac-nhan", renderToStaticMarkup(createElement(BuocXacNhan, { ten_xa: "Xã Thử Nghiệm", onGui: () => {}, onSua: () => {} }))],
      ["dang-gui", renderToStaticMarkup(createElement(DangGui))],
      ["xong", renderToStaticMarkup(createElement(KetQuaGui, { phieu: PHIEU, onGuiKhac: () => {} }))],
      ["loi", renderToStaticMarkup(createElement(LoiGui, { nhanh: "loi-mang", onGuiLai: () => {}, onSua: () => {} }))],
    ];
    for (const [kieu, html] of buoc) {
      const the = theMo(html, ID_DAU_BUOC[kieu]);
      expect(the, kieu).not.toBe("");
      expect(the, kieu).toContain('tabindex="-1"');
      expect(html.split(`id="${ID_DAU_BUOC[kieu]}"`).length - 1, kieu).toBe(1);
    }
    // Bước "nhập" trỏ vào tiêu đề chung của màn — chỉ vẽ được khi có phiên, nên kiểm trên mã nguồn.
    const man = SAN_XUAT.find((f) => f.path === "./man/GuiPhanAnhScreen.tsx")!.code;
    expect(man).toMatch(/<h1 className="cd-tieu-de" id=\{ID_DAU_BUOC\.nhap\} tabIndex=\{-1\}>/);
    // Hiệu ứng tìm đúng bảng ấy, theo `kieu` của bước.
    expect(man).toMatch(/getElementById\(ID_DAU_BUOC\[buoc\.kieu\]\)\?\.focus\(\)/);
    expect(new Set(Object.values(ID_DAU_BUOC)).size).toBe(Object.keys(ID_DAU_BUOC).length);
  });

  it("'đang gửi' là một vùng `role=\"status\"`", () => {
    expect(theMo(renderToStaticMarkup(createElement(DangGui)), ID_DAU_BUOC["dang-gui"])).toContain('role="status"');
  });

  it("gửi xong: `role=\"status\"` bọc câu và mã, KHÔNG bọc tiêu đề, thẻ phiếu hay nút", () => {
    const html = renderToStaticMarkup(createElement(KetQuaGui, { phieu: PHIEU, onGuiKhac: () => {} }));
    expect(html.split('role="status"').length - 1).toBe(1);
    const vung = html.match(/<div role="status">([\s\S]*?)<\/div>/)?.[1] ?? "";
    expect(vung).toContain(GUI.xong_ma);
    expect(vung).toContain(PHIEU.ma_tra_cuu);
    expect(vung).not.toContain(GUI.xong_tieu_de);
    expect(vung).not.toContain("<button");
  });

  it("câu lỗi gửi vẫn là `role=\"alert\"`", () => {
    const the = theMo(
      renderToStaticMarkup(createElement(LoiGui, { nhanh: "loi-mang", onGuiLai: () => {}, onSua: () => {} })),
      ID_DAU_BUOC.loi,
    );
    expect(the).toContain('role="alert"');
    expect(LOI_GUI["loi-mang"].cau.length).toBeGreaterThan(0);
  });

  it("tra cứu thấy phiếu: một câu ngắn `role=\"status\"`, thẻ phiếu nằm NGOÀI vùng ấy", () => {
    const html = renderToStaticMarkup(createElement(KetQuaTraCuu, { kq: { kieu: "xong", phieu: PHIEU } }));
    expect(html).toContain(`<p class="cd-cau" role="status">${TRA_CUU.tim_thay}</p>`);
    expect(html.split('role="status"').length - 1).toBe(1);
    // Thẻ phiếu vẫn vẽ, và nằm SAU câu thông báo đã đóng — không lọt vào vùng `status`.
    expect(html).toContain(PHIEU.noi_dung);
    expect(html.indexOf(`${TRA_CUU.tim_thay}</p>`)).toBeLessThan(html.indexOf('class="cd-phieu"'));
    expect(html).not.toContain('role="alert"');
  });

  it("tra cứu không thấy: vẫn `role=\"alert\"`, không có câu 'đã tìm thấy'", () => {
    const html = renderToStaticMarkup(createElement(KetQuaTraCuu, { kq: { kieu: "khong-thay" } }));
    expect(html).toContain('role="alert"');
    expect(html).not.toContain(TRA_CUU.tim_thay);
  });
});
