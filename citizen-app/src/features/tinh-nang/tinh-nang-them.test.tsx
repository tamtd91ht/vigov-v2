import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { COMPANY, CONTACT } from "../../content/company-profile";

import { hieuUngGiuManSang } from "./giu-man-sang";
import { KhungTinhNang } from "./khung";
import { ManDanhThiep } from "./ManDanhThiep";
import { ManThiepCuaChungToi, TheQrCuaChungToi, ThongTinTrongMa } from "./ManThiepCuaChungToi";
import { duongDanMaQR, MaQR } from "./MaQR";
import { noiDung, THIEP_CUA_CHUNG_TOI } from "./noi-dung";
import { vCardCuaChungToi } from "./vcard";

/**
 * DANH THIẾP SỐ CỦA CHÚNG TÔI — và hai điều ở đây không có phép kiểm nào khác nói hộ:
 *
 *   1. **`keepScreen` phải được TẮT LẠI khi rời màn.** Bật rồi bỏ đó là lấy pin của người dùng
 *      cho một tính năng họ đã rời khỏi, và không có gì báo lỗi.
 *   2. **Mã QR không kéo theo một lời hứa nào sai**: tệp tải về là tệp của CHÚNG TÔI.
 *
 * (11/10/2026, owner: "Kiểm tra đường truyền" and "Số hoá thiếp giấy" were removed with their code; their cases
 * left with them.)
 *
 * Bộ test dựng bằng `react-dom/server` — không có DOM để bấm — nên mọi nhánh trạng thái đi vào
 * qua các component THUẦN, và hợp đồng của hiệu ứng `keepScreen` được kiểm bằng hàm tách rời.
 */

const ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#x27;": "'",
  "&#39;": "'",
};

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, (thuc_the) => ENTITIES[thuc_the] ?? thuc_the)
    .replace(/\s+/g, " ");

