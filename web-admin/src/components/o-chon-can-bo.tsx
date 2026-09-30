import { luaChonCanBo } from "@/features/bien-ban/nhan-bien-ban";
import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

/**
 * Ô chọn MỘT cán bộ từ danh bạ chọn người (`GET /api/v1/staff-directory`). GỬI MÃ (`code`, mã nghiệp
 * vụ `CB-…`), HIỆN `Họ tên · Chức vụ` (`nhanLuaChonCanBo`). Giá trị đang lưu mà không còn trong danh
 * bạ vẫn có một dòng — xem `luaChonCanBo`.
 *
 * MỘT Ô CHO MỌI MÀN, KHÔNG MỖI MÀN MỘT BẢN. Dời ra đây từ `features/bien-ban/so-bien-ban.tsx` khi màn
 * Nhiệm vụ cần đúng ô ấy: màn Biên bản import `FormGiaoViec` của màn Nhiệm vụ, nên ô không thể nằm ở
 * một trong hai màn mà không thành vòng import — và một bản chép thứ hai là bản sẽ trôi (luật 9).
 *
 * `<select>` GỐC CỦA TRÌNH DUYỆT, có chủ ý: có nhãn (`htmlFor`), đi bằng bàn phím, và vùng bấm do hệ
 * điều hành vẽ — không một hộp tự dựng nào mà người lớn tuổi phải học cách dùng.
 *
 * `danhBa` LÀ MỘT MẢNG ĐÃ ĐỌC ĐƯỢC. Danh bạ chưa tải hay tải hỏng là chuyện BÊN GỌI phải nói ra bằng
 * chữ — ô này chỉ không bịa lựa chọn nào khi mảng rỗng.
 */
export function OChonCanBo({
  id,
  nhan,
  nhanTrong,
  giaTri,
  danhBa,
  khoa = false,
  dat,
}: {
  id: string;
  nhan: string;
  /** Chữ của lựa chọn rỗng — ý nghĩa của "không chọn ai" khác nhau ở từng ô, nên bên gọi nói. */
  nhanTrong: string;
  giaTri: string;
  danhBa: readonly identity_canBoChonNguoiRa[];
  /** Khoá ô (danh bạ đang tải). */
  khoa?: boolean;
  dat: (ma: string) => void;
}) {
  return (
    <div className="o-nhap">
      <label htmlFor={id}>{nhan}</label>
      <select
        id={id}
        className="o-chon"
        value={giaTri}
        disabled={khoa}
        onChange={(e) => dat(e.target.value)}
      >
        <option value="">{nhanTrong}</option>
        {luaChonCanBo(danhBa, giaTri).map((lc) => (
          <option key={lc.ma} value={lc.ma}>
            {lc.nhan}
          </option>
        ))}
      </select>
    </div>
  );
}
