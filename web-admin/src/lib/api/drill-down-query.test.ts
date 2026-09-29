import { afterEach, describe, expect, it, vi } from "vitest";

import { laySoNhiemVu } from "./tasks";
import { laySoPhanAnh } from "./citizen-reports";
import { laySoVanBanDen } from "./documents";

/**
 * Ba tuyến danh sách nhận lọc Tổng quan (SRS M7.2.2). Canh CHUỖI TRUY VẤN THẬT đi ra `fetch`, vì
 * đó là chỗ con số và số dòng khớp hay lệch nhau:
 *
 *   - số liệu tồn KHÔNG mang kỳ, kể cả khi bên gọi lỡ truyền kỳ vào;
 *   - văn bản `open`/`overdue` không bao giờ mang `from`/`to` — máy chủ trả 400 cho đúng ca ấy;
 *   - `from`/`to` không kèm `metric` không bao giờ đi lên.
 */

const FROM = "2026-09-20T17:00:00Z";
const TO = "2026-09-27T17:00:00+07:00";

function stubFetch(): () => URLSearchParams {
  const fake = vi.fn(
    async () =>
      new Response(JSON.stringify({ items: [], next_cursor: "", has_more: false }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fake);
  return () => {
    const [url] = fake.mock.calls[0] as unknown as [string];
    return new URL(url, "http://xa.test").searchParams;
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("GET /api/v1/tasks", () => {
  it("số liệu tồn: chỉ metric, KHÔNG from/to dù bên gọi truyền vào", async () => {
    const q = stubFetch();
    await laySoNhiemVu({ metric: "overdue", from: FROM, to: TO, limit: 20 });
    expect(q().get("metric")).toBe("overdue");
    expect(q().has("from")).toBe(false);
    expect(q().has("to")).toBe(false);
  });

  it("số liệu theo kỳ: metric + from + to, mã hoá đúng dấu +", async () => {
    const q = stubFetch();
    await laySoNhiemVu({ metric: "completed", from: FROM, to: TO });
    expect(q().get("metric")).toBe("completed");
    expect(q().get("from")).toBe(FROM);
    // `+07:00` về tới máy chủ nguyên vẹn, không thành dấu cách.
    expect(q().get("to")).toBe(TO);
  });

  it("from/to không kèm metric thì không đi lên", async () => {
    const q = stubFetch();
    await laySoNhiemVu({ from: FROM, to: TO });
    expect([...q().keys()]).toEqual([]);
  });
});

describe("GET /api/v1/citizen-reports", () => {
  it("in_progress là số liệu tồn: không from/to", async () => {
    const q = stubFetch();
    await laySoPhanAnh({ metric: "in_progress", from: FROM, to: TO });
    expect(q().get("metric")).toBe("in_progress");
    expect(q().has("from")).toBe(false);
    expect(q().has("to")).toBe(false);
  });

  it("late theo kỳ: đủ ba tham số", async () => {
    const q = stubFetch();
    await laySoPhanAnh({ metric: "late", from: FROM, to: TO, limit: 20 });
    expect(q().get("metric")).toBe("late");
    expect(q().get("from")).toBe(FROM);
    expect(q().get("to")).toBe(TO);
    expect(q().get("limit")).toBe("20");
  });
});

describe("GET /api/v1/incoming-documents", () => {
  it("open và overdue KHÔNG BAO GIỜ mang from/to (máy chủ trả 400)", async () => {
    for (const metric of ["open", "overdue"] as const) {
      const q = stubFetch();
      await laySoVanBanDen({ metric, from: FROM, to: TO });
      expect(q().get("metric"), metric).toBe(metric);
      expect(q().has("from"), metric).toBe(false);
      expect(q().has("to"), metric).toBe(false);
      vi.unstubAllGlobals();
    }
  });

  it("arrived mang đủ kỳ, và KHÔNG mang year khi màn không gửi", async () => {
    const q = stubFetch();
    await laySoVanBanDen({ metric: "arrived", from: FROM, to: TO });
    expect(q().get("metric")).toBe("arrived");
    expect(q().get("from")).toBe(FROM);
    expect(q().get("to")).toBe(TO);
    expect(q().has("year")).toBe(false);
  });

  it("from/to không kèm metric thì không đi lên (máy chủ trả 400)", async () => {
    const q = stubFetch();
    await laySoVanBanDen({ from: FROM, to: TO });
    expect(q().has("from")).toBe(false);
    expect(q().has("to")).toBe(false);
  });
});
