import { fileURLToPath } from "node:url";

import { defineConfig } from "vite";

// MỘT NGUỒN SỰ THẬT cho cấu hình lúc dựng — `scripts/deploy.mjs` nhập đúng mô-đun này. Nó đọc
// `.env.local` bằng `loadEnv` (Vite KHÔNG tự nạp tệp ấy vào `process.env`), để biến shell thắng
// tệp, và CHẶN mọi tên biến ngoài danh sách trắng. Đọc khối chú thích ở đó trước khi sửa.
import { docCauHinh } from "./scripts/cau-hinh.mjs";

/**
 * HAI BIẾN THỂ BẢN DỰNG — `VIGOV_BIEN_THE`.
 *
 * | Biến thể | Nội dung | Dùng để |
 * |---|---|---|
 * | `goc` | Ứng dụng sản phẩm đầy đủ, gồm sáu tính năng dùng chín quyền nền tảng | **BẢN NỘP** |
 * | `day-du` (mặc định) | `goc` + lớp khám phá + danh mục xã mẫu + bảng chẩn đoán | Thử nghiệm, demo |
 *
 * VÌ SAO BIẾN THỂ `quyen` KHÔNG CÒN: nó từng tồn tại vì ba màn quyền là một lớp trình diễn thêm
 * vào một app giới thiệu tĩnh — gỡ được, và bản nộp tối thiểu thì gỡ nó đi. Nay ba quyền ấy
 * thuộc về chính ứng dụng sản phẩm (quét danh thiếp · tìm văn phòng · đăng nhập bằng số Zalo), nên
 * `quyen` trùng hoàn toàn với `goc`. Hai biến thể nói cùng một thứ là hai biến thể sẽ lệch nhau.
 *
 * VÌ SAO TÁCH Ở TẦNG DỰNG CHỨ KHÔNG PHẢI MỘT CỜ LÚC CHẠY:
 *
 *   Một cờ lúc chạy để **tám tên đơn vị hành chính đặt ra** và số điện thoại mẫu nằm nguyên
 *   trong bundle gửi duyệt — chỉ là không vẽ ra. Tách ở tầng dựng thì bản `goc` **thật sự không
 *   chứa** chúng, và điều đó **kiểm được**: `grep` trên `dist/assets/app.js`, và một ca trong
 *   `src/bundle-for-zalo.test.ts` dựng thật rồi đọc bundle. Lời hứa thành sự thật đo được.
 *
 *   Đó cũng là điều làm việc này trung thực: bản nộp duyệt đúng bằng thứ người duyệt đọc. Không
 *   có gì bị giấu — chỉ là chọn nộp cái gì.
 *
 * VÌ SAO LÀ `resolve.alias` CHỨ KHÔNG PHẢI TREE-SHAKING: tree-shaking **không** loại được một
 * `import` tĩnh có mặt trong mã. Alias thì thay hẳn mô-đun ở bước phân giải, nên mã thật không
 * có đường nào đi vào bundle.
 *
 * VÌ SAO TÊN LÀ `VIGOV_BIEN_THE` CHỨ KHÔNG PHẢI `VIGOV_KHAM_PHA`: cờ này tắt **hai** thứ — lớp
 * khám phá và bảng chẩn đoán — nên một cái tên nói về riêng lớp khám phá là một cái tên nói
 * thiếu, và người sau sẽ thêm thứ thứ ba vào sau một cái tên không mô tả nó.
 */
const BIEN_THE_HOP_LE = ["goc", "day-du"] as const;
type BienThe = (typeof BIEN_THE_HOP_LE)[number];

/**
 * ĐỌC LÚC GỌI, KHÔNG PHẢI LÚC NẠP MÔ-ĐUN — `defineConfig` nhận một hàm chính vì việc này: một
 * tiến trình dựng hai biến thể liên tiếp (chính là `bundle-for-zalo.test.ts`) phải thấy giá trị
 * mới, chứ không thấy giá trị đã đóng băng lúc tệp cấu hình được nạp lần đầu.
 *
 * SAI TÊN THÌ DỪNG, KHÔNG ĐOÁN. `VIGOV_BIEN_THE=gôc` mà vẫn dựng tiếp nghĩa là dựng bản ĐẦY ĐỦ
 * rồi đem nộp duyệt dưới nhãn "bản gốc" — hỏng trong im lặng, và hỏng đúng ở chỗ đắt nhất.
 *
 * Giá trị đọc qua `cauHinh()`: `.env.local` cho máy local, biến shell đè lên nó. `npm run
 * build:goc` và `deploy.mjs` đặt biến shell, nên một dòng `VIGOV_BIEN_THE` trong tệp KHÔNG bao
 * giờ đổi được biến thể của một lệnh đã nói rõ nó dựng bản nào.
 */
