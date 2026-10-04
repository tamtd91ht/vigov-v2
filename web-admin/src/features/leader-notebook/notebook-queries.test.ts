import { describe, expect, it } from "vitest";

import { drillDownQuery, parseDrillDown } from "@/lib/drill-down";
import { duongDanHangChoLuiHan, duongDanSoNhiemVu, taskCountsPath, taskExtensionCountPath } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing path builders under test

import {
  ASSIGNED_BY_ME_FILTER,
  ASSIGNED_BY_ME_LIST,
  MY_EXTENSION_REQUESTS,
  OVERDUE_FILTER,
  OVERDUE_LIST,
  PAGE_SIZE,
  PENDING_APPROVAL_FILTER,
  sumCounts,
} from "./notebook-queries";

/**
 * The EXACT questions the Sổ tay asks the server (ADR 0071). A wrong parameter name is not a 400 for
 * every filter: some the server ignores, and then a column quietly shows the whole register under a
 * title that says it is a slice. So the built URLs are pinned, name by name.
 */

function params(path: string): Record<string, string> {
  return Object.fromEntries(new URL(path, "http://x").searchParams);
}

describe("column 1 — Việc quá hạn", () => {
  it("lists metric=overdue, oldest deadline first, whole commune (no scope)", () => {
    const path = duongDanSoNhiemVu({ ...OVERDUE_LIST, limit: PAGE_SIZE });
    expect(path.startsWith("/api/v1/tasks?")).toBe(true);
    expect(params(path)).toEqual({ metric: "overdue", sort: "due_at", order: "asc", limit: String(PAGE_SIZE) });
  });

  it("counts with the same filter on /task-counts", () => {
    expect(taskCountsPath(OVERDUE_FILTER)).toBe("/api/v1/task-counts?metric=overdue");
  });

  it("asks EXACTLY what /nhiem-vu?metric=overdue asks — spec 03:82 'khớp tuyệt đối'", () => {
    // The register's drill-down filter, built the way `app/nhiem-vu/page.tsx` builds it.
    const register = drillDownQuery(parseDrillDown("tasks", { metric: "overdue" }));
    expect(taskCountsPath(OVERDUE_FILTER)).toBe(taskCountsPath(register));
    // The list differs ONLY in order and page size — neither changes which rows match.
    const notebook = params(duongDanSoNhiemVu(OVERDUE_LIST));
    const fromRegister = params(duongDanSoNhiemVu({ ...register, sapXep: "created_at", chieu: "desc" }));
    for (const p of [notebook, fromRegister]) {
      delete p.sort;
      delete p.order;
    }
    expect(notebook).toEqual(fromRegister);
  });
});

describe("column 2 — Chờ tôi duyệt", () => {
  it("Duyệt hoàn thành: status=cho-duyet, scope absent (= the server's `all`)", () => {
    expect(params(duongDanSoNhiemVu({ ...PENDING_APPROVAL_FILTER, limit: PAGE_SIZE }))).toEqual({
      status: "cho-duyet",
      limit: String(PAGE_SIZE),
    });
    expect(taskCountsPath(PENDING_APPROVAL_FILTER)).toBe("/api/v1/task-counts?status=cho-duyet");
  });

  it("Duyệt lùi hạn: approver=me on the queue AND on its count route — `me`, never a staff code", () => {
    expect(duongDanHangChoLuiHan({ ...MY_EXTENSION_REQUESTS, limit: PAGE_SIZE })).toBe(
      `/api/v1/task-extensions?approver=me&limit=${PAGE_SIZE}`,
    );
    expect(taskExtensionCountPath(MY_EXTENSION_REQUESTS)).toBe("/api/v1/task-extension-counts?approver=me");
  });

  it("the count route never carries paging", () => {
    expect(taskExtensionCountPath({ approver: "me", limit: 5, cursor: "abc" })).toBe(
      "/api/v1/task-extension-counts?approver=me",
    );
    expect(taskExtensionCountPath()).toBe("/api/v1/task-extension-counts");
  });
});

describe("column 3 — Việc tôi đã giao", () => {
  it("scope=assigned-by-me & incomplete=true, newest first", () => {
    expect(params(duongDanSoNhiemVu({ ...ASSIGNED_BY_ME_LIST, limit: PAGE_SIZE }))).toEqual({
      scope: "assigned-by-me",
      incomplete: "true",
      sort: "created_at",
      order: "desc",
      limit: String(PAGE_SIZE),
    });
  });

  it("counts with the same filter, and no identity travels", () => {
    const path = taskCountsPath(ASSIGNED_BY_ME_FILTER);
    expect(path).toBe("/api/v1/task-counts?scope=assigned-by-me&incomplete=true");
    expect(path).not.toMatch(/CB-|assignee|created_by|assigner/);
  });

  it("`incomplete: false` sends nothing — the server accepts only `true`", () => {
    expect(taskCountsPath({ incomplete: false })).toBe("/api/v1/task-counts");
  });
});

describe("sumCounts", () => {
  it("adds every status the server reports — the badge is a server figure, not a row count", () => {
    expect(
      sumCounts({
        by_status: [
          { status: "moi-giao", count: 120 },
          { status: "dang-thuc-hien", count: 300 },
          { status: "tam-dung", count: 0 },
        ],
      }),
    ).toBe(420);
    expect(sumCounts({ by_status: [] })).toBe(0);
  });
});