const ve = (phan_tu: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(phan_tu);

/* ============================================================================================
   2. DANH THIẾP SỐ CỦA CHÚNG TÔI — mã QR, giữ màn sáng, tải tệp
   ============================================================================================ */

describe("mã QR danh thiếp", () => {
  it("dựng đúng một ô tối thành một hình vuông 1×1 ở đúng toạ độ", () => {
    expect(duongDanMaQR([[true]])).toBe("M0 0h1v1h-1z");
    expect(duongDanMaQR([[false]])).toBe("");
    expect(duongDanMaQR([])).toBe("");
    // Hàng trước, cột sau: một mã vẽ lộn hai trục là một mã soi gương, không quét được.
    expect(duongDanMaQR([[false, true], [false, false]])).toBe("M1 0h1v1h-1z");
  });

  it("vẽ ra một SVG vuông, có ô tối, và BỊ ẨN khỏi trình đọc màn hình", () => {
    const markup = ve(<MaQR noi_dung={vCardCuaChungToi()} className="ma-qr" />);
    // `aria-hidden` vì một mã QR là một hình: trình đọc màn hình không đọc được nó, và mọi
    // thông tin trong mã đều được hiện lại bằng chữ ngay dưới mã (ca kế tiếp).
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).not.toMatch(/<(title|desc)[\s>]/);
    const viewBox = /viewBox="0 0 (\d+) (\d+)"/.exec(markup);
    expect(viewBox, "mã QR không có viewBox").not.toBeNull();
    expect(viewBox![1]).toBe(viewBox![2]);
    expect(Number(viewBox![1]), "mã QR nhỏ hơn cả một mã phiên bản 1").toBeGreaterThan(20);
    expect(/ d="M/.test(markup), "mã QR không có một ô tối nào").toBe(true);
  });

  it("mã của một nội dung dài hơn thì to hơn — bằng chứng nó mã hoá thật, không vẽ bừa", () => {
    const nho = /viewBox="0 0 (\d+)/.exec(ve(<MaQR noi_dung="A" />))![1]!;
    const to = /viewBox="0 0 (\d+)/.exec(ve(<MaQR noi_dung={"A".repeat(400)} />))![1]!;
    expect(Number(to)).toBeGreaterThan(Number(nho));
  });
});

describe("danh thiếp số của chúng tôi", () => {
  it("hiện bằng CHỮ đúng những thông tin nằm trong mã", () => {
    // Đây là điều kiện để cái mã kia được phép `aria-hidden`: người khiếm thị, và người không có
    // máy thứ hai để quét, phải đọc được đúng những gì người quét nhận được.
    const chu = textOf(ve(<ThongTinTrongMa />));
    expect(chu).toContain(COMPANY.name);
    expect(chu).toContain(CONTACT.hotlineDialable);
    expect(chu).toContain(CONTACT.email);
    // Trang web chỉ hiện khi tấm thiếp thật sự có trường ấy. `COMPANY.website` trống từ
    // 21/09/2026 (xem `company-profile.ts`), nên dòng "Trang web" KHÔNG được vẽ ra: một nhãn
    // không có giá trị là một dòng trình đọc màn hình vẫn đọc mà không nói được gì.
    if (COMPANY.website === undefined) {
      expect(chu, "vẽ nhãn Trang web trong khi tấm thiếp không có địa chỉ nào").not.toContain(
        "Trang web",
      );
    } else {
      expect(chu).toContain(COMPANY.website);
    }
  });

  it("nội dung mã và phần chữ đọc từ CÙNG MỘT nguồn", () => {
    // Hai bản của một sự thật thì một bản sẽ cũ, và bản cũ là bản người ta quét về danh bạ.
    const vcard = vCardCuaChungToi();
    const chu = textOf(ve(<TheQrCuaChungToi noi_dung_vcard={vcard} />));
    const co_that = [COMPANY.name, CONTACT.hotlineDialable, CONTACT.email, COMPANY.website].filter(
      (gia_tri): gia_tri is string => typeof gia_tri === "string",
    );
    for (const gia_tri of co_that) {
      expect(vcard, `mã thiếu ${gia_tri}`).toContain(gia_tri);
      expect(chu, `phần chữ thiếu ${gia_tri}`).toContain(gia_tri);
    }
  });

  it("nói vì sao cần quyền giữ màn hình sáng và ghi tệp, ngay trên nút", () => {
    // Nội dung tệp tải về được khai ở chính sách quyền riêng tư; màn chỉ nói mục đích.
    const chu = textOf(ve(<ManThiepCuaChungToi />));
    expect(chu).toContain(noiDung("thiep-cua-chung-toi").vi_sao);
  });

  it("có nút giữ màn hình sáng, và nút ấy nói trạng thái bằng CHỮ", () => {
    const markup = ve(<ManThiepCuaChungToi />);
    expect(textOf(markup)).toContain(THIEP_CUA_CHUNG_TOI.nut_giu_sang);
    // `aria-pressed` cho trình đọc màn hình; nhãn đổi cho người nhìn. Không có nút bật/tắt nào
    // được phép chỉ đổi màu.
    expect(markup).toContain('aria-pressed="false"');
  });

  it("mã QR hiện TRƯỚC nút — thứ chính của màn không nằm sau một lời xin quyền", () => {
    const markup = ve(<ManThiepCuaChungToi />);
    expect(markup.indexOf("<svg")).toBeGreaterThanOrEqual(0);
    expect(markup.indexOf('class="ma-qr__khung"')).toBeLessThan(markup.indexOf("<button"));
  });
});

describe("giữ màn hình sáng: bật khi vào, LUÔN tắt khi ra", () => {
  it("bật khi người dùng bật", () => {
    const da_goi: boolean[] = [];
    hieuUngGiuManSang(true, (bat) => da_goi.push(bat));
    expect(da_goi).toEqual([true]);
  });

  it("TẮT LẠI khi rời màn — đây là ca không có gì khác nói hộ", () => {
    // Bật rồi bỏ đó thì màn hình người dùng sáng tới khi họ đóng hẳn ứng dụng: ta lấy pin của
    // họ cho một tính năng họ đã rời khỏi. Không có gì báo lỗi, không có test nào khác đỏ, và
    // người phát hiện ra là người thấy máy mình nóng giữa buổi.
    const da_goi: boolean[] = [];
    const don_dep = hieuUngGiuManSang(true, (bat) => da_goi.push(bat));
    don_dep();
    expect(da_goi).toEqual([true, false]);
  });

  it("vẫn TẮT khi rời màn kể cả lúc chưa từng bật", () => {
    // Gọi tắt trên một màn chưa từng bật là vô hại; bỏ sót một lần tắt thì không. Sai lệch về
    // phía an toàn, có chủ đích.
    const da_goi: boolean[] = [];
    const don_dep = hieuUngGiuManSang(false, (bat) => da_goi.push(bat));
    expect(da_goi).toEqual([]);
    don_dep();
    expect(da_goi).toEqual([false]);
  });
});

/* ============================================================================================
   TAB DANH THIẾP — ba khối, một `<h1>`
   ============================================================================================ */

describe("tab Danh thiếp giữ hai việc quanh một tấm thiếp", () => {
  const markup = ve(<ManDanhThiep />);

  it("có ĐÚNG MỘT `<h1>` dù có hai khối", () => {
    // `screens.test.tsx` đo điều này trên mọi màn; ghi lại ở đây vì đây là màn có nhiều khối, tức
    // màn dễ mọc ra cái `<h1>` thứ hai.
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    expect((markup.match(/<h2\b/g) ?? []).length).toBe(1);
  });

  it("chứa cả hai tính năng danh thiếp — và không còn khối thiếp giấy", () => {
    const chu = textOf(markup);
    expect(chu).toContain(noiDung("danh-thiep").tieu_de);
    expect(chu).toContain(noiDung("thiep-cua-chung-toi").tieu_de);
    expect(chu).not.toMatch(/thiếp giấy/i);
  });

  it("không có ô nhập nào — dây bẫy giai đoạn 1 vẫn đúng ở đây", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });
});

/* ============================================================================================
   BỐN NHÁNH KẾT QUẢ, CHO DANH THIẾP CỦA CHÚNG TÔI
   ============================================================================================ */

describe("danh thiếp của chúng tôi cũng nói đủ bốn nhánh, bằng tiếng Việt", () => {
  for (const ma of ["thiep-cua-chung-toi"] as const) {
    const nd = noiDung(ma);
    for (const [nhanh, cau] of [
      ["tu-choi", nd.tu_choi],
      ["ngoai-zalo", nd.ngoai_zalo],
      ["khong-lay-duoc", nd.khong_lay_duoc],
    ] as const) {
      it(`${ma} · ${nhanh}: một câu nói việc cần làm, không một mã lỗi`, () => {
        const chu = textOf(
          ve(
            <KhungTinhNang
              ma={ma}
              trang_thai={{ kieu: nhanh }}
              onBam={() => {}}
              cap_tieu_de="h2"
              veKetQua={() => null}
            />,
          ),
        );
        expect(chu).toContain(cau);
        expect(cau).not.toMatch(/\b(error|Error|undefined|null|-\d{3})\b/);
      });
    }
  }
});
