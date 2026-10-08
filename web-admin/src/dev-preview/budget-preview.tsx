"use client";

import { useEffect, useState } from "react";

import { BangThuChi } from "@/features/thu-chi/bang-thu-chi"; // vi-name-ok: existing component, rendered as-is
import { CAU_THIEU_QUYEN_XEM } from "@/features/thu-chi/nhan-thu-chi"; // vi-name-ok: existing sentence constant
import { CongQuyen } from "@/features/quyen/cong-quyen"; // vi-name-ok: existing gate component
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

import {
  budgetPreviewCase,
  previewBudgetCloses,
  previewBudgetImportAnswer,
  previewBudgetIndicators,
  previewBudgetSheet,
  type BudgetPreviewModal,
} from "./budget.fixture";
import { isDevPreviewPath } from "./preview-gate";

/**
 * `/xem-thu/thu-chi`: the REAL `BangThuChi` behind the same `CongQuyen` as `app/giai-ngan/thu-chi/page.tsx`,
 * on fixture data. `?modal=nap-excel` sets a stand-in `.xlsx` on the REAL hidden picker of `Nạp từ Excel`,
 * so the REAL upload runs and its `?case=` answer becomes the REAL toast.
 *
 * THE PICK IS DELAYED (`PICK_DELAY_MS`): a toast lives a few seconds, and headless Chrome takes its picture
 * when its virtual clock (15 s) runs out — picked at once, the toast would be gone from the picture.
 */
export function BudgetPreview({ modal }: { modal: BudgetPreviewModal | null }) {
  useState(() => {
    if (typeof window !== "undefined") installBudgetAnswers();
    return true;
  });
  usePickStandInFile(modal === "nap-excel");
  return (
    <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={CAU_THIEU_QUYEN_XEM}>
      <BangThuChi />
    </CongQuyen>
  );
}

const PICK_DELAY_MS = 11_000;

function usePickStandInFile(on: boolean): void {
  useEffect(() => {
    if (!on) return;
    const timer = window.setTimeout(() => {
      const input = document.querySelector<HTMLInputElement>(
        'input[type="file"][accept=".xlsx"]',
      );
      if (input === null) return;
      const dt = new DataTransfer();
      dt.items.add(new File(["xem-thu"], "bao-cao-thu-chi-ngan-sach.xlsx"));
      input.files = dt.files;
      input.dispatchEvent(new Event("change", { bubbles: true }));
    }, PICK_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [on]);
}

let installed = false;

/**
 * The Thu - Chi routes answered HERE (the shared answering machine has none), as a wrapper over the
 * installed fixture fetch — the `disbursement-preview.tsx` implementing-units pattern. Only on a preview
 * path; every other call is delegated unchanged.
 */
function installBudgetAnswers(): void {
  if (installed) return;
  installed = true;
  const next = window.fetch.bind(window);
  window.fetch = (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, window.location.origin);
    const method = (
      init?.method ?? (input instanceof Request ? input.method : "GET")
    ).toUpperCase();
    if (
      !isDevPreviewPath(window.location.pathname) ||
      url.origin !== window.location.origin
    )
      return next(input, init);
    const r = answerBudget(method, url);
    return r === null ? next(input, init) : Promise.resolve(r);
  };
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function answerBudget(method: string, url: URL): Response | null {
  const year = Number(url.searchParams.get("year")) || new Date().getFullYear();
  const p = url.pathname;
  if (method === "GET" && p === "/api/v1/budget-sheets") {
    const sheet = previewBudgetSheet(year, url.searchParams.get("kind") ?? "");
    return sheet === null
      ? json(
          {
            code: "not_found",
            message: "ngan_sach: không có bảng ngân sách này trong xã",
            trace_id: "",
          },
          404,
        )
      : json(sheet);
  }
  if (method === "GET" && p === "/api/v1/budget-indicators")
    return json(previewBudgetIndicators(year));
  if (method === "GET" && p === "/api/v1/budget-period-closes")
    return json(previewBudgetCloses(year));
  if (method === "POST" && p === "/api/v1/budget-sheets/imports") {
    const a = previewBudgetImportAnswer(
      year,
      budgetPreviewCase(
        new URLSearchParams(window.location.search).get("case"),
      ),
    );
    return json(a.body, a.status);
  }
  return null;
}
