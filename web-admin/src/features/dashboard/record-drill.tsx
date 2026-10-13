"use client";

import { ExternalLink } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { CHUA_PHAN_CONG, danhBaChoNhatKy, nhanCanBoNgan, nhanNgay } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing formatters of the register, imported not declared
import { petitionHref } from "@/features/phan-anh/petition-merge";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing org-unit client
import { layDanhSachDuAn } from "@/lib/api/du-an"; // vi-name-ok: existing project-list client
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import { countCitizenReports, laySoPhanAnh } from "@/lib/api/phieu-phan-anh"; // vi-name-ok: existing petition-register client
import type {
  documents_vanBanDenRa, // vi-name-ok: generated contract type
  finance_duAnRa, // vi-name-ok: generated contract type
  identity_danhBaChonNguoiRa, // vi-name-ok: generated contract type
  petitions_phieuPhanAnhRa, // vi-name-ok: generated contract type
} from "@/lib/api/schema.gen";
import { laySoVanBanDen } from "@/lib/api/van-ban"; // vi-name-ok: existing incoming-register client

import {
  DEFAULT_DRILL_HEADERS,
  DRILL_LOADING,
  DRILL_MORE,
  DRILL_OPEN,
  DRILL_OPEN_CLASS,
  DRILL_PAGE_SIZE,
  drillDescriptionOf,
  DrillTableDialog,
  type DrillBody,
  type DrillHeaders,
  type DrillRow,
} from "./drill-dialog";
import { drillHref, formatCount } from "./figures";
import { BUDGET_SCREEN_PATH, type DrillRequest } from "./view";

/**
 * Tổng quan's rows behind a figure of Văn bản & Đơn thư, Phản ánh người dân and Giải ngân ngân sách
 * (customer sheet web-admin-131026, TỔNG QUAN: "Khi click vào con số tổng sẽ mở popup danh sách tương
 * tự như cụm Nhiệm vụ") — the task dialog's frame (`drill-dialog.tsx`), one reader per register:
 *
 *   incoming-documents  GET /api/v1/incoming-documents?metric=…[&from&to]   (`document.read`)
 *   citizen-reports     GET /api/v1/citizen-reports?metric=…[&from&to]      (`feedback.read`)
 *   projects            GET /api/v1/investment-projects?year=…[&delayed_only=true]  (`budget.read`)
 *
 * THE ROWS ARE THE FIGURE'S ROWS: the list route's `metric` is the predicate the server counted the
 * figure with (ADR 0053 §7), so the dialog's row count is the figure (prototype `DrillDialog.tsx:33-35`).
 * The description's n is the server's: `citizen-report-counts` under the same filter; the project list
 * is unpaged (all or a refusal); the incoming register has no count route, so n is said only once every
 * page is read — never the length of one page dressed up as the total.
 *
 * `Mở` is a LINK to the register's screen: a petition opens by its `?id=` deep link (`petitionHref`);
 * `/van-ban` and `/giai-ngan` have no per-record deep link, so theirs open the same filtered list the
 * figure's link led to before (`/van-ban?metric=…`) and the Giải ngân screen.
 *
 * Rule 3: a petition's content and a document's summary are free text that can name a citizen. They go
 * into a table cell as text — never an attribute, a title, a URL or a log. `Mở` is named by the CODE.
 */
