import { nhanBoPhan } from "@/features/cau-hinh/nhan-can-bo";
import { traTen, type BangTraDanhMuc, type KetTra } from "@/features/cau-hinh/tra-danh-muc";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  ariaSua,
  COT_CHUC_VU,
  COT_DI_DONG,
  COT_HO_TEN,
  COT_KHOI,
  COT_MAY_BAN,
  NUT_SUA_THONG_TIN,
} from "./nhan-danh-ba";

/**
 * Bảng cán bộ của màn **Danh bạ** — `docs/ui-ux/12-danh-ba-can-bo.md §4`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY KHÔNG PHẢI BẢN SAO CỦA `features/cau-hinh/danh-ba-can-bo.tsx`, VÀ SỰ KHÁC NHAU LÀ SỰ KHÁC
 * NHAU MÀ CHÍNH ĐẶC TẢ ĐẶT RA (§1): tab `Cấu hình → Người dùng` quản lý **tài khoản đăng nhập** —
 * mã cán bộ, vai trò, trạng thái khoá, đăng nhập gần nhất, cấp tài khoản, đặt lại mật khẩu. Màn
 * này quản lý **thông tin liên hệ**: gọi ai, ở khối nào, số nào.
 *
 * Vì vậy bảng ở đây KHÔNG dựng lại `BangCanBo` của tab kia: mười một cột của nó mang đúng những
 * thứ màn liên hệ không nói tới, và sáu hành động của nó gồm khoá tài khoản với đặt lại mật khẩu —
 * hai việc có hậu quả nặng, không thuộc về một màn danh bạ. Bảy dòng dữ liệu chung thì lấy từ
 * CHÍNH `identity.canBoTomTat` của hợp đồng, không từ một kiểu chép lại.
 *
 * PHẦN DÙNG LẠI THÌ DÙNG LẠI THẬT, KHÔNG CHÉP: phép tra id bộ phận → tên (`traTen`) và năm câu
 * chữ cho năm ca tra (`nhanBoPhan`) đến thẳng từ `features/cau-hinh/`. Chép chúng sang đây là hai
 * bản sẽ trôi (luật 9, cấm #2) — và bản trôi sẽ là bản nói sai về một bộ phận đã bị xoá mềm.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỘT CHECKBOX, và sự vắng mặt ấy không phải quên. Ô chọn của đặc tả (§2, §4) chỉ phục
 * vụ ĐÚNG MỘT việc: thanh hành động hàng loạt `Thêm / Rút khỏi danh bạ Mini App`. Câu mở #12 đã bỏ
 * hẳn thao tác hàng loạt ấy, nên một cột checkbox ở đây là hai mươi ô tick không chọn cho việc gì.
 *
 * THUẦN TRÌNH BÀY: không đọc mạng, không giữ state. Nhờ vậy kết xuất được bằng `react-dom/server`
 * trong Node và bài kiểm hỏi thẳng được "dòng này có RA TỚI TRANG không".
 */
export function BangLienHe({
  danhSach,
  traBoPhan,
  onSua,
}: {
  danhSach: readonly identity_canBoTomTat[];
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số — không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  onSua: (cb: identity_canBoTomTat) => void;
}) {
  return (
    // `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím. Dưới 768px bảng cuộn
    // ngang chứ không đổi thành thẻ (`15-phu-luc §7`): đổi `display` của phần tử bảng làm mất ngữ
    // nghĩa bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng.
    <div className="bang-cuon" role="region" aria-label="Danh bạ cán bộ của đơn vị" tabIndex={0}>
      <table className="bang-can-bo">
        <caption className="an-thi-giac">
          Danh bạ cán bộ của đơn vị: họ tên, chức vụ, khối/đơn vị và số liên hệ.
        </caption>
        <thead>
          <tr>
            <th scope="col">{COT_HO_TEN}</th>
            <th scope="col">{COT_CHUC_VU}</th>
            <th scope="col">{COT_KHOI}</th>
            {/* HAI CỘT SỐ, KHÔNG MỘT — xem `COT_MAY_BAN` / `COT_DI_DONG` trong `nhan-danh-ba.ts`
                (#16). Đặc tả §4 chỉ vẽ một cột `Di động`; gộp lại dưới nhãn ấy là đặt số công vụ
                và dữ liệu cá nhân dưới cùng một cái tên. */}
            <th scope="col">{COT_MAY_BAN}</th>
            <th scope="col">{COT_DI_DONG}</th>
            <th scope="col">
              <span className="an-thi-giac">Hành động</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {danhSach.map((cb) => (
            <tr key={cb.id}>
              <td>
                {/* Tên đậm + dòng phụ thư điện tử — đúng §4. Tên KHÔNG phải nút mở chi tiết như
                    đặc tả vẽ: màn này không có khối chi tiết riêng, vì mọi trường `canBoTomTat`
                    mang nghĩa liên hệ đều đã nằm ngay trên dòng. Một nút mở ra đúng thứ đang hiện
                    là một nút không làm gì. */}
                <span className="ten-can-bo">{cb.full_name}</span>
                <span className="dong-phu">{cb.email}</span>
              </td>
              <td>{cb.position}</td>
              <td>
                <OKhoi ket={traTen(traBoPhan, cb.department_id)} />
              </td>
              {/* HIỆN NGUYÊN VĂN THỨ MÁY CHỦ TRẢ, KHÔNG CHE VÀ KHÔNG ĐỊNH DẠNG LẠI. Câu mở #11
                  chốt 22/09/2026: không che trong nội bộ xã. Máy chủ trả số nguyên vẹn, và việc
                  che còn nguyên ở bản xuất Excel cùng mọi đường ra ngoài cơ quan — hai bề mặt
                  chưa tồn tại. Định dạng lại thành `0900 000 001` như ví dụ đặc tả thì số copy ra
                  không gọi được và không tìm lại được. */}
              <td>{cb.phone}</td>
              <td>{cb.mobile}</td>
              <td>
                <span className="o-thao-tac">
                  {/* MỘT HÀNH ĐỘNG, KHÔNG BA. Đặc tả §4 vẽ ✕ (rút khỏi Mini App), ✎ (sửa) và 🗑
                      (xoá); hai cái ngoài cùng không có tuyến nào trong hợp đồng và mỗi cái còn bị
                      một quyết định đã chốt chặn (#12 và #10). Xem phần chưa mở ở `nhan-danh-ba.ts`. */}
                  <button
                    type="button"
                    className="nut-phu"
                    aria-label={ariaSua(cb.full_name)}
                    onClick={() => onSua(cb)}
                  >
                    {NUT_SUA_THONG_TIN}
                  </button>
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Ô cột "Khối / đơn vị".
 *
 * KHÔNG BAO GIỜ DỰNG RA MỘT Ô TRỐNG: `nhanBoPhan` luôn trả một câu, kể cả khi không tra được. Lớp
 * CSS đi theo LOẠI kết quả chứ không theo câu chữ, để "chưa phân bộ phận" (một trạng thái bình
 * thường) và "không tra được" (một dòng dữ liệu lệch, chỉ người quản trị sửa được) không trông
 * giống nhau.
 */
function OKhoi({ ket }: { ket: KetTra }) {
  return <span className={lopKhoi(ket)}>{nhanBoPhan(ket)}</span>;
}

function lopKhoi(ket: KetTra): string | undefined {
  switch (ket.loai) {
    case "coTen":
      return undefined;
    case "khongTraDuoc":
      return "nhan-lech";
    default:
      return "nhan-trong";
  }
}
