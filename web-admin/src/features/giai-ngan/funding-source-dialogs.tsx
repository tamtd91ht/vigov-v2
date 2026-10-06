"use client";

import { Pencil, Plus } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { createFundingSource, listFundingSourceProjects, setGrantedAmount } from "@/lib/api/funding-sources";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_fundingSourceOut, finance_fundingSourceProjectsOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { compactDong } from "@/lib/compact-dong";

// vi-name-ok: existing formatters of nhan-du-an.ts, imported unchanged (rule 12, invariant 3)
import { nhanTien, nhanTyLeGiaiNgan } from "./nhan-du-an";
// vi-name-ok: existing amount parser and its two sentences (rule 12, invariant 3)
import { CAU_SO_TIEN_KHONG_DOC_DUOC, CAU_SO_TIEN_VUOT_CHINH_XAC, docSoTien } from "./nhan-ghi-giai-ngan";
import { Glyph } from "./project-ui";

/**
 * The two dialogs of §6: the projects behind one card (prototype `SourceItemsDialog.tsx`) and
 * `Quản lý nguồn vốn` (prototype `BudgetSourceForm.tsx`). Field names in the prototype are inverted
 * against the contract — see `funding-source-progress.tsx`.
 *
 * NO DELETE AND NO RENAME (decided 06/10/2026): a source is declared once and serves every year, and
 * its name is what archived reports cite. There is no route for either, so no control either.
 */

/* ── Amount box ──────────────────────────────────────────────────────────────────────────────── */

export const SOURCE_NAME_MISSING = "Chưa có tên nguồn vốn.";
export const GRANTED_AMOUNT_MISSING = "Chưa có số vốn được giao. Năm này nguồn không giao vốn thì nhập 0.";

/**
 * A typed amount → đồng, through the screen's one parser (`docSoTien`). `blank` is returned as such:
 * the ADD form sends nothing for it ("not entered"), the EDIT form refuses it (the route requires a
 * figure, and a blank read as 0 would record "granted nothing" that nobody said).
 */
type AmountRead = { kind: "blank" } | { kind: "amount"; dong: number } | { kind: "invalid"; sentence: string };

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

/**
 * Which projects make up a card's "đã phân bổ". Each row is THIS SOURCE'S share of a project, not the
 * project's whole plan — the rows add up to the card (plus the server's "paid without allocation"
 * remainder, said under the table when it is not 0).
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
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<finance_fundingSourceProjectsOut> } | null>(
    null,
  );

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
    <ModalDialog titleId={PROJECTS_TITLE_ID} size="lg" onDismiss={onClose}>
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
            <SkeletonRows rows={3} />
          </>
        )}
        {current !== null && !current.ok && (
          <ErrorState
            title="Chưa tải được danh sách dự án của nguồn vốn này"
            message={<span role="alert">{current.thongBao}</span>}
            onRetry={() => setReloads((n) => n + 1)}
          />
        )}
        {current !== null && current.ok && <SourceProjectsTable data={current.duLieu} onNavigate={onClose} />}
      </div>
      <div className="flex shrink-0 justify-end">
        <Button type="button" variant="secondary" onClick={onClose}>
          Đóng
        </Button>
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
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {data.items.length === 0 ? (
        <p className="m-0 py-6 text-center text-[13px] text-ink-500">Chưa có dự án nào lấy vốn từ nguồn này.</p>
      ) : (
        <TableScroll aria-label={`Dự án lấy vốn từ ${data.name} năm ${data.year}`}>
          <table className={DATA_TABLE_CLASS}>
            <thead>
              <tr>
                <th scope="col" className="min-w-56">
                  Dự án
                </th>
                <th scope="col" className="text-right">
                  Phân bổ từ nguồn này
                </th>
                <th scope="col" className="text-right">
                  Đã chi
                </th>
                <th scope="col" className="text-right">
                  Tỷ lệ
                </th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((p) => (
                <tr key={p.id}>
                  <td className="whitespace-normal">
                    <Link
                      href={`/giai-ngan/du-an/${encodeURIComponent(p.id)}`}
                      onClick={onNavigate}
                      className="font-semibold text-ink-900 no-underline hover:underline"
                    >
                      {p.name}
                    </Link>
                    <span className="ma-muc block text-xs text-ink-500">{p.code}</span>
                  </td>
                  <td className="text-right font-semibold whitespace-nowrap tabular-nums">{nhanTien(p.allocated_amount)}</td>
                  <td className="text-right whitespace-nowrap tabular-nums">{nhanTien(p.disbursed_amount)}</td>
                  <td className="text-right font-semibold whitespace-nowrap tabular-nums">
                    {/* `null` = nothing allocated from this source: no denominator, never "0%". */}
                    {p.disbursed_ratio === null ? "—" : nhanTyLeGiaiNgan(p.disbursed_ratio)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableScroll>
      )}
      {data.disbursed_without_allocation_amount > 0 && (
        <p className="m-0 text-[13px] text-warning-600">
          Ngoài các dòng trên, còn {nhanTien(data.disbursed_without_allocation_amount)} đã chi từ nguồn này ở những dự
          án chưa khai phân bổ từ nguồn này.
        </p>
      )}
    </div>
  );
}

