import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_boPhanRa } from "@/lib/api/schema.gen";

import { banSua, banThem, dungCay, luaChonCha, moThem } from "./cay-bo-phan";
import {
  CAU_THIEU_QUYEN_GHI,
  GHI_CHU_CHUA_XOA,
  NUT_SUA_BO_PHAN,
  NUT_THEM_BO_PHAN,
  NUT_THEM_CON,
  nhanCayRong,
} from "./nhan-so-do";
import { BieuMauBoPhan, KhungSoDo, type ThaoTacCay } from "./tab-so-do-to-chuc";

/**
 * Canh việc các quyết định của `cay-bo-phan.ts` CÓ RA TỚI TRANG: nút ghi ẩn khi thiếu `admin.org`,
 * không có nút xoá nào, câu 409 hiện nguyên văn, biểu mẫu sửa không có ô nhập mã. Kết xuất bằng
 * `react-dom/server` trong Node — không jsdom (lý do ở `vitest.config.mts`).
 */

const KHONG_LAM_GI: ThaoTacCay = { themGoc: () => {}, themCon: () => {}, sua: () => {} };

function bp(id: string, name: string, parent_id = "", order = 0): identity_boPhanRa {
  return { id, code: id.toLowerCase(), name, parent_id, order, staff_count: 3 };
}

const LANH_DAO = bp("01JLD", "LÃNH ĐẠO", "", 1);
const MAU = [LANH_DAO, bp("01JVP", "VĂN PHÒNG", "01JLD", 1), bp("01JDU", "ĐẢNG UỶ", "", 2)];

function khung(coQuyenGhi: boolean, thieuQuyen = !coQuyenGhi, items = MAU) {
  return renderToStaticMarkup(
    <KhungSoDo
      tai={{ pha: "xong", items }}
      cay={dungCay(items)}
      coQuyenGhi={coQuyenGhi}
      thieuQuyen={thieuQuyen}
      thaoTac={KHONG_LAM_GI}
      bieuMauDauTab={null}
      bieuMauTaiThe={null}
    />,
  );
}

describe("thẻ bộ phận và nút ghi", () => {
  it("có admin.org: nút Thêm bộ phận, và ＋ ✎ trên TỪNG thẻ", () => {
    const html = khung(true);
    expect(html).toContain(NUT_THEM_BO_PHAN);
    expect(html.split(NUT_THEM_CON).length - 1).toBe(MAU.length);
    expect(html.split(NUT_SUA_BO_PHAN).length - 1).toBe(MAU.length);
    expect(html).toContain('aria-label="Thêm bộ phận con của VĂN PHÒNG"');
    expect(html).toContain('aria-label="Sửa bộ phận VĂN PHÒNG"');
    expect(html).not.toContain(CAU_THIEU_QUYEN_GHI);
  });

  it("KHÔNG có admin.org: cây vẫn hiện, không nút ghi nào, và nói vì sao", () => {
    const html = khung(false);
    expect(html).toContain("VĂN PHÒNG");
    expect(html).toContain("3 cán bộ");
    expect(html).not.toContain("<button");
    expect(html).toContain(CAU_THIEU_QUYEN_GHI);
  });

  it("chưa đọc xong phiên: không nút ghi, cũng chưa nói thiếu quyền", () => {
    const html = khung(false, false);
    expect(html).not.toContain("<button");
    expect(html).not.toContain(CAU_THIEU_QUYEN_GHI);
  });

  it("không vẽ nút xoá nào, kể cả nút mờ — và nói ra là chưa xoá được", () => {
    const html = khung(true);
    expect(html).not.toContain("🗑");
    expect(html).not.toMatch(/Xoá bộ phận/);
    expect(html).toContain(GHI_CHU_CHUA_XOA);
  });

  it("con nằm trong danh sách lồng của cha", () => {
    const html = khung(true);
    const cha = html.indexOf("LÃNH ĐẠO");
    const con = html.indexOf("VĂN PHÒNG");
    const doanGiua = html.slice(cha, con);
    expect(doanGiua).toContain('<ul class="cay-bo-phan">');
    expect(html.indexOf("ĐẢNG UỶ")).toBeGreaterThan(con);
  });

  it("sơ đồ rỗng nói theo quyền", () => {
    expect(khung(true, false, [])).toContain(nhanCayRong(true));
    expect(khung(false, true, [])).toContain(nhanCayRong(false));
  });

  it("lỗi đọc hiện nguyên câu của máy chủ", () => {
    const html = renderToStaticMarkup(
      <KhungSoDo
        tai={{ pha: "loi", thongBao: "Phiên đăng nhập đã hết hạn." }}
        cay={[]}
        coQuyenGhi={false}
        thieuQuyen={false}
        thaoTac={KHONG_LAM_GI}
        bieuMauDauTab={null}
        bieuMauTaiThe={null}
      />,
    );
    expect(html).toContain("Phiên đăng nhập đã hết hạn.");
  });
});

describe("biểu mẫu bộ phận", () => {
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

  it("biểu mẫu sửa: mã hiện thành chữ, không có ô nhập mã; ô cha không có chính nó và con cháu", () => {
    const goc = LANH_DAO;
    const html = bieuMau({
      dangMo: { kieu: "sua", bp: goc },
      ban: banSua(goc),
      luaChon: luaChonCha(cay, MAU, goc.id),
    });
    expect(html).not.toContain('name="ma"');
    expect(html).toContain("Mã 01jld. Mã đã cấp thì không sửa được.");
    expect(html).not.toContain('value="01JLD"');
    expect(html).not.toContain('value="01JVP"');
    expect(html).toContain('value="01JDU"');
  });

  it("biểu mẫu thêm con: có ô mã, cha chọn sẵn, nhãn thật cho từng ô", () => {
    const html = bieuMau({
      dangMo: moThem("01JLD", "LÃNH ĐẠO", () => "k"),
      ban: banThem("01JLD"),
      luaChon: luaChonCha(cay, MAU, null),
    });
    expect(html).toContain("Thêm bộ phận con của LÃNH ĐẠO");
    expect(html).toContain('name="ma"');
    for (const id of ["o-ten-bo-phan", "o-ma-bo-phan", "o-cha-bo-phan", "o-thu-tu-bo-phan"]) {
      expect(html).toContain(`for="${id}"`);
      expect(html).toContain(`id="${id}"`);
    }
    expect(html).toMatch(/<option value="01JLD" selected="">/);
  });
});
