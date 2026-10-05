import { describe, expect, it } from "vitest";

import { compactDong } from "./view";

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
