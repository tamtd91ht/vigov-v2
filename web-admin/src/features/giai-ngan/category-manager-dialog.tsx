"use client";

import { Layers, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { SkeletonRows } from "@/components/ui/skeleton";
// vi-name-ok: existing words and tier rules of the Cấu hình catalogue tab, imported unchanged (rule 12, invariant 3)
import { CANH_BAO_XOA, GIAI_THICH_O_MA, giaiThichKhongThaoTac } from "@/features/cau-hinh/nhan-danh-muc";
// vi-name-ok: existing tier rules, imported unchanged (rule 12, invariant 3)
import { choBatLai, kiemLyDoXoa, laMucGhi, thaoTacCuaMuc } from "@/features/cau-hinh/tang-danh-muc";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
// vi-name-ok: existing write functions of danh-muc.ts, imported unchanged (rule 12, invariant 3)
import { CAPITAL_PLAN_CATEGORY_WRITES, suaMuc, themMuc, xoaMuc } from "@/lib/api/danh-muc";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu"; // vi-name-ok: existing read function (rule 12, invariant 3)
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_danhSachHangMucRa, finance_hangMucRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

/**
 * `☰ Hạng mục` of the Giải ngân header — prototype `CategoryManagerDialog.tsx:56-173`: list the
 * commune's capital-plan categories, add one, rename in place, turn off / on, soft delete.
 *
 * WHY HERE AND NOT ONLY IN CẤU HÌNH (the prototype's own reason): the person who re-words a category
 * is the accountant building the report, not an administrator. Same routes, same rules — the writes go
 * through `lib/api/danh-muc.ts` (`CAPITAL_PLAN_CATEGORY_WRITES`), the tier rules through
 * `cau-hinh/tang-danh-muc.ts`. Nothing about a category is decided twice.
 *
 * THE KEY IS `budget.update` (user decision 06/10/2026): the button shows only with it. The category
 * write routes say `admin.lookup` until the server card that accepts either key lands; until then a
 * refusal is the server's sentence, shown verbatim. Hiding is UX — the route checks (rule 5).
 *
 * THREE DIFFERENCES FROM THE PROTOTYPE, each forced by the contract:
 *   - `Mã` is asked on add: the route requires `code` and the server never generates one; a code typed
 *     by the browser from the label would be an ISSUED code nobody chose (rule 7, invariant 3).
 *   - Delete asks a reason: the category DELETE route requires it (`finance_xoaHangMucVao`). The
 *     06/10 "no reason" decision is about PROJECTS only.
 *   - Delete is offered on tier 1 only, off on tiers 1-2, rename on all — `thaoTacCuaMuc`, the same
 *     function Cấu hình draws from. The prototype's "Đi kèm phần mềm" badge marks tiers 2-3.
 *
 * ROWS IN THE SERVER'S ORDER, NOT SORTED: the server returns them by `order`, and the Cấu hình tab
 * relies on that same order. A turned-off category stays listed — a project filed under it must still
 * find its name.
 */
export function CategoryManagerButton({
  canManage,
  onChanged,
}: {
  /** `budget.update` held. `false` → nothing is drawn (the prototype's `canRecord`). */
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
        variant="secondary"
        icon={<Layers aria-hidden="true" />}
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
  const [serverError, setServerError] = useState("");
  const [done, setDone] = useState("");

  const [code, setCode] = useState("");
  const [label, setLabel] = useState("");
  // MINTED WHEN THE FORM OPENS, REUSED ON RETRY, renewed only after a success: a retry after a network
  // error must carry the SAME key, or it becomes a second category (`danh-muc.ts`, `themMuc`).
  const [addKey, setAddKey] = useState(khoaChongTrungMoi);

  /** The row whose delete confirmation is open, with its reason as typed. */
  const [deleting, setDeleting] = useState<{ id: string; reason: string; localError: string } | null>(null);

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

  /** One write at a time; on success re-read here AND tell the page, whose filter holds its own copy. */
  function run(write: Promise<KetQua<unknown>>, sentence: string, after?: () => void): void {
    setBusy(true);
    setServerError("");
    setDone("");
    void write.then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setServerError(kq.thongBao);
        return;
      }
      after?.();
      setDone(sentence);
      setReads((n) => n + 1);
      onChanged();
    });
  }

  function add(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    if (busy || code.trim() === "" || label.trim() === "") return;
    // Last in the list, as the prototype does: the commune's familiar order (I…VI) is not broken by a
    // new row landing in the middle.
    const order = rows.reduce((max, r) => Math.max(max, r.order), 0) + 1;
    run(
      themMuc(CAPITAL_PLAN_CATEGORY_WRITES, { code: code.trim(), label: label.trim(), order }, addKey),
      `Đã thêm hạng mục “${label.trim()}”.`,
      () => {
        setCode("");
        setLabel("");
        setAddKey(khoaChongTrungMoi());
      },
    );
  }

  function rename(row: finance_hangMucRa, next: string): void {
    run(suaMuc(CAPITAL_PLAN_CATEGORY_WRITES, row.id, { label: next }), `Đã đổi tên hạng mục thành “${next}”.`);
  }

  function setActive(row: finance_hangMucRa, active: boolean): void {
    run(
      suaMuc(CAPITAL_PLAN_CATEGORY_WRITES, row.id, { active }),
      active ? `Đã bật lại hạng mục “${row.label}”.` : `Đã tắt hạng mục “${row.label}”.`,
    );
  }

  function confirmDelete(row: finance_hangMucRa): void {
    if (deleting === null || busy) return;
    const check = kiemLyDoXoa(deleting.reason);
    if (!check.ok) {
      setDeleting({ ...deleting, localError: check.loi });
      return;
    }
    run(xoaMuc(CAPITAL_PLAN_CATEGORY_WRITES, row.id, check.giaTri), `Đã xoá hạng mục “${row.label}”.`, () =>
      setDeleting(null),
    );
  }

  return (
    <ModalDialog titleId={TITLE_ID} size="lg" onDismiss={() => !busy && onClose()}>
      <ModalDialogHeader
        titleId={TITLE_ID}
        title="Hạng mục kế hoạch vốn"
        description="Báo cáo tiến độ cộng dồn theo hạng mục. Mỗi dự án thuộc đúng một hạng mục ở đây."
      />

      <form className="flex min-w-0 shrink-0 flex-wrap items-end gap-2" aria-label="Thêm hạng mục" onSubmit={add}>
        <Field label="Mã" htmlFor="ma-hang-muc-moi" grow="auto" className="w-40 max-w-full" hint={GIAI_THICH_O_MA}>
          <input
            id="ma-hang-muc-moi"
            value={code}
            autoComplete="off"
            placeholder="von-su-nghiep"
            disabled={busy}
            onChange={(e) => setCode(e.target.value)}
          />
        </Field>
        <Field label="Thêm hạng mục" htmlFor="ten-hang-muc-moi" grow="auto" className="min-w-0 flex-1 basis-56">
          <input
            id="ten-hang-muc-moi"
            value={label}
            autoComplete="off"
            placeholder="Vốn sự nghiệp có tính chất đầu tư"
            disabled={busy}
            onChange={(e) => setLabel(e.target.value)}
          />
        </Field>
        <Button
          type="submit"
          variant="primary"
          icon={<Plus aria-hidden="true" />}
          disabled={busy || code.trim() === "" || label.trim() === ""}
        >
          Thêm
        </Button>
      </form>

      {/* Two live lines, always in the DOM: one for what was done, one for the server's refusal. */}
      <p role="status" className="m-0 text-sm font-medium text-success-600 empty:hidden">
        {done}
      </p>
      {serverError !== "" && (
        <p role="alert" className="thong-bao-loi m-0">
          {serverError}
        </p>
      )}

      <div className="min-h-0 overflow-y-auto pr-1">
        {current === null && (
          <>
            <p role="status" className="an-thi-giac">
              Đang tải danh mục hạng mục…
            </p>
            <SkeletonRows rows={4} />
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
          <p className="m-0 py-6 text-center text-sm text-ink-500">Chưa có hạng mục nào.</p>
        )}
        {rows.length > 0 && (
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" aria-label="Các hạng mục kế hoạch vốn">
            {rows.map((row) => (
              <CategoryRow
                // Keyed by the label too: after a re-read the draft starts from the server's value.
                key={`${row.id}|${row.label}`}
                row={row}
                busy={busy}
                deleting={deleting !== null && deleting.id === row.id ? deleting : null}
                onRename={(next) => rename(row, next)}
                onSetActive={(active) => setActive(row, active)}
                onAskDelete={() => {
                  setServerError("");
                  setDeleting({ id: row.id, reason: "", localError: "" });
                }}
                onReasonChange={(reason) => deleting !== null && setDeleting({ ...deleting, reason, localError: "" })}
                onConfirmDelete={() => confirmDelete(row)}
                onCancelDelete={() => setDeleting(null)}
              />
            ))}
          </ul>
        )}
      </div>

      <div className="flex shrink-0 justify-end">
        <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>
          Đóng
        </Button>
      </div>
    </ModalDialog>
  );
}

