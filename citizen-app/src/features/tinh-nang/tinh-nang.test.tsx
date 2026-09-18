import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { OFFICES } from "../../content/company-profile";

import { docMaQR, thiepCoNoiDung } from "./danh-thiep";
import { cheToken, KetQuaToken, KhungTinhNang } from "./khung";
import { DangKyTuVan, DanhSachVanPhong, duongDanBanDo, TimVanPhong } from "./LienHeTinhNang";
import { KetQuaQuet, ManDanhThiep, TheDanhThiep } from "./ManDanhThiep";
import { MAN_DANH_THIEP } from "./index";
import {
  CHI_HIEN_LEN_MAN_HINH,
  DANH_THIEP,
  MA_RONG,
  type MaTinhNang,
  noiDung,
  NOI_DUNG_TINH_NANG,
  TOKEN_KHONG_CHUA_GI,
  TU_VAN,
  VAN_PHONG,
} from "./noi-dung";

/**
 * KÊNH CÔNG DÂN ĐÃ TỪNG ĐO ĐƯỢC **KHÔNG CÓ TEST NÀO**. Tệp này là phần của ba tính năng.
 *
 * BỐN ĐIỀU Ở ĐÂY KHÔNG CÓ PHÉP KIỂM NÀO KHÁC NÓI HỘ:
 *
 *   1. **Bộ bóc tách vCard.** Đây là phần LOGIC THẬT duy nhất của ứng dụng, nên nó là phần đáng
 *      test nhất. Một dòng gập không nối lại làm một địa chỉ email của người thật hiện ra sai,
 *      và người dùng gửi thư đi đâu đó.
 *   2. **Người dùng từ chối là đường đi bình thường.** Nhánh ấy phải hiện một câu tiếng Việt nói
 *      họ bấm lại được, KHÔNG phải một chuỗi lỗi kỹ thuật (README §Error message shape).
 *   3. **Màn hình nói ra rằng số điện thoại và toạ độ KHÔNG tới thiết bị**, và nói ra thứ bản
 *      dựng này CHƯA làm được với cái token ấy. Bỏ câu thứ hai đi là hứa một việc mã không làm.
 *   4. **Từ chối quyền không được làm mất tính năng.** Ba văn phòng và hai đường liên hệ phải
 *      còn nguyên trên màn, dù người dùng có bấm đồng ý hay không.
 *
 * Bộ test dựng bằng `react-dom/server` — không có DOM để bấm — nên bốn nhánh kết quả đi vào qua
 * `KhungTinhNang`, bản THUẦN nhận trạng thái bằng tham số.
 */

const ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#x27;": "'",
  "&#39;": "'",
};

/** Chữ người dùng đọc, không phải thẻ HTML. Giải mã thực thể vì câu chữ mới là thứ được kiểm. */
const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, (thuc_the) => ENTITIES[thuc_the] ?? thuc_the)
    .replace(/\s+/g, " ");

