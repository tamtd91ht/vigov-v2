"use client";

import { DownloadCloud, Link2, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { PendingButton, PendingMarker } from "@/components/ui/pending-feature";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import type { UpdateCategoryIn } from "@/lib/api/noi-dung";
import type { comms_danhMucRa } from "@/lib/api/schema.gen";

import {
  CATEGORY_DELETE_BUTTON,
  CATEGORY_DELETED_TOAST,
  CATEGORY_DELETE_NOTE,
  CATEGORY_EMPTY,
  CATEGORY_FROM_PORTAL_PART,
  CATEGORY_IMPORT_NOTE,
  CATEGORY_IMPORT_PART,
  CATEGORY_ITEM_COUNT_PART,
  CATEGORY_REASON_LABEL,
  CATEGORY_RENAMED_TOAST,
  CATEGORY_TURN_OFF,
  CATEGORY_TURN_ON,
  DELETE_REASON_MAX_CHARS,
  groupCategoriesByParent,
  NHAN_NUT_HUY,
  pendingContentPart,
  TEN_DANH_MUC_TOI_DA,
} from "./nhan-noi-dung";

/**
 * `⊞ Danh mục tin` — the list of the prototype's `CategoryManagerDialog` (`CategoryManagerDialog.tsx:168-316`),
 * grouped by parent, the categories with no parent first. Each row: the name, editable IN PLACE (Enter or
 * leaving the box saves, Esc puts the old name back) · `{n} bài` and `Từ Cổng` as disabled "?" (no route
 * serves either, owner D4 09/10/2026) · `Tắt` / `Bật` · the bin, which opens the soft delete with a reason
 * (rule 7 — the prototype's two-step `Xoá hẳn` is a hard delete this system does not have).
 *
 * A TURNED-OFF ROW IS DIMMED, as the prototype draws it (`CategoryManagerDialog.tsx:244,257`): the page
 * colour behind it, the name struck through. That is the state's only mark, so no badge repeats it.
 *
 * EVERY REFUSAL IS THE SERVER'S SENTENCE, VERBATIM. `category_cycle`, `parent_missing`,
 * `category_not_empty`, the self-parent 400 and the slug 400 each come with a Vietnamese sentence written
 * for this case (`service-comms/internal/http/noi_dung_mini_app.go`, `categoryWriteError`); `KetQua`
 * deliberately carries no `code` (`lib/api/goi.ts`), so there is nothing here to branch on and no second
 * copy of the rule to drift.
 *
 * NO PERMISSION GATE HERE, same as the rest of the screen: `content.update` is checked by the server on
 * every write, and a missing key reaches the officer as the 403 sentence (rule 5, forbidden #1).
 *
 * The two writes are props so a test can watch the exact bodies without a network.
 */
export function CategoryAdmin({
  categories,
  update,
  remove,
  changed,
}: {
  categories: readonly comms_danhMucRa[];
  update: (id: string, body: UpdateCategoryIn) => Promise<KetQua<comms_danhMucRa>>;
  remove: (id: string, reason: string) => Promise<KetQua<null>>;
  /** Called after a successful write: the parent re-reads the tree (another officer may have moved it). */
  changed: () => void;
}) {
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ id: string; message: string } | null>(null);

  function run(id: string, call: Promise<KetQua<unknown>>, okToast?: string): Promise<boolean> {
    setBusy(true);
    return call.then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setError({ id, message: kq.thongBao });
        return false;
      }
      setError(null);
      setDeletingId(null);
      if (okToast !== undefined) toast.success(okToast);
      changed();
      return true;
    });
  }

  return (
    <div className="category-admin max-h-[26rem] space-y-3 overflow-y-auto pr-1">
      {categories.length === 0 ? (
        <p className="m-0 py-6 text-center text-[12.5px] text-ink-muted">{CATEGORY_EMPTY}</p>
      ) : (
        groupCategoriesByParent(categories).map((g) => (
          <div key={g.parentId === "" ? "-" : g.parentId}>
            <p className="m-0 mb-1.5 text-[11px] font-semibold tracking-wide text-ink-muted uppercase">{g.name}</p>
            <ul className="m-0 list-none space-y-1.5 p-0">
              {g.items.map((dm) => {
                const rowError = error !== null && error.id === dm.id ? error.message : null;
                const deleting = deletingId === dm.id;
                return (
                  <li key={dm.id} className="m-0">
                    <CategoryRow
                      // Keyed by the name too: a reload with a new name rebuilds the box from it.
                      key={`${dm.id}|${dm.name}`}
                      category={dm}
                      busy={busy}
                      deleting={deleting}
                      rename={(name) => run(dm.id, update(dm.id, { name }), CATEGORY_RENAMED_TOAST)}
                      toggle={() => void run(dm.id, update(dm.id, { hidden: !dm.hidden }))}
                      askDelete={() => {
                        setError(null);
                        setDeletingId(deleting ? null : dm.id);
                      }}
                    />
                    {rowError !== null && !deleting && (
                      <p className="m-0 mt-1 text-[12px] font-medium text-danger" role="alert">
                        {rowError}
                      </p>
                    )}
                    {deleting && (
                      <CategoryDeleteForm
                        category={dm}
                        busy={busy}
                        error={rowError}
                        cancel={() => setDeletingId(null)}
                        confirm={(reason) => void run(dm.id, remove(dm.id, reason), CATEGORY_DELETED_TOAST)}
                      />
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        ))
      )}
    </div>
  );
}

/**
 * One row (prototype `CategoryRow`, `CategoryManagerDialog.tsx:240-314`). A component of its own because
 * the name box holds its own text and `PendingMarker` uses hooks.
 *
 * THE RENAME SENDS `{name}` AND NOTHING ELSE, and only when the trimmed name moved: an unchanged or empty box
 * puts the old name back without a call. A refusal puts the old name back too; its sentence is under the row.
 */
function CategoryRow({
  category: dm,
  busy,
  deleting,
  rename,
  toggle,
  askDelete,
}: {
  category: comms_danhMucRa;
  busy: boolean;
  deleting: boolean;
  rename: (name: string) => Promise<boolean>;
  toggle: () => void;
  askDelete: () => void;
}) {
  const [name, setName] = useState(dm.name);

  function saveName(): void {
    const next = name.trim();
    if (next === "" || next === dm.name) {
      setName(dm.name);
      return;
    }
    void rename(next).then((ok) => {
      if (!ok) setName(dm.name);
    });
  }

  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-[10px] border border-solid border-line px-2.5 py-2",
        dm.hidden ? "bg-canvas" : "bg-surface",
      )}
    >
      <input
        id={`ten-danh-muc-${dm.id}`}
        aria-label={`Tên danh mục ${dm.name}`}
        value={name}
        maxLength={TEN_DANH_MUC_TOI_DA}
        autoComplete="off"
        disabled={busy}
        onChange={(e) => setName(e.target.value)}
        onBlur={saveName}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            e.currentTarget.blur();
          }
          if (e.key === "Escape") {
            // Prevented: inside the native `<dialog>`, Esc would otherwise close the whole dialog.
            e.preventDefault();
            setName(dm.name);
          }
        }}
        className={cn(
          "h-8 min-w-0 flex-1 rounded-md border border-solid border-transparent bg-transparent px-1.5 [font-family:inherit] text-[12.5px] text-navy outline-none",
          "hover:border-line focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
          dm.hidden && "text-ink-muted line-through",
        )}
      />

      <ItemCountPending />
      <FromPortalPending />

      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="h-8 shrink-0 px-2 text-[11.5px]"
        disabled={busy}
        aria-label={`${dm.hidden ? CATEGORY_TURN_ON : CATEGORY_TURN_OFF} danh mục ${dm.name}`}
        onClick={toggle}
      >
        {dm.hidden ? CATEGORY_TURN_ON : CATEGORY_TURN_OFF}
      </Button>

      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="h-8 shrink-0 px-2 text-danger"
        icon={<Trash2 aria-hidden="true" focusable="false" className="size-3.5" />}
        aria-expanded={deleting}
        disabled={busy}
        aria-label={`${CATEGORY_DELETE_BUTTON} danh mục ${dm.name}`}
        onClick={askDelete}
      />
    </div>
  );
}

