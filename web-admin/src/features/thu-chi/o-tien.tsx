import { TriangleAlert } from "lucide-react";

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
  // The mark is decorative (`aria-hidden`): a screen reader reads "Không tính được — <câu>", never
  // "cảnh báo" first. A lucide icon instead of the `⚠` glyph (ADR 0068 §2), same words.
  return (
    <span title={lyDo} className="inline-flex flex-wrap items-baseline gap-x-1 text-warning-600">
      <TriangleAlert
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className="size-3.5 shrink-0 self-center"
      />
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
  // Red on purpose: each line is a figure that is wrong or unreadable and has to be fixed.
  return (
    <div className="thong-bao-loi m-0 rounded-xl border border-danger-200 bg-danger-50 px-4 py-3">
      <p className="m-0 inline-flex items-center gap-1.5 font-semibold">
        <TriangleAlert aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0" />
        {tieuDe} ({o.length})
      </p>
      <ul className="m-0 mt-1 pl-5">
        {o.map((x) => (
          <li key={x.khoa}>
            <strong>{x.noi}</strong>: {x.lyDo}
          </li>
        ))}
      </ul>
    </div>
  );
}
