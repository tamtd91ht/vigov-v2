import { describe, expect, it } from "vitest";

import {
  CAU_THIEU_QUYEN,
  PHAN_CHUA_DUNG,
  MO_TA_TRANG,
  demSoKhoi,
  nhanSoKhoi,
} from "./nhan-danh-ba";

/**
 * Bốn nhóm phép kiểm dưới đây canh những quyết định mà một lần sửa "cho gọn" sẽ phá mà không làm
 * đỏ gì khác: một con số bịa ra khi chưa đọc xong, một lời hứa về nút Mini App, và một danh sách
 * giải thích bị rút ngắn dần cho tới khi không còn nói gì.
 */

describe("thẻ KPI số khối / đơn vị", () => {
  it("chưa đọc xong thì KHÔNG phải số 0", () => {
    // Một thẻ hiện `0` trong lúc còn đang đọc là một câu khẳng định sai về sơ đồ tổ chức của một
    // cơ quan nhà nước — và nó trông y hệt câu đúng, nên không ai phát hiện.
    expect(demSoKhoi(null)).toEqual({ pha: "dangDoc" });
    expect(nhanSoKhoi(demSoKhoi(null))).not.toBe("0");
    expect(nhanSoKhoi(demSoKhoi(null))).not.toBe("");
  });

  it("đọc xong và rỗng thì ĐÚNG là 0 — khác hẳn ca trên", () => {
    // Một xã vừa onboard có sơ đồ tổ chức rỗng thật. Đây là con số đúng, và nó gọi người quản trị
    // đi khai bộ phận.
    expect(demSoKhoi({ ok: true, duLieu: { items: [] } })).toEqual({ pha: "xong", so: 0 });
    expect(nhanSoKhoi({ pha: "xong", so: 0 })).toBe("0");
  });

  it("đếm đúng số mục của danh mục", () => {
    expect(demSoKhoi({ ok: true, duLieu: { items: [{}, {}, {}, {}, {}] } })).toEqual({
      pha: "xong",
      so: 5,
    });
  });

  it("đọc hỏng thì mang theo câu của máy chủ, và KHÔNG ra một con số", () => {
    const kq = demSoKhoi({ ok: false, thongBao: "Phiên làm việc đã hết hạn." });
    expect(kq).toEqual({ pha: "loi", thongBao: "Phiên làm việc đã hết hạn." });
    expect(nhanSoKhoi(kq)).not.toMatch(/^\d+$/);
  });
});

describe("câu mô tả trang không hứa thứ màn hình không có", () => {
  it("KHÔNG nhắc tới nút Thêm vào danh bạ Mini App", () => {
    // Đặc tả §2 viết nguyên văn "Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App'".
    // Câu mở #12 (chốt 22/09/2026) đã bỏ hẳn thao tác ấy, nên in nguyên vế đó ra là hứa với cán
    // bộ một nút họ sẽ đi tìm và không thấy.
    expect(MO_TA_TRANG).not.toContain("Thêm vào danh bạ Mini App");
    // Nửa đầu của đặc tả thì giữ: màn này đúng là toàn bộ cán bộ của xã.
    expect(MO_TA_TRANG).toContain("Toàn bộ cán bộ của xã");
  });
});

describe("câu thiếu quyền nói ra tên khoá", () => {
  it("chứa đúng khoá `admin.user`", () => {
    // "Bạn không có quyền" trống trơn là câu khiến cán bộ gọi lên tỉnh hỏi mình thiếu quyền gì.
    expect(CAU_THIEU_QUYEN).toContain("admin.user");
  });

  it("không nhắc `content.read` — đặc tả §9.4 đề nghị khoá ấy, hợp đồng thì không", () => {
    // Hai tuyến màn này gọi đều khai `admin.user` ở máy chủ. Nói tên một khoá khác là chỉ cán bộ
    // đi xin đúng thứ không mở được màn này.
    expect(CAU_THIEU_QUYEN).not.toContain("content.read");
  });
});

describe("danh sách phần chưa mở", () => {
  it("mỗi mục nói cả VIỆC lẫn LÝ DO, không mục nào rỗng", () => {
    // Một mục chỉ có tên việc là một mục nói "cái này không có" mà không nói cái gì mở khoá nó —
    // tức lần sau vẫn phải ngồi suy lại từ đầu.
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.trim()).not.toBe("");
      expect(p.viSao.trim().length).toBeGreaterThan(40);
    }
  });

  it("nêu đủ bốn thứ đặc tả vẽ mà hợp đồng hoặc khách chưa cho phép", () => {
    const tatCa = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(tatCa).toContain("tìm kiếm");
    expect(tatCa).toContain("Mini App");
    expect(tatCa).toContain("Excel");
    expect(tatCa).toContain("Xoá khỏi danh bạ");
  });

  it("ca Mini App viện dẫn đúng quyết định #12, không viện một lý do kỹ thuật suông", () => {
    // Nếu lý do chỉ là "chưa có trường trong lược đồ" thì người sau sẽ thêm một cột và bật nút —
    // trong khi thứ chặn thật là một quyết định về dữ liệu cá nhân: phải hỏi ý từng người và lưu
    // lại sự đồng ý kèm thời điểm (Nghị định 13/2023/NĐ-CP).
    const mucMiniApp = PHAN_CHUA_DUNG.find((p) => p.ten.includes("MINI APP"));
    expect(mucMiniApp).toBeDefined();
    expect(mucMiniApp?.viSao).toContain("#12");
    expect(mucMiniApp?.viSao).toContain("đồng ý");
  });
});
