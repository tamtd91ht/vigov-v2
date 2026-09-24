import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import {
  TRANG_DAU,
  sangTrangSau,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { duongDanSoVanBanDen, duongDanSoVanBanDi } from "@/lib/api/van-ban";

import {
  ChonThuTu,
  THU_TU_SO,
  doiLocVeTrangDau,
  maThuTu,
  sapXepTheoThuTu,
} from "./loc-so-van-ban";

/**
 * Ô tìm và ô thứ tự của hai quyển sổ. Hai điều chịu lực:
 *
 *   1. Đổi chữ tìm hay thứ tự là VỀ TRANG ĐẦU — con trỏ của trang 3 thuộc về truy vấn cũ.
 *   2. Lựa chọn mặc định KHÔNG gửi `sort`/`order`; lựa chọn còn lại gửi đúng enum hợp đồng.
 */

const TRANG_BA: NganXepConTro = sangTrangSau(sangTrangSau(TRANG_DAU, "moc-2"), "moc-3");

describe("đổi bộ lọc là về trang đầu", () => {
  it("đổi chữ tìm khi đang ở trang 3: đặt giá trị TRƯỚC, rồi ngăn xếp về TRANG_DAU", () => {
    const thuTuGoi: string[] = [];
    const datTim = vi.fn(() => thuTuGoi.push("dat"));
    const datNganXep = vi.fn((n: NganXepConTro) => {
      thuTuGoi.push("trang");
      expect(n).toBe(TRANG_DAU);
    });

    doiLocVeTrangDau(datTim, datNganXep);

    expect(datTim).toHaveBeenCalledOnce();
    expect(datNganXep).toHaveBeenCalledWith(TRANG_DAU);
    expect(thuTuGoi).toEqual(["dat", "trang"]);
    // Và trang đầu là một URL KHÔNG mang con trỏ nào — con trỏ của trang 3 không đi theo.
    expect(TRANG_BA.hienTai).toBe("moc-3");
    const sau = duongDanSoVanBanDen({ tim: "rà soát", cursor: TRANG_DAU.hienTai });
    expect(sau).not.toContain("cursor");
    expect(new URLSearchParams(sau.split("?")[1]).get("q")).toBe("rà soát");
  });
});

describe("thứ tự — chỉ giá trị của enum hợp đồng", () => {
  it("mặc định KHÔNG gửi `sort` hay `order`", () => {
    expect(sapXepTheoThuTu("")).toEqual({});
    expect(duongDanSoVanBanDi({ ...sapXepTheoThuTu("") })).toBe("/api/v1/outgoing-documents");
  });

  it("'Số cũ nhất trước' gửi đúng `sort=number&order=asc`", () => {
    const d = duongDanSoVanBanDen({ ...sapXepTheoThuTu("so-tang") });
    expect(d).toBe("/api/v1/incoming-documents?sort=number&order=asc");
  });

  it("một mã lạ từ ô chọn KHÔNG thành một thứ tự — về mặc định", () => {
    // Ô chọn phát ra chuỗi; một giá trị bị sửa trong DevTools không được đi lên thành `sort=...`.
    expect(maThuTu("received_date")).toBe("");
    expect(maThuTu("toString")).toBe("");
    expect(maThuTu("so-tang")).toBe("so-tang");
  });

  it("ô chọn vẽ đủ các lựa chọn trong bảng, và đánh dấu đúng lựa chọn đang dùng", () => {
    const html = renderToStaticMarkup(
      <ChonThuTu id="thu-tu" thuTu="so-tang" datThuTu={() => {}} />,
    );
    for (const { nhan } of Object.values(THU_TU_SO)) expect(html).toContain(nhan);
    expect(html).toMatch(/<option value="so-tang" selected="">Số cũ nhất trước/);
  });
});
