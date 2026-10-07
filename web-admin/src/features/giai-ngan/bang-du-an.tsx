"use client";

import { ArrowDown, CircleAlert, CircleCheck, Search, Trash2, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Fragment, useEffect, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { getProjectSummary, layDanhSachDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type {
  finance_danhSachDuAnRa,
  finance_duAnRa,
  finance_fundingStatusOut,
  finance_hangMucRa,
  finance_latestIssueOut,
  finance_projectSummaryOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  delayPointsLabel,
  hangMucDuAn,
  nhanHangMuc,
  nhanNamRong,
  nhanNgay,
  nhanTien,
  nhanTyLeGiaiNgan,
  shortDongLabel,
  tienDoDuAn,
} from "./nhan-du-an";
import { DisbursementOverview, type SummaryState } from "./disbursement-overview";
import { latestIssueDateLine } from "./project-discussion-labels";
import { ProjectBulkDeleteDialog } from "./project-bulk-delete-dialog";
import { PEOPLE_LOADING, UNIT_NOT_LISTED, unitOwnerLabel, useProjectPeople, type ProjectPeople } from "./project-people";
import { BULK_DELETE_BUTTON, selectedProjectsLabel, type SelectedProject } from "./project-bulk-delete";
import { categoryRowLabel, groupProjects, type ProjectGroup } from "./project-groups";
import { sortProjects, type ProjectSort } from "./project-sort";
import { Glyph } from "./project-ui";
import { PROGRESS_BAR_CLASS, PROGRESS_TEXT_CLASS, progressTone } from "./progress-tone";
import { ScopeNotice } from "./scope-notice";
import {
  CHECKBOX_CLASS,
  CHECKBOX_LABEL_CLASS,
  PANEL_CLASS,
  SELECT_CLASS,
  TABLE_CLASS,
  TD_CLASS,
  TH_CLASS,
  TR_CLASS,
  TRACK_CLASS,
} from "./spec-classes";

/**
 * The register of the Giải ngân screen, in the prototype's order (`BudgetWorkspace.tsx:185-407`, spec
 * 02, ADR 0068 lần 6): scope notice · four KPI cards · cumulative chart · per-category table ·
 * per-source block · ONE filter row · the project table.
 *
 * THE YEAR BLOCK (§3 cards, §4 chart, §5 table) READS `GET /api/v1/investment-project-summary`
 * (`disbursement-overview.tsx`), separately from the list and UNFILTERED: its figures are the year's,
 * whatever filter the list below has on. It is read again with the list after every project write this
 * screen makes (add through `reloadSignal`, bulk delete), and after a `Hạng mục` write (labels).
 * Funding-source writes (§6) move none of its figures — plans and vouchers only — so they do not
 * re-read it. Edit and delete of one project happen on the detail page; coming back mounts this anew.
 *
 * `Vướng mắc mới nhất` is the list's own `latest_issue` (889d4598) — no read per row. The fourth card's
 * at-risk count stays a "?" (`pending-parts.tsx`): no rule defines it. `Đơn vị / phụ trách` is names
 * resolved from identity's two catalogues, read once per mount (`project-people.ts`).
 *
 * `GỘP THEO HẠNG MỤC` (on by default, §7.1): group headers carry the SERVER's `by_category` totals, and
 * only while the rows under a header are that whole category — see `project-groups.ts` (ADR 0068 lần 6
 * #11 keeps this against the prototype's browser sum).
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
  /** §7.1 `Chỉ dự án chậm` — filtered by the SERVER (`delayed_only=true`), part of the list's key. */
  const [delayedOnly, setDelayedOnly] = useState(false);
  /** §7.1 `Gộp theo hạng mục`, on by default as the spec has it. Presentation only: no re-read. */
  const [grouped, setGrouped] = useState(true);
  /** Org units + staff for the `Đơn vị / phụ trách` column: one read each per mount, not per year or row. */
  const people = useProjectPeople();
  /** Header sort (G1, user decision 07/10/2026). Presentation only: no re-read. */
  const [sort, setSort] = useState<ProjectSort>("code");
  /** `Tất cả đơn vị phụ trách` (G3): an `org_unit_id`, filtered on the loaded rows. "" = every unit. */
  const [unitId, setUnitId] = useState("");
  const router = useRouter();

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
  const khoa = `${nam}|${hangMucId}|${delayedOnly}|${lanTai}|${reloadSignal}`;

  useEffect(() => {
    let bo = false;
    layDanhSachDuAn({ nam, hangMucId, delayedOnly }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam, hangMucId, delayedOnly, khoa]);

  /**
   * The year summary, kept like the list: the result is stored WITH the key that produced it, and
   * "loading" is derived from the two differing. Not keyed by the list's filters — the year's figures
   * do not move when the list is narrowed. `summaryReads`: retry after an error, re-read after a bulk
   * delete.
   */
  const [summaryReads, setSummaryReads] = useState(0);
  const summaryKey = `${nam}|${reloadSignal}|${summaryReads}`;
  const [summaryLoaded, setSummaryLoaded] = useState<{
    key: string;
    kq: KetQua<finance_projectSummaryOut>;
  } | null>(null);
  useEffect(() => {
    let dropped = false;
    getProjectSummary(nam).then((kq) => {
      if (!dropped) setSummaryLoaded({ key: summaryKey, kq });
    });
    return () => {
      dropped = true;
    };
  }, [nam, summaryKey]);
  const summaryState: SummaryState =
    summaryLoaded === null || summaryLoaded.key !== summaryKey
      ? { phase: "loading" }
      : summaryLoaded.kq.ok
        ? { phase: "ready", summary: summaryLoaded.kq.duLieu }
        : { phase: "error", message: summaryLoaded.kq.thongBao };
  const summary = summaryState.phase === "ready" ? summaryState.summary : null;

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

  /**
   * Unit options = the distinct units the LOADED rows name (prototype `BudgetWorkspace.tsx:297-319`
   * lists what the data holds, not a catalogue), named from identity's org-unit catalogue. A chosen
   * unit the current load no longer names (another year, another category) reads as "" — derived, so
   * no effect has to clear it and no hidden filter empties the table.
   */
  const unitOptions = trangThai.pha === "xong" ? unitOptionsOf(trangThai.duLieu.items, people) : [];
  const activeUnitId = unitOptions.some((u) => u.id === unitId) ? unitId : "";

  const shown =
    trangThai.pha === "xong"
      ? {
          ...trangThai.duLieu,
          items: sortProjects(matchKeyword(matchUnit(trangThai.duLieu.items, activeUnitId), keyword), sort),
        }
      : null;

  const filtered = keyword.trim() !== "" || delayedOnly || activeUnitId !== "";
  const groups: ProjectGroup[] | undefined =
    grouped && shown !== null
      ? groupProjects(shown.items, {
          byCategory: summary?.by_category ?? null,
          danhMuc,
          totalsApply: !filtered && summary !== null && summary.year === shown.year,
          filtered,
        })
      : undefined;

  /**
   * Category options = the summary's `by_category` rows that can filter (prototype
   * `BudgetWorkspace.tsx:288-294`: the same source as the §5 table, so the two can never name a
   * category differently). Until the summary is read, the catalogue stands in. A chosen category neither
   * lists still gets an option, so the select shows what filters the list.
   */
  const categoryOptions: { id: string; label: string }[] =
    summary !== null
      ? summary.by_category
          .filter((r) => r.category_id !== undefined && r.category_id !== "")
          .map((r) => ({ id: r.category_id!, label: categoryRowLabel(r) }))
      : danhMuc.map((h) => ({ id: h.id, label: h.label }));
  const strayCategoryLabel =
    hangMucId !== "" && !categoryOptions.some((o) => o.id === hangMucId)
      ? nhanHangMuc(hangMucDuAn(hangMucId, danhMuc))
      : null;

  /** The mandatory notice (§1), in the server's words — the summary's, else the list's. */
  const scopeNotice =
    summary?.scope_notice ?? (trangThai.pha === "xong" ? trangThai.duLieu.scope_notice : undefined);

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
      {/* BANNER BẮT BUỘC (§1) — câu của MÁY CHỦ (`scope_notice`, xã sửa được ở Lời hệ thống). Không câu
          dự phòng ở client: xem `scope-notice.tsx`. */}
      {scopeNotice !== undefined && scopeNotice.trim() !== "" && (
        <div className="mb-5">
          <ScopeNotice text={scopeNotice} />
        </div>
      )}

      <DisbursementOverview
        state={summaryState}
        selectedCategoryId={hangMucId}
        onSelectCategory={datHangMucId}
        onRetry={() => setSummaryReads((n) => n + 1)}
      />
      {fundingProgress}

      {/* ONE filter row (spec 02 §7): search, category, unit, the two checkboxes, the bulk bar. */}
      <div className="mb-4 flex min-w-0 flex-wrap items-center gap-2.5">
        <div className="relative w-64 max-w-full">
          <Search
            aria-hidden="true"
            focusable="false"
            className="text-ink-muted pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
          />
          <input
            id="tim-du-an"
            type="search"
            aria-label="Tìm dự án"
            value={keyword}
            autoComplete="off"
            placeholder="Tìm theo tên hoặc mã dự án…"
            className={cn(controlClass, "h-9 bg-white pl-9 text-[12.5px] md:text-[12.5px]")}
            onChange={(e) => setKeyword(e.target.value)}
          />
        </div>

        <select
          id="loc-hang-muc"
          aria-label="Lọc theo hạng mục"
          className={SELECT_CLASS}
          value={hangMucId}
          onChange={(e) => datHangMucId(e.target.value)}
        >
          <option value="">Tất cả hạng mục</option>
          {categoryOptions.map((o) => (
            <option key={o.id} value={o.id}>
              {o.label}
            </option>
          ))}
          {strayCategoryLabel !== null && <option value={hangMucId}>{strayCategoryLabel}</option>}
        </select>

        {/* Unit filter (G3), hidden while no loaded project names a unit — as the prototype hides it. It
            narrows the rows already loaded; the list route takes no unit parameter, and needs none: it
            returned the whole year. */}
        {unitOptions.length > 0 && (
          <select
            id="loc-don-vi"
            aria-label="Lọc theo đơn vị phụ trách"
            className={SELECT_CLASS}
            value={activeUnitId}
            onChange={(e) => setUnitId(e.target.value)}
          >
            <option value="">Tất cả đơn vị phụ trách</option>
            {unitOptions.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        )}

        <FilterCheckbox id="loc-chi-du-an-cham" checked={delayedOnly} onChange={setDelayedOnly}>
          Chỉ dự án chậm
        </FilterCheckbox>
        <FilterCheckbox id="loc-gop-hang-muc" checked={grouped} onChange={setGrouped}>
          Gộp theo hạng mục
        </FilterCheckbox>

        {/* `Đã chọn N dự án · Xoá đã chọn` at the right of the filter row, as the prototype puts it
            (`:343-358`): after ticking rows the eye is on the table, not at the foot of the page. */}
        {canDelete && selectedRows.length > 0 && (
          <div className="ml-auto flex items-center gap-2" data-bulk-bar="">
            <span className="text-ink-muted text-[12px]">{selectedProjectsLabel(selectedRows.length)}</span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="text-danger hover:not-disabled:text-danger"
              icon={<Glyph icon={Trash2} className="size-3.5" />}
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
        // Spec 02 §8 "Loading": two `Skeleton h-28` in `space-y-3`; the sentence stays the live region.
        <div className="flex flex-col gap-3">
          <p role="status" className="an-thi-giac">
            Đang tải danh sách dự án…
          </p>
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-28 w-full" />
        </div>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
      {trangThai.pha === "loi" && (
        <div className={PANEL_CLASS}>
          <ErrorState
            title="Chưa tải được danh sách dự án"
            message={<span role="alert">{trangThai.thongBao}</span>}
            onRetry={() => datLanTai((n) => n + 1)}
          />
        </div>
      )}

      {trangThai.pha === "xong" && shown !== null && (
        <>
          {trangThai.duLieu.items.length === 0 && delayedOnly ? (
            // "No late project" is good news, not an empty year: never the `Thêm dự án` invitation
            // (a state the prototype does not draw; kept by ADR 0068 lần 6 #11, in the spec's card).
            <EmptyCard
              title="Không có dự án nào đang chậm"
              description="Bỏ chọn “Chỉ dự án chậm” để xem mọi dự án của năm."
            />
          ) : trangThai.duLieu.items.length === 0 ? (
            <EmptyCard title="Chưa có dự án nào" description={nhanNamRong(trangThai.duLieu.year)} action={emptyAction} />
          ) : shown.items.length === 0 ? (
            <EmptyCard
              title="Không có dự án nào khớp từ khoá"
              description="Thử tên hoặc mã dự án khác, hoặc xoá từ khoá để xem cả danh sách."
            />
          ) : (
            <BangDanhSach
              duLieu={shown}
              danhMuc={danhMuc}
              selection={selection}
              groups={groups}
              people={people}
              sort={sort}
              onSort={setSort}
              // The marker only when the summary is of the list's own year: a 2025 share drawn over 2026
              // bars would be a wrong figure read as a right one.
              timeElapsedRatio={summary !== null && summary.year === shown.year ? summary.time_elapsed_ratio : null}
              onOpenProject={(id) => router.push(projectPath(id))}
            />
          )}
        </>
      )}

      {canDelete && bulkRows !== null && (
        <ProjectBulkDeleteDialog
          projects={bulkRows}
          onClose={() => setBulkRows(null)}
          // Re-read the whole list: deleted rows leave, refused ones stay — the server says which. The
          // year summary too: a deleted project leaves every total it was in.
          onFinished={() => {
            datLanTai((n) => n + 1);
            setSummaryReads((n) => n + 1);
          }}
        />
      )}
    </section>
  );
}

const EMPTY_IDS: ReadonlySet<string> = new Set();

/** Spec 02 §8 "Rỗng": a white card, the 14px title, one 12.5px line, the action under it. */
function EmptyCard({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return (
    <div className="border-line shadow-card rounded-card border border-solid bg-white p-10 text-center" data-empty-card="">
      <p className="text-navy m-0 text-[14px] font-semibold">{title}</p>
      <p className="text-ink-muted m-0 mt-1.5 text-[12.5px]">{description}</p>
      {action !== undefined && action !== null && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}

/** A live §7.1 checkbox: spec 00 §4 "Checkbox native", the label wrapping the box. */
function FilterCheckbox({
  id,
  checked,
  onChange,
  children,
}: {
  id: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  children: ReactNode;
}) {
  return (
    <label htmlFor={id} className={CHECKBOX_LABEL_CLASS}>
      <input id={id} type="checkbox" className={CHECKBOX_CLASS} checked={checked} onChange={(e) => onChange(e.target.checked)} />
      {children}
    </label>
  );
}

/** What the table needs to draw its checkbox column. `undefined` = no column at all. */
export type RowSelection = {
  readonly has: (id: string) => boolean;
  readonly toggle: (id: string) => void;
  /** Ticks every row on screen, or unticks them all when every one is already ticked. */
  readonly toggleAll: () => void;
};

/** Rows of one org unit; "" = every row. */
function matchUnit(items: readonly finance_duAnRa[], unitId: string): finance_duAnRa[] {
  return unitId === "" ? [...items] : items.filter((d) => (d.org_unit_id ?? "") === unitId);
}

/**
 * Distinct `org_unit_id`s of the loaded rows with the name shown, sorted by name. While the catalogue
 * is not ready the option says so rather than printing an internal id.
 */
function unitOptionsOf(items: readonly finance_duAnRa[], people: ProjectPeople): { id: string; name: string }[] {
  const ids = [...new Set(items.map((d) => (d.org_unit_id ?? "").trim()).filter((id) => id !== ""))];
  const nameOf = (id: string): string =>
    people.units.phase === "ready" ? (people.units.names.get(id) ?? UNIT_NOT_LISTED) : "—";
  return ids.map((id) => ({ id, name: nameOf(id) })).sort((a, b) => a.name.localeCompare(b.name, "vi"));
}

/** The project page, `/giai-ngan/du-an/:id` (spec chapter head), id encoded. */
function projectPath(id: string): string {
  return `/giai-ngan/du-an/${encodeURIComponent(id)}`;
}

/** Rows whose name or code contains the typed words, case- and accent-sensitive as typed. */
function matchKeyword(items: readonly finance_duAnRa[], keyword: string): finance_duAnRa[] {
  const needle = keyword.trim().toLocaleLowerCase("vi");
  if (needle === "") return [...items];
  return items.filter(
    (d) => d.name.toLocaleLowerCase("vi").includes(needle) || d.code.toLocaleLowerCase("vi").includes(needle),
  );
}

/**
 * Bảng dự án — spec 02 §8 / prototype `BudgetItemTable.tsx`: Mã · Dự án · Đơn vị / phụ trách · KH vốn
 * năm · Đã giải ngân · Tiến độ · Nguồn vốn · Thời hạn giải ngân · Vướng mắc mới nhất. Draws the rows in
 * the order given (the server's code order, or the header sort the screen applied — `project-sort.ts`)
 * and drops none; inside each group too, when `groups` is passed — the groups follow their first row
 * (`project-groups.ts`). The `Nguồn vốn` header stays (spec 02 §8: the prototype forgot it).
 *
 * THE WHOLE ROW OPENS THE PROJECT (user decision 07/10/2026, prototype `BudgetItemTable.tsx:245-262`).
 * The name stays a real `<Link>` (ADR 0068 lần 6 #11): the row is not focusable, so the keyboard and
 * screen readers reach the project through the link. It is drawn as the prototype's plain bold name, no
 * underline. The checkbox cell stops the click — a mis-click there must not navigate away from a
 * selection.
 *
 * MONEY SHORT IN `KH vốn năm` / `Đã giải ngân` ("7,5 tỷ", `shortDongLabel`) — spec 02 §8, as the
 * prototype prints them. The full đồng stays in the cell's `title` and in visually-hidden text, and on
 * the project page, which is where a figure is copied into a report.
 *
 * THE DEADLINE IS NEVER RED (spec 02 §8 says red when passed): ADR 0077 #5.
 *
 * `year` lấy từ PHẢN HỒI chứ không từ trạng thái của ô chọn: một phản hồi không nói nó thuộc năm
 * nào thì không phân biệt được với phản hồi của năm khác (`du_an.go`, `danhSachDuAnRa.Year`).
 */
export function BangDanhSach({
  duLieu,
  selection,
  groups,
  people = PEOPLE_LOADING,
  sort,
  onSort,
  timeElapsedRatio = null,
  onOpenProject,
}: {
  duLieu: finance_danhSachDuAnRa;
  /** Kept for callers; the row no longer prints its category (spec 02 §8: no category sub-line). */
  danhMuc: readonly finance_hangMucRa[];
  /** Checkbox column + select-all; only passed for an account holding `budget.update`. */
  selection?: RowSelection;
  /**
   * `Gộp theo hạng mục` on: the rows of `duLieu.items` by category, each group under a header row
   * (`project-groups.ts`). `undefined` = one flat list in the server's order.
   */
  groups?: readonly ProjectGroup[];
  /** Names for the `Đơn vị / phụ trách` column. Absent = not loaded: "—" for an assigned row, never an id. */
  people?: ProjectPeople;
  /** Header sort in force. With `onSort`, the three money/progress headers become buttons. */
  sort?: ProjectSort;
  onSort?: (sort: ProjectSort) => void;
  /** The year's elapsed share (hundredths of a percent), from the summary of THIS list's year; `null` = no marker. */
  timeElapsedRatio?: number | null;
  /** Called with the project id when a row is clicked. Absent = rows are not clickable (the name link still is). */
  onOpenProject?: (id: string) => void;
}) {
  const columnCount = (selection !== undefined ? 1 : 0) + 9;
  const ticked = selection === undefined ? 0 : duLieu.items.filter((d) => selection.has(d.id)).length;
  const allChecked = duLieu.items.length > 0 && ticked === duLieu.items.length;

  function renderRow(d: finance_duAnRa) {
    const tienDo = tienDoDuAn(d.delay_score, d.is_delayed);
    const late = tienDo.loai === "cham";
    return (
      // Red left edge + faint red fill for a project the SERVER flagged late (prototype `:252-254`):
      // the eye scanning for late projects passes the left edge first. The words say it too.
      <tr
        key={d.id}
        data-project-row={d.id}
        className={cn(
          TR_CLASS,
          onOpenProject !== undefined && "cursor-pointer",
          late && "border-l-danger bg-danger/3 border-l-[3px]",
        )}
        onClick={
          onOpenProject === undefined
            ? undefined
            : (e) => {
                // The link navigates on its own; letting the row push too would navigate twice.
                if ((e.target as Element).closest("a")) return;
                onOpenProject(d.id);
              }
        }
      >
        {selection !== undefined && (
          <td className={TD_CLASS} onClick={(e) => e.stopPropagation()}>
            <input
              type="checkbox"
              className="accent-brand m-0 size-4"
              aria-label={`Chọn dự án ${d.code}`}
              checked={selection.has(d.id)}
              onChange={() => selection.toggle(d.id)}
            />
          </td>
        )}
        {/* `ma-muc` kept as the hook tests select; its legacy monospace is overridden — the spec prints the code sans. */}
        <td className={cn(TD_CLASS, "ma-muc text-ink-muted text-[11.5px] [font-family:inherit] font-semibold")}>{d.code}</td>
        <td className={cn(TD_CLASS, "whitespace-normal")}>
          {/* Đường dẫn con đúng như đặc tả ghi ở đầu chương: `/giai-ngan/du-an/:id`. */}
          <Link href={projectPath(d.id)} className="text-navy text-[12.5px] leading-snug font-semibold no-underline">
            {d.name}
          </Link>
          <span className="text-ink-muted mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10.5px]">
            {late && (
              <span className="text-danger flex items-center gap-0.5 font-semibold" data-late="">
                <Glyph icon={TriangleAlert} className="size-3 shrink-0" />
                chậm {delayPointsLabel(tienDo.diem)}
              </span>
            )}
          </span>
        </td>
        <td className={cn(TD_CLASS, "text-ink-muted text-[11.5px] whitespace-normal")} data-unit-owner="">
          {unitOwnerLabel(d, people)}
        </td>
        <td className={cn(TD_CLASS, "text-right text-[11.5px] tabular-nums")} data-money="planned">
          <ShortMoney dong={d.planned_amount} />
        </td>
        <td className={cn(TD_CLASS, "text-right text-[11.5px] tabular-nums")} data-money="disbursed">
          <ShortMoney dong={d.disbursed_amount} />
        </td>
        <td className={TD_CLASS}>
          <ProgressCell ratio={d.disbursed_ratio} timeElapsedRatio={timeElapsedRatio} />
        </td>
        <td className={TD_CLASS}>
          <FundingChip status={d.funding_status} names={d.funding_source_names} />
        </td>
        <td className={cn(TD_CLASS, "text-[11.5px] tabular-nums")}>{nhanNgay(d.disbursement_deadline)}</td>
        <td className={cn(TD_CLASS, "text-[11.5px] whitespace-normal")}>
          <LatestIssueCell issue={d.latest_issue} />
        </td>
      </tr>
    );
  }

  return (
    <div
      role="region"
      tabIndex={0}
      aria-label={`Danh sách dự án đầu tư năm ${duLieu.year}`}
      className="border-line shadow-card rounded-card overflow-x-auto border border-solid bg-white"
    >
      <table className={TABLE_CLASS} data-project-table="">
        <caption className="an-thi-giac">Dự án đầu tư của đơn vị trong năm ngân sách {duLieu.year}</caption>
        <thead>
          <tr className={TR_CLASS}>
            {selection !== undefined && (
              <th scope="col" className={cn(TH_CLASS, "w-10")}>
                <SelectAllBox
                  checked={allChecked}
                  indeterminate={ticked > 0 && !allChecked}
                  onToggle={selection.toggleAll}
                />
              </th>
            )}
            <th scope="col" className={cn(TH_CLASS, "w-20")}>
              Mã
            </th>
            <th scope="col" className={cn(TH_CLASS, "min-w-64")}>
              Dự án
            </th>
            <th scope="col" className={cn(TH_CLASS, "w-36")}>
              Đơn vị / phụ trách
            </th>
            {/* Money columns right-aligned so the digits of every row line up. */}
            <SortHead value="planned_amount" label="KH vốn năm" alignEnd className="w-28" sort={sort} onSort={onSort} />
            <SortHead value="disbursed_amount" label="Đã giải ngân" alignEnd className="w-28" sort={sort} onSort={onSort} />
            <SortHead value="disbursed_ratio" label="Tiến độ" className="w-44" sort={sort} onSort={onSort} />
            <th scope="col" className={cn(TH_CLASS, "w-36")}>
              Nguồn vốn
            </th>
            <th scope="col" className={cn(TH_CLASS, "w-24")}>
              Thời hạn giải ngân
            </th>
            <th scope="col" className={cn(TH_CLASS, "min-w-52")}>
              Vướng mắc mới nhất
            </th>
          </tr>
        </thead>
        <tbody>
          {groups === undefined
            ? duLieu.items.map(renderRow)
            : groups.map((g) => (
                <Fragment key={`group-${g.key}`}>
                  <tr data-group-header={g.key} className={cn(TR_CLASS, "bg-canvas hover:bg-canvas")}>
                    <th
                      scope="colgroup"
                      colSpan={columnCount}
                      className={cn(TD_CLASS, "text-navy py-2 text-left text-[12px] font-bold whitespace-normal")}
                    >
                      {g.title}
                      <span className="text-ink-muted ml-2 font-normal">{g.detail}</span>
                    </th>
                  </tr>
                  {g.items.map(renderRow)}
                </Fragment>
              ))}
        </tbody>
      </table>
    </div>
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
      className="accent-brand m-0 size-4"
      aria-label="Chọn tất cả dự án đang hiện"
      checked={checked}
      onChange={onToggle}
    />
  );
}

/**
 * Whether a project's funding is declared, and declared in full (prototype `SourceState`,
 * `BudgetItemTable.tsx:404-436`; spec 02 §8). Three states because they lead to three different jobs:
 * none → open the project and declare; short → declare the rest; full → nothing to do.
 *
 * THE STATE AND THE SHORTFALL ARE THE SERVER'S (`funding_status`), never re-derived from the names.
 * The shortfall is short (`Thiếu 300 triệu`), as the prototype prints it, the full đồng on hover. An
 * unknown state shows the raw string: guessing a friendly label would hide a contract drift.
 */
export function FundingChip({
  status,
  names,
}: {
  status: finance_fundingStatusOut | null | undefined;
  names: readonly string[] | undefined;
}) {
  if (status === null || status === undefined) return <span className="text-ink-muted text-[11.5px]">—</span>;
  let chip: ReactNode;
  switch (status.status) {
    case "chua-gan-nguon":
      return (
        <span className="text-tangerine text-[11.5px] font-semibold" data-funding-status={status.status}>
          Chưa gắn nguồn
        </span>
      );
    case "chua-du":
      chip = (
        <span className="text-tangerine font-semibold" title={nhanTien(status.shortfall_amount)}>
          Thiếu {shortDongLabel(status.shortfall_amount)}
        </span>
      );
      break;
    case "du":
      chip = <span className="text-leaf font-semibold">Đủ · {status.source_count} nguồn</span>;
      break;
    default:
      chip = <span className="text-ink font-semibold">{status.status}</span>;
  }
  const joined = (names ?? []).join(", ");
  return (
    <span className="block text-[11.5px]" data-funding-status={status.status} title={joined === "" ? undefined : joined}>
      {chip}
      {joined !== "" && <span className="text-ink-muted block truncate">{joined}</span>}
    </span>
  );
}

/**
 * §7.2 `Vướng mắc mới nhất` (prototype `BudgetItemTable.tsx:365-393`): the newest issue's text, then
 * `27/8/2026 · đã gỡ` — tangerine while open, grey once resolved, icon + words, never colour alone. "—"
 * for a project with none: the server fills the column from its own read, so absent means none.
 */
export function LatestIssueCell({ issue }: { issue: finance_latestIssueOut | null | undefined }) {
  if (issue === null || issue === undefined) return <span className="text-ink-muted">—</span>;
  return (
    <span
      className={cn("flex items-start gap-1", issue.resolved ? "text-ink-muted" : "text-tangerine")}
      data-latest-issue={issue.resolved ? "resolved" : "open"}
    >
      <Glyph icon={issue.resolved ? CircleCheck : CircleAlert} className="mt-0.5 size-3 shrink-0" />
      <span className="min-w-0 leading-snug break-words">
        {issue.text}
        <span className="text-ink-muted block text-[10.5px] tabular-nums">{latestIssueDateLine(issue)}</span>
      </span>
    </span>
  );
}

/**
 * A sortable header (G1, prototype `SortHead`, `BudgetItemTable.tsx:51-87`): one direction, largest
 * first; pressing the active one returns to code order. The arrow is faint until active. Without
 * `onSort` it is a plain header. `aria-sort` only on the active column.
 */
function SortHead({
  value,
  label,
  alignEnd = false,
  className,
  sort,
  onSort,
}: {
  value: ProjectSort;
  label: string;
  alignEnd?: boolean;
  className?: string;
  sort: ProjectSort | undefined;
  onSort: ((sort: ProjectSort) => void) | undefined;
}) {
  const active = sort === value;
  return (
    <th scope="col" className={cn(TH_CLASS, alignEnd && "text-right", className)} aria-sort={active ? "descending" : undefined}>
      {onSort === undefined ? (
        label
      ) : (
        <button
          type="button"
          onClick={() => onSort(active ? "code" : value)}
          className={cn(
            "hover:text-navy inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 [font:inherit] text-inherit focus-visible:outline-2 focus-visible:outline-brand",
            alignEnd && "w-full justify-end",
            active && "text-brand font-bold",
          )}
        >
          {label}
          <Glyph icon={ArrowDown} className={cn("size-3 shrink-0", !active && "opacity-25")} />
        </button>
      )}
    </th>
  );
}

/** "7,5 tỷ" for the eye, the full đồng on hover and for screen readers (the short form is hidden from them). */
function ShortMoney({ dong }: { dong: number }) {
  const full = nhanTien(dong);
  return (
    <>
      <span aria-hidden="true" title={full}>
        {shortDongLabel(dong)}
      </span>
      <span className="an-thi-giac">{full}</span>
    </>
  );
}

/**
 * The prototype's bar + percent (`BudgetItemTable.tsx:313-342`, spec 02 §8). The ratio is the SERVER's
 * (`disbursed_ratio`, hundredths of a percent); the bar only draws it, capped at full width while the
 * words keep the real figure.
 *
 * COLOUR BY THE 80/50/30 TIERS (`progress-tone.ts`) — bar and figure, as the prototype does. Whether the
 * project is LATE is not said here: that is the server's flag, drawn as the row's red edge and the
 * "chậm x điểm" words.
 *
 * THE ELAPSED-TIME MARKER (prototype `:325-331`) is the year summary's `time_elapsed_ratio`, passed in
 * only when that summary is of the list's year — the same calendar-year share for every project of a
 * year (`service-finance/internal/http/du_an.go`, `TimeElapsedRatio`), never the browser's clock.
 */
function ProgressCell({ ratio, timeElapsedRatio }: { ratio: number | null; timeElapsedRatio: number | null }) {
  // `null` = no capital allocated: words, never a 0% bar (it would read as the worst project).
  if (ratio === null || !Number.isFinite(ratio)) {
    return <span className="text-ink-muted text-[11.5px]">{nhanTyLeGiaiNgan(ratio)}</span>;
  }
  const width = Math.min(100, Math.max(0, ratio / 100));
  const tone = progressTone(ratio);
  return (
    <div className="flex items-center gap-2">
      <div aria-hidden="true" className={cn("relative h-1.5 min-w-16 flex-1 overflow-hidden rounded-full", TRACK_CLASS)}>
        <div
          data-progress-tone={tone}
          className={cn("h-full rounded-full", PROGRESS_BAR_CLASS[tone])}
          style={{ width: `${width}%` }}
        />
        {timeElapsedRatio !== null && Number.isFinite(timeElapsedRatio) && (
          <span
            aria-hidden="true"
            data-elapsed-marker=""
            className="bg-navy/55 absolute top-0 h-full w-[2px]"
            style={{ left: `${Math.min(100, Math.max(0, timeElapsedRatio / 100))}%` }}
          />
        )}
      </div>
      <span className={cn("w-11 shrink-0 text-right text-[11.5px] font-bold tabular-nums", PROGRESS_TEXT_CLASS[tone])}>
        {nhanTyLeGiaiNgan(ratio)}
      </span>
    </div>
  );
}
