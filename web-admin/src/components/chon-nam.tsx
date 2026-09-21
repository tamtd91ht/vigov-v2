"use client";

import { danhSachNam } from "@/lib/nam";

/**
 * Ô chọn năm, dùng chung cho màn giải ngân và cho lịch làm việc của xã.
 *
 * NĂM LUÔN HIỆN THÀNH CHỮ TRÊN MÀN HÌNH, và đó là toàn bộ lý do component này tồn tại thay vì
 * một hằng số trong mã gọi: tuyến phía sau bắt buộc tham số `year` và cố ý không mặc định, nên
 * nếu web tự điền một năm ở chỗ không ai nhìn thấy thì nó dựng lại đúng cái mặc định vô hình mà
 * máy chủ vừa từ chối dựng (`lib/nam.ts`).
 *
 * `<label>` gắn thật bằng `htmlFor`, không phải một `<span>` đặt cạnh: ở bề rộng 320px ô chọn
 * nằm xuống dòng dưới nhãn, và một nhãn không gắn thì trình đọc màn hình đọc ô chọn này thành
 * "hộp danh sách" không tên (`15-phu-luc §8`).
 */
export function ChonNam({
  id,
  nhan,
  nam,
  namGoc,
  datNam,
}: {
  id: string;
  nhan: string;
  /** Năm đang chọn. */
  nam: number;
  /**
   * Năm neo của danh sách — năm theo đồng hồ máy, đọc MỘT lần ở màn hình cha.
   *
   * Tách khỏi `nam` có chủ ý: dựng danh sách quanh năm ĐANG CHỌN sẽ làm cửa sổ trượt theo mỗi
   * lần bấm, nên chọn năm cũ nhất rồi mở lại ô chọn là thấy một năm cũ hơn nữa — một ô chọn tự
   * mọc dài ra không có điểm dừng, và cán bộ không còn biết mình đang đứng cách năm nay bao xa.
   */
  namGoc: number;
  datNam: (nam: number) => void;
}) {
  return (
    <p className="chon-nam">
      <label htmlFor={id}>{nhan}</label>{" "}
      <select
        id={id}
        value={nam}
        onChange={(e) => {
          // `value` của `<option>` là chuỗi, luôn là một trong những năm chính danh sách này
          // phát ra — nên không có nhánh "không đọc được số" nào để lo ở đây.
          datNam(Number(e.target.value));
        }}
      >
        {danhSachNam(namGoc).map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>
    </p>
  );
}
