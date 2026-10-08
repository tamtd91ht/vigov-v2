"use client";

import { Skeleton } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { MaTranPhanQuyen } from "./ma-tran-phan-quyen";
import {
  coTheSuaPhanQuyen,
  maVaiTroCuaToi,
  quyetDinhTabPhanQuyen,
  type QuyetDinhTab,
} from "./quyen-tab";

/**
 * Cổng của màn "Phân quyền" — `/nguoi-dung/phan-quyen` từ 05/10/2026, trước đó là một tab của
 * `/cau-hinh` (`docs/ui-ux/14-cau-hinh.md §12.8`). Cùng khuôn với `tab-nguoi-dung.tsx`, khác đúng một
 * khoá quyền.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ LỚP TIỆN DỤNG, KHÔNG PHẢI LỚP CHẶN. Hai lớp, và chỉ lớp thứ hai là biện pháp:
 *
 *   1. (ở đây) `permissions` từ `GET /api/v1/sessions/current` không có `admin.role` → không
 *      dựng ma trận, không gọi tuyến. Chỉ để cán bộ không phải bấm vào một thứ chắc chắn trả 403.
 *   2. (ở máy chủ) `GET /api/v1/role-permissions` khai `RequirePermission("admin.role")` và kiểm
 *      trên TỪNG yêu cầu, theo `(tenant_id, vai trò, quyền)`. Gọi thẳng tuyến bằng chính cookie
 *      phiên của mình vẫn 403 — không cần sửa một dòng JavaScript nào để thử.
 *
 * Bỏ lớp 1 thì màn hình xấu. Bỏ lớp 2 thì bảng phân quyền của một cơ quan nhà nước mở cho mọi tài
 * khoản đã đăng nhập (luật 5, cấm #1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * FAIL CLOSED: không đọc được danh sách quyền — phiên hết hạn, mạng hỏng, máy chủ lỗi — thì ẨN,
 * không phải hiện. Quyết định ấy nằm trong `quyen-tab.ts` để nó có bài test riêng: đó là ca không
 * nhìn thấy bằng mắt mà lại là ca sẽ xảy ra thật.
 */

export function TabPhanQuyen() {
  // Phiên đọc MỘT LẦN cho cả trang, ở `PhienProvider` — hai tab trên màn này dùng chung một câu
  // trả lời. Hai lời gọi là hai câu trả lời có thể khác nhau, và khác nhau ngay giữa một màn hình.
  const phien = usePhien();

  // `null` là "chưa đọc xong", không phải "không có quyền". Ba trạng thái, không hai.
  // Drawn as the prototype's `Loading` (three 44px bars); the sentence stays for screen readers.
  if (phien === null) {
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang kiểm tra quyền truy cập…
        </p>
        <div className="space-y-2">
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      </>
    );
  }
  const quyetDinh: QuyetDinhTab = quyetDinhTabPhanQuyen(phien);

  // KHÔNG ĐOÁN KHI KHÔNG ĐỌC ĐƯỢC QUYỀN: không dựng ma trận, và hiện đúng câu của máy chủ (thường
  // là "Phiên làm việc đã hết hạn" — nhưng chữ ấy do máy chủ viết, không do đây đoán).
  if (!quyetDinh.hien && quyetDinh.vi === "khong-doc-duoc") {
    return (
      <p className="thong-bao-loi" role="alert">
        {quyetDinh.thongBao}
      </p>
    );
  }

  if (!quyetDinh.hien) {
    // The prototype's `Denied` paragraph (`AccountWorkspace.tsx`, spec README §Quyền), verbatim.
    return (
      <p className="border-line text-ink-muted rounded-card border bg-white p-6 text-[13px]">
        Tài khoản của bạn không có quyền quản lý phân quyền. Liên hệ Chánh Văn phòng hoặc quản trị viên
        của đơn vị nếu cần.
      </p>
    );
  }

  // Hai giá trị dưới chỉ là tiện dụng:
  // `PUT /api/v1/roles/{id}/permissions` tự kiểm `admin.role` và #14 trên từng lần lưu.
  return (
    <MaTranPhanQuyen choSua={coTheSuaPhanQuyen(phien)} maVaiTroCuaToi={maVaiTroCuaToi(phien)} />
  );
}
