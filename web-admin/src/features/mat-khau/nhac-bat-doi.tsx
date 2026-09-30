"use client";

import { usePhien } from "@/features/phien/phien-hien-tai";

import { dangBiBatDoi } from "./bat-doi-mat-khau";

/**
 * Lời nhắc trên chính màn `/doi-mat-khau`, cho người tới đây vì **bị bắt đổi** chứ không vì tự
 * muốn đổi.
 *
 * VÌ SAO NÓ CẦN TỒN TẠI: hai người đứng trước cùng một biểu mẫu vì hai lý do khác hẳn nhau. Một
 * người chủ động vào đổi mật khẩu; người kia vừa đăng nhập lần đầu và bị hệ thống đưa tới đây mà
 * không hiểu vì sao. Nếu không có câu này, người thứ hai sẽ đọc ô "Mật khẩu hiện tại" và không
 * biết phải điền gì — họ chưa từng đặt mật khẩu nào. Câu trả lời là mật khẩu tạm quản trị viên
 * vừa đọc cho họ, và không nói ra thì đó là chỗ họ dừng lại và gọi điện hỏi.
 *
 * `role="status"` CHỨ KHÔNG `role="alert"`: đây là một trạng thái bình thường của một tài khoản
 * mới, không phải một thứ hỏng. Báo động cắt ngang người dùng trình đọc màn hình vì một chuyện
 * không có gì hỏng là cách làm họ tắt trình đọc đi.
 */
export function NhacBatDoiMatKhau() {
  return <KhungNhacBatDoi batDoi={dangBiBatDoi(usePhien())} />;
}

/**
 * Phần thuần trình bày, tách khỏi phần đọc phiên — cùng khuôn `KhungQuyen` trong
 * `features/quyen/cong-quyen.tsx`, và vì cùng một lý do: nhánh này không ai nhìn thấy trong lúc
 * phát triển (tài khoản người viết đã đổi mật khẩu từ lâu), nên nó phải kiểm được bằng
 * `react-dom/server` chứ không bằng mắt.
 */
export function KhungNhacBatDoi({ batDoi }: { batDoi: boolean }) {
  if (!batDoi) return null;

  return (
    <p className="nhac-bat-doi" role="status">
      Tài khoản của bạn đang dùng mật khẩu tạm do quản trị viên cấp. Hãy đổi sang mật khẩu của
      riêng bạn để tiếp tục sử dụng hệ thống. Ở ô <strong>Mật khẩu hiện tại</strong>, nhập chính
      mật khẩu tạm ấy.
    </p>
  );
}
