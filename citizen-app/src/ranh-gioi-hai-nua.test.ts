/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import { KHAI_BAO_LOI_GOI } from "./features/tinh-nang/zalo-api";

/**
 * BA RÀNG BUỘC KIẾN TRÚC CỦA MỘT MINI APP MANG HAI NỬA NGHIỆP VỤ — viết TRƯỚC khi nửa thứ hai
 * (`src/cong-dan/`) có tệp nghiệp vụ đầu tiên.
 *
 * MỘT App ID, MỘT bundle, MỘT origin. Đó không phải một lựa chọn kiến trúc, đó là hình dạng của
 * nền tảng: một Mini App được nhận diện bằng App ID, và 300 xã không thể là 300 app. Hệ quả:
 *
 *   • hai nửa chạy trong **cùng một tiến trình**, nên không có ranh giới tiến trình nào ngăn nửa
 *     này gọi hàm của nửa kia;
 *   • hai nửa dùng **cùng một kho lưu trữ**, nên `localStorage` của nửa này ĐỌC ĐƯỢC bởi nửa kia;
 *   • hai nửa **thừa hưởng cùng một tập quyền**, vì Zalo cấp quyền theo App ID.
 *
 * Cả ba hệ quả ấy đều **đúng theo cấu tạo** và không sửa được. Thứ sửa được là: mã nguồn có đi
 * qua những cầu nối ấy hay không. Ba ca dưới đây là ba câu trả lời **đo được** cho điều đó.
 *
 * ⚠ VÌ SAO VIẾT BÂY GIỜ, KHI `src/cong-dan/` CÒN RỖNG:
 *
 *   Một ranh giới viết sau khi hai bên đã có mã là một cuộc dọn dẹp: mỗi vi phạm là một tệp có
 *   người đang dùng, và cuộc thương lượng luôn kết thúc bằng một ngoại lệ. Viết trước thì lần vi
 *   phạm đầu tiên đỏ lên trước khi ai kịp dựa vào nó.
 *
 *   Nhưng một dây bẫy quét một thư mục rỗng là một dây bẫy **xanh vì không tìm thấy gì** — đúng
 *   chế độ hỏng mà `phase1-collects-nothing.test.ts` đã ghi lại hai lần. Nên mỗi ca ở đây cho
 *   hàm kiểm **ăn một vi phạm dựng sẵn** và khẳng định nó bắt, bên cạnh lượt quét trên cây mã
 *   thật. Lượt quét trả lời "hôm nay sạch"; ca dựng sẵn trả lời "và phép kiểm còn sống".
 */

