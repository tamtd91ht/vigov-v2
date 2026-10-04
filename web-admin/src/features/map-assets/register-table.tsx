import { MapPin, MapPinned, Pencil, Trash2 } from "lucide-react";
import { Fragment } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
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
  if (rows.length === 0) return <EmptyState icon={MapPinned} title="Không có địa điểm nào khớp bộ lọc." />;
  const groups = groupRows(rows);
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
            return (
              <Fragment key={g.code}>
                <tr className="register-group-row bg-surface-muted">
                  <th scope="colgroup" colSpan={6} className="text-left">
                    <span className="inline-flex items-center gap-2 text-[13px] font-semibold text-ink-900">
                      <span aria-hidden="true" className={cn("inline-block size-2.5 rounded-full", groupSwatchClass(g.code))} />
                      {groupLabel(g.code)}
                      <span className="font-normal text-ink-500">{`${g.rows.length}${partial ? "+" : ""} địa điểm`}</span>
                    </span>
                  </th>
                </tr>
                {g.rows.map((r) => (
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
                      <span className="flex flex-wrap items-center gap-1">
                        <Button
                          type="button"
                          variant="icon"
                          size="sm"
                          aria-label={`Xem ${r.name} trên bản đồ`}
                          disabled={!canLocate}
                          icon={<MapPin aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                          onClick={() => onLocate(r)}
                        />
                        {canUpdate && (
                          <>
                            <Button
                              type="button"
                              variant="icon"
                              size="sm"
                              aria-label={`Sửa ${r.name}`}
                              icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                              onClick={() => onEdit(r)}
                            />
                            <Button
                              type="button"
                              variant="icon"
                              size="sm"
                              aria-label={`Xoá ${r.name}`}
                              icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                              onClick={() => onDelete(r)}
                            />
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
