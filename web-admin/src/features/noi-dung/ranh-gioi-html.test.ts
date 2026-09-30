import { readFileSync, readdirSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * MỘT LỆNH CẤM KHÔNG CÓ PHÉP KIỂM LÀ MỘT LỆNH CẤM SẼ BỊ PHÁ TRONG IM LẶNG.
 *
 * `noi_dung` là HTML (§8) và MÁY CHỦ KHÔNG LÀM SẠCH NÓ — kho chưa có bộ làm sạch nào, và giới hạn
 * ấy được ghi thẳng trong `service-comms/internal/domain/noi_dung_mini_app.go`
 * (`ChuanHoaVanBanDai`). Một cán bộ có `content.update` đặt được `<script>` vào thứ mọi cư dân xã
 * mở trên điện thoại, và màn Phân quyền của xã có thể đã cấp khoá ấy cho nhiều người.
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
  fileURLToPath(new URL("../../lib/api/noi-dung.ts", import.meta.url)),
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
  });

  it("KHÔNG tệp nào của màn chứa `dangerouslySetInnerHTML`", () => {
    // Máy chủ lưu HTML NGUYÊN VĂN và không có bộ làm sạch nào phía sau. Dựng chuỗi ấy — kể cả chỉ
    // để "xem trước" — là chạy mã của người vừa gõ, trên màn hình quản trị của xã.
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
});
