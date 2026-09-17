"use client";

import { useEffect, useState } from "react";

import { layPhienHienTai } from "@/lib/api/phien";

import { DanhBaCanBo } from "./danh-ba-can-bo";
import { quyetDinhTabNguoiDung, type QuyetDinhTab } from "./quyen-tab";

/**
 * Cổng ẩn/hiện tab "Người dùng" — `docs/ui-ux/14-cau-hinh.md §12.8`: "Tab nào thiếu quyền thì
 * ẩn tab đó".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ LỚP TIỆN DỤNG, KHÔNG PHẢI LỚP CHẶN. Hai lớp, và chỉ lớp thứ hai là biện pháp:
 *
 *   1. (ở đây) `permissions` từ `GET /api/v1/sessions/current` không có `admin.user` → không
 *      dựng bảng, không gọi tuyến danh sách. Chỉ để cán bộ không phải bấm vào một thứ chắc
 *      chắn trả 403.
 *   2. (ở máy chủ) `GET /api/v1/staff` khai `RequirePermission("admin.user")` và kiểm trên
 *      TỪNG yêu cầu, theo `(tenant_id, role, permission)`. Gọi thẳng bằng cookie phiên của
 *      chính mình vẫn 403 — không cần sửa một dòng JavaScript nào để thử.
 *
 * Bỏ lớp 1 thì màn hình xấu. Bỏ lớp 2 thì danh bạ cán bộ của một cơ quan nhà nước mở cho mọi
 * tài khoản đã đăng nhập (luật 5, cấm #1: kiểm quyền ở giao diện thay cho tầng dịch vụ là
 * không kiểm gì cả).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * VÌ SAO ĐỌC QUYỀN Ở TRÌNH DUYỆT CHỨ KHÔNG Ở MÁY CHỦ: gọi ở phía máy chủ thì phải chuyển tiếp
 * cookie phiên từ yêu cầu vào lời gọi API — thêm một chỗ cầm cookie phiên, và là đúng chỗ dễ
 * chuyển tiếp sang sai host. Ở đây lời gọi là đường dẫn tương đối trên chính host của xã,
 * trình duyệt tự gửi cookie host-only, và ứng dụng web không bao giờ chạm vào nó
 * (`lib/api/goi.ts`).
 */

export function TabNguoiDung() {
  // `null` là "chưa đọc xong", không phải "không có quyền". Ba trạng thái, không hai.
  const [quyetDinh, datQuyetDinh] = useState<QuyetDinhTab | null>(null);

  useEffect(() => {
    let bo = false;
    layPhienHienTai().then((ketQua) => {
      if (bo) return;
      datQuyetDinh(quyetDinhTabNguoiDung(ketQua));
    });
    return () => {
      bo = true;
    };
  }, []);

  if (quyetDinh === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;

  // KHÔNG ĐOÁN KHI KHÔNG ĐỌC ĐƯỢC QUYỀN: không dựng bảng, và hiện đúng câu của máy chủ (thường
  // là "Phiên làm việc đã hết hạn" — nhưng chữ ấy do máy chủ viết, không do đây đoán).
  if (!quyetDinh.hien && quyetDinh.vi === "khong-doc-duoc") {
    return (
      <p className="thong-bao-loi" role="alert">
        {quyetDinh.thongBao}
      </p>
    );
  }

  if (!quyetDinh.hien) {
    return (
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền quản lý người dùng, nên tab này không hiển thị. Liên hệ
        quản trị viên của đơn vị nếu bạn cần quyền này.
      </p>
    );
  }

  return <DanhBaCanBo />;
}
