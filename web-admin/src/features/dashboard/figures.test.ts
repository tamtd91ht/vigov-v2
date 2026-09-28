import { describe, expect, it } from "vitest";

import type { documents_overdueQueueOut, petitions_overdueQueueOut } from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";

import {
  canSeeBlock,
  categoryLabel,
  comparisonLine,
  drillHref,
  drillTargetLabel,
  isPeriodTarget,
  formatCount,
  formatPercent,
  kindLabel,
  mergeQueues,
  missedSinceText,
  NO_VALUE,
  ratioPercent,
  URGENT_ROW_CAP,
} from "./figures";
import type { DrillTarget, QueueSource, UrgentRow } from "./figures";

const PERIOD = { from: "2026-09-01T00:00:00+07:00", to: "2026-09-28T16:43:01+07:00" };

describe("ratio — sample 0 is '—', never 0%", () => {
  it("on_time / sample as a percentage, one decimal, vi-VN", () => {
    expect(formatPercent(ratioPercent(1, 3))).toBe("33,3%");
    expect(formatPercent(ratioPercent(0, 4))).toBe("0,0%");
  });

  it("an empty sample has no rate", () => {
    expect(ratioPercent(0, 0)).toBeNull();
    expect(formatPercent(ratioPercent(0, 0))).toBe(NO_VALUE);
  });

  it("counts are grouped the Vietnamese way", () => {
    expect(formatCount(1234)).toBe("1.234");
    expect(formatCount(0)).toBe("0");
  });
});

describe("comparison line — period figures only, % only when previous is neither '—' nor 0", () => {
  it("a STOCK figure has no line at all (not even 'chưa có kỳ trước')", () => {
    expect(comparisonLine("stock", 14, 10, "lower-is-better", formatCount)).toBeNull();
  });

  it("previous failed → states '—', no percentage", () => {
    expect(comparisonLine("period", 3, null, "higher-is-better", formatCount)).toEqual({
      text: "Kỳ trước: —",
      tone: "neutral",
    });
  });

  it("previous 0 → states 0, no percentage (not +∞%)", () => {
    expect(comparisonLine("period", 3, 0, "higher-is-better", formatCount)).toEqual({
      text: "Kỳ trước: 0",
      tone: "neutral",
    });
  });

  it("current failed → no line (the cell already says '—')", () => {
    expect(comparisonLine("period", null, 5, "higher-is-better", formatCount)).toBeNull();
  });

  it("equal → 'không đổi'", () => {
    expect(comparisonLine("period", 5, 5, "higher-is-better", formatCount)?.text).toBe(
      "Không đổi so với kỳ trước",
    );
  });

  it("rise of a good figure is better; rise of a bad figure is worse; volumes stay neutral", () => {
    expect(comparisonLine("period", 9, 8, "higher-is-better", formatCount)).toEqual({
      text: "↑ +12,5% so với kỳ trước",
      tone: "better",
    });
    expect(comparisonLine("period", 4, 2, "lower-is-better", formatCount)).toEqual({
      text: "↑ +100,0% so với kỳ trước",
      tone: "worse",
    });
    expect(comparisonLine("period", 0, 2, "higher-is-better", formatCount)).toEqual({
      text: "↓ -100,0% so với kỳ trước",
      tone: "worse",
    });
    expect(comparisonLine("period", 12, 10, "neutral", formatCount)?.tone).toBe("neutral");
  });
});

describe("drill-down hrefs — the URL contract of the list screens", () => {
  const q = (href: string) => new URL(href, "https://xa.example").searchParams;

  it("task STOCK metrics carry no period", () => {
    for (const metric of ["in_progress", "overdue", "suspended"] as const) {
      const href = drillHref({ list: "tasks", metric }, PERIOD);
      expect(href).toBe(`/nhiem-vu?metric=${metric}`);
    }
  });

  it("task PERIOD metrics carry the same from/to, with `+` encoded", () => {
    const href = drillHref({ list: "tasks", metric: "completed" }, PERIOD);
    expect(href.startsWith("/nhiem-vu?")).toBe(true);
    expect(href).toContain("%2B07%3A00");
    expect(q(href).get("metric")).toBe("completed");
    expect(q(href).get("from")).toBe(PERIOD.from);
    expect(q(href).get("to")).toBe(PERIOD.to);
    expect(q(drillHref({ list: "tasks", metric: "on_time" }, PERIOD)).get("from")).toBe(PERIOD.from);
  });

  it("citizen reports: in_progress is stock, the other four are period", () => {
    expect(drillHref({ list: "citizen-reports", metric: "in_progress" }, PERIOD)).toBe(
      "/phan-anh?metric=in_progress",
    );
    for (const metric of ["received", "on_time_sample", "on_time", "late"] as const) {
      const href = drillHref({ list: "citizen-reports", metric }, PERIOD);
      expect(href.startsWith("/phan-anh?")).toBe(true);
      expect(q(href).get("to")).toBe(PERIOD.to);
    }
  });

  it("incoming documents: `open` and `overdue` send NO from/to (the server answers 400)", () => {
    expect(drillHref({ list: "incoming-documents", metric: "open" }, PERIOD)).toBe(
      "/van-ban?metric=open",
    );
    expect(drillHref({ list: "incoming-documents", metric: "overdue" }, PERIOD)).toBe(
      "/van-ban?metric=overdue",
    );
    const arrived = drillHref({ list: "incoming-documents", metric: "arrived" }, PERIOD);
    expect(q(arrived).get("from")).toBe(PERIOD.from);
  });
});

