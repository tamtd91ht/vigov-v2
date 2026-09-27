/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { App, KhungApp } from "../../App";
import { NHAN_KENH_CONG_DAN } from "../../cong-dan/man/KenhCongDan";
import { XAC_NHAN_XA } from "../../cong-dan/man/noi-dung";
import { COMPANY } from "../../content/company-profile";
import { SCREENS } from "../company-intro/screens";
import { GoiYXaScreen } from "./GoiYXaScreen";
import { nhanNguon, phanGiaiGoiY, TIEU_DE_XAC_NHAN_XA, type XaGoiY } from "./goi-y";

/**
 * VÌ SAO LỚP KHÁM PHÁ ĐƯỢC KIỂM, TRONG KHI NGUỒN XÃ PHÍA MÁY CHỦ CHƯA ĐƯỢC NỐI VÀO:
 *
 *   Vì đúng những quy tắc ở đây là thứ hỏng trong im lặng. Một dòng `?? "qr"` biến một liên kết
 *   chuyển tay thành một xã được chọn sẵn; một tên xã dựng từ tham số khi máy chủ chưa trả lời
 *   biến một hồ sơ thành hồ sơ gửi sang cơ quan khác. Cả hai đều dựng xanh, chạy mượt, và không
 *   có màn hình nào trông sai.
 *
 *   Kênh công dân của dự án trước được đo là **không có test nào**. Đây là chỗ con số ấy khác đi.
 *
 * ⚠ 27/09/2026 — DANH MỤC XÃ MẪU, BỘ CHỌN XÃ, TRANG XÃ MẪU VÀ NÚT "ĐỔI XÃ" ĐÃ BỊ XOÁ (ADR 0044 câu 4
 * · ADR 0047, quyết định của chủ sản phẩm). Các ca kiểm danh mục, bộ chọn và trang xã đi theo chúng.
 * Các ca ở dưới ghim điều thay vào: tên xã có đúng một nguồn (máy chủ), và hôm nay nguồn ấy chưa có
 * nên KHÔNG tên xã nào hiện ra.
 */

const render = (element: Parameters<typeof renderToStaticMarkup>[0]) =>
  renderToStaticMarkup(element);

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, " ")
    .replace(/\s+/g, " ");

/** Xã GIẢ của phép kiểm, như thể máy chủ vừa trả về. Không ứng với đơn vị hành chính nào. */
const XA_THU: XaGoiY = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Ví Dụ" };

const dauTrang = (markup: string) => textOf(markup.slice(0, markup.indexOf("</header>")));

/** Dựng `App` như thể nó vừa được mở bằng một chuỗi truy vấn — cùng cách `chien-dich.test.tsx`. */
function moVoi<T>(chuoi_truy_van: string, lam: () => T): T {
  const cua_so = globalThis as unknown as { window?: unknown };
  const truoc = cua_so.window;
  cua_so.window = { location: { search: chuoi_truy_van } };
  try {
    return lam();
  } finally {
    if (truoc === undefined) delete cua_so.window;
    else cua_so.window = truoc;
  }
}

// ---------------------------------------------------------------------------------------------

/**
 * DÂY BẪY: KHÔNG TÊN XÃ NÀO VIẾT THẲNG TRONG MÃ SẢN PHẨM.
 *
 * Trước 27/09 dây bẫy này miễn cho đúng MỘT tệp — danh mục xã mẫu. Tệp ấy đã bị xoá, nên nay nó
 * không miễn cho tệp nào: tên xã chỉ được đến từ máy chủ. Một tên xã viết thẳng vào mã là một cơ
 * quan nhà nước hiển thị một đơn vị hành chính mà không ai kiểm được nó có thật hay không.
 */
