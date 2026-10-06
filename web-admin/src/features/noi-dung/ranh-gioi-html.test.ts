import { readFileSync, readdirSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * MỘT LỆNH CẤM KHÔNG CÓ PHÉP KIỂM LÀ MỘT LỆNH CẤM SẼ BỊ PHÁ TRONG IM LẶNG.
 *
 * `noi_dung` là HTML (§8). Since ADR 0067 §1 the server sanitises it on every write
 * (`service-comms/internal/richtext`), but rows written before that are archival records that are
 * never rewritten, and the server is one wall, not two: this screen still never hands a string of
 * HTML to the page. The body is edited in Tiptap, which draws the document from its parsed schema.
 *
 * Vi phạm lệnh cấm dưới đây KHÔNG làm hỏng màn hình nào, KHÔNG làm đỏ test nào khác, và chạy đúng
 * trên máy người viết: một `dangerouslySetInnerHTML` thêm vào "để xem trước cho tiện" hiện ra đúng
 * bài viết mà người ấy vừa gõ. Nó chỉ lộ ra khi có người gõ một thẻ `<script>` — tức lộ ra trên màn
 * hình quản trị của một cơ quan nhà nước. Nên nó được kiểm bằng cách ĐỌC THẲNG MÃ NGUỒN, cùng
 * khuôn `src/ranh-gioi-nguon.test.ts`.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * PHẠM VI QUÉT LÀ MÃ CỦA MÀN NÀY, không phải cả `src/`. Không phải vì lệnh cấm chỉ đúng ở đây —
 * nó đúng ở mọi nơi — mà vì một tệp test của màn này mà đỏ vì mã của màn khác là một test không
 * ai biết phải sửa gì. Một dòng cho cả kho thuộc về `src/ranh-gioi-nguon.test.ts`; đã báo về.
 */

const GOC = fileURLToPath(new URL(".", import.meta.url));

/** Thư mục và tệp thuộc màn Nội dung Mini App, tính từ `src/`. */
const PHAM_VI = [
  fileURLToPath(new URL(".", import.meta.url)),
  fileURLToPath(new URL("../../app/noi-dung/", import.meta.url)),
  // Since 06/10/2026 the screen renders at `/mini-app` (its `Nội dung` tab); `/noi-dung` redirects there.
  fileURLToPath(new URL("../../app/mini-app/", import.meta.url)),
  fileURLToPath(new URL("../mini-app/", import.meta.url)),
  fileURLToPath(new URL("../../lib/api/noi-dung.ts", import.meta.url)),
  // §3's routes (ADR 0067 §2). The card itself (`portal-sync-card.tsx`) is in this folder already.
  fileURLToPath(new URL("../../lib/api/portal-sync.ts", import.meta.url)),
];

/**
 * Bỏ chú thích trước khi quét: chính các tệp bị quét GIẢI THÍCH chuỗi bị cấm, nên quét cả chú thích
 * thì test đỏ vì đúng phần văn bản dạy người sau tránh nó.
 *
 * Chỉ cắt khối chú thích nhiều dòng và những dòng bắt đầu bằng `//` — không cắt `//` giữa dòng, vì
 * `"https://…"` trong một chuỗi cũng có `//`. Đổi lại, một vi phạm viết cùng dòng với một chú thích
 * đuôi dòng sẽ lọt; chấp nhận, vì chiều sai kia — test đỏ oan rồi bị ai đó tắt đi — tệ hơn.
 */
function boChuThich(noiDung: string): string {
  return noiDung
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .filter((dong) => !/^\s*\/\//.test(dong))
    .join("\n");
}

function moiTepCuaMan(): { duongDan: string; noiDung: string }[] {
  const ra: { duongDan: string; noiDung: string }[] = [];

  const themTep = (day: string) => {
    if (![".ts", ".tsx"].includes(extname(day))) return;
    if (day.endsWith(".test.ts") || day.endsWith(".test.tsx")) return;
    ra.push({
      duongDan: relative(GOC, day).replace(/\\/g, "/"),
      noiDung: boChuThich(readFileSync(day, "utf8")),
    });
  };

  const duyet = (duong: string) => {
    // `statSync` chứ không bắt lỗi của `readdirSync`: một `try/catch` quanh phép đọc thư mục sẽ
    // nuốt luôn một đường dẫn GÕ SAI và biến nó thành "một tệp không đọc được" — tức bộ quét lặng
    // lẽ bỏ qua cả một thư mục của màn, và test vẫn xanh.
    if (!statSync(duong).isDirectory()) {
      themTep(duong);
      return;
    }
    for (const m of readdirSync(duong, { withFileTypes: true })) {
      const day = join(duong, String(m.name));
      if (m.isDirectory()) duyet(day);
      else themTep(day);
    }
  };

  for (const p of PHAM_VI) duyet(p);
  return ra;
}

const TEP = moiTepCuaMan();

describe("ranh giới HTML của màn Nội dung Mini App", () => {
  it("quét được đủ tệp của màn — một bộ quét rỗng là một bộ quét luôn xanh", () => {
    // Ba tệp mã của màn: `nhan-noi-dung.ts`, `so-noi-dung.tsx`, `app/noi-dung/page.tsx`, cộng
    // `lib/api/noi-dung.ts`. Con số dưới là sàn, không phải bản kê: thêm một tệp vào màn không
    // được làm đỏ, nhưng MẤT hết tệp thì phải đỏ.
    expect(TEP.length).toBeGreaterThanOrEqual(4);
    expect(TEP.map((t) => t.duongDan)).toContain("so-noi-dung.tsx");
    expect(TEP.map((t) => t.duongDan)).toContain("portal-sync-card.tsx");
    expect(TEP.some((t) => t.duongDan.endsWith("lib/api/portal-sync.ts"))).toBe(true);
  });

  it("KHÔNG tệp nào của màn chứa `dangerouslySetInnerHTML`", () => {
    // A stored body may predate the server's sanitiser (ADR 0067 §1 decision 5). Rendering it as a
    // string — even "just a preview" — runs whatever it holds on the commune's admin screen.
    const viPham = TEP.filter((t) => t.noiDung.includes("dangerouslySetInnerHTML")).map(
      (t) => t.duongDan,
    );
    expect(viPham).toEqual([]);
  });

  it("KHÔNG tệp nào của màn đụng tới `innerHTML` hay `document.write`", () => {
    // Cùng một lỗ hổng, hai lối đi khác. `dangerouslySetInnerHTML` là lối của React; `innerHTML`
    // trên một `ref` là lối của DOM, và nó không bị lệnh cấm trên bắt được.
    const viPham = TEP.filter((t) => /\binnerHTML\b|document\s*\.\s*write\b/.test(t.noiDung)).map(
      (t) => t.duongDan,
    );
    expect(viPham).toEqual([]);
  });

  it("KHÔNG tệp nào của màn dựng một `href` hay `src` từ dữ liệu máy chủ trả", () => {
    // Danh sách trắng lược đồ ở máy chủ (`http`/`https`) chỉ chặn được lúc GHI. Một hàng cũ trong
    // CSDL không có gì bảo đảm điều đó, nên `image_url` và `source_url` hiện dưới dạng CHỮ, không
    // phải một liên kết bấm được hay một thẻ ảnh. `javascript:…` trong một `href` là thực thi mã.
    const viPham = TEP.filter((t) =>
      /(href|src)\s*=\s*\{[^}]*(image_url|source_url)/.test(t.noiDung),
    ).map((t) => t.duongDan);
    expect(viPham).toEqual([]);
  });

  it("every `src` the screen builds is a server-signed preview, through `coverPreviewSrc` / `audioPreviewSrc` / `bodyImagePreviewSrc`", () => {
    // Body images (ADR 0067 §Sửa đổi 03/10/2026) brought a third: the editor's figure NodeView draws the
    // server's signed preview of each body image, through `bodyImagePreviewSrc` (the cover's parse). It is
    // allowed in THAT FILE ONLY — the NodeView is the one place a body image is drawn, and the document
    // itself never holds a src (`rich-text.test.ts`).
    // The broadcast audio (ADR 0067 §4) brought the `<audio>` player: its `src` is held to the same rule,
    // through `audioPreviewSrc` (`broadcast-audio.test.ts`: only `preview_url`, only http(s), only `ready`).
    // The cover upload (§7) brought the first `<img>` to this screen. The rule above forbids two
    // sources by name; this one is the allow-list: every `src={x}` must be an identifier bound, in
    // the same file, to `coverPreviewSrc(…)` — which reads only `preview_url` and only lets an
    // absolute http(s) URL through (`cover-image.test.ts`). A `blob:` of the officer's own file, an
    // `image_url`, or any other string becomes red here.
    const found: string[] = [];
    const viPham: string[] = [];
    for (const t of TEP) {
      for (const m of t.noiDung.matchAll(/\bsrc\s*=\s*\{([^}]*)\}/g)) {
        const expr = (m[1] ?? "").trim();
        found.push(`${t.duongDan}: ${expr}`);
        const binding = new RegExp(`const\\s+${expr}\\s*=([^;]*);`).exec(t.noiDung);
        const allowed =
          t.duongDan === "body-image-view.tsx"
            ? /\bbodyImagePreviewSrc\(/
            : /\b(coverPreviewSrc|audioPreviewSrc)\(/;
        const ok =
          /^[A-Za-z_$][\w$]*$/.test(expr) &&
          binding !== null &&
          allowed.test(binding[1] ?? "") &&
          !/image_url|source_url|createObjectURL|blob:/.test(binding[1] ?? "");
        if (!ok) viPham.push(`${t.duongDan}: src={${expr}}`);
      }
    }
    // A scan that finds nothing is a scan that is always green: the previews exist, so they must be seen.
    expect(found.length).toBeGreaterThanOrEqual(3);
    expect(found.some((f) => f.startsWith("broadcast-audio-field.tsx"))).toBe(true);
    expect(found.some((f) => f.startsWith("body-image-view.tsx"))).toBe(true);
    // `bodyImagePreviewSrc` anywhere else is not a way around the rule above: only the NodeView may bind it.
    expect(
      TEP.filter((t) => t.duongDan !== "body-image-view.tsx" && t.duongDan !== "body-image.ts")
        .filter((t) => /\bbodyImagePreviewSrc\(/.test(t.noiDung))
        .map((t) => t.duongDan),
    ).toEqual([]);
    // No src is set through the DOM either (`img.src = …`, `setAttribute("src", …)`): the scan above only
    // reads JSX, so a NodeView written with plain DOM would pass it unseen.
    expect(
      TEP.filter((t) => /\.src\s*=|setAttribute\(\s*["']src["']/.test(t.noiDung)).map((t) => t.duongDan),
    ).toEqual([]);
    expect(viPham).toEqual([]);
  });

  it("no file of the screen makes a `blob:` / object URL", () => {
    const viPham = TEP.filter((t) => /createObjectURL|["'`]blob:/.test(t.noiDung)).map((t) => t.duongDan);
    expect(viPham).toEqual([]);
  });
});