const RAW_SOURCES = import.meta.glob("./**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/**
 * Bỏ chú thích trước khi quét — cùng lý do, và cùng biểu thức, với `phase1-collects-nothing`:
 * chính tệp này GIẢI THÍCH bằng văn xuôi những thứ nó cấm, và quét văn bản thô thì lời giải
 * thích vi phạm đúng cái luật nó mô tả. `//` đứng sau `:` được tha để `https://…` sống sót.
 */
function boChuThich(ma: string): string {
  return ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
}

type TepNguon = { path: string; code: string };

const TEP_SAN_XUAT: readonly TepNguon[] = Object.entries(RAW_SOURCES)
  .filter(([path]) => !path.includes(".test."))
  .map(([path, code]) => ({ path, code: boChuThich(code) }));

/* =============================================================================================
   RÀNG BUỘC 3a — RANH GIỚI HAI NỬA, CẤM CẢ HAI CHIỀU
   ============================================================================================= */

type Nua = "thuong-mai" | "nha-nuoc";

/**
 * HAI NỬA, KHAI BẰNG TIỀN TỐ ĐƯỜNG DẪN — và cái thứ ba, `TRUNG_LAP`, là thứ làm bảng này trung
 * thực.
 *
 * `App.tsx`, `main.tsx`, `components/` và `lib/` là **lớp vỏ**: chúng không phục vụ khách hàng
 * doanh nghiệp và cũng không phục vụ công dân, chúng dựng khung và đọc tham số mở app. Xếp chúng
 * vào một nửa sẽ cấm nửa kia dùng thanh tab — tức là cấm một thứ không có hại, và một dây bẫy
 * cấm thứ vô hại là một dây bẫy sắp bị tắt.
 *
 * `features/kham-pha/` và `features/diagnostics/` cũng ở đây, và đó là một quyết định có hạn:
 * lớp khám phá nói chuyện xã/phường nên nó THUỘC VỀ nửa nhà nước về nghiệp vụ, nhưng hôm nay nó
 * là một lớp TRÌNH DIỄN sau `resolve.alias`, bị gỡ khỏi bản nộp, và nó đứng trên dữ liệu đặt ra.
 * Ngày nó nói chuyện với máy chủ thật, nó chuyển sang `./cong-dan/` — và ca "mọi tệp phải thuộc
 * đúng một khu" ở dưới là thứ bắt người chuyển phải khai lại bảng này thay vì để nó trôi.
 */
const KHU_VUC: Readonly<Record<Nua | "trung-lap", readonly string[]>> = {
  "thuong-mai": [
    "./content/",
    "./features/company-intro/",
    "./features/tinh-nang/",
    "./features/dang-nhap/",
    // Bộ chọn ba bước trên màn chủ (22/09/2026). Nó đứng trên `SOLUTIONS` — danh mục sản phẩm của
    // một doanh nghiệp — và không biết gì về xã/phường: nửa thương mại, không phải trung lập.
    "./features/goi-y-giai-phap/",
    // BỀ MẶT YÊU CẦU (22/09/2026, giai đoạn B) — tư vấn, báo giá, đề nghị gọi lại.
    "./features/yeu-cau/",
    /**
     * ⚠ `./api/` LÀ CLIENT CỦA `vihat-miniapp`, KHÔNG PHẢI CLIENT CỦA ViGov — VÀ HAI THƯ MỤC TÊN
     * `api` TRONG CÙNG MỘT CÂY MÃ LÀ MỘT CÁI BẪY PHẢI NÓI RA, KHÔNG PHẢI MỘT SỰ TRÙNG TÊN.
     *
     *   `./api/`          — backend thương mại `vihat-miniapp`: phiên đăng nhập, yêu cầu tư vấn.
     *                       NỬA THƯƠNG MẠI.
     *   `./cong-dan/api/` — client API của ViGov, chưa có tệp nào. NỬA NHÀ NƯỚC, và `CUA_CLIENT_VIGOV`
     *                       ở dưới cấm MỌI tệp ngoài `./cong-dan/` nhập nó — kể cả lớp vỏ trung lập.
     *
     *   Hai ràng buộc ấy không đè lên nhau: tiền tố `./cong-dan/api/` dài hơn và không phải tiền tố
     *   của `./api/`. Ca "`./api/` là nửa thương mại, `./cong-dan/api/` vẫn bị cấm từ ngoài" ở cuối
     *   tệp giữ cho hai thứ ấy không bị ai gộp lại.
     */
    "./api/",
  ],
  "nha-nuoc": ["./cong-dan/"],
  "trung-lap": [
    "./App.tsx",
    "./main.tsx",
    "./components/",
    "./lib/",
    "./features/kham-pha/",
    "./features/diagnostics/",
  ],
};

/**
 * CLIENT API CỦA ViGov — MỘT ĐƯỜNG DẪN, KHAI TỪ TRƯỚC KHI TỆP TỒN TẠI.
 *
 * Đây là ràng buộc NẶNG NHẤT trong tệp này, và nó gắt hơn ranh giới hai nửa ở trên một bậc:
 * ranh giới kia cấm nửa thương mại nhập nửa nhà nước; ràng buộc này cấm **mọi tệp bên ngoài
 * `./cong-dan/`** nhập client ViGov — kể cả lớp vỏ trung lập.
 *
 * VÌ SAO CẢ LỚP VỎ CŨNG BỊ CẤM: `App.tsx` là tệp duy nhất được nối hai nửa lại, và nó **không
 * nằm sau `resolve.alias` nào** — một `import` trong đó đi thẳng vào BẢN NỘP. Nếu lớp vỏ được
 * phép nhập client ViGov thì phiên công dân có một đường tới từ chính chỗ mà nửa thương mại gọi
 * được, và ranh giới ở trên chỉ còn là một lời hứa. Nửa nhà nước mở màn hình của mình; client
 * chỉ được gọi từ bên trong nửa ấy.
 *
 * Tệp chưa tồn tại. Đó là lý do nó được khai **bây giờ**: một đường dẫn khai trước là một đường
 * dẫn người viết tệp đầu tiên phải đọc.
 */
const CUA_CLIENT_VIGOV = "./cong-dan/api/";

/** Tiền tố khớp một trong các khu đã khai. */
function khuCua(duong_dan: string): Nua | "trung-lap" | null {
  for (const [khu, tien_to] of Object.entries(KHU_VUC)) {
    if (tien_to.some((t) => (t.endsWith("/") ? duong_dan.startsWith(t) : duong_dan === t))) {
      return khu as Nua | "trung-lap";
    }
  }
  return null;
}

/**
 * Mọi chuỗi tên mô-đun một tệp nhập vào — `import … from`, `await import()`, `require()`.
 *
 * BẮT THEO **CHUỖI TÊN MÔ-ĐUN**, KHÔNG THEO CÚ PHÁP `import`. Đây là bài học đã trả giá một lần
 * trong kho này: một dây bẫy khớp `import { X } from "Y"` chết trong im lặng ngày mã đổi sang
 * `await import("Y")`, và không có gì đỏ lên để báo rằng một phép kiểm vừa mất nội dung
 * (`phase1-collects-nothing.test.ts`, khe `zmp-sdk`, 18/09). Ở đây mọi hình thức nhập đều phải
 * gõ ra một chuỗi trong ngoặc, nên bắt chuỗi là bắt tất.
 */
function tenModuleNhap(ma: string): string[] {
  const ra: string[] = [];
  for (const khop of ma.matchAll(/(?:\bfrom|\bimport|\brequire)\s*\(?\s*["'`]([^"'`]+)["'`]/g)) {
    ra.push(khop[1]!);
  }
  return ra;
}

/**
 * Quy một tên mô-đun về đường dẫn trong `src/`, hoặc `null` nếu nó không trỏ vào `src/`.
 *
 * Ba hình dạng phải quy: tương đối (`../content/x`), bí danh `@/…` (tsconfig `paths`), và bí danh
 * biến thể `bien-the/…` (`vite.config.ts` `resolve.alias`). Gói ngoài (`react`, `zmp-sdk`) trả
 * `null` — chúng không thuộc nửa nào.
 */
function giaiDuongDan(tu_tep: string, ten: string): string | null {
  if (ten.startsWith("bien-the/kham-pha")) return "./features/kham-pha/index.ts";
  if (ten.startsWith("bien-the/chan-doan")) return "./features/diagnostics/index.ts";
  // Cửa thứ ba (24/09/2026). Không quy nó thì `App.tsx` nhập kênh công dân qua alias mà ranh giới
  // KHÔNG nhìn thấy — `null` ở đây là "gói ngoài, không thuộc nửa nào", tức một điểm mù.
  if (ten.startsWith("bien-the/cong-dan")) return "./cong-dan/index.ts";
  if (ten.startsWith("@/")) return `./${ten.slice(2)}`;
  if (!ten.startsWith(".")) return null;

  const doan = tu_tep.split("/").slice(0, -1);
  for (const buoc of ten.split("/")) {
    if (buoc === "." || buoc === "") continue;
    if (buoc === "..") doan.pop();
    else doan.push(buoc);
  }
  return doan.join("/");
}

/** Một lần nhập vượt ranh giới: tệp nào, nhập gì, từ khu nào sang khu nào. */
type ViPhamNhap = { tu: string; toi: string; khu_tu: string; khu_toi: string };

function nhapVuotRanhGioi(tep: readonly TepNguon[]): ViPhamNhap[] {
  const ra: ViPhamNhap[] = [];
  for (const f of tep) {
    const khu_tu = khuCua(f.path);
    if (khu_tu === null) continue; // ca riêng ở dưới lo chuyện một tệp không thuộc khu nào
    for (const ten of tenModuleNhap(f.code)) {
      const toi = giaiDuongDan(f.path, ten);
      if (toi === null) continue;
      const khu_toi = khuCua(toi);
      if (khu_toi === null) continue;
      const vuot =
        (khu_tu === "thuong-mai" && khu_toi === "nha-nuoc") ||
        (khu_tu === "nha-nuoc" && khu_toi === "thuong-mai");
      if (vuot) ra.push({ tu: f.path, toi, khu_tu, khu_toi });
    }
  }
  return ra;
}

/** Nhập client ViGov từ bên ngoài nửa nhà nước — cấm cả với lớp vỏ trung lập. */
function nhapClientViGovTuNgoai(tep: readonly TepNguon[]): ViPhamNhap[] {
  const ra: ViPhamNhap[] = [];
  for (const f of tep) {
    if (f.path.startsWith("./cong-dan/")) continue;
    for (const ten of tenModuleNhap(f.code)) {
      const toi = giaiDuongDan(f.path, ten);
      if (toi !== null && toi.startsWith(CUA_CLIENT_VIGOV)) {
        ra.push({ tu: f.path, toi, khu_tu: khuCua(f.path) ?? "chưa khai", khu_toi: "client ViGov" });
      }
    }
  }
  return ra;
}

describe("3a — ranh giới hai nửa, cấm cả hai chiều", () => {
  it("quét cây mã THẬT — một lượt quét rỗng sẽ xanh vì lý do sai", () => {
    const duong_dan = TEP_SAN_XUAT.map((f) => f.path);
    expect(duong_dan).toContain("./App.tsx");
    expect(duong_dan).toContain("./content/company-profile.ts");
    expect(duong_dan).toContain("./features/tinh-nang/zalo-api.ts");
    // Nửa nhà nước phải CÓ MẶT trong lượt quét kể cả khi chưa có nghiệp vụ. Một khu được canh mà
    // lượt quét không đọc tới thì "được canh" và "không tồn tại" là một.
    expect(
      duong_dan.filter((p) => p.startsWith("./cong-dan/")).length,
      "nửa nhà nước không có tệp nào trong lượt quét — ranh giới đang canh một khoảng trống",
    ).toBeGreaterThan(0);
    expect(duong_dan.length).toBeGreaterThanOrEqual(15);
  });

  it("mọi tệp sản xuất thuộc ĐÚNG MỘT khu — thư mục mới buộc phải khai", () => {
    // Không có ca này thì một thư mục mới (`src/thanh-toan/`) rơi ra ngoài mọi tiền tố, và ranh
    // giới KHÔNG cấm nó nhập bất cứ thứ gì của bất cứ nửa nào — im lặng, vì nó không thuộc nửa
    // nào để mà vi phạm.
    const chua_khai = TEP_SAN_XUAT.filter((f) => khuCua(f.path) === null).map((f) => f.path);
    expect(
      chua_khai,
      "tệp không thuộc nửa thương mại, nửa nhà nước, hay lớp vỏ trung lập. Khai nó vào `KHU_VUC` " +
        "— chọn khu là chọn nó được nhập gì và ai được nhập nó.",
    ).toEqual([]);
  });

  it("nửa thương mại KHÔNG nhập nửa nhà nước, và nửa nhà nước KHÔNG nhập nửa thương mại", () => {
    const vi_pham = nhapVuotRanhGioi(TEP_SAN_XUAT);
    expect(
      vi_pham.map((v) => `${v.tu} (${v.khu_tu}) -> ${v.toi} (${v.khu_toi})`),
      "một nửa vừa nhập tệp của nửa kia. Hai nửa chạy trong cùng một tiến trình, nên một lời gọi " +
        "hàm là tất cả những gì cần để phiên công dân với tới được từ màn bán hàng — và ngược lại, " +
        "để nội dung doanh nghiệp đi vào một màn hình công dân đang chờ một cơ quan nhà nước.",
    ).toEqual([]);
  });

  it("KHÔNG tệp nào ngoài `./cong-dan/` nhập client API của ViGov — kể cả lớp vỏ", () => {
    const vi_pham = nhapClientViGovTuNgoai(TEP_SAN_XUAT);
    expect(
      vi_pham.map((v) => `${v.tu} (${v.khu_tu}) -> ${v.toi}`),
      `client ViGov chỉ được gọi từ bên trong \`./cong-dan/\`. Một đường nhập từ ngoài — nhất là ` +
        `từ \`App.tsx\`, tệp KHÔNG nằm sau một \`resolve.alias\` nào — đưa tuyến phiên công dân ` +
        `thẳng vào BẢN NỘP của nửa thương mại.`,
    ).toEqual([]);
  });

  /* ------------------------------------------------------------------------------------------
     THỬ ĐỘT BIẾN DỰNG SẴN — phần trả lời câu "phép kiểm trên còn sống không".

     Ba ca trên quét một cây mã hôm nay SẠCH ở cả ba ràng buộc, nên cả ba đều xanh, và một dây
     bẫy xanh không nói lên gì. Ca dưới cho từng hàm kiểm ăn một vi phạm dựng sẵn.
     ------------------------------------------------------------------------------------------ */

  it("bắt được nhập vượt ranh giới ở CẢ HAI CHIỀU, mọi hình thức nhập", () => {
    const VI_PHAM: readonly TepNguon[] = [
      // thương mại -> nhà nước
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { X } from "../../cong-dan/phien";' },
      { path: "./content/company-profile.ts", code: 'const x = await import("../cong-dan/xa");' },
      { path: "./features/tinh-nang/zalo-api.ts", code: 'const x = require("@/cong-dan/phien");' },
      // Qua alias cũng là nhập nửa nhà nước — cửa `bien-the/cong-dan` không phải lối tắt.
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { KenhCongDan } from "bien-the/cong-dan";' },
      // nhà nước -> thương mại
      { path: "./cong-dan/TrangXa.tsx", code: 'import { COMPANY } from "../content/company-profile";' },
      { path: "./cong-dan/api/vigov.ts", code: 'import { thanYeuCau } from "@/features/dang-nhap/hop-dong";' },
      { path: "./cong-dan/man/Gui.tsx", code: "import { KhoiDangNhap } from '../../features/tinh-nang/index';" },
    ];
    for (const tep of VI_PHAM) {
      expect(
        nhapVuotRanhGioi([tep]).length,
        `ranh giới không bắt được: ${tep.path} — ${tep.code}`,
      ).toBe(1);
    }

    // Và nó KHÔNG kêu oan. Một dây bẫy kêu sai chỗ bị tắt nhanh y như một dây bẫy câm.
    const HOP_LE: readonly TepNguon[] = [
      { path: "./App.tsx", code: 'import { COMPANY } from "./content/company-profile";' },
      { path: "./App.tsx", code: 'import { X } from "./cong-dan/index";' },
      { path: "./App.tsx", code: 'import { KenhCongDan } from "bien-the/cong-dan";' },
      { path: "./features/tinh-nang/zalo-api.ts", code: 'const sdk = await import("zmp-sdk");' },
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { useState } from "react";' },
      { path: "./cong-dan/index.ts", code: 'import { thamSo } from "../lib/launch-params";' },
    ];
    for (const tep of HOP_LE) {
      expect(nhapVuotRanhGioi([tep]), `ranh giới kêu oan ở: ${tep.code}`).toEqual([]);
    }
  });

  it("bắt được nhập client ViGov từ NỬA THƯƠNG MẠI và từ LỚP VỎ", () => {
    const VI_PHAM: readonly TepNguon[] = [
      { path: "./App.tsx", code: 'import { goiViGov } from "./cong-dan/api/vigov";' },
      { path: "./features/dang-nhap/goi-may-chu.ts", code: 'import { x } from "../../cong-dan/api/phien";' },
      { path: "./features/tinh-nang/LienHeTinhNang.tsx", code: 'const c = await import("@/cong-dan/api/vigov");' },
      { path: "./lib/launch-params.ts", code: 'require("../cong-dan/api/vigov");' },
    ];
    for (const tep of VI_PHAM) {
      expect(
        nhapClientViGovTuNgoai([tep]).length,
        `client ViGov lọt từ: ${tep.path} — ${tep.code}`,
      ).toBe(1);
    }

    // Từ bên TRONG nửa nhà nước thì được — nếu không thì chính nửa ấy không gọi nổi máy chủ của
    // mình, và một dây bẫy chặn cả mã sản phẩm là một dây bẫy sắp bị ai đó tắt.
    expect(
      nhapClientViGovTuNgoai([
        { path: "./cong-dan/man/TrangXa.tsx", code: 'import { goiViGov } from "../api/vigov";' },
      ]),
    ).toEqual([]);
  });

  /**
   * ⚠ HAI THƯ MỤC TÊN `api`, VÀ CHÚNG KHÔNG ĐƯỢC GỘP — 22/09/2026.
   *
   *   `./api/` (mới) là client của `vihat-miniapp`, backend THƯƠNG MẠI: phiên đăng nhập và bề mặt
   *   yêu cầu tư vấn. `./cong-dan/api/` là client của ViGov, nửa NHÀ NƯỚC, chưa có tệp nào.
   *
   *   Hai cái tên giống nhau trong một cây mã là chỗ người đọc sau này tự kết luận "chắc là một
   *   chỗ" rồi chuyển một tệp sang cho gọn. Ca này ghim cả hai vế: `./api/` thuộc nửa thương mại,
   *   và ràng buộc nặng nhất của tệp này — cấm MỌI tệp ngoài `./cong-dan/` nhập client ViGov —
   *   KHÔNG bị nới theo, kể cả cho `./api/`.
   */
  it("`./api/` là nửa THƯƠNG MẠI, và nó vẫn KHÔNG được nhập client ViGov", () => {
    expect(khuCua("./api/goi-may-chu.ts")).toBe("thuong-mai");
    expect(khuCua("./api/hop-dong-yeu-cau.ts")).toBe("thuong-mai");
    // Và tiền tố `./api/` KHÔNG vô tình phủ lên `./cong-dan/api/`.
    expect(khuCua("./cong-dan/api/vigov.ts")).toBe("nha-nuoc");
    expect(
      nhapClientViGovTuNgoai([
        { path: "./api/goi-may-chu.ts", code: 'import { x } from "../cong-dan/api/vigov";' },
      ]).length,
      "client ViGov lọt từ chính tệp gọi mạng của nửa thương mại",
    ).toBe(1);
  });
});

/* =============================================================================================
   RÀNG BUỘC 3b — KHÔNG LƯU TRỮ ĐỊNH DANH, Ở CẢ HAI NỬA
   ============================================================================================= */

/**
 * MỘT BUNDLE LÀ MỘT ORIGIN, NÊN HAI NỬA DÙNG CHUNG MỌI KHO LƯU TRỮ — theo đúng cấu tạo, không
 * sửa được bằng cách viết cẩn thận.
 *
 *   `localStorage` mà nửa thương mại ghi thì nửa nhà nước ĐỌC ĐƯỢC, và ngược lại. Không có
 *   `partition`, không có namespace nào của nền tảng ngăn được — chúng là một `Storage` duy nhất
 *   của một origin duy nhất. Nghĩa là một dòng "lưu tạm số điện thoại cho tiện" ở màn bán hàng
 *   đặt số điện thoại của một người thật vào đúng chỗ mà mã của kênh công dân đọc ra được, và
 *   ngược lại một `phien` của kênh công dân nằm ở chỗ mã thương mại đọc ra được.
 *
 *   Thêm một lớp nữa: thiết bị cho mượn được. Một phiếu phiên ghi xuống máy sống sót qua việc
 *   đóng app, nên người mượn máy tiếp theo mở app ra là đã đăng nhập sẵn thành người khác.
 *
 * ĐỦ BA API, KHÔNG NỚI: `localStorage` · `sessionStorage` · `IndexedDB`. Lệnh cấm trong
 * `phase1-collects-nothing.test.ts` là lệnh cấm của giai đoạn 1 và KHÔNG bị gỡ; lệnh cấm ở đây
 * đứng trên một lý do khác (hai nửa, một origin) và bắt thêm những hình thức gọi IndexedDB mà
 * một cái tên trần không thấy.
 */
const KHO_LUU_TRU =
  /\blocalStorage\b|\bsessionStorage\b|\bindexedDB\b|\bIDBFactory\b|\bIDBDatabase\b|\bIDBOpenDBRequest\b|\bIDBTransaction\b|\bIDBObjectStore\b|\bdocument\s*\.\s*cookie\b/;

function luuXuongMay(tep: readonly TepNguon[]): string[] {
  return tep.filter((f) => KHO_LUU_TRU.test(f.code)).map((f) => f.path);
}

describe("3b — không nửa nào ghi định danh xuống thiết bị", () => {
  it("không tệp sản xuất nào chạm localStorage · sessionStorage · IndexedDB", () => {
    expect(
      luuXuongMay(TEP_SAN_XUAT),
      "một nửa vừa ghi trạng thái xuống máy. Hai nửa dùng CHUNG một origin, nên thứ ghi ra đọc " +
        "được từ nửa kia; và một thiết bị cho mượn được thì thứ ghi ra sống sót qua người dùng " +
        "tiếp theo. Phiếu phiên, số điện thoại và xã đã chọn sống trong `useState` — mất khi đóng " +
        "app, đúng như chính sách quyền riêng tư đang khai.",
    ).toEqual([]);
  });

  it("bắt được cả ba API, ở mọi hình thức gọi — kể cả hình thức không gõ thẳng cái tên", () => {
    // MỘT PHÉP KIỂM VỀ CHÍNH DÂY BẪY. Lệnh cấm cũ khớp cái tên trần `indexedDB`, và `indexedDB`
    // là thứ duy nhất của ba API ấy có nhiều đường đi tới: một `IDBOpenDBRequest` nhận từ một
    // hàm khác, một `IDBTransaction` truyền vào. Bắt thêm họ tên `IDB*` thì không hình thức nào
    // đi vòng mà không gõ ra một trong những cái tên này.
    const LOT_QUA_NEU_KHONG_CANH = [
      'localStorage.setItem("phien", t);',
      "window.localStorage.removeItem(k);",
      'sessionStorage.setItem("so", so);',
      "globalThis.sessionStorage.clear();",
      'indexedDB.open("phien");',
      "self.indexedDB.deleteDatabase(t);",
      "function mo(db: IDBDatabase) { return db; }",
      "const y: IDBOpenDBRequest = r;",
      "function ghi(t: IDBTransaction) {}",
      "function kho(s: IDBObjectStore) {}",
      "const f: IDBFactory = g;",
      'document.cookie = "phien=" + token;',
    ];
    for (const dong of LOT_QUA_NEU_KHONG_CANH) {
      expect(
        luuXuongMay([{ path: "./features/tinh-nang/zalo-api.ts", code: dong }]),
        `lệnh cấm lưu trữ không bắt được: ${dong}`,
      ).toEqual(["./features/tinh-nang/zalo-api.ts"]);
    }

    // KHÔNG MIỄN CHO NỬA NÀO — kể cả nửa nhà nước, nơi "lưu phiên cho đỡ phải đăng nhập lại" là
    // câu sẽ được nói ra trước tiên, và là đúng câu làm một thiết bị cho mượn thành một tài khoản
    // cho mượn.
    expect(luuXuongMay([{ path: "./cong-dan/phien.ts", code: 'localStorage.setItem("p", t);' }])).toEqual([
      "./cong-dan/phien.ts",
    ]);

    // Và không kêu oan ở thứ chỉ TRÔNG giống: một biến tên `luuTam` trong bộ nhớ không phải kho
    // lưu trữ của trình duyệt, và một dây bẫy kêu oan là một dây bẫy sắp bị tắt.
    for (const dong of ["const [luuTam, datLuuTam] = useState(null);", "const kho = new Map();"]) {
      expect(luuXuongMay([{ path: "./App.tsx", code: dong }]), `kêu oan ở: ${dong}`).toEqual([]);
    }
  });
});

/* =============================================================================================
   RÀNG BUỘC 3c — MỖI LỜI GỌI SDK KHAI MỤC ĐÍCH TẠI CHỖ
   ============================================================================================= */

/**
 * `zalo-api.ts` là tệp DUY NHẤT chạm `zmp-sdk` (`phase1-collects-nothing.test.ts` giữ điều đó),
 * nên nó cũng là tệp duy nhất có thể khai đủ. Ca dưới đối chiếu bảng khai với chính mã nguồn ấy:
 * mỗi `sdk.<tên>(` phải có một dòng trong `KHAI_BAO_LOI_GOI`, và mỗi dòng phải trỏ về một lời gọi
 * có thật.
 *
 * ĐỌC MÃ NGUỒN CHỨ KHÔNG ĐỌC MỘT DANH SÁCH THỨ HAI: một danh sách chép tay chỉ mô tả cái người
 * chép NHỚ, và nó đứng yên trong lúc mã đi tiếp. Màn "Quản lý quyền" đọc cùng một bảng, nên màn
 * hình ấy không bao giờ nói ít hơn thứ app thật sự gọi.
 */
const MA_ZALO_API = boChuThich(RAW_SOURCES["./features/tinh-nang/zalo-api.ts"] ?? "");

function loiGoiTrongMa(ma: string): string[] {
  return [...new Set([...ma.matchAll(/\bsdk\s*\.\s*([A-Za-z][A-Za-z0-9_]*)\s*\(/g)].map((m) => m[1]!))];
}

describe("3c — mỗi lời gọi nền tảng khai mục đích tại chỗ", () => {
  it("đọc được mã nguồn của tệp SDK — nếu không, mọi ca dưới xanh vì lý do sai", () => {
    expect(MA_ZALO_API.length, "không đọc được `features/tinh-nang/zalo-api.ts`").toBeGreaterThan(
      1000,
    );
    expect(loiGoiTrongMa(MA_ZALO_API).length).toBeGreaterThanOrEqual(10);
  });

  /**
   * ⚠ CA NÀY RA ĐỜI TỪ MỘT LẦN THỬ ĐỘT BIẾN **THẤT BẠI** — 21/09/2026. Ghi lại vì đó là lý do nó
   * tồn tại, và vì không có lần thử ấy thì lỗ hổng dưới đây đã đi vào kho như một dây bẫy "xanh".
   *
   *   Phép đối chiếu ở ca ngay dưới tìm `sdk.<tên>(`. Lần thử đột biến đầu tiên viết lời gọi mới
   *   dưới dạng `(sdk as unknown as { openChat: … }).openChat()` — và phép đối chiếu **KHÔNG BẮT**:
   *   giữa `sdk` và tên hàm có một phép ép kiểu, nên chuỗi `sdk.openChat(` không hề xuất hiện.
   *   Một lời gọi nền tảng mới đi vào app mà bảng khai không biết, và màn Quản lý quyền nói thiếu.
   *
   *   Nới biểu thức để "bắt cả ép kiểu" là đổi một phép kiểm chính xác lấy một phép kiểm đoán mò.
   *   Cách đúng là cấm chính cái hình dạng che giấu: trong tệp này, `sdk` không được ép kiểu và
   *   không được gán sang tên khác. Không có hai hình dạng ấy thì `sdk.<tên>(` là hình dạng DUY
   *   NHẤT một lời gọi nền tảng viết ra được, và phép đối chiếu bên dưới là đầy đủ.
   */
  it("không ai ép kiểu hay đổi tên `sdk` — đó là hai cách một lời gọi trốn khỏi bảng khai", () => {
    expect(
      MA_ZALO_API,
      "một phép ép kiểu trên `sdk` giấu lời gọi khỏi phép đối chiếu bên dưới. Nếu `zmp-sdk` khai " +
        "thiếu một API, khai bổ sung kiểu ấy ở một chỗ có tên, đừng ép kiểu ngay tại lời gọi.",
    ).not.toMatch(/\bsdk\b\s*(?:as\b|satisfies\b)/);
    expect(
      MA_ZALO_API,
      "`sdk` vừa được gán sang một tên khác. Lời gọi qua tên ấy không mang chuỗi `sdk.` nào, nên " +
        "nó không có mặt trong phép đối chiếu với bảng khai.",
    ).not.toMatch(/(?:const|let|var)\s+[A-Za-z_$][\w$]*\s*(?::[^=\n]*)?=\s*sdk\s*[;,)]/);
  });

  it("mỗi lời gọi trong mã có ĐÚNG MỘT dòng khai, và mỗi dòng khai trỏ về một lời gọi có thật", () => {
    const trong_ma = loiGoiTrongMa(MA_ZALO_API).sort();
    const da_khai = KHAI_BAO_LOI_GOI.map((k) => k.api).sort();

    expect(
      trong_ma.filter((ten) => !da_khai.includes(ten)),
      "một lời gọi nền tảng không có dòng khai. Thêm nó vào `KHAI_BAO_LOI_GOI` — màn Quản lý " +
        "quyền đọc từ bảng ấy, nên một lời gọi không khai là một quyền app dùng mà không màn nào " +
        "nói ra, trong một hồ sơ đang xin đúng những quyền đó.",
    ).toEqual([]);

    expect(
      da_khai.filter((ten) => !trong_ma.includes(ten)),
      "một dòng khai không còn lời gọi nào ứng với nó. Gỡ dòng ấy — một màn hình liệt kê một " +
        "quyền app KHÔNG dùng là khai thừa với người duyệt.",
    ).toEqual([]);

    expect(new Set(da_khai).size, "hai dòng khai cùng một lời gọi").toBe(da_khai.length);
  });

  it("mỗi dòng khai nói ĐỦ: nửa nào · màn nào · tính năng nào · để làm gì", () => {
    for (const k of KHAI_BAO_LOI_GOI) {
      expect(["thuong-mai", "nha-nuoc", "ca-hai"], `${k.api}: nửa không hợp lệ`).toContain(k.nua);
      expect(k.man.trim().length, `${k.api}: không khai màn nào dùng`).toBeGreaterThan(0);
      expect(k.tinh_nang.trim().length, `${k.api}: không khai tính năng nào dùng`).toBeGreaterThan(0);
      // Ngưỡng 40 ký tự, không phải "khác rỗng": một chữ "để đăng nhập" lọt qua mọi phép kiểm
      // độ dài > 0 và không nói gì với người đọc màn Quản lý quyền — mà người đọc ấy là người
      // đang quyết định có chia sẻ dữ liệu của mình hay không.
      expect(
        k.de_lam_gi.trim().length,
        `${k.api}: câu "để làm gì" quá ngắn để nói được gì cho người dùng`,
      ).toBeGreaterThan(40);
    }
  });

  it("ba quyền đang xin Zalo đều có mặt trong bảng khai", () => {
    // `getPhoneNumber` · `getLocation` · `scanQRCode` là ba quyền hồ sơ này xin. Thiếu một dòng
    // khai cho một trong ba là nộp một hồ sơ xin quyền mà không nói được nó dùng vào việc gì.
    for (const ten of ["getPhoneNumber", "getLocation", "scanQRCode"]) {
      const khai = KHAI_BAO_LOI_GOI.find((k) => k.api === ten);
      expect(khai, `không có dòng khai cho quyền đang xin: ${ten}`).toBeDefined();
    }
  });

  it("mọi lời gọi đưa dữ liệu ra khỏi máy đều nói ra điều đó", () => {
    // Cột `roi_khoi_may` rỗng nghĩa là KHÔNG CÓ GÌ rời khỏi máy — một khẳng định, không phải một
    // ô chưa điền. Ba lời gọi của luồng đăng nhập là ba lời gọi duy nhất có dữ liệu đi ra, và
    // chúng phải nói ra; ca này đỏ lên nếu ai đó làm rỗng một trong ba.
    for (const ten of ["getPhoneNumber", "getAccessToken"]) {
      const khai = KHAI_BAO_LOI_GOI.find((k) => k.api === ten)!;
      expect(
        khai.roi_khoi_may.trim().length,
        `${ten} gửi một mã tới máy chủ nhưng bảng khai nói không có gì rời khỏi máy`,
      ).toBeGreaterThan(0);
    }
    // Và chiều ngược lại: `getLocation` KHÔNG gửi gì đi — mã vị trí ở lại trên máy, và màn hình
    // nói thẳng rằng vì thế nó chưa xếp được văn phòng theo khoảng cách.
    expect(
      KHAI_BAO_LOI_GOI.find((k) => k.api === "getLocation")!.roi_khoi_may,
      "bảng khai nói mã vị trí rời khỏi máy — nếu đúng thì chính sách quyền riêng tư đang thiếu một mục",
    ).toBe("");
  });

  it("nửa nhà nước chưa khai lời gọi riêng nào — và bảng nói ra nó thừa hưởng những gì", () => {
    // Quyền cấp theo App ID: ngày `src/cong-dan/` có tệp đầu tiên, nó thừa hưởng NGUYÊN VẸN mọi
    // quyền mà nửa thương mại đã xin được, không ai cấp lại. Ca này ghim tình trạng hôm nay để
    // lần khai đầu tiên của nửa ấy là một thay đổi có người đọc, chứ không phải một dòng lặng lẽ.
    expect(
      KHAI_BAO_LOI_GOI.filter((k) => k.nua === "nha-nuoc").map((k) => k.api),
      "nửa nhà nước vừa khai một lời gọi riêng — cập nhật ca này cùng lúc, và kiểm lại xem màn " +
        "Quản lý quyền có còn nói đúng việc nửa ấy dùng quyền vào đâu không",
    ).toEqual([]);
    expect(
      KHAI_BAO_LOI_GOI.filter((k) => k.nua === "ca-hai").length,
      "không lời gọi nào được khai là dùng chung — nhưng quyền cấp theo App ID thì luôn dùng chung",
    ).toBeGreaterThan(0);
  });
});
