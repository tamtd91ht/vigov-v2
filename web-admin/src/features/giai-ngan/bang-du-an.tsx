"use client";

import { FolderKanban, Gauge, Search, SearchX, Trash2, TrendingDown } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { PendingCell, PendingColumnHeader } from "@/components/ui/pending-feature";
import { SkeletonRows } from "@/components/ui/skeleton";
import { layDanhSachDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type {
  finance_danhSachDuAnRa,
  finance_duAnRa,
  finance_fundingStatusOut,
  finance_hangMucRa,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  hangMucDuAn,
  lopHangMuc,
  nhanHangMuc,
  nhanNamRong,
  nhanNgay,
  nhanNguongCham,
  nhanTien,
  nhanTienDo,
  nhanTyLeGiaiNgan,
  tienDoDuAn,
} from "./nhan-du-an";
import { pendingPart } from "./nhan-ghi-giai-ngan";
import { DisbursementOverviewPending, LATEST_ISSUE_COLUMN, ProjectFilterPending, UNIT_OWNER } from "./pending-parts";
import { ProjectBulkDeleteDialog } from "./project-bulk-delete-dialog";
import { BULK_DELETE_BUTTON, selectedProjectsLabel, type SelectedProject } from "./project-bulk-delete";
import { Glyph } from "./project-ui";
import { ScopeNotice } from "./scope-notice";

/**
 * The register of the Giải ngân screen, in the prototype's order (`BudgetWorkspace.tsx:185-407`, ADR
 * 0068 lần 5): scope banner · four KPI cards · cumulative chart · per-category table · per-source
 * block · ONE filter row · the project table.
 *
 * WHAT THE PROTOTYPE DRAWS THAT THIS DOES NOT FILL, and why — read before adding a figure:
 *
 *   KPI cards · cumulative chart · per-category block · the `Đơn vị / phụ trách` and
 *   `Vướng mắc mới nhất` columns · `Chỉ dự án chậm` · `Gộp theo hạng mục`
 *
 * All are disabled "?" placeholders at their prototype position (ADR 0068 §14, `pending-parts.tsx`);
 * none shows a figure. The per-source block is live and passed in (`fundingProgress`), and so is the
 * `Nguồn vốn` column (`FundingChip`, from the list's `funding_status`). No route returns year totals
 * or issue data, and `org_unit_id` / `assignee_id` arrive as internal ids. Drawing "0 vướng mắc" would tell leadership a figure nobody
 * measured.
 *
 * KHÔNG GỘP THEO HẠNG MỤC (prototype bật mặc định): gộp cần tổng theo nhóm, và tổng ấy phải cộng ở
 * máy chủ trên nguyên tập dự án. So the category a project belongs to is on its row instead, under
 * its name — the information the group header would carry.
 *
 * THE SEARCH BOX FILTERS THE LIST ALREADY LOADED, and that is exact, not an approximation: the list
 * route returns the WHOLE year or refuses (`lib/api/du-an.ts`, no pagination), and the box only hides
 * rows — it sums nothing.
 */

type TrangThaiBang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: finance_danhSachDuAnRa };

