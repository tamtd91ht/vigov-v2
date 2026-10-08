"use client";

import { Layers, Loader2, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Skeleton } from "@/components/ui/skeleton";
// vi-name-ok: existing words of the Cấu hình catalogue tab, imported unchanged (rule 12, invariant 3)
import { giaiThichKhongThaoTac } from "@/features/cau-hinh/nhan-danh-muc";
// vi-name-ok: existing tier rules, imported unchanged (rule 12, invariant 3)
import { choBatLai, laMucGhi, thaoTacCuaMuc } from "@/features/cau-hinh/tang-danh-muc";
import { addCapitalPlanCategory, deleteCapitalPlanCategory } from "@/lib/api/capital-plan-categories";
// vi-name-ok: existing write function of danh-muc.ts, imported unchanged (rule 12, invariant 3)
import { CAPITAL_PLAN_CATEGORY_WRITES, suaMuc } from "@/lib/api/danh-muc";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu"; // vi-name-ok: existing read function (rule 12, invariant 3)
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_danhSachHangMucRa, finance_hangMucRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_QUAN_LY_DANH_MUC } from "@/lib/quyen";

/**
 * `☰ Hạng mục` of the Giải ngân header — prototype `CategoryManagerDialog.tsx`, spec 03: list the
 * commune's capital-plan categories, add one, rename in place, turn off / on, soft delete.
 *
 * WHY HERE AND NOT ONLY IN CẤU HÌNH (the prototype's own reason): the person who re-words a category
 * is the accountant building the report, not an administrator. Same routes, same rules — the writes go
 * through `lib/api/danh-muc.ts` (`CAPITAL_PLAN_CATEGORY_WRITES`), the tier rules through
 * `cau-hinh/tang-danh-muc.ts`. Nothing about a category is decided twice.
 *
 * THE KEYS ARE `budget.update` OR `admin.lookup` (`canManageCategories`): the write routes accept
 * either since e9f669f1 (ADR 0077 #1). Hiding is UX — the route checks (rule 5).
 *
 * AS THE PROTOTYPE SINCE 07/10/2026 (backend e9f669f1):
 *   - Add asks the LABEL only; the server derives the code and never reissues a used one (rule 7,
 *     invariant 3). The code is not shown (spec 03 row). Its 409s are re-worded around the label
 *     (`lib/api/capital-plan-categories.ts`), since there is no code box to fix.
 *   - Delete is the inline `Xoá hẳn / Thôi` confirm, with no reason: the route's reason is optional and
 *     the server records its fixed default (rule 7, invariant 1 still holds server-side).
 *
 * OUTCOMES ARE TOASTS (spec 03 §Toast, ADR 0068 lần 6 #4): a success says the spec's sentence; a
 * refusal says THE SERVER'S sentence verbatim — the spec's generic "Không … được hạng mục." would hide
 * why (a 409 about the label, a 403), and its one code-specific line (`system_lookup`) is for a delete
 * this dialog never offers on a shipped row.
 *
 * ONE DIFFERENCE KEPT (ADR 0068 lần 6 #11): delete is offered on tier 1 only, off on tiers 1-2, rename
 * on all — `thaoTacCuaMuc`, the same function Cấu hình draws from. The prototype's "Đi kèm phần mềm"
 * badge marks tiers 2-3, which are never deletable.
 *
 * ROWS IN THE SERVER'S ORDER, NOT SORTED: the server returns them by `order`, and the Cấu hình tab
 * relies on that same order. A turned-off category stays listed — a project filed under it must still
 * find its name.
 */
/**
 * Whether the session's keys open the dialog: `budget.update` OR `admin.lookup`, the two keys the
 * category write routes accept. UX only — fail closed on an empty list (no session = no key).
 */
export function canManageCategories(permissions: readonly string[]): boolean {
  return coQuyen(permissions, QUYEN_GHI_NGAN_SACH) || coQuyen(permissions, QUYEN_QUAN_LY_DANH_MUC);
}

