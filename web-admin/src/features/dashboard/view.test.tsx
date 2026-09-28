import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NHAN_CHENH_LECH } from "@/features/thu-chi/nhan-thu-chi";
import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type {
  documents_incomingSummaryOut,
  finance_chiSoNamRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";

import { NO_SOURCE_DATA, NOTHING_URGENT } from "./figures";
import { MISSING_REPORT_READ } from "./overview";
import { periodWindows } from "./period";
import {
  blockVisibility,
  CitizenReportBlock,
  DashboardView,
  FiscalBlock,
  IncomingDocumentBlock,
  TaskBlock,
  UrgentPanel,
} from "./view";
import type { DashboardData, SummaryPair } from "./view";

/**
 * The branches a developer holding every key never sees — denied, failed, empty sample, 503 queue —
 * rendered to a string and read back. `&amp;`-free labels are used where possible so the assertions
 * read like the screen.
 */

const WINDOWS = periodWindows("month", new Date("2026-09-28T09:43:00Z"));

const TASKS_ZERO: petitions_taskSummaryOut = {
  in_progress: 0,
  overdue: 0,
  suspended: 0,
  completed: 0,
  on_time_sample: 0,
  on_time: 0,
};

const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });
const failed = (thongBao: string) => ({ ok: false as const, thongBao });

