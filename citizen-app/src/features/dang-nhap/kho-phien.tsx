/**
 * CHỖ GIỮ PHIÊN — CHỈ TRONG BỘ NHỚ, VÀ "CHỈ TRONG BỘ NHỚ" LÀ MỘT RÀNG BUỘC KIỂM ĐƯỢC.
 *
 * ⚠ VÌ SAO TỆP NÀY RA ĐỜI 22/09/2026, VÀ VÌ SAO NÓ LÀ CHỖ NGUY HIỂM NHẤT CỦA LƯỢT NÀY.
 *
 *   Tới hôm qua, phiếu phiên sống trong `useState` của chính khối đăng nhập (`PhatHanhPhien`) và
 *   không ai ngoài khối ấy cần nó. Giai đoạn B đổi điều đó: màn "Tư vấn & báo giá" và màn "Yêu
 *   cầu của tôi" nằm ở chỗ khác trong cây React và đều cần phiếu.
 *
 *   Cách rẻ nhất để chuyền một giá trị giữa hai màn là ghi nó xuống máy — `localStorage.setItem`,
 *   một dòng, chạy ngay. Đó chính là dòng mà `phase1-collects-nothing.test.ts` cấm ở MỌI tệp, và
 *   lệnh cấm ấy KHÔNG được thu hẹp trong lượt này: một bearer ghi xuống máy là một bearer ở lại
 *   sau khi người dùng đóng ứng dụng, trên một thiết bị có thể cho mượn. Câu "ứng dụng không lưu
 *   gì xuống máy bạn" trong chính sách quyền riêng tư đứng được là nhờ đúng lệnh cấm ấy.
 *
 *   Nên chỗ giữ là một React context trên `useState`. Đóng app là mất, mở lại đăng nhập một chạm
 *   — và màn hình NÓI RA điều đó (`noi-dung.ts`), thay vì để người dùng tự phát hiện.
 *
 * ⚠ PHIÊN LÀ MỘT PHIẾU TẠM. `hop-dong.ts` đã cảnh báo: ADR 0005 nói phiên được PHÁT HÀNH LẠI sau
 * khi công dân chọn xã, nên bearer nhận lúc đăng nhập không sống tới cuối đời phiên làm việc.
 * Không một dòng nào ở đây được dựng trên giả định "đăng nhập một lần rồi giữ mãi" — và đó là lý
 * do `datPhien` nhận cả `null`: hết hạn giữa chừng là đường đi BÌNH THƯỜNG, không phải một lỗi.
 *
 * ⚠ FAIL CLOSED KHI KHÔNG CÓ NHÀ CUNG CẤP. Giá trị mặc định của context là "chưa đăng nhập" chứ
 * không phải một ngoại lệ ném ra: một `throw` ở đây giết cả cây React (màn trắng) vì một màn hình
 * quên bọc, còn "chưa đăng nhập" thì dẫn người dùng tới đúng lời mời đăng nhập. Và nó fail closed
 * theo đúng nghĩa: không phiên ⇒ không gửi được gì ⇒ không đọc được gì của ai.
 */
import { createContext, type ReactNode, useContext, useMemo, useState } from "react";

import type { Phien } from "./hop-dong";

export type KhoPhien = {
  /** Phiên đang có, hoặc `null` khi chưa đăng nhập. */
  phien: Phien | null;
  /** Đặt phiên mới, hoặc `null` để quên phiên (hết hạn, máy chủ trả 401). */
  datPhien: (phien: Phien | null) => void;
};

const KHONG_CO_PHIEN: KhoPhien = { phien: null, datPhien: () => {} };

const NguCanhPhien = createContext<KhoPhien>(KHONG_CO_PHIEN);

export function NhaCungCapPhien({
  children,
  phien_ban_dau = null,
}: {
  children: ReactNode;
  /**
   * Phiên có sẵn lúc dựng. VỎ APP KHÔNG TRUYỀN THAM SỐ NÀY — nó có mặt để phép kiểm dựng được
   * trạng thái "đã đăng nhập", vì `NhaCungCapPhien` không có đường nào khác vào trạng thái ấy mà
   * không bấm một nút, và bộ test dựng bằng `react-dom/server` thì không có DOM để bấm.
   *
   * Không có khe này thì màn "Tư vấn và báo giá" ở trạng thái CÓ biểu mẫu — chỗ duy nhất trong cả
   * ứng dụng có một ô nhập — không ca kiểm nào chạm tới được.
   *
   * ⚠ NÓ KHÔNG PHẢI MỘT CỬA LƯU TRỮ. Giá trị vào đây là giá trị BAN ĐẦU của `useState`, không
   * phải một thứ đọc ra từ máy: vẫn không `localStorage`, vẫn mất khi đóng app.
   */
  phien_ban_dau?: Phien | null;
}) {
  const [phien, datPhien] = useState<Phien | null>(phien_ban_dau);
  // `useMemo` để mỗi lượt vẽ của vỏ app không tạo một đối tượng mới và bắt mọi màn đọc context
  // vẽ lại theo. Không phải tối ưu sớm: vỏ app vẽ lại mỗi lần đổi tab.
  const gia_tri = useMemo<KhoPhien>(() => ({ phien, datPhien }), [phien]);
  return <NguCanhPhien.Provider value={gia_tri}>{children}</NguCanhPhien.Provider>;
}

/** Đọc phiên đang giữ. Không có nhà cung cấp thì trả "chưa đăng nhập" — xem khối đầu tệp. */
export function dungPhien(): KhoPhien {
  return useContext(NguCanhPhien);
}

/**
 * Phiếu bearer để gửi đi, hoặc chuỗi RỖNG khi chưa đăng nhập.
 *
 * MỘT HÀM CHỨ KHÔNG PHẢI `phien?.token ?? ""` RẢI Ở BA MÀN: ngày phiếu có thêm một điều kiện
 * (ví dụ: bỏ qua phiếu đã quá `het_han`), một chỗ sửa là đủ. Ba chỗ thì chỗ thứ ba là chỗ quên.
 */
export function bearerCua(phien: Phien | null): string {
  return phien === null ? "" : phien.token;
}