export function CategoryManagerButton({
  canManage,
  onChanged,
}: {
  /** `canManageCategories(...)`. `false` → nothing is drawn (the prototype's `canRecord`). */
  canManage: boolean;
  /** After any successful write: the page re-reads the categories its filter and forms use. */
  onChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  if (!canManage) return null;
  return (
    <>
      <Button
        type="button"
        variant="outline"
        icon={<Layers aria-hidden="true" className="size-4" />}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        Hạng mục
      </Button>
      {open && <CategoryManagerDialog onClose={() => setOpen(false)} onChanged={onChanged} />}
    </>
  );
}

const TITLE_ID = "tieu-de-quan-ly-hang-muc";

type Loaded = { key: number; result: KetQua<finance_danhSachHangMucRa> };

/** Mounted = open (`ModalDialog`). Reads its own list, so it shows turned-off rows as the server has them. */
export function CategoryManagerDialog({ onClose, onChanged }: { onClose: () => void; onChanged: () => void }) {
  const [reads, setReads] = useState(0);
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [busy, setBusy] = useState(false);
  const [adding, setAdding] = useState(false);

  const [label, setLabel] = useState("");
  // MINTED WHEN THE FORM OPENS, REUSED ON RETRY, renewed only after a success: a retry after a network
  // error must carry the SAME key, or it becomes a second category (`danh-muc.ts`, `themMuc`).
  const [addKey, setAddKey] = useState(khoaChongTrungMoi);

  /** Id of the row whose `Xoá hẳn / Thôi` confirm is open. One at a time. */
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    let dropped = false;
    layHangMucKeHoachVon().then((result) => {
      if (!dropped) setLoaded({ key: reads, result });
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  const current = loaded !== null && loaded.key === reads ? loaded.result : null;
  const rows: readonly finance_hangMucRa[] = current !== null && current.ok ? current.duLieu.items : [];

  /**
   * One write at a time; on success re-read here AND tell the page, whose filter holds its own copy.
   * `done` is the spec's success sentence, or `null` where the spec has none (turning on / off).
   */
  function run(write: Promise<KetQua<unknown>>, done: string | null, after?: () => void): void {
    setBusy(true);
    void write.then((kq) => {
      setBusy(false);
      setAdding(false);
      if (!kq.ok) {
        toast.error(kq.thongBao);
        return;
      }
      after?.();
      if (done !== null) toast.success(done);
      setReads((n) => n + 1);
      onChanged();
    });
  }

  function add(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    if (busy || label.trim() === "") return;
    // Last in the list, as the prototype does: the commune's familiar order (I…VI) is not broken by a
    // new row landing in the middle.
    const order = rows.reduce((max, r) => Math.max(max, r.order), 0) + 1;
    setAdding(true);
    run(addCapitalPlanCategory(label.trim(), order, addKey), "Đã thêm hạng mục.", () => {
      setLabel("");
      setAddKey(khoaChongTrungMoi());
    });
  }

  function rename(row: finance_hangMucRa, next: string): void {
    run(suaMuc(CAPITAL_PLAN_CATEGORY_WRITES, row.id, { label: next }), "Đã đổi tên hạng mục.");
  }

  function setActive(row: finance_hangMucRa, active: boolean): void {
    run(suaMuc(CAPITAL_PLAN_CATEGORY_WRITES, row.id, { active }), null);
  }

  function confirmDelete(row: finance_hangMucRa): void {
    if (busy) return;
    run(deleteCapitalPlanCategory(row.id), "Đã xoá hạng mục.", () => setDeletingId(null));
  }

  return (
    <ModalDialog
      titleId={TITLE_ID}
      className="max-w-[44rem]"
      // The prototype opens with the cursor in `Thêm hạng mục` (brief §3.3).
      initialFocusId="ten-hang-muc-moi"
      onDismiss={() => !busy && onClose()}
    >
      <ModalDialogHeader
        titleId={TITLE_ID}
        title="Hạng mục kế hoạch vốn"
        description="Báo cáo tiến độ cộng dồn theo hạng mục. Mỗi dự án thuộc đúng một hạng mục ở đây."
      />

      <form className="flex min-w-0 shrink-0 items-end gap-2" aria-label="Thêm hạng mục" onSubmit={add}>
        <div className="min-w-0 flex-1">
          <label htmlFor="ten-hang-muc-moi" className="text-navy block text-[11.5px] font-semibold">
            Thêm hạng mục
          </label>
          <input
            id="ten-hang-muc-moi"
            value={label}
            autoComplete="off"
            placeholder="Vốn sự nghiệp có tính chất đầu tư"
            disabled={busy}
            className={cn(controlClass, "mt-1 h-9 bg-white text-[12.5px] md:text-[12.5px]")}
            onChange={(e) => setLabel(e.target.value)}
          />
        </div>
        <Button
          type="submit"
          variant="primary"
          icon={adding ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : <Plus aria-hidden="true" className="size-4" />}
          disabled={busy || label.trim() === ""}
          aria-busy={adding || undefined}
        >
          Thêm
        </Button>
      </form>

      <div className="max-h-[26rem] min-h-0 overflow-y-auto pr-1">
        {current === null && (
          <>
            <p role="status" className="an-thi-giac">
              Đang tải danh mục hạng mục…
            </p>
            <Skeleton className="h-40 w-full" />
          </>
        )}
        {current !== null && !current.ok && (
          <ErrorState
            title="Chưa tải được danh mục hạng mục"
            message={<span role="alert">{current.thongBao}</span>}
            onRetry={() => setReads((n) => n + 1)}
          />
        )}
        {current !== null && current.ok && rows.length === 0 && (
          <p className="text-ink-muted m-0 py-6 text-center text-[12.5px]">Chưa có hạng mục nào.</p>
        )}
        {rows.length > 0 && (
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" aria-label="Các hạng mục kế hoạch vốn">
            {rows.map((row) => (
              <CategoryRow
                // Keyed by the label too: after a re-read the draft starts from the server's value.
                key={`${row.id}|${row.label}`}
                row={row}
                busy={busy}
                confirming={deletingId === row.id}
                onRename={(next) => rename(row, next)}
                onSetActive={(active) => setActive(row, active)}
                onAskDelete={() => setDeletingId(row.id)}
                onConfirmDelete={() => confirmDelete(row)}
                onCancelDelete={() => setDeletingId(null)}
              />
            ))}
          </ul>
        )}
      </div>
    </ModalDialog>
  );
}

/**
 * One category (spec 03 "Một dòng hạng mục"): label edited in place, saved on leaving the box (Enter
 * leaves it, Esc reverts) — the prototype's reason: ten rows each with its own Save button make the eye
 * hunt for the right one. A turned-off row is grey and struck through; that, and its `Bật` button, say
 * it is off — no separate badge.
 */
function CategoryRow({
  row,
  busy,
  confirming,
  onRename,
  onSetActive,
  onAskDelete,
  onConfirmDelete,
  onCancelDelete,
}: {
  row: finance_hangMucRa;
  busy: boolean;
  /** The `Xoá hẳn / Thôi` confirm is open on this row. */
  confirming: boolean;
  onRename: (next: string) => void;
  onSetActive: (active: boolean) => void;
  onAskDelete: () => void;
  onConfirmDelete: () => void;
  onCancelDelete: () => void;
}) {
  const [draft, setDraft] = useState(row.label);
  // A row whose tier the contract does not state gets NO write control (closed when unsure).
  const allowed = thaoTacCuaMuc(row);
  const canReactivate = choBatLai(row);
  const writable = laMucGhi(row);
  const inputId = `nhan-hang-muc-${row.id}`;
  const smallButton = "h-8 shrink-0 px-2 text-[11.5px]";

  function save(): void {
    const next = draft.trim();
    if (next === "" || next === row.label) {
      setDraft(row.label);
      return;
    }
    onRename(next);
  }

  return (
    <li
      data-category-row={row.code}
      data-active={row.active ? "" : undefined}
      className={cn(
        "border-line flex min-w-0 items-center gap-2 rounded-[10px] border border-solid px-2.5 py-2",
        row.active ? "bg-white" : "bg-canvas",
      )}
    >
      <label htmlFor={inputId} className="an-thi-giac">
        Tên hạng mục {row.label}
      </label>
      <input
        id={inputId}
        value={draft}
        readOnly={!allowed.doiNhan}
        disabled={busy}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => allowed.doiNhan && save()}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
          if (e.key === "Escape") {
            // The dialog's own Esc would close it with the draft still typed; here Esc means "undo".
            e.preventDefault();
            e.stopPropagation();
            setDraft(row.label);
          }
        }}
        className={cn(
          "text-ink h-8 min-w-0 flex-1 rounded-md border border-solid border-transparent bg-transparent px-1.5 [font-family:inherit] text-[12.5px] shadow-none outline-none",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
          !row.active && "text-ink-muted line-through",
        )}
      />
      {writable && row.tier !== 1 && (
        <Badge tone="neutral" className="bg-canvas text-ink-muted" title={giaiThichKhongThaoTac(row.tier)}>
          Đi kèm phần mềm
        </Badge>
      )}

      {allowed.tat && row.active && (
        <Button
          type="button"
          size="sm"
          variant="outline"
          className={smallButton}
          aria-label={`Tắt hạng mục ${row.label}`}
          disabled={busy}
          onClick={() => onSetActive(false)}
        >
          Tắt
        </Button>
      )}
      {canReactivate && (
        <Button
          type="button"
          size="sm"
          variant="outline"
          className={smallButton}
          aria-label={`Bật hạng mục ${row.label}`}
          disabled={busy}
          onClick={() => onSetActive(true)}
        >
          Bật
        </Button>
      )}
      {/* The prototype's inline confirm (`CategoryManagerDialog.tsx:221-253`): the trash icon turns
          into `Xoá hẳn / Thôi` in place — no reason, no second form. */}
      {allowed.xoa &&
        (confirming ? (
          <span className="flex shrink-0 gap-1.5">
            <Button
              type="button"
              size="sm"
              variant="primary"
              className={cn(smallButton, "bg-danger hover:not-disabled:bg-danger/90 text-white")}
              disabled={busy}
              aria-busy={busy || undefined}
              onClick={onConfirmDelete}
            >
              Xoá hẳn
            </Button>
            <Button type="button" size="sm" variant="outline" className={smallButton} disabled={busy} onClick={onCancelDelete}>
              Thôi
            </Button>
          </span>
        ) : (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="text-danger hover:not-disabled:text-danger h-8 shrink-0 px-2"
            aria-label={`Xoá hạng mục ${row.label}`}
            title={`Xoá hạng mục ${row.label}`}
            disabled={busy}
            onClick={onAskDelete}
          >
            <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
        ))}
    </li>
  );
}
