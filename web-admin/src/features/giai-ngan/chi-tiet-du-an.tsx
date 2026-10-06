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
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { KhoiChungTu, useProjectVouchers } from "./chung-tu-du-an";
import { ProjectEditPanel, ProjectHeaderActions, ProjectRemoveDialog } from "./ghi-du-an";
import { nhanNgay, nhanTien, nhanTienDo, nhanTyLeGiaiNgan, percentLabel, tienDoDuAn } from "./nhan-du-an";
import { ProjectRecordTabs } from "./pending-parts";
import { ProjectCurvePanel } from "./project-curve";
import { officerLabel, PEOPLE_LOADING, unitLabel, useProjectPeople, type ProjectPeople } from "./project-people";
import { Glyph, ProgressBadge } from "./project-ui";
import { ScopeNotice } from "./scope-notice";

/**
 * Trang chi tiết một dự án in the prototype's composition (`BudgetItemDetail.tsx`, ADR 0068 lần 5):
 * the `← Theo dõi giải ngân` link, the inline edit panel when open, then ONE card — header (code,
 * name, `Sửa dự án`), progress pill, figures, progress bar, the funding block, and the four tabs.
 *
 * HAI TRONG BỐN TAB VẪN KHÔNG CÓ (Vướng mắc · Trao đổi): không tab nào có tuyến phía sau trong hợp
 * đồng REST. They are drawn DISABLED with a "?" (ADR 0068 §14, `pending-parts.tsx`) — never a live tab
 * that opens an empty panel. So Chứng từ, not the prototype's Vướng mắc, is the open tab. `Biểu đồ`
 * is live since 06/10/2026 (`project-curve.tsx`).
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

  return (
    <section className="flex min-w-0 flex-col" aria-label="Chi tiết dự án">
      <Link
        href="/giai-ngan"
        className="mb-3 inline-flex w-fit items-center gap-1.5 text-[13px] text-ink-500 no-underline hover:text-ink-900"
      >
        <Glyph icon={ArrowLeft} className="size-3.5 shrink-0" />
        Theo dõi giải ngân
      </Link>

      {/* Câu của máy chủ trên chính dự án (`scope_notice`), chỉ khi dự án đã về — `scope-notice.tsx`.
          Not in the prototype's detail page; kept because the banner is mandatory (§1). */}
      {trangThai.pha === "xong" && !daXoa && (
        <div className="mb-4">
          <ScopeNotice text={trangThai.duAn.scope_notice} />
        </div>
      )}

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
  const late = tienDo.loai === "cham";
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
          <p className="ma-muc m-0 text-xs font-semibold text-ink-500">{duAn.code}</p>
          <h2 id="ten-du-an-chi-tiet" className="m-0 mt-0.5 text-base leading-snug font-bold text-ink-900">
            {duAn.name}
          </h2>
          {/* The prototype's line names the funding sources and the officer (spec §8 "· phụ trách
              Chưa phân công"); the budget year stays, it is the set the project belongs to. */}
          <p className="m-0 mt-1 text-xs text-ink-500" data-funding-sources="">
            {sourceNamesLine(duAn)} · Năm ngân sách {duAn.year}
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
          {/* HAI MỐC KHÁC NHAU, CỐ Ý ĐỂ CẠNH NHAU. §9 nói rõ: công trình xong tháng 3 vẫn có thể
              phải giải ngân trước 31/12, nên "ngày hoàn thành" không thay được "thời hạn giải
              ngân". Not in the prototype's grid; kept because it is the date asked at settlement. */}
          <Figure label="Thời hạn giải ngân">{nhanNgay(duAn.disbursement_deadline)}</Figure>
        </dl>

        <div className="mt-4">
          {ratio !== null && Number.isFinite(ratio) && (
            // The marker is a SIBLING of the track, not inside it: the track clips (`overflow-hidden`)
            // and the marker stands a little above and below it, as the prototype draws it.
            <div aria-hidden="true" className="relative">
              <div className="h-2.5 overflow-hidden rounded-full bg-surface-subtle-2">
                <div
                  className={cn("h-full rounded-full", late ? "bg-danger-500" : "bg-success-500")}
                  style={{ width: `${Math.min(100, Math.max(0, ratio / 100))}%` }}
                />
              </div>
              {elapsed !== null && (
                <span
                  data-elapsed-marker=""
                  className="absolute -top-1 -bottom-1 w-0.5 -translate-x-1/2 rounded-full bg-ink-900"
                  style={{ left: `${Math.min(100, Math.max(0, elapsed / 100))}%` }}
                />
              )}
            </div>
          )}
          <p className="m-0 mt-1.5 text-xs text-ink-500" data-progress-caption="">
            {ratio === null ? nhanTyLeGiaiNgan(ratio) : `Giải ngân ${nhanTyLeGiaiNgan(ratio)}`}
            {elapsed !== null && ` · thời gian đã trôi qua ${percentLabel(elapsed)}`}
          </p>
        </div>

        {duAn.description !== undefined && duAn.description !== "" && (
          <p className="m-0 mt-3 text-sm whitespace-pre-line text-ink-700">{duAn.description}</p>
        )}

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
      className="mt-4 rounded-xl border border-line p-3"
      data-project-funding=""
    >
      <h3 id="tieu-de-giai-ngan-theo-nguon" className="m-0 mb-2 text-xs font-bold tracking-wide text-ink-500 uppercase">
        Giải ngân theo nguồn vốn
      </h3>
      <ul className="m-0 flex list-none flex-col gap-2.5 p-0">
        {allocations.map((a) => {
          const ratio = a.disbursed_ratio;
          const known = ratio !== null && Number.isFinite(ratio);
          const width = known ? Math.min(100, Math.max(0, ratio / 100)) : 0;
          return (
            <li key={a.funding_source_id} className="min-w-0" data-allocation={a.funding_source_id}>
              <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-[13px]">
                <span className="min-w-0 font-semibold text-ink-900">{a.name}</span>
                <span className="text-ink-500 tabular-nums">
                  {nhanTien(a.disbursed_amount)} / {nhanTien(a.amount)} ·{" "}
                  <span className="font-semibold text-ink-900">
                    {known ? nhanTyLeGiaiNgan(ratio) : "—"}
                  </span>
                </span>
              </div>
              <div aria-hidden="true" className="mt-1 h-2 overflow-hidden rounded-full bg-surface-subtle-2">
                <div className="h-full rounded-full bg-success-500" style={{ width: `${width}%` }} />
              </div>
            </li>
          );
        })}
      </ul>
      {unallocatedPlanAmount !== null && unallocatedPlanAmount !== undefined && unallocatedPlanAmount > 0 && (
        <p className="m-0 mt-2 text-[13px] text-warning-600">
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
      <dt className="text-xs font-semibold text-ink-500">{label}</dt>
      <dd className="m-0 text-sm font-semibold break-words text-ink-900 tabular-nums">{children}</dd>
    </div>
  );
}
