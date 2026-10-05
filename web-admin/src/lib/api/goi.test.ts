import { afterEach, describe, expect, it, vi } from "vitest";

import { docJSON, errorMessageOr, LOI_CHUA_HO_TRO, LOI_KHONG_RO, stripTechnicalPrefix, thongBaoLoi } from "./goi";

/**
 * Two display rules of `goi.ts` from the 05/10/2026 tester report: BC-01 (a 404 without a JSON body is
 * "not supported yet", not "cannot connect") and §4.3 (one leading technical prefix stripped).
 */

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("stripTechnicalPrefix — real server sentences", () => {
  it.each([
    ["bo_phan: thiếu tên bộ phận", "Thiếu tên bộ phận"],
    ["can_bo: email đã được dùng cho một cán bộ khác", "Email đã được dùng cho một cán bộ khác"],
    ["ngan_sach: số tiền phải lớn hơn 0", "Số tiền phải lớn hơn 0"],
    ["du_an: thiếu tên dự án", "Thiếu tên dự án"],
    ["văn bản đến: ngày ký văn bản sau ngày đến", "Ngày ký văn bản sau ngày đến"],
    ["văn bản đi: thiếu nơi nhận", "Thiếu nơi nhận"],
    ["văn bản: thiếu trích yếu", "Thiếu trích yếu"],
  ])("%s → %s", (raw, shown) => {
    expect(stripTechnicalPrefix(raw)).toBe(shown);
  });

  it("strips the tag but NOT the backticked field name that follows it", () => {
    expect(stripTechnicalPrefix("danh_muc: `code` chỉ gồm chữ thường, số và dấu gạch ngang")).toBe(
      "`code` chỉ gồm chữ thường, số và dấu gạch ngang",
    );
  });

  it("one prefix only; a plain Vietnamese sentence is untouched", () => {
    expect(stripTechnicalPrefix("du_an: ngan_sach: x")).toBe("Ngan_sach: x");
    const plain = "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.";
    expect(stripTechnicalPrefix(plain)).toBe(plain);
    expect(stripTechnicalPrefix("Lưu ý: số đã cấp không cấp lại")).toBe("Lưu ý: số đã cấp không cấp lại");
    // A prefix with nothing after it is not a sentence to empty out.
    expect(stripTechnicalPrefix("bo_phan: ")).toBe("bo_phan: ");
  });
});

function stub(status: number, body: string, contentType = "application/json") {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { status, headers: { "Content-Type": contentType } })));
}

describe("thongBaoLoi — BC-01", () => {
  it("404 with a plain-text body (route not deployed) → 'chưa hỗ trợ', not 'không kết nối'", async () => {
    expect(await thongBaoLoi(new Response("404 page not found", { status: 404 }))).toBe(LOI_CHUA_HO_TRO);
  });

  it("404 the server explained keeps its own sentence, prefix stripped", async () => {
    const res = new Response(JSON.stringify({ code: "not_found", message: "du_an: không tìm thấy dự án" }), { status: 404 });
    expect(await thongBaoLoi(res)).toBe("Không tìm thấy dự án");
  });

  it("other statuses without a JSON body stay 'không kết nối được'", async () => {
    expect(await thongBaoLoi(new Response("<html>bad gateway</html>", { status: 502 }))).toBe(LOI_KHONG_RO);
  });

  it("a genuine network failure stays 'không kết nối được'", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new TypeError("network"); }));
    expect(await docJSON("/api/v1/x")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });

  it("docJSON surfaces both through the same path", async () => {
    stub(404, "404 page not found", "text/plain");
    expect(await docJSON("/api/v1/budget-reports")).toEqual({ ok: false, thongBao: LOI_CHUA_HO_TRO });
    stub(400, JSON.stringify({ code: "invalid", message: "bo_phan: thiếu tên bộ phận" }));
    expect(await docJSON("/api/v1/org-units")).toEqual({ ok: false, thongBao: "Thiếu tên bộ phận" });
  });

  it("errorMessageOr keeps the caller's fallback for a non-JSON body", async () => {
    expect(await errorMessageOr(new Response("<html/>", { status: 413 }), "Tệp quá lớn.")).toBe("Tệp quá lớn.");
  });
});
