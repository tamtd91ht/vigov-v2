/**
 * Ô GHI CHÚ — TỆP DUY NHẤT TRONG CẢ ỨNG DỤNG ĐƯỢC PHÉP CÓ MỘT Ô NHẬP.
 *
 * ⚠ TỆP NÀY TỒN TẠI ĐỂ LỆNH CẤM Ô NHẬP CÒN HẸP ĐƯỢC. `phase1-collects-nothing.test.ts` cấm
 * `<form|input|textarea|select>` ở mọi tệp và miễn cho ĐÚNG ĐƯỜNG DẪN NÀY (`TEP_O_GHI_CHU`).
 *
 *   Tách một component 30 dòng ra một tệp riêng trông như thừa. Nó không thừa: ngoại lệ đo bằng
 *   ĐƯỜNG DẪN, nên để `<textarea>` trong `TuVanBaoGiaScreen.tsx` thì ngoại lệ phải mang tên tệp
 *   màn hình — và từ đó mọi ô nhập thứ hai, thứ ba thêm vào màn ấy đều lọt, không có gì đỏ lên.
 *   Một tệp chỉ có đúng một ô thì ô thứ hai phải đi qua một cuộc trò chuyện.
 *
 *   Có một ca kiểm cho lệnh cấm ăn một `<textarea>` đặt ở `TuVanBaoGiaScreen.tsx` — tệp NGAY
 *   CẠNH tệp này — và khẳng định nó vẫn bắt.
 *
 * ⚠ VÀ NGOẠI LỆ CHỈ LÀ CHO Ô NHẬP. Một `localStorage` đặt ở tệp này — "lưu tạm chữ đang gõ cho
 * tiện" — vẫn ĐỎ, và có một ca kiểm cho đúng điều đó. Chữ người dùng gõ không được ghi xuống máy.
 *
 * ⚠ KHÔNG `<form>`, CHỈ `<textarea>`. Một `<form>` mang theo hành vi gửi mặc định của trình
 * duyệt: Enter trong một ô là một lần điều hướng của cả trang, và trong một Mini App thì đó là
 * một màn trắng. Nút gửi là một `<button type="button">` gọi hàm, như mọi nút khác của app này.
 *
 * ⚠ ĐẾM THEO KÝ TỰ (rune), KHÔNG THEO BYTE — `conLaiGhiChu` trong hợp đồng. Một ký tự tiếng Việt
 * có dấu tốn tới 3 byte UTF-8; đếm theo byte thì ô này báo hết chỗ khi người dùng mới gõ chừng
 * một phần ba mức thật, và không có gì đỏ lên vì máy chủ vẫn nhận.
 *
 * ⚠ KHÔNG `console.log` MỘT KÝ TỰ NÀO CỦA Ô NÀY. Đây là văn bản tự do: người dùng có thể gõ vào
 * đó số điện thoại của chính mình, tên người khác, bất cứ thứ gì (luật 3, bất biến 1).
 */
import { conLaiGhiChu, TRAN_GHI_CHU } from "../../api/hop-dong-yeu-cau";

import { TU_VAN } from "./noi-dung";

export function OGhiChu({
  gia_tri,
  onDoi,
  id,
}: {
  gia_tri: string;
  onDoi: (chu: string) => void;
  /** `id` truyền vào chứ không gõ cứng: nhãn `<label htmlFor>` và ô phải khớp nhau, và màn hình
      là nơi biết cả hai. */
  id: string;
}) {
  const con_lai = conLaiGhiChu(gia_tri);
  const qua_dai = con_lai < 0;

  return (
    <div className="o-ghi-chu">
      {/* `<label>` THẬT, KHÔNG PHẢI MỘT `<p>` ĐẶT PHÍA TRÊN. Chỉ `<label htmlFor>` mới làm trình
          đọc màn hình đọc nhãn khi con trỏ vào ô, và mới làm một cú chạm vào dòng chữ đưa được
          bàn phím lên — thứ đáng giá nhất với ngón tay của một người lớn tuổi. */}
      <label className="o-ghi-chu__nhan" htmlFor={id}>
        {TU_VAN.nhan_ghi_chu}
      </label>
      <textarea
        id={id}
        className="o-ghi-chu__o"
        value={gia_tri}
        rows={4}
        placeholder={TU_VAN.goi_y_ghi_chu}
        onChange={(su_kien) => onDoi(su_kien.target.value)}
        /* `aria-describedby` nối ô với dòng đếm ngược: người dùng trình đọc màn hình nghe được
           "còn bao nhiêu ký tự" mà không phải đi tìm dòng ấy. */
        aria-describedby={`${id}-dem`}
        aria-invalid={qua_dai}
      />
      {/* TRẠNG THÁI NÓI BẰNG CHỮ, KHÔNG BẰNG RIÊNG MÀU. Khi vượt trần, dòng này đổi hẳn nội dung
          chứ không chỉ đổi sang màu đỏ — người mù màu và người đọc ngoài nắng vẫn phải biết.

          ⚠ ONE LINE, AND IT IS NOT A LIVE REGION WHILE UNDER THE CAP (14/10/2026). It used to be
          `role="status"`: a screen reader read "Còn 999 ký tự", "Còn 998 ký tự"… over every keystroke,
          and a second line repeated the cap. Under the cap the count is reached through
          `aria-describedby` only, when the person asks for it. Over the cap the line moves INTO the
          `role="alert"` wrapper below, so crossing the cap is announced — the one change that needs it.
          The wrapper is always rendered: a live region added together with its text is often not read. */}
      <div role="alert">
        {qua_dai && (
          <p id={`${id}-dem`} className="o-ghi-chu__dem o-ghi-chu__dem--qua">
            {TU_VAN.qua_dai(-con_lai, TRAN_GHI_CHU)}
          </p>
        )}
      </div>
      {/* The cap is read from the contract's constant, never typed into JSX: two places holding one number drift. */}
      {!qua_dai && (
        <p id={`${id}-dem`} className="o-ghi-chu__dem">
          {TU_VAN.con_lai(con_lai, TRAN_GHI_CHU)}
        </p>
      )}
    </div>
  );
}