describe("không tệp sản phẩm nào mang một tên đơn vị hành chính", () => {
  const RAW = import.meta.glob("../../**/*.{ts,tsx}", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  /** Bỏ chú thích: nhiều tệp GIẢI THÍCH bằng lời rằng chúng không chứa tên xã nào. */
  const khongChuThich = (ma: string) =>
    ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

  // Vite rút gọn khoá của `import.meta.glob` theo vị trí TỆP NÀY: tệp cùng thư mục thành
  // "./x.ts", tệp ngoài thành "../../x.ts".
  const TRONG_KHAM_PHA = (duong: string) => duong.startsWith("./");

  const SAN_PHAM = Object.entries(RAW)
    .filter(([duong]) => !duong.includes(".test."))
    .map(([duong, ma]) => ({ duong, ma: khongChuThich(ma) }));

  /** Một tên đơn vị hành chính là một chuỗi BẮT ĐẦU bằng loại đơn vị — đó là hình dạng của nó. */
  const LA_TEN_DON_VI = /^(Xã|Phường|Đặc khu|Thị trấn)\s+\S/u;

  it("quét đúng cây mã thật — một lượt quét rỗng cũng xanh, và xanh sai lý do", () => {
    const duong = SAN_PHAM.map((t) => t.duong);
    expect(duong).toContain("../../App.tsx");
    expect(duong.filter(TRONG_KHAM_PHA).length).toBeGreaterThanOrEqual(3);
    expect(duong.length).toBeGreaterThanOrEqual(10);
    // Danh mục mẫu đã bị xoá. Nó quay lại là quay lại một nguồn tên xã thứ hai.
    expect(duong.filter((d) => d.endsWith("/demo-danh-muc-xa.ts"))).toEqual([]);
  });

  it("không chuỗi nào trong mã sản phẩm là tên đơn vị hành chính", () => {
    const viPham: string[] = [];
    for (const tep of SAN_PHAM) {
      for (const khop of tep.ma.matchAll(/"([^"\\\n]*)"|'([^'\\\n]*)'/g)) {
        const chuoi = khop[1] ?? khop[2] ?? "";
        if (LA_TEN_DON_VI.test(chuoi)) viPham.push(`${tep.duong}: ${chuoi}`);
      }
    }
    expect(
      viPham,
      "một tên đơn vị hành chính được viết thẳng vào mã. Tên xã chỉ có một nguồn: máy chủ.",
    ).toEqual([]);
  });

  it("trong thư mục khám phá thì kể cả chữ trong JSX cũng không được mang tên xã", () => {
    // Phép kiểm trên chỉ thấy chuỗi trong dấu nháy. `<p>Xã An Thịnh</p>` không phải chuỗi — nên
    // riêng thư mục này, nơi rủi ro nằm, quét thẳng toàn văn.
    const viPham = SAN_PHAM.filter(
      (tep) => TRONG_KHAM_PHA(tep.duong) && /(Xã|Phường|Đặc khu|Thị trấn)\s+\p{Lu}/u.test(tep.ma),
    ).map((tep) => tep.duong);
    expect(viPham).toEqual([]);
  });

  it("không còn lối chọn xã hay đổi xã nào trong mã sản phẩm (ADR 0044 câu 4 · ADR 0047)", () => {
    // Một phiên, một xã. Một nút "Chọn xã khác" hay "Đổi xã" quay lại là quay lại một đường để
    // công dân rời xã của phiên mà máy chủ không hề biết.
    const CAM = /Chọn xã khác|Đổi xã|Chọn xã để tiếp tục|ChonXaScreen|onDoiXa|onChonXaKhac/;
    const viPham = SAN_PHAM.filter((tep) => CAM.test(tep.ma)).map((tep) => tep.duong);
    expect(viPham).toEqual([]);
  });
});

// ---------------------------------------------------------------------------------------------

