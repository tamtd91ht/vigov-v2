import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { COMPANY, CONTACT } from "../../content/company-profile";

import { hieuUngGiuManSang } from "./giu-man-sang";
import { KhungTinhNang } from "./khung";
import { KetQuaDuongTruyen, KiemTraDuongTruyen } from "./KiemTraDuongTruyen";
import { ManDanhThiep } from "./ManDanhThiep";
import { ManThiepCuaChungToi, TheQrCuaChungToi, ThongTinTrongMa } from "./ManThiepCuaChungToi";
import { duongDanMaQR, MaQR } from "./MaQR";
import {
  DUONG_TRUYEN,
  KIEU_KET_NOI,
  kieuKetNoi,
  noiDung,
  SO_HOA_THIEP,
  THIEP_CUA_CHUNG_TOI,
} from "./noi-dung";
import { AnhDanhThiep, SoHoaThiepGiay, TraLoiQuyenMayAnh } from "./SoHoaThiepGiay";
import { vCardCuaChungToi } from "./vcard";

/**
 * BA TÍNH NĂNG THÊM VÀO, SÁU QUYỀN THÊM VÀO — và bốn điều ở đây không có phép kiểm nào khác nói hộ:
 *
 *   1. **Nhãn kiểu kết nối phải đủ bốn giá trị nền tảng khai HÔM NAY, cộng một giá trị lạ.** Giá
 *      trị lạ là trường hợp đáng kiểm nhất: nó xảy ra khi Zalo thêm một kiểu mạng mới, tức là
 *      SAU khi ứng dụng này đã nằm trên máy người dùng.
 *   2. **`keepScreen` phải được TẮT LẠI khi rời màn.** Bật rồi bỏ đó là lấy pin của người dùng
 *      cho một tính năng họ đã rời khỏi, và không có gì báo lỗi.
 *   3. **Ranh giới phải được nói ra**: chưa đọc được chữ trên ảnh, chưa đo được tốc độ mạng.
 *      Một màn hình im lặng về thứ nó không làm là một màn hình hứa suông.
 *   4. **Ảnh và mã QR không kéo theo một lời hứa nào sai**: ảnh không rời khỏi máy, tệp tải về
 *      là tệp của CHÚNG TÔI.
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
   1. KIỂM TRA ĐƯỜNG TRUYỀN — ánh xạ `networkType` sang nhãn tiếng Việt
   ============================================================================================ */

