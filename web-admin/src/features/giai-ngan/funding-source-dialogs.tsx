"use client";

import { Pencil } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass, Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import {
  createFundingSource,
  listFundingSourceProjects,
  setGrantedAmount,
} from "@/lib/api/funding-sources";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type {
  finance_fundingSourceOut,
  finance_fundingSourceProjectsOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

// vi-name-ok: existing formatters of nhan-du-an.ts, imported unchanged (rule 12, invariant 3)
import { nhanTien, shortDongLabel } from "./nhan-du-an";
// vi-name-ok: existing amount parser and its two sentences (rule 12, invariant 3)
import {
  CAU_SO_TIEN_KHONG_DOC_DUOC,
  CAU_SO_TIEN_VUOT_CHINH_XAC,
  docSoTien,
} from "./nhan-ghi-giai-ngan";
import { PROGRESS_TEXT_CLASS, progressTone } from "./progress-tone";
import { Glyph } from "./project-ui";
import { SUB_TABLE_HEAD_ROW_CLASS } from "./spec-classes";

/**
 * The two dialogs of §6 (spec 06 §B, §C): the projects behind one card (prototype
 * `SourceItemsDialog.tsx`) and `Thêm nguồn vốn` (prototype `BudgetSourceForm.tsx`). Field names in the
 * prototype are inverted against the contract — see `funding-source-progress.tsx`.
 *
 * NO DELETE AND NO RENAME (decided 06/10/2026): a source is declared once and serves every year, and
 * its name is what archived reports cite. There is no route for either, so no control either.
 *
 * OUTCOMES ARE TOASTS (ADR 0068 lần 6 #4): a success says the spec's sentence; a refusal says the
 * SERVER's sentence verbatim (409 name taken, catalogue full, 403…) — the spec's generic "Không thêm
 * được nguồn vốn. Vui lòng thử lại." would hide why. A missing or unreadable field stays inline.
 */

/* ── Amount box ──────────────────────────────────────────────────────────────────────────────── */

/** Spec 06 §B's sentence for a missing name, shown under the field. */
export const SOURCE_NAME_MISSING = "Vui lòng nhập tên nguồn vốn.";
export const GRANTED_AMOUNT_MISSING =
  "Chưa có số vốn được giao. Năm này nguồn không giao vốn thì nhập 0.";

/**
 * A typed amount → đồng, through the screen's one parser (`docSoTien`). `blank` is returned as such:
 * the ADD form sends nothing for it ("not entered"), the EDIT form refuses it (the route requires a
 * figure, and a blank read as 0 would record "granted nothing" that nobody said).
 */
type AmountRead =
  | { kind: "blank" }
  | { kind: "amount"; dong: number }
  | { kind: "invalid"; sentence: string };

function readAmount(typed: string): AmountRead {
  const read = docSoTien(typed);
  switch (read.loai) {
    case "trong":
      return { kind: "blank" };
    case "so":
      return { kind: "amount", dong: read.dong };
    case "vuotChinhXac":
      return { kind: "invalid", sentence: CAU_SO_TIEN_VUOT_CHINH_XAC };
    default:
      return { kind: "invalid", sentence: CAU_SO_TIEN_KHONG_DOC_DUOC };
  }
}

/** The formatted figure under a money box, as the project form does (`ghi-du-an.tsx`). */
function amountHint(typed: string): string | undefined {
  const read = docSoTien(typed);
  return read.loai === "so" ? nhanTien(read.dong) : undefined;
}

/* ── Projects behind one card ────────────────────────────────────────────────────────────────── */

const PROJECTS_TITLE_ID = "tieu-de-du-an-theo-nguon-von";

/** Spec 06 §C ratio: one decimal at most. */
const RATIO_ONE_DECIMAL = new Intl.NumberFormat("vi-VN", {
  maximumFractionDigits: 1,
});

/**
 * Which projects make up a card's "đã phân bổ". Each row is THIS SOURCE'S share of a project, not the
 * project's whole plan — the rows add up to the card (plus the server's "paid without allocation"
 * remainder, said under the table when it is not 0).
 *
 * Spec 06 §C chrome: 52rem wide, 85vh tall, no footer button — the dialog's own ✕ closes it.
 */
export function FundingSourceProjectsDialog({
  source,
  year,
  onClose,
}: {
  source: finance_fundingSourceOut;
  year: number;
  onClose: () => void;
}) {
  const [reloads, setReloads] = useState(0);
  const key = `${source.id}|${year}|${reloads}`;
  const [loaded, setLoaded] = useState<{
    key: string;
    result: KetQua<finance_fundingSourceProjectsOut>;
  } | null>(null);

  useEffect(() => {
    let dropped = false;
    listFundingSourceProjects(source.id, year).then((result) => {
      if (!dropped) setLoaded({ key, result });
    });
    return () => {
      dropped = true;
    };
  }, [source.id, year, key]);

  const current = loaded !== null && loaded.key === key ? loaded.result : null;

  return (
    <ModalDialog
      titleId={PROJECTS_TITLE_ID}
      className="max-h-[85vh] max-w-[52rem]"
      onDismiss={onClose}
    >
      <ModalDialogHeader
        titleId={PROJECTS_TITLE_ID}
        title={source.name}
        description={`Đã phân bổ ${nhanTien(source.allocated_amount)} cho ${source.project_count} dự án trong năm ${year}.`}
      />
      <div className="min-h-0 overflow-y-auto">
        {current === null && (
          <>
            <p role="status" className="an-thi-giac">
              Đang tải danh sách dự án…
            </p>
            <Skeleton className="h-40 w-full" />
          </>
        )}
        {current !== null && !current.ok && (
          <ErrorState
            title="Chưa tải được danh sách dự án của nguồn vốn này"
            message={<span role="alert">{current.thongBao}</span>}
            onRetry={() => setReloads((n) => n + 1)}
          />
        )}
        {current !== null && current.ok && (
          <SourceProjectsTable data={current.duLieu} onNavigate={onClose} />
        )}
      </div>
    </ModalDialog>
  );
}

/** Money IN FULL ĐỒNG in a table: a rounded figure here is copied straight into a report (`bang-du-an.tsx`). */
function SourceProjectsTable({
  data,
  onNavigate,
}: {
  data: finance_fundingSourceProjectsOut;
  onNavigate: () => void;
}) {
  const th = "py-2 pr-3 font-semibold";
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {data.items.length === 0 ? (
        <p className="text-ink-muted m-0 py-8 text-center text-[12.5px]">
          Chưa có dự án nào lấy vốn từ nguồn này.
        </p>
      ) : (
        <table
          className="w-full border-collapse text-[12.5px]"
          aria-label={`Dự án lấy vốn từ ${data.name} năm ${data.year}`}
        >
          <thead>
            <tr className={SUB_TABLE_HEAD_ROW_CLASS}>
              <th scope="col" className={cn(th, "text-left")}>
                Dự án
              </th>
              <th scope="col" className={cn(th, "text-right")}>
                Phân bổ từ nguồn này
              </th>
              <th scope="col" className={cn(th, "text-right")}>
                Đã chi
              </th>
              <th scope="col" className="py-2 text-right font-semibold">
                Tỷ lệ
              </th>
            </tr>
          </thead>
          <tbody>
            {data.items.map((p) => (
              <tr
                key={p.id}
                className="border-line border-b last:border-b-0"
              >
                <td className="py-2.5 pr-3">
                  <Link
                    href={`/giai-ngan/du-an/${encodeURIComponent(p.id)}`}
                    onClick={onNavigate}
                    className="text-navy no-underline hover:underline"
                  >
                    {p.name}
                  </Link>
                  <span className="ma-muc text-ink-muted block [font-family:inherit] text-[11px]">
                    {p.code}
                  </span>
                </td>
                <td className="text-navy py-2.5 pr-3 text-right font-semibold whitespace-nowrap tabular-nums">
                  {nhanTien(p.allocated_amount)}
                </td>
                <td className="py-2.5 pr-3 text-right whitespace-nowrap tabular-nums">
                  {nhanTien(p.disbursed_amount)}
                </td>
                {/* `null` = nothing allocated from this source: no denominator, never "0%". */}
                {p.disbursed_ratio === null ? (
                  <td className="py-2.5 text-right font-semibold tabular-nums">
                    —
                  </td>
                ) : (
                  <td
                    className={cn(
                      "py-2.5 text-right font-semibold whitespace-nowrap tabular-nums",
                      PROGRESS_TEXT_CLASS[progressTone(p.disbursed_ratio)],
                    )}
                  >
                    {RATIO_ONE_DECIMAL.format(p.disbursed_ratio / 100)}%
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {/* Not in the prototype: without it the rows visibly fail to add up to the card (§13 rule 6). */}
      {data.disbursed_without_allocation_amount > 0 && (
        <p className="text-tangerine m-0 text-[12px]">
          Ngoài các dòng trên, còn{" "}
          {nhanTien(data.disbursed_without_allocation_amount)} đã chi từ nguồn
          này ở những dự án chưa khai phân bổ từ nguồn này.
        </p>
      )}
    </div>
  );
}

/* ── Thêm nguồn vốn ──────────────────────────────────────────────────────────────────────────── */

const MANAGE_TITLE_ID = "tieu-de-quan-ly-nguon-von";
const ADD_FORM_ID = "form-them-nguon-von";

/**
 * Spec 06 §B "Thêm nguồn vốn": the name, this year's granted amount, the commune's existing sources
 * (with an in-place `Sửa` of the year's granted amount), and ONE footer `[Đóng] [Thêm nguồn vốn]`. A
 * successful add says so, resets and closes, as the spec does.
 *
 * THE DESCRIPTION IS THE SPEC'S, MADE TRUE: the spec says "mỗi dự án thuộc về một nguồn"; our projects
 * draw on one OR SEVERAL sources (ADR 0075 #2a), so the sentence says that.
 *
 * AFTER EVERY WRITE THE CALLER RE-READS (`onChanged`), never patches: the write replies carry no
 * derived figure worth trusting next to the cards. `sources` is `null` while that re-read runs.
 */
export function ManageFundingSourcesDialog({
  year,
  sources,
  onChanged,
  onClose,
}: {
  year: number;
  sources: readonly finance_fundingSourceOut[] | null;
  onChanged: () => void;
  onClose: () => void;
}) {
  return (
    <ModalDialog
      titleId={MANAGE_TITLE_ID}
      onDismiss={onClose}
      className="max-w-[38rem]"
    >
      <ModalDialogHeader
        titleId={MANAGE_TITLE_ID}
        title="Thêm nguồn vốn"
        description="Nguồn vốn là gốc của cây theo dõi: mỗi dự án lấy vốn từ một hoặc nhiều nguồn, và báo cáo cộng ngược lên theo nguồn."
      />
      <AddFundingSourceForm
        year={year}
        onAdded={() => {
          onChanged();
          onClose();
        }}
        onClose={onClose}
        existing={
          <SourceList year={year} sources={sources} onSaved={onChanged} />
        }
      />
    </ModalDialog>
  );
}

function SourceList({
  year,
  sources,
  onSaved,
}: {
  year: number;
  sources: readonly finance_fundingSourceOut[] | null;
  onSaved: () => void;
}) {
  if (sources === null) {
    return (
      <p role="status" className="text-ink-muted m-0 text-[12px]">
        Đang tải danh sách nguồn vốn…
      </p>
    );
  }
  if (sources.length === 0) return null;
  return (
    <section
      className="border-line bg-canvas rounded-[10px] border border-solid p-3"
      aria-labelledby="tieu-de-nguon-von-dang-co"
    >
      <h3
        id="tieu-de-nguon-von-dang-co"
        className="text-ink-muted m-0 mb-2 text-[11.5px] font-semibold"
      >
        Xã đang có {sources.length} nguồn vốn
      </h3>
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        {sources.map((s) => (
          <SourceRow key={s.id} source={s} year={year} onSaved={onSaved} />
        ))}
      </ul>
    </section>
  );
}

/** One source: its figures for the year, and `Sửa` → the year's granted amount in place (PUT). */
function SourceRow({
  source,
  year,
  onSaved,
}: {
  source: finance_fundingSourceOut;
  year: number;
  onSaved: () => void;
}) {
  const [editing, setEditing] = useState(false);
  // RAW digits, never the formatted figure: what the box holds must be exactly what the server holds.
  const [typed, setTyped] = useState(String(source.granted_amount));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputId = `von-duoc-giao-${source.id}`;

  function save(e: FormEvent) {
    e.preventDefault();
    const read = readAmount(typed);
    if (read.kind === "blank") return setError(GRANTED_AMOUNT_MISSING);
    if (read.kind === "invalid") return setError(read.sentence);
    // Unchanged = no write: a PUT that changes nothing would still leave an audit entry.
    if (read.dong === source.granted_amount) {
      setEditing(false);
      setError(null);
      return;
    }
    setBusy(true);
    setGrantedAmount(source.id, year, read.dong).then((result) => {
      setBusy(false);
      if (!result.ok) {
        toast.error(result.thongBao);
        return;
      }
      toast.success("Đã cập nhật vốn được giao.");
      setEditing(false);
      setError(null);
      onSaved();
    });
  }

  return (
    <li className="text-[12px]">
      <div className="flex items-baseline gap-2">
        <span className="text-navy min-w-0 font-semibold">{source.name}</span>
        {!editing && (
          <button
            type="button"
            onClick={() => {
              setTyped(String(source.granted_amount));
              setEditing(true);
            }}
            aria-label={`Sửa vốn được giao năm ${year} của ${source.name}`}
            className="text-brand flex shrink-0 cursor-pointer items-center gap-1 border-0 bg-transparent p-0 [font-family:inherit] text-[11px] hover:underline"
          >
            <Glyph icon={Pencil} className="size-3" />
            Sửa
          </button>
        )}
      </div>

      {editing ? (
        <form
          onSubmit={save}
          className="mt-1.5 flex min-w-0 flex-col gap-1"
          aria-label={`Vốn được giao năm ${year} của ${source.name}`}
        >
          <div className="flex min-w-0 items-center gap-2">
            <label htmlFor={inputId} className="an-thi-giac">
              Vốn được giao năm {year} (đồng)
            </label>
            <input
              id={inputId}
              value={typed}
              inputMode="numeric"
              autoComplete="off"
              className={cn(
                controlClass,
                "h-8 min-w-0 flex-1 bg-white text-[12px] tabular-nums md:text-[12px]",
              )}
              onChange={(e) => setTyped(e.target.value)}
            />
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={busy}
              aria-busy={busy || undefined}
            >
              Lưu
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => {
                setEditing(false);
                setError(null);
              }}
            >
              Huỷ
            </Button>
          </div>
          {amountHint(typed) !== undefined && error === null && (
            <p className="text-ink-muted m-0 text-[11px] tabular-nums">
              {amountHint(typed)}
            </p>
          )}
          {error !== null && (
            <p className="text-danger m-0 text-[12px] font-medium" role="alert">
              {error}
            </p>
          )}
        </form>
      ) : (
        <p className="text-ink-muted m-0 mt-0.5">
          Được giao{" "}
          <span title={nhanTien(source.granted_amount)}>
            {shortDongLabel(source.granted_amount)}
          </span>{" "}
          · đã phân bổ{" "}
          <span title={nhanTien(source.allocated_amount)}>
            {shortDongLabel(source.allocated_amount)}
          </span>{" "}
          ·{" "}
          {source.overallocated_amount > 0 ? (
            <span className="text-danger font-semibold">
              vượt nguồn {shortDongLabel(source.overallocated_amount)}
            </span>
          ) : (
            <span className="text-tangerine font-semibold">
              còn {shortDongLabel(source.unallocated_amount)}
            </span>
          )}{" "}
          · {source.project_count} dự án
        </p>
      )}
    </li>
  );
}

/**
 * Name + this year's granted amount (optional), the existing list between the fields and the footer,
 * as spec 06 §B lays it out. Blank amount = NOT SENT ("not entered yet"), which the server records
 * differently from 0 ("granted nothing").
 *
 * THE IDEMPOTENCY KEY is made when the form opens and kept across a failed retry — the first POST may
 * have reached the server. The dialog closes on success, so the next opening makes a new one.
 */
function AddFundingSourceForm({
  year,
  onAdded,
  onClose,
  existing,
}: {
  year: number;
  onAdded: () => void;
  onClose: () => void;
  existing: ReactNode;
}) {
  const [name, setName] = useState("");
  const [amount, setAmount] = useState("");
  const [idempotencyKey] = useState(khoaChongTrungMoi);
  const [busy, setBusy] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [amountError, setAmountError] = useState<string | null>(null);

  function submit(e: FormEvent) {
    e.preventDefault();
    const trimmed = name.trim();
    const read = readAmount(amount);
    setNameError(trimmed === "" ? SOURCE_NAME_MISSING : null);
    setAmountError(read.kind === "invalid" ? read.sentence : null);
    if (trimmed === "" || read.kind === "invalid") return;

    setBusy(true);
    createFundingSource(
      {
        name: trimmed,
        year,
        granted_amount: read.kind === "amount" ? read.dong : undefined,
      },
      idempotencyKey,
    ).then((result) => {
      setBusy(false);
      if (!result.ok) {
        toast.error(result.thongBao);
        return;
      }
      toast.success("Đã thêm nguồn vốn.");
      setName("");
      setAmount("");
      onAdded();
    });
  }

  return (
    // The existing list holds its own small forms (`Sửa`), so it sits OUTSIDE this form — forms do not
    // nest — and the footer's submit button reaches back with `form=`.
    <div className="flex min-h-0 min-w-0 flex-col gap-3.5 overflow-y-auto">
      <form
        id={ADD_FORM_ID}
        onSubmit={submit}
        aria-label="Thêm nguồn vốn"
        className="flex min-w-0 flex-col gap-3.5"
      >
        <Field
          label="Tên nguồn vốn"
          required
          htmlFor="ten-nguon-von"
          grow="auto"
          className="min-w-0"
          error={nameError ?? undefined}
        >
          <input
            id="ten-nguon-von"
            value={name}
            autoComplete="off"
          // Spec 06 §B: the dialog opens to type a name.
            autoFocus
            placeholder="Chương trình mục tiêu quốc gia"
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field
          label="Vốn được giao trong năm (đồng)"
          htmlFor="von-duoc-giao-moi"
          grow="auto"
          className="min-w-0"
          hint={amountHint(amount) ?? "Chưa rõ thì để trống, bổ sung sau."}
          error={amountError ?? undefined}
        >
          <input
            id="von-duoc-giao-moi"
            value={amount}
            inputMode="numeric"
            autoComplete="off"
            placeholder="8.000.000.000"
            className="tabular-nums"
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
      </form>
      {existing}
      <div className="flex justify-end gap-2 pt-1">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onClick={onClose}
        >
          Đóng
        </Button>
        <Button
          type="submit"
          form={ADD_FORM_ID}
          variant="primary"
          disabled={busy}
          aria-busy={busy || undefined}
        >
          Thêm nguồn vốn
        </Button>
      </div>
    </div>
  );
}