describe("every link round-trips through the receiver (`parseDrillDown`) as VALID", () => {
  const ALL: readonly DrillTarget[] = [
    ...(["in_progress", "overdue", "suspended", "completed", "on_time_sample", "on_time"] as const).map(
      (metric) => ({ list: "tasks" as const, metric }),
    ),
    ...(["in_progress", "received", "on_time_sample", "on_time", "late"] as const).map((metric) => ({
      list: "citizen-reports" as const,
      metric,
    })),
    ...(["arrived", "open", "overdue"] as const).map((metric) => ({
      list: "incoming-documents" as const,
      metric,
    })),
  ];

  it.each(ALL.map((t) => [`${t.list}:${t.metric}`, t] as const))("%s", (_name, target) => {
    const href = drillHref(target, PERIOD);
    // exactly what the browser hands the page: one value per key, `%2B` decoded back to `+`
    const params = Object.fromEntries(new URLSearchParams(href.slice(href.indexOf("?") + 1)));
    const parsed = parseDrillDown(target.list, params);
    expect(parsed.kind).toBe("active");
    if (parsed.kind !== "active") return;
    expect(parsed.metric).toBe(target.metric);
    expect(parsed.period).toEqual(isPeriodTarget(target) ? PERIOD : null);
    expect(drillTargetLabel(target, PERIOD)).toBe(parsed.label);
  });

  it("a raw `+` would NOT round-trip — the reason the offset must be encoded", () => {
    const raw = `metric=completed&from=${PERIOD.from}&to=${PERIOD.to}`;
    expect(parseDrillDown("tasks", Object.fromEntries(new URLSearchParams(raw))).kind).toBe(
      "invalid",
    );
  });
});

describe("canSeeBlock — BOTH keys", () => {
  it("report.read + module key → shown", () => {
    expect(canSeeBlock(["report.read", "task.read"], "task.read")).toBe(true);
  });

  it("module key without report.read → hidden (denied case)", () => {
    expect(canSeeBlock(["task.read"], "task.read")).toBe(false);
  });

  it("report.read without the module key → hidden (denied case)", () => {
    expect(canSeeBlock(["report.read", "document.read"], "task.read")).toBe(false);
  });
});

function petitionsOk(items: petitions_overdueQueueOut["items"]) {
  return { ok: true as const, duLieu: { items } };
}

