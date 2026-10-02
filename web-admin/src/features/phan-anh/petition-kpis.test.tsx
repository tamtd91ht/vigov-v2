import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { parseDrillDown } from "@/lib/drill-down";
import type { petitions_citizenReportSummaryOut } from "@/lib/api/schema.gen";

import { KPI_NO_RATING, KPI_PENDING_CAPTION } from "./nhan-phieu";
import { kpiHref, kpiPeriod, PetitionKpisView } from "./petition-kpis";

/**
 * §3 — the four KPI cards. What matters: the AVERAGE is the server's sum / sample with one decimal and
 * never "0" for an empty sample, and every figure opens EXACTLY the list the server counted it from —
 * read back through the receiving side's parser, so a link the list would call invalid is red here.
 */

const PERIOD = { from: "2026-07-04T03:00:00.000Z", to: "2026-10-02T03:00:00.000Z" };

function summary(change: Partial<petitions_citizenReportSummaryOut> = {}): petitions_citizenReportSummaryOut {
  return {
    received: 21,
    in_progress: 20,
    on_time_sample: 3,
    on_time: 1,
    late: 2,
    rating_sample: 3,
    rating_sum: 13,
    low_rating: 1,
    publication_pending: 16,
    ...change,
  };
}

function view(s: petitions_citizenReportSummaryOut) {
  return renderToStaticMarkup(<PetitionKpisView summary={{ ok: true, duLieu: s }} period={PERIOD} />);
}

/** `&` in an href is written `&amp;` by React. */
const href = (h: string) => h.replace(/&/g, "&amp;");

describe("kpiPeriod / kpiHref — the window and the list behind each figure", () => {
  it("the window is the 90 days before now, half-open", () => {
    const now = new Date("2026-10-02T03:00:00Z");
    expect(kpiPeriod(now)).toEqual(PERIOD);
  });

  it("period metrics carry the window; stock metrics carry NONE — and the receiver accepts every link", () => {
    for (const m of ["received", "on_time", "late"] as const) {
      const q = new URLSearchParams(kpiHref(m, PERIOD).split("?")[1]);
      expect(q.get("from"), m).toBe(PERIOD.from);
      expect(q.get("to"), m).toBe(PERIOD.to);
    }
    for (const m of ["in_progress", "rating_sample", "low_rating", "publication_pending"] as const) {
      const q = new URLSearchParams(kpiHref(m, PERIOD).split("?")[1]);
      expect(q.has("from"), m).toBe(false);
      expect(q.has("to"), m).toBe(false);
    }
    for (const m of ["received", "in_progress", "on_time", "late", "rating_sample", "low_rating", "publication_pending"] as const) {
      const h = kpiHref(m, PERIOD);
      expect(h.startsWith("/phan-anh?metric="), m).toBe(true);
      const parsed = parseDrillDown("citizen-reports", Object.fromEntries(new URLSearchParams(h.split("?")[1])));
      expect(parsed.kind, m).toBe("active");
      expect(h, m).not.toMatch(/tenant/i);
    }
  });
});

describe("PetitionKpisView — the four cards", () => {
  it("figures and captions; each figure is a link to its list", () => {
    const html = view(summary());
    expect(html).toContain("Tổng phản ánh 90 ngày");
    expect(html).toContain(">21</a>");
    expect(html).toContain("20 phiếu đang xử lý");
    expect(html).toContain("33,3% đúng hạn");
    expect(html).toContain("4,3/5");
    expect(html).toContain("1 phiếu bị đánh giá thấp");
    expect(html).toContain(">16</a>");
    expect(html).toContain(KPI_PENDING_CAPTION);
    for (const m of ["received", "in_progress", "on_time", "late", "rating_sample", "low_rating", "publication_pending"] as const) {
      expect(html, m).toContain(`href="${href(kpiHref(m, PERIOD))}"`);
    }
  });

  it("the average is computed sum / sample with one decimal (8 / 2 → 4,0/5)", () => {
    expect(view(summary({ rating_sum: 8, rating_sample: 2 }))).toContain("4,0/5");
  });

  it("sample 0: the average is —, never 0, with the sentence; and no rating_sample link", () => {
    const html = view(summary({ rating_sum: 0, rating_sample: 0, low_rating: 0 }));
    expect(html).not.toContain("0,0/5");
    expect(html).toContain(KPI_NO_RATING);
    expect(html).not.toContain(`href="${href(kpiHref("rating_sample", PERIOD))}"`);
    // The low-rating list still opens (0 of them is a true count).
    expect(html).toContain("0 phiếu bị đánh giá thấp");
  });

  it("a server before 02/10/2026 (fields absent): “Chưa có dữ liệu”, never 0", () => {
    const html = view(
      summary({ rating_sample: undefined, rating_sum: undefined, low_rating: undefined, publication_pending: null }),
    );
    expect(html.match(/Chưa có dữ liệu/g)?.length).toBe(2);
    expect(html).not.toContain("phiếu bị đánh giá thấp");
  });

  it("late > 0 is red WITH the icon beside it — never colour alone", () => {
    const html = view(summary({ late: 2 }));
    expect(html).toMatch(/text-danger-600[^>]*>.*lucide-alarm-clock/);
    expect(view(summary({ late: 0 }))).not.toContain("lucide-alarm-clock");
  });

  it("on-time sample 0: no percentage, never 0%", () => {
    const html = view(summary({ on_time: 0, on_time_sample: 0 }));
    expect(html).not.toContain("0,0% đúng hạn");
    expect(html).toContain("Chưa có phiếu nào có hạn trong 90 ngày qua");
  });

  it("the overdue NUMBER is not shown anywhere (ADR 0007 decision 10a)", () => {
    expect(view(summary())).not.toMatch(/quá hạn \d|trễ \d+ (ngày|giờ)/i);
  });

  it("loading: a status sentence and skeletons, no figure", () => {
    const html = renderToStaticMarkup(<PetitionKpisView summary={null} period={PERIOD} />);
    expect(html).toContain("Đang tải số liệu phản ánh…");
    expect(html).toContain('aria-busy="true"');
    expect(html).not.toContain("<a ");
  });

  it("refused: the server's sentence verbatim, with Tải lại", () => {
    const cau = "Tài khoản của bạn không có quyền xem báo cáo.";
    const html = renderToStaticMarkup(
      <PetitionKpisView summary={{ ok: false, thongBao: cau }} period={PERIOD} onReload={() => {}} />,
    );
    expect(html).toContain(cau);
    expect(html).toContain('role="alert"');
    expect(html).toContain("Tải lại");
  });
});
