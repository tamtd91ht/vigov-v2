import { describe, expect, it } from "vitest";

import {
  drillDownKey,
  drillDownQuery,
  formatPeriod,
  isPeriodMetric,
  parseDrillDown,
  parseRfc3339,
  STOCK_PERIOD_LABEL,
} from "./drill-down";

/**
 * Bộ nhận lọc Tổng quan (SRS M7.2.2). Canh ba điều mà một lần sửa MỘT DÒNG phá được với `tsc` sạch:
 *
 *   1. Mọi đường dẫn hỏng là `invalid` — không bao giờ `none` (mất câu cảnh báo) và không bao giờ
 *      `active` với một phần đã "sửa hộ".
 *   2. Kỳ BẮT BUỘC ở số liệu theo kỳ và BỊ CẤM ở số liệu tồn, theo đúng bảng của từng máy chủ.
 *   3. Mốc là RFC3339 theo nghĩa của `time.Parse` của Go, không theo nghĩa rộng của `Date.parse`.
 */

// Tuần 21/09–27/09/2026 theo giờ Việt Nam, nửa mở: `to` là 00:00 thứ Hai 28/09.
const FROM = "2026-09-20T17:00:00Z";
const TO = "2026-09-27T17:00:00Z";

describe("không có tham số lọc", () => {
  it("vắng cả ba ⇒ none; tham số khác của màn không phải việc của hàm này", () => {
    expect(parseDrillDown("tasks", {})).toEqual({ kind: "none" });
    expect(parseDrillDown("tasks", { status: "moi-giao", sort: "code" })).toEqual({ kind: "none" });
  });
});

describe("số liệu hợp lệ", () => {
  it("số liệu tồn không kèm kỳ ⇒ active, kỳ in 'tính đến hiện tại'", () => {
    const d = parseDrillDown("tasks", { metric: "overdue" });
    expect(d).toEqual({
      kind: "active",
      metric: "overdue",
      label: "Quá hạn",
      period: null,
      periodLabel: STOCK_PERIOD_LABEL,
    });
    expect(STOCK_PERIOD_LABEL).toBe("tính đến hiện tại");
  });

  it("số liệu theo kỳ kèm kỳ ⇒ active, giữ NGUYÊN chuỗi gốc để gửi lại", () => {
    const d = parseDrillDown("citizen-reports", { metric: "late", from: FROM, to: TO });
    expect(d.kind).toBe("active");
    if (d.kind !== "active") return;
    expect(d.label).toBe("Trễ hạn trong kỳ");
    expect(d.period).toEqual({ from: FROM, to: TO });
    // `to` không thuộc kỳ: ngày cuối in ra là Chủ nhật 27/09, không phải thứ Hai 28/09.
    expect(d.periodLabel).toBe("21/09/2026–27/09/2026");
  });

  it("đủ nhãn cho cả mười bốn số liệu của ba sổ", () => {
    const nhan = (r: Parameters<typeof parseDrillDown>[0], metric: string, ky: boolean) => {
      const d = parseDrillDown(r, ky ? { metric, from: FROM, to: TO } : { metric });
      return d.kind === "active" ? d.label : d.kind;
    };
    expect(nhan("tasks", "in_progress", false)).toBe("Đang thực hiện");
    expect(nhan("tasks", "overdue", false)).toBe("Quá hạn");
    expect(nhan("tasks", "suspended", false)).toBe("Tạm dừng");
    expect(nhan("tasks", "completed", true)).toBe("Hoàn thành trong kỳ");
    expect(nhan("tasks", "on_time_sample", true)).toBe("Có hạn trong kỳ");
    expect(nhan("tasks", "on_time", true)).toBe("Hoàn thành đúng hạn");
    expect(nhan("citizen-reports", "in_progress", false)).toBe("Đang xử lý");
    expect(nhan("citizen-reports", "received", true)).toBe("Nhận vào trong kỳ");
    expect(nhan("citizen-reports", "on_time_sample", true)).toBe("Có hạn trong kỳ");
    expect(nhan("citizen-reports", "on_time", true)).toBe("Đúng hạn");
    expect(nhan("citizen-reports", "late", true)).toBe("Trễ hạn trong kỳ");
    expect(nhan("incoming-documents", "arrived", true)).toBe("Đến trong kỳ");
    expect(nhan("incoming-documents", "open", false)).toBe("Chưa xử lý xong");
    expect(nhan("incoming-documents", "overdue", false)).toBe("Quá hạn xử lý");
  });
});

