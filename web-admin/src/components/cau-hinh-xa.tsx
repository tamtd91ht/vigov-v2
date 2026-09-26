"use client";

import { createContext, useContext, type ReactNode } from "react";

import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";

// `phanHienThi` và kiểu `CauHinhXaHienThi` ở `@/lib/cau-hinh-xa-hien-thi`: trang máy chủ gọi
// `phanHienThi`, mà một hàm xuất từ tệp `"use client"` này thì máy chủ không gọi được.
export type { CauHinhXaHienThi };

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