export function RecordDrillDialog({
  request,
  onClose,
}: {
  request: Exclude<DrillRequest, { list: "tasks" }>;
  onClose: () => void;
}) {
  // Names for the unit and staff columns: both `any-authenticated` reads of this commune, read once.
  const [unitNames, setUnitNames] = useState<ReadonlyMap<string, string>>(new Map());
  const [directoryResult, setDirectoryResult] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  useEffect(() => {
    let cancelled = false;
    layDanhMucBoPhan().then((r) => {
      if (!cancelled && r.ok) setUnitNames(new Map(r.duLieu.items.map((b) => [b.id, b.name])));
    });
    layDanhBaChonNguoi().then((r) => {
      if (!cancelled) setDirectoryResult(r);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  const directory = useMemo(() => danhBaChoNhatKy(directoryResult), [directoryResult]);
  const unit = (id: string | undefined) => (id === undefined || id === "" ? "—" : (unitNames.get(id) ?? "—"));
  const person = (code: string | undefined) => nhanCanBoNgan(code ?? "", directory, CHUA_PHAN_CONG);

  switch (request.list) {
    case "incoming-documents": {
      const { metric, period } = request;
      const listHref = drillHref({ list: "incoming-documents", metric }, period);
      return (
        <PagedDrill<documents_vanBanDenRa>
          readKey={`incoming-documents|${metric}|${period.from}|${period.to}`}
          title={request.label}
          noun="văn bản"
          read={(cursor) =>
            laySoVanBanDen({ metric, from: period.from, to: period.to, limit: DRILL_PAGE_SIZE, cursor })
          }
          count={null}
          toRow={(d) => {
            const code = `${d.number}/${d.year}`;
            return {
              key: d.id,
              code,
              title: d.summary,
              unit: unit(d.holding_unit),
              assignee: person(d.assignee),
              due: nhanNgay(d.due_at),
              open: <OpenLink href={listHref} code={code} />,
            };
          }}
          onClose={onClose}
        />
      );
    }
    case "citizen-reports": {
      const { metric, period } = request;
      const filter = { metric, from: period.from, to: period.to };
      return (
        <PagedDrill<petitions_phieuPhanAnhRa>
          readKey={`citizen-reports|${metric}|${period.from}|${period.to}`}
          title={request.label}
          noun="phản ánh"
          read={(cursor) => laySoPhanAnh({ ...filter, limit: DRILL_PAGE_SIZE, cursor })}
          count={() => countCitizenReports(filter).then((r) => (r.ok ? r.duLieu.total : null))}
          toRow={(p) => ({
            key: p.code,
            code: p.code,
            title: p.content,
            unit: unit(p.unit),
            assignee: person(p.assignee),
            // The commitment the register prints as "Hạn" (`so-phan-anh.tsx`): the resolve deadline,
            // "—" until classification fixes it (rule 10, inv. 2).
            due: nhanNgay(p.resolve_due),
            open: <OpenLink href={petitionHref(p.code)} code={p.code} />,
          })}
          onClose={onClose}
        />
      );
    }
    case "projects": {
      const { year, delayedOnly } = request;
      return (
        <PagedDrill<finance_duAnRa>
          readKey={`projects|${year}|${delayedOnly}`}
          title={request.label}
          noun="dự án"
          headers={PROJECT_HEADERS}
          // Unpaged: the route returns every project of the year or refuses (`du-an.ts`), so one read
          // is the whole list and its length is the count.
          read={() =>
            layDanhSachDuAn({ nam: year, delayedOnly }).then((r) =>
              r.ok ? { ok: true as const, duLieu: { items: r.duLieu.items, next_cursor: "", has_more: false } } : r,
            )
          }
          count={null}
          toRow={(p) => ({
            key: p.id,
            code: p.code,
            title: p.name,
            unit: unit(p.org_unit_id),
            assignee: person(p.assignee_id),
            due: nhanNgay(p.disbursement_deadline),
            open: <OpenLink href={BUDGET_SCREEN_PATH} code={p.code} />,
          })}
          onClose={onClose}
        />
      );
    }
  }
}

/** Giải ngân's columns: a project has a name, an officer in charge and a disbursement deadline. */
export const PROJECT_HEADERS: DrillHeaders = ["Mã", "Tên", "Bộ phận", "Người phụ trách", "Hạn giải ngân"];

function OpenLink({ href, code }: { href: string; code: string }) {
  return (
    <Link href={href} className={DRILL_OPEN_CLASS} aria-label={`${DRILL_OPEN} ${code}`}>
      {DRILL_OPEN}
      <ExternalLink aria-hidden="true" focusable="false" className="size-3" />
    </Link>
  );
}

type Page<T> = { readonly items: readonly T[]; readonly next_cursor: string; readonly has_more: boolean };

/**
 * The description of a register with NO count route: n once every page is read; before that, how many
 * rows are on screen and that more remain — never one page's length passed off as the figure.
 */
export function partialDescription(noun: string, shown: number, complete: boolean): string {
  if (complete) return drillDescriptionOf(noun, shown);
  return `Đang hiện ${formatCount(shown)} ${noun} đầu — bấm "${DRILL_MORE}" để đọc tiếp.`;
}

/** One register's rows, paged by the server's cursor — the task dialog's states, generic over the row. */
function PagedDrill<T>({
  readKey,
  title,
  noun,
  headers = DEFAULT_DRILL_HEADERS,
  read,
  count,
  toRow,
  onClose,
}: {
  readKey: string;
  title: string;
  noun: string;
  headers?: DrillHeaders;
  read: (cursor?: string) => Promise<KetQua<Page<T>>>;
  /** The server's count under the same filter, or `null` when the register has no count route. */
  count: (() => Promise<number | null>) | null;
  toRow: (item: T) => DrillRow;
  onClose: () => void;
}) {
  const [page, setPage] = useState<{ key: string; result: KetQua<Page<T>> } | null>(null);
  const [total, setTotal] = useState<{ key: string; value: number | null } | null>(null);
  const [more, setMore] = useState<{ key: string; items: readonly T[]; cursor: string; hasMore: boolean } | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);

  // `read` and `count` are rebuilt every render; `readKey` names the filter they carry, so it alone
  // decides when to read again.
  useEffect(() => {
    let cancelled = false;
    read().then((result) => {
      if (!cancelled) setPage({ key: readKey, result });
    });
    count?.().then((value) => {
      if (!cancelled) setTotal({ key: readKey, value });
    });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [readKey]);

  const shown = page !== null && page.key === readKey ? page : null;
  const shownTotal = total !== null && total.key === readKey ? total.value : null;
  const extra = more !== null && more.key === readKey ? more : null;
  const first = shown !== null && shown.result.ok ? shown.result.duLieu : null;
  const items = first === null ? [] : [...first.items, ...(extra?.items ?? [])];
  const cursor = extra !== null ? extra.cursor : (first?.next_cursor ?? "");
  const hasMore = (extra !== null ? extra.hasMore : (first?.has_more ?? false)) && cursor !== "";

  function loadMore(): void {
    if (loadingMore || !hasMore || shown === null) return;
    const at = readKey;
    setLoadingMore(true);
    read(cursor).then((r) => {
      setLoadingMore(false);
      if (!r.ok) return;
      setMore((m) => ({
        key: at,
        items: [...(m !== null && m.key === at ? m.items : []), ...r.duLieu.items],
        cursor: r.duLieu.next_cursor,
        hasMore: r.duLieu.has_more,
      }));
    });
  }

  const body: DrillBody =
    shown === null
      ? { kind: "loading" }
      : !shown.result.ok
        ? { kind: "error", message: shown.result.thongBao }
        : { kind: "rows", rows: items.map(toRow) };
  const description =
    shown === null || !shown.result.ok
      ? DRILL_LOADING
      : count !== null
        ? drillDescriptionOf(noun, shownTotal)
        : partialDescription(noun, items.length, !hasMore);

  return (
    <DrillTableDialog
      title={title}
      description={description}
      headers={headers}
      body={body}
      hasMore={hasMore}
      loadingMore={loadingMore}
      onMore={loadMore}
      onClose={onClose}
    />
  );
}
