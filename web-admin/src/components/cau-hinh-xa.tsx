"use client";

import { createContext, useContext, type ReactNode } from "react";

import type { TenantConfig } from "@/lib/tenant-config";

/**
 * Phần cấu hình xã được phép đi xuống trình duyệt: **chỉ những gì để hiển thị**.
 *
 * `tenantId` CỐ Ý KHÔNG có ở đây, dù máy chủ đã biết nó. Một giá trị mà client không bao giờ
 * cầm là một giá trị client không thể gửi ngược lên — mà client tự khai xã chính là client tự
 * cấp quyền (luật 1, cấm #2). `host` cũng không: trình duyệt đã ở trên host ấy rồi, mang thêm
 * một bản sao chỉ tạo chỗ cho hai bản lệch nhau.
 */
export type CauHinhXaHienThi = {
  /** Tên xã tại thời điểm này. Một thuộc tính, không phải định danh. */
  displayName: string;
  /** Cơ quan cấp trên — hiện ở dòng dưới tên xã trên đầu trang. */
  parentAuthority: string;
};

/** Rút phần hiển thị ra khỏi cấu hình đầy đủ. Gọi ở máy chủ, ngay trước khi dựng provider. */
export function phanHienThi(cauHinh: TenantConfig): CauHinhXaHienThi {
  return { displayName: cauHinh.displayName, parentAuthority: cauHinh.parentAuthority };
}

// Mặc định là `null`, không phải một xã rỗng. Một xã mặc định trong ngữ cảnh là đúng thứ luật 1
// gọi là "rơi về xã mặc định": nó sẽ hiện tên sai trên màn hình của một cơ quan nhà nước mà
// không có test nào đỏ.
const NguCanh = createContext<CauHinhXaHienThi | null>(null);

export function CauHinhXaProvider({
  giaTri,
  children,
}: {
  giaTri: CauHinhXaHienThi;
  children: ReactNode;
}) {
  return <NguCanh.Provider value={giaTri}>{children}</NguCanh.Provider>;
}

/** Đọc cấu hình xã. Không có provider bao ngoài là lỗi lập trình — hỏng to, hỏng sớm. */
export function useCauHinhXa(): CauHinhXaHienThi {
  const giaTri = useContext(NguCanh);
  if (giaTri === null) {
    throw new Error("useCauHinhXa: thiếu CauHinhXaProvider — cấu hình xã phải do máy chủ truyền xuống");
  }
  return giaTri;
}
