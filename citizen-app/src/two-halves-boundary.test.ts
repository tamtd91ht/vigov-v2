/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import { KHAI_BAO_LOI_GOI } from "./features/tinh-nang/zalo-api";

/**
 * BA RÀNG BUỘC KIẾN TRÚC CỦA MỘT MINI APP MANG HAI NỬA NGHIỆP VỤ — viết TRƯỚC khi nửa thứ hai
 * (`src/citizen/`) có tệp nghiệp vụ đầu tiên.
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
 * ⚠ VÌ SAO VIẾT BÂY GIỜ, KHI `src/citizen/` CÒN RỖNG:
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
function stripComments(code: string): string {
  return code.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
}

type SourceFile = { path: string; code: string };

const PRODUCTION_FILES: readonly SourceFile[] = Object.entries(RAW_SOURCES)
  .filter(([path]) => !path.includes(".test."))
  .map(([path, code]) => ({ path, code: stripComments(code) }));

/* =============================================================================================
   RÀNG BUỘC 3a — RANH GIỚI HAI NỬA, CẤM CẢ HAI CHIỀU
   ============================================================================================= */

type Half = "thuong-mai" | "nha-nuoc";

/**
 * HAI NỬA, KHAI BẰNG TIỀN TỐ ĐƯỜNG DẪN — và cái thứ ba, `trung-lap`, là thứ làm bảng này trung
 * thực.
 *
 * `App.tsx`, `main.tsx`, `components/` và `lib/` là **lớp vỏ**: chúng không phục vụ khách hàng
 * doanh nghiệp và cũng không phục vụ công dân, chúng dựng khung và đọc tham số mở app. Xếp chúng
 * vào một nửa sẽ cấm nửa kia dùng thanh tab — tức là cấm một thứ không có hại, và một dây bẫy
 * cấm thứ vô hại là một dây bẫy sắp bị tắt.
 *
 * `features/kham-pha/` cũng ở đây, và đó là một quyết định có hạn: lớp khám phá nói chuyện
 * xã/phường nên nó THUỘC VỀ nửa nhà nước về nghiệp vụ, nhưng hôm nay nó chỉ là vỏ màn xác nhận xã,
 * chưa gọi máy chủ nào. Ngày nó nói chuyện với máy chủ thật, nó chuyển sang `./citizen/` — và ca
 * "mọi tệp phải thuộc đúng một khu" ở dưới là thứ bắt người chuyển phải khai lại bảng này.
 * (`features/diagnostics/` từng đứng cạnh nó; thư mục ấy đã bị xoá 27/09/2026.)
 *
 * Tên khu (`thuong-mai` · `nha-nuoc` · `trung-lap`) là CHÍNH giá trị cột `nua` của `KHAI_BAO_LOI_GOI`
 * (nửa thương mại), nên chúng đổi cùng bảng ấy, không đổi riêng ở đây.
 */
const ZONES: Readonly<Record<Half | "trung-lap", readonly string[]>> = {
  "thuong-mai": [
    "./content/",
    "./features/company-intro/",
    "./features/tinh-nang/",
    "./features/log-in/",
    // Bộ chọn ba bước trên màn chủ (22/09/2026). Nó đứng trên `SOLUTIONS` — danh mục sản phẩm của
    // một doanh nghiệp — và không biết gì về xã/phường: nửa thương mại, không phải trung lập.
    "./features/solution-suggestion/",
    // BỀ MẶT YÊU CẦU (22/09/2026, giai đoạn B) — tư vấn, báo giá, đề nghị gọi lại.
    "./features/yeu-cau/",
    /**
     * ⚠ `./api/` LÀ CLIENT CỦA `vihat-miniapp`, KHÔNG PHẢI CLIENT CỦA ViGov — VÀ HAI THƯ MỤC TÊN
     * `api` TRONG CÙNG MỘT CÂY MÃ LÀ MỘT CÁI BẪY PHẢI NÓI RA, KHÔNG PHẢI MỘT SỰ TRÙNG TÊN.
     *
     *   `./api/`          — backend thương mại `vihat-miniapp`: phiên đăng nhập, yêu cầu tư vấn.
     *                       NỬA THƯƠNG MẠI.
     *   `./citizen/api/` — client API của ViGov. NỬA NHÀ NƯỚC, và `VIGOV_CLIENT_DIR`
     *                       ở dưới cấm MỌI tệp ngoài `./citizen/` nhập nó — kể cả lớp vỏ trung lập.
     *
     *   Hai ràng buộc ấy không đè lên nhau: tiền tố `./citizen/api/` dài hơn và không phải tiền tố
     *   của `./api/`. Ca "`./api/` là nửa thương mại, `./citizen/api/` vẫn bị cấm từ ngoài" ở cuối
     *   tệp giữ cho hai thứ ấy không bị ai gộp lại.
     */
    "./api/",
  ],
  "nha-nuoc": ["./citizen/"],
  "trung-lap": [
    "./App.tsx",
    "./main.tsx",
    "./components/",
    "./lib/",
    "./features/kham-pha/",
    // Shell pieces of the commune's OWN app (28/09/2026) — today only the feedback-draft store `CommuneApp`
    // injects (§3b). Neutral like `lib/`: it serves neither the commercial half nor the state half's
    // screens; and §3b pins that `App.tsx` is the only file importing it.
    "./commune-app/",
  ],
};