export function BangDuAn({
  nam,
  danhMuc,
  reloadSignal,
  emptyAction,
  fundingProgress,
  canDelete = false,
}: {
  /** Budget year chosen in the page header. */
  nam: number;
  danhMuc: readonly finance_hangMucRa[];
  /** Bumped by the page after a project is added: the list re-reads. */
  reloadSignal: number;
  /** `+ Thêm dự án` repeated inside the empty state; `null` for an account without the key. */
  emptyAction?: ReactNode;
  /** §6 block (`FundingSourceProgress`), drawn after the category table as the prototype orders it. */
  fundingProgress?: ReactNode;
  /**
   * `budget.update` held (user decision 06/10/2026): the checkbox column, select-all and `Xoá đã chọn`.
   * `false` by default — a caller that forgets it draws fewer controls, never more. UX only: the
   * DELETE route checks its own key on every call (rule 5).
   */
  canDelete?: boolean;
}) {
  const [hangMucId, datHangMucId] = useState("");
  const [keyword, setKeyword] = useState("");

  /**
   * KẾT QUẢ ĐƯỢC LƯU KÈM BỘ LỌC ĐÃ SINH RA NÓ, và "đang tải" được SUY RA từ chỗ hai bộ lọc lệch
   * nhau — không phải đặt bằng một `setState` ngay trong thân effect.
   *
   * Không chỉ để hết lỗi lint: cách viết kia có một cửa sổ, dù hẹp, ở đó bảng của năm cũ vẫn
   * đứng trên màn hình dưới ô chọn đã hiện năm mới. Một bảng tiền của năm 2025 nằm dưới dòng
   * chữ "2026" là con số sai được đọc thành con số đúng.
   */
  const [daTai, datDaTai] = useState<{ khoa: string; kq: KetQua<finance_danhSachDuAnRa> } | null>(
    null,
  );

  /**
   * Bộ đếm lần tải ("Tải lại" after an error); `reloadSignal` is the page's count of added projects.
   *
   * ĐỌC LẠI CẢ DANH SÁCH, KHÔNG VÁ HÀNG MỚI VÀO: phản hồi của tuyến thêm (`duAnGhiRa`) cố ý KHÔNG
   * mang `disbursed_amount`, `disbursed_ratio`, `delay_score` hay `is_delayed` — tuyến ghi không
   * đọc chúng. Vá một hàng dựng từ phản hồi ấy sẽ đặt bốn ô trống hoặc bốn số 0 vào một bảng mà
   * mọi hàng khác đang mang số thật, và chúng trông y hệt nhau.
   */
  const [lanTai, datLanTai] = useState(0);
  const khoa = `${nam}|${hangMucId}|${lanTai}|${reloadSignal}`;

  useEffect(() => {
    let bo = false;
    layDanhSachDuAn({ nam, hangMucId }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam, hangMucId, khoa]);

  /**
   * Selected project ids, TIED TO THE LOAD THAT DREW THEM: a new year, a new category filter or a
   * re-read after a delete makes `khoa` differ, and the selection reads as empty — no effect clearing
   * it, and no way for a tick made on the 2025 list to delete from the 2026 one.
   */
  const [picked, setPicked] = useState<{ khoa: string; ids: ReadonlySet<string> } | null>(null);
  const selectedIds: ReadonlySet<string> = picked !== null && picked.khoa === khoa ? picked.ids : EMPTY_IDS;
  /** Snapshot of the rows sent to the confirm dialog, or `null` when it is closed. */
  const [bulkRows, setBulkRows] = useState<readonly SelectedProject[] | null>(null);

  const trangThai: TrangThaiBang =
    daTai === null || daTai.khoa !== khoa
      ? { pha: "dangTai" }
      : daTai.kq.ok
        ? { pha: "xong", duLieu: daTai.kq.duLieu }
        : { pha: "loi", thongBao: daTai.kq.thongBao };

  const shown =
    trangThai.pha === "xong"
      ? { ...trangThai.duLieu, items: matchKeyword(trangThai.duLieu.items, keyword) }
      : null;

  // From the WHOLE loaded list, not only the rows the search box leaves: a ticked row hidden by a
  // keyword is still selected, and the confirm dialog lists it by name before anything is sent.
  const selectedRows: finance_duAnRa[] =
    trangThai.pha === "xong" ? trangThai.duLieu.items.filter((d) => selectedIds.has(d.id)) : [];

  const selection: RowSelection | undefined =
    canDelete && shown !== null
      ? {
          has: (id) => selectedIds.has(id),
          toggle: (id) => {
            const next = new Set(selectedIds);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            setPicked({ khoa, ids: next });
          },
          // Select-all acts on the rows ON SCREEN: ticking it never selects a row the clerk cannot see.
          toggleAll: () => {
            const next = new Set(selectedIds);
            const allOn = shown.items.every((d) => next.has(d.id));
            for (const d of shown.items) {
              if (allOn) next.delete(d.id);
              else next.add(d.id);
            }
            setPicked({ khoa, ids: next });
          },
        }
      : undefined;

  return (
    <section className="flex min-w-0 flex-col" aria-label="Theo dõi giải ngân theo dự án">
      {/* BANNER BẮT BUỘC (§1) — câu của MÁY CHỦ (`scope_notice`, xã sửa được ở Lời hệ thống), nên nó
          chỉ hiện khi danh sách đã về. Không câu dự phòng ở client: xem `scope-notice.tsx`. */}
      {trangThai.pha === "xong" && (
        <div className="mb-5">
          <ScopeNotice text={trangThai.duLieu.scope_notice} />
        </div>
      )}

      <DisbursementOverviewPending />
      {fundingProgress}

      {/* ONE filter row (prototype `:271-359`): search, category, the two checkboxes. The budget
          year is in the page header, as the prototype puts it. */}
      <div className="mb-4 flex min-w-0 flex-wrap items-center gap-2.5">
        <Field label="Tìm dự án" hideLabel htmlFor="tim-du-an" icon={Search} grow="auto" className="w-64 max-w-full">
          <input
            id="tim-du-an"
            type="search"
            value={keyword}
            autoComplete="off"
            placeholder="Tìm theo tên hoặc mã dự án…"
            onChange={(e) => setKeyword(e.target.value)}
          />
        </Field>

        <Field label="Lọc theo hạng mục" hideLabel htmlFor="loc-hang-muc" kind="select" grow="auto">
          <select
            id="loc-hang-muc"
            value={hangMucId}
            onChange={(e) => datHangMucId(e.target.value)}
            // Danh mục rỗng là đường THÔNG THƯỜNG hôm nay (danh mục ship rỗng), nên ô chọn chỉ còn
            // một lựa chọn "Tất cả" — tắt nó đi để không mời cán bộ bấm vào một ô không lọc được gì.
            disabled={danhMuc.length === 0}
          >
            <option value="">Tất cả hạng mục</option>
            {danhMuc.map((h) => (
              <option key={h.id} value={h.id}>
                {h.label}
              </option>
            ))}
          </select>
        </Field>

        {/* The prototype's unit filter shows only once a project names a unit; no project carries a
            unit NAME here (ids only), so — by the prototype's own rule — it is not drawn. */}

        <ProjectFilterPending />

        {/* `Đã chọn N dự án · Xoá đã chọn` at the right of the filter row, as the prototype puts it
            (`:343-358`): after ticking rows the eye is on the table, not at the foot of the page. */}
        {canDelete && selectedRows.length > 0 && (
          <div className="ml-auto flex items-center gap-2" data-bulk-bar="">
            <span className="text-[13px] text-ink-500">{selectedProjectsLabel(selectedRows.length)}</span>
            <Button
              type="button"
              variant="danger"
              size="sm"
              icon={<Glyph icon={Trash2} />}
              aria-haspopup="dialog"
              onClick={() =>
                setBulkRows(
                  selectedRows.map((d) => ({ id: d.id, code: d.code, name: d.name, planned_amount: d.planned_amount })),
                )
              }
            >
              {BULK_DELETE_BUTTON}
            </Button>
          </div>
        )}
      </div>

      {trangThai.pha === "dangTai" && (
        // FIRST LOAD (spec §8b): the sentence stays the live region, read out as before; the eye
        // gets row-shaped placeholders so the page does not jump when the list arrives.
        <div className="rounded-card bg-surface">
          <p role="status" className="an-thi-giac">
            Đang tải danh sách dự án…
          </p>
          <SkeletonRows />
        </div>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
      {trangThai.pha === "loi" && (
        <div className="rounded-card bg-surface">
          <ErrorState
            title="Chưa tải được danh sách dự án"
            message={<span role="alert">{trangThai.thongBao}</span>}
            onRetry={() => datLanTai((n) => n + 1)}
          />
        </div>
      )}

      {trangThai.pha === "xong" && shown !== null && (
        <>
          {trangThai.duLieu.items.length === 0 ? (
            <div className="rounded-card bg-surface">
              <EmptyState
                icon={FolderKanban}
                title="Chưa có dự án nào"
                description={nhanNamRong(trangThai.duLieu.year)}
                action={emptyAction ?? undefined}
              />
            </div>
          ) : shown.items.length === 0 ? (
            <div className="rounded-card bg-surface">
              <EmptyState
                icon={SearchX}
                tone="neutral"
                title="Không có dự án nào khớp từ khoá"
                description="Thử tên hoặc mã dự án khác, hoặc xoá từ khoá để xem cả danh sách."
              />
            </div>
          ) : (
            <BangDanhSach duLieu={shown} danhMuc={danhMuc} selection={selection} />
          )}

          {/* NGƯỠNG LÀ CỦA MÁY CHỦ, HIỆN RA ĐỂ NGƯỜI ĐỌC BIẾT CHỮ "CHẬM" ĐANG ĐO BẰNG GÌ. The prototype
              prints it in the third KPI card's hint; those cards are placeholders, so it stays as
              the line under the table rather than disappearing. */}
          <p className="m-0 mt-2 inline-flex items-center gap-1.5 text-xs text-ink-500">
            <Glyph icon={Gauge} className="size-3.5 shrink-0" />
            {nhanNguongCham(trangThai.duLieu.delay_threshold)}
          </p>
        </>
      )}

      {canDelete && bulkRows !== null && (
        <ProjectBulkDeleteDialog
          projects={bulkRows}
          onClose={() => setBulkRows(null)}
          // Re-read the whole list: deleted rows leave, refused ones stay — the server says which.
          onFinished={() => datLanTai((n) => n + 1)}
        />
      )}
    </section>
  );
}

const EMPTY_IDS: ReadonlySet<string> = new Set();

/** What the table needs to draw its checkbox column. `undefined` = no column at all. */
export type RowSelection = {
  readonly has: (id: string) => boolean;
  readonly toggle: (id: string) => void;
  /** Ticks every row on screen, or unticks them all when every one is already ticked. */
  readonly toggleAll: () => void;
};

/** Rows whose name or code contains the typed words, case- and accent-sensitive as typed. */
function matchKeyword(items: readonly finance_duAnRa[], keyword: string): finance_duAnRa[] {
  const needle = keyword.trim().toLocaleLowerCase("vi");
  if (needle === "") return [...items];
  return items.filter(
    (d) => d.name.toLocaleLowerCase("vi").includes(needle) || d.code.toLocaleLowerCase("vi").includes(needle),
  );
}

/**
 * Bảng dự án — prototype columns (`BudgetItemTable.tsx:191-216, 276-393`): Mã · Dự án · Đơn vị /
 * phụ trách · KH vốn năm · Đã giải ngân · Tiến độ · Nguồn vốn · Thời hạn giải ngân · Vướng mắc mới
 * nhất. Giữ NGUYÊN thứ tự máy chủ trả về và không lọc bỏ dòng nào.
 *
 * NO "CÒN LẠI" COLUMN, as in the prototype: the remainder is on the project page, and an over-plan
 * disbursement still shows here as a ratio above 100% — never clamped (spec §13 rule 2).
 *
 * MONEY IN FULL ĐỒNG, not the prototype's short form ("7,5 tỷ"): a rounded figure here goes straight
 * into a budget report.
 *
 * `year` lấy từ PHẢN HỒI chứ không từ trạng thái của ô chọn: một phản hồi không nói nó thuộc năm
 * nào thì không phân biệt được với phản hồi của năm khác (`du_an.go`, `danhSachDuAnRa.Year`).
 */
export function BangDanhSach({
  duLieu,
  danhMuc,
  selection,
}: {
  duLieu: finance_danhSachDuAnRa;
  danhMuc: readonly finance_hangMucRa[];
  /** Checkbox column + select-all; only passed for an account holding `budget.update`. */
  selection?: RowSelection;
}) {
  const ticked = selection === undefined ? 0 : duLieu.items.filter((d) => selection.has(d.id)).length;
  const allChecked = duLieu.items.length > 0 && ticked === duLieu.items.length;
  return (
    <TableScroll sticky aria-label={`Danh sách dự án đầu tư năm ${duLieu.year}`}>
      <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
        <caption className="an-thi-giac">
          Dự án đầu tư của đơn vị trong năm ngân sách {duLieu.year}
        </caption>
        <thead>
          <tr>
            {selection !== undefined && (
              <th scope="col" className="w-10">
                <SelectAllBox
                  checked={allChecked}
                  indeterminate={ticked > 0 && !allChecked}
                  onToggle={selection.toggleAll}
                />
              </th>
            )}
            <th scope="col" className="w-20">
              Mã
            </th>
            <th scope="col" className="min-w-64">
              Dự án
            </th>
            <PendingColumnHeader info={pendingPart(UNIT_OWNER)} className="w-36">
              Đơn vị / phụ trách
            </PendingColumnHeader>
            {/* Money columns right-aligned so the digits of every row line up (spec §6.7). */}
            <th scope="col" className="text-right">
              KH vốn năm
            </th>
            <th scope="col" className="text-right">
              Đã giải ngân
            </th>
            <th scope="col" className="w-44">
              Tiến độ
            </th>
            <th scope="col" className="w-44">
              Nguồn vốn
            </th>
            <th scope="col">Thời hạn giải ngân</th>
            <PendingColumnHeader info={pendingPart(LATEST_ISSUE_COLUMN)} className="min-w-52">
              Vướng mắc mới nhất
            </PendingColumnHeader>
          </tr>
        </thead>
        <tbody>
          {duLieu.items.map((d) => {
            const tienDo = tienDoDuAn(d.delay_score, d.is_delayed);
            const hangMuc = hangMucDuAn(d.category_id, danhMuc);
            const late = tienDo.loai === "cham";
            return (
              // Red left edge for a project the SERVER flagged late (prototype `:252-254`): the eye
              // scanning for late projects passes the left edge first. The words say it too.
              <tr key={d.id} className={cn(late && "border-l-[3px] border-l-danger-500")}>
                {selection !== undefined && (
                  <td>
                    <input
                      type="checkbox"
                      className="size-4"
                      aria-label={`Chọn dự án ${d.code}`}
                      checked={selection.has(d.id)}
                      onChange={() => selection.toggle(d.id)}
                    />
                  </td>
                )}
                <td className="ma-muc text-xs font-semibold text-ink-500">{d.code}</td>
                <td className="whitespace-normal">
                  {/* Đường dẫn con đúng như đặc tả ghi ở đầu chương: `/giai-ngan/du-an/:id`. */}
                  <Link
                    href={`/giai-ngan/du-an/${encodeURIComponent(d.id)}`}
                    className="leading-snug font-semibold text-ink-900 no-underline hover:underline"
                  >
                    {d.name}
                  </Link>
                  <span className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-ink-500">
                    {late && (
                      <span className="inline-flex items-center gap-0.5 font-semibold text-danger-600">
                        <Glyph icon={TrendingDown} className="size-3 shrink-0" />
                        {nhanTienDo(tienDo)}
                      </span>
                    )}
                    <span className={lopHangMuc(hangMuc)}>{nhanHangMuc(hangMuc)}</span>
                  </span>
                </td>
                <PendingCell />
                <td className="text-right tabular-nums">{nhanTien(d.planned_amount)}</td>
                <td className="text-right tabular-nums">{nhanTien(d.disbursed_amount)}</td>
                <td>
                  <ProgressCell ratio={d.disbursed_ratio} late={late} />
                </td>
                <td className="whitespace-normal">
                  <FundingChip status={d.funding_status} names={d.funding_source_names} />
                </td>
                <td className="tabular-nums">{nhanNgay(d.disbursement_deadline)}</td>
                <PendingCell />
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}

/**
 * The header box: ticked when every row on screen is, MIXED (`indeterminate`) when some are. That
 * third state is a DOM property with no HTML attribute, hence the ref.
 */
function SelectAllBox({
  checked,
  indeterminate,
  onToggle,
}: {
  checked: boolean;
  indeterminate: boolean;
  onToggle: () => void;
}) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current !== null) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      ref={ref}
      type="checkbox"
      className="size-4"
      aria-label="Chọn tất cả dự án đang hiện"
      checked={checked}
      onChange={onToggle}
    />
  );
}

/**
 * Whether a project's funding is declared, and declared in full (prototype `SourceState`,
 * `BudgetItemTable.tsx:404-436`; spec §7.2). Three states because they lead to three different jobs:
 * none → open the project and declare; short → declare the rest; full → nothing to do.
 *
 * THE STATE AND THE SHORTFALL ARE THE SERVER'S (`funding_status`), never re-derived from the names.
 * The shortfall is in full đồng, like every amount of this table. An unknown state shows the raw
 * string: guessing a friendly label would hide a contract drift.
 */
export function FundingChip({
  status,
  names,
}: {
  status: finance_fundingStatusOut | null | undefined;
  names: readonly string[] | undefined;
}) {
  if (status === null || status === undefined) return <span className="text-ink-500">—</span>;
  let chip: ReactNode;
  switch (status.status) {
    case "chua-gan-nguon":
      chip = <span className="font-semibold text-warning-600">Chưa gắn nguồn</span>;
      break;
    case "chua-du":
      chip = <span className="font-semibold text-warning-600">Thiếu {nhanTien(status.shortfall_amount)}</span>;
      break;
    case "du":
      chip = <span className="font-semibold text-success-600">Đủ · {status.source_count} nguồn</span>;
      break;
    default:
      chip = <span className="font-semibold text-ink-700">{status.status}</span>;
  }
  const joined = (names ?? []).join(", ");
  return (
    <span className="block text-xs" data-funding-status={status.status}>
      {chip}
      {joined !== "" && (
        <span className="block max-w-44 truncate text-ink-500" title={joined}>
          {joined}
        </span>
      )}
    </span>
  );
}

/**
 * The prototype's bar + percent (`BudgetItemTable.tsx:313-342`). The ratio is the SERVER's
 * (`disbursed_ratio`, hundredths of a percent); the bar only draws it, capped at full width while the
 * words keep the real figure. No elapsed-time marker: the server does not send that figure, and a
 * browser-clock copy would drift from it at both ends of the year (`chi-tiet-du-an.tsx`).
 *
 * Red only for a project the server flagged late, green otherwise — no colour thresholds of our own.
 */
function ProgressCell({ ratio, late }: { ratio: number | null; late: boolean }) {
  // `null` = no capital allocated: words, never a 0% bar (it would read as the worst project).
  if (ratio === null || !Number.isFinite(ratio)) {
    return <span className="text-[13px] text-ink-500">{nhanTyLeGiaiNgan(ratio)}</span>;
  }
  const width = Math.min(100, Math.max(0, ratio / 100));
  return (
    <div className="flex items-center gap-2">
      <div aria-hidden="true" className="relative h-1.5 min-w-16 flex-1 overflow-hidden rounded-full bg-surface-subtle-2">
        <div
          className={cn("h-full rounded-full", late ? "bg-danger-500" : "bg-success-500")}
          style={{ width: `${width}%` }}
        />
      </div>
      <span className={cn("shrink-0 text-right text-[13px] font-semibold tabular-nums", late && "text-danger-600")}>
        {nhanTyLeGiaiNgan(ratio)}
      </span>
    </div>
  );
}
