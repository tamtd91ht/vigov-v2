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
 * Giá trị sống trong `useState` của màn cha — không ghi xuống máy (`two-halves-boundary.test.ts` §3b).
 * Đóng app là mất bản đang gõ: cái giá ấy được nói ra, không giấu (xem báo cáo lượt này).
 */
type CommonProps = {
  id: string;
  label: string;
  suggestion?: string;
  value: string;
  max: number;
  onChange: (value: string) => void;
};

export function TextField(
  props: CommonProps & { input_mode?: "text" | "tel" },
) {
  const suggestion_id = props.suggestion ? `${props.id}-goi-y` : undefined;
  return (
    <div className="cd-o">
      <label className="cd-o__nhan" htmlFor={props.id}>
        {props.label}
      </label>
      {props.suggestion && (
        <p className="cd-o__goi-y" id={suggestion_id}>
          {props.suggestion}
        </p>
      )}
      <input
        className="cd-o__nhap"
        id={props.id}
        type={props.input_mode === "tel" ? "tel" : "text"}
        inputMode={props.input_mode === "tel" ? "tel" : "text"}
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        maxLength={props.max}
        value={props.value}
        aria-describedby={suggestion_id}
        onChange={(e) => props.onChange(e.target.value)}
      />
    </div>
  );
}

export function TextAreaField(props: CommonProps & { required?: boolean }) {
  const suggestion_id = props.suggestion ? `${props.id}-goi-y` : undefined;
  return (
    <div className="cd-o">
      <label className="cd-o__nhan" htmlFor={props.id}>
        {props.label}
      </label>
      {props.suggestion && (
        <p className="cd-o__goi-y" id={suggestion_id}>
          {props.suggestion}
        </p>
      )}
      <textarea
        className="cd-o__nhap cd-o__nhap--doan"
        id={props.id}
        autoComplete="off"
        maxLength={props.max}
        rows={6}
        required={props.required}
        aria-required={props.required}
        value={props.value}
        aria-describedby={suggestion_id}
        onChange={(e) => props.onChange(e.target.value)}
      />
    </div>
  );
}
