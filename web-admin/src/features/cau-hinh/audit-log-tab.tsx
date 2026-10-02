"use client";

import { ChevronDown, FileClock, Filter, RotateCw, SearchX, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { FilterBar } from "@/components/ui/filter-bar";
import { NoAccess } from "@/components/ui/no-access";
import { SkeletonRows } from "@/components/ui/skeleton";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { getAuditEntries, type AuditFilter, type AuditSourceKey } from "@/lib/api/audit-entries";

import {
  applyPage,
  heldBackCount,
  initialSources,
  markLoading,
  nextToLoad,
  safeBoundary,
  visibleRows,
  type SourceState,
} from "./audit-log-merge";
import {
  actorLabel,
  atLabel,
  AUDIT_SOURCE_LABEL,
  deltaText,
  EMPTY_AUDIT_FILTER,
  filterFromDraft,
  ipLabel,
  sourceErrorLine,
  type AuditFilterDraft,
} from "./audit-log-view";
import { auditLogTabDecision } from "./quyen-tab";

/**
 * "Cấu hình → Nhật ký hệ thống" (§12.1, ADR 0054). One key, `admin.audit`, on all five reads — the
 * tab hides as a whole without it (convenience; each service refuses on its own).
 *
 * FIVE REQUESTS PER PAGE-ONE, one per service, merged here (`audit-log-merge.ts`). Each read also
 * writes one `xem_nhat_ky_he_thong` entry in that service (ADR 0054 §5), which is why the tab loads
 * only when opened by an allowed account and never polls.
 *
 * A RESPONSE FROM AN OLDER FILTER IS DROPPED (`generation`): five services answer in any order, and a
 * page of the previous filter folded into the new list is a row that does not match what staff asked.
 */
export function AuditLogTab() {
  const phien = usePhien();
  const decision = phien === null ? null : auditLogTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [draft, setDraft] = useState<AuditFilterDraft>(EMPTY_AUDIT_FILTER);
  const [filter, setFilter] = useState<AuditFilter>({});
  const [filterError, setFilterError] = useState<string | null>(null);
  const [sources, setSources] = useState<SourceState[]>(initialSources);
  const generation = useRef(0);

  // Page one of the empty filter, once, when the tab becomes allowed. Later loads start from the
  // filter form's submit, not from here.
  useEffect(() => {
    if (allowed) loadAll(generation, setSources, {});
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền xem nhật ký hệ thống, nên tab này không hiển thị.
        </p>
      </div>
    );
  }

  function applyFilter() {
    const r = filterFromDraft(draft);
    if (!r.ok) {
      setFilterError(r.message);
      return;
    }
    setFilterError(null);
    setFilter(r.filter);
    setSources(initialSources());
    loadAll(generation, setSources, r.filter);
  }

  function clearFilter() {
    setDraft(EMPTY_AUDIT_FILTER);
    setFilterError(null);
    setFilter({});
    setSources(initialSources());
    loadAll(generation, setSources, {});
  }

  function loadMore(key: AuditSourceKey) {
    const s = sources.find((x) => x.key === key);
    if (s === undefined || s.loading) return;
    setSources((all) => markLoading(all, key));
    loadPage(generation, generation.current, setSources, key, filter, s.cursor);
  }

  return (
    <AuditLogView
      sources={sources}
      draft={draft}
      setDraft={setDraft}
      filterError={filterError}
      onFilter={applyFilter}
      onClear={clearFilter}
      onLoad={loadMore}
    />
  );
}

type Generation = { current: number };
type SetSources = (update: (s: SourceState[]) => SourceState[]) => void;

/** One page of one source, folded in only if no newer filter started meanwhile. */
function loadPage(
  generation: Generation,
  gen: number,
  setSources: SetSources,
  key: AuditSourceKey,
  f: AuditFilter,
  cursor: string,
) {
  void getAuditEntries(key, f, cursor).then((r) => {
    if (gen !== generation.current) return;
    setSources((s) => applyPage(s, key, r));
  });
}

/** Page one of all five sources under a new generation. */
function loadAll(generation: Generation, setSources: SetSources, f: AuditFilter) {
  generation.current += 1;
  const gen = generation.current;
  for (const s of initialSources()) loadPage(generation, gen, setSources, s.key, f, "");
}

/** Pure rendering, exported so the tests read the markup of each merge state. */
export function AuditLogView({
  sources,
  draft,
  setDraft,
  filterError,
  onFilter,
  onClear,
  onLoad,
}: {
  sources: readonly SourceState[];
  draft: AuditFilterDraft;
  setDraft: (d: AuditFilterDraft) => void;
  filterError: string | null;
  onFilter: () => void;
  onClear: () => void;
  onLoad: (key: AuditSourceKey) => void;
}) {
  const rows = visibleRows(sources);
  const boundary = safeBoundary(sources);
  const next = nextToLoad(sources);
  const held = heldBackCount(sources);
  const failed = sources.filter((s) => s.error !== null);
  const anyLoading = sources.some((s) => s.loading);
  // How many of the filters behind the "Bộ lọc" button are set (ADR 0068 §12) — read from the draft,
  // so a hidden filter in use is never a filter nobody can see is on.
  const moreActive = [draft.actor, draft.action].filter((v) => v.trim() !== "").length;

  const fromDate = (
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-tu">Từ ngày</label>
          <input
            id="loc-nhat-ky-tu"
            type="date"
            value={draft.fromDate}
            onChange={(e) => setDraft({ ...draft, fromDate: e.target.value })}
          />
        </div>
  );
  const toDate = (
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-den">Đến hết ngày</label>
          <input
            id="loc-nhat-ky-den"
            type="date"
            value={draft.toDate}
            onChange={(e) => setDraft({ ...draft, toDate: e.target.value })}
          />
        </div>
  );
  const actorAndAction = (
      <>
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-nguoi">Người thực hiện</label>
          <input
            id="loc-nhat-ky-nguoi"
            value={draft.actor}
            placeholder="Mã cán bộ, hoặc system"
            autoComplete="off"
            onChange={(e) => setDraft({ ...draft, actor: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-thao-tac">Thao tác</label>
          <input
            id="loc-nhat-ky-thao-tac"
            value={draft.action}
            autoComplete="off"
            aria-describedby="giai-thich-loc-thao-tac"
            onChange={(e) => setDraft({ ...draft, action: e.target.value })}
          />
          <p className="ghi-chu" id="giai-thich-loc-thao-tac">
            Nhập đúng mã thao tác như trong cột Thao tác. Lượt xem nhật ký (xem_nhat_ky_he_thong) chỉ
            hiện khi lọc đúng mã ấy.
          </p>
        </div>
      </>
  );
  const subject = (
        <div className="o-nhap flex-[1_1_320px]">
          <label htmlFor="loc-nhat-ky-doi-tuong">Đối tượng</label>
          <input
            id="loc-nhat-ky-doi-tuong"
            value={draft.subject}
            placeholder="Mã tra cứu, số văn bản, mã cán bộ…"
            autoComplete="off"
            onChange={(e) => setDraft({ ...draft, subject: e.target.value })}
          />
        </div>
  );

  return (
    <section
      className="tab-danh-muc m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm"
      aria-labelledby="tieu-de-nhat-ky"
    >
      <CardHeader className="m-0">
        <div className="min-w-0 flex-1 basis-64">
          <CardTitle as="h2" id="tieu-de-nhat-ky" className="flex items-center gap-2">
            <FileClock aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
            Nhật ký hệ thống
          </CardTitle>
          <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">
            Vết thao tác trong đơn vị ở năm phân hệ, mới nhất trước. Nhật ký chỉ để đọc: không mục nào sửa
            hay xoá được. Mỗi lần mở hoặc lọc nhật ký cũng được ghi lại người xem.
          </p>
        </div>
      </CardHeader>

      {/* Five controls → the shared FilterBar (ADR 0068 §12): the search-like box first, the two dates,
          the rest behind "Bộ lọc". Every control stays inside the same `<form>`, so submit is unchanged. */}
      <form
        className="form-danh-muc m-0 flex min-w-0 flex-col gap-3 rounded-none border-0 border-b border-line px-4 py-3.5 shadow-none [&>*]:my-0"
        aria-label="Lọc nhật ký hệ thống"
        onSubmit={(e) => {
          e.preventDefault();
          onFilter();
        }}
      >
        <FilterBar
          id="loc-nhat-ky"
          primary={
            <>
              {subject}
              {fromDate}
              {toDate}
            </>
          }
          more={actorAndAction}
          moreActiveCount={moreActive}
        />
        <div className="cum-nut flex flex-wrap justify-end gap-2">
          <Button type="button" variant="ghost" icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />} onClick={onClear}>
            Bỏ lọc
          </Button>
          <Button type="submit" variant="primary" icon={<Filter aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Lọc
          </Button>
        </div>
        {filterError !== null && (
          <p className="thong-bao-loi" role="alert">
            {filterError}
          </p>
        )}
      </form>

      {/* §7: every failed module is NAMED, above the list, so the list never reads as complete. */}
      <div className="flex min-w-0 flex-col gap-3 px-4 pt-3 empty:hidden [&>*]:my-0">
      {failed.map((s) => (
        <p key={s.key} className="thong-bao-loi flex flex-wrap items-center gap-2" role="alert">
          {sourceErrorLine(s.key, s.error ?? "")}{" "}
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<RotateCw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            disabled={s.loading}
            onClick={() => onLoad(s.key)}
          >
            Tải lại
          </Button>
        </p>
      ))}
      </div>

      {boundary.kind === "blocked" && anyLoading && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải nhật ký…
          </p>
          <SkeletonRows rows={5} />
        </>
      )}

      {rows.length === 0 && boundary.kind !== "blocked" && !anyLoading && (
        <EmptyState icon={SearchX} title="Không có mục nhật ký nào khớp bộ lọc." />
      )}

      {rows.length > 0 && (
        <TableScroll sticky aria-label="Các mục nhật ký hệ thống" className="mt-3 rounded-none border-0 border-t border-line shadow-none">
          <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
            <thead>
              <tr>
                <th scope="col">Thời điểm</th>
                <th scope="col">Phân hệ</th>
                <th scope="col">Người thực hiện</th>
                <th scope="col">Thao tác</th>
                <th scope="col">Đối tượng</th>
                <th scope="col">Địa chỉ IP</th>
                <th scope="col">Chi tiết</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(({ source, entry, rowKey }) => {
                const delta = deltaText(entry.delta);
                return (
                  <tr key={rowKey}>
                    <td>{atLabel(entry.at)}</td>
                    <td>{AUDIT_SOURCE_LABEL[source]}</td>
                    <td>{actorLabel(entry)}</td>
                    <td className="ma-muc">{entry.action}</td>
                    <td className="ma-muc">{entry.subject !== "" ? entry.subject : "—"}</td>
                    <td>{ipLabel(entry)}</td>
                    <td>
                      {delta === null ? (
                        "Không có"
                      ) : (
                        // TEXT, never HTML: the delta is what staff typed (rule 13, invariant 3).
                        <details>
                          <summary>Xem</summary>
                          <pre className="chi-tiet-nhat-ky">{delta}</pre>
                        </details>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </TableScroll>
      )}

      {held > 0 && (
        <p className="ghi-chu m-0 px-4 pt-3 text-[13px] text-ink-500">
          Còn {held} mục đã tải đang chờ phân hệ khác để xếp đúng thứ tự thời gian — bấm Xem thêm.
        </p>
      )}

      {next !== null && boundary.kind !== "blocked" && (
        <div className="cum-nut flex justify-center border-t border-line px-4 py-3">
          <Button
            type="button"
            variant="secondary"
            icon={<ChevronDown aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            disabled={sources.find((s) => s.key === next)?.loading ?? true}
            onClick={() => onLoad(next)}
          >
            Xem thêm
          </Button>
        </div>
      )}
    </section>
  );
}
