import { useEffect, useRef } from "react";

/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC CÓ Ô NHẬP — `phase1-collects-nothing.test.ts` miễn lệnh cấm
 * `<input|textarea>` cho ĐÚNG tệp này (24/09/2026), cạnh `features/yeu-cau/OGhiChu.tsx`.
 *
 * Mọi ô chữ của hai màn "Gửi phản ánh" và "Tra cứu phiếu" đi qua hai component dưới, nên mỗi điểm
 * thu thập mới phải đi qua đúng chỗ này — và ba điều dưới không ai phải nhớ lại ở từng màn:
 *
 *   · NHÃN Ở TRÊN Ô, không chỉ là chữ mờ trong ô (`skills/accessibility-elderly` #4): chữ mờ biến
 *     mất khi bắt đầu gõ, và người lớn tuổi quên mất ô này hỏi gì.
 *   · `autoComplete="off"`: nội dung phản ánh và số điện thoại không đi vào bộ nhớ gợi ý của
 *     trình duyệt trên một máy có thể cho mượn.
 *   · `maxLength` đặt theo giới hạn máy chủ, để người dân không gõ xong một trang rồi bị từ chối.
 *
 * Giá trị sống trong `useState` của màn cha — không ghi xuống máy (`ranh-gioi-hai-nua.test.ts` §3b).
 * Đóng app là mất bản đang gõ: cái giá ấy được nói ra, không giấu (xem báo cáo lượt này).
 */
type ChungProps = {
  id: string;
  nhan: string;
  goi_y?: string;
  gia_tri: string;
  toi_da: number;
  onDoi: (gia_tri: string) => void;
};

/**
 * `required`: OPTIONAL — only the typed-contact path asks for it (ADR 0080: name and number are the commune's
 * only way back to the citizen). It sets `aria-required`, so a screen reader says what the label says.
 * `autoFocus`: OPTIONAL — only the commune send screen's name box, when "Sửa" opens it (08/10/2026): the box
 * replaces the button just pressed, and without it focus would fall back to the page.
 */
export function ONhapDong(
  props: ChungProps & { kieu_ban_phim?: "text" | "tel"; required?: boolean; autoFocus?: boolean },
) {
  const id_goi_y = props.goi_y ? `${props.id}-goi-y` : undefined;
  return (
    <div className="cd-o">
      <label className="cd-o__nhan" htmlFor={props.id}>
        {props.nhan}
      </label>
      {props.goi_y && (
        <p className="cd-o__goi-y" id={id_goi_y}>
          {props.goi_y}
        </p>
      )}
      <input
        className="cd-o__nhap"
        id={props.id}
        type={props.kieu_ban_phim === "tel" ? "tel" : "text"}
        inputMode={props.kieu_ban_phim === "tel" ? "tel" : "text"}
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        maxLength={props.toi_da}
        required={props.required}
        aria-required={props.required}
        autoFocus={props.autoFocus === true}
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => props.onDoi(e.target.value)}
      />
    </div>
  );
}

/**
 * Height = the text's height, so the box grows line by line; `.cd-o__nhap--grow`'s `max-height` caps it at ten
 * lines, past which it scrolls. "auto" first, or a box could never shrink back after text is deleted. A measure
 * that is not a number (no layout: tests, a server render) leaves the CSS height alone.
 */
export function fitToContent(box: HTMLTextAreaElement): void {
  box.style.height = "auto";
  const content = box.scrollHeight;
  const borders = box.offsetHeight - box.clientHeight;
  if (!Number.isFinite(content) || content <= 0) return;
  box.style.height = `${content + (Number.isFinite(borders) ? borders : 0)}px`;
}

/**
 * `autoFocus`: OPTIONAL and off unless a caller asks — only the commune app's send screen does, right after the
 * citizen tapped a field (`PROTOTYPE.md` §6.2). The shared app never passes it, so its screens are unchanged.
 * `autoGrow`: OPTIONAL, same single caller (owner, 08/10/2026): three lines to start, growing with the text up to
 * ten (`fitToContent`). The shared app's form and the rating keep their six-line box.
 */
export function ONhapDoan(props: ChungProps & { bat_buoc?: boolean; autoFocus?: boolean; autoGrow?: boolean }) {
  const id_goi_y = props.goi_y ? `${props.id}-goi-y` : undefined;
  const autoGrow = props.autoGrow === true;
  const box = useRef<HTMLTextAreaElement>(null);
  // A value set from outside (a restored draft, a step drawn again) is fitted too, not only typing.
  useEffect(() => {
    if (autoGrow && box.current !== null) fitToContent(box.current);
  }, [autoGrow, props.gia_tri]);
  return (
    <div className="cd-o">
      <label className="cd-o__nhan" htmlFor={props.id}>
        {props.nhan}
      </label>
      {props.goi_y && (
        <p className="cd-o__goi-y" id={id_goi_y}>
          {props.goi_y}
        </p>
      )}
      <textarea
        ref={box}
        className={autoGrow ? "cd-o__nhap cd-o__nhap--doan cd-o__nhap--grow" : "cd-o__nhap cd-o__nhap--doan"}
        id={props.id}
        autoComplete="off"
        maxLength={props.toi_da}
        rows={autoGrow ? 3 : 6}
        required={props.bat_buoc}
        aria-required={props.bat_buoc}
        autoFocus={props.autoFocus === true}
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => {
          // Fitted in the handler, before the next paint — an effect would draw one frame with a scrollbar.
          if (autoGrow) fitToContent(e.currentTarget);
          props.onDoi(e.target.value);
        }}
      />
    </div>
  );
}
