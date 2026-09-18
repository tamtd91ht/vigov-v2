/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungApp } from "../../App";
import { COMPANY } from "../../content/company-profile";
import { SCREENS } from "../company-intro/screens";
import { ChonXaScreen } from "./ChonXaScreen";
import { DEMO_DANH_MUC_XA, DEMO_GHI_CHU, DEMO_timTheoMa } from "./demo-danh-muc-xa";
import { GoiYXaScreen } from "./GoiYXaScreen";
import { LOI_NHAN, nhanNguon, phanGiaiGoiY } from "./goi-y";

/**
 * VÌ SAO LỚP KHÁM PHÁ ĐƯỢC KIỂM, TRONG KHI KÊNH CÔNG DÂN CHƯA CÓ MÁY CHỦ NÀO:
 *
 *   Vì đúng những quy tắc ở đây là thứ hỏng trong im lặng. Một dòng `?? "qr"` thêm vào để "cho
 *   demo mượt" biến một liên kết chuyển tay thành một xã được chọn sẵn; một tên xã đoán ra khi
 *   tra mã không thấy biến một hồ sơ thành hồ sơ gửi sang cơ quan khác. Cả hai đều dựng xanh,
 *   chạy mượt, và không có màn hình nào trông sai.
 *
 *   Kênh công dân của dự án trước được đo là **không có test nào**. Đây là chỗ con số ấy khác đi.
 */

const render = (element: Parameters<typeof renderToStaticMarkup>[0]) =>
  renderToStaticMarkup(element);

const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, " ")
    .replace(/\s+/g, " ");

const XA_MOT = DEMO_DANH_MUC_XA[0]!;

// ---------------------------------------------------------------------------------------------

describe("danh mục xã của bản trình diễn", () => {
  it("mang mã ULID mờ, không phải mã hành chính", () => {
    // Luật 1, bất biến 2. Một mã mang nghĩa thì lần sáp nhập đầu tiên buộc viết lại khoá ngoại
    // trên hồ sơ lưu trữ — thứ pháp luật không cho phép. 26 ký tự, bảng chữ Crockford base32
    // (không có I, L, O, U vì chúng lẫn với 1 và 0 khi người ta đọc mã qua điện thoại).
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(xa.id, `${xa.ten}: mã không đủ 26 ký tự`).toHaveLength(26);
      expect(xa.id, `${xa.ten}: mã có ký tự ngoài bảng Crockford base32`).toMatch(
        /^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$/,
      );
    }
    expect(new Set(DEMO_DANH_MUC_XA.map((xa) => xa.id)).size).toBe(DEMO_DANH_MUC_XA.length);
  });

  it("kể đủ ba loại đơn vị hành chính cấp xã, không chỉ 'Xã'", () => {
    // Từ 01/7/2025 chính quyền hai cấp: tỉnh → xã · phường · đặc khu. Một danh mục chỉ toàn
    // "Xã" dạy người đọc mã rằng loại đơn vị là một hằng số, và mô hình dữ liệu sẽ theo đó.
    const loai = (tien_to: string) => DEMO_DANH_MUC_XA.filter((xa) => xa.ten.startsWith(tien_to));
    expect(loai("Xã ").length).toBeGreaterThan(0);
    expect(loai("Phường ").length).toBeGreaterThan(0);
    expect(loai("Đặc khu ").length).toBeGreaterThan(0);
  });

  it("mỗi dòng đều có tỉnh/thành — hai xã trùng tên ở hai tỉnh là chuyện bình thường", () => {
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(xa.tinh.trim(), `${xa.ten}: thiếu tỉnh/thành`).not.toBe("");
    }
  });

  it("tra không thấy thì trả null, KHÔNG đoán một xã nào", () => {
    expect(DEMO_timTheoMa("01JKHONGCOTHATKHONGCOTHAT0")).toBeNull();
    expect(DEMO_timTheoMa("")).toBeNull();
    expect(DEMO_timTheoMa(XA_MOT.id)).toEqual(XA_MOT);
  });
});

/**
 * DÂY BẪY CHỐNG RẢI DỮ LIỆU GIẢ.
 *
 * Danh mục này là dữ liệu đặt ra cho một buổi trình diễn và **phải biến mất** khi `ListTenants`
 * của service `platform` có cài đặt. Một danh mục giả rải ở ba nơi là một danh mục sẽ sót lại
 * một mẩu khi có người dọn — và cái mẩu sót lại là một cơ quan nhà nước hiển thị đơn vị hành
 * chính không tồn tại. Xoá được sạch chỉ khi chỉ có MỘT nơi để xoá.
 */
