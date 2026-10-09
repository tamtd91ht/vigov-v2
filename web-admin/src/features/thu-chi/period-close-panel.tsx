"use client";

import { CircleCheck, LockKeyhole, LockKeyholeOpen } from "lucide-react";
import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key generator
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { Field } from "@/components/ui/field";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing shared result type
import { closeBudgetPeriod, reopenBudgetPeriodClose } from "@/lib/api/budget-period-close";
import type {
  finance_budgetPeriodCloseIn,
  finance_budgetPeriodCloseOut,
} from "@/lib/api/schema.gen";

import { khoaSauLanGhi } from "./nhan-thu-chi"; // vi-name-ok: existing key-rotation rule of this feature
import {
  AFTER_CLOSE_NOTE,
  buildCloseBody,
  charCount,
  closeConfirmTitle,
  closeConsequence,
  closeStatusLabel,
  formatInstant,
  periodLabel,
  reopenConsequence,
  REOPEN_REASON_MAX,
  sortCloses,
  validateReopenReason,
} from "./period-close";

/**
 * "Chốt kỳ ngân sách" block of the Thu - Chi screen, for the selected year.
 *
 * `budget.read` reads the history (the page gate already requires it); CLOSE and REOPEN stand
 * behind `budget.confirm` (`routes.go`, budget-period-closes block). Hiding the controls without
 * that key is convenience, not security — the server checks `(tenant_id, role, permission)` on
 * every call and a direct call with the same session cookie still answers 403 (rule 5 forbidden #1).
 *
 * After every successful close or reopen the WHOLE screen reloads (`onChanged`): the lock state of
 * the sheet and of every entry derives from this list, and patching it locally would leave a lock
 * badge the server no longer agrees with.
 */
export type ClosesView =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; closes: readonly finance_budgetPeriodCloseOut[] };

export function BudgetPeriodClosePanel({
  year,
  view,
  canConfirm,
  onChanged,
}: {
  year: number;
  view: ClosesView;
  canConfirm: boolean;
  /** A close or reopen succeeded: reload closes, sheet and indicators. */
  onChanged: () => void;
}) {
  // Each idempotency key is created when its form OPENS, kept on failure (a retry after a network
  // error must not become a second act), and renewed after success — `khoaSauLanGhi`.
  const [closeKey, setCloseKey] = useState(khoaChongTrungMoi);
  const [reopenKey, setReopenKey] = useState(khoaChongTrungMoi);
  const [pending, setPending] = useState<finance_budgetPeriodCloseIn | null>(null);
  const [reopening, setReopening] = useState<finance_budgetPeriodCloseOut | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  function settle(
    result: KetQua<finance_budgetPeriodCloseOut>,
    done: (close: finance_budgetPeriodCloseOut) => string,
  ): boolean {
    setBusy(false);
    if (!result.ok) {
      // VERBATIM server sentence: a 409 names the period and the close code already in force.
      setError(result.thongBao);
      setNotice(null);
      return false;
    }
    setError(null);
    setNotice(done(result.duLieu));
    onChanged();
    return true;
  }

  // The spec's shell for this kept block (spec 03 §B): the prototype's card — a 10px hairline box, no
  // shadow, `px-4 py-3` — with the title at the size of the other blocks' titles (14px navy bold).
  // Its own markup rather than `Card` + overrides: `rounded-card` / `shadow-card` are theme names
  // tailwind-merge does not know, so an override would leave both radius classes on the element.
  // `data-slot="card"` keeps the history table's scroller from drawing a second frame (`globals.css`).
  return (
    <section
      data-slot="card"
      aria-labelledby="tieu-de-chot-ky"
      className="flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid border-line bg-surface px-4 py-3"
    >
      <div className="min-w-0">
        <h3 id="tieu-de-chot-ky" className="m-0 inline-flex items-center gap-2 text-[14px] leading-snug font-bold text-navy">
          <LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-brand-600" />
          Chốt kỳ ngân sách năm {year}
        </h3>
        <p className="m-0 mt-0.5 text-[11.5px] text-ink-muted">
          Chốt theo tháng khoá việc thêm, gỡ đợt thu chi của tháng đó. Chốt cả năm khoá thêm việc
          sửa bảng, khoản mục và số liệu của năm.
        </p>
      </div>

      <div className="flex min-w-0 flex-col gap-3">
      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
      {notice !== null && (
        <p role="status" className="m-0 inline-flex items-center gap-1.5 text-sm text-success-600">
          <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0" />
          {notice}
        </p>
      )}

      {view.phase === "loading" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải các lần chốt kỳ…
          </p>
          <SkeletonRows rows={2} className="-mx-4" />
        </>
      )}
      {view.phase === "error" && (
        <p className="thong-bao-loi m-0" role="alert">
          {view.message} Chưa đọc được trạng thái chốt kỳ; máy chủ vẫn từ chối mọi thao tác ghi
          vào kỳ đã chốt.
        </p>
      )}
      {view.phase === "ready" && (
        <PeriodCloseList
          closes={view.closes}
          canConfirm={canConfirm}
          busy={busy}
          onReopen={(c) => {
            setReopening(c);
            setPending(null);
            setError(null);
            setNotice(null);
          }}
        />
      )}

      {canConfirm && reopening !== null && (
        <ReopenForm
          close={reopening}
          busy={busy}
          cancel={() => setReopening(null)}
          submit={(reason) => {
            setBusy(true);
            reopenBudgetPeriodClose(reopening.code, reason, reopenKey).then((r) => {
              const ok = settle(r, (c) => `Đã mở chốt ${periodLabel(c).toLowerCase()} (${c.code}).`);
              setReopenKey((k) => khoaSauLanGhi(k, ok, khoaChongTrungMoi));
              if (ok) setReopening(null);
            });
          }}
        />
      )}

      {canConfirm &&
        (pending === null ? (
          <CloseForm
            year={year}
            busy={busy}
            chosen={(body) => {
              setPending(body);
              setReopening(null);
              setError(null);
              setNotice(null);
            }}
            invalid={(message) => {
              setError(message);
              setNotice(null);
            }}
          />
        ) : (
          <CloseConfirm
            body={pending}
            busy={busy}
            cancel={() => setPending(null)}
            confirm={() => {
              setBusy(true);
              closeBudgetPeriod(pending, closeKey).then((r) => {
                const ok = settle(r, (c) => `Đã chốt ${periodLabel(c).toLowerCase()}, mã ${c.code}.`);
                setCloseKey((k) => khoaSauLanGhi(k, ok, khoaChongTrungMoi));
                if (ok) setPending(null);
              });
            }}
          />
        ))}
      </div>
    </section>
  );
}

