"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

import type { KetQua } from "@/lib/api/goi";
import { layPhienHienTai } from "@/lib/api/phien";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

/**
 * Phiên làm việc hiện tại, đọc **một lần** cho cả trang.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO LÀ MỘT PROVIDER CHỨ KHÔNG PHẢI MỖI NƠI TỰ GỌI.
 *
 * Hai chỗ cần cùng một câu trả lời: đầu trang cần họ tên và chức vụ, cổng ẩn/hiện tab cần
 * danh sách quyền. Để mỗi bên tự gọi `GET /api/v1/sessions/current` thì mỗi lần mở màn hình
 * là hai lời gọi — và con số ấy chỉ đi lên, vì màn hình thứ ba cần biết "tôi là ai" sẽ thêm
 * một lời gọi nữa mà không ai thấy nó cộng dồn.
 *
 * Nhưng cái đắt hơn không phải số lời gọi: hai lời gọi là HAI CÂU TRẢ LỜI CÓ THỂ KHÁC NHAU.
 * Một phiên hết hạn giữa hai lời gọi cho ra một màn hình vừa hiện tên cán bộ trên đầu trang
 * vừa báo "phiên đã hết hạn" ở thân trang — người đọc không biết tin cái nào.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * ĐỌC Ở TRÌNH DUYỆT, không ở máy chủ: gọi phía máy chủ thì phải tự chuyển tiếp cookie phiên
 * từ yêu cầu vào lời gọi API — thêm một chỗ cầm cookie, và là đúng chỗ dễ chuyển tiếp sang
 * sai host. Ở đây đường dẫn là tương đối trên chính host của xã, trình duyệt tự gửi cookie
 * host-only, và mã ứng dụng không bao giờ chạm vào nó (`lib/api/goi.ts`).
 */

/** `null` nghĩa là CHƯA ĐỌC XONG — khác hẳn "đọc xong và hỏng". Ba trạng thái, không hai. */
export type PhienDaDoc = KetQua<identity_phienHienTaiRa> | null;

const NguCanh = createContext<PhienDaDoc | undefined>(undefined);

export function PhienProvider({ children }: { children: ReactNode }) {
  const [phien, datPhien] = useState<PhienDaDoc>(null);

  useEffect(() => {
    let bo = false;
    layPhienHienTai().then((kq) => {
      if (!bo) datPhien(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  return <NguCanh.Provider value={phien}>{children}</NguCanh.Provider>;
}

/**
 * Phiên hiện tại, hoặc **ném lỗi** nếu không có provider bao ngoài.
 *
 * Ném chứ không trả một giá trị dự phòng: một giá trị dự phòng ở đây nghĩa là đầu trang hiện
 * một khoảng trống hoặc cổng quyền quyết định trên một danh sách quyền rỗng, và cả hai đều
 * trông y hệt lúc chạy đúng. Quên bọc provider là lỗi lúc dựng, và nó phải hỏng ngay ở lần
 * mở màn hình đầu tiên.
 */
export function usePhien(): PhienDaDoc {
  const gt = useContext(NguCanh);
  if (gt === undefined) {
    throw new Error("usePhien: thiếu <PhienProvider> bao ngoài");
  }
  return gt;
}
