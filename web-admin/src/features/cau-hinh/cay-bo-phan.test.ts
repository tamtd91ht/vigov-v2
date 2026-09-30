import { afterEach, describe, expect, it, vi } from "vitest";

import type { identity_boPhanRa } from "@/lib/api/schema.gen";

import {
  API_SO_DO,
  banSua,
  banThem,
  dungCay,
  guiBieuMau,
  luaChonCha,
  moThem,
  thanSua,
  thanThem,
  traiPhang,
  voiConChau,
  type NutCay,
} from "./cay-bo-phan";
import { CHUA_CO_THAY_DOI, LOI_THU_TU } from "./nhan-so-do";

function bp(id: string, name: string, parent_id = "", order = 0, code = id.toLowerCase()): identity_boPhanRa {
  return { id, code, name, parent_id, order, staff_count: 2 };
}

/** Tên theo thứ tự hiển thị, thụt theo cấp — đọc được bằng mắt khi ca đỏ. */
function hinh(cay: readonly NutCay[]): string[] {
  return traiPhang(cay).map((d) => `${"  ".repeat(d.cap)}${d.bp.name}`);
}

/*
 * Sơ đồ mẫu, cố ý đưa vào LỘN THỨ TỰ (con đứng trước cha, thứ tự không theo mảng):
 *
 *   LÃNH ĐẠO (order 1)
 *     VĂN PHÒNG (order 2)
 *       TỔ MỘT CỬA (order 0)
 *     BỘ PHẬN A (order 2)   ← cùng order với VĂN PHÒNG, tên đứng trước theo vần
 *   ĐẢNG UỶ (order 1)       ← cùng order với LÃNH ĐẠO, tên đứng trước theo vần
 */
const VAN_PHONG = bp("01JVP", "VĂN PHÒNG", "01JLD", 2);
const MAU: readonly identity_boPhanRa[] = [
  bp("01JMOTCUA", "TỔ MỘT CỬA", "01JVP", 0),
  VAN_PHONG,
  bp("01JLD", "LÃNH ĐẠO", "", 1),
  bp("01JDU", "ĐẢNG UỶ", "", 1),
  bp("01JA", "BỘ PHẬN A", "01JLD", 2),
];

describe("dựng cây", () => {
  it("con dưới cha, mỗi cấp sắp theo order rồi theo tên", () => {
    expect(hinh(dungCay(MAU))).toEqual([
      "ĐẢNG UỶ",
      "LÃNH ĐẠO",
      "  BỘ PHẬN A",
      "  VĂN PHÒNG",
      "    TỔ MỘT CỬA",
    ]);
  });

  it("order nhỏ đứng trước dù tên đứng sau theo vần", () => {
    const cay = dungCay([bp("01J1", "AN", "", 5), bp("01J2", "XUÂN", "", 1)]);
    expect(hinh(cay)).toEqual(["XUÂN", "AN"]);
  });

  it("không bỏ mất bộ phận nào: cha không có trong danh sách, hay vòng lặp, thì lên cấp cao nhất", () => {
    const cay = dungCay([
      bp("01JMO", "MỒ CÔI", "01JKHONGCO"),
      bp("01JV1", "VÒNG MỘT", "01JV2"),
      bp("01JV2", "VÒNG HAI", "01JV1"),
    ]);
    const ten = traiPhang(cay).map((d) => d.bp.name);
    expect([...ten].sort()).toEqual(["MỒ CÔI", "VÒNG HAI", "VÒNG MỘT"].sort());
  });

  it("danh sách rỗng là cây rỗng", () => {
    expect(dungCay([])).toEqual([]);
  });
});

