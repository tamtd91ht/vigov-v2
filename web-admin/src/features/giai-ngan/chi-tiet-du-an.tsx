"use client";

import { ArrowLeft, CircleCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";

import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import { layChiTietDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type { finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { KhoiChungTu } from "./chung-tu-du-an";
import { KhoiSuaXoaDuAn } from "./ghi-du-an";
import { KhoiChuaDungGhi } from "./khoi-chua-dung-ghi";
import {
  nhanNgay,
  nhanTien,
  nhanTienDo,
  nhanTyLeGiaiNgan,
  tienDoDuAn,
} from "./nhan-du-an";
import { Glyph, ProgressBadge } from "./project-ui";
import { ScopeNotice } from "./scope-notice";

/**
 * Trang chi tiết một dự án — `docs/ui-ux/06-giai-ngan.md §8`, cộng `[✎ Sửa dự án]`, `🗑 Gỡ dự án`
 * và khối "Chứng từ" của §8.2.
 *
 * BA TRONG BỐN TAB CỦA ĐẶC TẢ VẪN KHÔNG CÓ (Vướng mắc · Biểu đồ · Trao đổi): không tab nào có
 * tuyến phía sau trong hợp đồng REST. Một thanh tab mà bấm vào không ra gì là những lần hứa suông
 * với cán bộ — tab chỉ mọc khi tuyến mọc.
 *
 * TAB "CHỨNG TỪ" NAY CÓ SÁU TUYẾN GHI VÀ KHÔNG CÓ TUYẾN ĐỌC. Khối chứng từ dưới đây vì thế chỉ giữ
 * được những chứng từ của chính phiên làm việc này, và nó NÓI RA điều đó — xem `chung-tu-du-an.tsx`
 * và mục đầu của `PHAN_CHUA_DUNG_GHI`.
 *
 * KHỐI "GIẢI NGÂN THEO NGUỒN VỐN" CŨNG KHÔNG: `phan_bo_nguon_von` chưa có tuyến nào, và một khối
 * rỗng gắn nhãn "0 đ / 0 đ" đọc thành "xã chưa gắn nguồn nào" — một khẳng định về dữ liệu của xã
 * mà màn hình này không có căn cứ để đưa ra.
 *
 * KHÔNG CÓ VẠCH "THỜI GIAN ĐÃ TRÔI QUA" (§8, thanh tiến độ). Con số ấy là một phép tính trên
 * đồng hồ và trên biên của năm ngân sách; máy chủ tính nó để ra `delay_score` nhưng KHÔNG gửi
 * nó về. Dựng lại phép tính ở trình duyệt là một bản thứ hai đọc đồng hồ của MÁY CÁN BỘ, và nó
 * sẽ lệch bản của máy chủ đúng vào hai đầu năm — thiếu ở hợp đồng, đã báo lên, không vá tạm.
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
  const [danhMuc, datDanhMuc] = useState<readonly finance_hangMucRa[]>([]);
  const khoa = `${id}|${lanTai}`;

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

  // FAIL CLOSED — xem `bang-du-an.tsx`. HAI khoá đọc riêng, và chúng không suy ra nhau.
  const phien = usePhien();
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);
  const coXacNhan = coQuyen(dsQuyen, QUYEN_XAC_NHAN_NGAN_SACH);

  return (
    <section className="man-giai-ngan flex min-w-0 flex-col gap-4" aria-labelledby="tieu-de-chi-tiet-du-an">
      <p className="duong-lui m-0">
        <Link href="/giai-ngan" className="inline-flex items-center gap-1.5">
          <Glyph icon={ArrowLeft} className="size-4 shrink-0" />
          Theo dõi giải ngân
        </Link>
      </p>
      {/* The page's `<h1>` already says "Chi tiết dự án"; this heading stays for the region's name. */}
      <h2 id="tieu-de-chi-tiet-du-an" className="an-thi-giac">
        Chi tiết dự án
      </h2>
      {/* Câu của máy chủ trên chính dự án (`scope_notice`), chỉ khi dự án đã về — `scope-notice.tsx`. */}
      {trangThai.pha === "xong" && <ScopeNotice text={trangThai.duAn.scope_notice} />}

      {/* DỰ ÁN VỪA BỊ GỠ THÌ KHÔNG DỰNG LẠI NÓ. Máy chủ trả 204 không thân, và đọc lại sẽ ra 404 —
          một câu "Không tìm thấy dự án" ngay sau một thao tác thành công đọc như một lỗi. */}
      {daXoa ? (
        <Card>
          <EmptyState
            icon={CircleCheck}
            title="Đã gỡ dự án."
            description="Bản ghi vẫn còn trong hệ thống kèm người gỡ và lý do (xoá mềm), và mã dự án không quay lại dãy."
            action={<Link href="/giai-ngan">Về danh sách dự án</Link>}
          />
        </Card>
      ) : (
        <>
          <KhoiChuaDungGhi />

          {trangThai.pha === "dangTai" && (
            // FIRST LOAD (spec §8b): the sentence stays the live region; the eye gets the card shape.
            <Card>
              <p role="status" className="an-thi-giac">
                Đang tải dự án…
              </p>
              <div aria-hidden="true" className="flex flex-col gap-4 p-4">
                <Skeleton className="h-5 w-2/3" />
                <Skeleton className="w-40" />
                <div className="grid gap-3 sm:grid-cols-2">
                  {Array.from({ length: 6 }, (_, i) => (
                    <Skeleton key={i} className="h-4" />
                  ))}
                </div>
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
              <ThongTinDuAn duAn={trangThai.duAn} />

              <KhoiSuaXoaDuAn
                duAn={trangThai.duAn}
                danhMuc={danhMuc}
                coGhi={coGhi}
                coXacNhan={coXacNhan}
                daSuaXong={() => datLanTai((n) => n + 1)}
                daXoaXong={() => datDaXoa(true)}
              />

              {/* MỖI LẦN GHI CHỨNG TỪ XONG LÀ MỘT LẦN ĐỌC LẠI DỰ ÁN: `disbursed_amount`,
                  `remaining_amount`, `disbursed_ratio` và `delay_score` đều suy ra từ chứng từ. */}
              <KhoiChungTu
                duAnID={trangThai.duAn.id}
                coGhi={coGhi}
                coXacNhan={coXacNhan}
                daGhiXong={() => datLanTai((n) => n + 1)}
              />
            </>
          )}
        </>
      )}
    </section>
  );
}