describe("mergeQueues — three queues, two row shapes", () => {
  const tasks: QueueSource = {
    module: "task",
    result: petitionsOk([
      {
        kind: "han-xu-ly",
        code: "NV07",
        category_code: "theo-van-ban",
        missed_deadline: "2026-09-10T01:00:00Z",
        critical: true,
      },
    ]),
  };
  const reports: QueueSource = {
    module: "citizen-report",
    result: petitionsOk([
      {
        kind: "han-phan-loai",
        code: "PA-4K7M-92XR-BTVD",
        category_code: "",
        missed_deadline: "2026-09-05T01:00:00Z",
        critical: false,
      },
    ]),
  };
  const docs: QueueSource = {
    module: "incoming-document",
    result: {
      ok: true,
      duLieu: {
        items: [
          {
            kind: "van-ban-den",
            id: "01JDOC",
            code: "VB-DEN-2026-0007",
            due_at: "2026-09-08T01:00:00Z",
            critical: false,
            holding_unit: "01JUNIT",
          },
        ],
        as_of: "2026-09-28T09:00:00Z",
      } satisfies documents_overdueQueueOut,
    },
  };

  it("normalises missed_deadline and due_at and sorts oldest deadline first", () => {
    const m = mergeQueues([tasks, reports, docs]);
    expect(m.rows.map((r) => r.code)).toEqual(["PA-4K7M-92XR-BTVD", "VB-DEN-2026-0007", "NV07"]);
    expect(m.rows[1]?.missedAt).toBe("2026-09-08T01:00:00Z");
    expect(m.failures).toEqual([]);
  });

  it("drops the document row's id and holding unit — they cannot be rendered by a later edit", () => {
    const row = mergeQueues([docs]).rows[0] as UrgentRow;
    expect(Object.keys(row).sort()).toEqual(
      ["categoryCode", "code", "critical", "kind", "missedAt", "module"].sort(),
    );
    expect(JSON.stringify(row)).not.toContain("01JUNIT");
    expect(JSON.stringify(row)).not.toContain("01JDOC");
  });

  it("keeps at most 10 rows", () => {
    const many: QueueSource = {
      module: "task",
      result: petitionsOk(
        Array.from({ length: 8 }, (_, i) => ({
          kind: "han-xu-ly",
          code: `NV${String(i).padStart(2, "0")}`,
          category_code: "",
          missed_deadline: `2026-09-${String(i + 10).padStart(2, "0")}T01:00:00Z`,
          critical: false,
        })),
      ),
    };
    const m = mergeQueues([many, reports, docs, tasks]);
    expect(m.rows).toHaveLength(URGENT_ROW_CAP);
    expect(m.rows[0]?.code).toBe("PA-4K7M-92XR-BTVD");
  });

  it("a 503 queue becomes a FAILURE with the server's sentence, and the others still show", () => {
    const failed: QueueSource = {
      module: "citizen-report",
      result: {
        ok: false,
        thongBao: "Chưa tính được mức độ nghiêm trọng theo lịch làm việc của xã.",
      },
    };
    const m = mergeQueues([tasks, failed]);
    expect(m.failures).toEqual([
      {
        module: "citizen-report",
        message: "Chưa tính được mức độ nghiêm trọng theo lịch làm việc của xã.",
      },
    ]);
    expect(m.rows.map((r) => r.code)).toEqual(["NV07"]);
  });

  it("an unreadable deadline sorts last, not first", () => {
    const odd: QueueSource = {
      module: "task",
      result: petitionsOk([
        { kind: "han-xu-ly", code: "NVX", category_code: "", missed_deadline: "?", critical: false },
      ]),
    };
    expect(mergeQueues([odd, tasks]).rows.map((r) => r.code)).toEqual(["NV07", "NVX"]);
  });
});

describe("row labels", () => {
  const base = { code: "X", kind: "han-xu-ly", missedAt: "", critical: false } as const;

  it("petition field: '' is 'Chưa phân loại', a platform code resolves, an unknown code shows", () => {
    expect(categoryLabel({ ...base, module: "citizen-report", categoryCode: "" }, null)).toBe(
      "Chưa phân loại",
    );
    expect(
      categoryLabel({ ...base, module: "citizen-report", categoryCode: "rac-thai" }, null),
    ).toBe("Rác thải – Vệ sinh môi trường");
    expect(categoryLabel({ ...base, module: "citizen-report", categoryCode: "moi" }, null)).toBe(
      "moi",
    );
  });

  it("task type: the catalogue label when it loaded, otherwise the code", () => {
    const labels = new Map([["theo-van-ban", "Theo văn bản"]]);
    const row = { ...base, module: "task" as const, categoryCode: "theo-van-ban" };
    expect(categoryLabel(row, labels)).toBe("Theo văn bản");
    expect(categoryLabel(row, null)).toBe("theo-van-ban");
  });

  it("documents have no category", () => {
    expect(categoryLabel({ ...base, module: "incoming-document", categoryCode: null }, null)).toBe(
      null,
    );
  });

  it("kind labels, and an unknown kind is named, not guessed", () => {
    expect(kindLabel("han-phan-loai")).toBe("Hạn phân loại");
    expect(kindLabel("han-xu-ly-xong")).toBe("Hạn xử lý xong");
    expect(kindLabel("moi")).toContain("moi");
  });

  it("the deadline itself, in Viet Nam — never a computed duration", () => {
    expect(missedSinceText("2026-09-01T01:00:00Z")).toBe("quá hạn từ 08:00 01/09/2026");
    expect(missedSinceText("2026-09-01T01:00:00Z")).not.toMatch(/ngày|giờ/);
  });
});
