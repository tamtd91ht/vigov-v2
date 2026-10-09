import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_boPhanRa } from "@/lib/api/schema.gen";

import { banSua, banThem, dungCay, luaChonCha, moThem } from "./cay-bo-phan";
import { IMPORT_BUTTON } from "./excel-import-flow";
import { DIALOG_DESCRIPTION, EMPTY_TREE, NAME_REQUIRED, NUT_THEM_BO_PHAN } from "./nhan-so-do";
import { DELETE_BUTTON } from "./org-unit-delete";
import { BieuMauBoPhan, KhungSoDo } from "./tab-so-do-to-chuc";
import type { ThaoTacCay } from "./tab-so-do-to-chuc";

/**
 * Canh việc các quyết định của `cay-bo-phan.ts` CÓ RA TỚI TRANG theo spec 03: nút ghi ẩn khi thiếu
 * `admin.org`, cây lồng có đường nét đứt, hộp thoại không có ô mã, câu 409 hiện nguyên văn. Kết xuất
 * bằng `react-dom/server` trong Node — không jsdom (lý do ở `vitest.config.mts`).
 */

const KHONG_LAM_GI: ThaoTacCay = { themGoc: () => {}, themCon: () => {}, sua: () => {}, xoa: () => {} };

function bp(id: string, name: string, parent_id = "", order = 0): identity_boPhanRa {
  return { id, code: id.toLowerCase(), name, parent_id, order, staff_count: 3 };
}

const LANH_DAO = bp("01JLD", "LÃNH ĐẠO", "", 1);
const MAU = [LANH_DAO, bp("01JVP", "VĂN PHÒNG", "01JLD", 1), bp("01JDU", "ĐẢNG UỶ", "", 2)];

function khung(coQuyenGhi: boolean, items = MAU) {
  return renderToStaticMarkup(
    <KhungSoDo
      tai={{ pha: "xong", items }}
      cay={dungCay(items)}
      coQuyenGhi={coQuyenGhi}
      thaoTac={KHONG_LAM_GI}
      onImported={() => {}}
    />,
  );
}