describe("ô Bộ phận cha", () => {
  it("con cháu tính đủ cả khi danh sách không theo thứ tự cha trước con", () => {
    expect([...voiConChau(MAU, "01JLD")].sort()).toEqual(["01JA", "01JLD", "01JMOTCUA", "01JVP"]);
  });

  it("biểu mẫu sửa bỏ chính bộ phận ấy và mọi con cháu của nó", () => {
    const ds = luaChonCha(dungCay(MAU), MAU, "01JLD").map((d) => d.bp.id);
    expect(ds).toEqual(["01JDU"]);
  });

  it("biểu mẫu sửa một lá chỉ bỏ chính nó", () => {
    const ds = luaChonCha(dungCay(MAU), MAU, "01JMOTCUA").map((d) => d.bp.id);
    expect(ds).toEqual(["01JDU", "01JLD", "01JA", "01JVP"]);
  });

  it("biểu mẫu thêm chọn được mọi bộ phận", () => {
    expect(luaChonCha(dungCay(MAU), MAU, null)).toHaveLength(MAU.length);
  });
});

describe("thân POST từ bản nháp", () => {
  it("mã để trống thì vắng — máy chủ tự sinh từ tên", () => {
    const d = thanThem({ ...banThem(""), ten: "VĂN PHÒNG", ma: "   " });
    expect(d).toEqual({ kieu: "gui", than: { name: "VĂN PHÒNG", parent_id: undefined, order: undefined, code: undefined } });
  });

  it("thêm bộ phận con mang parent_id của cha, thứ tự là số", () => {
    const d = thanThem({ ten: "TỔ", ma: "to", chaId: "01JVP", thuTu: " 4 " });
    expect(d).toEqual({ kieu: "gui", than: { name: "TỔ", parent_id: "01JVP", order: 4, code: "to" } });
  });

  it("tên gửi nguyên chữ đã gõ — không viết hoa hộ", () => {
    const d = thanThem({ ...banThem(""), ten: "Văn phòng" });
    expect(d.kieu === "gui" && d.than.name).toBe("Văn phòng");
  });

  it("thứ tự không phải số nguyên là lỗi tại chỗ", () => {
    expect(thanThem({ ...banThem(""), ten: "A", thuTu: "3 chữ" })).toEqual({ kieu: "loi", loi: LOI_THU_TU });
    expect(thanThem({ ...banThem(""), ten: "A", thuTu: "1.5" })).toEqual({ kieu: "loi", loi: LOI_THU_TU });
  });
});

describe("thân PATCH từ bản nháp", () => {
  const goc = bp("01JVP", "VĂN PHÒNG", "01JLD", 2, "van-phong");

  it("chỉ mang trường đã đổi", () => {
    expect(thanSua(goc, { ...banSua(goc), ten: "VĂN PHÒNG UBND" })).toEqual({
      kieu: "gui",
      than: { name: "VĂN PHÒNG UBND" },
    });
    expect(thanSua(goc, { ...banSua(goc), thuTu: "7" })).toEqual({ kieu: "gui", than: { order: 7 } });
    expect(thanSua(goc, { ...banSua(goc), chaId: "01JDU" })).toEqual({
      kieu: "gui",
      than: { parent_id: "01JDU" },
    });
  });

  it("dời lên gốc gửi parent_id: \"\"", () => {
    expect(thanSua(goc, { ...banSua(goc), chaId: "" })).toEqual({ kieu: "gui", than: { parent_id: "" } });
  });

  it("không bao giờ mang `code`, kể cả khi bản nháp có mã khác", () => {
    const d = thanSua(goc, { ...banSua(goc), ma: "ma-khac", ten: "B" });
    expect(d.kieu === "gui" && Object.keys(d.than)).toEqual(["name"]);
  });

  it("không đổi gì thì không gửi; ô thứ tự xoá trắng là không đổi, không phải 0", () => {
    expect(thanSua(goc, banSua(goc))).toEqual({ kieu: "khongDoi" });
    expect(thanSua(goc, { ...banSua(goc), thuTu: "" })).toEqual({ kieu: "khongDoi" });
  });
});

/* ---- gửi, qua đúng hai hàm API thật, với fetch bị thay ------------------------------------- */

function phanHoi(status: number, than: unknown) {
  return new Response(JSON.stringify(than), { status, headers: { "Content-Type": "application/json" } });
}

