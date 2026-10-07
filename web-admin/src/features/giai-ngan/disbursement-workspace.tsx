"use client";

import { useEffect, useState } from "react";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import type { finance_hangMucRa } from "@/lib/api/schema.gen";
import { danhSachNam, namTheoDongHoMay } from "@/lib/nam";
import { cn } from "@/lib/cn";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

import { BangDuAn } from "./bang-du-an";
import { CategoryManagerButton, canManageCategories } from "./category-manager-dialog";
import { FundingSourceProgress } from "./funding-source-progress";
import { KhoiThemDuAn } from "./ghi-du-an";
import { DISBURSEMENT_READ_DENIED } from "./nhan-du-an";
import { DisbursementImportButton } from "./disbursement-import-dialog";
import { SELECT_CLASS } from "./spec-classes";

/**
 * The Giải ngân list screen in the prototype's frame (`BudgetWorkspace.tsx`, ADR 0068 lần 5): the page
 * header with `[Hạng mục] [Nhập giải ngân] [+ Thêm dự án]` and the budget-year select on the right of
 * the title row, then the register (`BangDuAn`).
 *
 * THE HEADER IS DRAWN FOR EVERY ACCOUNT, the register only behind `budget.read` (`CongQuyen`). The
 * three buttons appear only with `budget.update`, as the prototype draws them (`canRecord`) — except
 * `Hạng mục`, which `admin.lookup` also opens (its routes accept either key). UX only, `finance` checks
 * the keys on every call (rule 5).
 *
 * THE YEAR IS ON SCREEN, NEVER IMPLIED: the list route requires `year` and refuses to default it
 * (`lib/nam.ts`). The anchor year is read ONCE at mount, so the list of years does not jump under an
 * officer working across midnight of 31/12.
 */
export function DisbursementWorkspace() {
  const [anchorYear] = useState(namTheoDongHoMay);
  const [year, setYear] = useState(anchorYear);
  /** Bumped after a project is added: the register re-reads the whole list (see `bang-du-an.tsx`). */
  const [saves, setSaves] = useState(0);
  const [categories, setCategories] = useState<readonly finance_hangMucRa[]>([]);
  /** Bumped after the `Hạng mục` dialog writes: the filter select and the forms re-read the catalogue. */
  const [categoryReads, setCategoryReads] = useState(0);
  /** Bumped after a 201 of `Nhập giải ngân`: the per-source block re-reads (see its `key` below). */
  const [imports, setImports] = useState(0);

  /**
   * Danh mục hạng mục đọc RIÊNG, once and again after each write of the `Hạng mục` dialog: nó không đổi theo năm, và tuyến của nó khai
   * `any-authenticated` trong khi tuyến dự án đòi `budget.read`. Hỏng danh mục KHÔNG làm hỏng bảng —
   * the project's category then shows its code with the reason, instead of a blank screen.
   */
  useEffect(() => {
    let dropped = false;
    layHangMucKeHoachVon().then((kq) => {
      if (!dropped && kq.ok) setCategories(kq.duLieu.items);
    });
    return () => {
      dropped = true;
    };
  }, [categoryReads]);

  // FAIL CLOSED: an unread or failed session holds no key (rule 1, forbidden #1).
  const phien = usePhien();
  const permissions: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const canRecord = coQuyen(permissions, QUYEN_GHI_NGAN_SACH);
  const addProject = (
    <KhoiThemDuAn nam={year} danhMuc={categories} coGhi={canRecord} daGhiXong={() => setSaves((n) => n + 1)} />
  );

  const canManage = canManageCategories(permissions);
  const hasActions = canManage || canRecord;

  return (
    <>
      {/* Spec 02 §1 verbatim: no icon tile, the button group `ml-auto`, the year select last — and
          `ml-auto` on the select itself when there is no button group to push it right. */}
      <div className="mb-3 flex flex-wrap items-end gap-4">
        <div>
          <h1 className="text-navy m-0 text-[22px] font-bold">Theo dõi giải ngân</h1>
          <p className="text-ink-muted m-0 mt-1 text-[13px]">
            Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ.
          </p>
        </div>
        {hasActions && (
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {/* `budget.update` OR `admin.lookup`: the category write routes accept either (e9f669f1). */}
            <CategoryManagerButton canManage={canManage} onChanged={() => setCategoryReads((n) => n + 1)} />
            {canRecord && (
              <>
                {/* An import adds vouchers to many projects at once: the same full re-read as an add. */}
                <DisbursementImportButton
                  canImport={canRecord}
                  onImported={() => {
                    setSaves((n) => n + 1);
                    setImports((n) => n + 1);
                  }}
                />
                {addProject}
              </>
            )}
          </div>
        )}
        <select
          id="nam-ngan-sach"
          aria-label="Năm ngân sách"
          className={cn(SELECT_CLASS, !hasActions && "ml-auto")}
          value={year}
          onChange={(e) => setYear(Number(e.target.value))}
        >
          {danhSachNam(anchorYear).map((n) => (
            <option key={n} value={n}>
              Năm ngân sách {n}
            </option>
          ))}
        </select>
      </div>
      <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={DISBURSEMENT_READ_DENIED}>
        {/* The empty list repeats `+ Thêm dự án` (prototype `:375-380`); `null` without the key. */}
        <BangDuAn
          nam={year}
          danhMuc={categories}
          // Both counters only grow, so their sum changes on every add AND every `Hạng mục` write: the
          // list and the year summary re-read (the summary's category labels come from the catalogue).
          reloadSignal={saves + categoryReads}
          emptyAction={addProject}
          // §6, under the same `budget.read` gate as the register; its writes need `budget.update`.
          // Keyed by `imports`: imported vouchers carry a `Nguồn vốn`, so each source's disbursed amount
          // moves too — a remount re-reads it.
          fundingProgress={<FundingSourceProgress key={imports} year={year} canManage={canRecord} />}
          // Bulk selection + `Xoá đã chọn` under `budget.update` (user decision 06/10/2026).
          canDelete={canRecord}
        />
      </CongQuyen>
    </>
  );
}