/**
 * The close history: active closes first, then reopened ones with who, when and why. Rendered for
 * every reader of the screen — who closed a period is information everybody needs.
 */
export function PeriodCloseList({
  closes,
  canConfirm,
  busy,
  onReopen,
}: {
  closes: readonly finance_budgetPeriodCloseOut[];
  canConfirm: boolean;
  busy: boolean;
  onReopen: (close: finance_budgetPeriodCloseOut) => void;
}) {
  if (closes.length === 0) {
    return <p className="m-0 text-[12.5px] text-ink-muted">Năm này chưa có kỳ nào được chốt.</p>;
  }
  return (
    <TableScroll aria-label="Các lần chốt kỳ ngân sách" className="rounded-xl">
      <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
        <thead>
          <tr>
            <th scope="col">Kỳ</th>
            <th scope="col">Mã lần chốt</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Chốt</th>
            <th scope="col">Mở chốt</th>
            {canConfirm && (
              <th scope="col" className="text-right">
                Thao tác
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {sortCloses(closes).map((c) => (
            <tr key={c.id}>
              <td>{periodLabel(c)}</td>
              <td className="ma-muc">{c.code}</td>
              <td>
                {/* Tone by the `active` FLAG, never by the label: in force = the lock icon, reopened
                    = the open lock in grey. Icon + word, never colour alone. */}
                <Badge
                  tone={c.active ? "info" : "neutral"}
                  icon={c.active ? LockKeyhole : LockKeyholeOpen}
                >
                  {closeStatusLabel(c)}
                </Badge>
              </td>
              <td className="tabular-nums">
                {c.closed_by} · {formatInstant(c.closed_at)}
              </td>
              <td className="tabular-nums">
                {c.active ? (
                  "—"
                ) : (
                  <>
                    {c.reopened_by ?? "—"} · {formatInstant(c.reopened_at)}
                    <br />
                    <span className="ghi-chu whitespace-normal">Lý do: {c.reopen_reason ?? "—"}</span>
                  </>
                )}
              </td>
              {canConfirm && (
                <td className="text-right">
                  {c.active && (
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      icon={<LockKeyholeOpen aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                      disabled={busy}
                      aria-label={`Mở chốt ${periodLabel(c).toLowerCase()}`}
                      onClick={() => onReopen(c)}
                    >
                      Mở chốt
                    </Button>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/**
 * Step 1 of a close: choose a month or the whole year. Nothing is sent from here — the choice goes
 * to the confirm step, which says what the close will lock.
 */
export function CloseForm({
  year,
  busy,
  chosen,
  invalid,
}: {
  year: number;
  busy: boolean;
  chosen: (body: finance_budgetPeriodCloseIn) => void;
  invalid: (message: string) => void;
}) {
  // One row, as spec 03 §B draws it: the label "Kỳ cần chốt" (11.5px), the select (the spec's standard
  // native select, 36px, 12.5px), and `Button size="sm"` "Chốt kỳ…". No divider and no sub-heading —
  // the block's own title already says what this is; the form keeps an accessible name of its own.
  return (
    <form
      aria-label="Chốt một kỳ"
      className="flex min-w-0 flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const built = buildCloseBody(year, String(fd.get("period") ?? ""));
        if (built.ok) chosen(built.than);
        else invalid(built.thongBao);
      }}
    >
      <label htmlFor="chot-ky-period" className="text-[11.5px] font-medium text-ink">
        Kỳ cần chốt
      </label>
      {/* `min-w-0` / `min-h-9` / `pl-3` beat the legacy select frame's 12rem floor, 36px floor and
          padding (`globals.css`, `legacy` layer); its chevron image stays. */}
      <select
        id="chot-ky-period"
        name="period"
        required
        defaultValue=""
        className="h-9 min-h-9 min-w-0 rounded-md border border-solid border-line bg-surface pl-3 text-[12.5px] md:text-[12.5px]"
      >
        <option value="" disabled>
          Chọn kỳ
        </option>
        {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
          <option key={m} value={String(m)}>
            Tháng {m}/{year}
          </option>
        ))}
        <option value="year">Cả năm {year}</option>
      </select>
      <Button
        type="submit"
        variant="primary"
        size="sm"
        icon={<LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        disabled={busy}
      >
        Chốt kỳ…
      </Button>
    </form>
  );
}

/** Step 2 of a close: one sentence on what it locks, then the only button that sends. */
export function CloseConfirm({
  body,
  busy,
  confirm,
  cancel,
}: {
  body: finance_budgetPeriodCloseIn;
  busy: boolean;
  confirm: () => void;
  cancel: () => void;
}) {
  return (
    <ConfirmDialog
      as="form"
      icon={LockKeyhole}
      title={closeConfirmTitle(body)}
      titleAs="h4"
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        confirm();
      }}
      actions={
        <>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy || undefined}>
            <BusyLabel busy={busy} label="Chốt kỳ" busyText="Đang chốt…" />
          </Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={cancel}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="m-0">{closeConsequence(body)}</p>
      <p className="m-0 text-xs text-ink-500">{AFTER_CLOSE_NOTE}</p>
    </ConfirmDialog>
  );
}

/** Reopen one active close: a required reason, at most 500 characters, with a live counter. */
export function ReopenForm({
  close,
  busy,
  submit,
  cancel,
}: {
  close: finance_budgetPeriodCloseOut;
  busy: boolean;
  submit: (reason: string) => void;
  cancel: () => void;
}) {
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const count = charCount(text.trim());

  return (
    <ConfirmDialog
      as="form"
      icon={LockKeyholeOpen}
      title={`Mở chốt ${periodLabel(close).toLowerCase()} (${close.code})?`}
      titleAs="h4"
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const checked = validateReopenReason(text);
        if (!checked.ok) {
          setError(checked.thongBao);
          return;
        }
        setError(null);
        submit(checked.than);
      }}
      actions={
        <>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy || undefined}>
            <BusyLabel busy={busy} label="Mở chốt" busyText="Đang mở chốt…" />
          </Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={cancel}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="m-0">{reopenConsequence(close)}</p>
      <Field
        label="Lý do mở chốt (bắt buộc, được lưu cùng lần chốt)"
        htmlFor="mo-chot-reason"
        grow="auto"
        hint={
          <span id="mo-chot-reason-count" aria-live="polite" className="tabular-nums">
            {count}/{REOPEN_REASON_MAX} ký tự
          </span>
        }
      >
        <textarea
          id="mo-chot-reason"
          name="reason"
          className="o-nhap py-2"
          rows={3}
          required
          value={text}
          aria-describedby="mo-chot-reason-count"
          onChange={(e) => setText(e.target.value)}
        />
      </Field>
      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
    </ConfirmDialog>
  );
}