const ve = (phan_tu: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(phan_tu);

/* ============================================================================================
   BỘ BÓC TÁCH MÃ QR — phần logic thật duy nhất, nên là phần được kiểm kỹ nhất.
   ============================================================================================ */

const VCARD_DU = [
  "BEGIN:VCARD",
  "VERSION:3.0",
  "N:Nguyễn;An;Văn;;",
  "FN:Nguyễn Văn An",
  "ORG:Công ty TNHH Giải pháp Số;Phòng Kinh doanh",
  "TITLE:Trưởng phòng Kinh doanh",
  "TEL;TYPE=CELL:+84901234000",
  "EMAIL;TYPE=WORK:an.nguyen@vidu.vn",
  "URL:https://vidu.vn",
  "END:VCARD",
].join("\r\n");

describe("bóc tách mã QR: một chuỗi vào, một đối tượng ra, không tác dụng phụ", () => {
  it("vCard đủ trường: bóc đúng cả bảy thứ", () => {
    const doc = docMaQR(VCARD_DU);
    expect(doc.loai).toBe("danh-thiep");
    if (doc.loai !== "danh-thiep") return;

    // `FN` thắng `N` khi cả hai có mặt: đó là tên chủ thẻ tự viết ra.
    expect(doc.ho_ten).toBe("Nguyễn Văn An");
    expect(doc.chuc_danh).toBe("Trưởng phòng Kinh doanh");
    // `ORG` phân cấp bằng `;` — nối lại bằng dấu phẩy, không để lộ dấu chấm phẩy ra màn hình.
    expect(doc.to_chuc).toBe("Công ty TNHH Giải pháp Số, Phòng Kinh doanh");
    expect(doc.dien_thoai).toEqual(["+84901234000"]);
    expect(doc.email).toEqual(["an.nguyen@vidu.vn"]);
    expect(doc.trang_web).toEqual(["https://vidu.vn"]);
  });

  it("THAM SỐ TRÊN TÊN TRƯỜNG bị bỏ đi, giá trị thì không", () => {
    // `TEL;TYPE=CELL,VOICE;PREF=1:…` là hình dạng thật trên danh thiếp xuất từ iPhone. Cắt sai
    // chỗ thì số điện thoại hiện ra là "TYPE=CELL,VOICE" — và nút Gọi bấm vào một chuỗi rác.
    const doc = docMaQR(
      "BEGIN:VCARD\nTEL;TYPE=CELL,VOICE;PREF=1:0283 1010 000\nEND:VCARD",
    );
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.dien_thoai).toEqual(["0283 1010 000"]);
  });

  it("TIỀN TỐ NHÓM kiểu Apple (`item1.`) cũng bị bỏ", () => {
    const doc = docMaQR("BEGIN:VCARD\nitem1.EMAIL:lien.he@vidu.vn\nEND:VCARD");
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.email).toEqual(["lien.he@vidu.vn"]);
  });

  it("DÒNG GẬP được nối lại trước khi bóc", () => {
    // RFC 6350 §3.2: dòng sau bắt đầu bằng một khoảng trắng là phần tiếp của dòng trước. Không
    // nối thì email bị cắt làm đôi, và nửa sau thành một trường rác.
    const doc = docMaQR(
      ["BEGIN:VCARD", "EMAIL:mot-dia-chi-rat-dai-cua-doi", " -tac@vidu.vn", "END:VCARD"].join("\n"),
    );
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.email).toEqual(["mot-dia-chi-rat-dai-cua-doi-tac@vidu.vn"]);
  });

  it("vCard THIẾU TRƯỜNG là chuyện bình thường, không phải lỗi", () => {
    const doc = docMaQR("BEGIN:VCARD\nFN:Trần Bình\nEND:VCARD");
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.ho_ten).toBe("Trần Bình");
    expect(doc.to_chuc).toBe("");
    expect(doc.chuc_danh).toBe("");
    expect(doc.dien_thoai).toEqual([]);
    expect(doc.email).toEqual([]);
    expect(doc.trang_web).toEqual([]);
    expect(thiepCoNoiDung(doc)).toBe(true);
  });

  it("KHÔNG CÓ `FN` thì dựng tên từ `N`, theo THỨ TỰ TIẾNG VIỆT", () => {
    // `N:Họ;Tên;Đệm;Tiền tố;Hậu tố`. Xếp theo lối Anh ngữ ("An Nguyễn") là gọi sai tên một người
    // ngay ở dòng to nhất màn hình.
    const doc = docMaQR("BEGIN:VCARD\nN:Nguyễn;An;Văn;;\nEND:VCARD");
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.ho_ten).toBe("Nguyễn Văn An");
  });

  it("NHIỀU số điện thoại và email đều được giữ, không chỉ cái đầu", () => {
    const doc = docMaQR(
      [
        "BEGIN:VCARD",
        "TEL;TYPE=WORK:0283 1010 000",
        "TEL;TYPE=CELL:+84901234000",
        "EMAIL:a@vidu.vn",
        "EMAIL:b@vidu.vn",
        "END:VCARD",
      ].join("\n"),
    );
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.dien_thoai).toHaveLength(2);
    expect(doc.email).toEqual(["a@vidu.vn", "b@vidu.vn"]);
  });

  it("KÝ TỰ THOÁT được gỡ: một tên công ty có dấu phẩy không hiện ra dấu gạch chéo", () => {
    const doc = docMaQR("BEGIN:VCARD\nORG:Vihat\\, Chi nhánh Hà Nội\nEND:VCARD");
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(doc.to_chuc).toBe("Vihat, Chi nhánh Hà Nội");
  });

  it("vCard RỖNG RUỘT được nhận ra, để màn hình nói ra thay vì vẽ một tấm thiếp trắng", () => {
    const doc = docMaQR("BEGIN:VCARD\nVERSION:3.0\nEND:VCARD");
    if (doc.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(thiepCoNoiDung(doc)).toBe(false);
  });

  it("URL thuần là một LIÊN KẾT, không phải văn bản", () => {
    expect(docMaQR("https://vihatsoftware.com")).toEqual({
      loai: "lien-ket",
      duong_dan: "https://vihatsoftware.com",
    });
    expect(docMaQR("HTTP://vidu.vn/a?b=1")).toEqual({
      loai: "lien-ket",
      duong_dan: "HTTP://vidu.vn/a?b=1",
    });
  });

  it("chuỗi RỖNG và chuỗi chỉ có khoảng trắng đều là `rong`", () => {
    expect(docMaQR("")).toEqual({ loai: "rong" });
    expect(docMaQR("   \n\t ")).toEqual({ loai: "rong" });
  });

  it("RÁC KHÔNG PHẢI vCard vẫn được hiện NGUYÊN VĂN, không bị giấu đi", () => {
    // Giấu một mã vì "không đúng định dạng" là biến một tính năng thành một cánh cửa đóng: người
    // vừa quét cần biết mã ấy ghi gì.
    expect(docMaQR("MECARD:N:Ai đó;;")).toEqual({ loai: "van-ban", noi_dung: "MECARD:N:Ai đó;;" });
    expect(docMaQR("ABC-123-XYZ")).toEqual({ loai: "van-ban", noi_dung: "ABC-123-XYZ" });
    // Một chuỗi có `BEGIN:VCARD` ở GIỮA thì không phải vCard: dấu hiệu ấy phải ở đầu.
    expect(docMaQR("ghi chú BEGIN:VCARD")).toEqual({
      loai: "van-ban",
      noi_dung: "ghi chú BEGIN:VCARD",
    });
  });

  it("`BEGIN:VCARD` viết thường vẫn nhận ra", () => {
    expect(docMaQR("begin:vcard\nFN:Lê Cường\nend:vcard").loai).toBe("danh-thiep");
  });
});

