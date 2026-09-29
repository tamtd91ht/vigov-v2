/**
 * CHỖ GIỮ PHIÊN — CHỈ TRONG BỘ NHỚ, VÀ "CHỈ TRONG BỘ NHỚ" LÀ MỘT RÀNG BUỘC KIỂM ĐƯỢC.
 *
 * ⚠ VÌ SAO TỆP NÀY RA ĐỜI 22/09/2026, VÀ VÌ SAO NÓ LÀ CHỖ NGUY HIỂM NHẤT CỦA LƯỢT NÀY.
 *
 *   Tới hôm qua, phiếu phiên sống trong `useState` của chính khối đăng nhập (`SessionIssuer`) và
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
 * ⚠ PHIÊN LÀ MỘT PHIẾU TẠM. `contract.ts` đã cảnh báo: ADR 0005 nói phiên được PHÁT HÀNH LẠI sau
 * khi công dân chọn xã, nên bearer nhận lúc đăng nhập không sống tới cuối đời phiên làm việc.
 * Không một dòng nào ở đây được dựng trên giả định "đăng nhập một lần rồi giữ mãi" — và đó là lý
 * do `setSession` nhận cả `null`: hết hạn giữa chừng là đường đi BÌNH THƯỜNG, không phải một lỗi.
 *
 * ⚠ FAIL CLOSED KHI KHÔNG CÓ NHÀ CUNG CẤP. Giá trị mặc định của context là "chưa đăng nhập" chứ
 * không phải một ngoại lệ ném ra: một `throw` ở đây giết cả cây React (màn trắng) vì một màn hình
 * quên bọc, còn "chưa đăng nhập" thì dẫn người dùng tới đúng lời mời đăng nhập. Và nó fail closed
 * theo đúng nghĩa: không phiên ⇒ không gửi được gì ⇒ không đọc được gì của ai.
 */
import { createContext, type ReactNode, useContext, useMemo, useState } from "react";

import type { Session } from "./contract";

export type SessionStore = {
  /** Phiên đang có, hoặc `null` khi chưa đăng nhập. */
  session: Session | null;
  /** Đặt phiên mới, hoặc `null` để quên phiên (hết hạn, máy chủ trả 401). */
  setSession: (session: Session | null) => void;
};

const NO_SESSION: SessionStore = { session: null, setSession: () => {} };

const SessionContext = createContext<SessionStore>(NO_SESSION);

export function SessionProvider({
  children,
  initial_session = null,
}: {
  children: ReactNode;
  /**
   * Phiên có sẵn lúc dựng. VỎ APP KHÔNG TRUYỀN THAM SỐ NÀY — nó có mặt để phép kiểm dựng được
   * trạng thái "đã đăng nhập", vì `SessionProvider` không có đường nào khác vào trạng thái ấy mà
   * không bấm một nút, và bộ test dựng bằng `react-dom/server` thì không có DOM để bấm.
   *
   * Không có khe này thì màn "Tư vấn và báo giá" ở trạng thái CÓ biểu mẫu — chỗ duy nhất trong cả
   * ứng dụng có một ô nhập — không ca kiểm nào chạm tới được.
   *
   * ⚠ NÓ KHÔNG PHẢI MỘT CỬA LƯU TRỮ. Giá trị vào đây là giá trị BAN ĐẦU của `useState`, không
   * phải một thứ đọc ra từ máy: vẫn không `localStorage`, vẫn mất khi đóng app.
   */
  initial_session?: Session | null;
}) {
  const [session, setSession] = useState<Session | null>(initial_session);
  // `useMemo` để mỗi lượt vẽ của vỏ app không tạo một đối tượng mới và bắt mọi màn đọc context
  // vẽ lại theo. Không phải tối ưu sớm: vỏ app vẽ lại mỗi lần đổi tab.
  const value = useMemo<SessionStore>(() => ({ session, setSession }), [session]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

/** Đọc phiên đang giữ. Không có nhà cung cấp thì trả "chưa đăng nhập" — xem khối đầu tệp. */
export function useSession(): SessionStore {
  return useContext(SessionContext);
}

/**
 * Phiếu bearer để gửi đi, hoặc chuỗi RỖNG khi chưa đăng nhập.
 *
 * MỘT HÀM CHỨ KHÔNG PHẢI `session?.token ?? ""` RẢI Ở BA MÀN: ngày phiếu có thêm một điều kiện
 * (ví dụ: bỏ qua phiếu đã quá `expires_at`), một chỗ sửa là đủ. Ba chỗ thì chỗ thứ ba là chỗ quên.
 */
export function bearerOf(session: Session | null): string {
  return session === null ? "" : session.token;
}
