import { describe, expect, it } from "vitest";

import { TRANG_DAU, sangTrangSau } from "@/features/cau-hinh/ngan-xep-con-tro";
import { TU_KHOA_TIM_TOI_DA, chuanHoaTuKhoaTim, duongDanDanhSachCanBo, thanTimCanBo } from "@/lib/api/can-bo";

import {
  CAU_TU_KHOA_QUA_DAI,
  LUA_CHON_HIEN_THI,
  THU_TU_HIEN_THI,
  TRUY_VAN_DAU,
  apLoc,
  dangLoc,
  ketQuaGuiTim,
  maBoPhanLoc,
  maHienThi,
  thamSoDoc,
  type TruyVanDanhBa,
} from "./loc-danh-ba";

/**
 * Hai điều chịu lực của hàng lọc:
 *
 *   1. Đổi BẤT KỲ bộ lọc nào là về TRANG ĐẦU — con trỏ của trang 3 thuộc về truy vấn cũ.
 *   2. Bấm Tìm với chữ quá dài bị từ chối TẠI CHỖ, với một câu, và bộ lọc đang áp không đổi.
 */

function tuKhoa(tho: string) {
  const tu = chuanHoaTuKhoaTim(tho);
  if (tu.loai !== "hopLe") throw new Error("chữ mẫu phải hợp lệ");
  return tu;
}

/** Đang ở trang 3 của danh sách, đã lọc khối BP-LE. */
const TRANG_BA: TruyVanDanhBa = {
  loc: { tuKhoa: null, boPhan: "BP-LE", hienThi: "" },
  nganXep: sangTrangSau(sangTrangSau(TRANG_DAU, "moc-2"), "moc-3"),
};

describe("đổi bộ lọc là về trang đầu", () => {
  it("mỗi loại bộ lọc — chữ tìm, khối, trạng thái hiển thị — đều bỏ con trỏ cũ", () => {
    expect(thamSoDoc(TRANG_BA).cursor).toBe("moc-3");

    for (const doi of [
      { tuKhoa: tuKhoa("Huỳnh") },
      { tuKhoa: null },
      { boPhan: "BP-CHAN" },
      { boPhan: "" },
      { hienThi: "0" as const },
      { hienThi: "1" as const },
    ]) {
      const sau = apLoc(TRANG_BA, doi);
      expect(sau.nganXep).toBe(TRANG_DAU);
      expect(thamSoDoc(sau).cursor).toBeNull();
    }
  });

  it("bộ lọc kết hợp AND: đổi một bộ lọc giữ nguyên các bộ lọc kia", () => {
    const sau = apLoc(TRANG_BA, { hienThi: "0" });
    expect(sau.loc).toEqual({ tuKhoa: null, boPhan: "BP-LE", hienThi: "0" });
    expect(thamSoDoc(sau)).toEqual({ boPhan: "BP-LE", congKhai: false, cursor: null });
  });

  it("trang đầu sau khi đổi lọc là một URL KHÔNG mang con trỏ nào", () => {
    const sau = apLoc(TRANG_BA, { boPhan: "BP-CHAN" });
    expect(duongDanDanhSachCanBo(thamSoDoc(sau))).toBe("/api/v1/staff?unit=BP-CHAN");
  });

  it("…và một thân tìm KHÔNG mang con trỏ nào", () => {
    const sau = apLoc(TRANG_BA, { tuKhoa: tuKhoa("Huỳnh") });
    const tk = sau.loc.tuKhoa;
    if (tk === null) throw new Error("phải có chữ tìm");
    expect(thanTimCanBo(tk, thamSoDoc(sau)).cursor).toBe("");
  });

  it("sang trang GIỮ bộ lọc — chỉ ngăn xếp đổi", () => {
    const loc = apLoc(TRUY_VAN_DAU, { tuKhoa: tuKhoa("Huỳnh"), hienThi: "1" });
    const trang2 = { ...loc, nganXep: sangTrangSau(loc.nganXep, "moc-2") };
    expect(thamSoDoc(trang2)).toEqual({ boPhan: "", congKhai: true, cursor: "moc-2" });
    expect(trang2.loc.tuKhoa).toEqual({ loai: "hopLe", tu: "Huỳnh" });
  });
});