const DA_GHI = { id: "01JMOI", code: "moi", name: "BỘ PHẬN MỚI", parent_id: "", order: 0 };

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("gửi biểu mẫu", () => {
  it("Idempotency-Key sinh lúc MỞ, và lần bấm lại sau lỗi dùng ĐÚNG khoá ấy", async () => {
    const taoKhoa = vi.fn(() => `khoa-${taoKhoa.mock.calls.length}`);
    const dangMo = moThem("", null, taoKhoa);
    expect(taoKhoa).toHaveBeenCalledTimes(1);
    const khoaLucMo = dangMo.kieu === "them" ? dangMo.khoaChongTrung : "";

    const traVe = [phanHoi(503, { code: "x", message: "Máy chủ bận.", trace_id: "t" }), phanHoi(201, DA_GHI)];
    const gia = vi.fn(async () => traVe.shift() as Response);
    vi.stubGlobal("fetch", gia);

    const ban = { ...banThem(""), ten: "BỘ PHẬN MỚI" };
    expect(await guiBieuMau(dangMo, ban, API_SO_DO)).toEqual({ kieu: "loiMayChu", thongBao: "Máy chủ bận." });
    expect((await guiBieuMau(dangMo, ban, API_SO_DO)).kieu).toBe("xong");

    const khoa = gia.mock.calls.map((c) => {
      const init = (c as unknown as [string, RequestInit])[1];
      return new Headers(init.headers).get("Idempotency-Key");
    });
    expect(khoa).toEqual([khoaLucMo, khoaLucMo]);
    // Gửi không sinh thêm khoá nào.
    expect(taoKhoa).toHaveBeenCalledTimes(1);
  });

  it("mở biểu mẫu lần nữa là một lần thêm khác, một khoá khác", () => {
    const a = moThem("", null, () => crypto.randomUUID());
    const b = moThem("", null, () => crypto.randomUUID());
    expect(a.kieu === "them" && b.kieu === "them" && a.khoaChongTrung !== b.khoaChongTrung).toBe(true);
  });

  it("409 vòng lặp của máy chủ ra tới biểu mẫu NGUYÊN VĂN", async () => {
    const cau = "Không thể dời một bộ phận vào dưới chính nó hay dưới một bộ phận con của nó.";
    vi.stubGlobal("fetch", vi.fn(async () => phanHoi(409, { code: "org_unit_cycle", message: cau, trace_id: "t" })));
    const goc = VAN_PHONG;
    const kq = await guiBieuMau({ kieu: "sua", bp: goc }, { ...banSua(goc), chaId: "01JMOTCUA" }, API_SO_DO);
    expect(kq).toEqual({ kieu: "loiMayChu", thongBao: cau });
  });

  it("sửa mà không đổi gì thì không gọi mạng", async () => {
    const gia = vi.fn();
    vi.stubGlobal("fetch", gia);
    const goc = VAN_PHONG;
    expect(await guiBieuMau({ kieu: "sua", bp: goc }, banSua(goc), API_SO_DO)).toEqual({
      kieu: "loiTaiCho",
      loi: CHUA_CO_THAY_DOI,
    });
    expect(gia).not.toHaveBeenCalled();
  });

  it("sửa gửi PATCH không có `code`, lưu xong nói rõ bộ phận nào", async () => {
    const gia = vi.fn(async () => phanHoi(200, { ...DA_GHI, name: "TÊN MỚI" }));
    vi.stubGlobal("fetch", gia);
    const goc = VAN_PHONG;
    const kq = await guiBieuMau({ kieu: "sua", bp: goc }, { ...banSua(goc), ten: "TÊN MỚI" }, API_SO_DO);
    expect(kq).toEqual({ kieu: "xong", cau: "Đã lưu thay đổi của bộ phận TÊN MỚI." });
    const init = (gia.mock.calls[0] as unknown as [string, RequestInit])[1];
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toEqual({ name: "TÊN MỚI" });
  });
});
