import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_citizenReportSummaryOut } from "@/lib/api/schema.gen";

import { KPI_NO_RATING, KPI_PENDING_CAPTION } from "./nhan-phieu";
import { kpiPeriod, PetitionKpisView } from "./petition-kpis";

/**
 * §3 — the four KPI cards. What matters: the AVERAGE is the server's sum / sample with one decimal and
 * never "0" for an empty sample, and the figures are PLAIN (customer bug sheet row 56: they no longer
 * filter the list, as in the prototype).
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
  return renderToStaticMarkup(<PetitionKpisView summary={{ ok: true, duLieu: s }} />);
}

/** The visible text of one card's value line (screen-reader-only prefixes stripped). */
function cardValue(html: string, label: string): string {
  const card = html.split(`data-kpi="${label}"`)[1] ?? "";
  const value = card.match(/data-kpi-value=""[^>]*>([\s\S]*?)<\/p>/)?.[1] ?? "";
  return value
    .replace(/<span class="an-thi-giac">[\s\S]*?<\/span>/g, "")
    .replace(/<[^>]+>/g, "")
    .trim();
}

describe("kpiPeriod — the window counted", () => {
  it("the window is the 90 days before now, half-open", () => {
    const now = new Date("2026-10-02T03:00:00Z");
    expect(kpiPeriod(now)).toEqual(PERIOD);
  });
});

describe("PetitionKpisView — the four cards", () => {
  it("figures and captions, PLAIN — no figure is a link (bug sheet row 56)", () => {
    const html = view(summary());
    expect(html).toContain("Tổng phản ánh 90 ngày");
    expect(cardValue(html, "Tổng phản ánh 90 ngày")).toBe("21");
    expect(html).toContain("20 phiếu đang xử lý");
    expect(html).toContain("33,3% đúng hạn");
    expect(html).toContain("4,3/5");
    expect(html).toContain("1 phiếu bị đánh giá thấp");
    expect(cardValue(html, "Chờ kiểm duyệt")).toBe("16");
    expect(cardValue(html, "Đúng hạn / trễ hạn")).toBe("1 / 2");
    expect(html).toContain(KPI_PENDING_CAPTION);
    expect(html).not.toContain("<a ");
    expect(html).not.toContain("href=");
  });

  it("the average is computed sum / sample with one decimal (8 / 2 → 4,0/5)", () => {
    expect(view(summary({ rating_sum: 8, rating_sample: 2 }))).toContain("4,0/5");
  });

  it("sample 0: the average is —, never 0; the prototype's one hint line only", () => {
    const html = view(summary({ rating_sum: 0, rating_sample: 0, low_rating: 0 }));
    expect(html).not.toContain("0,0/5");
    expect(cardValue(html, "Điểm hài lòng trung bình")).toBe("—");
    // Prototype `FeedbackWorkspace.tsx:108-116`: one hint, no second sentence under it.
    expect(html).not.toContain(KPI_NO_RATING);
    // 0 low ratings is a true count, shown.
    expect(html).toContain("0 phiếu bị đánh giá thấp");
  });

  it("a server before 02/10/2026 (fields absent): —, never 0", () => {
    const html = view(
      summary({ rating_sample: undefined, rating_sum: undefined, low_rating: undefined, publication_pending: null }),
    );
    expect(cardValue(html, "Điểm hài lòng trung bình")).toBe("—");
    expect(cardValue(html, "Chờ kiểm duyệt")).toBe("—");
    expect(html).not.toContain("phiếu bị đánh giá thấp");
    expect(cardValue(html, "Chờ kiểm duyệt")).not.toBe("0");
  });

  it("late > 0: the WHOLE value is danger, with the warning icon before the hint — never colour alone", () => {
    const html = view(summary({ late: 2 }));
    expect(html).toMatch(/data-kpi-value="" class="[^"]*text-danger[^"]*"|class="[^"]*text-danger[^"]*" data-kpi-value=""/);
    expect(html).toContain("lucide-triangle-alert");
    const calm = view(summary({ late: 0 }));
    expect(calm).toMatch(/class="[^"]*text-leaf[^"]*"[^>]*data-kpi-value=""|data-kpi-value=""[^>]*class="[^"]*text-leaf/);
    expect(calm).not.toContain("lucide-triangle-alert");
  });

  it("prototype card: no icon tile, uppercase 11px label, 18px bold value, 11px hint", () => {
    const html = view(summary());
    expect(html).not.toMatch(/lucide-(inbox|target|star|eye)/);
    expect(html.match(/text-\[11px\] font-semibold tracking-wide uppercase/g)?.length).toBe(4);
    expect(html.match(/text-\[18px\] font-bold/g)?.length).toBe(4);
  });

  it("on-time sample 0: no percentage, never 0%", () => {
    const html = view(summary({ on_time: 0, on_time_sample: 0 }));
    expect(html).not.toContain("0,0% đúng hạn");
    expect(html).toContain("Chưa có phiếu nào có hạn trong 90 ngày qua");
  });

  it("the overdue NUMBER is not shown anywhere (ADR 0007 decision 10a)", () => {
    expect(view(summary())).not.toMatch(/quá hạn \d|trễ \d+ (ngày|giờ)/i);
  });

  it("loading: a status sentence and ONE h-24 skeleton (prototype `FeedbackWorkspace.tsx:92`), no figure", () => {
    const html = renderToStaticMarkup(<PetitionKpisView summary={null} />);
    expect(html).toContain("Đang tải số liệu phản ánh…");
    expect(html).toContain('aria-busy="true"');
    expect(html).not.toContain("<a ");
    expect(html.match(/aria-hidden="true" class="[^"]*h-24/g)?.length).toBe(1);
  });

  it("refused: the server's sentence verbatim, with Tải lại", () => {
    const cau = "Tài khoản của bạn không có quyền xem báo cáo.";
    const html = renderToStaticMarkup(
      <PetitionKpisView summary={{ ok: false, thongBao: cau }} onReload={() => {}} />,
    );
    expect(html).toContain(cau);
    expect(html).toContain('role="alert"');
    expect(html).toContain("Tải lại");
  });
});
