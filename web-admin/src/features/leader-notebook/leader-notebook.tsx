"use client";

import { useEffect, useMemo, useState } from "react";

import { CongQuyen } from "@/features/quyen/cong-quyen"; // vi-name-ok: existing permission-gate component
import { danhBaChoNhatKy, docBangNhanTrangThai } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing readers of the register, imported unchanged
import { usePhien } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing session hook
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import { getTaskCounts, getTaskExtensionCount, layHangChoLuiHan, laySoNhiemVu } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing task clients
import type { identity_danhBaChonNguoiRa, petitions_danhSachTrangThaiNhiemVuRa, petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu"; // vi-name-ok: existing status-label client
import { coQuyen, QUYEN_DUYET_HOAN_THANH_NHIEM_VU, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen"; // vi-name-ok: existing permission constants

import {
  ASSIGNED_BY_ME_FILTER,
  ASSIGNED_BY_ME_LIST,
  MY_EXTENSION_REQUESTS,
  NO_ACCESS_SENTENCE,
  OVERDUE_FILTER,
  OVERDUE_LIST,
  PAGE_SIZE,
  PENDING_APPROVAL_FILTER,
  sumCounts,
} from "./notebook-queries";
import { NotebookView, type ListState, type Loaded, type Section } from "./notebook-view";

/**
 * Sổ tay lãnh đạo — the reading half. Every list and every figure is ONE server read through the
 * register's clients (`lib/api/nhiem-vu.ts`); nothing is counted or filtered here (ADR 0071).
 *
 * THE GATE IS `task.read` — every route behind the three columns declares exactly that key, so the
 * gate cannot hide anything the server would serve. It is convenience: each route checks it again
 * (rule 5, forbidden #1). Inside the gate, the `Duyệt hoàn thành` group additionally needs
 * `task.approve` (ADR 0071): without it the group is neither read nor drawn — those are rows this
 * account cannot act on.
 */
export function LeaderNotebook() {
  return (
    <CongQuyen khoa={QUYEN_XEM_NHIEM_VU} cauThieuQuyen={NO_ACCESS_SENTENCE}>
      <NotebookColumns />
    </CongQuyen>
  );
}

type Page<T> = { readonly items: readonly T[]; readonly next_cursor: string; readonly has_more: boolean };
type PageReader<T> = (cursor: string | null) => Promise<KetQua<Page<T>>>;
type CountReader = () => Promise<KetQua<number>>;

/* MODULE-LEVEL READERS: stable identities, so the effects below run once per reload, not per render. */

const readPastDue: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...OVERDUE_LIST, limit: PAGE_SIZE, cursor });
const countPastDue: CountReader = () => getTaskCounts(OVERDUE_FILTER).then(toFigure);

const readCompletion: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...PENDING_APPROVAL_FILTER, limit: PAGE_SIZE, cursor });
const countCompletion: CountReader = () => getTaskCounts(PENDING_APPROVAL_FILTER).then(toFigure);

const readExtensions: PageReader<petitions_deNghiChoDuyetRa> = (cursor) =>
  layHangChoLuiHan({ ...MY_EXTENSION_REQUESTS, limit: PAGE_SIZE, cursor });
const countExtensions: CountReader = () =>
  getTaskExtensionCount(MY_EXTENSION_REQUESTS).then((r) => (r.ok ? { ok: true, duLieu: r.duLieu.count } : r));

const readAssigned: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...ASSIGNED_BY_ME_LIST, limit: PAGE_SIZE, cursor });
const countAssigned: CountReader = () => getTaskCounts(ASSIGNED_BY_ME_FILTER).then(toFigure);

function toFigure(r: Awaited<ReturnType<typeof getTaskCounts>>): KetQua<number> {
  return r.ok ? { ok: true, duLieu: sumCounts(r.duLieu) } : r;
}

