import { readFileSync, readdirSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * Bốn ràng buộc dưới đây không kiểm được bằng một test chức năng: vi phạm chúng KHÔNG làm hỏng
 * màn hình nào, không làm đỏ test nào khác, và vẫn chạy đúng trên máy người viết. Chúng chỉ lộ
 * ra khi có xã thứ hai — tức là lộ ra trong sản xuất, giữa hai cơ quan nhà nước. Nên chúng được
 * kiểm bằng cách đọc thẳng mã nguồn.
 *
 * Tệp test bị loại khỏi phạm vi quét: chính tệp này phải viết ra các chuỗi bị cấm để tìm chúng.
 */

const GOC = fileURLToPath(new URL(".", import.meta.url));

/**
 * Bỏ chú thích trước khi quét: các tệp dưới đây GIẢI THÍCH những chuỗi bị cấm, nên quét cả chú
 * thích thì test đỏ vì đúng phần văn bản dạy người sau tránh chúng.
 *
 * Chỉ cắt khối chú thích nhiều dòng và những dòng bắt đầu bằng `//` — không cắt `//` giữa dòng, vì
 * `"https://…"` trong một chuỗi cũng có `//`. Đổi lại, một vi phạm viết cùng dòng với một chú
 * thích đuôi dòng sẽ lọt; chấp nhận, vì chiều sai kia — test đỏ oan rồi bị ai đó tắt đi — tệ hơn.
 */
function boChuThich(noiDung: string): string {
  return noiDung
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .filter((dong) => !/^\s*\/\//.test(dong))
    .join("\n");
}

function moiTepNguon(): { duongDan: string; noiDung: string }[] {
  const ra: { duongDan: string; noiDung: string }[] = [];
  const duyet = (thuMuc: string) => {
    for (const muc of readdirSync(thuMuc, { withFileTypes: true })) {
      const day = join(thuMuc, muc.name);
      if (muc.isDirectory()) {
        duyet(day);
      } else if ([".ts", ".tsx", ".css"].includes(extname(muc.name)) && !muc.name.endsWith(".test.ts")) {
        ra.push({
          duongDan: relative(GOC, day).replace(/\\/g, "/"),
          noiDung: boChuThich(readFileSync(day, "utf8")),
        });
      }
    }
  };
  duyet(GOC);
  return ra;
}

const TEP = moiTepNguon();

function viPham(mau: RegExp, chua?: (duongDan: string) => boolean): string[] {
  return TEP.filter((t) => (chua ? !chua(t.duongDan) : true))
    .filter((t) => mau.test(t.noiDung))
    .map((t) => t.duongDan);
}

describe("ranh giới của mã nguồn web quản trị", () => {
  it("quét được ít nhất một tệp — một bộ quét rỗng là một bộ quét luôn xanh", () => {
    expect(TEP.length).toBeGreaterThan(5);
  });

  it("không dòng nào đụng tới cookie từ phía client", () => {
    // Cookie phiên do dịch vụ identity đặt bằng Set-Cookie: httpOnly, secure, SameSite=Lax và
    // KHÔNG có thuộc tính Domain. Mọi lần client ghi cookie đều là một lần thuộc tính bị viết
    // lại ở nơi không ai soát — kể cả khi lần ấy viết đúng.
    expect(viPham(/document\s*\.\s*cookie/)).toEqual([]);
  });

  it("không nơi nào trong ứng dụng tự dựng cookie phiên", () => {
    // `cookieOptions` trong lib/session.ts là bản mô tả quy tắc, không phải chỗ để gọi. Nếu có
    // ngày nó được gọi, phải có người đọc lại nó cùng `identity/internal/http/cookie.go`
    // — hai tệp ấy hiện KHÔNG khớp nhau về thuộc tính Domain (đã nêu trong báo cáo).
    expect(viPham(/cookieOptions\s*\(/, (d) => d === "lib/session.ts")).toEqual([]);
  });

  it("không có biến NEXT_PUBLIC_ nào", () => {
    // Biến tiền tố công khai được thay lúc BUILD và nằm luôn trong bundle. Một bundle không thể
    // mang tên của 300 xã (luật 8, bất biến 4; luật 1, bất biến 10). Hiện ứng dụng này không
    // cần biến nào cả: API ở cùng host nên đường dẫn tương đối là đủ.
    const mau = new RegExp(["NEXT", "PUBLIC"].join("_") + "_");
    expect(viPham(mau)).toEqual([]);
  });

  it("không có tên miền nào viết cứng trong mã", () => {
    // Một host trong mã là một xã trong mã. Kể cả `.vigov.vn` dùng làm miền cookie — đúng cái
    // một dòng làm cookie của xã này đi tới mọi tên miền con của xã khác (luật 1, cấm #3).
    expect(viPham(/vigov\.vn/)).toEqual([]);
  });
});
