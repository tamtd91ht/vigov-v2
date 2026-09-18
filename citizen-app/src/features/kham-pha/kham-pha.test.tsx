/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungApp } from "../../App";
import { COMPANY } from "../../content/company-profile";
import { SCREENS } from "../company-intro/screens";
import { ChonXaScreen } from "./ChonXaScreen";
import * as danhMuc from "./demo-danh-muc-xa";
import {
  DEMO_DANH_MUC_XA,
  DEMO_GHI_CHU,
  DEMO_GHI_CHU_TRANG_XA,
  DEMO_TEN_DICH_VU,
  DEMO_timTheoMa,
  type LoaiDichVu,
} from "./demo-danh-muc-xa";
import { GoiYXaScreen } from "./GoiYXaScreen";
import { LOI_NHAN, nhanNguon, phanGiaiGoiY } from "./goi-y";
import { DichVuDong, LOI_NHAN_DANG_LAM, TrangXaScreen } from "./TrangXaScreen";

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

  it("mỗi xã có nội dung RIÊNG, không phải một trang dùng chung", () => {
    // Một trang xã giống hệt trang xã bên cạnh thì tên xã ở header là bằng chứng duy nhất rằng
    // công dân vào đúng chỗ — và một bằng chứng duy nhất, đọc lướt, thì không ai đọc.
    const rieng = (lay: (xa: (typeof DEMO_DANH_MUC_XA)[number]) => string) =>
      new Set(DEMO_DANH_MUC_XA.map(lay)).size;
    expect(rieng((xa) => xa.gioi_thieu)).toBe(DEMO_DANH_MUC_XA.length);
    expect(rieng((xa) => xa.dien_thoai_truc)).toBe(DEMO_DANH_MUC_XA.length);
    expect(rieng((xa) => xa.dich_vu.join("|"))).toBeGreaterThan(1);
    expect(rieng((xa) => xa.gio_lam_viec)).toBeGreaterThan(1);
  });

  it("mỗi xã mở 3–5 dịch vụ, không trùng, và đều là dịch vụ có tên", () => {
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(xa.dich_vu.length, `${xa.ten}: số dịch vụ`).toBeGreaterThanOrEqual(3);
      expect(xa.dich_vu.length, `${xa.ten}: số dịch vụ`).toBeLessThanOrEqual(5);
      expect(new Set(xa.dich_vu).size, `${xa.ten}: có dịch vụ lặp`).toBe(xa.dich_vu.length);
      for (const loai of xa.dich_vu) {
        // Một mục không có tên là một nút trống trên màn hình của người dân.
        expect(DEMO_TEN_DICH_VU[loai]?.trim(), `${xa.ten}: dịch vụ ${loai} không có tên`).toBeTruthy();
      }
    }
  });

  it("mỗi xã có một dòng giới thiệu và một khung giờ làm việc đọc được", () => {
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(xa.gioi_thieu.trim().length, `${xa.ten}: thiếu câu giới thiệu`).toBeGreaterThan(20);
      expect(xa.gio_lam_viec, `${xa.ten}: giờ làm việc không có giờ nào`).toMatch(/\d/);
    }
  });
});

/**
 * ⚠ DỮ LIỆU CÁ NHÂN TRONG DANH MỤC MẪU — luật 3.
 *
 * Danh mục này in ra một SỐ ĐIỆN THOẠI trên màn hình của một ứng dụng đã xuất bản. Một số thật
 * lọt vào đây là dữ liệu cá nhân đã phát tán, không thu hồi được: bundle đã tải về máy người
 * dùng, và nếu là số máy của một cán bộ thì đó là số cá nhân bị công bố dưới tên một cơ quan
 * nhà nước.
 *
 * VÌ SAO PHÉP QUÉT NẰM Ở ĐÂY CHỨ KHÔNG PHẢI TRONG `company-profile.test.ts`:
 *
 *   Phép quét ở đó đọc `collectStrings(profile)` — mọi chuỗi của **mô-đun nội dung công ty**, và
 *   không thấy tệp này. Kéo danh mục xã sang đó thì tệp test nội dung công ty phụ thuộc vào dữ
 *   liệu trình diễn, và nó sẽ đỏ đúng vào hôm có người xoá danh mục ấy đi (README §Còn thiếu #0)
 *   — một phép kiểm đỏ vì một lý do không liên quan là một phép kiểm sắp bị tắt. Nên cùng một
 *   phép quét, cùng cách tự bảo trì, đặt cạnh dữ liệu mà nó canh.
 *
 *   `phase1-collects-nothing.test.ts` quét toàn cây mã như một lưới cuối; ca dưới đây là lưới
 *   đầu, và nó nói ra được tên xã nào đang mang số sai.
 */
