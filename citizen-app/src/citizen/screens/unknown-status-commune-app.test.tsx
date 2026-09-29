import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { MyReportSummary } from "../api/citizen-report-contract";

import { STATUS_UNLABELLED, COMMUNE_APP_SCREENS } from "./copy";
import { StatusChip, CommuneReportCard, CommuneReportList, stepLabel } from "./CommuneAppReports";
import { STATUS_GROUP_LABEL } from "./status-groups";

/**
 * An unknown status code in the commune's own app gets the SAME neutral treatment as the shared app
 * (`statusLabel`): never a guessed group, never "Đã đóng" on a ticket that may still be open, never the raw
 * code. The contract types `status` as a bare string, so a tenth status can arrive before this app is updated.
 */

const UNKNOWN = "trang-thai-moi";

function row(status: string, code: string): MyReportSummary {
  return {
    lookup_code: code,
    status,
    field: "",
    field_label: "",
    content_excerpt: "Đèn đường hỏng",
    clock_from: "2026-09-28T01:00:00Z",
    acknowledge_due: null,
    resolve_due: null,
    rating: null,
    rated_at: null,
  };
}

const ready = (items: MyReportSummary[], hasMore = false) =>
  ({ kind: "ready", items, cursor: hasMore ? "c" : "", hasMore, loadingMore: false, moreFailure: null }) as const;

describe("commune app: unknown status code", () => {
  it("the chip shows the neutral sentence and a neutral style — not a group label, not the raw code", () => {
    const html = renderToStaticMarkup(createElement(StatusChip, { status: UNKNOWN }));
    expect(html).toBe(`<span class="xa-chip-tt xa-chip-tt--unknown">${STATUS_UNLABELLED}</span>`);
    for (const label of Object.values(STATUS_GROUP_LABEL)) expect(html).not.toContain(label);
    expect(html).not.toContain(UNKNOWN);
    expect(html).not.toContain("xa-chip-tt--da-dong");
  });

  it("a known code still shows its group chip", () => {
    const html = renderToStaticMarkup(createElement(StatusChip, { status: "khong-tiep-nhan" }));
    expect(html).toBe(`<span class="xa-chip-tt xa-chip-tt--da-dong">${STATUS_GROUP_LABEL["da-dong"]}</span>`);
  });

  it("the petition card carries the neutral chip", () => {
    const html = renderToStaticMarkup(createElement(CommuneReportCard, { report: row(UNKNOWN, "PA-1"), onOpen: () => {} }));
    expect(html).toContain(STATUS_UNLABELLED);
    expect(html).not.toContain(STATUS_GROUP_LABEL["da-dong"]);
  });

  it("the list counts it under 'Tất cả' only — no group, 'Đã đóng' included, claims it", () => {
    const html = renderToStaticMarkup(
      createElement(CommuneReportList, {
        state: ready([row(UNKNOWN, "PA-AAAAAAAAAAAA"), row("da-dong", "PA-BBBBBBBBBBBB")]),
        onOpenReport: () => {},
        onLookup: () => {},
        onOpen: () => {},
        onRetry: () => {},
        onLoadMore: () => {},
      }),
    );
    expect(html).toContain(`${COMMUNE_APP_SCREENS.filter_all} (2)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-dong"]} (1)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-tiep-nhan"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["dang-xu-ly"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-xu-ly-xong"]} (0)`);
    // Shown in "Tất cả" (the default filter), with the neutral sentence.
    expect(html).toContain("PA-AAAAAAAAAAAA");
    expect(html).toContain(STATUS_UNLABELLED);
  });

  it("the timeline never prints a raw unknown code", () => {
    expect(stepLabel(UNKNOWN)).toBe(STATUS_UNLABELLED);
    expect(stepLabel("toString")).toBe(STATUS_UNLABELLED);
    expect(stepLabel("dang-xu-ly")).toBe("Đang xử lý");
  });
});
