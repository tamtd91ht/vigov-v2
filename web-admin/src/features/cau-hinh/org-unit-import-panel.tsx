"use client";

import type { KetQua } from "@/lib/api/goi";
import type { OrgUnitImportResult } from "@/lib/api/org-unit-import";
import type { identity_orgUnitImportPreviewOut } from "@/lib/api/schema.gen";

import { ExcelImportPanel, ExcelImportView } from "./excel-import-panel";
import type { ImportBusy } from "./excel-import-panel";
import { ORG_UNIT_IMPORT_TARGET } from "./excel-import-targets";

export { ErrorsTable } from "./excel-import-panel";

/**
 * The org chart's import panel, opened by "⬆ Nhập từ Excel" (only drawn for `admin.org`; the server
 * checks the same key on all three routes). `onImported` makes the tab read the tree again: the created
 * units' staff counts exist only on the server.
 *
 * The panel is the shared one (`excel-import-panel.tsx`) bound to `ORG_UNIT_IMPORT_TARGET`: the org
 * chart was the first import, and its flow is now every import's flow.
 */
export function OrgUnitImportPanel({
  onImported,
  onClose,
  asDialog = false,
}: {
  onImported: () => void;
  onClose: () => void;
  asDialog?: boolean;
}) {
  return <ExcelImportPanel target={ORG_UNIT_IMPORT_TARGET} onImported={onImported} onClose={onClose} asDialog={asDialog} />;
}

/**
 * Pure rendering in the org chart's CONTRACT shape (`units`), kept so its tests read the generated
 * type. It only renames `units` to the shared view's `rows`.
 */
export function OrgUnitImportView({
  preview,
  ...rest
}: {
  fileChosen: boolean;
  preview: KetQua<identity_orgUnitImportPreviewOut> | null;
  result: OrgUnitImportResult | null;
  busy: ImportBusy;
  templateError: string;
  onDownloadTemplate: () => void;
  onChooseFile: (f: File | null) => void;
  onPreview: () => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const normalised =
    preview === null || !preview.ok
      ? preview
      : {
          ok: true as const,
          duLieu: { valid: preview.duLieu.valid, rows: preview.duLieu.units, errors: preview.duLieu.errors },
        };
  return <ExcelImportView target={ORG_UNIT_IMPORT_TARGET} preview={normalised} {...rest} />;
}
