"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { FormMessage } from "@/components/form-parts";
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
 */

export const PAGE_SIZE = 20;

export const LIST_ACTION = "xem danh sách xã";

export function CommuneListToolbar({ permissionKeys }: { permissionKeys: readonly string[] }) {
  // UI hint only: POST /communes is guarded by both keys on the server.
  if (!canCreateCommune(permissionKeys)) return null;
  return (
    <div className="toolbar">
      <Link href="/xa/moi" className="primary-link-button">
        Tạo xã
      </Link>
    </div>
  );
}

export function CommuneTable({ items }: { items: readonly CommuneSummary[] }) {
  if (items.length === 0) {
    return (
      <section className="empty-state">
        <p className="empty-state-title">Sổ xã chưa có xã nào.</p>
      </section>
    );
  }
  return (
    <div className="table-wrap">
      <table className="data-table">
        <caption className="visually-hidden">Danh sách xã trên nền tảng</caption>
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
                <Link href={`/xa/${encodeURIComponent(c.id)}`}>{c.name}</Link>
              </td>
              <td>{c.province}</td>
              <td>
                <StatusBadge active={c.active} />
              </td>
              <td>{c.domains[0] ?? "—"}</td>
              <td>{c.domains.length > 1 ? c.domains.length - 1 : "Không có"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function CommuneList() {
  const keys = usePermissionKeys();
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

  return (
    <>
      <CommuneListToolbar permissionKeys={keys} />
      {loaded ? <CommuneTable items={items} /> : null}
      {!loaded && loading ? (
        <p role="status" className="loading-line">
          Đang tải danh sách xã…
        </p>
      ) : null}
      <FormMessage text={error} />
      {loaded && hasMore ? (
        <button type="button" className="secondary-button" onClick={loadMore} disabled={loading} aria-busy={loading}>
          {loading ? "Đang tải…" : "Xem thêm"}
        </button>
      ) : null}
    </>
  );
}
