"use client";

import { Banknote } from "lucide-react";
import { useEffect, useState } from "react";

import { Field } from "@/components/ui/field";
import { PageHeader } from "@/components/ui/page-header";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import type { finance_hangMucRa } from "@/lib/api/schema.gen";
import { danhSachNam, namTheoDongHoMay } from "@/lib/nam";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

import { BangDuAn } from "./bang-du-an";
import { FundingSourceProgress } from "./funding-source-progress";
import { KhoiThemDuAn } from "./ghi-du-an";
import { DISBURSEMENT_READ_DENIED } from "./nhan-du-an";
import { DisbursementHeaderActions } from "./pending-parts";

/**
 * The Giải ngân list screen in the prototype's frame (`BudgetWorkspace.tsx`, ADR 0068 lần 5): the page
 * header with `[Hạng mục] [Nhập giải ngân] [+ Thêm dự án]` and the budget-year select on the right of
 * the title row, then the register (`BangDuAn`).
 *
 * THE HEADER IS DRAWN FOR EVERY ACCOUNT, the register only behind `budget.read` (`CongQuyen`). The
 * three buttons appear only with `budget.update`, as the prototype draws them (`canRecord`) — UX only,
 * `finance` checks both keys on every call (rule 5).
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

  /**
   * Danh mục hạng mục đọc RIÊNG và chỉ một lần: nó không đổi theo năm, và tuyến của nó khai
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
  }, []);

  // FAIL CLOSED: an unread or failed session holds no key (rule 1, forbidden #1).
  const phien = usePhien();
  const permissions: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const canRecord = coQuyen(permissions, QUYEN_GHI_NGAN_SACH);
  const addProject = (
    <KhoiThemDuAn nam={year} danhMuc={categories} coGhi={canRecord} daGhiXong={() => setSaves((n) => n + 1)} />
  );

  return (
    <>
      <PageHeader
        icon={Banknote}
        title="Theo dõi giải ngân"
        subtitle="Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ."
        actions={
          <>
            {canRecord && (
              <>
                <DisbursementHeaderActions />
                {addProject}
              </>
            )}
            <Field label="Năm ngân sách" hideLabel htmlFor="nam-ngan-sach" kind="select" grow="auto">
              <select id="nam-ngan-sach" value={year} onChange={(e) => setYear(Number(e.target.value))}>
                {danhSachNam(anchorYear).map((n) => (
                  <option key={n} value={n}>
                    Năm ngân sách {n}
                  </option>
                ))}
              </select>
            </Field>
          </>
        }
      />
      <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={DISBURSEMENT_READ_DENIED}>
        {/* The empty list repeats `+ Thêm dự án` (prototype `:375-380`); `null` without the key. */}
        <BangDuAn
          nam={year}
          danhMuc={categories}
          reloadSignal={saves}
          emptyAction={addProject}
          // §6, under the same `budget.read` gate as the register; its writes need `budget.update`.
          fundingProgress={<FundingSourceProgress year={year} canManage={canRecord} />}
        />
      </CongQuyen>
    </>
  );
}
