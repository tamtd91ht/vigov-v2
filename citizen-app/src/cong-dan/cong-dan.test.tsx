/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { guiPhanAnh, traCuuPhieu } from "./api/goi-vigov";
import { thanGuiPhanAnh } from "./api/hop-dong-phan-anh";
import { taoLanGui } from "./api/lan-gui";
import { layPhienViGov } from "./api/phien-vigov";
import { diaChiViGov } from "./api/dia-chi-vigov";
import { GuiPhanAnhScreen, kiemPhanAnh, PHAN_ANH_TRONG } from "./man/GuiPhanAnhScreen";
import { KENH_CHUA_MO, GUI, nhanTrangThai, giaiThichTrangThai, TRANG_THAI } from "./man/noi-dung";
import { thoiDiemVN } from "./man/thoi-diem";
import { TraCuuPhieuScreen } from "./man/TraCuuPhieuScreen";

/**
 * KÊNH CÔNG DÂN, VỚI NGUỒN PHIÊN THẬT — tức là ĐÓNG. Tệp này KHÔNG giả lập phiên: mọi ca ở đây chạy
 * đúng mã sẽ nằm trong bản thử hôm nay. Các ca cần một phiên giả (khoá chống trùng, thân gửi đi,
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
  it("nguồn phiên trả `null`, và địa chỉ ViGov là chuỗi rỗng", () => {
    expect(layPhienViGov()).toBeNull();
    expect(diaChiViGov("/api/v1/my-citizen-reports")).toBe("");
  });

  it("gửi và tra cứu dừng ở `chua-co-phien`, fetch KHÔNG được gọi", async () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);

    const lan = taoLanGui(thanGuiPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "Ổ gà trước cổng chợ" }));
    expect(await guiPhanAnh(lan)).toEqual({ kieu: "chua-co-phien" });
    expect(await traCuuPhieu("MA-THU-01")).toEqual({ kieu: "chua-co-phien" });
    expect(await traCuuPhieu("")).toEqual({ kieu: "chua-co-phien" });
    expect(fetch_gia).not.toHaveBeenCalled();
  });

  it("hai màn nói 'kênh chưa mở', không vẽ ô nhập nào, không gọi mạng", () => {
    const fetch_gia = vi.fn();
    vi.stubGlobal("fetch", fetch_gia);

    for (const man of [
      createElement(GuiPhanAnhScreen, { onQuayLai: () => {} }),
      createElement(TraCuuPhieuScreen, { onQuayLai: () => {} }),
    ]) {
      const html = renderToStaticMarkup(man);
      expect(html).toContain(KENH_CHUA_MO.tieu_de);
      expect(html).not.toMatch(/<(input|textarea|form)[\s/>]/);
      // Không nút gửi nào khi kênh đóng.
      expect(html).not.toContain(GUI.nut_tiep);
    }
    expect(fetch_gia).not.toHaveBeenCalled();
  });
});

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

  it("đủ chín trạng thái của ADR 0027, nhãn nguyên văn, và `da-tiep-nhan` có câu cho người dân", () => {
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

  it("không `console.*` ở bất kỳ tệp nào của nửa nhà nước (luật 3)", () => {
    expect(SAN_XUAT.filter((f) => /\bconsole\s*\./.test(f.code)).map((f) => f.path)).toEqual([]);
  });
});