describe("đường dẫn hỏng ⇒ invalid, không bao giờ none hay active", () => {
  it("số liệu lạ, số liệu của SỔ KHÁC, số liệu rỗng, khoá của nguyên mẫu", () => {
    expect(parseDrillDown("tasks", { metric: "bogus" }).kind).toBe("invalid");
    // `suspended` là số liệu của nhiệm vụ, không của phản ánh; `open` là của văn bản.
    expect(parseDrillDown("citizen-reports", { metric: "suspended" }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "open" }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "" }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "constructor" }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "toString" }).kind).toBe("invalid");
  });

  it("số liệu theo kỳ THIẾU kỳ, hay thiếu một đầu", () => {
    expect(parseDrillDown("tasks", { metric: "completed" }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "completed", from: FROM }).kind).toBe("invalid");
    expect(parseDrillDown("incoming-documents", { metric: "arrived", to: TO }).kind).toBe("invalid");
  });

  it("số liệu tồn KÈM kỳ — kể cả chỉ một đầu", () => {
    expect(parseDrillDown("incoming-documents", { metric: "open", from: FROM, to: TO }).kind).toBe(
      "invalid",
    );
    expect(parseDrillDown("incoming-documents", { metric: "overdue", from: FROM }).kind).toBe(
      "invalid",
    );
    expect(parseDrillDown("tasks", { metric: "in_progress", to: TO }).kind).toBe("invalid");
  });

  it("kỳ không kèm số liệu", () => {
    expect(parseDrillDown("incoming-documents", { from: FROM, to: TO }).kind).toBe("invalid");
  });

  it("from không trước hẳn to", () => {
    expect(parseDrillDown("tasks", { metric: "completed", from: TO, to: FROM }).kind).toBe("invalid");
    expect(parseDrillDown("tasks", { metric: "completed", from: FROM, to: FROM }).kind).toBe(
      "invalid",
    );
  });

  it("một tham số lặp hai lần", () => {
    expect(parseDrillDown("tasks", { metric: ["overdue", "in_progress"] }).kind).toBe("invalid");
  });

  it("mốc không phải RFC3339", () => {
    expect(parseDrillDown("tasks", { metric: "completed", from: "2026-09-21", to: TO }).kind).toBe(
      "invalid",
    );
  });
});

