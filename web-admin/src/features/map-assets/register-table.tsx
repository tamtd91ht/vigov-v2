import { ChevronDown, ChevronRight, MapPin, MapPinned, Pencil, Trash2 } from "lucide-react";
import { Fragment, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { IconButton } from "@/components/ui/icon-button";
import { cn } from "@/lib/cn";
import type { comms_mapAssetRowOut } from "@/lib/api/schema.gen";

import { UNVERIFIED_CHIP, VERIFIED_CHIP, statusLabel } from "./labels";
import { groupSwatchClass } from "./map-logic";

/** Rows grouped by `asset_type_code`, in the server's order (group, then name). */
export function groupRows(rows: readonly comms_mapAssetRowOut[]): { code: string; rows: comms_mapAssetRowOut[] }[] {
  const out: { code: string; rows: comms_mapAssetRowOut[] }[] = [];
  for (const r of rows) {
    const last = out[out.length - 1];
    if (last !== undefined && last.code === r.asset_type_code) last.rows.push(r);
    else out.push({ code: r.asset_type_code, rows: [r] });
  }
  return out;
}

/**
 * "Sổ địa điểm" (spec §7): a table grouped by group, header row "● {label}  {n} địa điểm".
 * `representative` / `phone` are shown AS THE LIST RETURNS THEM — the list is always masked server-side.
 *
 * The last group may continue on the next page (the server pages in group order): its count then
 * reads "n+" rather than claiming a total this page does not know.
 *
 * Prototype `AssetRegisterTable.tsx`: pressing a group header folds the group (a commune has eleven
 * groups and nobody reads them all at once); per row, outline icon buttons — locate, edit, delete.
 * Delete still opens the reason dialog (rule 7), never deletes on the row.
 */
export function RegisterTable({
  rows,
  hasMore,
  groupLabel,
  unitName,
  canUpdate,
  canLocate,
  onLocate,
  onEdit,
  onDelete,
}: {
  rows: readonly comms_mapAssetRowOut[];
  hasMore: boolean;
  groupLabel: (code: string) => string;
  unitName: (id: string) => string;
  canUpdate: boolean;
  /** A map exists to locate on (frame set and basemap configured). */
  canLocate: boolean;
  onLocate: (row: comms_mapAssetRowOut) => void;
  onEdit: (row: comms_mapAssetRowOut) => void;
  onDelete: (row: comms_mapAssetRowOut) => void;
}) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  if (rows.length === 0) return <EmptyState icon={MapPinned} title="Không có địa điểm nào khớp bộ lọc." />;
  const groups = groupRows(rows);
  const toggle = (code: string) =>
    setCollapsed((c) => {
      const next = new Set(c);
      if (next.has(code)) next.delete(code);
      else next.add(code);
      return next;
    });
  return (
    <TableScroll sticky aria-label="Sổ địa điểm">
      <table className={DATA_TABLE_CLASS}>
        <thead>
          <tr>
            <th scope="col">Địa điểm</th>
            <th scope="col">Thôn / Tổ dân phố</th>
            <th scope="col">Người đại diện</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Xác minh</th>
            <th scope="col">
              <span className="an-thi-giac">Thao tác</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {groups.map((g, gi) => {
            const partial = hasMore && gi === groups.length - 1;
            const shut = collapsed.has(g.code);
            const Chevron = shut ? ChevronRight : ChevronDown;
            return (
              <Fragment key={g.code}>
                <tr className="register-group-row bg-surface-muted">
                  <th scope="colgroup" colSpan={6} className="p-0 text-left">
                    <button
                      type="button"
                      aria-expanded={!shut}
                      data-group-toggle={g.code}
                      onClick={() => toggle(g.code)}
                      className="flex min-h-10 w-full cursor-pointer items-center gap-2 border-0 bg-transparent px-3 py-2 text-left text-[13px] font-semibold text-ink-900 focus-visible:outline-2 focus-visible:outline-brand-500"
                    >
                      <Chevron aria-hidden="true" className="size-3.5 shrink-0" />
                      <span aria-hidden="true" className={cn("inline-block size-2.5 shrink-0 rounded-full", groupSwatchClass(g.code))} />
                      <span className="min-w-0">{groupLabel(g.code)}</span>
                      <span className="font-normal text-ink-500">{`${g.rows.length}${partial ? "+" : ""} địa điểm`}</span>
                    </button>
                  </th>
                </tr>
                {!shut && g.rows.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <span className="font-semibold text-ink-900">{r.name}</span>
                      {(r.address ?? "") !== "" && <span className="block text-xs text-ink-500">{r.address}</span>}
                    </td>
                    <td>{r.residential_unit_id ? unitName(r.residential_unit_id) : "—"}</td>
                    <td>
                      {(r.representative ?? "") === "" && (r.phone ?? "") === "" ? (
                        "—"
                      ) : (
                        <>
                          <span>{(r.representative ?? "") === "" ? "—" : r.representative}</span>
                          {(r.phone ?? "") !== "" && <span className="block text-xs text-ink-500">{r.phone}</span>}
                        </>
                      )}
                    </td>
                    <td>{statusLabel(r.status)}</td>
                    <td>
                      <Badge tone={r.verified ? "success" : "neutral"}>{r.verified ? VERIFIED_CHIP : UNVERIFIED_CHIP}</Badge>
                    </td>
                    <td>
                      <span className="flex items-center justify-end gap-1.5">
                        <IconButton type="button" variant="secondary" label={`Xem ${r.name} trên bản đồ`} disabled={!canLocate} onClick={() => onLocate(r)}>
                          <MapPin aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        </IconButton>
                        {canUpdate && (
                          <>
                            <IconButton type="button" variant="secondary" label={`Sửa ${r.name}`} onClick={() => onEdit(r)}>
                              <Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />
                            </IconButton>
                            <IconButton
                              type="button"
                              variant="secondary"
                              className="text-danger-600 hover:not-disabled:bg-danger-50 hover:not-disabled:text-danger-600"
                              label={`Xoá ${r.name}`}
                              onClick={() => onDelete(r)}
                            >
                              <Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />
                            </IconButton>
                          </>
                        )}
                      </span>
                    </td>
                  </tr>
                ))}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}
