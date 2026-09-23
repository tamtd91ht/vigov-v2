import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

import {
  CHUA_TACH_NHIEM_VU,
  DAU_GACH,
  dongMeta,
  nhanBadge,
  nhanNgayHop,
  nhanTienDoKetLuan,
  PHAN_CHUA_DUNG,
  soThuTuKetLuan,
  tachKetLuan,
  tachThanhPhan,
} from "./nhan-bien-ban";

function ketLuan(sua: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
  return {
    id: "01JKL1",
    ordinal: 1,
    content: "Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam – Bình Trị.",
    task_count: 0,
    task_done_count: 0,
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

function bienBan(sua: Partial<petitions_bienBanRa> = {}): petitions_bienBanRa {
  return {
    id: "01JBB1",
    title: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
    held_on: "2026-08-05",
    reference_no: "31/BB-UBND",
    location: "Phòng họp UBND xã",
    chaired_by: "",
    conclusions: [ketLuan()],
    task_count: 3,
    task_done_count: 1,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

describe("ngày họp là một NGÀY LỊCH, không phải một mốc thời gian", () => {
  it("`2026-08-05` thành `5/8/2026`, không số 0 đứng đầu", () => {
    expect(nhanNgayHop("2026-08-05")).toBe("5/8/2026");
    expect(nhanNgayHop("2026-12-31")).toBe("31/12/2026");
  });

  it("KHÔNG đi qua `new Date` — nguồn tệp không có phép dựng ngày nào", () => {
    // Đây là bài kiểm về MÃ NGUỒN vì hậu quả không hiện ra ở máy chạy test: `new Date("2026-08-05")`
    // là nửa đêm UTC, và chỉ trình duyệt ở múi giờ âm mới vẽ ra ngày 4/8. Một bài kiểm chạy ở UTC
    // sẽ xanh vĩnh viễn trong khi màn hình của một xã hiện sai ngày họp.
    const nguon = readFileSync(new URL("./nhan-bien-ban.ts", import.meta.url), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split("\n")
      .filter((d) => !/^\s*\/\//.test(d))
      .join("\n");

    expect(nguon).not.toMatch(/new\s+Date\s*\(/);
    expect(nguon).not.toMatch(/Date\s*\.\s*parse/);
    expect(nguon).not.toMatch(/toLocaleDateString/);
  });

  it("chuỗi không đọc được thì là dấu gạch, KHÔNG phải một ngày bịa ra", () => {
    expect(nhanNgayHop("")).toBe(DAU_GACH);
    expect(nhanNgayHop("05/08/2026")).toBe(DAU_GACH);
    expect(nhanNgayHop("2026-08-05T00:00:00Z")).toBe(DAU_GACH);
    expect(nhanNgayHop("2026-13-05")).toBe(DAU_GACH);
  });
});

describe("dòng meta: phần nào thiếu thì bỏ", () => {
  it("đủ ba phần thì nối bằng ` · `", () => {
    expect(dongMeta(bienBan())).toBe("5/8/2026 · 31/BB-UBND · Phòng họp UBND xã");
  });

  it("thiếu số hiệu thì BỎ HẲN đoạn ấy, không để một chỗ trống giữa hai dấu chấm", () => {
    expect(dongMeta(bienBan({ reference_no: "" }))).toBe("5/8/2026 · Phòng họp UBND xã");
  });

  it("chỉ còn ngày họp thì dòng meta chỉ có ngày họp", () => {
    expect(dongMeta(bienBan({ reference_no: "", location: "" }))).toBe("5/8/2026");
  });

  it("ngày họp không đọc được thì nó cũng bị bỏ khỏi dòng meta", () => {
    // Một dấu gạch đứng lẫn giữa số hiệu và địa điểm trông như một trường rỗng chứ không như một
    // giá trị hỏng.
    expect(dongMeta(bienBan({ held_on: "" }))).toBe("31/BB-UBND · Phòng họp UBND xã");
  });
});

describe("hai bộ đếm, và chúng đến từ MÁY CHỦ", () => {
  it("badge dùng `task_count`/`task_done_count` của biên bản, không cộng lại từ mảng kết luận", () => {
    // Biên bản có MỘT kết luận với 0 nhiệm vụ, nhưng máy chủ nói 1/3. Cộng lại ở client sẽ ra
    // `0/0` — một con số thứ hai của cùng một sự thật, và là con số lãnh đạo đọc.
    expect(nhanBadge(bienBan())).toBe("1 kết luận · 1/3 nhiệm vụ xong");
  });

  it("`Chưa tách thành nhiệm vụ nào` KHÁC `0/3 nhiệm vụ đã hoàn thành`", () => {
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 0, task_done_count: 0 }))).toBe(
      CHUA_TACH_NHIEM_VU,
    );
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 3, task_done_count: 0 }))).toBe(
      "0/3 nhiệm vụ đã hoàn thành",
    );
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 1, task_done_count: 1 }))).toBe(
      "1/1 nhiệm vụ đã hoàn thành",
    );
  });
});

