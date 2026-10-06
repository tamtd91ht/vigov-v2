"use client";

import { useId, type ReactNode } from "react";

import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";

/**
 * The centred box the Cấu hình screen opens a form in, where the prototype does (`OrgChart`'s
 * `OrgUnitDialog`, `CatalogueImportButton`'s `ExcelImportDialog`; ADR 0068 lần 5).
 *
 * The form inside keeps its own markup, handlers and errors unchanged: `.hop-thoai-cau-hinh` (globals)
 * only drops the in-flow frame `.form-danh-muc` draws and the form's own `<h4>`, since the dialog's
 * heading now says the same words. A second frame or a second title inside a box reads as two forms.
 *
 * `onDismiss` is Esc. The caller decides — a send in flight must not lose its form from view.
 */
export function ConfigDialog({
  title,
  description,
  onDismiss,
  size = "md",
  hideHeader = false,
  children,
}: {
  title: ReactNode;
  description?: ReactNode;
  onDismiss: () => void;
  size?: "md" | "lg";
  /**
   * The content draws its own visible title (a `ConfirmDialog`): the header stays as the dialog's
   * accessible name, out of view, so the title is not shown twice.
   */
  hideHeader?: boolean;
  children: ReactNode;
}) {
  const titleId = useId();
  const header = <ModalDialogHeader titleId={titleId} title={title} description={description} />;
  return (
    <ModalDialog titleId={titleId} onDismiss={onDismiss} size={size} className="hop-thoai-cau-hinh">
      {hideHeader ? <div className="an-thi-giac">{header}</div> : header}
      {/* The scroll region the dialog's layout contract asks for: a tall form keeps its buttons reachable. */}
      <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto [&>*]:my-0">{children}</div>
    </ModalDialog>
  );
}
