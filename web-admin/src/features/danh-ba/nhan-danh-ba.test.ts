import { describe, expect, it } from "vitest";

import {
  CAU_THIEU_QUYEN,
  DANH_BA_RONG,
  KHONG_KHOP_LOC,
  PHAN_CHUA_DUNG,
  MO_TA_TRANG,
  demSoKhoi,
  departmentOptionText,
  filteredTotal,
  nhanSoKhoi,
  shownCountText,
  tallyFigures,
  unitCountText,
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

describe("huy hiệu số khối / đơn vị ở đầu trang", () => {
  it("chỉ ghép con số khi đã đếm xong; lúc đang đếm hay hỏng thì KHÔNG có chữ số nào", () => {
    expect(unitCountText({ pha: "xong", so: 3 })).toBe("3 khối / đơn vị");
    expect(unitCountText({ pha: "xong", so: 0 })).toBe("0 khối / đơn vị");
    expect(unitCountText({ pha: "dangDoc" })).not.toMatch(/\d/);
    expect(unitCountText({ pha: "loi", thongBao: "x" })).not.toMatch(/\d/);
  });
});

describe("câu chữ của bản mẫu (StaffDirectoryWorkspace.tsx), nguyên văn", () => {
  it("mô tả trang: chọn người rồi bấm 'Thêm vào danh bạ Mini App' — nay đúng với màn có cột chọn", () => {
    expect(MO_TA_TRANG).toBe(
      "Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm “Thêm vào danh bạ Mini App” để bà con gọi được.",
    );
  });

  it("danh bạ rỗng và rỗng-theo-bộ-lọc là HAI câu khác nhau", () => {
    expect(DANH_BA_RONG).toBe(
      "Chưa có cán bộ nào. Tải mẫu Excel ở nút “Nhập từ Excel”, điền theo bảng danh bạ xã đang dùng rồi tải lên.",
    );
    expect(KHONG_KHOP_LOC).not.toBe(DANH_BA_RONG);
  });

  it("dòng dưới bảng: 'Hiển thị n cán bộ.'", () => {
    expect(shownCountText(37)).toBe("Hiển thị 37 cán bộ.");
  });
});

describe("số liệu từ GET /api/v1/staff-counts", () => {
  const T = {
    total: 12,
    published: 5,
    departments: [
      { id: "BP-A", total: 7, published: 4 },
      { id: "BP-B", total: 3, published: 1 },
    ],
  };

  it("ba thẻ: tổng, đang hiện, số khối CÓ người — chưa đọc / đọc hỏng thì KHÔNG phải số", () => {
    expect(tallyFigures({ ok: true, duLieu: T })).toEqual({ total: "12", published: "5", departments: "2" });
    for (const v of Object.values(tallyFigures(null))) expect(v).toBe("đang đếm…");
    for (const v of Object.values(tallyFigures({ ok: false, thongBao: "x" }))) expect(v).not.toMatch(/\d/);
  });

  it("tổng theo bộ lọc (không chữ tìm): cả xã, theo khối, theo trạng thái; khối vắng mặt là 0", () => {
    expect(filteredTotal(T, "", null)).toBe(12);
    expect(filteredTotal(T, "", true)).toBe(5);
    expect(filteredTotal(T, "", false)).toBe(7);
    expect(filteredTotal(T, "BP-A", null)).toBe(7);
    expect(filteredTotal(T, "BP-A", false)).toBe(3);
    expect(filteredTotal(T, "BP-NONE", null)).toBe(0);
  });

  it("mục ô khối: 'Tên (đang hiện/tổng)'; chưa có số liệu thì chỉ tên, không đoán '(0/0)'", () => {
    expect(departmentOptionText("Văn phòng", "BP-A", T)).toBe("Văn phòng (4/7)");
    expect(departmentOptionText("Trạm y tế", "BP-NONE", T)).toBe("Trạm y tế (0/0)");
    expect(departmentOptionText("Văn phòng", "BP-A", null)).toBe("Văn phòng");
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

  it("chỉ còn 'Xoá đã chọn' — hai thẻ KPI đã đọc được số, Ảnh đại diện bỏ vì bản mẫu không có", () => {
    const tatCa = PHAN_CHUA_DUNG.map((p) => `${p.ten} ${p.viSao}`).join(" ");
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual(["Xoá đã chọn"]);
    expect(tatCa).toContain("lý do");
    // Nhập cán bộ từ Excel ĐÃ dựng — một dòng "chưa mở" cho nó là câu sai.
    expect(tatCa).not.toMatch(/Excel/);
    // Nút xoá MỘT dòng ĐÃ dựng — mục này chỉ nói về xoá NHIỀU người.
    expect(tatCa).not.toContain("Xoá khỏi danh bạ");
  });

  it("nút Mini App và dòng Có Zalo ĐÃ dựng — không còn dòng 'chưa mở' nào nói về chúng", () => {
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toMatch(/nút thêm\/rút|Có Zalo|thứ tự hiển thị/i);
    }
  });

  it("KHÔNG còn nói bà con chưa thấy danh bạ trên Mini App — tuyến `GET /api/v1/commune-staff` đã có", () => {
    // Câu cũ đúng cho tới khi tuyến đọc của Mini App được mở. Để nó lại là nói với người quản trị
    // rằng số chưa lên kênh công khai, trong khi nó đã lên — đúng chiều sai nguy hiểm nhất với một
    // số di động cá nhân.
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toContain("Bà con xem danh bạ");
      expect(p.viSao).not.toMatch(/bà con chưa thấy|Mini App đọc danh bạ/i);
    }
  });
});
