import { ChevronDown, ChevronRight, Loader2, MapPin, Pencil, Trash2 } from "lucide-react";
import { Fragment, useId, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS } from "@/components/ui/data-table";
import { cn } from "@/lib/cn";
import type { comms_mapAssetRowOut } from "@/lib/api/schema.gen";

import { verificationBadgeClass } from "./detail-panel";
import {
  REGISTER_EMPTY_FILTERED,
  REGISTER_EMPTY_HINT,
  REGISTER_EMPTY_TITLE,
  ROW_DELETE_CANCEL,
  ROW_DELETE_CONFIRM,
  ROW_DELETE_REASON_LABEL,
  ROW_DELETE_TITLE,
  ROW_EDIT_TITLE,
  ROW_LOCATE_TITLE,
  UNVERIFIED_CHIP,
  VERIFIED_CHIP,
  statusLabel,
} from "./labels";
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

/** The register's card — prototype `AssetRegisterTable.tsx` (spec 06). Shared by the table and its empty state. */
const CARD = "rounded-card border border-line bg-white shadow-card";

/** Row action — prototype `Button size="sm" variant="outline" className="h-8 px-2"`, icon 14px. */
const ACTION = "h-8 px-2";

/**
 * Empty register (spec 06). Filtered → says the FILTER found nothing: "Chưa có địa điểm nào" would tell
 * the officer the commune has no place at all. The "Thêm đối tượng" sentence only for whom the button exists.
 */
export function RegisterEmpty({ filtered, canUpdate }: { filtered: boolean; canUpdate: boolean }) {
  return (
    <div data-register-empty="" className={cn(CARD, "p-12 text-center")}>
      <p className="m-0 text-[14px] font-semibold text-navy">{filtered ? REGISTER_EMPTY_FILTERED : REGISTER_EMPTY_TITLE}</p>
      {!filtered && canUpdate && <p className="m-0 mt-1.5 text-[12.5px] text-ink-muted">{REGISTER_EMPTY_HINT}</p>}
    </div>
  );
}

