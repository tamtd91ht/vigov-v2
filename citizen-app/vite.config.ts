import { defineConfig } from "vite";

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

export default defineConfig({
  base: "./",
  plugins: [thePlainScript],
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
});
