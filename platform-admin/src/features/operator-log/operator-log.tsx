"use client";

import { Funnel, ScrollText } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState, type FormEvent } from "react";

import { FormMessage, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatDateTime } from "@/features/communes/commune-parts";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { listOperatorAuditEntries, type OperatorAuditEntry, type OperatorAuditPage } from "@/lib/api";
import { operatorLogError } from "@/lib/errors";

import { actionLabel, compactValue, logRange, type LogRange } from "./operator-log-model";

/**
 * The operator log (ADR 0073 #2): read only, newest first (the server's one order), filtered by date
 * and — on a commune's page — by that commune, which is the PATH of the request, never a query
 * parameter. Any `ops.*` key reads it. Every page read is itself trailed by the server
 * (`operator_log.read`), so "Xem thêm" is one more trailed read, never a prefetch.
 *
 * No total: the server sends none (core/page). Pagination is the cursor and "Xem thêm".
 */

export const LOG_PAGE_SIZE = 50;

export const PLATFORM_WIDE = "Toàn nền tảng";

function Change({ entry }: { entry: OperatorAuditEntry }) {
  const before = compactValue(entry.before);
  const after = compactValue(entry.after);
  if (before === "" && after === "") return null;
  return (
    <dl className="m-0 mt-1 flex flex-col gap-0.5 text-xs text-ink-500">
      {before !== "" ? (
        <div className="flex min-w-0 gap-1">
          <dt className="shrink-0">Trước:</dt>
          <dd className="m-0 min-w-0 font-mono break-all">{before}</dd>
        </div>
      ) : null}
      {after !== "" ? (
        <div className="flex min-w-0 gap-1">
          <dt className="shrink-0">Sau:</dt>
          <dd className="m-0 min-w-0 font-mono break-all">{after}</dd>
        </div>
      ) : null}
    </dl>
  );
}