function NotebookColumns() {
  const session = usePhien();
  // Inside `CongQuyen`, so the session is read and holds `task.read`. FAIL CLOSED all the same: an
  // unread session holds no key.
  const canApprove =
    session !== null && session.ok && coQuyen(session.duLieu.permissions, QUYEN_DUYET_HOAN_THANH_NHIEM_VU);

  const pastDue = useSection(true, readPastDue, countPastDue);
  const completion = useSection(canApprove, readCompletion, countCompletion);
  const extensions = useSection(true, readExtensions, countExtensions);
  const assigned = useSection(true, readAssigned, countAssigned);

  // Names for the assignee line and the commune's status labels: one read each for the page. A failed
  // read is not an error state — rows then show the staff code, and the pill the default label.
  const [directoryResult, setDirectoryResult] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [labelsResult, setLabelsResult] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(null);
  useEffect(() => {
    let cancelled = false;
    layDanhBaChonNguoi().then((r) => {
      if (!cancelled) setDirectoryResult(r);
    });
    layTrangThaiNhiemVu().then((r) => {
      if (!cancelled) setLabelsResult(r);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  const directory = useMemo(() => danhBaChoNhatKy(directoryResult), [directoryResult]);
  const labels = docBangNhanTrangThai(labelsResult);

  return (
    <>
      {labels.canhBao !== null && <p className="ghi-chu">{labels.canhBao}</p>}
      <NotebookView
        pastDue={pastDue}
        completion={canApprove ? completion : null}
        extensions={extensions}
        assigned={assigned}
        directory={directory}
        statusLabels={labels.bang}
        now={new Date()}
      />
    </>
  );
}

type PageLoad<T> = {
  readonly token: number;
  readonly load: Loaded<readonly T[]>;
  readonly cursor: string;
  readonly hasMore: boolean;
};

/**
 * One list and its server count. `Tải lại` re-reads BOTH from the first page: the two must describe the
 * same moment. A late answer for an older reload is dropped, never drawn over a newer one.
 */
function useSection<T>(enabled: boolean, readPage: PageReader<T>, readCount: CountReader): Section<T> {
  const [reload, setReload] = useState(0);
  const [page, setPage] = useState<PageLoad<T> | null>(null);
  const [count, setCount] = useState<{ token: number; load: Loaded<number> } | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    readPage(null).then((r) => {
      if (cancelled) return;
      setPage(
        r.ok
          ? {
              token: reload,
              load: { phase: "done", value: r.duLieu.items },
              cursor: r.duLieu.next_cursor,
              hasMore: r.duLieu.has_more && r.duLieu.next_cursor !== "",
            }
          : { token: reload, load: { phase: "error", message: r.thongBao }, cursor: "", hasMore: false },
      );
    });
    readCount().then((r) => {
      if (cancelled) return;
      setCount({ token: reload, load: r.ok ? { phase: "done", value: r.duLieu } : { phase: "error", message: r.thongBao } });
    });
    return () => {
      cancelled = true;
    };
  }, [enabled, readPage, readCount, reload]);

  const current = page !== null && page.token === reload ? page : null;

  function more(): void {
    if (current === null || current.load.phase !== "done" || !current.hasMore || loadingMore) return;
    const before = current;
    const shown = current.load.value;
    setLoadingMore(true);
    setMoreError(null);
    readPage(before.cursor).then((r) => {
      setLoadingMore(false);
      if (!r.ok) {
        setMoreError(r.thongBao);
        return;
      }
      setPage((p) =>
        p === null || p.token !== before.token
          ? p
          : {
              token: p.token,
              load: { phase: "done", value: [...shown, ...r.duLieu.items] },
              cursor: r.duLieu.next_cursor,
              hasMore: r.duLieu.has_more && r.duLieu.next_cursor !== "",
            },
      );
    });
  }

  const list: ListState<T> = {
    load: current === null ? { phase: "loading" } : current.load,
    hasMore: current?.hasMore ?? false,
    loadingMore,
    moreError,
    onRetry: () => {
      setMoreError(null);
      setReload((n) => n + 1);
    },
    onMore: more,
  };
  return { list, count: count !== null && count.token === reload ? count.load : { phase: "loading" } };
}