describe("số thứ tự kết luận — số ĐÃ CẤP, không phải vị trí trong danh sách", () => {
  it("lấy đúng `ordinal` máy chủ trả, kể cả khi nó không liên tục", () => {
    // Xoá mềm kết luận ② thì kết luận kế tiếp mang số ④. Khoảng trống ấy ĐÚNG (luật 7, bất biến
    // 3): số đã cấp thì không bao giờ cấp lại, và biên bản giấy đã in mang con số cũ.
    expect(soThuTuKetLuan(ketLuan({ ordinal: 4 }))).toBe(4);
    expect(soThuTuKetLuan(ketLuan({ ordinal: 1 }))).toBe(1);
  });
});

describe("tách ô nhập nhiều dòng", () => {
  it("mỗi dòng một mục, bỏ dòng trắng, giữ nguyên thứ tự người gõ", () => {
    expect(tachThanhPhan("Ông A\n\n  Bà B  \n")).toEqual(["Ông A", "Bà B"]);
    expect(tachKetLuan("Kết luận một\nKết luận hai\n")).toEqual([
      "Kết luận một",
      "Kết luận hai",
    ]);
  });

  it("ô trống cho ra danh sách rỗng — §7.3: biên bản không có kết luận vẫn lưu được", () => {
    expect(tachKetLuan("")).toEqual([]);
    expect(tachKetLuan("   \n  ")).toEqual([]);
  });
});

describe("phần chưa dựng được", () => {
  it("mỗi mục có tên và lý do, không mục nào rỗng", () => {
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.trim()).not.toBe("");
      expect(p.viSao.trim()).not.toBe("");
    }
  });

  /**
   * CA NÀY TRƯỚC ĐÒI "nút `Tách thành nhiệm vụ` nằm trong danh sách". Nó đúng cho tới 24/09/2026,
   * khi biểu mẫu "Giao việc mới" chưa có. Nút nay đã dựng; thứ còn thiếu là bảng ĐIỀN SẴN của §3.
   * Ca đổi theo hành vi chứ không bị gỡ: nếu một ngày ai đó dựng nốt phần điền sẵn mà quên xoá mục
   * này, hoặc ngược lại xoá mục mà chưa dựng, thì đúng ca này đỏ.
   */
  it("thứ CÒN THIẾU của §3 là ĐIỀN SẴN, và lý do nêu đúng chỗ không sửa được lượt này", () => {
    const muc = PHAN_CHUA_DUNG.find((p) => p.ten.includes("ĐIỀN SẴN"));

    expect(muc).toBeDefined();
    // Lý do phải chỉ ĐÍCH DANH biểu mẫu dùng chung: người đọc sau cần biết sửa ở đâu, và biết vì
    // sao không được chép nó sang đây.
    expect(muc?.viSao).toContain("FormGiaoViec");
    expect(muc?.viSao).toContain("features/nhiem-vu/so-nhiem-vu.tsx");
  });
});