describe("danh mục demo là nguồn tên xã duy nhất trong mã sản phẩm", () => {
  const RAW = import.meta.glob("../../**/*.{ts,tsx}", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  /** Bỏ chú thích: nhiều tệp GIẢI THÍCH bằng lời rằng chúng không chứa tên xã nào. */
  const khongChuThich = (ma: string) =>
    ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

  // Vite rút gọn khoá của `import.meta.glob` theo vị trí TỆP NÀY: tệp cùng thư mục thành
  // "./x.ts", tệp ngoài thành "../../x.ts". Nên nhận diện theo đuôi đường dẫn, không theo một
  // chuỗi tuyệt đối — một khoá đoán sai làm phép quét im lặng không tìm thấy gì.
  const LA_TEP_DANH_MUC = (duong: string) => duong.endsWith("/demo-danh-muc-xa.ts");
  /** Cùng thư mục với tệp test này, tức `features/kham-pha/` — thư mục của lớp khám phá. */
  const TRONG_KHAM_PHA = (duong: string) => duong.startsWith("./");

  const SAN_PHAM = Object.entries(RAW)
    .filter(([duong]) => !duong.includes(".test."))
    .map(([duong, ma]) => ({ duong, ma: khongChuThich(ma) }));

  /** Một tên đơn vị hành chính là một chuỗi BẮT ĐẦU bằng loại đơn vị — đó là hình dạng của nó. */
  const LA_TEN_DON_VI = /^(Xã|Phường|Đặc khu|Thị trấn)\s+\S/u;

  it("quét đúng cây mã thật — một lượt quét rỗng cũng xanh, và xanh sai lý do", () => {
    const duong = SAN_PHAM.map((t) => t.duong);
    expect(duong).toContain("../../App.tsx");
    expect(duong.filter(LA_TEP_DANH_MUC)).toHaveLength(1);
    expect(duong.filter(TRONG_KHAM_PHA).length).toBeGreaterThanOrEqual(3);
    expect(duong.length).toBeGreaterThanOrEqual(10);
  });

  it("không tệp nào khác chứa một chuỗi là tên đơn vị hành chính", () => {
    const viPham: string[] = [];
    for (const tep of SAN_PHAM) {
      if (LA_TEP_DANH_MUC(tep.duong)) continue;
      for (const khop of tep.ma.matchAll(/"([^"\\\n]*)"|'([^'\\\n]*)'/g)) {
        const chuoi = khop[1] ?? khop[2] ?? "";
        if (LA_TEN_DON_VI.test(chuoi)) viPham.push(`${tep.duong}: ${chuoi}`);
      }
    }
    expect(
      viPham,
      "một tên đơn vị hành chính được viết thẳng vào mã, ngoài danh mục demo.\n" +
        "Danh mục demo phải xoá được bằng cách xoá MỘT tệp khi ListTenants có cài đặt — " +
        "tên rải ra chỗ khác là tên sẽ sót lại sau khi dọn.",
    ).toEqual([]);
  });

  it("trong thư mục khám phá thì kể cả chữ trong JSX cũng không được mang tên xã", () => {
    // Phép kiểm trên chỉ thấy chuỗi trong dấu nháy. `<p>Xã An Thịnh</p>` không phải chuỗi — nên
    // riêng thư mục này, nơi rủi ro nằm, quét thẳng toàn văn.
    const viPham = SAN_PHAM.filter(
      (tep) =>
        TRONG_KHAM_PHA(tep.duong) &&
        !LA_TEP_DANH_MUC(tep.duong) &&
        /(Xã|Phường|Đặc khu|Thị trấn)\s+\p{Lu}/u.test(tep.ma),
    ).map((tep) => tep.duong);
    expect(viPham).toEqual([]);
  });
});

// ---------------------------------------------------------------------------------------------