/* ── Quản lý nguồn vốn ───────────────────────────────────────────────────────────────────────── */

const MANAGE_TITLE_ID = "tieu-de-quan-ly-nguon-von";

/**
 * The commune's sources with THIS year's granted amount, editable in place, and the add form.
 *
 * AFTER EVERY WRITE THE CALLER RE-READS (`onChanged`), never patches: the write replies carry no
 * derived figure worth trusting next to the cards (a new source's 201 is all zeros by construction,
 * and the PUT returns only the amount). `sources` is `null` while that re-read runs.
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
    <ModalDialog titleId={MANAGE_TITLE_ID} onDismiss={onClose} className="max-w-[38rem]">
      <ModalDialogHeader
        titleId={MANAGE_TITLE_ID}
        title="Quản lý nguồn vốn"
        description="Mỗi nguồn vốn khai một lần và dùng chung cho mọi năm. Vốn được giao ghi riêng cho từng năm ngân sách."
      />
      <div className="flex min-h-0 flex-col gap-4 overflow-y-auto">
        <AddFundingSourceForm year={year} onAdded={onChanged} />
        <SourceList year={year} sources={sources} onSaved={onChanged} />
      </div>
      <div className="flex shrink-0 justify-end">
        <Button type="button" variant="secondary" onClick={onClose}>
          Đóng
        </Button>
      </div>
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
      <p role="status" className="m-0 text-[13px] text-ink-500">
        Đang tải danh sách nguồn vốn…
      </p>
    );
  }
  if (sources.length === 0) return null;
  return (
    <section className="rounded-[10px] border border-line p-3" aria-labelledby="tieu-de-nguon-von-dang-co">
      <h3 id="tieu-de-nguon-von-dang-co" className="m-0 mb-2 text-xs font-semibold text-ink-500">
        Xã đang có {sources.length} nguồn vốn
      </h3>
      <ul className="m-0 flex list-none flex-col gap-3 p-0">
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
        setError(result.thongBao);
        return;
      }
      setEditing(false);
      setError(null);
      onSaved();
    });
  }

  return (
    <li className="text-[13px]">
      <div className="flex items-baseline justify-between gap-2">
        <span className="min-w-0 font-semibold text-ink-900">{source.name}</span>
        {!editing && (
          <button
            type="button"
            onClick={() => {
              setTyped(String(source.granted_amount));
              setEditing(true);
            }}
            aria-label={`Sửa vốn được giao năm ${year} của ${source.name}`}
            className="flex shrink-0 cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-xs text-brand-600 hover:underline"
          >
            <Glyph icon={Pencil} className="size-3" />
            Sửa
          </button>
        )}
      </div>

      {editing ? (
        <form
          onSubmit={save}
          className="mt-1.5 flex min-w-0 flex-col gap-1.5"
          aria-label={`Vốn được giao năm ${year} của ${source.name}`}
        >
          <div className="flex min-w-0 flex-wrap items-end gap-2">
            <Field
              label={`Vốn được giao năm ${year} (đồng)`}
              htmlFor={inputId}
              grow="auto"
              className="min-w-0 flex-1"
              hint={amountHint(typed)}
            >
              <input
                id={inputId}
                value={typed}
                inputMode="numeric"
                autoComplete="off"
                className="tabular-nums"
                onChange={(e) => setTyped(e.target.value)}
              />
            </Field>
            <Button type="submit" variant="primary" size="sm" disabled={busy} aria-busy={busy || undefined}>
              <BusyLabel busy={busy} label="Lưu" busyText={BUSY_SAVING} />
            </Button>
            <Button
              type="button"
              variant="secondary"
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
          {error !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {error}
            </p>
          )}
        </form>
      ) : (
        <p className="m-0 mt-0.5 text-ink-500">
          Được giao năm {year}: <span title={nhanTien(source.granted_amount)}>{compactDong(source.granted_amount)}</span>{" "}
          · đã phân bổ <span title={nhanTien(source.allocated_amount)}>{compactDong(source.allocated_amount)}</span>
          {source.overallocated_amount > 0 ? (
            <span className="font-semibold text-danger-600"> · vượt nguồn {compactDong(source.overallocated_amount)}</span>
          ) : source.granted_amount > 0 ? (
            <span className={cn(source.unallocated_amount > 0 && "font-semibold text-warning-600")}>
              {" "}
              · còn {compactDong(source.unallocated_amount)} chưa phân bổ
            </span>
          ) : null}{" "}
          · {source.project_count} dự án
        </p>
      )}
    </li>
  );
}

/**
 * Name + this year's granted amount (optional). Blank amount = NOT SENT ("not entered yet"), which the
 * server records differently from 0 ("granted nothing").
 *
 * THE IDEMPOTENCY KEY is made when the form opens and kept across a failed retry — the first POST may
 * have reached the server. A NEW key is made only after a success, for the next source.
 *
 * 409 (name taken, catalogue full) and every other refusal show the server's Vietnamese sentence
 * verbatim (`goi.ts`): rewriting it here would be a second copy of a business rule.
 */
