"use client";

import { FolderKanban, Gauge, Search, SearchX, TrendingDown } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";

import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { PendingCell, PendingColumnHeader } from "@/components/ui/pending-feature";
import { SkeletonRows } from "@/components/ui/skeleton";
import { layDanhSachDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type { finance_danhSachDuAnRa, finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";
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
import {
  DisbursementOverviewPending,
  FUNDING_COLUMN,
  LATEST_ISSUE_COLUMN,
  ProjectFilterPending,
  UNIT_OWNER,
} from "./pending-parts";
import { Glyph } from "./project-ui";
import { ScopeNotice } from "./scope-notice";

/**
 * The register of the Giải ngân screen, in the prototype's order (`BudgetWorkspace.tsx:185-407`, ADR
 * 0068 lần 5): scope banner · four KPI cards · cumulative chart · per-category table · per-source
 * block · ONE filter row · the project table.
 *
 * WHAT THE PROTOTYPE DRAWS THAT THIS DOES NOT FILL, and why — read before adding a figure:
 *
 *   KPI cards · cumulative chart · per-category block · the `Đơn vị / phụ trách`,
 *   `Nguồn vốn` and `Vướng mắc mới nhất` columns · `Chỉ dự án chậm` · `Gộp theo hạng mục`
 *
 * All are disabled "?" placeholders at their prototype position (ADR 0068 §14, `pending-parts.tsx`);
 * none shows a figure. The per-source block is live and passed in (`fundingProgress`). No route
 * returns year totals or issue data, the list carries no funding per project, and `org_unit_id` /
 * `assignee_id` arrive as internal ids. Drawing "0 vướng mắc" would tell leadership a figure nobody
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
            <BangDanhSach duLieu={shown} danhMuc={danhMuc} />
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
    </section>
  );
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
}: {
  duLieu: finance_danhSachDuAnRa;
  danhMuc: readonly finance_hangMucRa[];
}) {
  return (
    <TableScroll sticky aria-label={`Danh sách dự án đầu tư năm ${duLieu.year}`}>
      <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
        <caption className="an-thi-giac">
          Dự án đầu tư của đơn vị trong năm ngân sách {duLieu.year}
        </caption>
        <thead>
          <tr>
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
            <PendingColumnHeader info={pendingPart(FUNDING_COLUMN)}>Nguồn vốn</PendingColumnHeader>
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
                <PendingCell />
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
