"use client";

import { useEffect, useRef, useState } from "react";

import { usePhien } from "@/features/session/current-session";
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
import { auditLogTabDecision } from "./tab-permissions";

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
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền xem nhật ký hệ thống, nên tab này không hiển thị.
      </p>
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

  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-nhat-ky">
      <h2 id="tieu-de-nhat-ky">Nhật ký hệ thống</h2>
      <p className="ghi-chu">
        Vết thao tác trong đơn vị ở năm phân hệ, mới nhất trước. Nhật ký chỉ để đọc: không mục nào sửa
        hay xoá được. Mỗi lần mở hoặc lọc nhật ký cũng được ghi lại người xem.
      </p>

      <form
        className="form-danh-muc"
        aria-label="Lọc nhật ký hệ thống"
        onSubmit={(e) => {
          e.preventDefault();
          onFilter();
        }}
      >
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-tu">Từ ngày</label>
          <input
            id="loc-nhat-ky-tu"
            type="date"
            value={draft.fromDate}
            onChange={(e) => setDraft({ ...draft, fromDate: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-den">Đến hết ngày</label>
          <input
            id="loc-nhat-ky-den"
            type="date"
            value={draft.toDate}
            onChange={(e) => setDraft({ ...draft, toDate: e.target.value })}
          />
        </div>
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
        <div className="o-nhap">
          <label htmlFor="loc-nhat-ky-doi-tuong">Đối tượng</label>
          <input
            id="loc-nhat-ky-doi-tuong"
            value={draft.subject}
            placeholder="Mã tra cứu, số văn bản, mã cán bộ…"
            autoComplete="off"
            onChange={(e) => setDraft({ ...draft, subject: e.target.value })}
          />
        </div>
        <div className="cum-nut">
          <button type="submit" className="nut-chinh">
            Lọc
          </button>
          <button type="button" className="nut-phu" onClick={onClear}>
            Bỏ lọc
          </button>
        </div>
        {filterError !== null && (
          <p className="thong-bao-loi" role="alert">
            {filterError}
          </p>
        )}
      </form>

      {/* §7: every failed module is NAMED, above the list, so the list never reads as complete. */}
      {failed.map((s) => (
        <p key={s.key} className="thong-bao-loi" role="alert">
          {sourceErrorLine(s.key, s.error ?? "")}{" "}
          <button type="button" className="nut-phu" disabled={s.loading} onClick={() => onLoad(s.key)}>
            Tải lại
          </button>
        </p>
      ))}

      {boundary.kind === "blocked" && anyLoading && <p role="status">Đang tải nhật ký…</p>}

      {rows.length === 0 && boundary.kind !== "blocked" && !anyLoading && (
        <p className="trang-thai-rong">Không có mục nhật ký nào khớp bộ lọc.</p>
      )}

      {rows.length > 0 && (
        <div className="bang-cuon" role="region" aria-label="Các mục nhật ký hệ thống" tabIndex={0}>
          <table className="bang-danh-muc">
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
        </div>
      )}

      {held > 0 && (
        <p className="ghi-chu">
          Còn {held} mục đã tải đang chờ phân hệ khác để xếp đúng thứ tự thời gian — bấm Xem thêm.
        </p>
      )}

      {next !== null && boundary.kind !== "blocked" && (
        <div className="cum-nut">
          <button
            type="button"
            className="nut-phu"
            disabled={sources.find((s) => s.key === next)?.loading ?? true}
            onClick={() => onLoad(next)}
          >
            Xem thêm
          </button>
        </div>
      )}
    </section>
  );
}