function AddFundingSourceForm({ year, onAdded }: { year: number; onAdded: () => void }) {
  const [name, setName] = useState("");
  const [amount, setAmount] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(khoaChongTrungMoi);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [added, setAdded] = useState<string | null>(null);

  function submit(e: FormEvent) {
    e.preventDefault();
    setAdded(null);
    const trimmed = name.trim();
    if (trimmed === "") return setError(SOURCE_NAME_MISSING);
    const read = readAmount(amount);
    if (read.kind === "invalid") return setError(read.sentence);

    setBusy(true);
    createFundingSource(
      { name: trimmed, year, granted_amount: read.kind === "amount" ? read.dong : undefined },
      idempotencyKey,
    ).then((result) => {
      setBusy(false);
      if (!result.ok) {
        setError(result.thongBao);
        return;
      }
      setError(null);
      setName("");
      setAmount("");
      setIdempotencyKey(khoaChongTrungMoi());
      setAdded(result.duLieu.name);
      onAdded();
    });
  }

  return (
    <form onSubmit={submit} aria-label="Thêm nguồn vốn" className="flex min-w-0 flex-col gap-3">
      <Field
        label="Tên nguồn vốn *"
        htmlFor="ten-nguon-von"
        grow="auto"
        className="min-w-0"
        hint="Tên đã khai thì không đổi được, và nguồn vốn không xoá được."
      >
        <input
          id="ten-nguon-von"
          value={name}
          autoComplete="off"
          placeholder="Chương trình mục tiêu quốc gia"
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <Field
        label={`Vốn được giao năm ${year} (đồng)`}
        htmlFor="von-duoc-giao-moi"
        grow="auto"
        className="min-w-0"
        hint={amountHint(amount) ?? "Chưa rõ thì để trống, bổ sung sau."}
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
      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
      {added !== null && (
        <p className="m-0 text-[13px] text-success-600" role="status">
          Đã thêm nguồn vốn “{added}”.
        </p>
      )}
      <div className="flex justify-end">
        <Button
          type="submit"
          variant="primary"
          icon={busy ? undefined : <Glyph icon={Plus} />}
          disabled={busy}
          aria-busy={busy || undefined}
        >
          <BusyLabel busy={busy} label="Thêm nguồn vốn" busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
  );
}
