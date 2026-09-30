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

export function ONhapDong(
  props: ChungProps & { kieu_ban_phim?: "text" | "tel" },
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
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => props.onDoi(e.target.value)}
      />
    </div>
  );
}

export function ONhapDoan(props: ChungProps & { bat_buoc?: boolean }) {
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
      <textarea
        className="cd-o__nhap cd-o__nhap--doan"
        id={props.id}
        autoComplete="off"
        maxLength={props.toi_da}
        rows={6}
        required={props.bat_buoc}
        aria-required={props.bat_buoc}
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => props.onDoi(e.target.value)}
      />
    </div>
  );
}