describe("thẻ bộ phận và nút ghi", () => {
  it("có admin.org: nút Thêm bộ phận, và ＋ ✎ 🗑 trên TỪNG thẻ, title theo spec, tên đọc được kèm tên bộ phận", () => {
    const html = khung(true);
    expect(html).toContain(NUT_THEM_BO_PHAN);
    expect(html.match(/aria-label="Thêm bộ phận con của [^"]+"/g)?.length).toBe(MAU.length);
    expect(html.match(/aria-label="Sửa bộ phận [^"]+"/g)?.length).toBe(MAU.length);
    expect(html.match(/aria-label="Xoá bộ phận [^"]+"/g)?.length).toBe(MAU.length);
    expect(html).toContain('aria-label="Thêm bộ phận con của VĂN PHÒNG"');
    expect(html).toContain('aria-label="Sửa bộ phận VĂN PHÒNG"');
    expect(html).toContain('aria-label="Xoá bộ phận VĂN PHÒNG"');
    expect(html.match(/title="Thêm bộ phận con"/g)?.length).toBe(MAU.length);
    expect(html.match(/title="Sửa bộ phận"/g)?.length).toBe(MAU.length);
    expect(html.match(/title="Xoá bộ phận"/g)?.length).toBe(MAU.length);
    expect(html).toContain(IMPORT_BUTTON);
  });

  it("CA BỊ TỪ CHỐI: thiếu admin.org → cây vẫn hiện, không nút nào, không câu 'chỉ xem'", () => {
    const html = khung(false);
    expect(html).toContain("VĂN PHÒNG");
    expect(html).toContain("3 cán bộ");
    expect(html).not.toContain("<button");
    expect(html).not.toContain(DELETE_BUTTON);
    expect(html).not.toContain(IMPORT_BUTTON);
    expect(html).not.toMatch(/Xoá bộ phận/);
    expect(html).not.toContain("chỉ xem");
  });

  it("REGRESSION (spec 03): thẻ theo spec, không lớp legacy `.the-bo-phan` / `.cay-bo-phan`", () => {
    const html = khung(true);
    expect(html).toContain(
      "border-line shadow-card flex flex-wrap items-center gap-3 rounded-[10px] border border-solid bg-white px-4 py-2.5",
    );
    // ~4px between cards, ~65px per card (user decision 09/10/2026, spec §3).
    expect(html).toMatch(/<ul class="m-0 list-none space-y-1 p-0" aria-label=/);
    expect(html).toContain("bg-navy/8 text-navy grid size-9");
    expect(html).toContain('<code class="text-ink-muted text-[11px]">01jvp</code>');
    expect(html).not.toMatch(/class="[^"]*\b(the-bo-phan|cay-bo-phan|cum-nut)\b/);
    // Every small button undoes the legacy 32px min-height (h-7).
    expect(html.match(/<button[^>]*class="[^"]*\bh-7\b[^"]*"/g)?.length).toBe(
      html.match(/<button[^>]*class="[^"]*\bmin-h-0\b[^"]*"/g)?.length,
    );
  });

  it("con nằm trong danh sách lồng của cha, có đường nét đứt bên trái", () => {
    const html = khung(true);
    const cha = html.indexOf("LÃNH ĐẠO");
    const con = html.indexOf("VĂN PHÒNG");
    const doanGiua = html.slice(cha, con);
    expect(doanGiua).toMatch(/<ul class="[^"]*border-l-2 border-dashed[^"]*pl-5">/);
    expect(html.indexOf("ĐẢNG UỶ")).toBeGreaterThan(con);
  });

  it("sơ đồ rỗng: một câu cho mọi tài khoản", () => {
    expect(khung(true, [])).toContain(EMPTY_TREE);
    expect(khung(false, [])).toContain(`>${EMPTY_TREE}</p>`);
  });

  it("đang tải: khung xương, câu đọc cho trình đọc màn hình", () => {
    const html = renderToStaticMarkup(
      <KhungSoDo tai={{ pha: "dangDoc" }} cay={[]} coQuyenGhi={false} thaoTac={KHONG_LAM_GI} onImported={() => {}} />,
    );
    expect(html).toContain('role="status"');
    expect(html.match(/h-11 w-full/g)?.length).toBe(3);
  });

  it("REGRESSION (S12): đang tải với admin.org — chỉ hàng Nhập từ Excel + khung xương, chưa có nút Thêm bộ phận", () => {
    const html = renderToStaticMarkup(
      <KhungSoDo tai={{ pha: "dangDoc" }} cay={[]} coQuyenGhi thaoTac={KHONG_LAM_GI} onImported={() => {}} />,
    );
    expect(html).toContain(IMPORT_BUTTON);
    expect(html).not.toContain(NUT_THEM_BO_PHAN);
    expect(html.match(/h-11 w-full/g)?.length).toBe(3);
  });

  it("lỗi đọc hiện nguyên câu của máy chủ", () => {
    const html = renderToStaticMarkup(
      <KhungSoDo
        tai={{ pha: "loi", thongBao: "Phiên đăng nhập đã hết hạn." }}
        cay={[]}
        coQuyenGhi={false}
        thaoTac={KHONG_LAM_GI}
        onImported={() => {}}
      />,
    );
    expect(html).toContain("Phiên đăng nhập đã hết hạn.");
  });
});

describe("hộp thoại bộ phận", () => {
  const cay = dungCay(MAU);

  function bieuMau(props: Partial<Parameters<typeof BieuMauBoPhan>[0]> & Pick<Parameters<typeof BieuMauBoPhan>[0], "dangMo" | "ban" | "luaChon">) {
    return renderToStaticMarkup(
      <BieuMauBoPhan
        datBan={() => {}}
        loiTaiCho=""
        loiMayChu=""
        dangGui={false}
        onGui={() => {}}
        onHuy={() => {}}
        {...props}
      />,
    );
  }

  it("câu 409 vòng lặp của máy chủ hiện nguyên văn, chữ đã gõ còn nguyên", () => {
    const cau = "Không thể dời một bộ phận vào dưới chính nó hay dưới một bộ phận con của nó.";
    const goc = LANH_DAO;
    const html = bieuMau({
      dangMo: { kieu: "sua", bp: goc },
      ban: { ...banSua(goc), ten: "LÃNH ĐẠO MỚI" },
      luaChon: luaChonCha(cay, MAU, goc.id),
      loiMayChu: cau,
    });
    expect(html).toContain(cau);
    expect(html).toContain('value="LÃNH ĐẠO MỚI"');
  });

  it("REGRESSION (spec 03): sửa — tiêu đề 'Sửa bộ phận', không ô mã, không dòng 'Mã đã cấp'; ô Trực thuộc không có chính nó và con cháu", () => {
    const goc = LANH_DAO;
    const html = bieuMau({
      dangMo: { kieu: "sua", bp: goc },
      ban: banSua(goc),
      luaChon: luaChonCha(cay, MAU, goc.id),
    });
    expect(html).toMatch(/<h2[^>]*>Sửa bộ phận<\/h2>/);
    expect(html).toContain(DIALOG_DESCRIPTION);
    expect(html).not.toContain('name="ma"');
    expect(html).not.toContain("Mã đã cấp");
    expect(html).not.toContain('value="01JLD"');
    expect(html).not.toContain('value="01JVP"');
    expect(html).toContain('value="01JDU"');
    expect(html).toContain(">Lưu<");
    expect(html).not.toMatch(/class="[^"]*\b(form-danh-muc|o-nhap|ghi-chu)\b/);
  });

  it("thêm con: tiêu đề 'Thêm bộ phận', cha chọn sẵn, Tên → Trực thuộc → Thứ tự, nút Huỷ rồi Thêm", () => {
    const html = bieuMau({
      dangMo: moThem("01JLD", "LÃNH ĐẠO", () => "k"),
      ban: banThem("01JLD"),
      luaChon: luaChonCha(cay, MAU, null),
    });
    expect(html).toMatch(/<h2[^>]*>Thêm bộ phận<\/h2>/);
    expect(html).not.toContain('name="ma"');
    expect(html).toContain('placeholder="Ví dụ: Bộ phận Một cửa"');
    expect(html).toContain('<option value="">— Trực thuộc Uỷ ban —</option>');
    for (const id of ["o-ten-bo-phan", "o-cha-bo-phan", "o-thu-tu-bo-phan"]) {
      expect(html).toContain(`for="${id}"`);
      expect(html).toContain(`id="${id}"`);
    }
    expect(html.indexOf(">Tên bộ phận<")).toBeLessThan(html.indexOf(">Trực thuộc<"));
    expect(html.indexOf(">Trực thuộc<")).toBeLessThan(html.indexOf(">Thứ tự<"));
    expect(html).toMatch(/<option value="01JLD" selected="">/);
    expect(html.indexOf(">Huỷ<")).toBeLessThan(html.indexOf(">Thêm<"));
    expect(html).toContain("sm:max-w-lg");
    expect(html).toContain("bg-muted/50");
  });

  it("tên trống: câu lỗi hiện TẠI CHỖ trong hộp, ô tên đánh dấu sai", () => {
    const html = bieuMau({
      dangMo: moThem("", null, () => "k"),
      ban: banThem(""),
      luaChon: luaChonCha(cay, MAU, null),
      loiTaiCho: NAME_REQUIRED,
    });
    expect(html).toContain(`role="alert" class="text-danger m-0 text-[12px] font-medium">${NAME_REQUIRED}</p>`);
    expect(html).toMatch(/id="o-ten-bo-phan"[^>]*aria-invalid="true"/);
  });
});
