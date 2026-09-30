"use client";

import { useState } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key generator
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

  return (
    <section className="khoi-chi-tiet" aria-labelledby="tieu-de-chot-ky">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-chot-ky">Chốt kỳ ngân sách năm {year}</h3>
      </div>
      <p className="ghi-chu">
        Chốt theo tháng khoá việc thêm, gỡ đợt thu chi của tháng đó. Chốt cả năm khoá thêm việc sửa
        bảng, khoản mục và số liệu của năm.
      </p>

      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}
      {notice !== null && <p role="status">{notice}</p>}

      {view.phase === "loading" && <p role="status">Đang tải các lần chốt kỳ…</p>}
      {view.phase === "error" && (
        <p className="thong-bao-loi" role="alert">
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
    return <p className="trang-thai-rong">Năm này chưa có kỳ nào được chốt.</p>;
  }
  return (
    <div className="bang-cuon" role="region" aria-label="Các lần chốt kỳ ngân sách" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            <th scope="col">Kỳ</th>
            <th scope="col">Mã lần chốt</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Chốt</th>
            <th scope="col">Mở chốt</th>
            {canConfirm && <th scope="col">Thao tác</th>}
          </tr>
        </thead>
        <tbody>
          {sortCloses(closes).map((c) => (
            <tr key={c.id}>
              <td>{periodLabel(c)}</td>
              <td className="ma-muc">{c.code}</td>
              <td>
                <span className={c.active ? "chip chip-hoat-dong" : "chip chip-ngung"}>
                  {closeStatusLabel(c)}
                </span>
              </td>
              <td>
                {c.closed_by} · {formatInstant(c.closed_at)}
              </td>
              <td>
                {c.active ? (
                  "—"
                ) : (
                  <>
                    {c.reopened_by ?? "—"} · {formatInstant(c.reopened_at)}
                    <br />
                    <span className="ghi-chu">Lý do: {c.reopen_reason ?? "—"}</span>
                  </>
                )}
              </td>
              {canConfirm && (
                <td className="o-thao-tac">
                  {c.active && (
                    <button
                      type="button"
                      className="nut-phu"
                      disabled={busy}
                      aria-label={`Mở chốt ${periodLabel(c).toLowerCase()}`}
                      onClick={() => onReopen(c)}
                    >
                      Mở chốt
                    </button>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const built = buildCloseBody(year, String(fd.get("period") ?? ""));
        if (built.ok) chosen(built.than);
        else invalid(built.thongBao);
      }}
    >
      <h4>Chốt một kỳ</h4>
      <p>
        <label htmlFor="chot-ky-period">Kỳ cần chốt</label>{" "}
        <select id="chot-ky-period" name="period" className="o-chon" required defaultValue="">
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
      </p>
      <button type="submit" className="nut-chinh" disabled={busy}>
        Chốt kỳ…
      </button>
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
    <form
      className="khoi-chua-khai"
      onSubmit={(e) => {
        e.preventDefault();
        confirm();
      }}
    >
      <h4>{closeConfirmTitle(body)}</h4>
      <p className="hau-qua">{closeConsequence(body)}</p>
      <p className="ghi-chu">{AFTER_CLOSE_NOTE}</p>
      <button type="submit" className="nut-chinh" disabled={busy}>
        Chốt kỳ
      </button>{" "}
      <button type="button" className="nut-phu" disabled={busy} onClick={cancel}>
        Huỷ
      </button>
    </form>
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
    <form
      className="khoi-chua-khai"
      onSubmit={(e) => {
        e.preventDefault();
        const checked = validateReopenReason(text);
        if (!checked.ok) {
          setError(checked.thongBao);
          return;
        }
        setError(null);
        submit(checked.than);
      }}
    >
      <h4>Mở chốt {periodLabel(close).toLowerCase()} ({close.code})</h4>
      <p className="hau-qua">{reopenConsequence(close)}</p>
      <p>
        <label htmlFor="mo-chot-reason">Lý do mở chốt (bắt buộc, được lưu cùng lần chốt)</label>
        <br />
        <textarea
          id="mo-chot-reason"
          name="reason"
          className="o-nhap"
          rows={3}
          required
          value={text}
          aria-describedby="mo-chot-reason-count"
          onChange={(e) => setText(e.target.value)}
        />
        <br />
        <span id="mo-chot-reason-count" className="ghi-chu" aria-live="polite">
          {count}/{REOPEN_REASON_MAX} ký tự
        </span>
      </p>
      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="nut-chinh" disabled={busy}>
        Mở chốt
      </button>{" "}
      <button type="button" className="nut-phu" disabled={busy} onClick={cancel}>
        Huỷ
      </button>
    </form>
  );
}