/**
 * One category: label edited in place, saved on leaving the box (Enter leaves it, Esc reverts) — the
 * prototype's reason: ten rows each with its own Save button make the eye hunt for the right one.
 */
function CategoryRow({
  row,
  busy,
  deleting,
  onRename,
  onSetActive,
  onAskDelete,
  onReasonChange,
  onConfirmDelete,
  onCancelDelete,
}: {
  row: finance_hangMucRa;
  busy: boolean;
  deleting: { reason: string; localError: string } | null;
  onRename: (next: string) => void;
  onSetActive: (active: boolean) => void;
  onAskDelete: () => void;
  onReasonChange: (reason: string) => void;
  onConfirmDelete: () => void;
  onCancelDelete: () => void;
}) {
  const [draft, setDraft] = useState(row.label);
  // A row whose tier the contract does not state gets NO write control (closed when unsure).
  const allowed = thaoTacCuaMuc(row);
  const canReactivate = choBatLai(row);
  const writable = laMucGhi(row);
  const inputId = `nhan-hang-muc-${row.id}`;
  const reasonId = `ly-do-xoa-hang-muc-${row.id}`;

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
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-control border border-solid border-line px-2.5 py-2",
        row.active ? "bg-surface" : "bg-surface-subtle",
      )}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <label htmlFor={inputId} className="an-thi-giac">
          Tên hạng mục {row.code}
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
            "h-8 min-w-40 flex-1 rounded-control border border-solid border-transparent bg-transparent px-1.5 text-sm",
            "focus:border-line",
            !row.active && "text-ink-500 line-through",
          )}
        />
        <span className="ma-muc shrink-0 text-xs text-ink-500">{row.code}</span>
        {writable && row.tier !== 1 && (
          <Badge tone="neutral" title={giaiThichKhongThaoTac(row.tier)}>
            Đi kèm phần mềm
          </Badge>
        )}
        {!row.active && <Badge tone="neutral">Đã tắt</Badge>}

        {allowed.tat && row.active && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
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
            variant="secondary"
            aria-label={`Bật lại hạng mục ${row.label}`}
            disabled={busy}
            onClick={() => onSetActive(true)}
          >
            Bật lại
          </Button>
        )}
        {allowed.xoa && deleting === null && (
          <IconButton
            type="button"
            variant="secondary"
            className="text-danger-600 hover:not-disabled:border-danger-600 hover:not-disabled:text-danger-600"
            label={`Xoá hạng mục ${row.label}`}
            disabled={busy}
            onClick={onAskDelete}
          >
            <Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />
          </IconButton>
        )}
      </div>

      {deleting !== null && (
        <form
          className="flex min-w-0 flex-col gap-2 border-t border-solid border-line pt-2"
          aria-label={`Xoá hạng mục ${row.label}`}
          onSubmit={(e) => {
            e.preventDefault();
            onConfirmDelete();
          }}
        >
          <p className="m-0 text-[13px] text-ink-700">{CANH_BAO_XOA}</p>
          <Field label="Lý do xoá *" htmlFor={reasonId} grow="auto">
            <input
              id={reasonId}
              value={deleting.reason}
              autoComplete="off"
              disabled={busy}
              aria-invalid={deleting.localError !== ""}
              onChange={(e) => onReasonChange(e.target.value)}
            />
          </Field>
          {deleting.localError !== "" && (
            <p role="alert" className="thong-bao-loi m-0">
              {deleting.localError}
            </p>
          )}
          <div className="flex flex-wrap justify-end gap-2">
            <Button type="submit" size="sm" variant="danger" disabled={busy} aria-busy={busy || undefined}>
              <BusyLabel busy={busy} label="Xoá hạng mục" busyText={BUSY_DELETING} />
            </Button>
            <Button type="button" size="sm" variant="secondary" disabled={busy} onClick={onCancelDelete}>
              Huỷ
            </Button>
          </div>
        </form>
      )}
    </li>
  );
}
