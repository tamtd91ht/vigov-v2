import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  cheToken,
  KetQuaQR,
  KetQuaToken,
  KhuQuyen,
  ManQuetQR,
  ManQuyenThuan,
  ManSoDienThoai,
  ManViTri,
} from "./ManQuyen";
import { MAN_QUYEN } from "./index";
import {
  CHI_HIEN_LEN_MAN_HINH,
  DAN_NHAP_QUYEN,
  type MaQuyen,
  MA_RONG,
  noiDung,
  NOI_DUNG_QUYEN,
  QR_RONG,
  TOKEN_KHONG_CHUA_GI,
} from "./noi-dung";

/**
 * KÊNH CÔNG DÂN ĐÃ TỪNG ĐO ĐƯỢC **KHÔNG CÓ TEST NÀO**. Tệp này là phần của lớp quyền.
 *
 * BA ĐIỀU Ở ĐÂY KHÔNG CÓ PHÉP KIỂM NÀO KHÁC NÓI HỘ:
 *
 *   1. **Người dùng từ chối là đường đi bình thường.** Nhánh ấy phải hiện một câu tiếng Việt nói
 *      họ bấm lại được, KHÔNG phải một chuỗi lỗi kỹ thuật (README §Error message shape). Không ai
 *      kiểm thì sáu tuần nữa nó thành `Lỗi: -201`, và người đọc câu ấy là người vừa dùng đúng
 *      quyền của mình để nói không với một cơ quan nhà nước.
 *   2. **Màn hình nói ra rằng số điện thoại và toạ độ KHÔNG tới thiết bị.** Đó là sự thật về nền
 *      tảng (chỉ có `token`, xem `zalo-api.ts`) và là lý do đáng tin nhất để người dân bấm đồng
 *      ý. Mất câu ấy thì màn hình chỉ còn một nút xin dữ liệu, không có lời giải thích nào.
 *   3. **Token không bao giờ hiện trọn vẹn.** Nó sống 2 phút và đổi được dữ liệu ở máy chủ.
 *
 * Bộ test dựng bằng `react-dom/server` — không có DOM để bấm — nên bốn nhánh kết quả đi vào qua
 * `ManQuyenThuan`, bản THUẦN nhận trạng thái bằng tham số. Cùng lối với `DichVuDong` ở trang xã.
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

const BA_MAN: ReadonlyArray<{ ma: MaQuyen; man: () => ReturnType<typeof ManSoDienThoai> }> = [
  { ma: "so-dien-thoai", man: ManSoDienThoai },
  { ma: "vi-tri", man: ManViTri },
  { ma: "quet-qr", man: ManQuetQR },
];

describe("mỗi màn quyền tự giải thích được cho người duyệt", () => {
  it("có đúng ba màn, và không màn nào thiếu một câu nào", () => {
    // Một trường rỗng ở đây là một màn hình có nút mà không có lý do — đúng thứ chính sách Mini
    // App (điều 3.3.4) từ chối xét duyệt.
    expect(NOI_DUNG_QUYEN).toHaveLength(3);
    for (const nd of NOI_DUNG_QUYEN) {
      for (const [khoa, gia_tri] of Object.entries(nd)) {
        expect(gia_tri.trim().length, `${nd.ma}: trường "${khoa}" rỗng`).toBeGreaterThan(0);
      }
    }
  });

  for (const { ma, man: Man } of BA_MAN) {
    it(`${ma}: nói VÌ SAO cần quyền, có đúng một nút, và một tiêu đề bậc nhất`, () => {
      const markup = ve(<Man />);
      const nd = noiDung(ma);
      const chu = textOf(markup);

      expect(chu, "màn không nói vì sao cần quyền này").toContain(nd.vi_sao);
      expect(chu).toContain(nd.tieu_de);
      expect(chu, "không thấy nhãn nút").toContain(nd.nut);

      // Một nút: màn hình nói một việc (`skills/accessibility-elderly` #4).
      expect((markup.match(/<button/g) ?? []).length).toBe(1);
      expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);

      // Lời hứa của chính bản dựng này, trên cả ba màn.
      expect(chu).toContain(CHI_HIEN_LEN_MAN_HINH);
    });

    it(`${ma}: chưa bấm thì chưa có kết quả, và nút chưa bị khoá`, () => {
      const markup = ve(<Man />);
      expect(markup).toContain('role="status"');
      expect(markup, "nút bị khoá ngay từ đầu").not.toContain("disabled");
    });
  }
});

describe("từ chối là đường đi bình thường, không phải lỗi", () => {
  for (const nd of NOI_DUNG_QUYEN) {
    it(`${nd.ma}: nói họ đã từ chối và bấm lại được, bằng tiếng Việt`, () => {
      const chu = textOf(
        ve(
          <ManQuyenThuan
            ma={nd.ma}
            trang_thai={{ kieu: "tu-choi" }}
            onBam={() => {}}
            veKetQua={() => null}
          />,
        ),
      );
      expect(chu).toContain(nd.tu_choi);
      // VIỆC CẦN LÀM BÂY GIỜ, chứ không phải một mã lỗi.
      expect(nd.tu_choi).toMatch(/bấm lại/);
      expect(nd.tu_choi).not.toMatch(/-?\d{3}|[Ee]rror|code|SDK/);
    });

    it(`${nd.ma}: ngoài Zalo thì NÓI RA, không để màn trắng`, () => {
      // `zmp-sdk` hỏng lúc nhập khi không chạy trong Zalo (xem `zalo-api.ts`). Một `catch {}` im
      // lặng ở đó là một màn hình trống trên máy người duyệt.
      const chu = textOf(
        ve(
          <ManQuyenThuan
            ma={nd.ma}
            trang_thai={{ kieu: "ngoai-zalo" }}
            onBam={() => {}}
            veKetQua={() => null}
          />,
        ),
      );
      expect(chu).toContain(nd.ngoai_zalo);
      expect(nd.ngoai_zalo).toMatch(/Zalo/);
    });

    it(`${nd.ma}: hỏng vì lý do khác thì vẫn nói việc cần làm tiếp`, () => {
      const chu = textOf(
        ve(
          <ManQuyenThuan
            ma={nd.ma}
            trang_thai={{ kieu: "khong-lay-duoc" }}
            onBam={() => {}}
            veKetQua={() => null}
          />,
        ),
      );
      expect(chu).toContain(nd.khong_lay_duoc);
      expect(nd.khong_lay_duoc).not.toMatch(/-?\d{3}|[Ee]rror|code|SDK/);
    });
  }

  it("đang chờ thì khoá nút và NÓI bằng chữ, không chỉ bằng màu", () => {
    const markup = ve(
      <ManQuyenThuan
        ma="so-dien-thoai"
        trang_thai={{ kieu: "dang-cho" }}
        onBam={() => {}}
        veKetQua={() => null}
      />,
    );
    expect(markup).toContain("disabled");
    expect(markup).toContain('aria-busy="true"');
    expect(textOf(markup)).toContain(noiDung("so-dien-thoai").dang_cho);
  });
});

describe("hai màn token: màn hình không có dữ liệu cá nhân để mà che", () => {
  const TOKEN = "AbCdEf0123456789xyz";

  for (const ma of ["so-dien-thoai", "vi-tri"] as const) {
    it(`${ma}: hiện độ dài và vài ký tự đầu, KHÔNG hiện trọn token`, () => {
      const markup = ve(<KetQuaToken ma={ma} token={TOKEN} />);
      const chu = textOf(markup);

      expect(chu).toContain(String(TOKEN.length));
      expect(chu).toContain(cheToken(TOKEN));
      // Token sống 2 phút và đổi được dữ liệu ở máy chủ. Hiện trọn vẹn là mời người đứng cạnh
      // chụp lại trong hai phút ấy.
      expect(markup, "token hiện trọn vẹn trên màn hình").not.toContain(TOKEN);
    });

    it(`${ma}: nói rõ dữ liệu thật KHÔNG nằm trong token và không tới thiết bị`, () => {
      const chu = textOf(ve(<KetQuaToken ma={ma} token={TOKEN} />));
      expect(chu).toContain(TOKEN_KHONG_CHUA_GI[ma]);
      expect(TOKEN_KHONG_CHUA_GI[ma]).toMatch(/không nằm trong mã này/);
      expect(TOKEN_KHONG_CHUA_GI[ma]).toMatch(/máy chủ/);
    });
  }

  it("token rỗng là câu trả lời thật của môi trường phát triển — nói ra, không để ô trống", () => {
    expect(textOf(ve(<KetQuaToken ma="vi-tri" token="" />))).toContain(MA_RONG);
  });

  it("che token chỉ để vài ký tự đầu, và không rò gì khi token quá ngắn", () => {
    expect(cheToken("AbCdEf0123456789")).toBe("AbCdEf…");
    expect(cheToken("abc")).toBe("…");
    expect(cheToken("")).toBe("…");
  });
});

describe("màn quét QR: thứ duy nhất trả về dữ liệu thật", () => {
  it("hiện nguyên nội dung quét được — đó là thứ công dân cần đọc", () => {
    const noi_dung_qr = "VIGOV-TRA-CUU-01JCQR0000000000000000";
    expect(textOf(ve(<KetQuaQR noi_dung_qr={noi_dung_qr} />))).toContain(noi_dung_qr);
  });

  it("mã không có nội dung thì nói ra, không hiện một ô trống", () => {
    expect(textOf(ve(<KetQuaQR noi_dung_qr="" />))).toContain(QR_RONG);
  });
});

describe("khu vực quyền: một màn tại một thời điểm", () => {
  const markup = ve(<KhuQuyen />);

  it("dẫn nhập nói trước rằng người dùng bấm thì Zalo mới hỏi", () => {
    expect(textOf(markup)).toContain(DAN_NHAP_QUYEN);
  });

  it("có ba nút chọn màn, và đúng một màn đang được xem", () => {
    for (const nd of NOI_DUNG_QUYEN) {
      expect(textOf(markup), `thiếu nút chọn: ${nd.nhan_chon}`).toContain(nd.nhan_chon);
    }
    // Trạng thái "đang xem" đọc được bằng trình đọc màn hình, không chỉ bằng màu nền.
    expect((markup.match(/aria-pressed="true"/g) ?? []).length).toBe(1);

    // Một `<h1>` — ba màn xếp chồng thì màn hình không còn nói một việc, và trình đọc màn hình
    // nghe thấy ba tiêu đề bậc nhất trên một trang.
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
  });

  it("không mở ô nhập nào — ba màn này lấy dữ liệu từ nền tảng, không hỏi người dùng gõ", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)[\s/>]/);
  });
});

describe("tab quyền dẫn tới đúng khu vực ấy", () => {
  it("cửa `bien-the/quyen` trỏ vào chính KhuQuyen, và tab có nhãn đọc được", () => {
    // `screens.ts` chỉ biết `MAN_QUYEN`. Trỏ sai component thì tab thứ năm dẫn tới một màn khác —
    // và bản nộp xin quyền không còn chỗ nào nhìn thấy được để xin quyền.
    expect(MAN_QUYEN.component).toBe(KhuQuyen);
    expect(MAN_QUYEN.tabLabel.trim().length).toBeGreaterThan(0);
    expect(MAN_QUYEN.headerTitle.trim().length).toBeGreaterThan(0);
  });
});