describe("kiểu kết nối: bốn giá trị nền tảng khai, cộng mọi giá trị nó chưa khai", () => {
  it("ánh xạ đủ BỐN giá trị của `NetworkType` hôm nay", () => {
    // Bốn giá trị đọc từ `node_modules/zmp-sdk/index.d.ts` dòng 7–16, không từ tài liệu web.
    expect(kieuKetNoi("wifi")).toBe("wifi");
    expect(kieuKetNoi("cellular")).toBe("cellular");
    expect(kieuKetNoi("none")).toBe("none");
    expect(kieuKetNoi("unknown")).toBe("khong-xac-dinh");
  });

  it("một giá trị LẠ rơi về nhãn an toàn thay vì làm vỡ tính năng", () => {
    // Nền tảng được phép thêm giá trị thứ năm ở một bản SDK sau. Lúc ấy app đã nằm trên máy
    // người dùng: ném lỗi là làm vỡ một tính năng vì nền tảng vừa tốt lên, còn hiện chuỗi thô là
    // hiện chữ kỹ thuật tiếng Anh giữa một màn hình tiếng Việt.
    for (const la of ["5g", "ethernet", "WIFI", "", "  ", "null"]) {
      expect(kieuKetNoi(la), `giá trị lạ "${la}" không rơi về nhãn an toàn`).toBe("khong-xac-dinh");
    }
  });

  it("mỗi nhãn có một câu giải thích, và không câu nào chứa một con số đo được", () => {
    // ⚠ KHÔNG BỊA SỐ LIỆU. Nền tảng trả về đúng một chuỗi — kiểu kết nối. Một con số ms, Mbps
    // hay một điểm chất lượng ở đây là một phép đo không hề chạy, in ra trong ứng dụng của một
    // nhà cung cấp hạ tầng thoại — đúng chỗ khách hàng sẽ đem đối chiếu với máy đo thật.
    for (const [ma, mot] of Object.entries(KIEU_KET_NOI)) {
      expect(mot.nhan.length, `${ma} không có nhãn`).toBeGreaterThan(0);
      expect(mot.y_nghia.length, `${ma} không có câu giải thích`).toBeGreaterThan(30);
      expect(mot.y_nghia, `${ma} hứa một con số`).not.toMatch(/\d+\s*(ms|Mbps|kbps|%|điểm)/i);
    }
  });

  it("nói ra rằng nó KHÔNG đo tốc độ, ngay cạnh mọi kết quả", () => {
    for (const kieu of ["wifi", "cellular", "none", "unknown", "5g"]) {
      const chu = textOf(ve(<KetQuaDuongTruyen tu_nen_tang={kieu} />));
      expect(chu, `${kieu}: thiếu câu nói ra ranh giới`).toContain(DUONG_TRUYEN.khong_do_toc_do);
    }
  });

  it("hiện nhãn bằng CHỮ, không bằng riêng một màu hay một vạch sóng", () => {
    const chu = textOf(ve(<KetQuaDuongTruyen tu_nen_tang="cellular" />));
    expect(chu).toContain(DUONG_TRUYEN.nhan_ket_qua);
    expect(chu).toContain(KIEU_KET_NOI.cellular.nhan);
    expect(chu).toContain(KIEU_KET_NOI.cellular.y_nghia);
  });

  it("nói vì sao nó cần quyền, ngay trên nút — điều 3.3.4 của chính sách Mini App", () => {
    const chu = textOf(ve(<KiemTraDuongTruyen />));
    expect(chu).toContain(noiDung("duong-truyen").vi_sao);
    expect(chu).toContain(noiDung("duong-truyen").nut);
    expect(chu).toContain(DUONG_TRUYEN.dan_nhap);
  });
});

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

  it("nói ra rằng tệp tải về là tệp CỦA CHÚNG TÔI, không phải dữ liệu của người dùng", () => {
    // "Ứng dụng ghi một tệp xuống máy bạn" là một hành vi mới. Nói ra nó là nghĩa vụ, và nói ra
    // tệp ấy chứa gì là thứ phân biệt một lời khai với một lời trấn an.
    const chu = textOf(ve(<ManThiepCuaChungToi />));
    expect(chu).toContain(THIEP_CUA_CHUNG_TOI.tep_la_cua_chung_toi);
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
   3. SỐ HOÁ DANH THIẾP GIẤY — quyền máy ảnh, chọn ảnh, và ranh giới OCR
   ============================================================================================ */

describe("số hoá danh thiếp giấy", () => {
  it("TỪ CHỐI QUYỀN là đường đi bình thường: không mã lỗi, và nói việc còn làm được", () => {
    const chu = textOf(ve(<TraLoiQuyenMayAnh cho_phep={false} />));
    expect(chu).toContain(SO_HOA_THIEP.tu_choi_quyen);
    expect(chu).not.toMatch(/\b(error|Error|lỗi|mã lỗi|-\d{3})\b/);
    // Và nó nói ra điều `index.d.ts` đã ghi: câu trả lời được ghi nhớ cho những lần sau, muốn
    // đổi ý thì vào phần Quản lý quyền của Zalo.
    expect(chu).toContain("Quản lý quyền");
  });

  it("CHO PHÉP thì nói việc cần làm tiếp, không dừng ở một dấu tích", () => {
    expect(textOf(ve(<TraLoiQuyenMayAnh cho_phep />))).toContain(SO_HOA_THIEP.cho_phep);
  });

  it("nói thẳng rằng bản này CHƯA đọc được chữ trên ảnh", () => {
    // ⚠ Ranh giới thật: OCR cần một bước máy chủ. Màn hình nói ra thay vì vẽ một thanh tiến
    // trình cho một việc không xảy ra.
    const chu = textOf(ve(<SoHoaThiepGiay />));
    expect(chu).toContain(SO_HOA_THIEP.chua_doc_duoc_chu);
  });

  it("nói ra rằng ảnh KHÔNG RỜI KHỎI MÁY, và nói trước khi có ảnh nào", () => {
    // Người quyết định có đưa ảnh của một người khác vào hay không cần đọc câu này TRƯỚC.
    const chu = textOf(ve(<SoHoaThiepGiay />));
    expect(chu).toContain(SO_HOA_THIEP.anh_khong_roi_may);
    expect(chu).toContain(SO_HOA_THIEP.nut_chon_anh);
  });

  it("hiện ảnh bằng một `alt` mô tả VAI TRÒ, không bịa nội dung tấm thiếp", () => {
    const markup = ve(
      <AnhDanhThiep duong_dan="/tam/anh-vua-chon.jpg" hong={false} onHong={() => {}} />,
    );
    expect(markup).toContain('src="/tam/anh-vua-chon.jpg"');
    expect(markup).toContain(`alt="${SO_HOA_THIEP.nhan_anh}"`);
  });

  it("ảnh không nạp được thì nói việc cần làm, không để một khung vỡ", () => {
    const chu = textOf(ve(<AnhDanhThiep duong_dan="/tam/hong.jpg" hong onHong={() => {}} />));
    expect(chu).toContain(SO_HOA_THIEP.khong_hien_duoc_anh);
  });
});

/* ============================================================================================
   TAB DANH THIẾP — ba khối, một `<h1>`
   ============================================================================================ */

describe("tab Danh thiếp giữ ba việc quanh một tấm thiếp", () => {
  const markup = ve(<ManDanhThiep />);

  it("có ĐÚNG MỘT `<h1>` dù có ba khối", () => {
    // `screens.test.tsx` đo điều này trên mọi màn; ghi lại ở đây vì đây là màn duy nhất có ba
    // khối, tức màn duy nhất dễ mọc ra cái `<h1>` thứ hai.
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    expect((markup.match(/<h2\b/g) ?? []).length).toBe(2);
  });

  it("chứa cả ba tính năng danh thiếp", () => {
    const chu = textOf(markup);
    expect(chu).toContain(noiDung("danh-thiep").tieu_de);
    expect(chu).toContain(noiDung("thiep-cua-chung-toi").tieu_de);
    expect(chu).toContain(noiDung("so-hoa-thiep").tieu_de);
  });

  it("không có ô nhập nào — dây bẫy giai đoạn 1 vẫn đúng ở đây", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });
});

/* ============================================================================================
   BỐN NHÁNH KẾT QUẢ, CHO CẢ BA TÍNH NĂNG MỚI
   ============================================================================================ */

describe("ba tính năng mới cũng nói đủ bốn nhánh, bằng tiếng Việt", () => {
  for (const ma of ["duong-truyen", "thiep-cua-chung-toi", "so-hoa-thiep"] as const) {
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
