import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import {
  DAN_LICH_LAM_VIEC,
  nhanCa,
  nhanGio,
  nhanNgay,
  nhanNgayLamBuRong,
  nhanNgayNghiRong,
  tenThu,
} from "./nhan-lich-lam-viec";

describe("tên thứ theo ISO 8601", () => {
  it("1 là thứ Hai và 7 là Chủ nhật", () => {
    // ISO, KHÔNG phải `Date.getDay()` (0 là Chủ nhật) và không phải số thứ tiếng Việt (thứ Hai
    // là 2). Quy đổi nhầm giữa hai hệ là lệch một ngày — và một ca làm việc lệch một ngày làm
    // mọi hạn đi qua ngày ấy bị đếm sai.
    expect(tenThu(1)).toBe("Thứ Hai");
    expect(tenThu(7)).toBe("Chủ nhật");
  });

  it("bảy thứ là bảy tên khác nhau", () => {
    const ten = [1, 2, 3, 4, 5, 6, 7].map(tenThu);
    expect(new Set(ten).size).toBe(7);
  });

  it("số ngoài 1..7 nói ra là không hợp lệ, không in một con số trần", () => {
    expect(tenThu(0)).toContain("không hợp lệ");
    expect(tenThu(8)).toContain("không hợp lệ");
  });
});

describe("giờ tường và ca làm việc", () => {
  it("HH:MM:SS đọc gọn còn HH:MM", () => {
    expect(nhanGio("07:30:00")).toBe("07:30");
  });

  it("một ca đọc thành khoảng", () => {
    expect(nhanCa("07:30:00", "11:30:00")).toBe("07:30 – 11:30");
  });

  it("chuỗi sai khuôn hiện nguyên văn", () => {
    expect(nhanGio("hong")).toBe("hong");
  });
});

describe("ngày", () => {
  it("YYYY-MM-DD đọc thành dd/MM/yyyy", () => {
    expect(nhanNgay("2026-09-02")).toBe("02/09/2026");
  });

  it("KHÔNG dựng `Date` ở đâu trong module này", () => {
    // Đọc thẳng mã nguồn, vì một ca test gọi hàm không bắt được lỗi này: cả hai cách viết đều
    // trả đúng kết quả trên máy đặt múi giờ UTC+7, và chỉ sai trên máy của một cán bộ đặt múi
    // giờ khác — tức là sai trong sản xuất và không sai ở đây. Một ngày nghỉ lễ lùi một ngày là
    // một hạn xử lý đếm xuyên qua ngày trụ sở đóng cửa.
    const nguon = readFileSync(new URL("./nhan-lich-lam-viec.ts", import.meta.url), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split("\n")
      .filter((d) => !/^\s*\/\//.test(d))
      .join("\n");

    expect(nguon).not.toMatch(/new\s+Date\s*\(/);
    expect(nguon).not.toMatch(/Date\s*\.\s*(parse|now)/);
  });
});

describe("câu chữ nói rõ đây là cấu hình CỦA XÃ", () => {
  it("câu dẫn nói thẳng rằng ba bảng này không phải quy định chung của hệ thống", () => {
    // Không phải chữ cho đẹp: một cán bộ đọc màn hình này rồi tưởng "hệ thống quy định giờ làm
    // như vậy" sẽ không bao giờ nghĩ tới việc phải sửa nó — và hạn xử lý của cả xã tính sai
    // trong im lặng.
    expect(DAN_LICH_LAM_VIEC).toContain("riêng của đơn vị này");
    expect(DAN_LICH_LAM_VIEC).toContain("không phải quy định chung");
  });

  it("KHÔNG một phép cộng giờ làm việc nào nằm trong module này", () => {
    // `identity` sở hữu cả ba bảng VÀ phép cộng (`identity.AdvanceWorkingHours`). Một bản tính
    // thứ hai ở trình duyệt sẽ lệch bản của máy chủ vào đúng ngày lễ (ADR 0007, luật 10 #4).
    const nguon = readFileSync(new URL("./nhan-lich-lam-viec.ts", import.meta.url), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split("\n")
      .filter((d) => !/^\s*\/\//.test(d))
      .join("\n");

    expect(nguon).not.toMatch(/setHours|getTime|addDays|cong(Gio|Ngay)|tinhHan/i);
  });

  it("năm nghỉ lễ rỗng và năm làm bù rỗng nói HAI câu khác nhau", () => {
    // Một năm không có ngày làm bù nào là chuyện bình thường của phần lớn các năm. Một năm chưa
    // khai ngày nghỉ lễ nào là đơn vị chưa nhập lịch. Hai việc khác nhau thì không được nhìn
    // giống nhau.
    expect(nhanNgayNghiRong(2026)).not.toBe(nhanNgayLamBuRong(2026));
    expect(nhanNgayNghiRong(2026)).toContain("2026");
    expect(nhanNgayLamBuRong(2026)).toContain("2026");
  });
});
