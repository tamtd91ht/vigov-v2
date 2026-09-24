"use client";

/**
 * Hộp xác nhận GỠ — **có ô lý do**, và ô ấy không phải để cho đẹp.
 *
 * §6 chỉ vẽ "hộp xác nhận"; máy chủ đòi `reason` trong thân và từ chối 400 khi thiếu (luật 7 bất
 * biến 1 kể tên `delete_reason`). Một hộp chỉ có nút Đồng ý sẽ nhận 400 ở mọi lần bấm.
 *
 * TỆP RIÊNG vì hai nơi dùng nó — bảng (gỡ bảng, gỡ khoản mục) và hộp `⇄` (gỡ đợt) — và hộp `⇄`
 * nằm ở tệp khác; để nó trong `bang-thu-chi.tsx` là một vòng nhập giữa hai tệp.
 *
 * Lý do chỉ gồm khoảng trắng bị chặn ở đây chứ không chỉ bằng `required`: trình duyệt coi `" "` là
 * đã điền, còn máy chủ thì không, và cán bộ nhận một 400 cho một ô trông như đã điền.
 */
export function FormGoKemLyDo({
  tieuDe,
  canhBao,
  dangGui,
  huy,
  luu,
  idTruong = "go-reason",
}: {
  tieuDe: string;
  canhBao: string;
  dangGui: boolean;
  huy: () => void;
  luu: (lyDo: string) => void;
  /** `id` của ô lý do — hộp `⇄` truyền một `id` khác để không trùng với hộp gỡ của bảng. */
  idTruong?: string;
}) {
  return (
    <form
      className="khoi-chua-khai"
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const lyDo = String(fd.get("reason") ?? "").trim();
        if (lyDo === "") return;
        luu(lyDo);
      }}
    >
      <h3>{tieuDe}</h3>
      <p className="hau-qua">{canhBao}</p>
      <p>
        <label htmlFor={idTruong}>Lý do gỡ (bắt buộc, được lưu cùng bản ghi)</label>{" "}
        <input
          id={idTruong}
          name="reason"
          className="o-nhap"
          type="text"
          required
          maxLength={500}
        />
      </p>
      <button type="submit" className="nut-chinh" disabled={dangGui}>
        Gỡ
      </button>{" "}
      <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
        Huỷ
      </button>
    </form>
  );
}