/** The table alone; `showCommune` false on a commune's own page, where the column would repeat. */
export function OperatorLogTable({ items, showCommune }: { items: readonly OperatorAuditEntry[]; showCommune: boolean }) {
  if (items.length === 0) {
    return <EmptyState icon={ScrollText} tone="neutral" title="Không có thao tác nào trong khoảng thời gian này." className="py-6" />;
  }
  return (
    <TableScroll aria-label="Nhật ký vận hành">
      <table className={DATA_TABLE_CLASS}>
        <caption className="sr-only">Nhật ký vận hành, mới nhất trước</caption>
        <thead>
          <tr>
            <th scope="col">Thời điểm</th>
            <th scope="col">Người vận hành</th>
            <th scope="col">Thao tác</th>
            {showCommune ? <th scope="col">Xã</th> : null}
            <th scope="col">Đối tượng</th>
            <th scope="col">Lý do</th>
          </tr>
        </thead>
        <tbody>
          {items.map((e, i) => (
            // No id on the wire: time + position is unique within one loaded list.
            <tr key={`${e.at}-${i}`}>
              <td className="text-[13px] whitespace-nowrap text-ink-700">{formatDateTime(e.at)}</td>
              <td className="font-mono text-[13px] text-ink-900">{e.actor}</td>
              <td className="text-ink-900">
                <span className="font-semibold">{actionLabel(e.action)}</span>
                <Change entry={e} />
              </td>
              {showCommune ? (
                <td className="text-ink-700">
                  {e.commune === null ? (
                    <span className="text-ink-500">{PLATFORM_WIDE}</span>
                  ) : (
                    <Link
                      href={`/xa/${encodeURIComponent(e.commune.id)}`}
                      className="text-brand-700 no-underline hover:underline focus-visible:underline"
                    >
                      {e.commune.name || e.commune.id}
                    </Link>
                  )}
                </td>
              ) : null}
              <td className="font-mono text-[13px] break-all text-ink-700">{e.subject || "—"}</td>
              <td className="text-ink-700">{e.reason || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** "Xem thêm" only while the server says there is more. */
export function OperatorLogMore({ hasMore, loading, onMore }: { hasMore: boolean; loading: boolean; onMore: () => void }) {
  if (!hasMore) return null;
  return (
    <div className="flex justify-center">
      <Button type="button" variant="secondary" onClick={onMore} disabled={loading} aria-busy={loading}>
        {loading ? "Đang tải…" : "Xem thêm"}
      </Button>
    </div>
  );
}

function Filters({
  onApply,
  disabled,
}: {
  onApply: (r: LogRange) => void;
  disabled: boolean;
}) {
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [error, setError] = useState<string | null>(null);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const r = logRange(fromDate, toDate);
    if (!r.ok) return setError(r.text);
    setError(null);
    onApply(r.range);
  }

  return (
    <form className="flex flex-col gap-2" method="get" onSubmit={submit} noValidate aria-label="Lọc nhật ký theo ngày">
      <div className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[minmax(0,12rem)_minmax(0,12rem)_auto]">
        <TextField label="Từ ngày" name="from" type="date" value={fromDate} onChange={(e) => setFromDate(e.target.value)} disabled={disabled} />
        <TextField label="Đến ngày" name="to" type="date" value={toDate} onChange={(e) => setToDate(e.target.value)} disabled={disabled} />
        <div>
          <Button type="submit" variant="secondary" disabled={disabled} icon={<Funnel aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Lọc
          </Button>
        </div>
      </div>
      <FormMessage text={error} />
    </form>
  );
}

/** The whole log: filters, table, "Xem thêm". With `communeId`, that commune's acts only. */
export function OperatorLog({ communeId }: { communeId?: string }) {
  const guarded = useGuardedError();
  const [range, setRange] = useState<LogRange>({});
  const [page, setPage] = useState<OperatorAuditPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fail = useCallback(
    (err: unknown) => guarded(err, "xem nhật ký vận hành", (e) => operatorLogError(e)),
    [guarded],
  );

  useEffect(() => {
    let alive = true;
    listOperatorAuditEntries({ ...range, limit: LOG_PAGE_SIZE }, communeId).then(
      (p) => {
        if (!alive) return;
        setPage(p);
        setLoading(false);
      },
      (err: unknown) => {
        if (!alive) return;
        setPage(null);
        setError(fail(err));
        setLoading(false);
      },
    );
    return () => {
      alive = false;
    };
  }, [range, communeId, fail]);

  function apply(r: LogRange) {
    setLoading(true);
    setError(null);
    setRange(r);
  }

  async function more() {
    if (page === null || loading || !page.has_more) return;
    setLoading(true);
    setError(null);
    try {
      const next = await listOperatorAuditEntries({ ...range, limit: LOG_PAGE_SIZE, cursor: page.next_cursor }, communeId);
      setPage({ items: [...page.items, ...next.items], next_cursor: next.next_cursor, has_more: next.has_more });
    } catch (err) {
      setError(fail(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Filters onApply={apply} disabled={loading} />
      {page === null && loading ? (
        <div>
          <p role="status" className="sr-only">
            Đang tải nhật ký vận hành…
          </p>
          <SkeletonRows rows={5} />
        </div>
      ) : null}
      {page === null && !loading && error !== null ? (
        <ErrorState role="alert" title="Chưa tải được nhật ký vận hành" message={error} />
      ) : null}
      {page !== null ? (
        <>
          <OperatorLogTable items={page.items} showCommune={communeId === undefined} />
          <FormMessage text={error} />
          <OperatorLogMore hasMore={page.has_more} loading={loading} onMore={more} />
        </>
      ) : null}
    </div>
  );
}

/** `/nhat-ky-van-hanh`: the log of every commune plus the platform-wide changes, in a card. */
export function OperatorLogScreen() {
  return (
    <Card as="section" className="p-4">
      <OperatorLog />
    </Card>
  );
}
