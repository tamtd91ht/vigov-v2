"use client";

import type { ReactNode } from "react";

import { NoAccess } from "@/components/ui/no-access";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { quyetDinhTheoKhoa, type QuyetDinhHien } from "@/lib/quyen";

/**
 * Cổng ẩn/hiện **một màn hình** theo một khoá quyền.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ LỚP TIỆN DỤNG, KHÔNG PHẢI LỚP CHẶN — và câu này không phải lời rào đón, nó là ranh
 * giới quyết định chỗ nào được phép sai mà không mất dữ liệu. Hai lớp, chỉ lớp thứ hai là biện
 * pháp:
 *
 *   1. (ở đây) `permissions` từ `GET /api/v1/sessions/current` không có khoá → không dựng bảng,
 *      không gọi tuyến nào. Chỉ để cán bộ không phải bấm vào một thứ chắc chắn trả 403.
 *   2. (ở máy chủ) từng tuyến khai `RequirePermission(...)` và kiểm theo
 *      `(tenant_id, role, permission)` trên TỪNG yêu cầu. Gọi thẳng tuyến bằng chính cookie
 *      phiên của mình vẫn 403 — không cần sửa một dòng JavaScript nào để thử.
 *
 * Bỏ lớp 1 thì màn hình xấu. Bỏ lớp 2 thì dữ liệu của một cơ quan nhà nước mở cho mọi tài khoản
 * đã đăng nhập (luật 5, cấm #1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * MỘT MÀN, MỘT KHOÁ. Không có biến thể nhận danh sách khoá: xem `quyetDinhTheoKhoa`.
 */
export function CongQuyen({
  khoa,
  cauThieuQuyen,
  children,
}: {
  khoa: string;
  /**
   * Câu hiện ra khi tài khoản thiếu quyền. Nhận vào chứ không viết cứng ở đây: câu ấy phải nói
   * đúng TÊN việc mà tài khoản không làm được, và "bạn không có quyền" trống trơn là câu khiến
   * cán bộ gọi lên tỉnh hỏi mình thiếu quyền gì.
   */
  cauThieuQuyen: string;
  children: ReactNode;
}) {
  const phien = usePhien();

  // `null` là "chưa đọc xong", KHÔNG phải "không có quyền". Ba trạng thái, không hai — hiện câu
  // "không có quyền" trong lúc còn đang đọc là nói một điều chưa biết là đúng hay sai.
  const quyetDinh: QuyetDinhHien | null = phien === null ? null : quyetDinhTheoKhoa(phien, khoa);

  return (
    <KhungQuyen quyetDinh={quyetDinh} cauThieuQuyen={cauThieuQuyen}>
      {children}
    </KhungQuyen>
  );
}

/**
 * Phần thuần trình bày của cổng quyền, tách khỏi phần đọc phiên.
 *
 * TÁCH RA ĐỂ CA BỊ TỪ CHỐI CÓ BÀI TEST. Nhánh "thiếu quyền" và nhánh "không đọc được quyền" là
 * hai nhánh KHÔNG ai nhìn thấy trong lúc phát triển — tài khoản người viết luôn có đủ quyền — và
 * đúng chúng là hai nhánh phải đúng. Gói trong một component tự gọi `usePhien()` thì chúng chỉ
 * kiểm được qua một trình duyệt giả lập; tách ra thì kết xuất được bằng `react-dom/server`.
 */
export function KhungQuyen({
  quyetDinh,
  cauThieuQuyen,
  children,
}: {
  quyetDinh: QuyetDinhHien | null;
  cauThieuQuyen: string;
  children: ReactNode;
}) {
  if (quyetDinh === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;

  // KHÔNG ĐOÁN KHI KHÔNG ĐỌC ĐƯỢC QUYỀN: hiện đúng câu của máy chủ (thường là "Phiên làm việc đã
  // hết hạn"), và không dựng gì.
  if (!quyetDinh.hien && quyetDinh.vi === "khong-doc-duoc") {
    return (
      <p className="thong-bao-loi" role="alert">
        {quyetDinh.thongBao}
      </p>
    );
  }

  // Thiếu quyền là trạng thái BÌNH THƯỜNG của một tài khoản, không phải lỗi: `trang-thai-rong`
  // chứ không `role="alert"` — báo động cắt ngang người dùng trình đọc màn hình vì một chuyện
  // không có gì hỏng.
  //
  // DRAWN WITH THE SHARED `NoAccess` (spec v2 §8b), and the screen's own sentence stays, verbatim,
  // in the same `<p className="trang-thai-rong">`: it names WHICH right is missing, which the generic
  // title cannot. The legacy box of that class is flattened from the wrapper (same variant for every
  // override) so it reads as the caption of the state, not a second box under it. No role is given,
  // for the reason above.
  if (!quyetDinh.hien) {
    return (
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">{cauThieuQuyen}</p>
      </div>
    );
  }

  return <>{children}</>;
}