/** `{n} bài` with a "?" where the number would be — the list route carries no item count. */
function ItemCountPending() {
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 text-[11px] text-ink-muted tabular-nums"
      aria-disabled="true"
      data-pending=""
    >
      <PendingMarker info={pendingContentPart(CATEGORY_ITEM_COUNT_PART)} />
      bài
    </span>
  );
}

/** The prototype's `Từ Cổng` badge, greyed, with its "?" — the row carries no source. */
function FromPortalPending() {
  return (
    <span className="inline-flex shrink-0 items-center gap-1" aria-disabled="true" data-pending="">
      <Badge tone="neutral" icon={Link2} className="text-ink-muted opacity-60">
        Từ Cổng
      </Badge>
      <PendingMarker info={pendingContentPart(CATEGORY_FROM_PORTAL_PART)} />
    </span>
  );
}

/**
 * The prototype's box above the add row (`CategoryManagerDialog.tsx:107-125`): its sentence and `Lấy danh mục
 * từ Cổng`, the button drawn disabled with a "?" — no route imports the portal's tree (owner D4).
 * `bg-canvas` is the prototype's `bg-surface` (the page grey; this app's `bg-surface` is white).
 */
export function CategoryImportPending() {
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-[10px] border border-solid border-line bg-canvas p-2.5">
      <p className="m-0 flex-1 text-[11.5px] text-ink-muted">{CATEGORY_IMPORT_NOTE}</p>
      <PendingButton
        info={pendingContentPart(CATEGORY_IMPORT_PART)}
        size="sm"
        icon={<DownloadCloud aria-hidden="true" focusable="false" className="size-4" />}
      />
    </div>
  );
}

