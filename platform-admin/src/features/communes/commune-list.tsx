"use client";

import { Landmark, Plus } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { FormMessage } from "@/components/form-parts";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { listCommunes, type CommuneSummary } from "@/lib/api";
import { canCreateCommune } from "@/lib/permissions";

import { StatusBadge } from "./commune-parts";

/**
 * `/xa` — the commune directory (ADR 0048 §01/10 #4). Registry metadata only: name, province,
 * status, domains (ADR 0003; `communeView` is the whole of what the server sends).
 *
 * Cursor pagination with "Xem thêm": the server's order is creation time, and a cursor is the one
 * way a page boundary stays put while communes are being created (core/page).
 *
 * No filter bar: the list route takes no filter (`listCommunes` sends `limit` and `cursor` only),
 * and a search box that filtered only the rows already loaded would hide communes on later pages.
 */

export const PAGE_SIZE = 20;

export const LIST_ACTION = "xem danh sách xã";

export function CommuneListToolbar({ permissionKeys }: { permissionKeys: readonly string[] }) {
  // UI hint only: POST /communes is guarded by both keys on the server.
  if (!canCreateCommune(permissionKeys)) return null;
  return (
    <Link href="/xa/moi" className={buttonVariants({ variant: "primary" })}>
      <Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />
      Tạo xã
    </Link>
  );
}

/** The page header's action slot: the toolbar fed with the operator's keys. */
export function CommuneListActions() {
  return <CommuneListToolbar permissionKeys={usePermissionKeys()} />;
}

export function CommuneTable({ items }: { items: readonly CommuneSummary[] }) {
  if (items.length === 0) {
    return (
      <Card as="section">
        <EmptyState icon={Landmark} title="Sổ xã chưa có xã nào." />
      </Card>
    );
  }
  return (
    <TableScroll sticky aria-label="Danh sách xã">
      <table className={DATA_TABLE_CLASS}>
        <caption className="sr-only">Danh sách xã trên nền tảng</caption>
        <thead>
          <tr>
            <th scope="col">Tên xã</th>
            <th scope="col">Tỉnh, thành phố</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Tên miền chính</th>
            <th scope="col">Tên miền khác</th>
          </tr>
        </thead>
        <tbody>
          {items.map((c) => (
            <tr key={c.id}>
              <td>
                <Link
                  href={`/xa/${encodeURIComponent(c.id)}`}
                  className="font-semibold text-brand-700 no-underline hover:underline focus-visible:underline"
                >
                  {c.name}
                </Link>
              </td>
              <td className="text-ink-700">{c.province}</td>
              <td>
                <StatusBadge active={c.active} />
              </td>
              <td className="font-mono text-[13px] text-ink-700">{c.domains[0] ?? "—"}</td>
              <td className="text-ink-700">{c.domains.length > 1 ? c.domains.length - 1 : "Không có"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

export function CommuneList() {
  const guarded = useGuardedError();
  const [items, setItems] = useState<CommuneSummary[]>([]);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    listCommunes({ limit: PAGE_SIZE }).then(
      (page) => {
        if (!alive) return;
        setItems(page.items);
        setCursor(page.next_cursor);
        setHasMore(page.has_more);
        setLoaded(true);
        setLoading(false);
      },
      (err: unknown) => {
        if (!alive) return;
        setError(guarded(err, LIST_ACTION));
        setLoading(false);
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  async function loadMore() {
    if (loading || !hasMore) return;
    setLoading(true);
    setError(null);
    try {
      const page = await listCommunes({ limit: PAGE_SIZE, cursor });
      setItems((prev) => [...prev, ...page.items]);
      setCursor(page.next_cursor);
      setHasMore(page.has_more);
    } catch (err) {
      setError(guarded(err, LIST_ACTION));
    } finally {
      setLoading(false);
    }
  }

  // The first page never arrived: the server's sentence, verbatim, in place of the table. No
  // "Tải lại" — the screen has no reload mechanism (`ui/error-state.tsx`).
  if (!loaded && error !== null) {
    return (
      <Card as="section">
        <ErrorState role="alert" title="Chưa tải được danh sách xã" message={error} />
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {loaded ? <CommuneTable items={items} /> : null}
      {!loaded && loading ? (
        <Card>
          <p role="status" className="sr-only">
            Đang tải danh sách xã…
          </p>
          <SkeletonRows rows={6} />
        </Card>
      ) : null}
      {loaded ? <FormMessage text={error} /> : null}
      {loaded && hasMore ? (
        <div className="flex justify-center">
          <Button type="button" variant="secondary" onClick={loadMore} disabled={loading} aria-busy={loading}>
            {loading ? "Đang tải…" : "Xem thêm"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
