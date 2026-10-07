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

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { NO_DATA_CAPTION } from "@/components/ui/stat-card";

import { NO_SOURCE_DATA, NOTHING_URGENT } from "./figures";
import type { UrgentRow } from "./figures";
import { MISSING_REPORT_READ } from "./overview";
import { periodWindows } from "./period";
import {
  blockVisibility,
  CitizenReportBlock,
  DashboardView,
  DashboardBlocks,
  DashboardHeader,
  DocumentBlock,
  FiscalBlock,
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

/** The value inside the element `aria-describedby` points at — `undefined` if not rendered. */
function valueOf(html: string, id: string): string | undefined {
  return new RegExp(`id="${id}-value"[^>]*>([^<]*)<`).exec(html)?.[1];
}

const TASK_KEYS = blockVisibility(["report.read", "task.read"]);
const DOCUMENT_KEYS = blockVisibility(["report.read", "document.read"]);

/** The page with the given loads. */
function page(patch: Partial<DashboardData>, visible = TASK_KEYS): string {
  return renderToStaticMarkup(
    <DashboardView data={{ ...emptyData(), ...patch }} visible={visible} onPeriodChange={() => {}} />,
  );
}

describe("task figures (khối Nhiệm vụ)", () => {
  it("count 0 shows 0; an empty on-time sample shows '—', never 0%", () => {
    const html = page({ tasks: { current: ok(TASKS_ZERO), previous: ok(TASKS_ZERO) } });
    // đang thực hiện · quá hạn · hoàn thành · tạm dừng · đúng hạn — every figure, each once
    expect(valuesOf(html)).toEqual(["0", "0", "0", "0", "—"]);
    expect(html).not.toContain("0,0%");
    expect(html).toContain("0/0 việc có hạn");
  });

  it("an overdue count above 0 is red AND worded — never colour alone", () => {
    const html = page({
      tasks: { current: ok({ ...TASKS_ZERO, overdue: 2 }), previous: ok(TASKS_ZERO) },
    });
    expect(valueOf(html, "tasks-overdue")).toBe("2");
    const tile = /<li[^>]*>(?:(?!<\/li>).)*metric=overdue(?:(?!<\/li>).)*<\/li>/s.exec(html)?.[0] ?? "";
    expect(tile).toContain("text-danger-600");
    expect(tile).toContain(">Quá hạn<");
  });

  it("every figure is a link named 'Xem danh sách đằng sau: {nhãn}' to its list", () => {
    const html = page({
      tasks: { current: ok({ ...TASKS_ZERO, overdue: 1 }), previous: ok(TASKS_ZERO) },
    });
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Quá hạn"');
    expect(html).toContain('href="/nhiem-vu?metric=overdue"');
    // the % cell leads to the numerator, with the period
    expect(html).toMatch(/href="\/nhiem-vu\?metric=on_time&amp;from=2026-09-01T00%3A00%3A00%2B07%3A00/);
  });

  it("stock figures carry no comparison line; period figures do", () => {
    const html = page({
      tasks: {
        current: ok({ ...TASKS_ZERO, in_progress: 24, completed: 9 }),
        previous: ok({ ...TASKS_ZERO, in_progress: 10, completed: 8 }),
      },
    });
    expect(html).toContain("+12,5% so với kỳ trước");
    // 24 vs 10 would be +140,0% — must not be drawn for a stock figure
    expect(html).not.toContain("140,0%");
    expect(html).not.toContain("chưa có kỳ trước");
  });

  it("a FAILED call shows '—' and the server's sentence — never 0, never 'nothing pending'", () => {
    const html = page({
      tasks: { current: failed("Đã xảy ra lỗi. Vui lòng thử lại."), previous: ok(TASKS_ZERO) },
    });
    expect(valuesOf(html)).toEqual(["—", "—", "—", "—", "—"]);
    expect(html).toContain("Đã xảy ra lỗi. Vui lòng thử lại.");
    expect(html).toContain('role="alert"');
    expect(html).toContain("Tải lại");
  });

  it("a failed PREVIOUS call keeps the current figures and says the previous is missing", () => {
    const html = page({
      tasks: { current: ok({ ...TASKS_ZERO, completed: 3 }), previous: failed("Mạng hỏng.") },
    });
    expect(valueOf(html, "tasks-completed")).toBe("3");
    expect(html).toContain("Kỳ trước: —");
    expect(html).toContain("Không tải được số liệu kỳ trước: Mạng hỏng.");
  });

  it("loading shows neither 0 nor '—', and is not 'nothing pending'", () => {
    const html = page({ tasks: { current: null, previous: null } });
    expect(valuesOf(html)).toEqual(["…", "…", "…", "…", "…"]);
    expect(html).not.toContain("Tải lại");
  });
});

describe("incoming document figures", () => {
  const doc: documents_incomingSummaryOut = {
    from: "2026-09-01T00:00:00+07:00",
    to: "2026-09-28T16:43:01+07:00",
    as_of: "2026-09-28T09:43:00Z",
    arrived: 5,
    open: 6,
    overdue: 4,
  };
  const html = page({ incomingDocuments: { current: ok(doc), previous: ok(doc) } }, DOCUMENT_KEYS);

  it("`open` and `overdue` link WITHOUT from/to; `arrived` with them", () => {
    expect(html).toContain('href="/van-ban?metric=open"');
    expect(html).toContain('href="/van-ban?metric=overdue"');
    expect(html).toMatch(/href="\/van-ban\?metric=arrived&amp;from=/);
  });

  it("stock figures say when 'now' was (as_of), in Viet Nam", () => {
    expect(html).toContain("tính đến 16:43 28/09/2026");
  });

  it("an account without task.read sees no task figure", () => {
    expect(html).not.toContain('href="/nhiem-vu');
  });

  it("the no-source tile says so, the unbuilt figure carries the '?' — never 0, never a link", () => {
    const block = renderToStaticMarkup(
      <DocumentBlock pair={{ current: ok(doc), previous: ok(doc) }} windows={WINDOWS} />,
    );
    expect(block.split(NO_DATA_CAPTION)).toHaveLength(2);
    const noSource = new RegExp(`<li[^>]*title="${NO_SOURCE_DATA}"[^>]*>.*?</li>`, "s").exec(block)?.[0] ?? "";
    expect(noSource).toContain("Tỷ lệ đúng hạn văn bản");
    expect(noSource).not.toContain("href=");
    const pending = /<li[^>]*data-pending=""[^>]*>.*?<\/li>/s.exec(block)?.[0] ?? "";
    expect(pending).toContain("Đơn thư trong kỳ");
    expect(pending).toContain(`aria-label="${pendingMarkerLabel("Đơn thư trong kỳ")}"`);
    expect(pending).toContain("—");
    expect(pending).not.toContain("href=");
    // the three document counts are links, each once
    expect(block.match(/href="\/van-ban\?/g)).toHaveLength(3);
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
  const block = (current: petitions_citizenReportSummaryOut) =>
    renderToStaticMarkup(<CitizenReportBlock pair={{ current: ok(current), previous: ok(current) }} windows={WINDOWS} />);

  it("labels, ratio with sample", () => {
    const html = block(r);
    expect(html).toContain("Nhận vào trong kỳ");
    expect(html).toContain("33,3%");
    // TQ-05: the note names the server's real denominator (settled + classification ceiling missed).
    expect(html).toContain("1/3 phiếu đúng hạn, trên số phiếu xử lý xong hoặc quá hạn phân loại trong kỳ");
    expect(html).not.toContain("phiếu có hạn");
    expect(html).toContain("Trễ hạn trong kỳ");
    expect(html).toContain('href="/phan-anh?metric=in_progress"');
  });

  it("Điểm hài lòng: the real average with its sample, linked to the rated petitions, no period", () => {
    const html = block({ ...r, rating_sum: 13, rating_sample: 3 });
    expect(valueOf(html, "citizen-reports-rating")).toBe("4,3/5");
    expect(html).toContain("3 phiếu được chấm");
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Điểm hài lòng"');
    expect(html).toContain('href="/phan-anh?metric=rating_sample"');
    // the same division as the Phản ánh screen; no "?" left for it
    expect(html).not.toContain(pendingMarkerLabel("Điểm hài lòng"));
  });

  it("Điểm hài lòng with an EMPTY sample: '—' and the no-rating sentence — never 0 or 0,0/5", () => {
    const html = block({ ...r, rating_sum: 0, rating_sample: 0 });
    expect(valueOf(html, "citizen-reports-rating")).toBe("—");
    expect(html).toContain("Chưa có phiếu nào được người dân chấm điểm");
    expect(html).not.toContain("0,0/5");
  });

  it("Điểm hài lòng from a server without the two fields: '—', no sentence, no fake average", () => {
    const html = block(r);
    expect(valueOf(html, "citizen-reports-rating")).toBe("—");
    expect(html).not.toContain("phiếu được chấm");
  });

  it("DENIED: without feedback.read the block — and its rating — is not drawn", () => {
    const rated = { ...r, rating_sum: 13, rating_sample: 3 };
    const html = page(
      { citizenReports: { current: ok(rated), previous: ok(rated) } },
      blockVisibility(["report.read", "task.read"]),
    );
    expect(html).not.toContain("Điểm hài lòng");
    expect(html).not.toContain("4,3/5");
    expect(html).not.toContain('href="/phan-anh?metric=rating_sample"');
  });

  it("ALLOWED: with report.read and feedback.read the page shows the rating", () => {
    const rated = { ...r, rating_sum: 13, rating_sample: 3 };
    const html = page(
      { citizenReports: { current: ok(rated), previous: ok(rated) } },
      blockVisibility(["report.read", "feedback.read"]),
    );
    expect(valueOf(html, "citizen-reports-rating")).toBe("4,3/5");
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
    // no figure of any module: no link to its list, no block that groups them
    expect(html).not.toContain('href="/nhiem-vu');
    expect(html).not.toContain('href="/van-ban');
    expect(html).not.toContain('href="/phan-anh');
    expect(html).not.toContain('href="/giai-ngan');
    expect(html).not.toContain('aria-label="Nhiệm vụ"');
    expect(html).not.toContain('aria-label="Văn bản &amp; Đơn thư"');
    expect(html).not.toContain('aria-label="Phản ánh người dân"');
    expect(html).not.toContain("Thu – Chi ngân sách");
    expect(html).not.toContain("Giải ngân ngân sách");
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
    expect(html).toContain('aria-label="Văn bản &amp; Đơn thư"');
    expect(html).toContain('aria-label="Phản ánh người dân"');
    expect(html).toContain("Giải ngân ngân sách");
    expect(html).toContain("Thu – Chi ngân sách");
    // the prototype's subline: the period, then the instant the figures were read, on one line
    expect(html).toContain("Kỳ tháng này: 1/9/2026 – 30/9/2026 · tính đến 16:43 28/09/2026");
    // what "kỳ trước" means — visible text, not a hover-only tooltip
    expect(html).toContain(
      "So với cùng khoảng thời gian đã trôi qua của kỳ trước: 00:00 01/08/2026",
    );
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
    expect(lists).toHaveLength(13); // 5 tasks + 3 documents + 5 citizen reports (rating included)
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

describe("layout — the prototype frame (owner, 05/10/2026)", () => {
  const ALL = blockVisibility(["report.read", "task.read", "document.read", "feedback.read", "budget.read"]);
  const blocksOf = (html: string) => [...html.matchAll(/data-block="([^"]+)"/g)].map((m) => m[1]);
  const all = () =>
    renderToStaticMarkup(<DashboardView data={emptyData()} visible={ALL} onPeriodChange={() => {}} />);

  it("six blocks in the prototype's order, then 'Cần xử lý ngay' as the 7th cell", () => {
    expect(blocksOf(all())).toEqual([
      "tasks",
      "documents",
      "budget",
      "fiscal",
      "citizen-reports",
      "economy",
      "urgent",
    ]);
    const html = all();
    const titles = [
      "Nhiệm vụ",
      "Văn bản &amp; Đơn thư",
      "Giải ngân ngân sách",
      "Thu – Chi ngân sách",
      "Phản ánh người dân",
      "Kinh tế &amp; Tài nguyên",
      "Cần xử lý ngay",
    ].map((t) => html.indexOf(`aria-label="${t}"`));
    expect(titles.every((i) => i >= 0)).toBe(true);
    expect([...titles].sort((a, b) => a - b)).toEqual(titles);
  });

  it("ONE grid: 1 column, 2 from lg, 3 from 2xl — every block inside it, no auto-rows-fr", () => {
    const html = all();
    const grid = /<div data-dashboard-grid="" class="([^"]*)"/.exec(html)?.[1] ?? "";
    expect(grid.split(" ")).toEqual(expect.arrayContaining(["grid", "grid-cols-1", "lg:grid-cols-2", "2xl:grid-cols-3"]));
    expect(html).not.toContain("auto-rows-fr");
    expect(html.match(/data-dashboard-grid=""/g)).toHaveLength(1);
    // every block after the grid opens — none drawn in a row of its own above or below it
    const opens = html.indexOf("data-dashboard-grid");
    expect(html.indexOf('data-block="tasks"')).toBeGreaterThan(opens);
  });

  it("a block of more than four figures may take 3 tile columns; one of three never does", () => {
    const html = all();
    const tasks = /data-block="tasks".*?<ul class="([^"]*)"/s.exec(html)?.[1] ?? "";
    expect(tasks).toContain("@md:grid-cols-3");
    const fiscal = /data-block="fiscal".*?<ul class="([^"]*)"/s.exec(html)?.[1] ?? "";
    expect(fiscal).toContain("grid-cols-2");
    expect(fiscal).not.toContain("grid-cols-3");
  });

  it("unbuilt blocks stay in place with their '?' in the header, tiles '—', no link", () => {
    const html = all();
    for (const [key, name] of [
      ["budget", "Giải ngân ngân sách"],
      ["economy", "Kinh tế & Tài nguyên"],
    ] as const) {
      const block = new RegExp(`<section[^>]*data-block="${key}".*?</section>`, "s").exec(html)?.[0] ?? "";
      expect(block).toContain(`aria-label="${pendingMarkerLabel(name).replaceAll("&", "&amp;")}"`);
      expect(block).not.toContain("href=");
      expect(block).toContain("—");
    }
  });

  it("the header: period buttons, the BUILT PDF/XLSX/PPTX and Trình chiếu (no '?') — no refresh, no stale badge", () => {
    const html = all();
    for (const label of ["Tuần này", "Tháng này", "Quý này", "Năm nay"]) expect(html).toContain(`>${label}</button>`);
    expect(html).toMatch(/aria-pressed="true"[^>]*>Tháng này</);
    expect(html).toContain(">Tổng quan điều hành</h1>");
    expect(html).toContain('aria-label="Xuất báo cáo"');
    expect(html).not.toContain(pendingMarkerLabel("Xuất báo cáo PDF, XLSX, PPTX"));
    expect(html).toContain('aria-label="Chế độ trình chiếu phòng họp"');
    expect(html).not.toContain(pendingMarkerLabel("Chế độ trình chiếu phòng họp"));
    expect(html).not.toContain("Tính lại ngay");
    expect(html).not.toContain("số liệu cũ");
    expect(html).not.toContain("cũ hơn 10 phút");
  });

  it("the header WITHOUT context (gate closed): title and disabled actions, no period, no figure", () => {
    const html = renderToStaticMarkup(<DashboardHeader />);
    expect(html).toContain(">Tổng quan điều hành</h1>");
    expect(html).not.toContain("Tháng này");
    expect(html).not.toContain("tính đến");
    expect(html).not.toContain("href=");
  });

  it("a metric opens its pre-filtered list (?metric=…), never a dialog", () => {
    const task: petitions_taskSummaryOut = { ...TASKS_ZERO, in_progress: 3 };
    const html = page({ tasks: { current: ok(task), previous: ok(task) } });
    const link = /<a[^>]*aria-describedby="tasks-in_progress-value"[^>]*>/.exec(html)?.[0] ?? "";
    expect(link).toContain('href="/nhiem-vu?metric=in_progress"');
    // a plain link: nothing on the tile opens a dialog
    expect(link).not.toContain("aria-haspopup");
    expect(link).not.toContain("<button");
  });

  it("DENIED per block: a missing key removes the block — the others keep their order", () => {
    const html = renderToStaticMarkup(
      <DashboardView
        data={emptyData()}
        visible={blockVisibility(["report.read", "document.read", "feedback.read"])}
        onPeriodChange={() => {}}
      />,
    );
    expect(blocksOf(html)).toEqual(["documents", "citizen-reports", "economy", "urgent"]);
  });

  it("/bao-cao's use of the same grid has no 'Cần xử lý ngay' cell", () => {
    const html = renderToStaticMarkup(
      <DashboardBlocks data={emptyData()} visible={ALL} onReload={() => {}} layout="report" />,
    );
    expect(blocksOf(html)).toEqual(["tasks", "documents", "budget", "fiscal", "citizen-reports", "economy"]);
    expect(html).not.toContain("Cần xử lý ngay");
  });

  it("'Cần xử lý ngay' with rows: a count, and a capped list that scrolls inside the cell", () => {
    const html = renderToStaticMarkup(
      <UrgentPanel
        queue={ok({
          rows: [
            {
              module: "task",
              code: "NV-0001",
              kind: "han-xu-ly-xong",
              categoryCode: null,
              missedAt: "2026-09-01T01:00:00Z",
              critical: false,
            },
          ],
          failures: [],
        })}
        taskTypeLabels={null}
      />,
    );
    expect(html).toMatch(/role="region"[^>]*aria-label="Danh sách cần xử lý ngay"[^>]*tabindex="0"[^>]*class="[^"]*max-h-64[^"]*overflow-auto/);
    expect(html).toContain(">1</span>");
  });
});

describe("composition — the prototype's Panel / MetricTile / alert row (ADR 0068 lần 5)", () => {
  const row = (patch: Partial<UrgentRow>): UrgentRow => ({
    module: "task",
    code: "NV-0001",
    kind: "han-xu-ly-xong",
    categoryCode: null,
    missedAt: "2026-09-01T01:00:00Z",
    critical: false,
    ...patch,
  });
  const urgent = (rows: UrgentRow[]) =>
    renderToStaticMarkup(<UrgentPanel queue={ok({ rows, failures: [] })} taskTypeLabels={null} />);

  it("block title row: small upper-case muted <h2>, no icon before it", () => {
    const html = page({ tasks: { current: ok(TASKS_ZERO), previous: ok(TASKS_ZERO) } });
    const block = /<section[^>]*data-block="tasks"[^>]*>(.*?)<\/h2>/s.exec(html)?.[1] ?? "";
    expect(block).toMatch(/<h2 class="[^"]*uppercase[^"]*">Nhiệm vụ$/);
    expect(block).toContain("text-[11.5px]");
    expect(block).not.toContain("<svg");
  });

  it("tile order is value · label · delta, the value large and bold, no icon on the label", () => {
    const html = page({
      tasks: { current: ok({ ...TASKS_ZERO, completed: 9 }), previous: ok({ ...TASKS_ZERO, completed: 8 }) },
    });
    const tile = /<a[^>]*aria-describedby="tasks-completed-value"[^>]*>(.*?)<\/a>/s.exec(html)?.[1] ?? "";
    const value = tile.indexOf('id="tasks-completed-value"');
    const label = tile.indexOf(">Hoàn thành trong kỳ<");
    const delta = tile.indexOf("+12,5% so với kỳ trước");
    expect(value).toBeGreaterThanOrEqual(0);
    expect(label).toBeGreaterThan(value);
    expect(delta).toBeGreaterThan(label);
    expect(tile).toContain("font-bold");
    // Scaled to the TILE (container query), not the window — a `vw` size outran narrow tiles.
    expect(tile).toContain("text-[clamp(16px,12cqi,26px)]");
    expect(tile).not.toMatch(/clamp\([^)]*vw/);
    // exactly one icon: the delta line's arrow
    expect(tile.match(/<svg/g)).toHaveLength(1);
  });

  it("'Cần xử lý ngay' with NO row: no count pill, no red frame, the prototype's sentence", () => {
    const html = urgent([]);
    expect(html).toContain("Không có việc nào quá hạn. Rất tốt.");
    expect(html).not.toContain("bg-danger-solid");
    expect(html).not.toContain("border-danger-200");
  });

  it("'Cần xử lý ngay' WITH rows: red count pill and red frame", () => {
    const html = urgent([row({}), row({ code: "NV-0002" })]);
    expect(html).toMatch(/<span class="[^"]*bg-danger-solid[^"]*">2<\/span>/);
    expect(html).toMatch(/<section[^>]*class="[^"]*border-danger-200[^"]*"[^>]*data-block="urgent"|data-block="urgent"[^>]*class="[^"]*border-danger-200/);
  });

  it("a TASK row links to its detail; a citizen-report or document row is not a link", () => {
    const html = urgent([
      row({ code: "NV-0001" }),
      row({ module: "citizen-report", code: "PA-7F3K-9QXR-MNPT", kind: "han-xu-ly-xong", categoryCode: "" }),
      row({ module: "incoming-document", code: "VB-12", kind: "van-ban-den" }),
    ]);
    expect(html).toContain('href="/nhiem-vu?task=NV-0001"');
    expect(html.match(/<a /g)).toHaveLength(1);
    expect(html).toContain("PA-7F3K-9QXR-MNPT");
    expect(html).toContain("VB-12");
  });

  it("a critical row writes 'Nghiêm trọng' — red is never the only sign; a normal row does not", () => {
    expect(urgent([row({ critical: true })])).toContain("Nghiêm trọng");
    expect(urgent([row({ critical: false })])).not.toContain("Nghiêm trọng");
  });

  it("header: period buttons are separate small buttons, the chosen one solid navy", () => {
    const html = renderToStaticMarkup(
      <DashboardView data={emptyData()} visible={TASK_KEYS} onPeriodChange={() => {}} />,
    );
    const pressed = /<button class="([^"]*)" type="button" aria-pressed="true"/.exec(html)?.[1] ?? "";
    expect(pressed).toContain("bg-brand-600");
    expect(html).toContain('role="group" aria-label="Kỳ báo cáo"');
    for (const f of ["PDF", "XLSX", "PPTX"]) expect(html).toMatch(new RegExp(`<button[^>]*disabled=""[^>]*><svg[^>]*>.*?</svg>${f}</button>`));
  });
});
