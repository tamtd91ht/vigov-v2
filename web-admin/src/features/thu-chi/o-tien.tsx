import { O_KHONG_TINH_DUOC } from "./nhan-thu-chi";

/**
 * Một ô tiền: hoặc chữ đã định dạng (số, `—`, "Không đọc được"), hoặc dấu KHÔNG TÍNH ĐƯỢC.
 *
 * Câu lý do phải ĐỌC ĐƯỢC bằng trình đọc màn hình và trên màn cảm ứng, không chỉ bằng chuột: một
 * `title` một mình không tới được hai nhóm ấy. Nên câu đi hai đường — `title` cho người rê chuột,
 * và chữ ẩn thị giác (`an-thi-giac`) cho trình đọc. Ở chỗ hẹp (ô của bảng) câu hiện rõ ở danh sách
 * `DanhSachKhongTinh` dưới bảng; ở chỗ rộng (thẻ) `hienLyDo` in câu ngay cạnh dấu.
 */
export function OTien({
  chu,
  lyDo,
  hienLyDo = false,
}: {
  chu: string;
  /** `lyDoKhongTinh(...)` — `null` là ô bình thường. */
  lyDo: string | null;
  hienLyDo?: boolean;
}) {
  if (lyDo === null) return <>{chu}</>;
  return (
    <span title={lyDo}>
      <span aria-hidden="true">⚠ </span>
      {O_KHONG_TINH_DUOC}
      {hienLyDo ? <> — {lyDo}</> : <span className="an-thi-giac"> — {lyDo}</span>}
    </span>
  );
}

/**
 * Danh sách các ô không tính được của một bảng, HIỆN RÕ dưới bảng.
 *
 * Người dùng màn cảm ứng không rê chuột được lên ô để đọc `title`, và câu lý do là việc họ phải làm
 * ("gỡ đợt ghi nhầm", "kiểm tra các số quá lớn ở các dòng bên dưới"). Không có ô nào thì không vẽ.
 */
export function DanhSachKhongTinh({
  tieuDe,
  o,
}: {
  tieuDe: string;
  o: readonly { khoa: string; noi: string; lyDo: string }[];
}) {
  if (o.length === 0) return null;
  return (
    <div className="thong-bao-loi">
      <p>
        {tieuDe} ({o.length})
      </p>
      <ul>
        {o.map((x) => (
          <li key={x.khoa}>
            <strong>{x.noi}</strong>: {x.lyDo}
          </li>
        ))}
      </ul>
    </div>
  );
}