/* ============================================================================================
   BA TÍNH NĂNG — mỗi cái tự giải thích được cho người duyệt.
   ============================================================================================ */

describe("mỗi tính năng tự giải thích được cho người duyệt", () => {
  it("có đúng ba tính năng, và không cái nào thiếu một câu nào", () => {
    // Một trường rỗng ở đây là một màn hình có nút mà không có lý do — đúng thứ chính sách Mini
    // App (điều 3.3.4) từ chối xét duyệt.
    expect(NOI_DUNG_TINH_NANG).toHaveLength(3);
    for (const nd of NOI_DUNG_TINH_NANG) {
      for (const [khoa, gia_tri] of Object.entries(nd)) {
        expect(gia_tri.trim().length, `${nd.ma}: trường "${khoa}" rỗng`).toBeGreaterThan(0);
      }
    }
  });

  it("không câu nào nhắc tới ngữ cảnh cơ quan nhà nước", () => {
    // Ứng dụng này là sản phẩm của một doanh nghiệp công nghệ. Bản dựng được quét ở
    // `bundle-for-zalo.test.ts`; ca này bắt sớm hơn một tầng, ngay tại tệp nội dung.
    const tat_ca = [
      ...NOI_DUNG_TINH_NANG.flatMap((nd) => Object.values(nd)),
      ...Object.values(DANH_THIEP),
      ...Object.values(VAN_PHONG),
      ...Object.values(TU_VAN),
      ...Object.values(TOKEN_KHONG_CHUA_GI),
      CHI_HIEN_LEN_MAN_HINH,
      MA_RONG,
    ].join(" ");
    for (const tu of ["cơ quan", "công dân", "chính quyền", "hành chính", "thủ tục"]) {
      expect(tat_ca, `nội dung ba tính năng còn chữ "${tu}"`).not.toContain(tu);
    }
  });

  for (const ma of ["danh-thiep", "van-phong", "tu-van"] as const) {
    it(`${ma}: nói VÌ SAO cần quyền, có nút, và trạng thái đọc được`, () => {
      const nd = noiDung(ma);
      const markup = ve(
        <KhungTinhNang
          ma={ma}
          trang_thai={{ kieu: "chua-goi" }}
          onBam={() => {}}
          veKetQua={() => null}
        />,
      );
      const chu = textOf(markup);

      expect(chu, "màn không nói vì sao cần quyền này").toContain(nd.vi_sao);
      expect(chu).toContain(nd.tieu_de);
      expect(chu, "không thấy nhãn nút").toContain(nd.nut);
      expect(markup).toContain('role="status"');
      expect(markup, "nút bị khoá ngay từ đầu").not.toContain("disabled");
      expect(chu).toContain(CHI_HIEN_LEN_MAN_HINH);
    });
  }

  it("cấp tiêu đề đi theo chỗ đặt: tab riêng dùng h1, khối trong màn Liên hệ dùng h2", () => {
    // Một màn hình chỉ được có MỘT `<h1>`. Màn Liên hệ đã có `<h1>Liên hệ</h1>`, nên hai tính
    // năng ở đó phải là `<h2>` — nếu không, trình đọc màn hình nghe ba tiêu đề bậc nhất.
    expect(ve(<ManDanhThiep />)).toContain("<h1");
    expect(ve(<TimVanPhong />)).not.toContain("<h1");
    expect(ve(<DangKyTuVan />)).not.toContain("<h1");
  });

  it("không mở ô nhập nào — ba tính năng lấy dữ liệu từ nền tảng, không hỏi người dùng gõ", () => {
    for (const phan_tu of [<ManDanhThiep />, <TimVanPhong />, <DangKyTuVan />]) {
      expect(ve(phan_tu)).not.toMatch(/<(form|input|textarea|select)[\s/>]/);
    }
  });
});