function docBienThe(): BienThe {
  const dat = (cauHinh().VIGOV_BIEN_THE || "day-du").trim();
  if (!(BIEN_THE_HOP_LE as readonly string[]).includes(dat)) {
    throw new Error(
      `VIGOV_BIEN_THE="${dat}" không phải biến thể nào cả. Chỉ nhận: ${BIEN_THE_HOP_LE.join(" · ")}.`,
    );
  }
  return dat as BienThe;
}

const duongDan = (tuong_doi: string) => fileURLToPath(new URL(tuong_doi, import.meta.url));

/**
 * Hai cái tên này là hai cửa duy nhất vào hai phần gỡ được. Tệp ngoài chỉ được nhập qua chúng —
 * nhập thẳng một tệp bên trong là đi vòng qua alias, và bản rút gọn khi ấy vẫn dựng xanh trong
 * khi mang theo đúng thứ đáng lẽ không có. `bien-the.test.ts` là thứ canh điều đó.
 *
 * Bản `goc` là BẢN NỘP: ứng dụng sản phẩm đủ sáu tính năng, **gồm cả khối đăng nhập gọi máy chủ
 * thật**, nhưng không tên đơn vị hành chính đặt ra, không số điện thoại mẫu, không bảng chẩn
 * đoán. `day-du` là bản demo nội bộ và có tất cả.
 */
function aliasTheoBienThe(): Record<string, string> {
  const day_du = docBienThe() === "day-du";
  return {
    "bien-the/kham-pha": duongDan(
      day_du ? "./src/features/kham-pha/index.ts" : "./src/features/kham-pha/index.rong.ts",
    ),
    "bien-the/chan-doan": duongDan(
      day_du ? "./src/features/diagnostics/index.ts" : "./src/features/diagnostics/index.rong.ts",
    ),
    // KHÔNG CÓ CỬA CHO KHỐI ĐĂNG NHẬP, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH ĐÃ ĐẢO NGƯỢC MỘT LẦN — 20/09.
    // Bản đầu đặt lời gọi máy chủ sau một cửa `bien-the/dang-nhap` để bản nộp không gọi mạng.
    // Nay CẢ HAI biến thể gọi thật, nên không còn gì để tách; giữ lại cái cửa khi hai bên nó
    // giống hệt nhau là giữ lại đúng cái bẫy mà biến thể `quyen` đã để lại một lần.
  };
}

/**
 * ĐỊA CHỈ MÁY CHỦ — MỘT BIẾN LÚC DỰNG, cùng khuôn với `VIGOV_BIEN_THE` ngay trên.
 *
 * `VIGOV_API_HOST` đi vào mã dưới cái tên `__VIGOV_API_HOST__`, và chỉ `features/dang-nhap/`
 * đọc nó. ⚠ CẢ HAI BIẾN THỂ ĐỀU CẦN NÓ, kể cả BẢN NỘP: quên khai là nộp một nút đăng nhập nói
 * "người dựng bản cần đặt biến…". `scripts/deploy.mjs` chặn đường đẩy khi biến còn rỗng —
 * chặn ở đó chứ không ném lỗi lúc dựng, vì `npm test` và `npm run dev` phải chạy được trên một
 * máy chưa có địa chỉ máy chủ nào.
 *
 * ⚠ KHÔNG CÓ GIÁ TRỊ MẶC ĐỊNH, VÀ KHÔNG BAO GIỜ ĐƯỢC CÓ. Một `?? "https://…"` ở đây là gửi hai
 * mã đăng nhập của một người thật tới một máy chủ không ai chọn, trên mọi bản dựng của mọi máy
 * quên đặt biến. Chưa khai thì `hop-dong.ts` trả về chuỗi rỗng và màn hình nói ra điều đó —
 * fail closed.
 *
 * ⚠ ĐÂY KHÔNG PHẢI CHỖ ĐỂ MỘT BÍ MẬT NÀO. `define` chèn giá trị THẲNG vào bundle, tức là vào
 * một tệp gửi lên Zalo và tải về máy người dùng (luật 8, bất biến 4 — cùng lý do với
 * `NEXT_PUBLIC_*`). Một địa chỉ máy chủ thì công khai được; khoá bí mật của Mini App thì chỉ
 * nằm ở backend (ADR 0020, bất biến 2).
 */
function diaChiMayChu(): string {
  return cauHinh().VIGOV_API_HOST;
}

/**
 * Đọc cấu hình MỘT LẦN MỖI LẦN DỰNG, không phải một lần mỗi tiến trình.
 *
 * `bundle-for-zalo.test.ts` dựng hai biến thể liên tiếp trong cùng một tiến trình bằng cách
 * đổi `process.env.VIGOV_BIEN_THE` giữa hai lần. Nhớ kết quả ở cấp mô-đun thì lần dựng thứ hai
 * đọc lại giá trị của lần đầu — và ca kiểm "hai biến thể" sẽ so bản `goc` với chính nó.
 *
 * Cache theo giá trị shell đang thấy: cùng một lần dựng thì đọc đĩa đúng một lần (kiểm danh
 * sách trắng cũng chỉ chạy một lần, thông báo lỗi không lặp ba lần), đổi shell thì đọc lại.
 */