describe("parseRfc3339 — theo nghĩa của time.Parse, không theo Date.parse", () => {
  it("nhận Z, độ lệch có dấu, phần lẻ của giây", () => {
    expect(parseRfc3339("2026-09-21T00:00:00Z")).toBe(Date.UTC(2026, 8, 21));
    expect(parseRfc3339("2026-09-21T07:00:00+07:00")).toBe(Date.UTC(2026, 8, 21));
    expect(parseRfc3339("2026-09-20T19:30:00-04:30")).toBe(Date.UTC(2026, 8, 21));
    expect(parseRfc3339("2026-09-21T00:00:00.5Z")).toBe(Date.UTC(2026, 8, 21, 0, 0, 0, 500));
    expect(parseRfc3339("2026-09-21T00:00:00.123456789Z")).toBe(
      Date.UTC(2026, 8, 21, 0, 0, 0, 123),
    );
  });

  it("từ chối những gì Date.parse nhận mà Go từ chối", () => {
    // Date.parse lăn 30/02 sang 02/03 và đọc 24:00 thành ngày hôm sau.
    expect(Number.isNaN(Date.parse("2026-02-30T00:00:00Z"))).toBe(false);
    expect(parseRfc3339("2026-02-30T00:00:00Z")).toBeNull();
    expect(parseRfc3339("2026-09-21T24:00:00Z")).toBeNull();
    expect(parseRfc3339("2026-09-21 00:00:00Z")).toBeNull();
    expect(parseRfc3339("2026-13-01T00:00:00Z")).toBeNull();
    expect(parseRfc3339("2026-09-21T00:60:00Z")).toBeNull();
    expect(parseRfc3339("2026-09-21T00:00:60Z")).toBeNull();
    expect(parseRfc3339("2026-09-21T00:00:00+24:00")).toBeNull();
  });

  it("từ chối mốc thiếu múi giờ, thiếu giây, chữ thường, và dấu + đã bị đọc thành dấu cách", () => {
    expect(parseRfc3339("2026-09-21T00:00:00")).toBeNull();
    expect(parseRfc3339("2026-09-21T00:00Z")).toBeNull();
    expect(parseRfc3339("2026-09-21t00:00:00z")).toBeNull();
    // `+07:00` không mã hoá trong đường dẫn về tới đây thành ` 07:00`. Không sửa hộ.
    expect(parseRfc3339("2026-09-21T00:00:00 07:00")).toBeNull();
  });

  it("29/02 chỉ có ở năm nhuận", () => {
    expect(parseRfc3339("2028-02-29T00:00:00Z")).toBe(Date.UTC(2028, 1, 29));
    expect(parseRfc3339("2026-02-29T00:00:00Z")).toBeNull();
  });
});

describe("in kỳ theo giờ Việt Nam", () => {
  it("to − 1ms, múi Asia/Ho_Chi_Minh, dd/MM/yyyy có đệm số 0", () => {
    // Tháng 9/2026 theo giờ ta: 01/09 00:00+07 → 01/10 00:00+07.
    const from = Date.UTC(2026, 7, 31, 17);
    const to = Date.UTC(2026, 8, 30, 17);
    expect(formatPeriod(from, to)).toBe("01/09/2026–30/09/2026");
  });

  it("kỳ kết thúc giữa ngày thì ngày ấy thuộc kỳ", () => {
    // 28/09 10:00 giờ ta.
    expect(formatPeriod(Date.UTC(2026, 8, 20, 17), Date.UTC(2026, 8, 28, 3))).toBe(
      "21/09/2026–28/09/2026",
    );
  });
});

describe("chuyển xuống bộ gọi API", () => {
  it("drillDownQuery: tồn chỉ có metric; theo kỳ có đủ ba; none/invalid rỗng", () => {
    expect(drillDownQuery(parseDrillDown("tasks", { metric: "suspended" }))).toEqual({
      metric: "suspended",
    });
    expect(
      drillDownQuery(parseDrillDown("incoming-documents", { metric: "arrived", from: FROM, to: TO })),
    ).toEqual({ metric: "arrived", from: FROM, to: TO });
    expect(drillDownQuery(parseDrillDown("tasks", {}))).toEqual({});
    expect(drillDownQuery(parseDrillDown("tasks", { metric: "bogus" }))).toEqual({});
  });

  it("isPeriodMetric khớp bảng của từng máy chủ", () => {
    expect(isPeriodMetric("tasks", "completed")).toBe(true);
    expect(isPeriodMetric("tasks", "overdue")).toBe(false);
    expect(isPeriodMetric("citizen-reports", "in_progress")).toBe(false);
    expect(isPeriodMetric("citizen-reports", "received")).toBe(true);
    expect(isPeriodMetric("incoming-documents", "arrived")).toBe(true);
    expect(isPeriodMetric("incoming-documents", "open")).toBe(false);
    expect(isPeriodMetric("incoming-documents", "overdue")).toBe(false);
  });

  it("drillDownKey đổi khi lọc đổi, để màn được dựng lại", () => {
    const a = drillDownKey(parseDrillDown("tasks", { metric: "overdue" }));
    const b = drillDownKey(parseDrillDown("tasks", { metric: "in_progress" }));
    const c = drillDownKey(parseDrillDown("tasks", {}));
    expect(new Set([a, b, c]).size).toBe(3);
  });
});
