"use client";

import { Upload } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";

import type { ImportTarget } from "./excel-import-flow";
import { ExcelImportDialog } from "./excel-import-dialog";
import { SMALL_BUTTON_CLASS } from "./config-ui";

/**
 * "Nhập từ Excel" at the head of Sơ đồ tổ chức, Thôn / Tổ dân phố and Danh mục (spec `02-khung-trang.md`,
 * the prototype's `CatalogueImportButton`): a right-aligned small outline Upload button above the tab,
 * opening the import as a dialog.
 *
 * THE FLOW IS NOT THE PROTOTYPE'S ONE ROUTE. The prototype imports through `/admin/catalogues/{key}`;
 * this system imports through each owning service's three routes (ADR 0059), already bound per
 * resource in `excel-import-targets.tsx`. So this takes a `target` and opens the shared 672px
 * `ExcelImportDialog` (spec §1, user 09/10/2026) — the same dialog `/nguoi-dung` opens for staff. The
 * dialog title is the target's own (`Nhập … từ Excel`); `title` overrides it when a tab needs the
 * spec's wording.
 *
 * Gating is the caller's: render this only for an account holding the target's write key (the
 * prototype hides the button when the server does not list the catalogue). The server checks the key
 * on every import route regardless.
 */
export function ConfigImportButton<R, C = R>({
  target,
  onImported,
  title,
}: {
  target: ImportTarget<R, C>;
  /** Read the tab's rows again: what the import created exists only on the server. */
  onImported: () => void;
  title?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <div className="mb-3 flex justify-end">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={SMALL_BUTTON_CLASS}
          icon={<Upload aria-hidden="true" focusable="false" className="size-4" />}
          onClick={() => setOpen(true)}
        >
          Nhập từ Excel
        </Button>
      </div>
      {open && (
        <ExcelImportDialog
          target={title === undefined ? target : { ...target, title }}
          onImported={onImported}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}
