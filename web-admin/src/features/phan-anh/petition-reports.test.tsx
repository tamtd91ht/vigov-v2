// @vitest-environment jsdom
//
// jsdom for this file: the period picker is BEHAVIOUR (pressing `Quý này` asks a new [from, to)).

import { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { periodWindows, toQueryPeriod } from "@/features/dashboard/period";
import type { KetQua } from "@/lib/api/goi";
import { citizenReportBreakdownPath } from "@/lib/api/phieu-phan-anh";
import type { petitions_citizenReportBreakdownOut } from "@/lib/api/schema.gen";

import { PetitionReports, PetitionReportsView } from "./petition-reports";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

const BREAKDOWN: petitions_citizenReportBreakdownOut = {
  as_of: "2026-10-09T03:00:00Z",
  totals: { on_time_sample: 8, on_time: 6, late: 2, overdue: 3 },
  fields: [
    { field_code: "rac-thai", received: 10, finished: 7, on_time_sample: 7, on_time: 6, late: 1, rating_sample: 4, rating_sum: 18, overdue: 2 },
    { field_code: "", received: 3, finished: 0, on_time_sample: 1, on_time: 0, late: 1, rating_sample: 0, rating_sum: 0, overdue: 1 },
  ],
  units: [
    { org_unit_id: "01JBOPHAN", finished: 4, handling_sample: 4, handling_working_seconds: 4 * 2.5 * 3600 },
    { org_unit_id: "", finished: 1, handling_sample: 0, handling_working_seconds: 0 },
  ],
  residential_units: [
    { residential_unit_id: "01JTHON1", residential_unit_name: "Thôn Hà Lam", received: 5, overdue: 2 },
    { residential_unit_id: "", received: 8, overdue: 1 },
  ],
};

const UNITS = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);

describe("Báo cáo — what each section shows (ADR 0053 §Sửa đổi 09/10/2026)", () => {
  const html = renderToStaticMarkup(
    <PetitionReportsView breakdown={{ ok: true, duLieu: BREAKDOWN }} unitNames={UNITS} />,
  );

  it("on time / late: figures, the rate computed from the sample, the overdue STOCK labelled as such", () => {
    expect(html).toContain("Đúng hạn và trễ hạn");
    expect(html).toContain(">6<");
    expect(html).toContain(">2<");
    expect(html).toContain("75,0%");
    expect(html).toContain("tỷ lệ đúng hạn");
    expect(html).toContain("đang trễ hạn (hiện tại)");
    expect(html).toContain(">3<");
    // The overdue stock is never turned into a rate: no second percentage.
    expect(html.match(/%</g)?.length).toBe(1);
    expect(html).toContain('style="width:75%"');
  });

  it("by field: catalogue label; the `\"\"` row reads `Chưa phân loại`; satisfaction WITH its sample", () => {
    expect(html).toContain("Rác thải – Vệ sinh môi trường");
    expect(html).toContain("Chưa phân loại");
    expect(html).toContain("4,5/5 (4 phiếu)");
    expect(html).toContain("—");
  });

  it("by unit: names from the org-unit lookup; `\"\"` → `Chưa chuyển bộ phận`; average in WORKING hours", () => {
    expect(html).toContain("VĂN PHÒNG ĐẢNG ỦY");
    expect(html).toContain("Chưa chuyển bộ phận");
    expect(html).toContain("2,5 giờ làm việc");
  });

  it("by hamlet: the recorded name; `\"\"` → `Chưa xác định địa bàn`", () => {
    expect(html).toContain("Thôn Hà Lam");
    expect(html).toContain("Chưa xác định địa bàn");
    expect(html).toContain("Điểm đen là nơi vừa nhiều phản ánh vừa xử lý không kịp.");
  });

  it("an empty sample shows `—`, never 0%; no bar", () => {
    const empty = renderToStaticMarkup(
      <PetitionReportsView
        breakdown={{ ok: true, duLieu: { ...BREAKDOWN, totals: { on_time_sample: 0, on_time: 0, late: 0, overdue: 0 } } }}
        unitNames={UNITS}
      />,
    );
    expect(empty).not.toContain("0,0%");
    expect(empty).not.toContain("data-on-time-bar");
  });

  it.each([
    ["503 `working_hours_unavailable`", "Chưa đo được giờ làm việc lúc này. Vui lòng thử lại sau ít phút."],
    ["409 `working_calendar_not_configured`", "Xã chưa cấu hình lịch làm việc. Hãy cấu hình ở màn hình Cấu hình."],
  ])("%s: the server's sentence verbatim, and Tải lại", (_name, cau) => {
    const h = renderToStaticMarkup(
      <PetitionReportsView breakdown={{ ok: false, thongBao: cau }} unitNames={UNITS} onReload={() => {}} />,
    );
    expect(h).toContain(cau);
    expect(h).toContain("Tải lại");
    expect(h).not.toContain("Đúng hạn và trễ hạn");
  });
});

describe("Báo cáo — the period is /bao-cao's, [from, to) in Viet Nam", () => {
  it("opens on `Tháng này` with periodWindows' current window; `Quý này` asks the quarter", async () => {
    const now = new Date("2026-10-09T03:00:00Z");
    const load = vi.fn(async (): Promise<KetQua<petitions_citizenReportBreakdownOut>> => ({ ok: true, duLieu: BREAKDOWN }));
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<PetitionReports unitNames={UNITS} load={load} now={() => now} />));
    await act(async () => {
      await Promise.resolve();
    });
    expect(load).toHaveBeenLastCalledWith(toQueryPeriod(periodWindows("month", now).current));
    const quarter = [...host.querySelectorAll("button")].find((b) => b.textContent === "Quý này")!;
    act(() => quarter.click());
    await act(async () => {
      await Promise.resolve();
    });
    expect(load).toHaveBeenLastCalledWith(toQueryPeriod(periodWindows("quarter", now).current));
    expect(quarter.getAttribute("aria-pressed")).toBe("true");
    expect(host.textContent).toContain("Kỳ quý này");
  });

  it("the path carries exactly from/to, encoded", () => {
    expect(citizenReportBreakdownPath({ from: "2026-10-01T00:00:00+07:00", to: "2026-10-09T10:00:01+07:00" })).toBe(
      "/api/v1/citizen-report-breakdown?from=2026-10-01T00%3A00%3A00%2B07%3A00&to=2026-10-09T10%3A00%3A01%2B07%3A00",
    );
  });
});