let nho: { khoa: string; gia_tri: ReturnType<typeof docCauHinh> } | null = null;

function cauHinh(): ReturnType<typeof docCauHinh> {
  const khoa = `${process.env.VIGOV_BIEN_THE ?? ""}\u0000${process.env.VIGOV_API_HOST ?? ""}`;
  if (nho === null || nho.khoa !== khoa) nho = { khoa, gia_tri: docCauHinh() };
  return nho.gia_tri;
}

/**
 * Build configuration for the Zalo Mini App bundle.
 *
 * `base: "./"` — assets must be referenced RELATIVE to the bundle. A Mini App is served from a
 * path chosen by the platform, not from the root of a domain we control, so absolute `/assets/…`
 * URLs resolve to somewhere that does not exist and the app opens blank on a real device while
 * working perfectly in `vite preview`.
 *
 * No `@vitejs/plugin-react`: its job is Fast Refresh in dev. Vite's own esbuild pipeline already
 * compiles `.tsx` with the automatic JSX runtime declared in tsconfig.json, so the plugin would
 * be a dependency added for a convenience this four-screen app does not need.
 */
/**
 * Đổi thẻ script của trang dựng ra từ MODULE sang CỔ ĐIỂN.
 *
 * Không phải khẩu vị — đây là điều kiện để bản nộp chạy được, và nó đã được thử chứ không suy
 * đoán. `zmp-cli sync-config` đọc `dist/index.html` để điền `app-config.json`, và Zalo chỉ nạp
 * đúng những tệp khai trong đó. Chạy trên hai bản HTML khác nhau một chỗ:
 *
 *   <script type="module" crossorigin src="./assets/app.js">  ->  listSyncJS: ["inline.js"]
 *   <script src="./assets/app.js">                            ->  listSyncJS: ["inline.js",
 *                                                                              "./assets/app.js"]
 *
 * Bản trên thiếu chính bundle của ứng dụng, và thiếu TRONG IM LẶNG: `sync-config` báo thành
 * công, `vite preview` chạy hoàn hảo, và app trắng trơn trên máy thật.
 *
 * Vite giữ `type="module"` kể cả khi `format: "iife"` và `target: "es2015"`, nên phải sửa ở
 * bước phát HTML. `enforce: "post"` để chạy SAU khi Vite đã chèn thẻ.
 */
const thePlainScript = {
  name: "zalo-classic-script",
  enforce: "post" as const,
  transformIndexHtml(html: string) {
    return html.replace(/<script\s+type="module"\s+crossorigin\s+src=/g, "<script src=");
  },
};

// Hàm, không phải hằng: xem `docBienThe` — biến môi trường phải được đọc mỗi lần dựng.
export default defineConfig(() => ({
  base: "./",
  plugins: [thePlainScript],
  resolve: { alias: aliasTheoBienThe() },
  define: { __VIGOV_API_HOST__: JSON.stringify(diaChiMayChu()) },
  build: {
    outDir: "dist",
    // Zalo reviews and hosts a static bundle. Keeping sourcemaps out keeps the uploaded
    // package small and keeps our source off a device we do not control.
    sourcemap: false,

    // CLASSIC SCRIPT, ONE FILE, STABLE NAMES — and every word of that is forced by how the
    // platform loads a Mini App, not by taste.
    //
    // Zalo does not serve our index.html. It builds its own shell and loads exactly the files
    // listed in app-config.json, which `zmp-cli sync-config` fills in by reading the built HTML.
    // Run against Vite's default output, that command produced:
    //
    //     listCSS:    ["./assets/index-EzjO69V_.css"]
    //     listSyncJS: ["inline.js"]          <- the app bundle is MISSING
    //
    // It skipped the application bundle because Vite emits `<script type="module">` and the
    // tool only collects classic scripts. Submitting that ships an app whose JavaScript is
    // never loaded: blank on a real device, perfect in `vite preview`. Exactly the failure
    // bundle-for-zalo.test.ts exists to catch, one layer further out.
    //
    // `format: "iife"` gives a classic script. Fixed names instead of content hashes because
    // app-config.json is COMMITTED: a hashed name makes that file wrong the moment anyone
    // rebuilds, and wrong in a way nothing reports. Cache busting is the platform's job here —
    // it versions the whole bundle on deploy.
    target: "es2015",
    modulePreload: false,
    rollupOptions: {
      output: {
        format: "iife",
        entryFileNames: "assets/app.js",
        chunkFileNames: "assets/app-[name].js",
        assetFileNames: "assets/app.[ext]",
      },
    },
  },
}));
