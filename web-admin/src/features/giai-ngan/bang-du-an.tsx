"use client";

import { FolderKanban, Gauge } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { ChonNam } from "@/components/chon-nam";
import { Card, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field, Toolbar } from "@/components/ui/field";
import { SkeletonRows } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhSachDuAn } from "@/lib/api/du-an";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type { finance_danhSachDuAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { namTheoDongHoMay } from "@/lib/nam";
import { coQuyen, QUYEN_GHI_NGAN_SACH } from "@/lib/quyen";

import { KhoiThemDuAn } from "./ghi-du-an";
import { KhoiChuaDungGhi } from "./khoi-chua-dung-ghi";
import {
  GHI_CHU_CHI_XEM_GIAI_NGAN,
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
import { Glyph, ProgressBadge } from "./project-ui";
import { ScopeNotice } from "./scope-notice";

/**
 * Bảng dự án đầu tư của một năm ngân sách — `docs/ui-ux/06-giai-ngan.md §7`, cộng nút `+ Thêm dự
 * án` của §9.
 *
 * NHỮNG GÌ ĐẶC TẢ VẼ MÀ ĐÂY KHÔNG DỰNG, và vì sao — đọc trước khi thêm vào:
 *
 *   4 thẻ KPI · biểu đồ luỹ kế · bảng tiến độ theo hạng mục · khối tiến độ theo nguồn vốn ·
 *   chip nguồn vốn trên từng dòng · cột "Vướng mắc mới nhất" · cột "Đơn vị / phụ trách" bằng TÊN
 *
 * Không thứ nào trong số đó có tuyến phía sau trong hợp đồng REST. Nguồn vốn và vướng mắc chưa có
 * tuyến nào; `org_unit_id` và `assignee_id` về dưới dạng ID nội bộ, và tra chúng thành tên người
 * là việc của tuyến khác dưới quyền khác. Vẽ ra một ô rỗng gắn nhãn "0 vướng mắc" là nói với lãnh
 * đạo một con số không ai đo.
 *
 * ⚠ CHỨNG TỪ NAY CÓ SÁU TUYẾN GHI (nhưng vẫn không có tuyến ĐỌC danh sách) — xem
 * `chung-tu-du-an.tsx` và `PHAN_CHUA_DUNG_GHI`.
 *
 * KHÔNG GỘP THEO HẠNG MỤC (đặc tả bật mặc định): gộp cần tổng theo nhóm, và tổng ấy phải cộng ở
 * máy chủ trên nguyên tập dự án. Cộng ở trình duyệt trên danh sách đã lọc là một tổng đúng cho
 * tới lần đầu ai đó lọc, rồi sai lặng lẽ.
 */

type TrangThaiBang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: finance_danhSachDuAnRa };

export function BangDuAn() {
  // Năm neo đọc MỘT lần khi component gắn vào, không đọc lại mỗi lần dựng: đọc lại sẽ làm danh
  // sách năm nhảy ngay giữa phiên làm việc của một cán bộ trực đêm 31/12.
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);
  const [hangMucId, datHangMucId] = useState("");
  const [danhMuc, datDanhMuc] = useState<readonly finance_hangMucRa[]>([]);

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
   * Bộ đếm lần tải. Mỗi lần THÊM DỰ ÁN xong thì tăng một, và danh sách đọc lại từ máy chủ.
   *
   * ĐỌC LẠI CẢ DANH SÁCH, KHÔNG VÁ HÀNG MỚI VÀO: phản hồi của tuyến thêm (`duAnGhiRa`) cố ý KHÔNG
   * mang `disbursed_amount`, `disbursed_ratio`, `delay_score` hay `is_delayed` — tuyến ghi không
   * đọc chúng. Vá một hàng dựng từ phản hồi ấy sẽ đặt bốn ô trống hoặc bốn số 0 vào một bảng mà
   * mọi hàng khác đang mang số thật, và chúng trông y hệt nhau.
   */
  const [lanTai, datLanTai] = useState(0);
  const khoa = `${nam}|${hangMucId}|${lanTai}`;

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

  /**
   * Danh mục hạng mục đọc RIÊNG và chỉ một lần: nó không đổi theo năm, và tuyến của nó khai
   * `any-authenticated` trong khi tuyến dự án đòi `budget.read`. Hỏng danh mục KHÔNG làm hỏng
   * bảng — cột Hạng mục khi ấy hiện mã kèm lời giải thích, thay vì cả màn hình trắng vì một
   * danh mục phụ.
   */
  useEffect(() => {
    let bo = false;
    layHangMucKeHoachVon().then((kq) => {
      if (!bo && kq.ok) datDanhMuc(kq.duLieu.items);
    });
    return () => {
      bo = true;
    };
  }, []);

  /**
   * FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào — "chưa rõ" không được
   * hành xử như "có" (luật 1, cấm #1).
   *
   * CHỈ ĐỌC `budget.update` Ở MÀN NÀY. Bốn thao tác `budget.confirm` của phân hệ (xác nhận, khoá,
   * mở khoá, gỡ) đều nằm ở trang chi tiết, và đọc sẵn khoá ấy ở đây là để lại một biến mà chỗ dùng
   * duy nhất của nó là chỗ ai đó sẽ gắn nhầm một cái nút vào.
   */
  const phien = usePhien();
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);

  return (
    <section className="man-giai-ngan flex min-w-0 flex-col gap-4" aria-labelledby="tieu-de-du-an">
      {/* BANNER BẮT BUỘC (§1) — câu của MÁY CHỦ (`scope_notice`, xã sửa được ở Lời hệ thống), nên nó
          chỉ hiện khi danh sách đã về. Không câu dự phòng ở client: xem `scope-notice.tsx`. */}
      {trangThai.pha === "xong" && <ScopeNotice text={trangThai.duLieu.scope_notice} />}
      <p className="m-0 text-[13px] text-ink-500">{GHI_CHU_CHI_XEM_GIAI_NGAN}</p>

      <KhoiChuaDungGhi />

      <KhoiThemDuAn
        nam={nam}
        danhMuc={danhMuc}
        coGhi={coGhi}
        daGhiXong={() => datLanTai((n) => n + 1)}
      />

      <Card>
        <CardHeader>
          <CardTitle id="tieu-de-du-an" className="inline-flex items-center gap-2">
            <Glyph icon={FolderKanban} className="size-[18px] shrink-0 text-brand-600" />
            Dự án đầu tư
          </CardTitle>
        </CardHeader>

        {/* TWO filters, no search box on this screen: an aligned row, no "Bộ lọc" button (ADR 0068
            §12, "Hàng ≤ 3 ô"). Year first — it decides which budget every figure belongs to. */}
        <Toolbar className="[&_.chon-nam]:m-0 [&_.chon-nam_label]:text-xs [&_.chon-nam_select]:h-10">
          <ChonNam id="nam-ngan-sach" nhan="Năm ngân sách" nam={nam} namGoc={namGoc} datNam={datNam} />

          <Field label="Hạng mục" htmlFor="loc-hang-muc" kind="select">
            <select
              id="loc-hang-muc"
              value={hangMucId}
              onChange={(e) => datHangMucId(e.target.value)}
              // Danh mục rỗng là đường THÔNG THƯỜNG hôm nay (danh mục ship rỗng), nên ô chọn chỉ
              // còn một lựa chọn "Tất cả" — tắt nó đi để không mời cán bộ bấm vào một ô không lọc
              // được gì.
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
        </Toolbar>

        {trangThai.pha === "dangTai" && (
          // FIRST LOAD (spec §8b): the sentence stays the live region, read out as before; the eye
          // gets row-shaped placeholders so the card does not jump when the list arrives.
          <>
            <p role="status" className="an-thi-giac">
              Đang tải danh sách dự án…
            </p>
            <SkeletonRows />
          </>
        )}

        {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`.
            "Tải lại" bumps the reload counter this list already has (`lanTai`). */}
        {trangThai.pha === "loi" && (
          <ErrorState
            title="Chưa tải được danh sách dự án"
            message={<span role="alert">{trangThai.thongBao}</span>}
            onRetry={() => datLanTai((n) => n + 1)}
          />
        )}

        {trangThai.pha === "xong" &&
          (trangThai.duLieu.items.length === 0 ? (
            <EmptyState icon={FolderKanban} title={nhanNamRong(trangThai.duLieu.year)} />
          ) : (
            <BangDanhSach duLieu={trangThai.duLieu} danhMuc={danhMuc} />
          ))}

        {/* NGƯỠNG LÀ CỦA MÁY CHỦ, HIỆN RA ĐỂ NGƯỜI ĐỌC BIẾT CHỮ "CHẬM" ĐANG ĐO BẰNG GÌ. Metadata of
            the list, so it sits in the card footer. */}
        {trangThai.pha === "xong" && (
          <CardFooter>
            <span className="inline-flex items-center gap-1.5">
              <Glyph icon={Gauge} className="size-3.5 shrink-0" />
              {nhanNguongCham(trangThai.duLieu.delay_threshold)}
            </span>
          </CardFooter>
        )}
      </Card>
    </section>
  );
}

/**
 * Bảng dự án. Giữ NGUYÊN thứ tự máy chủ trả về và không lọc bỏ dòng nào.
 *
 * `year` lấy từ PHẢN HỒI chứ không từ trạng thái của ô chọn: một phản hồi không nói nó thuộc năm
 * nào thì không phân biệt được với phản hồi của năm khác, và máy chủ gửi `year` về đúng vì lý do
 * ấy (`service-finance/internal/http/du_an.go`, `danhSachDuAnRa.Year`).
 */
export function BangDanhSach({
  duLieu,
  danhMuc,
}: {
  duLieu: finance_danhSachDuAnRa;
  danhMuc: readonly finance_hangMucRa[];
}) {
  return (
    <TableScroll
      sticky
      aria-label={`Danh sách dự án đầu tư năm ${duLieu.year}`}
      className="rounded-none border-0 shadow-none"
    >
      <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
        <caption className="an-thi-giac">
          Dự án đầu tư của đơn vị trong năm ngân sách {duLieu.year}
        </caption>
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Dự án</th>
            <th scope="col">Hạng mục</th>
            {/* Money columns right-aligned so the digits of every row line up (spec §6.7). */}
            <th scope="col" className="text-right">
              KH vốn năm
            </th>
            <th scope="col" className="text-right">
              Đã giải ngân
            </th>
            <th scope="col" className="text-right">
              Còn lại
            </th>
            <th scope="col">Tiến độ</th>
            <th scope="col">Thời hạn giải ngân</th>
          </tr>
        </thead>
        <tbody>
          {duLieu.items.map((d) => {
            const tienDo = tienDoDuAn(d.delay_score, d.is_delayed);
            const hangMuc = hangMucDuAn(d.category_id, danhMuc);
            return (
              <tr key={d.id}>
                <td className="ma-muc text-[13px] text-ink-500">{d.code}</td>
                <td className="min-w-48 whitespace-normal">
                  {/* Đường dẫn con đúng như đặc tả ghi ở đầu chương: `/giai-ngan/du-an/:id`. */}
                  <Link
                    href={`/giai-ngan/du-an/${encodeURIComponent(d.id)}`}
                    className="font-semibold text-brand-700 no-underline hover:underline"
                  >
                    {d.name}
                  </Link>
                </td>
                <td>
                  <span className={lopHangMuc(hangMuc)}>{nhanHangMuc(hangMuc)}</span>
                </td>
                <td className="text-right tabular-nums">{nhanTien(d.planned_amount)}</td>
                <td className="text-right tabular-nums">{nhanTien(d.disbursed_amount)}</td>
                {/* Số âm hiện nguyên là số âm: giải ngân vượt kế hoạch phải nhìn thấy được. */}
                <td className="text-right tabular-nums">{nhanTien(d.remaining_amount)}</td>
                <td>
                  <ProgressBadge progress={tienDo}>{nhanTienDo(tienDo)}</ProgressBadge>
                  <span className="dong-phu mt-1 tabular-nums">{nhanTyLeGiaiNgan(d.disbursed_ratio)}</span>
                </td>
                <td className="tabular-nums">{nhanNgay(d.disbursement_deadline)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}
