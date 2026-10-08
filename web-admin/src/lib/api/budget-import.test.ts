import { afterEach, describe, expect, it, vi } from "vitest";

import { budgetImportPath, commitBudgetImport } from "./budget-import";
import { FILE_TYPE_FALLBACK } from "./excel-import";

afterEach(() => vi.unstubAllGlobals());

function answer(res: Response) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => res),
  );
}

const FILE = new Blob(["x"]);

describe("budget-import client", () => {
  it("sends `year` as the query word the contract declares", () => {
    expect(budgetImportPath(2026)).toBe(
      "/api/v1/budget-sheets/imports?year=2026",
    );
  });

  it("a replayed 201 is still a success, with no body to trust", async () => {
    answer(
      new Response(JSON.stringify({ code: "NS-2026-CHI-02", replayed: true }), {
        status: 201,
      }),
    );
    expect(await commitBudgetImport(FILE, "a.xlsx", 2026, "k")).toEqual({
      ok: true,
      out: null,
    });
  });

  it("409: the server's sentence (prefix removed) and its code", async () => {
    answer(
      new Response(
        JSON.stringify({
          code: "budget_period_closed",
          message: "ngan_sach: kỳ năm 2026 đã chốt",
          trace_id: "",
        }),
        {
          status: 409,
        },
      ),
    );
    expect(await commitBudgetImport(FILE, "a.xlsx", 2026, "k")).toEqual({
      ok: false,
      message: "Kỳ năm 2026 đã chốt",
      code: "budget_period_closed",
      errors: [],
    });
  });

  it("a proxy's 415 page: the file-type fallback, no code", async () => {
    answer(new Response("<html>", { status: 415 }));
    expect(await commitBudgetImport(FILE, "a.xlsx", 2026, "k")).toEqual({
      ok: false,
      message: FILE_TYPE_FALLBACK,
      errors: [],
    });
  });
});