describe("danh mục mẫu không mang dữ liệu cá nhân", () => {
  /** Mọi chuỗi mô-đun này xuất ra — gom từ chính đối tượng mô-đun, không gõ tay. */
  function gomChuoi(gia_tri: unknown, vao: string[] = []): string[] {
    if (typeof gia_tri === "string") vao.push(gia_tri);
    else if (Array.isArray(gia_tri)) for (const mot of gia_tri) gomChuoi(mot, vao);
    else if (gia_tri && typeof gia_tri === "object")
      for (const mot of Object.values(gia_tri)) gomChuoi(mot, vao);
    return vao;
  }

  /**
   * Gom từ ĐỐI TƯỢNG MÔ-ĐUN, không từ một danh sách gõ tay — đó là thứ làm phép quét tự bảo trì.
   * Thêm một trường vào `XaDemo` (email, địa chỉ trụ sở, tên người trực) mà danh sách gõ tay
   * không được cập nhật thì trường ấy xuất bản mà chưa từng đi qua phép quét nào, và không có gì
   * nói ra điều đó. Hàm không phải chuỗi nên `DEMO_timTheoMa` tự rơi ra ngoài.
   */
  const MOI_CHUOI = gomChuoi(danhMuc);

  it("dùng ĐÚNG dải số giả đã thoả thuận cho mọi số trực", () => {
    // Luật 3, bất biến 5: ví dụ và dữ liệu mẫu dùng `0900000000`, ở đây là biến thể `090000000x`
    // để tám xã khác số nhau. Chỉ chữ số — dấu cách để đọc do màn hình thêm vào.
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(xa.dien_thoai_truc, `${xa.ten}: số trực ngoài dải giả đã thoả thuận`).toMatch(
        /^090000000\d$/,
      );
    }
  });

  it("không chuỗi nào là một số di động Việt Nam ngoài dải giả ấy", () => {
    for (const chuoi of MOI_CHUOI) {
      const so = chuoi.replace(/[\s.\-()]/g, "").replace(/090000000\d/g, "SO-GIA");
      expect(so, `số trông như số thật trong: ${chuoi}`).not.toMatch(/(^|\D)0[35789]\d{8}(\D|$)/);
    }
  });

  it("không chuỗi nào chứa một số định danh cá nhân 12 chữ số", () => {
    for (const chuoi of MOI_CHUOI) {
      expect(chuoi.replace(/\s/g, ""), `dãy 12 chữ số trong: ${chuoi}`).not.toMatch(
        /(^|\D)\d{12}(\D|$)/,
      );
    }
  });

  it("lưu tiếng Việt ở dạng dựng sẵn (NFC), không dạng tách dấu", () => {
    // "ế" dán từ một số trình soạn thảo về dưới dạng "ê" + dấu sắc rời: nhìn y hệt trong diff,
    // vẽ lệch dấu trên một số phông Android, và làm mọi phép so chuỗi hỏng vì một lý do không
    // nhìn thấy được.
    for (const chuoi of MOI_CHUOI) {
      expect(chuoi.normalize("NFC"), `tiếng Việt tách dấu trong: ${chuoi}`).toBe(chuoi);
    }
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

describe("trang xã — mỗi xã một nội dung riêng", () => {
  const markup = render(<TrangXaScreen xa={XA_MOT} />);

  it("in tên, tỉnh/thành, câu giới thiệu, số trực và giờ làm việc của ĐÚNG xã ấy", () => {
    const chu = textOf(markup);
    expect(chu).toContain(XA_MOT.ten);
    expect(chu).toContain(XA_MOT.tinh);
    expect(chu).toContain(XA_MOT.gioi_thieu);
    expect(chu).toContain(XA_MOT.gio_lam_viec);
    // Số hiện ra có dấu cách cho dễ đọc; kho chỉ giữ chữ số. Bỏ khoảng trắng rồi mới so.
    expect(chu.replace(/\s/g, "")).toContain(XA_MOT.dien_thoai_truc);
  });

  it("KHÔNG mang nội dung của xã khác", () => {
    // Đây là chế độ hỏng đắt nhất của màn này: công dân đọc tên xã mình rồi đọc tiếp số điện
    // thoại của xã bên cạnh, và gọi vào đó.
    const chu = textOf(markup).replace(/\s/g, "");
    for (const xa of DEMO_DANH_MUC_XA) {
      if (xa.id === XA_MOT.id) continue;
      expect(chu, `lẫn số trực của ${xa.ten}`).not.toContain(xa.dien_thoai_truc);
      expect(textOf(markup), `lẫn giới thiệu của ${xa.ten}`).not.toContain(xa.gioi_thieu);
    }
  });

  it("dựng được cho từng xã một, và xã nào ra dịch vụ của xã ấy", () => {
    const MOI_LOAI = Object.keys(DEMO_TEN_DICH_VU) as LoaiDichVu[];
    for (const xa of DEMO_DANH_MUC_XA) {
      const chu = textOf(render(<TrangXaScreen xa={xa} />));
      for (const loai of MOI_LOAI) {
        const co = xa.dich_vu.includes(loai);
        expect(chu.includes(DEMO_TEN_DICH_VU[loai]), `${xa.ten} · ${loai}`).toBe(co);
      }
    }
  });

  it("mỗi dịch vụ là một nút bấm được, và không có nút nào khác trên màn", () => {
    expect((markup.match(/<button\b/g) ?? []).length).toBe(XA_MOT.dich_vu.length);
  });

  it("nói TRẠNG THÁI bằng chữ trên từng mục, không bằng riêng màu", () => {
    expect((markup.match(/Chưa mở/g) ?? []).length).toBe(XA_MOT.dich_vu.length);
  });

  it("không dựng màn giả: không một liên kết nào dẫn đi đâu", () => {
    // Các mục chưa có màn nào phía sau. Một `<a>` ở đây là một lời hứa app chưa giữ được — và
    // số điện thoại thì là số GIẢ của bản trình diễn, nên một `tel:` là một cuộc gọi mất không.
    expect(markup).not.toMatch(/<a[\s>]/);
    expect(markup).not.toContain("tel:");
  });

  it("nói rõ liên hệ, giờ và dịch vụ đều là dữ liệu mẫu", () => {
    // "Danh mục mẫu" chỉ nói về danh sách xã. Người đọc trang này có quyền hiểu rằng số điện
    // thoại thì thật — nên câu ở đây phải gọi tên cả ba thứ.
    expect(textOf(markup)).toContain(DEMO_GHI_CHU_TRANG_XA);
  });

  it("mở không một ô nhập liệu nào", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });

  it("cho đúng một tiêu đề cấp một, và giấu mọi hình vẽ khỏi trình đọc màn hình", () => {
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) {
      expect(svg).toContain('aria-hidden="true"');
    }
  });
});

describe("một mục dịch vụ chưa mở thì nói ra, chứ không im lặng", () => {
  const dong = (mo: boolean) =>
    render(<DichVuDong loai={XA_MOT.dich_vu[0]!} mo={mo} onBam={() => {}} />);

  it("chưa bấm thì chưa có lời nhắn nào, và nút khai đang đóng", () => {
    expect(textOf(dong(false))).not.toContain(LOI_NHAN_DANG_LAM);
    expect(dong(false)).toContain('aria-expanded="false"');
  });

  it("bấm rồi thì hiện lời nhắn, và trình đọc màn hình đọc được nó", () => {
    // Bấm một nút mà không có gì xảy ra là cách nhanh nhất để một người lớn tuổi kết luận rằng
    // app hỏng (`skills/accessibility-elderly`). `role="status"` để người không nhìn màn hình
    // cũng nhận được câu trả lời ấy.
    const mo = dong(true);
    expect(textOf(mo)).toContain(LOI_NHAN_DANG_LAM);
    expect(mo).toContain('role="status"');
    expect(mo).toContain('aria-expanded="true"');
  });

  it("lời nhắn nói việc cần làm bây giờ, không nói mã lỗi", () => {
    // README §Error message shape. "Sắp ra mắt" là một câu không chỉ ai đi đâu cả; người dân bấm
    // vào "phản ánh hiện trường" là người dân đang có việc cần báo hôm nay.
    expect(LOI_NHAN_DANG_LAM).toMatch(/gọi số điện thoại trực/);
    expect(LOI_NHAN_DANG_LAM).not.toMatch(/\b(error|lỗi|code|404|500)\b/i);
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