describe("từ chối là đường đi bình thường, không phải lỗi", () => {
  for (const nd of NOI_DUNG_TINH_NANG) {
    const khung = (kieu: "tu-choi" | "ngoai-zalo" | "khong-lay-duoc") =>
      textOf(
        ve(
          <KhungTinhNang
            ma={nd.ma as MaTinhNang}
            trang_thai={{ kieu }}
            onBam={() => {}}
            veKetQua={() => null}
          />,
        ),
      );

    it(`${nd.ma}: nói họ đã từ chối và vẫn dùng được, bằng tiếng Việt`, () => {
      expect(khung("tu-choi")).toContain(nd.tu_choi);
      expect(nd.tu_choi).not.toMatch(/-?\d{3}|[Ee]rror|code|SDK/);
    });

    it(`${nd.ma}: ngoài Zalo thì NÓI RA, không để màn trắng`, () => {
      expect(khung("ngoai-zalo")).toContain(nd.ngoai_zalo);
      expect(nd.ngoai_zalo).toMatch(/Zalo/);
    });

    it(`${nd.ma}: hỏng vì lý do khác thì vẫn nói việc cần làm tiếp`, () => {
      expect(khung("khong-lay-duoc")).toContain(nd.khong_lay_duoc);
      expect(nd.khong_lay_duoc).not.toMatch(/-?\d{3}|[Ee]rror|code|SDK/);
    });
  }

  it("đang chờ thì khoá nút và NÓI bằng chữ, không chỉ bằng màu", () => {
    const markup = ve(
      <KhungTinhNang
        ma="danh-thiep"
        trang_thai={{ kieu: "dang-cho" }}
        onBam={() => {}}
        veKetQua={() => null}
      />,
    );
    expect(markup).toContain("disabled");
    expect(markup).toContain('aria-busy="true"');
    expect(textOf(markup)).toContain(noiDung("danh-thiep").dang_cho);
  });
});