/**
 * Delete one category — a soft delete with a MANDATORY reason (rule 7). The button stays off until a
 * reason is typed; the server refuses an empty one anyway. No `Ẩn thay vì xoá` here (the prototype has
 * none): the row's own `Tắt` hides a category.
 */
export function CategoryDeleteForm({
  category,
  busy,
  error,
  cancel,
  confirm,
}: {
  category: comms_danhMucRa;
  busy: boolean;
  error: string | null;
  cancel: () => void;
  confirm: (reason: string) => void;
}) {
  const [reason, setReason] = useState("");
  const fid = `xoa-danh-muc-${category.id}`;
  const trimmed = reason.trim();

  function submit(e: FormEvent) {
    e.preventDefault();
    if (trimmed === "") return;
    confirm(trimmed);
  }

  return (
    <form className={SUB_FORM_CLASS} onSubmit={submit} aria-labelledby={`${fid}-tieu-de`}>
      <h4 id={`${fid}-tieu-de`}>Xoá danh mục: {category.name}</h4>
      <p className="ghi-chu m-0">{CATEGORY_DELETE_NOTE}</p>

      <Field label={CATEGORY_REASON_LABEL} htmlFor={`${fid}-ly-do`} grow="auto" className="[&_textarea]:py-2.5">
        <textarea
          id={`${fid}-ly-do`}
          name={`${fid}-ly-do`}
          rows={3}
          required
          value={reason}
          maxLength={DELETE_REASON_MAX_CHARS}
          onChange={(e) => setReason(e.target.value)}
        />
      </Field>

      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <button type="button" className={cn("nut-phu", SUB_BUTTON, buttonVariants({ variant: "secondary" }))} disabled={busy} onClick={cancel}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className={cn("nut-phu nut-xoa", SUB_BUTTON, buttonVariants({ variant: "danger" }))} disabled={busy || trimmed === ""}>
          <Trash2 aria-hidden="true" focusable="false" />
          Xoá danh mục
        </button>
      </div>
    </form>
  );
}

/**
 * The inline delete form under a row: a muted panel, not the legacy blue-edged box. `h4` restyled from here
 * because `.form-danh-muc h4` sets its own margin and size.
 */
const SUB_FORM_CLASS = cn(
  "form-danh-muc mt-1 mb-0 flex flex-col gap-4 rounded-xl border border-line border-l-line bg-surface-muted p-4",
  "[&_h4]:m-0 [&_h4]:text-sm [&_h4]:font-semibold",
);

/** Native buttons keep `type` first; the legacy 44px / full-width rules of `.form-danh-muc` are lifted. */
const SUB_BUTTON = "w-auto min-h-0";
