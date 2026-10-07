"use client";

import { useState } from "react";

import { CategoryManagerDialog } from "@/features/giai-ngan/category-manager-dialog";
import { ChiTietDuAn } from "@/features/giai-ngan/chi-tiet-du-an";
import { DisbursementImportDialog } from "@/features/giai-ngan/disbursement-import-dialog";
import { DisbursementWorkspace } from "@/features/giai-ngan/disbursement-workspace";
import { FundingSourceProjectsDialog, ManageFundingSourcesDialog } from "@/features/giai-ngan/funding-source-dialogs";
import { DISBURSEMENT_READ_DENIED } from "@/features/giai-ngan/nhan-du-an";
import { TAB_ID } from "@/features/giai-ngan/pending-parts";
import { ProjectBulkDeleteDialog } from "@/features/giai-ngan/project-bulk-delete-dialog";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

import {
  PREVIEW_PROJECT_ID,
  previewFundingSources,
  previewImplementingUnits,
  previewProjects,
} from "./disbursement.fixture";
import { isDevPreviewPath } from "./preview-gate";
import type { PreviewModal, PreviewTab } from "./preview-params";
import { usePressWhenReady } from "./preview-shell";

/** `?tab=` words mapped to the REAL tab ids of `ProjectRecordTabs`. */
const TAB_IDS: Record<PreviewTab, string> = {
  "vuong-mac": TAB_ID.issues,
  "chung-tu": TAB_ID.vouchers,
  "bieu-do": TAB_ID.chart,
  "trao-doi": TAB_ID.discussion,
};

/**
 * The list screen: the REAL `DisbursementWorkspace` (header, KPI cards, cumulative chart, category
 * table, funding-source block, filter bar, project register — all fed by the fixture answering
 * machine), plus the dialog `modal` names, opened over it:
 *   - the REAL exported dialogs are mounted directly (Hạng mục, Nhập giải ngân, Quản lý nguồn vốn,
 *     a source's projects, bulk delete with two projects selected);
 *   - `them-du-an` presses the workspace's own `+ Thêm dự án` button, because that dialog is not
 *     exported — the preview never copies a component.
 * Closing a dialog closes it; a write inside one is refused by the answering machine.
 */
export function DisbursementPreview({ modal }: { modal: PreviewModal | null }) {
  useState(() => {
    if (typeof window !== "undefined") installImplementingUnitsAnswer();
    return true;
  });
  const [open, setOpen] = useState<PreviewModal | null>(modal);
  const close = () => setOpen(null);
  const noop = () => {};
  // The workspace anchors its year on the machine clock (`lib/nam.ts`); the dialogs read the same year.
  const [year] = useState(() => new Date().getFullYear());
  const sources = previewFundingSources(year).items;
  const selected = previewProjects(year)
    .items.slice(1, 3)
    .map((p) => ({ id: p.id, code: p.code, name: p.name, planned_amount: p.planned_amount }));

  usePressWhenReady(open === "them-du-an" ? 'button[aria-haspopup="dialog"]' : null, "Thêm dự án");

  return (
    <>
      <DisbursementWorkspace />
      {open === "hang-muc" && <CategoryManagerDialog onClose={close} onChanged={noop} />}
      {open === "nhap-excel" && <DisbursementImportDialog onClose={close} onImported={noop} />}
      {open === "them-nguon" && <ManageFundingSourcesDialog year={year} sources={sources} onChanged={noop} onClose={close} />}
      {open === "chi-tiet-nguon" && <FundingSourceProjectsDialog source={sources[0]!} year={year} onClose={close} />}
      {open === "xoa-nhieu" && <ProjectBulkDeleteDialog projects={selected} onClose={close} onFinished={noop} />}
    </>
  );
}

/**
 * The project detail page: the REAL `ChiTietDuAn` behind the same `CongQuyen` and wrapper as
 * `app/giai-ngan/du-an/[id]/page.tsx`, on the fixture project with vouchers in all three states, two
 * issues and two comments. `tab` presses the REAL tab button (its state is internal).
 */
export function ProjectDetailPreview({ tab }: { tab: PreviewTab }) {
  useState(() => {
    if (typeof window !== "undefined") installImplementingUnitsAnswer();
    return true;
  });
  usePressWhenReady(tab === "vuong-mac" ? null : `#${TAB_IDS[tab]}`);
  return (
    <div className="mx-auto w-full max-w-[76rem] min-w-0">
      <h1 className="an-thi-giac">Chi tiết dự án</h1>
      <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={DISBURSEMENT_READ_DENIED}>
        <ChiTietDuAn id={PREVIEW_PROJECT_ID} />
      </CongQuyen>
    </div>
  );
}

let unitsAnswerInstalled = false;

/**
 * `GET /api/v1/implementing-units?year=` (the project form's typed-unit suggestions, 65afbdcd) answered
 * HERE from `disbursement.fixture.ts`: the shared answering machine (`fixture-fetch.ts`) has no route for
 * it and is another session's file. A wrapper over the installed fixture fetch, set once in the first
 * render — after `PreviewShell` installed its own — answering only that GET on a preview path and
 * delegating every other call unchanged (the `tasks-preview.tsx` extension-history pattern).
 */
function installImplementingUnitsAnswer(): void {
  if (unitsAnswerInstalled) return;
  unitsAnswerInstalled = true;
  const next = window.fetch.bind(window);
  window.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(raw, window.location.origin);
    const method = (init?.method ?? (input instanceof Request ? input.method : "GET")).toUpperCase();
    if (
      !isDevPreviewPath(window.location.pathname) ||
      url.origin !== window.location.origin ||
      url.pathname !== "/api/v1/implementing-units" ||
      method !== "GET"
    ) {
      return next(input, init);
    }
    const year = Number(url.searchParams.get("year")) || new Date().getFullYear();
    const body = JSON.stringify(previewImplementingUnits(year));
    return Promise.resolve(new Response(body, { status: 200, headers: { "Content-Type": "application/json" } }));
  };
}