describe("mức tin theo nguồn — tham số gợi ý, không bao giờ quyết", () => {
  it("máy chủ chưa tra ra xã thì KHÔNG gợi ý gì, với mọi nguồn", () => {
    // Máy chủ trả rỗng / lỗi thì `cong-dan/man/XacNhanXa.tsx` truyền `null`. Một nguồn "đủ tin"
    // không được phép bù cho một tên xã không có.
    for (const nguon of ["qr", "zns", "share", "", "QR"]) {
      expect(phanGiaiGoiY(nguon, null)).toEqual({ kieu: "khong-co" });
    }
  });

  it("quét QR tại trụ sở xã, máy chủ đã tra ra xã → chọn sẵn, một chạm xác nhận", () => {
    const ra = phanGiaiGoiY("qr", XA_THU);
    expect(ra.kieu).toBe("chon-san");
    if (ra.kieu === "chon-san") expect(ra.xa).toEqual(XA_THU);
  });

  it("liên kết do xã gửi (zns) thì cũng chọn sẵn", () => {
    expect(phanGiaiGoiY("zns", XA_THU).kieu).toBe("chon-san");
  });

  it("liên kết chuyển tay, nguồn lạ, hoặc không khai nguồn → không gợi ý, không có mặc định 'coi như qr'", () => {
    // Luật 1, cấm #1: một mặc định trên đường cô lập. Đúng một dòng `?? "qr"` là đủ để mọi liên
    // kết không rõ nguồn trở thành một xã chọn sẵn, và không test nào khác đỏ.
    for (const nguon of ["share", "", "email", "QR", "constructor"]) {
      expect(phanGiaiGoiY(nguon, XA_THU), `nguồn "${nguon}"`).toEqual({ kieu: "khong-co" });
    }
  });

  it("máy chủ trả một tên rỗng thì cũng không gợi ý — một màn xác nhận không có tên là vô nghĩa", () => {
    expect(phanGiaiGoiY("qr", { ten: "  ", tinh: "Tỉnh Ví Dụ" })).toEqual({ kieu: "khong-co" });
  });

  it("nói nguồn bằng tiếng Việt của người dân, không bằng khoá kỹ thuật", () => {
    for (const nguon of ["qr", "zns", "share", ""]) {
      const nhan = nhanNguon(nguon);
      // "QR" được giữ lại: đó là từ người dân dùng hằng ngày, không phải khoá nội bộ.
      expect(nhan).not.toMatch(/\b(zns|share|src|tenant)\b/i);
      expect(nhan.length).toBeGreaterThan(10);
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("màn xác nhận xã", () => {
  const markup = render(
    <GoiYXaScreen xa={XA_THU} nguon="qr" onXacNhan={() => {}} onKhongPhai={() => {}} />,
  );

  it("hiện TÊN xã và tỉnh/thành", () => {
    const chu = textOf(markup);
    expect(chu).toContain(XA_THU.ten);
    expect(chu).toContain(XA_THU.tinh);
  });

  it("nói ra vì sao xã này được chọn sẵn, bằng chữ chứ không bằng màu", () => {
    expect(textOf(markup)).toContain(nhanNguon("qr"));
  });

  it("cho đúng hai đường đi: xác nhận, hoặc 'không phải xã này' — không có 'chọn xã khác'", () => {
    const nut = markup.match(/<button\b/g) ?? [];
    expect(nut).toHaveLength(2);
    expect(textOf(markup)).toContain("Đúng, tiếp tục");
    expect(textOf(markup)).toContain("Không phải xã này");
    expect(textOf(markup)).not.toMatch(/chọn xã khác|đổi sang xã khác/i);
  });

  it("hứa trước rằng tên xã sẽ theo suốt mọi màn hình", () => {
    expect(textOf(markup)).toMatch(/mọi màn hình|mỗi màn hình/);
  });

  it("mở không một ô nhập liệu nào", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });

  it("cho một tiêu đề cấp một, và giấu mọi hình vẽ khỏi trình đọc màn hình", () => {
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) {
      expect(svg).toContain('aria-hidden="true"');
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("bất di dịch #2 — xác nhận xã rồi thì tên xã hiện trên MỌI màn hình", () => {
  it("in tên xã ở đầu từng màn hình một, không sót màn nào", () => {
    // Đây là bất biến đắt nhất của kênh công dân: công dân không thấy mình đang gửi cho xã nào
    // thì gửi nhầm, xã nhận việc ngoài địa bàn, phải chuyển hoặc từ chối, và người dân chờ vô ích.
    for (const man of SCREENS) {
      const markup = render(
        <KhungApp man={man.id} onChonMan={() => {}} xaDaChon={XA_THU}>
          <p>nội dung</p>
        </KhungApp>,
      );
      expect(dauTrang(markup), `màn ${man.id} không hiện tên xã ở header`).toContain(XA_THU.ten);
      expect(dauTrang(markup)).toContain(man.headerTitle);
    }
  });

  it("header KHÔNG có nút đổi xã — một phiên, một xã", () => {
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={XA_THU}>
        <p>nội dung</p>
      </KhungApp>,
    );
    const header = markup.slice(0, markup.indexOf("</header>"));
    expect(header).not.toMatch(/<button\b/);
    expect(textOf(header)).not.toMatch(/đổi xã/i);
  });

  it("chưa xác nhận xã thì ô ấy giữ tên đơn vị phát hành", () => {
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null}>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(dauTrang(markup)).toContain(COMPANY.name);
  });

  it("giấu thanh tab khi đang xác nhận xã — mỗi màn một việc", () => {
    // `skills/accessibility-elderly` #4. Một thanh tab còn đó trong lúc chưa có xã là bốn nút
    // bấm vào không có gì xảy ra, và đó là lúc người lớn tuổi kết luận app hỏng.
    const dangXacNhan = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null} khamPha>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(dangXacNhan).not.toContain("tabbar");
    expect(dauTrang(dangXacNhan)).toContain(TIEU_DE_XAC_NHAN_XA);

    const binhThuong = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={XA_THU}>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(binhThuong).toContain("tabbar");
  });

  it("đang gợi ý thì header VẪN CHƯA mang tên xã — tham số không chọn thay công dân", () => {
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null} khamPha>
        <GoiYXaScreen xa={XA_THU} nguon="qr" onXacNhan={() => {}} onKhongPhai={() => {}} />
      </KhungApp>,
    );
    expect(dauTrang(markup)).toContain(COMPANY.name);
    expect(dauTrang(markup)).not.toContain(XA_THU.ten);
    // Nhưng thân màn hình thì có, vì đó chính là thứ đang chờ xác nhận.
    expect(textOf(markup)).toContain(XA_THU.ten);
  });
});

// ---------------------------------------------------------------------------------------------

/**
 * ĐƯỜNG LIÊN KẾT KHÔNG ĐỦ ĐỂ HỎI MÁY CHỦ → KHÔNG TÊN XÃ NÀO, VÀ APP MỞ PHẦN GIỚI THIỆU. Ca này dựng
 * chính `App`, không dựng một mảnh của nó: ai đó "nối tạm" một tên xã dựng từ tham số QR vào `App.tsx`
 * thì đây là ca đỏ lên.
 *
 * 27/09/2026: tham số tên miền là `d` (ADR 0047 §Trả lời). `t` và `v` đã bỏ — chúng vẫn nằm trong
 * danh sách dưới để ghim rằng chúng KHÔNG mở gì. `d` không kèm `src` tin được, hoặc sai khuôn, cũng thế.
 */
describe("app mở bằng một liên kết không đủ tin — fail closed, không hỏi máy chủ", () => {
  const MO = [
    "",
    "?src=qr&t=xa-vi-du.vigov.example",
    "?src=zns&t=xa-vi-du.vigov.example",
    "?src=qr&t=01JDEMXA00000000000000000A",
    "?src=qr&v=2",
    "?d=xa-vi-du.vigov.example",
    "?src=share&d=xa-vi-du.vigov.example",
    "?src=QR&d=xa-vi-du.vigov.example",
    "?src=qr&d=https://xa-vi-du.vigov.example",
    "?src=qr&d=xa%20vi%20du",
  ];

  it("không hiện màn xác nhận, không hiện tên xã nào, không hiện lối vào kênh công dân", () => {
    for (const chuoi of MO) {
      const markup = moVoi(chuoi, () => render(<App />));
      const chu = textOf(markup);
      expect(chu, `mở bằng "${chuoi}"`).not.toContain("Bạn cần liên hệ với xã này?");
      expect(chu, `mở bằng "${chuoi}"`).not.toContain(TIEU_DE_XAC_NHAN_XA);
      expect(chu, `mở bằng "${chuoi}"`).not.toContain(NHAN_KENH_CONG_DAN);
      // Tên miền trên QR không bao giờ được vẽ ra như thể nó là tên xã.
      expect(chu, `mở bằng "${chuoi}"`).not.toContain("xa-vi-du");
      expect(dauTrang(markup), `mở bằng "${chuoi}"`).toContain(COMPANY.name);
    }
  });

  it("mở phần giới thiệu, có thanh tab — không ngõ cụt", () => {
    for (const chuoi of MO) {
      const markup = moVoi(chuoi, () => render(<App />));
      expect(markup, `mở bằng "${chuoi}"`).toContain("tabbar");
    }
  });
});

/**
 * ĐƯỜNG LIÊN KẾT ĐỦ TIN (`d` đúng khuôn + `src` qr/zns) → LỚP KHÁM PHÁ MỞ Ở BƯỚC "ĐANG TÌM XÃ".
 *
 * `renderToStaticMarkup` không chạy hiệu ứng, nên lượt dựng đầu tiên là đúng thứ người dân thấy trong
 * lúc app hỏi máy chủ: một câu chờ, KHÔNG một tên xã nào, KHÔNG tên miền nào, không thanh tab. Tên xã
 * chỉ hiện khi máy chủ trả lời — `cong-dan/cong-khai.test.tsx` kiểm bước ấy.
 */
describe("app mở bằng QR có `d` — hỏi máy chủ, không tự dựng tên xã", () => {
  for (const chuoi of ["?src=qr&d=xa-vi-du.vigov.example", "?d=xa-vi-du.vigov.example&src=zns"]) {
    it(`mở bằng "${chuoi}": câu chờ, header 'Xác nhận xã', không tab, không tên xã, không tên miền`, () => {
      const markup = moVoi(chuoi, () => render(<App />));
      const chu = textOf(markup);
      expect(chu).toContain(XAC_NHAN_XA.dang_tra);
      expect(dauTrang(markup)).toContain(TIEU_DE_XAC_NHAN_XA);
      expect(dauTrang(markup)).toContain(COMPANY.name);
      expect(markup).not.toContain("tabbar");
      expect(chu).not.toContain("xa-vi-du");
      expect(chu).not.toContain("Bạn cần liên hệ với xã này?");
      expect(chu).not.toContain(NHAN_KENH_CONG_DAN);
    });
  }
});