describe("hai tính năng dùng token: màn hình không có dữ liệu cá nhân để mà che", () => {
  const TOKEN = "AbCdEf0123456789xyz";

  for (const [ma, da_nhan, noi_them] of [
    ["tu-van", TU_VAN.da_nhan_ma, TU_VAN.chua_gui_di],
    ["van-phong", VAN_PHONG.da_nhan_ma, VAN_PHONG.chua_xep_duoc],
  ] as const) {
    it(`${ma}: hiện độ dài và vài ký tự đầu, KHÔNG hiện trọn token`, () => {
      const markup = ve(
        <KetQuaToken ma={ma} token={TOKEN} da_nhan={da_nhan} noi_them={noi_them} />,
      );
      const chu = textOf(markup);

      expect(chu).toContain(String(TOKEN.length));
      expect(chu).toContain(cheToken(TOKEN));
      // Token sống 2 phút và đổi được dữ liệu ở máy chủ. Hiện trọn vẹn là mời người đứng cạnh
      // chụp lại trong hai phút ấy.
      expect(markup, "token hiện trọn vẹn trên màn hình").not.toContain(TOKEN);
    });

    it(`${ma}: nói rõ dữ liệu thật KHÔNG nằm trong token, VÀ nói ra thứ bản này chưa làm`, () => {
      const chu = textOf(
        ve(<KetQuaToken ma={ma} token={TOKEN} da_nhan={da_nhan} noi_them={noi_them} />),
      );
      expect(chu).toContain(TOKEN_KHONG_CHUA_GI[ma]);
      expect(TOKEN_KHONG_CHUA_GI[ma]).toMatch(/không nằm trong mã này/);
      expect(TOKEN_KHONG_CHUA_GI[ma]).toMatch(/máy chủ/);
      // Vế thứ hai: ranh giới. Bỏ nó đi là hứa một việc bản dựng này không làm.
      expect(chu).toContain(noi_them);
    });
  }

  it("token rỗng là câu trả lời thật của môi trường phát triển — nói ra, không để ô trống", () => {
    expect(
      textOf(ve(<KetQuaToken ma="van-phong" token="" da_nhan="x" noi_them="y" />)),
    ).toContain(MA_RONG);
  });

  it("che token chỉ để vài ký tự đầu, và không rò gì khi token quá ngắn", () => {
    expect(cheToken("AbCdEf0123456789")).toBe("AbCdEf…");
    expect(cheToken("abc")).toBe("…");
    expect(cheToken("")).toBe("…");
  });
});