function valuesOf(html: string): string[] {
  return [...html.matchAll(/id="[^"]*-value"[^>]*>([^<]*)</g)].map((m) => m[1] ?? "");
}

describe("TaskBlock", () => {
  it("count 0 shows 0; an empty on-time sample shows '—', never 0%", () => {
    const html = renderToStaticMarkup(
      <TaskBlock pair={{ current: ok(TASKS_ZERO), previous: ok(TASKS_ZERO) }} windows={WINDOWS} />,
    );
    expect(valuesOf(html)).toEqual(["0", "0", "0", "0", "—"]);
    expect(html).not.toContain("0,0%");
    expect(html).toContain("0/0 việc có hạn");
  });

  it("every figure is a link named 'Xem danh sách đằng sau: {nhãn}' to its list", () => {
    const html = renderToStaticMarkup(
      <TaskBlock pair={{ current: ok(TASKS_ZERO), previous: ok(TASKS_ZERO) }} windows={WINDOWS} />,
    );
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Quá hạn"');
    expect(html).toContain('href="/nhiem-vu?metric=overdue"');
    // the % cell leads to the numerator, with the period
    expect(html).toMatch(/href="\/nhiem-vu\?metric=on_time&amp;from=2026-09-01T00%3A00%3A00%2B07%3A00/);
  });

  it("stock figures carry no comparison line; period figures do", () => {
    const html = renderToStaticMarkup(
      <TaskBlock
        pair={{
          current: ok({ ...TASKS_ZERO, in_progress: 24, completed: 9 }),
          previous: ok({ ...TASKS_ZERO, in_progress: 10, completed: 8 }),
        }}
        windows={WINDOWS}
      />,
    );
    expect(html).toContain("↑ +12,5% so với kỳ trước");
    // 24 vs 10 would be +140,0% — must not be drawn for a stock figure
    expect(html).not.toContain("140,0%");
    expect(html).not.toContain("chưa có kỳ trước");
  });

  it("a FAILED call shows '—' and the server's sentence — never 0", () => {
    const html = renderToStaticMarkup(
      <TaskBlock
        pair={{ current: failed("Đã xảy ra lỗi. Vui lòng thử lại."), previous: ok(TASKS_ZERO) }}
        windows={WINDOWS}
      />,
    );
    expect(valuesOf(html)).toEqual(["—", "—", "—", "—", "—"]);
    expect(html).toContain("Đã xảy ra lỗi. Vui lòng thử lại.");
    expect(html).toContain('role="alert"');
  });

  it("a failed PREVIOUS call keeps the current figures and says the previous is missing", () => {
    const html = renderToStaticMarkup(
      <TaskBlock
        pair={{ current: ok({ ...TASKS_ZERO, completed: 3 }), previous: failed("Mạng hỏng.") }}
        windows={WINDOWS}
      />,
    );
    expect(valuesOf(html)[3]).toBe("3");
    expect(html).toContain("Kỳ trước: —");
    expect(html).toContain("Không tải được số liệu kỳ trước: Mạng hỏng.");
  });

  it("loading shows neither 0 nor '—'", () => {
    const html = renderToStaticMarkup(
      <TaskBlock pair={{ current: null, previous: null }} windows={WINDOWS} />,
    );
    expect(valuesOf(html)).toEqual(["…", "…", "…", "…", "…"]);
  });
});

describe("IncomingDocumentBlock", () => {
  const doc: documents_incomingSummaryOut = {
    from: "2026-09-01T00:00:00+07:00",
    to: "2026-09-28T16:43:01+07:00",
    as_of: "2026-09-28T09:43:00Z",
    arrived: 5,
    open: 6,
    overdue: 4,
  };
  const html = renderToStaticMarkup(
    <IncomingDocumentBlock pair={{ current: ok(doc), previous: ok(doc) }} windows={WINDOWS} />,
  );

  it("`open` and `overdue` link WITHOUT from/to; `arrived` with them", () => {
    expect(html).toContain('href="/van-ban?metric=open"');
    expect(html).toContain('href="/van-ban?metric=overdue"');
    expect(html).toMatch(/href="\/van-ban\?metric=arrived&amp;from=/);
  });

  it("stock figures say when 'now' was (as_of), in Viet Nam", () => {
    expect(html).toContain("tính đến 16:43 28/09/2026");
  });

  it("cells with no source say so — never 0", () => {
    expect(html).toContain("Đơn thư trong kỳ");
    expect(html).toContain("Tỷ lệ đúng hạn văn bản");
    expect(html.split(NO_SOURCE_DATA)).toHaveLength(3);
  });
});

describe("CitizenReportBlock", () => {
  const r: petitions_citizenReportSummaryOut = {
    received: 2,
    in_progress: 14,
    on_time_sample: 3,
    on_time: 1,
    late: 2,
  };
  it("labels, ratio with sample, and the satisfaction cell without data", () => {
    const html = renderToStaticMarkup(
      <CitizenReportBlock pair={{ current: ok(r), previous: ok(r) }} windows={WINDOWS} />,
    );
    expect(html).toContain("Nhận vào trong kỳ");
    expect(html).toContain("33,3%");
    expect(html).toContain("1/3 phiếu có hạn");
    expect(html).toContain("Trễ hạn trong kỳ");
    expect(html).toContain("Điểm hài lòng");
    expect(html).toContain('href="/phan-anh?metric=in_progress"');
  });
});

describe("FiscalBlock — ADR 0035 §A: the reason sentence instead of a figure", () => {
  const indicators: finance_chiSoNamRa = {
    year: 2026,
    revenue_achievement: {
      name: "Thu đạt dự toán",
      basis_points: null,
      unavailable_reason: "Bảng thu năm 2026 chưa đánh dấu dòng tổng.",
    },
    expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
    balance: { amount: null, unavailable_reason: "Chưa có bảng chi năm 2026." },
    revenue_totals: [],
  };

  it("shows the server's sentence, the temporary balance label, and links to Thu - Chi", () => {
    const html = renderToStaticMarkup(<FiscalBlock result={ok(indicators)} year={2026} />);
    expect(html).toContain("Bảng thu năm 2026 chưa đánh dấu dòng tổng.");
    expect(html).toContain("Chưa có bảng chi năm 2026.");
    expect(html).toContain("91,30%");
    expect(html).toContain(NHAN_CHENH_LECH);
    expect(html).toContain('href="/giai-ngan/thu-chi"');
    expect(html).toContain("Luỹ kế năm 2026, không so với kỳ trước.");
    expect(html).not.toContain("so với kỳ trước<");
  });

  it("a failed call shows '—' and the error line", () => {
    const html = renderToStaticMarkup(<FiscalBlock result={failed("Mạng hỏng.")} year={2026} />);
    expect(html).toContain("Mạng hỏng.");
    expect(valuesOf(html)).toEqual(["—", "—", "—"]);
  });
});

describe("UrgentPanel", () => {
  it("a 503 queue is an explicit error line, and NOT 'nothing urgent'", () => {
    const html = renderToStaticMarkup(
      <UrgentPanel
        queue={ok({
          rows: [],
          failures: [
            {
              module: "incoming-document",
              message: "Chưa tính được lịch làm việc của xã.",
            },
          ],
        })}
        taskTypeLabels={null}
      />,
    );
    expect(html).toContain("Văn bản đến: Chưa tính được lịch làm việc của xã.");
    expect(html).not.toContain(NOTHING_URGENT);
  });

  it("every queue answered with no row → the spec's empty sentence", () => {
    const html = renderToStaticMarkup(
      <UrgentPanel queue={ok({ rows: [], failures: [] })} taskTypeLabels={null} />,
    );
    expect(html).toContain(NOTHING_URGENT);
  });

  it("a row shows module, code, kind, category, the deadline and the critical badge — nothing else", () => {
    const html = renderToStaticMarkup(
      <UrgentPanel
        queue={ok({
          rows: [
            {
              module: "citizen-report",
              code: "PA-7F3K-9QXR-MNPT",
              kind: "han-xu-ly-xong",
              categoryCode: "rac-thai",
              missedAt: "2026-09-01T01:00:00Z",
              critical: true,
            },
          ],
          failures: [],
        })}
        taskTypeLabels={null}
      />,
    );
    expect(html).toContain("Phản ánh");
    expect(html).toContain("PA-7F3K-9QXR-MNPT");
    expect(html).toContain("Hạn xử lý xong");
    expect(html).toContain("Rác thải – Vệ sinh môi trường");
    expect(html).toContain("quá hạn từ 08:00 01/09/2026");
    expect(html).toContain("Nghiêm trọng");
  });
});

function emptyData(): DashboardData {
  const none: SummaryPair<never> = { current: null, previous: null };
  return {
    windows: WINDOWS,
    fetchedAt: Date.parse("2026-09-28T09:43:00Z"),
    tasks: none,
    incomingDocuments: none,
    citizenReports: none,
    fiscal: null,
    fiscalYear: 2026,
    queue: ok({ rows: [], failures: [] }),
    taskTypeLabels: null,
  };
}

describe("DashboardView — access per block", () => {
  it("report.read alone: NO module block is rendered (not 0, not '—')", () => {
    const visible = blockVisibility(["report.read"]);
    expect(visible).toEqual({
      tasks: false,
      incomingDocuments: false,
      citizenReports: false,
      budget: false,
    });
    const html = renderToStaticMarkup(
      <DashboardView data={emptyData()} visible={visible} onPeriodChange={() => {}} />,
    );
    expect(html).not.toContain('aria-label="Nhiệm vụ"');
    expect(html).not.toContain('aria-label="Văn bản đến"');
    expect(html).not.toContain('aria-label="Phản ánh người dân"');
    expect(html).not.toContain("Thu – Chi ngân sách");
    expect(html).not.toContain("Cần xử lý ngay");
    // the block with no source and no key of its own stays, muted
    expect(html).toContain("Kinh tế &amp; Tài nguyên");
  });

  it("task.read without report.read opens nothing — both keys are needed", () => {
    expect(blockVisibility(["task.read", "document.read"]).tasks).toBe(false);
  });

  it("all keys: every block, the meta line and the period picker", () => {
    const visible = blockVisibility([
      "report.read",
      "task.read",
      "document.read",
      "feedback.read",
      "budget.read",
    ]);
    const html = renderToStaticMarkup(
      <DashboardView data={emptyData()} visible={visible} onPeriodChange={() => {}} />,
    );
    expect(html).toContain('aria-label="Nhiệm vụ"');
    expect(html).toContain('aria-label="Văn bản đến"');
    expect(html).toContain('aria-label="Phản ánh người dân"');
    expect(html).toContain("Giải ngân ngân sách");
    expect(html).toContain("Kỳ tháng này: 1/9/2026 – 30/9/2026 · tính đến 16:43 28/09/2026");
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain("Tháng này");
    expect(html).toContain(NOTHING_URGENT);
    // no snapshot machinery (user decision)
    expect(html).not.toContain("Tính lại ngay");
  });
});

describe("every link RENDERED on the page is valid at the receiving list", () => {
  const REGISTER = {
    "/nhiem-vu": "tasks",
    "/phan-anh": "citizen-reports",
    "/van-ban": "incoming-documents",
  } as const;

  it("parses as an active drill-down, and the count cells' labels are the lists' headings", () => {
    const task: petitions_taskSummaryOut = { ...TASKS_ZERO, on_time_sample: 2, on_time: 1 };
    const doc: documents_incomingSummaryOut = {
      from: "",
      to: "",
      as_of: "2026-09-28T09:43:00Z",
      arrived: 1,
      open: 1,
      overdue: 1,
    };
    const rep: petitions_citizenReportSummaryOut = {
      received: 1,
      in_progress: 1,
      on_time_sample: 1,
      on_time: 1,
      late: 0,
    };
    const data: DashboardData = {
      ...emptyData(),
      tasks: { current: ok(task), previous: ok(task) },
      incomingDocuments: { current: ok(doc), previous: ok(doc) },
      citizenReports: { current: ok(rep), previous: ok(rep) },
    };
    const visible = blockVisibility(["report.read", "task.read", "document.read", "feedback.read"]);
    const html = renderToStaticMarkup(
      <DashboardView data={data} visible={visible} onPeriodChange={() => {}} />,
    );

    const hrefs = [...html.matchAll(/href="([^"]*)"/g)].map((m) =>
      (m[1] ?? "").replaceAll("&amp;", "&"),
    );
    const lists = hrefs.filter((h) => (h.split("?")[0] ?? "") in REGISTER);
    expect(lists).toHaveLength(12); // 5 tasks + 3 documents + 4 citizen reports
    for (const href of lists) {
      const [path, query = ""] = href.split("?") as [keyof typeof REGISTER, string];
      const parsed = parseDrillDown(REGISTER[path], Object.fromEntries(new URLSearchParams(query)));
      expect(parsed.kind, href).toBe("active");
    }
    // count cells take their name from the receiver's table
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Chưa xử lý xong"');
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Nhận vào trong kỳ"');
  });
});

describe("page gate — report.read", () => {
  it("an account without report.read reads the sentence and no figure", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen quyetDinh={{ hien: false, vi: "khong-du-quyen" }} cauThieuQuyen={MISSING_REPORT_READ}>
        <p>SỐ LIỆU</p>
      </KhungQuyen>,
    );
    expect(html).toContain("report.read");
    expect(html).not.toContain("SỐ LIỆU");
  });
});