/**
 * CLIENT API CỦA ViGov — MỘT ĐƯỜNG DẪN, KHAI TỪ TRƯỚC KHI TỆP TỒN TẠI.
 *
 * Đây là ràng buộc NẶNG NHẤT trong tệp này, và nó gắt hơn ranh giới hai nửa ở trên một bậc:
 * ranh giới kia cấm nửa thương mại nhập nửa nhà nước; ràng buộc này cấm **mọi tệp bên ngoài
 * `./citizen/`** nhập client ViGov — kể cả lớp vỏ trung lập.
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
const VIGOV_CLIENT_DIR = "./citizen/api/";

/**
 * Tiền tố khớp một trong các khu đã khai.
 *
 * `${path}/` CŨNG ĐƯỢC THỬ: một lần nhập THƯ MỤC (`from "./citizen"`, đi qua `index.ts`) quy
 * về `./citizen`, không có dấu `/` cuối, và không khớp tiền tố `./citizen/` nào. Thiếu vế ấy thì
 * đúng cách `App.tsx` nhập kênh công dân từ 27/09/2026 là một điểm mù của cả ranh giới.
 */
function zoneOf(path: string): Half | "trung-lap" | null {
  const candidates = [path, `${path}/`];
  for (const [zone, prefixes] of Object.entries(ZONES)) {
    if (prefixes.some((t) => (t.endsWith("/") ? candidates.some((d) => d.startsWith(t)) : path === t))) {
      return zone as Half | "trung-lap";
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
function importedModuleNames(code: string): string[] {
  const out: string[] = [];
  for (const match of code.matchAll(/(?:\bfrom|\bimport|\brequire)\s*\(?\s*["'`]([^"'`]+)["'`]/g)) {
    out.push(match[1]!);
  }
  return out;
}

/**
 * Quy một tên mô-đun về đường dẫn trong `src/`, hoặc `null` nếu nó không trỏ vào `src/`.
 *
 * Hai hình dạng phải quy: tương đối (`../content/x`) và bí danh `@/…` (tsconfig `paths`). Gói
 * ngoài (`react`, `zmp-sdk`) trả `null` — chúng không thuộc nửa nào. (Bí danh biến thể `bien-the/…`
 * đã bị gỡ khỏi `vite.config.ts` 27/09/2026; một `bien-the/…` quay lại sẽ không dựng được.)
 */
function resolvePath(from_file: string, name: string): string | null {
  if (name.startsWith("@/")) return `./${name.slice(2)}`;
  if (!name.startsWith(".")) return null;

  const segments = from_file.split("/").slice(0, -1);
  for (const step of name.split("/")) {
    if (step === "." || step === "") continue;
    if (step === "..") segments.pop();
    else segments.push(step);
  }
  return segments.join("/");
}

/** Một lần nhập vượt ranh giới: tệp nào, nhập gì, từ khu nào sang khu nào. */
type ImportViolation = { from: string; to: string; zone_from: string; zone_to: string };

function crossBoundaryImports(files: readonly SourceFile[]): ImportViolation[] {
  const out: ImportViolation[] = [];
  for (const f of files) {
    const zone_from = zoneOf(f.path);
    if (zone_from === null) continue; // ca riêng ở dưới lo chuyện một tệp không thuộc khu nào
    for (const name of importedModuleNames(f.code)) {
      const to = resolvePath(f.path, name);
      if (to === null) continue;
      const zone_to = zoneOf(to);
      if (zone_to === null) continue;
      const crosses =
        (zone_from === "thuong-mai" && zone_to === "nha-nuoc") ||
        (zone_from === "nha-nuoc" && zone_to === "thuong-mai");
      if (crosses) out.push({ from: f.path, to, zone_from, zone_to });
    }
  }
  return out;
}

/** Nhập client ViGov từ bên ngoài nửa nhà nước — cấm cả với lớp vỏ trung lập. */
function vigovClientImportsFromOutside(files: readonly SourceFile[]): ImportViolation[] {
  const out: ImportViolation[] = [];
  for (const f of files) {
    if (f.path.startsWith("./citizen/")) continue;
    for (const name of importedModuleNames(f.code)) {
      const to = resolvePath(f.path, name);
      // `${to}/` vì cùng lý do với `zoneOf`: `from "./citizen/api"` là nhập THƯ MỤC client.
      if (to !== null && `${to}/`.startsWith(VIGOV_CLIENT_DIR)) {
        out.push({ from: f.path, to, zone_from: zoneOf(f.path) ?? "chưa khai", zone_to: "client ViGov" });
      }
    }
  }
  return out;
}

describe("3a — ranh giới hai nửa, cấm cả hai chiều", () => {
  it("quét cây mã THẬT — một lượt quét rỗng sẽ xanh vì lý do sai", () => {
    const paths = PRODUCTION_FILES.map((f) => f.path);
    expect(paths).toContain("./App.tsx");
    expect(paths).toContain("./content/company-profile.ts");
    expect(paths).toContain("./features/tinh-nang/zalo-api.ts");
    // Nửa nhà nước phải CÓ MẶT trong lượt quét kể cả khi chưa có nghiệp vụ. Một khu được canh mà
    // lượt quét không đọc tới thì "được canh" và "không tồn tại" là một.
    expect(
      paths.filter((p) => p.startsWith("./citizen/")).length,
      "nửa nhà nước không có tệp nào trong lượt quét — ranh giới đang canh một khoảng trống",
    ).toBeGreaterThan(0);
    // The renamed state-half directories are really read: a zone pinned to a directory the sweep no longer
    // sees is a zone guarding nothing.
    for (const dir of ["./citizen/api/", "./citizen/screens/", "./features/log-in/", "./features/solution-suggestion/"]) {
      expect(paths.some((p) => p.startsWith(dir)), `lượt quét không đọc tệp nào trong ${dir}`).toBe(true);
    }
    expect(paths.length).toBeGreaterThanOrEqual(15);
  });

  it("mọi tệp sản xuất thuộc ĐÚNG MỘT khu — thư mục mới buộc phải khai", () => {
    // Không có ca này thì một thư mục mới (`src/thanh-toan/`) rơi ra ngoài mọi tiền tố, và ranh
    // giới KHÔNG cấm nó nhập bất cứ thứ gì của bất cứ nửa nào — im lặng, vì nó không thuộc nửa
    // nào để mà vi phạm.
    const undeclared = PRODUCTION_FILES.filter((f) => zoneOf(f.path) === null).map((f) => f.path);
    expect(
      undeclared,
      "tệp không thuộc nửa thương mại, nửa nhà nước, hay lớp vỏ trung lập. Khai nó vào `ZONES` " +
        "— chọn khu là chọn nó được nhập gì và ai được nhập nó.",
    ).toEqual([]);
  });

  it("nửa thương mại KHÔNG nhập nửa nhà nước, và nửa nhà nước KHÔNG nhập nửa thương mại", () => {
    const violations = crossBoundaryImports(PRODUCTION_FILES);
    expect(
      violations.map((v) => `${v.from} (${v.zone_from}) -> ${v.to} (${v.zone_to})`),
      "một nửa vừa nhập tệp của nửa kia. Hai nửa chạy trong cùng một tiến trình, nên một lời gọi " +
        "hàm là tất cả những gì cần để phiên công dân với tới được từ màn bán hàng — và ngược lại, " +
        "để nội dung doanh nghiệp đi vào một màn hình công dân đang chờ một cơ quan nhà nước.",
    ).toEqual([]);
  });

  it("KHÔNG tệp nào ngoài `./citizen/` nhập client API của ViGov — kể cả lớp vỏ", () => {
    const violations = vigovClientImportsFromOutside(PRODUCTION_FILES);
    expect(
      violations.map((v) => `${v.from} (${v.zone_from}) -> ${v.to}`),
      `client ViGov chỉ được gọi từ bên trong \`./citizen/\`. Một đường nhập từ ngoài — nhất là ` +
        `từ \`App.tsx\` — cho nửa thương mại một đường tới tuyến phiên công dân.`,
    ).toEqual([]);
  });

  /* ------------------------------------------------------------------------------------------
     THỬ ĐỘT BIẾN DỰNG SẴN — phần trả lời câu "phép kiểm trên còn sống không".

     Ba ca trên quét một cây mã hôm nay SẠCH ở cả ba ràng buộc, nên cả ba đều xanh, và một dây
     bẫy xanh không nói lên gì. Ca dưới cho từng hàm kiểm ăn một vi phạm dựng sẵn.
     ------------------------------------------------------------------------------------------ */

  it("bắt được nhập vượt ranh giới ở CẢ HAI CHIỀU, mọi hình thức nhập", () => {
    const VIOLATIONS: readonly SourceFile[] = [
      // thương mại -> nhà nước
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { X } from "../../citizen/phien";' },
      { path: "./content/company-profile.ts", code: 'const x = await import("../citizen/xa");' },
      { path: "./features/tinh-nang/zalo-api.ts", code: 'const x = require("@/citizen/phien");' },
      // Nhập THƯ MỤC (qua `index.ts`) cũng là nhập nửa nhà nước — không có dấu `/` cuối không phải lối tắt.
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { CitizenChannel } from "../../citizen";' },
      // The two renamed commercial directories reach into the state half's renamed ones.
      { path: "./features/log-in/SessionIssuer.tsx", code: 'import { getVigovSession } from "../../citizen/api/vigov-session";' },
      { path: "./features/solution-suggestion/mapping.ts", code: 'import { COMMUNE_APP_UI } from "../../citizen/screens/copy";' },
      // nhà nước -> thương mại
      { path: "./citizen/CommuneHome.tsx", code: 'import { COMPANY } from "../content/company-profile";' },
      { path: "./citizen/api/vigov.ts", code: 'import { thanYeuCau } from "@/features/log-in/contract";' },
      { path: "./citizen/screens/Gui.tsx", code: "import { KhoiDangNhap } from '../../features/tinh-nang/index';" },
      { path: "./citizen/screens/CommuneHome.tsx", code: 'import { issueSession } from "../../features/log-in/server-calls";' },
      { path: "./citizen/screens/copy.ts", code: 'import { WORDS } from "../../features/solution-suggestion/SolutionSuggestionScreen";' },
    ];
    for (const file of VIOLATIONS) {
      expect(
        crossBoundaryImports([file]).length,
        `ranh giới không bắt được: ${file.path} — ${file.code}`,
      ).toBe(1);
    }

    // Và nó KHÔNG kêu oan. Một dây bẫy kêu sai chỗ bị tắt nhanh y như một dây bẫy câm.
    const VALID: readonly SourceFile[] = [
      { path: "./App.tsx", code: 'import { COMPANY } from "./content/company-profile";' },
      { path: "./App.tsx", code: 'import { X } from "./citizen/index";' },
      { path: "./App.tsx", code: 'import { CitizenChannel } from "./citizen";' },
      { path: "./features/tinh-nang/zalo-api.ts", code: 'const sdk = await import("zmp-sdk");' },
      { path: "./features/company-intro/HomeScreen.tsx", code: 'import { useState } from "react";' },
      { path: "./citizen/index.ts", code: 'import { paramsOf } from "../lib/launch-params";' },
    ];
    for (const file of VALID) {
      expect(crossBoundaryImports([file]), `ranh giới kêu oan ở: ${file.code}`).toEqual([]);
    }
  });

  it("bắt được nhập client ViGov từ NỬA THƯƠNG MẠI và từ LỚP VỎ", () => {
    const VIOLATIONS: readonly SourceFile[] = [
      { path: "./App.tsx", code: 'import { callVigov } from "./citizen/api/vigov";' },
      { path: "./features/log-in/server-calls.ts", code: 'import { x } from "../../citizen/api/phien";' },
      { path: "./features/tinh-nang/LienHeTinhNang.tsx", code: 'const c = await import("@/citizen/api/vigov");' },
      { path: "./lib/launch-params.ts", code: 'require("../citizen/api/vigov");' },
      { path: "./App.tsx", code: 'import { callVigov } from "./citizen/api";' },
      // The real client file under its new name, from the shell.
      { path: "./App.tsx", code: 'import { submitReport } from "./citizen/api/vigov-client";' },
    ];
    for (const file of VIOLATIONS) {
      expect(
        vigovClientImportsFromOutside([file]).length,
        `client ViGov lọt từ: ${file.path} — ${file.code}`,
      ).toBe(1);
    }

    // Từ bên TRONG nửa nhà nước thì được — nếu không thì chính nửa ấy không gọi nổi máy chủ của
    // mình, và một dây bẫy chặn cả mã sản phẩm là một dây bẫy sắp bị ai đó tắt.
    expect(
      vigovClientImportsFromOutside([
        { path: "./citizen/screens/CommuneHome.tsx", code: 'import { callVigov } from "../api/vigov";' },
      ]),
    ).toEqual([]);
  });

  /**
   * ⚠ HAI THƯ MỤC TÊN `api`, VÀ CHÚNG KHÔNG ĐƯỢC GỘP — 22/09/2026.
   *
   *   `./api/` (mới) là client của `vihat-miniapp`, backend THƯƠNG MẠI: phiên đăng nhập và bề mặt
   *   yêu cầu tư vấn. `./citizen/api/` là client của ViGov, nửa NHÀ NƯỚC.
   *
   *   Hai cái tên giống nhau trong một cây mã là chỗ người đọc sau này tự kết luận "chắc là một
   *   chỗ" rồi chuyển một tệp sang cho gọn. Ca này ghim cả hai vế: `./api/` thuộc nửa thương mại,
   *   và ràng buộc nặng nhất của tệp này — cấm MỌI tệp ngoài `./citizen/` nhập client ViGov —
   *   KHÔNG bị nới theo, kể cả cho `./api/`.
   */
  it("`./api/` là nửa THƯƠNG MẠI, và nó vẫn KHÔNG được nhập client ViGov", () => {
    expect(zoneOf("./api/goi-may-chu.ts")).toBe("thuong-mai");
    expect(zoneOf("./api/hop-dong-yeu-cau.ts")).toBe("thuong-mai");
    // Và tiền tố `./api/` KHÔNG vô tình phủ lên `./citizen/api/`.
    expect(zoneOf("./citizen/api/vigov.ts")).toBe("nha-nuoc");
    expect(
      vigovClientImportsFromOutside([
        { path: "./api/goi-may-chu.ts", code: 'import { x } from "../citizen/api/vigov";' },
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
const STORAGE_API =
  /\blocalStorage\b|\bsessionStorage\b|\bindexedDB\b|\bIDBFactory\b|\bIDBDatabase\b|\bIDBOpenDBRequest\b|\bIDBTransaction\b|\bIDBObjectStore\b|\bdocument\s*\.\s*cookie\b/;

/**
 * THE ONE-FILE EXCEPTION — 28/09/2026, the first this constraint has ever had.
 *
 *   WHY: ADR 0050 #7, owner decision 28/09/2026 ("3 điểm còn lại cũng theo require nhé"): the commune's
 *   OWN app keeps a feedback being written on the phone, as the requirements prototype does.
 *
 *   WHY THE "ONE ORIGIN" ARGUMENT ABOVE DOES NOT APPLY TO IT: the commune's own app is a SEPARATE Zalo App
 *   ID (`--vao-thang`, `FIXED_COMMUNE_DOMAIN !== null`, `App.tsx` renders `CommuneApp`) — a separate origin, with no
 *   commercial half in it and no privacy policy published yet (ADR 0047). The SHARED ViHAT app, whose two
 *   halves DO share one origin, still writes nothing: its promise "không lưu gì xuống máy" stands, and the
 *   cases below pin the three things that keep it — (1) `localStorage` in this ONE file only, nothing else
 *   of the pattern even here; (2) `App.tsx` is the only importer of that file; (3) `SharedApp` never
 *   references the store (and the store itself opens no storage when `FIXED_COMMUNE_DOMAIN === null`).
 *
 *   The state half (`./citizen/**`) still touches no storage API: it receives the store as a prop, the way
 *   it receives `getName` / `getSceneLocation`.
 */
const DRAFT_STORE_FILE = "./commune-app/feedback-draft-store.ts";

function storesOnDevice(files: readonly SourceFile[]): string[] {
  return files
    .filter((f) => STORAGE_API.test(f.code))
    .filter((f) => f.path !== DRAFT_STORE_FILE || STORAGE_API.test(f.code.replace(/\blocalStorage\b/g, " ")))
    .map((f) => f.path);
}

/** Every production file that imports the draft store — the answer must be exactly `App.tsx`. */
function draftStoreImporters(files: readonly SourceFile[]): string[] {
  return files
    .filter((f) =>
      importedModuleNames(f.code).some((name) => {
        const to = resolvePath(f.path, name);
        return to !== null && (to === DRAFT_STORE_FILE || `${to}.ts` === DRAFT_STORE_FILE);
      }),
    )
    .map((f) => f.path);
}

/** The body of `SharedApp` in `App.tsx` source — from its declaration to the end of the file. */
function sharedAppBody(appSource: string): string {
  const start = appSource.indexOf("function SharedApp(");
  return start < 0 ? "" : appSource.slice(start);
}

/** The body of `CommuneApp` — from its declaration up to `SharedApp`. */
function communeAppBody(appSource: string): string {
  const start = appSource.indexOf("export function CommuneApp(");
  const end = appSource.indexOf("function SharedApp(");
  return start < 0 || end < start ? "" : appSource.slice(start, end);
}

const DRAFT_STORE_REFERENCE = /feedbackDraftStore|feedback-draft-store|createFeedbackDraftStore|draftStore/;

describe("3b — không nửa nào ghi định danh xuống thiết bị", () => {
  it("không tệp sản xuất nào chạm localStorage · sessionStorage · IndexedDB (trừ đúng một tệp nháp, chỉ localStorage)", () => {
    expect(
      storesOnDevice(PRODUCTION_FILES),
      "một nửa vừa ghi trạng thái xuống máy. Hai nửa dùng CHUNG một origin, nên thứ ghi ra đọc " +
        "được từ nửa kia; và một thiết bị cho mượn được thì thứ ghi ra sống sót qua người dùng " +
        "tiếp theo. Phiếu phiên, số điện thoại và xã đã chọn sống trong `useState` — mất khi đóng " +
        "app, đúng như chính sách quyền riêng tư đang khai. Ngoại lệ duy nhất: `localStorage` trong " +
        `\`${DRAFT_STORE_FILE}\` (nháp của app riêng, ADR 0050 #7).`,
    ).toEqual([]);
  });

  it("bắt được cả ba API, ở mọi hình thức gọi — kể cả hình thức không gõ thẳng cái tên", () => {
    // MỘT PHÉP KIỂM VỀ CHÍNH DÂY BẪY. Lệnh cấm cũ khớp cái tên trần `indexedDB`, và `indexedDB`
    // là thứ duy nhất của ba API ấy có nhiều đường đi tới: một `IDBOpenDBRequest` nhận từ một
    // hàm khác, một `IDBTransaction` truyền vào. Bắt thêm họ tên `IDB*` thì không hình thức nào
    // đi vòng mà không gõ ra một trong những cái tên này.
    const UNGUARDED_ESCAPES = [
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
    for (const line of UNGUARDED_ESCAPES) {
      expect(
        storesOnDevice([{ path: "./features/tinh-nang/zalo-api.ts", code: line }]),
        `lệnh cấm lưu trữ không bắt được: ${line}`,
      ).toEqual(["./features/tinh-nang/zalo-api.ts"]);
    }

    // KHÔNG MIỄN CHO NỬA NÀO — kể cả nửa nhà nước, nơi "lưu phiên cho đỡ phải đăng nhập lại" là
    // câu sẽ được nói ra trước tiên, và là đúng câu làm một thiết bị cho mượn thành một tài khoản
    // cho mượn.
    expect(storesOnDevice([{ path: "./citizen/phien.ts", code: 'localStorage.setItem("p", t);' }])).toEqual([
      "./citizen/phien.ts",
    ]);

    // THE ONE-FILE EXCEPTION (28/09/2026) IS ONE FILE AND ONE API. The same call the draft store makes, in
    // any other file — the screen that uses the store, the rest of the state half, the commercial half, the
    // shell, a sibling in `commune-app/`, a `.tsx` twin — is still red.
    const SAME_CALL = 'localStorage.setItem("vigov.feedback.draft.v1", JSON.stringify(d));';
    for (const path of [
      "./citizen/screens/CommuneAppReports.tsx",
      "./citizen/screens/CommuneHome.tsx",
      "./citizen/api/vigov-client.ts",
      "./features/log-in/session-store.tsx",
      "./features/yeu-cau/OGhiChu.tsx",
      "./App.tsx",
      "./lib/fixed-commune.ts",
      "./commune-app/other-store.ts",
      "./commune-app/feedback-draft-store.tsx",
    ]) {
      expect(storesOnDevice([{ path, code: SAME_CALL }]), `localStorage lọt ở: ${path}`).toEqual([path]);
    }
    // In the allowed file itself: `localStorage` passes, every OTHER storage API is still red.
    expect(storesOnDevice([{ path: DRAFT_STORE_FILE, code: SAME_CALL }])).toEqual([]);
    for (const line of ['sessionStorage.setItem("so", so);', 'indexedDB.open("nhap");', 'document.cookie = "p=" + t;']) {
      expect(storesOnDevice([{ path: DRAFT_STORE_FILE, code: `${SAME_CALL}\n${line}` }]), `lọt ở tệp nháp: ${line}`).toEqual([
        DRAFT_STORE_FILE,
      ]);
    }

    // Và không kêu oan ở thứ chỉ TRÔNG giống: một biến tên `luuTam` trong bộ nhớ không phải kho
    // lưu trữ của trình duyệt, và một dây bẫy kêu oan là một dây bẫy sắp bị tắt.
    for (const line of ["const [luuTam, datLuuTam] = useState(null);", "const kho = new Map();"]) {
      expect(storesOnDevice([{ path: "./App.tsx", code: line }]), `kêu oan ở: ${line}`).toEqual([]);
    }
  });

  it("the draft store exists where the exception points, and ONLY App.tsx imports it", () => {
    // An exception pointing at a file the sweep never reads exempts nothing — and the day that file is
    // renamed, the exception silently points at nothing.
    expect(PRODUCTION_FILES.map((f) => f.path)).toContain(DRAFT_STORE_FILE);
    expect(
      draftStoreImporters(PRODUCTION_FILES),
      "a file other than App.tsx imports the feedback-draft store. Only the commune's own app may hold it; " +
        "a commercial screen or the state half reaching it puts device storage back into the shared app.",
    ).toEqual(["./App.tsx"]);

    // Must-still-catch: an import from anywhere else, in any form.
    for (const file of [
      { path: "./features/yeu-cau/TuVanBaoGiaScreen.tsx", code: 'import { feedbackDraftStore } from "../../commune-app/feedback-draft-store";' },
      { path: "./citizen/screens/CommuneAppReports.tsx", code: 'const s = await import("../../commune-app/feedback-draft-store");' },
      { path: "./lib/launch-params.ts", code: 'import { feedbackDraftStore } from "@/commune-app/feedback-draft-store";' },
    ]) {
      expect(draftStoreImporters([file]), `nhập kho nháp lọt từ: ${file.path}`).toEqual([file.path]);
    }
  });

  it("SharedApp (the shared ViHAT app) never receives the draft store; CommuneApp does", () => {
    const app = RAW_SOURCES["./App.tsx"] ?? "";
    const shared = sharedAppBody(stripComments(app));
    const commune = communeAppBody(stripComments(app));
    // Both slices must be non-empty — an empty slice is green for the wrong reason.
    expect(shared.length, "không tìm thấy `function SharedApp(` trong App.tsx").toBeGreaterThan(500);
    expect(commune.length, "không tìm thấy `export function CommuneApp(` trong App.tsx").toBeGreaterThan(0);
    expect(
      shared,
      "SharedApp references the feedback-draft store. The shared app promises 'không lưu gì xuống máy', and " +
        "its two halves share one origin — the draft belongs to the commune's own App ID only (ADR 0050 #7).",
    ).not.toMatch(DRAFT_STORE_REFERENCE);
    expect(commune, "CommuneApp no longer injects the draft store — the draft feature is dead").toMatch(
      /draftStore=\{feedbackDraftStore\}/,
    );

    // Must-still-catch: the same wiring written into SharedApp is red.
    const wired = `${app}\n<CommuneHome draftStore={feedbackDraftStore} />`;
    expect(sharedAppBody(wired)).toMatch(DRAFT_STORE_REFERENCE);
    expect(sharedAppBody("function SharedApp() { return <CitizenChannel draftStore={x} />; }")).toMatch(DRAFT_STORE_REFERENCE);
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
const ZALO_API_SOURCE = stripComments(RAW_SOURCES["./features/tinh-nang/zalo-api.ts"] ?? "");

function callsInCode(code: string): string[] {
  return [...new Set([...code.matchAll(/\bsdk\s*\.\s*([A-Za-z][A-Za-z0-9_]*)\s*\(/g)].map((m) => m[1]!))];
}

describe("3c — mỗi lời gọi nền tảng khai mục đích tại chỗ", () => {
  it("đọc được mã nguồn của tệp SDK — nếu không, mọi ca dưới xanh vì lý do sai", () => {
    expect(ZALO_API_SOURCE.length, "không đọc được `features/tinh-nang/zalo-api.ts`").toBeGreaterThan(
      1000,
    );
    expect(callsInCode(ZALO_API_SOURCE).length).toBeGreaterThanOrEqual(10);
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
      ZALO_API_SOURCE,
      "một phép ép kiểu trên `sdk` giấu lời gọi khỏi phép đối chiếu bên dưới. Nếu `zmp-sdk` khai " +
        "thiếu một API, khai bổ sung kiểu ấy ở một chỗ có tên, đừng ép kiểu ngay tại lời gọi.",
    ).not.toMatch(/\bsdk\b\s*(?:as\b|satisfies\b)/);
    expect(
      ZALO_API_SOURCE,
      "`sdk` vừa được gán sang một tên khác. Lời gọi qua tên ấy không mang chuỗi `sdk.` nào, nên " +
        "nó không có mặt trong phép đối chiếu với bảng khai.",
    ).not.toMatch(/(?:const|let|var)\s+[A-Za-z_$][\w$]*\s*(?::[^=\n]*)?=\s*sdk\s*[;,)]/);
  });

  it("mỗi lời gọi trong mã có ĐÚNG MỘT dòng khai, và mỗi dòng khai trỏ về một lời gọi có thật", () => {
    const in_code = callsInCode(ZALO_API_SOURCE).sort();
    const declared = KHAI_BAO_LOI_GOI.map((k) => k.api).sort();

    expect(
      in_code.filter((name) => !declared.includes(name)),
      "một lời gọi nền tảng không có dòng khai. Thêm nó vào `KHAI_BAO_LOI_GOI` — màn Quản lý " +
        "quyền đọc từ bảng ấy, nên một lời gọi không khai là một quyền app dùng mà không màn nào " +
        "nói ra, trong một hồ sơ đang xin đúng những quyền đó.",
    ).toEqual([]);

    expect(
      declared.filter((name) => !in_code.includes(name)),
      "một dòng khai không còn lời gọi nào ứng với nó. Gỡ dòng ấy — một màn hình liệt kê một " +
        "quyền app KHÔNG dùng là khai thừa với người duyệt.",
    ).toEqual([]);

    expect(new Set(declared).size, "hai dòng khai cùng một lời gọi").toBe(declared.length);
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
    for (const name of ["getPhoneNumber", "getLocation", "scanQRCode"]) {
      const declaration = KHAI_BAO_LOI_GOI.find((k) => k.api === name);
      expect(declaration, `không có dòng khai cho quyền đang xin: ${name}`).toBeDefined();
    }
  });

  it("mọi lời gọi đưa dữ liệu ra khỏi máy đều nói ra điều đó", () => {
    // Cột `roi_khoi_may` rỗng nghĩa là KHÔNG CÓ GÌ rời khỏi máy — một khẳng định, không phải một
    // ô chưa điền. Ba lời gọi của luồng đăng nhập là ba lời gọi duy nhất có dữ liệu đi ra, và
    // chúng phải nói ra; ca này đỏ lên nếu ai đó làm rỗng một trong ba.
    // `getLocation` JOINED THIS LIST 29/09/2026: on "Gửi phản ánh" its code now goes to `vihat-miniapp`
    // `POST /api/v1/location` to become coordinates. Until that day this case pinned the opposite
    // ("the location code never leaves the phone") — and its message said what flipping it means: the
    // privacy policy now owes a section, pinned in `content/chinh-sach.test.ts`.
    for (const name of ["getPhoneNumber", "getAccessToken", "getLocation"]) {
      const declaration = KHAI_BAO_LOI_GOI.find((k) => k.api === name)!;
      expect(
        declaration.roi_khoi_may.trim().length,
        `${name} gửi một mã tới máy chủ nhưng bảng khai nói không có gì rời khỏi máy`,
      ).toBeGreaterThan(0);
    }
    // The Contact screen still sends nothing for it — the row must keep saying so, not only the new half.
    const location = KHAI_BAO_LOI_GOI.find((k) => k.api === "getLocation")!;
    expect(location.roi_khoi_may).toMatch(/Gửi phản ánh/);
    expect(location.roi_khoi_may).toMatch(/Liên hệ: không có gì rời khỏi máy/);
  });

  it("nửa nhà nước khai đúng một lời gọi riêng (getUserInfo) — và bảng nói ra nó thừa hưởng những gì", () => {
    // Quyền cấp theo App ID: ngày `src/citizen/` có tệp đầu tiên, nó thừa hưởng NGUYÊN VẸN mọi
    // quyền mà nửa thương mại đã xin được, không ai cấp lại. Ca này ghim tình trạng hôm nay để
    // lần khai đầu tiên của nửa ấy là một thay đổi có người đọc, chứ không phải một dòng lặng lẽ.
    expect(
      KHAI_BAO_LOI_GOI.filter((k) => k.nua === "nha-nuoc").map((k) => k.api),
      "nửa nhà nước vừa khai một lời gọi riêng — cập nhật ca này cùng lúc, và kiểm lại xem màn " +
        "Quản lý quyền có còn nói đúng việc nửa ấy dùng quyền vào đâu không",
      // 28/09/2026: lời khai đầu tiên của nửa nhà nước — lấy họ tên qua hộp xin quyền của Zalo cho ứng
      // dụng riêng của xã (chủ dự án: "chỉ cần xin quyền để lấy được name"). Tên không rời máy.
    ).toEqual(["getUserInfo"]);
    expect(
      KHAI_BAO_LOI_GOI.filter((k) => k.nua === "ca-hai").length,
      "không lời gọi nào được khai là dùng chung — nhưng quyền cấp theo App ID thì luôn dùng chung",
    ).toBeGreaterThan(0);
  });
});