describe("tấm thiếp quét được: một thẻ có cấu trúc, không phải một cục chữ thô", () => {
  const thiep = docMaQR(VCARD_DU);
  if (thiep.loai !== "danh-thiep") throw new Error("mẫu vCard hỏng");

  const markup = ve(<TheDanhThiep thiep={thiep} onGoi={() => {}} onMoLienKet={() => {}} />);

  it("hiện đủ tên, công ty, chức danh — mỗi thứ dưới nhãn của nó", () => {
    const chu = textOf(markup);
    for (const nhan of [DANH_THIEP.nhan_ho_ten, DANH_THIEP.nhan_to_chuc, DANH_THIEP.nhan_chuc_danh]) {
      expect(chu, `thiếu nhãn: ${nhan}`).toContain(nhan);
    }
    expect(chu).toContain(thiep.ho_ten);
    expect(chu).toContain(thiep.to_chuc);
    expect(chu).toContain(thiep.chuc_danh);
  });

  it("mỗi cách liên hệ có đúng một nút hành động của nó", () => {
    const chu = textOf(markup);
    expect(chu).toContain(DANH_THIEP.nut_goi);
    expect(chu).toContain(DANH_THIEP.nut_email);
    expect(chu).toContain(DANH_THIEP.nut_mo_lien_ket);
    // Email đi qua một thẻ `mailto:` — ý định cục bộ, giao cho ứng dụng thư của người dùng.
    expect(markup).toContain(`href="mailto:${thiep.email[0]}"`);
    // Số điện thoại KHÔNG có `tel:`: nó đi qua `openPhone` của nền tảng.
    expect(markup, "số điện thoại lọt vào một URL").not.toContain("tel:");
  });

  it("thiếu trường nào thì không vẽ dòng ấy — không có nhãn nào đứng trống", () => {
    const it_chu = docMaQR("BEGIN:VCARD\nFN:Trần Bình\nEND:VCARD");
    if (it_chu.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    const chu = textOf(ve(<TheDanhThiep thiep={it_chu} onGoi={() => {}} onMoLienKet={() => {}} />));
    expect(chu).toContain("Trần Bình");
    expect(chu).not.toContain(DANH_THIEP.nhan_to_chuc);
    expect(chu).not.toContain(DANH_THIEP.nut_goi);
  });

  it("thiếp rỗng ruột thì nói ra bằng một câu, không vẽ một thẻ trắng", () => {
    const rong = docMaQR("BEGIN:VCARD\nVERSION:3.0\nEND:VCARD");
    if (rong.loai !== "danh-thiep") throw new Error("phải là danh thiếp");
    expect(textOf(ve(<TheDanhThiep thiep={rong} onGoi={() => {}} onMoLienKet={() => {}} />))).toContain(
      DANH_THIEP.thiep_rong,
    );
  });
});

describe("mã không phải danh thiếp vẫn dùng được", () => {
  const ve_qr = (noi_dung_qr: string) =>
    textOf(ve(<KetQuaQuet noi_dung_qr={noi_dung_qr} onGoi={() => {}} onMoLienKet={() => {}} />));

  it("liên kết: nói rõ đây là liên kết, hiện nguyên văn, và có nút mở", () => {
    const chu = ve_qr("https://vihatsoftware.com");
    expect(chu).toContain(DANH_THIEP.tieu_de_lien_ket);
    expect(chu).toContain("https://vihatsoftware.com");
    expect(chu).toContain(DANH_THIEP.nut_mo_lien_ket);
  });

  it("văn bản thuần: hiện NGUYÊN VĂN, có nhãn nói rõ đó là nguyên văn", () => {
    const chu = ve_qr("ABC-123-XYZ");
    expect(chu).toContain(DANH_THIEP.tieu_de_van_ban);
    expect(chu).toContain(DANH_THIEP.nguyen_van);
    expect(chu).toContain("ABC-123-XYZ");
  });

  it("mã rỗng: nói ra, không hiện một ô trống", () => {
    expect(ve_qr("")).toContain(DANH_THIEP.ma_rong);
  });

  it("thiếp quét được kèm một câu nhắc về dữ liệu cá nhân của NGƯỜI KHÁC", () => {
    expect(ve_qr(VCARD_DU)).toContain(DANH_THIEP.rieng_tu);
  });
});

describe("tìm văn phòng: từ chối quyền KHÔNG được làm mất tính năng", () => {
  it("ba văn phòng và nút chỉ đường hiện ngay, trước khi bấm gì", () => {
    const chu = textOf(ve(<TimVanPhong />));
    for (const van_phong of OFFICES) {
      expect(chu, `thiếu văn phòng: ${van_phong.name}`).toContain(van_phong.name);
      expect(chu, `thiếu địa chỉ: ${van_phong.name}`).toContain(van_phong.address);
    }
    expect((ve(<DanhSachVanPhong onChiDuong={() => {}} />).match(/<button/g) ?? []).length).toBe(
      OFFICES.length,
    );
  });

  it("người từ chối vẫn thấy đủ ba văn phòng và nút chỉ đường", () => {
    const chu = textOf(
      ve(
        <KhungTinhNang
          ma="van-phong"
          trang_thai={{ kieu: "tu-choi" }}
          onBam={() => {}}
          veKetQua={() => null}
          duoi_cung={<DanhSachVanPhong onChiDuong={() => {}} />}
        />,
      ),
    );
    expect(chu).toContain(noiDung("van-phong").tu_choi);
    for (const van_phong of OFFICES) expect(chu).toContain(van_phong.address);
    expect(chu).toContain(VAN_PHONG.nut_chi_duong);
  });

  it("đường chỉ đường chỉ mang ĐỊA CHỈ VĂN PHÒNG, không mang gì của người dùng", () => {
    // Ứng dụng không có toạ độ của người dùng (chỉ có token), nên không có gì để mang theo — và
    // ca này là thứ giữ cho điều đó đúng nếu sau này ai đó có toạ độ trong tay.
    const duong = duongDanBanDo(OFFICES[0]!.address);
    expect(duong).toContain(encodeURIComponent(OFFICES[0]!.address));
    expect(duong).not.toMatch(/token|lat|lng|latitude|longitude/i);
  });

  it("câu nói ra ranh giới: chưa xếp được theo khoảng cách, và nói vì sao", () => {
    // Viết "đang tìm văn phòng gần bạn…" rồi hiện danh sách theo thứ tự cũ là nói dối bằng giao
    // diện. `getLocation` chỉ trả token, và đổi token cần một bước máy chủ.
    expect(VAN_PHONG.chua_xep_duoc).toMatch(/máy chủ/);
    expect(VAN_PHONG.chua_xep_duoc).toMatch(/chưa/);
  });
});

describe("đăng ký tư vấn: luôn có một đường liên hệ chạy được NGAY", () => {
  const markup = ve(<DangKyTuVan />);

  it("hotline và email nằm ngay dưới nút đăng ký, trước khi bấm gì", () => {
    expect(markup).toContain("href=\"tel:");
    expect(markup).toContain("href=\"mailto:");
    expect(textOf(markup)).toContain(TU_VAN.nhac_lien_he);
  });

  it("câu nói ra ranh giới: bản này CHƯA gửi yêu cầu đi đâu", () => {
    expect(TU_VAN.chua_gui_di).toMatch(/chưa gửi/);
    expect(textOf(ve(<KetQuaToken ma="tu-van" token="abcdefgh" da_nhan={TU_VAN.da_nhan_ma} noi_them={TU_VAN.chua_gui_di} />))).toContain(
      TU_VAN.chua_gui_di,
    );
  });
});

describe("tab Danh thiếp dẫn tới đúng tính năng ấy", () => {
  it("cửa `features/tinh-nang` trỏ vào chính màn quét, và tab có nhãn đọc được", () => {
    expect(MAN_DANH_THIEP.component).toBe(ManDanhThiep);
    expect(MAN_DANH_THIEP.tabLabel.trim().length).toBeGreaterThan(0);
    expect(MAN_DANH_THIEP.headerTitle.trim().length).toBeGreaterThan(0);
  });
});
