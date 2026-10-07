"use client";

import { ArrowLeft, CircleCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";

import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import { layChiTietDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type { finance_duAnRa, finance_hangMucRa, finance_projectAllocationOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH, QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";

import { KhoiChungTu, useProjectVouchers } from "./chung-tu-du-an";
import { ProjectEditPanel, ProjectHeaderActions, ProjectRemoveDialog } from "./ghi-du-an";
import { nhanNgay, nhanTien, nhanTienDo, nhanTyLeGiaiNgan, percentLabel, tienDoDuAn } from "./nhan-du-an";
import { PROGRESS_BAR_CLASS, progressTone } from "./progress-tone";
import { ProjectRecordTabs } from "./pending-parts";
import { ProjectCommentsPanel } from "./project-comments";
import { ProjectCurvePanel } from "./project-curve";
import { ProjectIssuesPanel, useProjectIssues } from "./project-issues";
import { officerLabel, PEOPLE_LOADING, unitLabel, useProjectPeople, type ProjectPeople } from "./project-people";
import { Glyph, ProgressBadge } from "./project-ui";
import { TRACK_CLASS } from "./spec-classes";

/**
 * Trang chi tiết một dự án in the prototype's composition (`BudgetItemDetail.tsx`, ADR 0068 lần 5):
 * the `← Theo dõi giải ngân` link, the inline edit panel when open, then ONE card — header (code,
 * name, `Sửa dự án`), progress pill, figures, progress bar, the funding block, and the four tabs.
 *
 * ALL FOUR TABS ARE LIVE: Vướng mắc (default, as the prototype) and Trao đổi since 889d4598
 * (`project-issues.tsx`, `project-comments.tsx`), Biểu đồ since 06/10/2026 (`project-curve.tsx`).
 * Names of authors and mentioned staff come from the SAME staff-directory read as the officer's name.
 *
 * TAB "CHỨNG TỪ" ĐỌC DANH SÁCH TỪ MÁY CHỦ (db94b35c) bằng CÙNG khoá đọc lại dự án, nên một lần ghi
 * chứng từ đọc lại cả hai — xem `useProjectVouchers` trong `chung-tu-du-an.tsx`.
 *
 * KHỐI "GIẢI NGÂN THEO NGUỒN VỐN" is live since 8245698b (`funding_allocations` on the detail,
 * `ProjectFundingBlock` below).
 *
 * VẠCH "THỜI GIAN ĐÃ TRÔI QUA" trên thanh tiến độ (§8) đặt tại `time_elapsed_ratio` của MÁY CHỦ —
 * cùng con số máy chủ dùng để ra `delay_score` (a3fdcac2). Never re-derived from the officer's clock:
 * that copy would drift from the server's at both ends of the year.
 */

type TrangThaiChiTiet =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duAn: finance_duAnRa };

export function ChiTietDuAn({ id }: { id: string }) {
  /**
   * KẾT QUẢ LƯU KÈM KHOÁ ĐÃ SINH RA NÓ, và "đang tải" SUY RA từ chỗ hai khoá lệch nhau — cùng
   * khuôn `bang-du-an.tsx`, và cùng lý do: một `setState` thẳng trong thân effect vừa là thứ React
   * Compiler cấm, vừa để lại một cửa sổ ở đó số của dự án CŨ còn đứng trên màn hình sau khi vừa
   * ghi xong một chứng từ làm chúng thay đổi.
   */
  const [daTai, datDaTai] = useState<{ khoa: string; kq: KetQua<finance_duAnRa> } | null>(null);
  const [lanTai, datLanTai] = useState(0);
  const [daXoa, datDaXoa] = useState(false);
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [danhMuc, datDanhMuc] = useState<readonly finance_hangMucRa[]>([]);
  const khoa = `${id}|${lanTai}`;
  const vouchers = useProjectVouchers(id, String(lanTai));
  // §8.1: its own reload count — an issue write moves no figure of the project, so it re-reads only the
  // timeline (and with it the tab's `(N)`); a project re-read (retry, edit) re-reads it too.
  const [issueReloads, setIssueReloads] = useState(0);
  const issues = useProjectIssues(id, `${lanTai}|${issueReloads}`);
  // Org units and staff, read ONCE for the page: the card's names and the edit form's selects.
  const people = useProjectPeople();

  useEffect(() => {
    let bo = false;
    layChiTietDuAn(id).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [id, khoa]);

  /** Danh mục hạng mục cho ô chọn của biểu mẫu sửa. Hỏng danh mục KHÔNG làm hỏng trang. */
  useEffect(() => {
    let bo = false;
    layHangMucKeHoachVon().then((kq) => {
      if (!bo && kq.ok) datDanhMuc(kq.duLieu.items);
    });
    return () => {
      bo = true;
    };
  }, []);

  const trangThai: TrangThaiChiTiet =
    daTai === null || daTai.khoa !== khoa
      ? { pha: "dangTai" }
      : daTai.kq.ok
        ? { pha: "xong", duAn: daTai.kq.duLieu }
        : { pha: "loi", thongBao: daTai.kq.thongBao };

  // FAIL CLOSED — xem `disbursement-workspace.tsx`. HAI khoá đọc riêng, và chúng không suy ra nhau.
  const phien = usePhien();
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);
  const coXacNhan = coQuyen(dsQuyen, QUYEN_XAC_NHAN_NGAN_SACH);
  // The comment route's own key (`budget.read`): the page is reachable without it only by a stale session.
  const canRead = coQuyen(dsQuyen, QUYEN_XEM_GIAI_NGAN);

  return (
    <section className="flex min-w-0 flex-col" aria-label="Chi tiết dự án">
      <Link
        href="/giai-ngan"
        className="text-ink-muted hover:text-navy mb-3 inline-flex w-fit items-center gap-1.5 text-[12.5px] no-underline"
      >
        <Glyph icon={ArrowLeft} className="size-3.5 shrink-0" />
        Theo dõi giải ngân
      </Link>

      {/* No scope notice here (spec 07, row D2): the list page carries it. */}

      {/* DỰ ÁN VỪA BỊ GỠ THÌ KHÔNG DỰNG LẠI NÓ. Máy chủ trả 204 không thân, và đọc lại sẽ ra 404 —
          một câu "Không tìm thấy dự án" ngay sau một thao tác thành công đọc như một lỗi. */}
      {daXoa ? (
        <Card>
          <EmptyState
            icon={CircleCheck}
            title="Đã gỡ dự án."
            description="Bản ghi vẫn còn trong hệ thống kèm người gỡ (xoá mềm), và mã dự án không quay lại dãy."
            action={<Link href="/giai-ngan">Về danh sách giải ngân</Link>}
          />
        </Card>
      ) : (
        <>
          {trangThai.pha === "dangTai" && (
            // FIRST LOAD (spec §8b): the sentence stays the live region; the eye gets the card shape.
            <Card>
              <p role="status" className="an-thi-giac">
                Đang tải dự án…
              </p>
              <div aria-hidden="true" className="flex flex-col gap-3 p-5">
                <Skeleton className="h-7 w-1/2" />
                <Skeleton className="h-24 w-full" />
                <Skeleton className="h-64 w-full" />
              </div>
            </Card>
          )}

          {/* MỘT CÂU DUY NHẤT CHO CẢ "KHÔNG CÓ DỰ ÁN ẤY" LẪN "DỰ ÁN CỦA XÃ KHÁC": máy chủ trả cùng
              một 404 cho cả hai, và giao diện không dựng lại sự phân biệt ấy. */}
          {trangThai.pha === "loi" && (
            <Card>
              <ErrorState
                title="Chưa tải được dự án"
                message={<span role="alert">{trangThai.thongBao}</span>}
                onRetry={() => datLanTai((n) => n + 1)}
              />
            </Card>
          )}

          {trangThai.pha === "xong" && (
            <>
              {/* The edit form opens ON the page, above the card — a dialog would cover the very
                  figures needed to edit right (prototype `:125-145`). */}
              {editing && coGhi && (
                <ProjectEditPanel
                  duAn={trangThai.duAn}
                  danhMuc={danhMuc}
                  people={people}
                  onClose={() => setEditing(false)}
                  onSaved={() => datLanTai((n) => n + 1)}
                />
              )}

              <ThongTinDuAn
                duAn={trangThai.duAn}
                people={people}
                actions={
                  <ProjectHeaderActions
                    coGhi={coGhi}
                    editing={editing}
                    onToggleEdit={() => setEditing((v) => !v)}
                    onRemove={() => setRemoving(true)}
                  />
                }
              >
                {/* MỖI LẦN GHI CHỨNG TỪ XONG LÀ MỘT LẦN ĐỌC LẠI DỰ ÁN VÀ DANH SÁCH CHỨNG TỪ:
                    `disbursed_amount`, `remaining_amount`, `disbursed_ratio` và `delay_score` đều suy
                    ra từ chứng từ. */}
                <ProjectRecordTabs
                  voucherCount={vouchers.phase === "ready" ? vouchers.count : undefined}
                  chart={<ProjectCurvePanel projectId={trangThai.duAn.id} />}
                  issueCount={issues.phase === "ready" ? issues.openCount : undefined}
                  issues={
                    <ProjectIssuesPanel
                      projectId={trangThai.duAn.id}
                      issues={issues}
                      staff={people.staff}
                      canRecord={coGhi}
                      onChanged={() => setIssueReloads((n) => n + 1)}
                    />
                  }
                  discussion={
                    <ProjectCommentsPanel projectId={trangThai.duAn.id} staff={people.staff} canComment={canRead} />
                  }
                >
                  <KhoiChungTu
                    duAnID={trangThai.duAn.id}
                    allocations={trangThai.duAn.funding_allocations ?? []}
                    vouchers={vouchers}
                    coGhi={coGhi}
                    coXacNhan={coXacNhan}
                    daGhiXong={() => datLanTai((n) => n + 1)}
                  />
                </ProjectRecordTabs>
              </ThongTinDuAn>

              {/* `budget.update`, as the button (user decision 06/10/2026, `ProjectHeaderActions`). */}
              {removing && coGhi && (
                <ProjectRemoveDialog
                  duAn={trangThai.duAn}
                  onClose={() => setRemoving(false)}
                  onRemoved={() => datDaXoa(true)}
                />
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}

/**
 * The project card (prototype `ItemBody`, `BudgetItemDetail.tsx:249-324`). Presentational, so it
 * renders in tests without the network; `actions` is the header's buttons, `children` the tabs.
 */
export function ThongTinDuAn({
  duAn,
  people = PEOPLE_LOADING,
  actions,
  children,
}: {
  duAn: finance_duAnRa;
  /** Names for `org_unit_id` / `assignee_id` (`project-people.ts`). Absent = not loaded: "—", never the id. */
  people?: ProjectPeople;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  const tienDo = tienDoDuAn(duAn.delay_score, duAn.is_delayed);
  const ratio = duAn.disbursed_ratio;
  /** The server's share of the budget year gone; absent on an older reply → no marker, no words. */
  const elapsed =
    duAn.time_elapsed_ratio !== undefined && duAn.time_elapsed_ratio !== null && Number.isFinite(duAn.time_elapsed_ratio)
      ? duAn.time_elapsed_ratio
      : null;

  return (
    <Card as="article" aria-labelledby="ten-du-an-chi-tiet">
      <header className="flex items-start gap-3 border-b border-line px-5 py-4">
        <div className="min-w-0 flex-1">
          <p className="ma-muc text-ink-muted m-0 [font-family:inherit] text-[11px] font-semibold">{duAn.code}</p>
          <h2 id="ten-du-an-chi-tiet" className="text-navy m-0 mt-0.5 text-[16px] leading-snug font-bold">
            {duAn.name}
          </h2>
          {/* Spec 07 header line: the funding sources and the officer, nothing else (row D5). */}
          <p className="text-ink-muted m-0 mt-1 text-[11.5px]" data-funding-sources="">
            {sourceNamesLine(duAn)}
            <span data-project-officer=""> · phụ trách {officerLabel(duAn, people)}</span>
          </p>
        </div>
        {actions}
      </header>

      <div className="px-5 py-4">
        <div className="mb-4 flex flex-wrap gap-2">
          <ProgressBadge progress={tienDo}>{nhanTienDo(tienDo)}</ProgressBadge>
        </div>

        <dl className="m-0 grid min-w-0 grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
          <Figure label="Kế hoạch vốn năm">{nhanTien(duAn.planned_amount)}</Figure>
          {/* "Tổng mức được duyệt" về đây đã áp sẵn quy tắc §9 cho ô để trống — máy chủ làm việc
              ấy, nên không có nhánh "để trống thì lấy bằng kế hoạch vốn" nào ở phía web. */}
          <Figure label="Tổng mức được duyệt">{nhanTien(duAn.approved_amount)}</Figure>
          <Figure label="Đã giải ngân">{nhanTien(duAn.disbursed_amount)}</Figure>
          {/* Số âm hiện nguyên là số âm: giải ngân vượt kế hoạch phải nhìn thấy được. */}
          <Figure label="Còn lại">{nhanTien(duAn.remaining_amount)}</Figure>
          <Figure label="Thời gian thực hiện">{executionPeriod(duAn.start_date, duAn.completion_date)}</Figure>
          <Figure label="Đơn vị thực hiện">{unitLabel(duAn, people)}</Figure>
          {/* Six cells as spec 07 (row D8). The disbursement deadline is in the edit form and the list. */}
        </dl>

        <div className="mt-4">
          {ratio !== null && Number.isFinite(ratio) && (
            // Spec 07 §Body 3: the fill in the 80/50/30 tier (not the late flag — that is the badge), the
            // elapsed-time marker INSIDE the track, full track height.
            <div aria-hidden="true" className={cn("relative h-2.5 overflow-hidden rounded-full", TRACK_CLASS)}>
              <div
                className={cn("h-full rounded-full", PROGRESS_BAR_CLASS[progressTone(ratio)])}
                data-progress-tone={progressTone(ratio)}
                style={{ width: `${Math.min(100, Math.max(0, ratio / 100))}%` }}
              />
              {elapsed !== null && (
                <span
                  data-elapsed-marker=""
                  className="bg-navy/55 absolute top-0 h-full w-[2px]"
                  style={{ left: `${Math.min(100, Math.max(0, elapsed / 100))}%` }}
                />
              )}
            </div>
          )}
          <p className="text-ink-muted m-0 mt-1.5 text-[11.5px]" data-progress-caption="">
            {ratio === null ? nhanTyLeGiaiNgan(ratio) : `Giải ngân ${nhanTyLeGiaiNgan(ratio)}`}
            {elapsed !== null && ` · thời gian đã trôi qua ${percentLabel(elapsed)}`}
          </p>
        </div>

        <ProjectFundingBlock
          allocations={duAn.funding_allocations}
          unallocatedPlanAmount={duAn.unallocated_plan_amount}
        />

        {children !== undefined && <div className="mt-5">{children}</div>}
      </div>

      {/* ĐƠN VỊ THỰC HIỆN VÀ CÁN BỘ PHỤ TRÁCH KHÔNG HIỆN BẰNG ID: a reference the catalogue does not
          list, or one read while the catalogue failed, shows words or "—" (`project-people.ts`). */}
    </Card>
  );
}

/** "Ngân sách tỉnh · Ngân sách xã", or "Chưa gắn nguồn vốn" (prototype `BudgetItemDetail.tsx:257-262`). */
function sourceNamesLine(duAn: finance_duAnRa): string {
  const names = duAn.funding_allocations?.map((a) => a.name) ?? duAn.funding_source_names ?? [];
  return names.length === 0 ? "Chưa gắn nguồn vốn" : names.join(" · ");
}

/**
 * `GIẢI NGÂN THEO NGUỒN VỐN` (prototype `BudgetItemDetail.tsx:333-372`, spec §8): per source,
 * disbursed / allocated · % and a bar. A project spending 60% overall may have used all of the
 * province's share and none of the commune's — two different stories at settlement.
 *
 * EVERY FIGURE IS THE SERVER'S. `disbursed_ratio` is hundredths of a percent, NOT clamped (the words
 * keep a figure above 100%, the bar stops at full width), and `null` when the allocated amount is 0 —
 * "—" over an empty track, never "0%". Nothing is drawn for a project with no allocation: the header
 * line already says "Chưa gắn nguồn vốn", and a block of "0 đ / 0 đ" would read as a measurement.
 */
export function ProjectFundingBlock({
  allocations,
  unallocatedPlanAmount,
}: {
  allocations: readonly finance_projectAllocationOut[] | undefined;
  unallocatedPlanAmount: number | null | undefined;
}) {
  if (allocations === undefined || allocations.length === 0) return null;
  return (
    <section
      aria-labelledby="tieu-de-giai-ngan-theo-nguon"
      className="border-line mt-4 rounded-[10px] border border-solid bg-white p-3"
      data-project-funding=""
    >
      <h3 id="tieu-de-giai-ngan-theo-nguon" className="text-ink-muted m-0 mb-2 text-[10.5px] font-bold tracking-wide uppercase">
        Giải ngân theo nguồn vốn
      </h3>
      <ul className="m-0 flex list-none flex-col gap-2.5 p-0">
        {allocations.map((a) => {
          const ratio = a.disbursed_ratio;
          const known = ratio !== null && Number.isFinite(ratio);
          const width = known ? Math.min(100, Math.max(0, ratio / 100)) : 0;
          return (
            <li key={a.funding_source_id} className="min-w-0" data-allocation={a.funding_source_id}>
              <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-[12px]">
                <span className="text-navy min-w-0 font-semibold">{a.name}</span>
                <span className="text-ink-muted tabular-nums">
                  {nhanTien(a.disbursed_amount)} / {nhanTien(a.amount)} ·{" "}
                  <span className="text-navy font-semibold">
                    {known ? nhanTyLeGiaiNgan(ratio) : "—"}
                  </span>
                </span>
              </div>
              <div aria-hidden="true" className={cn("relative mt-1 h-2 overflow-hidden rounded-full", TRACK_CLASS)}>
                <div
                  className={cn("h-full rounded-full", known ? PROGRESS_BAR_CLASS[progressTone(ratio)] : "")}
                  style={{ width: `${width}%` }}
                />
              </div>
            </li>
          );
        })}
      </ul>
      {unallocatedPlanAmount !== null && unallocatedPlanAmount !== undefined && unallocatedPlanAmount > 0 && (
        <p className="text-tangerine m-0 mt-2 text-[11.5px]">
          Còn {nhanTien(unallocatedPlanAmount)} của kế hoạch vốn năm chưa gắn nguồn nào.
        </p>
      )}
    </section>
  );
}

/**
 * `01/01/2026 → 20/03/2026` (prototype `Thời gian thực hiện`). Both dates unset is ONE "Chưa đặt",
 * not "Chưa đặt → Chưa đặt"; one unset keeps its own "Chưa đặt" so the reader sees which is missing.
 */
function executionPeriod(start: string | undefined, end: string | undefined): string {
  const from = start ?? "";
  const to = end ?? "";
  if (from === "" && to === "") return nhanNgay("");
  return `${nhanNgay(from)} → ${nhanNgay(to)}`;
}

/** One `<dt>`/`<dd>` pair of the figure grid (prototype `Figure`). */
function Figure({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <dt className="text-ink-muted text-[10.5px] font-semibold tracking-wide uppercase">{label}</dt>
      <dd className="text-navy m-0 text-[12.5px] font-semibold break-words tabular-nums">{children}</dd>
    </div>
  );
}