describe("bấm Tìm", () => {
  it("chữ rỗng → bỏ tìm (`tuKhoa: null`), tức quay về GET", () => {
    expect(ketQuaGuiTim("")).toEqual({ doi: { tuKhoa: null } });
    expect(ketQuaGuiTim("   ")).toEqual({ doi: { tuKhoa: null } });
  });

  it("chữ hợp lệ → chữ đã chuẩn hoá", () => {
    expect(ketQuaGuiTim("  Huỳnh   Văn ")).toEqual({
      doi: { tuKhoa: { loai: "hopLe", tu: "Huỳnh Văn" } },
    });
  });

  it("200 ký tự được nhận, 201 bị từ chối kèm câu — đếm ký tự, không đếm byte", () => {
    expect("doi" in ketQuaGuiTim("ễ".repeat(TU_KHOA_TIM_TOI_DA))).toBe(true);
    expect(ketQuaGuiTim("ễ".repeat(TU_KHOA_TIM_TOI_DA + 1))).toEqual({ loi: CAU_TU_KHOA_QUA_DAI });
  });

  it("câu từ chối nói giới hạn, và KHÔNG nhắc lại chữ đã gõ", () => {
    const go = `0900000000 ${"a".repeat(TU_KHOA_TIM_TOI_DA)}`;
    const kq = ketQuaGuiTim(go);
    expect(kq).toEqual({ loi: CAU_TU_KHOA_QUA_DAI });
    expect(CAU_TU_KHOA_QUA_DAI).toContain(String(TU_KHOA_TIM_TOI_DA));
    expect(CAU_TU_KHOA_QUA_DAI).not.toContain("0900000000");
  });
});

describe("ô trạng thái hiển thị", () => {
  it("ba lựa chọn, nguyên văn đặc tả §3, mặc định đứng đầu", () => {
    expect(THU_TU_HIEN_THI.map((m) => LUA_CHON_HIEN_THI[m].nhan)).toEqual([
      "Hiện và chưa hiện",
      "Đang hiện trên Mini App",
      "Chưa hiện",
    ]);
    // Thứ tự vẽ phủ ĐỦ bảng — thêm một lựa chọn mà quên mảng thứ tự là một lựa chọn không ai thấy.
    expect([...THU_TU_HIEN_THI].sort()).toEqual(Object.keys(LUA_CHON_HIEN_THI).sort());
  });

  it("`0` là `false`, không phải 'cả hai'", () => {
    expect(LUA_CHON_HIEN_THI["0"].congKhai).toBe(false);
    expect(LUA_CHON_HIEN_THI["1"].congKhai).toBe(true);
    expect(LUA_CHON_HIEN_THI[""].congKhai).toBeNull();
  });

  it("giá trị lạ từ ô chọn → cả hai", () => {
    expect(maHienThi("true")).toBe("");
    expect(maHienThi("toString")).toBe("");
    expect(maHienThi("0")).toBe("0");
  });
});

describe("ô khối / đơn vị", () => {
  const DANH_MUC = [{ id: "BP-LE" }, { id: "BP-CHAN" }];

  it("chỉ nhận id có trong danh mục đã đọc", () => {
    expect(maBoPhanLoc("BP-CHAN", DANH_MUC)).toBe("BP-CHAN");
    expect(maBoPhanLoc("BP-XA-KHAC", DANH_MUC)).toBe("");
    expect(maBoPhanLoc("", DANH_MUC)).toBe("");
  });
});

describe("danh sách rỗng khi đang lọc", () => {
  it("không lọc gì thì `dangLoc` là false; mỗi bộ lọc bật nó lên", () => {
    expect(dangLoc(TRUY_VAN_DAU.loc)).toBe(false);
    expect(dangLoc(apLoc(TRUY_VAN_DAU, { tuKhoa: tuKhoa("x") }).loc)).toBe(true);
    expect(dangLoc(apLoc(TRUY_VAN_DAU, { boPhan: "BP" }).loc)).toBe(true);
    expect(dangLoc(apLoc(TRUY_VAN_DAU, { hienThi: "0" }).loc)).toBe(true);
  });
});
