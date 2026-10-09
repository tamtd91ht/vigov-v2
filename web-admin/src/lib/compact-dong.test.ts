import { describe, expect, it } from "vitest";

import { compactDong, compactDongReport } from "./compact-dong";

// Tester screenshot 06/10/2026: "690.000.000 đồn" clipped and "9.640.000.000 / đồng" broken over two
// lines in a dashboard tile. A tile shows the sum short, like the prototype's "4.317 tỷ"; the exact
// amount stays on hover and in the Thu – Chi register.
describe("compactDong", () => {
  it("writes billions as tỷ with at most two decimals, Vietnamese separators", () => {
    expect(compactDong(9_640_000_000)).toBe("9,64 tỷ đồng");
    expect(compactDong(8_150_000_000)).toBe("8,15 tỷ đồng");
    expect(compactDong(4_317_000_000_000)).toBe("4.317 tỷ đồng");
  });

  it("writes millions as triệu with at most one decimal", () => {
    expect(compactDong(690_000_000)).toBe("690 triệu đồng");
    expect(compactDong(1_250_000)).toBe("1,3 triệu đồng");
  });

  it("keeps the sign of a negative balance and writes small sums in full", () => {
    expect(compactDong(-690_000_000)).toBe("-690 triệu đồng");
    expect(compactDong(950_000)).toBe("950.000 đồng");
    expect(compactDong(0)).toBe("0 đồng");
  });
});

// /bao-cao only (owner 09/10/2026, D6): the prototype's `formatDongShort`, no "đồng".
describe("compactDongReport", () => {
  it("the prototype's three bands: thousand tỷ grouped, tỷ with one decimal comma, triệu rounded", () => {
    expect(compactDongReport(4_316_800_000_000)).toBe("4.317 tỷ");
    expect(compactDongReport(3_400_000_000)).toBe("3,4 tỷ");
    expect(compactDongReport(9_640_000_000)).toBe("9,6 tỷ");
    expect(compactDongReport(690_000_000)).toBe("690 triệu");
    expect(compactDongReport(950_000)).toBe("950.000");
  });

  it("keeps the sign of a negative balance", () => {
    expect(compactDongReport(-3_400_000_000)).toBe("-3,4 tỷ");
    expect(compactDongReport(-690_000_000)).toBe("-690 triệu");
  });

  it("leaves the shared compactDong untouched", () => {
    expect(compactDong(9_640_000_000)).toBe("9,64 tỷ đồng");
  });
});