/** Phần thuần trình bày, tách ra để kết xuất được trong test mà không cần mạng. */
export function ThongTinDuAn({ duAn }: { duAn: finance_duAnRa }) {
  const tienDo = tienDoDuAn(duAn.delay_score, duAn.is_delayed);

  return (
    <Card>
      <CardHeader className="justify-between">
        <div className="min-w-0">
          <CardTitle as="h3" className="text-base">
            {duAn.name}
          </CardTitle>
          <p className="ma-muc m-0 mt-1 text-[13px] text-ink-500">{duAn.code}</p>
        </div>
        <ProgressBadge progress={tienDo}>{nhanTienDo(tienDo)}</ProgressBadge>
      </CardHeader>

      <div className="flex min-w-0 flex-col gap-4 p-4">
        {/* Label–value pairs (spec v2 §8b), in the same order as before. */}
        <dl className={DETAIL_LIST}>
          <DetailItem label="Năm ngân sách">{duAn.year}</DetailItem>

          <DetailItem label="Kế hoạch vốn năm">{nhanTien(duAn.planned_amount)}</DetailItem>

          {/* "Tổng mức được duyệt" về đây đã áp sẵn quy tắc §9 cho ô để trống — máy chủ làm việc
              ấy, nên không có nhánh "để trống thì lấy bằng kế hoạch vốn" nào ở phía web. */}
          <DetailItem label="Tổng mức được duyệt">{nhanTien(duAn.approved_amount)}</DetailItem>

          <DetailItem label="Đã giải ngân">{nhanTien(duAn.disbursed_amount)}</DetailItem>

          <DetailItem label="Còn lại">{nhanTien(duAn.remaining_amount)}</DetailItem>

          <DetailItem label="Tỷ lệ giải ngân">{nhanTyLeGiaiNgan(duAn.disbursed_ratio)}</DetailItem>

          {/* HAI MỐC KHÁC NHAU, CỐ Ý ĐỂ CẠNH NHAU. §9 nói rõ: công trình xong tháng 3 vẫn có thể
              phải giải ngân trước 31/12, nên "ngày hoàn thành" không thay được "thời hạn giải
              ngân". Gộp hai dòng này làm một là mất đúng mốc bị hỏi khi quyết toán. */}
          <DetailItem label="Thời hạn giải ngân">{nhanNgay(duAn.disbursement_deadline)}</DetailItem>

          {/* Trường tuỳ chọn trong hợp đồng: vắng mặt nghĩa là máy chủ không nói gì, và `nhanNgay`
              nói ra điều đó bằng "Chưa đặt" chứ không bằng một ô trống. */}
          <DetailItem label="Ngày khởi công">{nhanNgay(duAn.start_date ?? "")}</DetailItem>

          <DetailItem label="Ngày hoàn thành">{nhanNgay(duAn.completion_date ?? "")}</DetailItem>
        </dl>

        {duAn.description !== undefined && duAn.description !== "" && (
          <p className="m-0 text-sm whitespace-pre-line text-ink-700">{duAn.description}</p>
        )}
      </div>

      {/* ĐƠN VỊ THỰC HIỆN VÀ CÁN BỘ PHỤ TRÁCH KHÔNG HIỆN Ở ĐÂY. Hợp đồng chỉ trả ID nội bộ
          (`org_unit_id`, `assignee_id`); tra chúng thành tên bộ phận và tên cán bộ là việc của
          tuyến khác dưới quyền khác, và in một chuỗi ULID lên màn hình cán bộ không nói với ai
          điều gì. */}
    </Card>
  );
}

/** Label–value grid of the project card: one column on a phone, two pairs per row from 640px. */
const DETAIL_LIST =
  "m-0 grid min-w-0 gap-x-6 gap-y-3 rounded-xl border border-line bg-surface-muted p-4 sm:grid-cols-2";

/** One `<dt>`/`<dd>` pair; figures in tabular digits so amounts line up down the column. */
function DetailItem({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <dt className="text-xs font-semibold text-ink-500">{label}</dt>
      <dd className="m-0 text-sm font-medium break-words text-ink-900 tabular-nums">{children}</dd>
    </div>
  );
}
