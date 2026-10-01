import { defineConfig } from "vite";

// MỘT NGUỒN SỰ THẬT cho cấu hình lúc dựng — `scripts/deploy.mjs` nhập đúng mô-đun này. Nó đọc
// `.env.local` bằng `loadEnv` (Vite KHÔNG tự nạp tệp ấy vào `process.env`), để biến shell thắng
// tệp, và CHẶN mọi tên biến ngoài danh sách trắng. Đọc khối chú thích ở đó trước khi sửa.
import { buildLabel, demoBuild, docCauHinh, xaCoDinh } from "./scripts/cau-hinh.mjs";

/**
 * MỘT BẢN DỰNG, KHÔNG BIẾN THỂ — 27/09/2026, quyết định của chủ sản phẩm.
 *
 * Trước ngày này có hai biến thể (`goc` / `day-du`) tách nhau bằng `resolve.alias`, để bản nộp
 * Zalo duyệt không mang lớp khám phá, danh mục xã mẫu, bảng chẩn đoán và kênh công dân. Chủ sản
 * phẩm đã bỏ cả khái niệm ấy: "demo" chỉ là cách nói về giai đoạn làm, về kỹ thuật đây là app DÙNG
 * THẬT. Giai đoạn 1 (trước công văn của xã) là app chung của ViHAT, mở bằng QR bản thử nghiệm mang
 * tên miền xã; giai đoạn 2 là App ID riêng của xã. Cả hai chạy CÙNG MỘT citizen-app, nên không còn
 * gì để tách: danh mục xã mẫu và bảng chẩn đoán đã bị xoá khỏi mã, không phải giấu sau alias.
 *
 * Thứ bị gỡ theo: `VIGOV_BIEN_THE`, ba cửa `bien-the/…`, các tệp `index.rong.ts`. Tên miền xã chỉ
 * chọn App ID ĐÍCH lúc đẩy (`scripts/deploy.mjs --domain=`); nó KHÔNG đổi nội dung bundle — bundle
 * là một. NGOẠI LỆ DUY NHẤT (chủ dự án, 27/09/2026): thêm `--vao-thang` thì tên miền ấy được nung
 * vào `__VIGOV_XA_CO_DINH__` và app mở thẳng vào xã — không có cờ ấy, bundle vẫn là một.
 * (Tệp này không nhắc tên tệp chọn đích, kể cả trong chú thích: có một ca kiểm ghim theo
 * CHUỖI rằng nó không nhập tệp ấy.)
 */

/**
 * ĐỊA CHỈ MÁY CHỦ — BIẾN LÚC DỰNG DUY NHẤT.
 *
 * `VIGOV_API_HOST` đi vào mã dưới cái tên `__VIGOV_API_HOST__`, và chỉ `features/dang-nhap/`
 * đọc nó. ⚠ Bản đẩy lên Zalo CẦN NÓ: quên khai là nộp một nút đăng nhập nói
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
 * Cache theo giá trị shell đang thấy: cùng một lần dựng thì đọc đĩa đúng một lần (kiểm danh
 * sách trắng cũng chỉ chạy một lần, thông báo lỗi không lặp lại), đổi shell thì đọc lại — một
 * tiến trình test đổi `VIGOV_API_HOST` giữa hai lần dựng phải thấy giá trị mới.
 */
let nho: { khoa: string; gia_tri: ReturnType<typeof docCauHinh> } | null = null;

function cauHinh(): ReturnType<typeof docCauHinh> {
  const khoa = process.env.VIGOV_API_HOST ?? "";
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

// Hàm, không phải hằng: biến môi trường phải được đọc mỗi lần dựng, không đóng băng lúc nạp tệp.
export default defineConfig(() => ({
  base: "./",
  plugins: [thePlainScript],
  // `__VIGOV_XA_CO_DINH__`: rỗng trừ khi `deploy.mjs --vao-thang` dựng app riêng của một xã — xem
  // `xaCoDinh` trong scripts/cau-hinh.mjs và src/lib/xa-co-dinh.ts.
  define: {
    __VIGOV_API_HOST__: JSON.stringify(diaChiMayChu()),
    __VIGOV_XA_CO_DINH__: JSON.stringify(xaCoDinh()),
    // `__VIGOV_DEMO__`: `false` unless `deploy.mjs --vao-thang --demo` builds a commune's own app for a
    // pre-submission demo (owner, 30/09/2026) — see `demoBuild` in scripts/cau-hinh.mjs and src/lib/demo-build.ts.
    __VIGOV_DEMO__: JSON.stringify(demoBuild()),
    // `__VIGOV_BUILD_LABEL__`: short commit + build date for the Cá nhân footer, "" without git — see
    // `buildLabel` in scripts/cau-hinh.mjs and src/lib/build-label.ts.
    __VIGOV_BUILD_LABEL__: JSON.stringify(buildLabel()),
  },
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
    // ONE FILE BY CONSTRUCTION (`iife` cannot split), so the 500 kB default only repeats on every build
    // that the file is the file. Measured 01/10/2026: 835 kB raw / 224 kB gzip. The limit sits above that
    // so the warning comes back the day the bundle really grows, not never.
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      // EMPTY_IMPORT_META only, and only because it is harmless HERE: the one `import.meta` user is Vite's
      // preload helper around `await import("zmp-sdk")` (zalo-api.ts — dynamic on purpose: a failed import
      // means "not inside Zalo"). With `iife` that import is inlined, the helper is called with no deps and
      // never reads `import.meta.url`. NOT `define: { "import.meta": {} }` — Vitest shares this config and
      // the source-scanning tests run on `import.meta.glob`. Every other warning still prints.
      onwarn(warning, warn) {
        if (warning.code === "EMPTY_IMPORT_META") return;
        warn(warning);
      },
      output: {
        format: "iife",
        entryFileNames: "assets/app.js",
        chunkFileNames: "assets/app-[name].js",
        assetFileNames: "assets/app.[ext]",
      },
    },
  },
}));