describe("mức tin theo nguồn — tham số gợi ý, không bao giờ quyết", () => {
  it("quét QR tại trụ sở xã thì chọn sẵn, một chạm xác nhận", () => {
    const ra = phanGiaiGoiY(XA_MOT.id, "qr");
    expect(ra.kieu).toBe("chon-san");
    if (ra.kieu === "chon-san") expect(ra.xa).toEqual(XA_MOT);
  });

  it("liên kết do xã gửi (zns) thì cũng chọn sẵn", () => {
    expect(phanGiaiGoiY(XA_MOT.id, "zns").kieu).toBe("chon-san");
  });

  it("liên kết chuyển tay (share) thì LUÔN bắt chọn tường minh", () => {
    // Một liên kết chuyển tay nói lên ý định của NGƯỜI GỬI. Nó không nói gì về người nhận.
    const ra = phanGiaiGoiY(XA_MOT.id, "share");
    expect(ra.kieu).toBe("phai-chon");
    if (ra.kieu === "phai-chon") expect(ra.li_do).toBe("nguon-yeu");
  });

  it("không khai nguồn thì bắt chọn — không có mặc định 'coi như qr'", () => {
    // Luật 1, cấm #1: một mặc định trên đường cô lập. Đúng một dòng `?? "qr"` là đủ để mọi liên
    // kết không rõ nguồn trở thành một xã chọn sẵn, và không test nào khác đỏ.
    expect(phanGiaiGoiY(XA_MOT.id, "").kieu).toBe("phai-chon");
    expect(phanGiaiGoiY(XA_MOT.id, "email").kieu).toBe("phai-chon");
    expect(phanGiaiGoiY(XA_MOT.id, "QR").kieu).toBe("phai-chon");
  });

  it("không có tham số t thì vẫn ra được màn chọn xã, không ngõ cụt", () => {
    const ra = phanGiaiGoiY("", "qr");
    expect(ra.kieu).toBe("phai-chon");
    if (ra.kieu === "phai-chon") expect(ra.li_do).toBe("khong-tra-duoc");
  });

  it("tra mã không ra xã thì bắt chọn, và không kèm theo tên nào", () => {
    // Công dân xác nhận theo TÊN xã. Một tên đoán ra ở bước này là một hồ sơ gửi sang cơ quan
    // khác — công dân chờ vô ích rồi mất lòng tin vào cả kênh.
    const ra = phanGiaiGoiY("01JKHONGCOTHATKHONGCOTHAT0", "qr");
    expect(ra.kieu).toBe("phai-chon");
    expect(JSON.stringify(ra)).not.toContain(XA_MOT.ten);
  });

  it("nói nguồn bằng tiếng Việt của người dân, không bằng khoá kỹ thuật", () => {
    for (const nguon of ["qr", "zns", "share", ""]) {
      const nhan = nhanNguon(nguon);
      // "QR" được giữ lại: đó là từ người dân dùng hằng ngày, không phải khoá nội bộ. `zns`,
      // `src`, `share`, `tenant` thì không nói gì với ai ngoài người viết mã.
      expect(nhan).not.toMatch(/\b(zns|share|src|tenant)\b/i);
      expect(nhan.length).toBeGreaterThan(10);
    }
  });

  it("lời nhắn nói việc cần làm, không nói mã lỗi", () => {
    for (const nhan of Object.values(LOI_NHAN)) {
      expect(nhan).toMatch(/chọn xã/);
      expect(nhan).not.toMatch(/\b(error|lỗi|mã|code|\d{3})\b/i);
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("màn xác nhận xã", () => {
  const markup = render(
    <GoiYXaScreen xa={XA_MOT} nguon="qr" onXacNhan={() => {}} onChonXaKhac={() => {}} />,
  );

  it("hiện TÊN xã và tỉnh/thành, không hiện mã", () => {
    const chu = textOf(markup);
    expect(chu).toContain(XA_MOT.ten);
    expect(chu).toContain(XA_MOT.tinh);
    // Một màn xác nhận hiện ULID là một màn không ai xác nhận được.
    expect(chu).not.toContain(XA_MOT.id);
  });

  it("nói ra vì sao xã này được chọn sẵn, bằng chữ chứ không bằng màu", () => {
    expect(textOf(markup)).toContain(nhanNguon("qr"));
  });

  it("cho đúng hai đường đi: xác nhận, hoặc chọn xã khác", () => {
    const nut = markup.match(/<button\b/g) ?? [];
    expect(nut).toHaveLength(2);
    expect(textOf(markup)).toContain("Chọn xã khác");
  });

  it("hứa trước rằng tên xã sẽ theo suốt mọi màn hình", () => {
    expect(textOf(markup)).toMatch(/mọi màn hình|mỗi màn hình/);
  });

  it("cho một tiêu đề cấp một, và giấu mọi hình vẽ khỏi trình đọc màn hình", () => {
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) {
      expect(svg).toContain('aria-hidden="true"');
    }
  });
});

describe("màn chọn xã", () => {
  const markup = render(
    <ChonXaScreen li_do="nguon-yeu" onChon={() => {}} onXemGioiThieu={() => {}} />,
  );

  it("liệt kê đủ danh mục, mỗi xã là một nút bấm được", () => {
    const chu = textOf(markup);
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(chu, `thiếu ${xa.ten}`).toContain(xa.ten);
      expect(chu).toContain(xa.tinh);
    }
    // Mỗi xã một nút, cộng nút thoát về phần giới thiệu công ty.
    expect((markup.match(/<button\b/g) ?? []).length).toBe(DEMO_DANH_MUC_XA.length + 1);
  });

  it("nói thẳng đây là dữ liệu mẫu của bản trình diễn", () => {
    expect(textOf(markup)).toContain(DEMO_GHI_CHU);
  });

  it("giải thích vì sao phải chọn, khi công dân không phải người vừa yêu cầu", () => {
    expect(textOf(markup)).toContain(LOI_NHAN["nguon-yeu"]);
  });

  it("không giải thích gì khi chính công dân bấm Đổi xã", () => {
    const tuBam = render(<ChonXaScreen li_do={null} onChon={() => {}} onXemGioiThieu={() => {}} />);
    expect(textOf(tuBam)).not.toContain(LOI_NHAN["nguon-yeu"]);
    expect(textOf(tuBam)).not.toContain(LOI_NHAN["khong-tra-duoc"]);
  });

  it("mở không một ô nhập liệu nào — kể cả ô tìm theo tên", () => {
    // Giai đoạn 1 không thu thập gì, và đó là thứ làm luật 3 đúng bằng CẤU TRÚC. Tám dòng thì
    // cuộn nhanh hơn gõ; ô tìm kiếm quay lại cùng lúc với danh mục thật.
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });

  it("giấu mọi hình vẽ khỏi trình đọc màn hình, và cho đúng một tiêu đề cấp một", () => {
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) {
      expect(svg).toContain('aria-hidden="true"');
    }
  });
});

// ---------------------------------------------------------------------------------------------

describe("bất di dịch #2 — chọn xã rồi thì tên xã hiện trên MỌI màn hình", () => {
  it("in tên xã ở đầu từng màn hình một, không sót màn nào", () => {
    // Đây là bất biến đắt nhất của kênh công dân: công dân không thấy mình đang gửi cho xã nào
    // thì gửi nhầm, xã nhận việc ngoài địa bàn, phải chuyển hoặc từ chối, và người dân chờ vô ích.
    for (const man of SCREENS) {
      const markup = render(
        <KhungApp man={man.id} onChonMan={() => {}} xaDaChon={XA_MOT} onDoiXa={() => {}}>
          <p>nội dung</p>
        </KhungApp>,
      );
      const dau = textOf(markup.slice(0, markup.indexOf("</header>")));
      expect(dau, `màn ${man.id} không hiện tên xã ở header`).toContain(XA_MOT.ten);
      expect(dau).toContain(man.headerTitle);
    }
  });

  it("mở đường ĐỔI XÃ tường minh ngay cạnh tên xã", () => {
    // ADR 0005: đổi xã là hành động tường minh, không bao giờ tự động.
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={XA_MOT} onDoiXa={() => {}}>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(textOf(markup.slice(0, markup.indexOf("</header>")))).toContain("Đổi xã");
  });

  it("chưa chọn xã thì ô ấy giữ tên đơn vị phát hành, và không có nút đổi xã", () => {
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null} onDoiXa={() => {}}>
        <p>nội dung</p>
      </KhungApp>,
    );
    const dau = textOf(markup.slice(0, markup.indexOf("</header>")));
    expect(dau).toContain(COMPANY.name);
    expect(dau).not.toContain("Đổi xã");
  });

  it("giấu thanh tab khi đang chọn xã — mỗi màn một việc", () => {
    // `skills/accessibility-elderly` #4. Một thanh tab còn đó trong lúc chưa có xã là bốn nút
    // bấm vào không có gì xảy ra, và đó là lúc người lớn tuổi kết luận app hỏng.
    const dangChon = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null} onDoiXa={() => {}} khamPha>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(dangChon).not.toContain("tabbar");
    expect(textOf(dangChon.slice(0, dangChon.indexOf("</header>")))).toContain("Chọn xã");

    const binhThuong = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={XA_MOT} onDoiXa={() => {}}>
        <p>nội dung</p>
      </KhungApp>,
    );
    expect(binhThuong).toContain("tabbar");
  });

  it("đang gợi ý thì header VẪN CHƯA mang tên xã — tham số không chọn thay công dân", () => {
    // Lớp khám phá (ADR 0005) chỉ dẫn giao diện. Chừng nào công dân chưa xác nhận thì chưa có
    // xã nào, và header phải nói đúng điều đó.
    const markup = render(
      <KhungApp man="home" onChonMan={() => {}} xaDaChon={null} onDoiXa={() => {}}>
        <GoiYXaScreen xa={XA_MOT} nguon="qr" onXacNhan={() => {}} onChonXaKhac={() => {}} />
      </KhungApp>,
    );
    const dau = textOf(markup.slice(0, markup.indexOf("</header>")));
    expect(dau).toContain(COMPANY.name);
    expect(dau).not.toContain(XA_MOT.ten);
    // Nhưng thân màn hình thì có, vì đó chính là thứ đang chờ xác nhận.
    expect(textOf(markup)).toContain(XA_MOT.ten);
  });
});