/**
 * "Sổ địa điểm" (spec 06): a table grouped by group, header row "● {label}  {n} địa điểm".
 * `representative` / `phone` are shown AS THE LIST RETURNS THEM — the list is always masked server-side.
 *
 * The last group may continue on the next page (the server pages in group order): its count then
 * reads "n+" rather than claiming a total this page does not know.
 *
 * Prototype `AssetRegisterTable.tsx`: pressing a group row folds the group (a commune has eleven groups
 * and nobody reads them all at once) — the whole row takes the click, the button inside carries the
 * keyboard and `aria-expanded`. Per row, outline icon buttons — locate, edit, remove. Remove asks ON
 * THE ROW ("Gỡ hẳn" / "Thôi"), as the prototype does, plus a REQUIRED reason on the line under it: a
 * soft delete records why (rule 7, owner 11/10/2026).
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
  /** Soft delete with the officer's reason; `true` once deleted (the screen reloads the register). */
  onDelete: (row: comms_mapAssetRowOut, reason: string) => Promise<boolean>;
}) {
  const reasonId = useId();
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  const [confirming, setConfirming] = useState<{ readonly id: string; readonly reason: string; readonly sending: boolean } | null>(null);
  const groups = groupRows(rows);
  const toggle = (code: string) =>
    setCollapsed((c) => {
      const next = new Set(c);
      if (next.has(code)) next.delete(code);
      else next.add(code);
      return next;
    });

  async function remove(row: comms_mapAssetRowOut) {
    if (confirming === null || confirming.sending) return;
    const reason = confirming.reason.trim();
    if (reason === "") return;
    setConfirming({ ...confirming, sending: true });
    const done = await onDelete(row, reason);
    setConfirming((c) => (done ? null : c === null ? null : { ...c, sending: false }));
  }

  return (
    <div className={cn(CARD, "overflow-x-auto")}>
      <table className={DATA_TABLE_CLASS} aria-label="Sổ địa điểm">
        <thead>
          <tr>
            <th scope="col" className="min-w-72">
              Địa điểm
            </th>
            <th scope="col" className="w-44">
              Thôn / Tổ dân phố
            </th>
            <th scope="col" className="w-40">
              Người đại diện
            </th>
            <th scope="col" className="w-28">
              Trạng thái
            </th>
            <th scope="col" className="w-32">
              Xác minh
            </th>
            <th scope="col" className="w-24">
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
                <tr className="register-group-row cursor-pointer bg-canvas hover:bg-canvas" onClick={() => toggle(g.code)}>
                  <th scope="colgroup" colSpan={6} className="p-0 text-left">
                    {/* No onClick of its own: its click (mouse, Enter, Space) bubbles to the row. */}
                    <button
                      type="button"
                      aria-expanded={!shut}
                      data-group-toggle={g.code}
                      className="flex min-h-10 w-full cursor-pointer items-center gap-2 border-0 bg-transparent px-3 py-2 text-left [font-family:inherit] text-[12px] font-bold text-navy focus-visible:outline-2 focus-visible:outline-brand-500"
                    >
                      <Chevron aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
                      <span aria-hidden="true" className={cn("inline-block size-2.5 shrink-0 rounded-full", groupSwatchClass(g.code))} />
                      <span className="min-w-0">{groupLabel(g.code)}</span>
                      <span className="font-normal text-ink-muted">{`${g.rows.length}${partial ? "+" : ""} địa điểm`}</span>
                    </button>
                  </th>
                </tr>
                {!shut &&
                  g.rows.map((r) => {
                    const asking = confirming !== null && confirming.id === r.id ? confirming : null;
                    return (
                      <Fragment key={r.id}>
                        <tr>
                          <td className="py-2 whitespace-normal">
                            <span className="text-[12.5px] font-semibold text-navy">{r.name}</span>
                            {(r.address ?? "") !== "" && <span className="block text-[11px] text-ink-muted">{r.address}</span>}
                          </td>
                          <td className="py-2 text-[11.5px] text-ink-muted">
                            {r.residential_unit_id ? unitName(r.residential_unit_id) : "—"}
                          </td>
                          <td className="py-2 text-[11.5px] text-ink-muted">
                            <span>{(r.representative ?? "") === "" ? "—" : r.representative}</span>
                            {(r.phone ?? "") !== "" && <span className="block tabular-nums">{r.phone}</span>}
                          </td>
                          <td className="py-2 text-[11.5px]">{statusLabel(r.status)}</td>
                          <td className="py-2">
                            <Badge data-badge="" icon={null} className={cn(verificationBadgeClass(r.verified), "text-[10.5px]")}>
                              {r.verified ? VERIFIED_CHIP : UNVERIFIED_CHIP}
                            </Badge>
                          </td>
                          <td className="py-2">
                            <span className="flex items-center justify-end gap-1.5">
                              {asking !== null ? (
                                <>
                                  <Button
                                    type="button"
                                    variant="primary"
                                    size="sm"
                                    className="h-8 bg-danger px-2 text-[11px] text-white hover:not-disabled:bg-danger/90"
                                    disabled={asking.sending || asking.reason.trim() === ""}
                                    aria-busy={asking.sending}
                                    icon={
                                      asking.sending ? (
                                        <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
                                      ) : undefined
                                    }
                                    onClick={() => void remove(r)}
                                  >
                                    {ROW_DELETE_CONFIRM}
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    className="h-8 px-2 text-[11px]"
                                    disabled={asking.sending}
                                    onClick={() => setConfirming(null)}
                                  >
                                    {ROW_DELETE_CANCEL}
                                  </Button>
                                </>
                              ) : (
                                <>
                                  <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    className={ACTION}
                                    title={ROW_LOCATE_TITLE}
                                    aria-label={`Xem ${r.name} trên bản đồ`}
                                    disabled={!canLocate}
                                    onClick={() => onLocate(r)}
                                  >
                                    <MapPin aria-hidden="true" focusable="false" className="size-3.5" />
                                  </Button>
                                  {canUpdate && (
                                    <>
                                      <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        className={ACTION}
                                        title={ROW_EDIT_TITLE}
                                        aria-label={`Sửa ${r.name}`}
                                        onClick={() => onEdit(r)}
                                      >
                                        <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
                                      </Button>
                                      <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        className={cn(ACTION, "text-danger hover:not-disabled:text-danger")}
                                        title={ROW_DELETE_TITLE}
                                        aria-label={`Gỡ ${r.name} khỏi bản đồ`}
                                        onClick={() => setConfirming({ id: r.id, reason: "", sending: false })}
                                      >
                                        <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
                                      </Button>
                                    </>
                                  )}
                                </>
                              )}
                            </span>
                          </td>
                        </tr>
                        {asking !== null && (
                          <tr data-row-delete="">
                            <td colSpan={6} className="bg-danger/6 py-2">
                              <label htmlFor={reasonId} className="block text-[11.5px] font-medium text-ink-900">
                                {ROW_DELETE_REASON_LABEL}
                              </label>
                              <input
                                id={reasonId}
                                data-row-delete-reason=""
                                name="reason"
                                required
                                // The officer just pressed "Gỡ": the reason is the next thing to type.
                                autoFocus
                                value={asking.reason}
                                disabled={asking.sending}
                                onChange={(e) => setConfirming({ ...asking, reason: e.target.value })}
                                onKeyDown={(e) => {
                                  if (e.key === "Enter") void remove(r);
                                  if (e.key === "Escape") setConfirming(null);
                                }}
                                className="mt-1 block h-8 w-full max-w-md rounded-md border border-danger/30 bg-white px-2.5 [font-family:inherit] text-base text-ink-900 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-[12px]"
                              />
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    );
                  })}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
