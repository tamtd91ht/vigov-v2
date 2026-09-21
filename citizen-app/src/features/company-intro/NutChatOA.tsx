import { useState } from "react";

import { DUONG_DAN_CHAT_OA } from "../../content/dich-ra-ngoai";
import { moRaNgoai } from "../tinh-nang/mo-ra-ngoai";
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung";

import { MessagingGlyph } from "./icons";

/** Nhãn của nút. Hằng có tên để `bundle-for-zalo.test.ts` đọc lại đúng chuỗi màn hình vẽ. */
export const NHAN_NUT_CHAT = "Chat Zalo";

/**
 * NÚT CHAT NỔI — mở cửa sổ trò chuyện với Official Account, từ MỌI màn hình.
 *
 * ⚠ NÓ TRỎ VỀ OA **NỀN TẢNG**, KHÔNG PHẢI OA CỦA MỘT XÃ NÀO — và ở giai đoạn 2 đó là chỗ hiểu
 * nhầm đắt nhất của cả màn hình này.
 *
 *   ADR 0018 §Hệ quả điểm 3: mọi bề mặt OA bên trong Mini App trỏ về OA nền tảng. Một Mini App
 *   có một OA xác thực cho 200+ xã, nên không có "OA của xã bạn" để mở ở đây. Ngày kênh công dân
 *   sống, nút này phải nói ra điều đó, nếu không người dân bấm "quan tâm" và tưởng mình vừa theo
 *   dõi xã của mình. Địa chỉ và cảnh báo đầy đủ nằm ở `content/dich-ra-ngoai.ts` —
 *   `OA_NEN_TANG_ID` CHƯA ĐƯỢC ĐỐI CHIẾU với Developer Console.
 *
 * ⚠ BIẾN MẤT Ở LỚP KHÁM PHÁ, CÙNG LÚC VỚI THANH TAB (xem `App.tsx`). Màn chọn xã là màn "mỗi màn
 * một việc" (`skills/accessibility-elderly`, yêu cầu #4): việc ấy là chọn đúng xã, và một nút nổi
 * mời chat với một doanh nghiệp giữa lúc ấy là một đường rẽ sai chỗ.
 *
 * ⚠ CHỮ, KHÔNG PHẢI MỘT BIỂU TƯỢNG TRẦN. Một nút tròn chỉ có hình bong bóng thì người chưa từng
 * gặp quy ước ấy không biết nó làm gì — và đó đúng là nhóm người `skills/accessibility-elderly`
 * nói tới. Nút là một viên thuốc có chữ, cao đủ `--tap-min`.
 */
export function NutChatOA() {
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);

  async function moChat() {
    datKhongMoDuoc(!(await moRaNgoai("chat-oa", DUONG_DAN_CHAT_OA)));
  }

  return (
    <div className="chat-noi">
      {/* Ngoài Zalo thì `openWebview` không chạy. Nói ra bằng một câu chỉ việc cần làm tiếp —
          không mã lỗi, và không im lặng để người dùng bấm lại lần thứ ba. */}
      {khong_mo_duoc && (
        <p className="tn__loi chat-noi__loi" role="status">
          {LOI_MO_NGOAI}
        </p>
      )}
      <button type="button" className="nut-chat" onClick={() => void moChat()}>
        <MessagingGlyph className="nut-chat__glyph" />
        {NHAN_NUT_CHAT}
      </button>
    </div>
  );
}
