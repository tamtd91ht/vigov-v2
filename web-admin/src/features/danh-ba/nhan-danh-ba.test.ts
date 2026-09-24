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
  it("KHÔNG hứa thao tác chọn nhiều người — nói công khai làm cho TỪNG người", () => {
    // Đặc tả §2 viết nguyên văn "Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App'".
    // "Chọn người" là thao tác hàng loạt mà #12 đã bỏ.
    expect(MO_TA_TRANG).not.toContain("Chọn người");
    expect(MO_TA_TRANG).toContain("từng người");
    expect(MO_TA_TRANG).toContain("đồng ý");
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

  it("ô tìm và hai bộ lọc ĐÃ dựng — không còn dòng 'chưa mở' nào nói về chúng", () => {
    // Một dòng "chưa mở" cho thứ đang nằm ngay trên màn hình là một câu sai cán bộ đọc mỗi ngày.
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toMatch(/Ô tìm|Bộ lọc theo khối/);
      expect(p.viSao).not.toContain("không nhận tham số tìm kiếm");
    }
  });

  it("nêu đủ những thứ đặc tả vẽ mà hợp đồng hoặc khách chưa cho phép", () => {
    const tatCa = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(tatCa).toContain("TỔNG SỐ CÁN BỘ");
    expect(tatCa).toContain("Mini App");
    expect(tatCa).toContain("Excel");
    expect(tatCa).toContain("Xoá khỏi danh bạ");
  });

  it("nút Mini App và dòng Có Zalo ĐÃ dựng — không còn dòng 'chưa mở' nào nói về chúng", () => {
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toMatch(/nút thêm\/rút|Có Zalo|thứ tự hiển thị/i);
    }
  });

  it("nói thật rằng bà con CHƯA thấy danh bạ trên Mini App — tuyến đọc công khai chưa có", () => {
    // Người quản trị vừa bấm công khai sẽ mở Mini App ra xem. Không nói trước thì họ kết luận
    // thao tác hỏng, hoặc tệ hơn, tưởng số đã lên kênh công khai trong khi chưa.
    const muc = PHAN_CHUA_DUNG.find((p) => p.ten.includes("Bà con xem danh bạ"));
    expect(muc?.viSao).toContain("chưa có trong hợp đồng");
  });
});
