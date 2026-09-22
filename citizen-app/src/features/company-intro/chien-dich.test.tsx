import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { CHIEN_DICH, chienDichCua, KHOA_CHIEN_DICH } from "../../content/chien-dich";

import { DaiChienDich } from "./DaiChienDich";

const ve = (node: ReactElement) => renderToStaticMarkup(node);

/**
 * Dựng một lượt vẽ như thể app vừa được mở bằng một chuỗi truy vấn cho trước.
 *
 * `lib/launch-params.ts` đọc `window.location.search` và bọc try/catch, nên trong Node nó trả về
 * "không có tham số". Đặt một `window` giả quanh ĐÚNG lượt vẽ là cách rẻ nhất để hỏi câu duy nhất
 * đáng hỏi ở đây — *app làm gì với một tham số người ngoài tự đặt* — mà không cần một trình duyệt.
 *
 * LUÔN TRẢ `window` VỀ NGUYÊN TRẠNG trong `finally`: một biến toàn cục rò ra khỏi một ca kiểm làm
 * ca chạy sau nó xanh (hoặc đỏ) vì một lý do không có trong chính nó.
 */
function moVoi<T>(chuoi_truy_van: string, lam: () => T): T {
  const cua_so = globalThis as unknown as { window?: unknown };
  const truoc = cua_so.window;
  cua_so.window = { location: { search: chuoi_truy_van, href: `https://zalo.me/s/1${chuoi_truy_van}` } };
  try {
    return lam();
  } finally {
    if (truoc === undefined) delete cua_so.window;
    else cua_so.window = truoc;
  }
}

const chuCua = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ");

describe("bảng chiến dịch tra kín, fail closed", () => {
  it("có ít nhất một dòng — nếu không, mọi ca dưới xanh vì không tìm thấy gì", () => {
    expect(Object.keys(CHIEN_DICH).length).toBeGreaterThan(0);
  });

  it("mã có thật trả về đúng dòng của bảng", () => {
    for (const [ma, dong] of Object.entries(CHIEN_DICH)) {
      expect(chienDichCua(ma)).toBe(dong);
    }
  });

  it("mã lạ, mã rỗng, và tên một thuộc tính nguyên mẫu đều trả `null`", () => {
    // ⚠ BA CHUỖI CUỐI KHÔNG PHẢI ĐỂ CHO ĐỦ. `CHIEN_DICH["constructor"]` đọc thẳng vào nguyên mẫu
    // của `Object` và trả về một thứ KHÁC `undefined` — tức là một `?? null` thẳng thớm sẽ coi
    // `src=constructor` là một chiến dịch có thật, và thứ hiện ra là một mảnh nội bộ của
    // JavaScript, trên màn hình đứng tên một pháp nhân.
    for (const ma of ["", "khong-co-that", "VP-QUAY", "constructor", "toString", "__proto__"]) {
      expect(chienDichCua(ma), `mã "${ma}" được coi là một chiến dịch có thật`).toBeNull();
    }
  });
});

describe("dải chiến dịch trên màn chủ", () => {
  it("không có tham số nào thì không hiện gì", () => {
    // Đường phổ biến NHẤT từ lần mở thứ hai: mở từ danh sách app ghim, tìm trong Zalo, quay lại
    // tuần sau. Không đường nào trong số ấy mang tham số.
    expect(moVoi("", () => ve(<DaiChienDich />))).toBe("");
  });

  it("mã lạ thì không hiện gì — không dải, không câu lỗi", () => {
    expect(moVoi(`?${KHOA_CHIEN_DICH}=khong-co-that`, () => ve(<DaiChienDich />))).toBe("");
  });

  /**
   * ⚠ CA ĐẮT NHẤT CỦA TỆP NÀY: THAM SỐ KHÔNG BAO GIỜ ĐƯỢC IN RA MÀN HÌNH.
   *
   *   Tham số mở app là dữ liệu client tự đặt (luật 1, cấm #2). Một nhánh "in lại mã cho biết đã
   *   nhận" biến màn chủ thành chỗ người ngoài viết chữ lên: đưa ai đó một đường liên kết có
   *   `src=<câu muốn viết>` là đủ. Ca này cho hàm ăn đúng hình dạng tấn công ấy.
   */
  it("một chuỗi tham số tự đặt KHÔNG bao giờ lọt lên màn hình", () => {
    const cau_chen = "ViHAT-tang-ban-100-trieu";
    const markup = moVoi(`?${KHOA_CHIEN_DICH}=${cau_chen}`, () => ve(<DaiChienDich />));
    expect(markup, "app in nguyên văn tham số người mở tự đặt").toBe("");
    expect(markup).not.toContain(cau_chen);
  });

  it("mã có thật thì hiện ĐÚNG chữ trong bảng, và chỉ chữ ấy", () => {
    for (const [ma, dong] of Object.entries(CHIEN_DICH)) {
      const markup = moVoi(`?${KHOA_CHIEN_DICH}=${ma}`, () => ve(<DaiChienDich />));
      const chu = chuCua(markup);
      expect(chu, `chiến dịch "${ma}" không hiện tiêu đề của nó`).toContain(dong.tieu_de);
      expect(chu, `chiến dịch "${ma}" không hiện câu chào của nó`).toContain(dong.cau_chao);
      // Và chính MÃ ấy — một chuỗi của phía client — vẫn không lọt lên màn hình.
      expect(chu, `mã "${ma}" bị in ra màn hình`).not.toContain(ma);
    }
  });

  it("dải không mang nút nào — giai đoạn A chưa có tuyến nào nhận", () => {
    const ma = Object.keys(CHIEN_DICH)[0]!;
    const markup = moVoi(`?${KHOA_CHIEN_DICH}=${ma}`, () => ve(<DaiChienDich />));
    expect(markup, "dải chiến dịch mọc ra một nút").not.toMatch(/<(button|a|form|input)\b/);
  });
});
