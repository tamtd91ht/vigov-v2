/**
 * SỐ HOÁ DANH THIẾP GIẤY — `requestCameraPermission` + `openMediaPicker`.
 *
 * ⚠ RANH GIỚI ĐƯỢC NÓI RA, KHÔNG ĐƯỢC GIẢ VỜ VƯỢT QUA:
 *
 *   Bản này **chưa đọc được chữ trên ảnh**. Bóc tên, số điện thoại và email ra khỏi một tấm ảnh
 *   cần OCR, và OCR cần một bước máy chủ — thứ ứng dụng này không có và dây bẫy trong
 *   `phase1-collects-nothing.test.ts` cấm. Nên ở đây KHÔNG có một dòng mã nào giả vờ đang nhận
 *   dạng, không có một câu "đang phân tích…", và màn hình nói thẳng điều đó bằng tiếng Việt
 *   (`SO_HOA_THIEP.chua_doc_duoc_chu`). Một thanh tiến trình chạy cho một việc không xảy ra là
 *   nói dối bằng giao diện với người vừa đưa ảnh của mình vào.
 *
 * ⚠ LUẬT 3 — ẢNH MỘT TẤM DANH THIẾP GIẤY LÀ DỮ LIỆU CÁ NHÂN CỦA NGƯỜI KHÁC:
 *
 *   Hiện lên màn hình thì được — đó là toàn bộ việc người dùng vừa yêu cầu. Ngoài ra thì không:
 *   không `console.log` đường dẫn, không `console.log` nội dung, không chỗ lưu nào, không tên
 *   tệp, không khoá bộ nhớ đệm, không một lời gọi mạng nào. Đường dẫn sống trong `useState` và
 *   mất đi khi chọn ảnh khác hoặc rời màn hình.
 *
 *   Đường dẫn ấy đi vào thuộc tính `src` của một thẻ `<img>` — có cân nhắc, cùng lập luận với
 *   `mailto:` trong `ManDanhThiep.tsx`: luật 3 nhắm tới những URL ĐƯỢC GHI LẠI (đường gọi mạng,
 *   tên tệp, khoá cache). Đây là một đường dẫn TẠM trên chính máy người dùng, không rời khỏi
 *   tiến trình đang chạy, không được ghi ở đâu.
 *
 * ⚠ VÀ QUAN TRỌNG NHẤT: `chonAnhTuMay` KHÔNG truyền `serverUploadUrl` — xem `zalo-api.ts`. Có
 * nó thì ảnh của người dùng được SDK tải thẳng lên một máy chủ. Không có nó thì ảnh không rời
 * khỏi máy, và câu ấy trong chính sách quyền riêng tư là một sự thật về cấu trúc.
 */
import { useState } from "react";

import { TargetGlyph } from "../company-intro/icons";

import { KhungTinhNang, type TrangThai } from "./khung";
import { SO_HOA_THIEP } from "./noi-dung";
import { chonAnhTuMay, rungMotNhip, xinQuyenMayAnh } from "./zalo-api";

/**
 * Câu trả lời của người dùng cho lời xin quyền máy ảnh, THUẦN.
 *
 * `userAllow === false` KHÔNG phải một lỗi: lời gọi đã thành công, và câu trả lời là "không".
 * Nên nhánh ấy không có màu đỏ, không có mã lỗi, và nó nói việc còn làm được — chọn một ảnh đã
 * chụp sẵn. Từ chối không làm mất tính năng.
 */
export function TraLoiQuyenMayAnh({ cho_phep }: { cho_phep: boolean }) {
  return cho_phep ? (
    <p className="tn__xong">{SO_HOA_THIEP.cho_phep}</p>
  ) : (
    <p className="tn__loi">{SO_HOA_THIEP.tu_choi_quyen}</p>
  );
}

/**
 * Ảnh vừa chọn, THUẦN.
 *
 * `onError` có mặt vì đường dẫn tạm của nền tảng có thể không nạp được trong webview trên một
 * số máy — và một khung ảnh vỡ không nói được gì với người đang chờ. Nó đổi sang một câu tiếng
 * Việt nói việc cần làm tiếp, đúng README §Error message shape.
 */
export function AnhDanhThiep({
  duong_dan,
  hong,
  onHong,
}: {
  duong_dan: string;
  hong: boolean;
  onHong: () => void;
}) {
  if (hong) return <p className="tn__loi">{SO_HOA_THIEP.khong_hien_duoc_anh}</p>;

  return (
    <figure className="tn__anh">
      {/* `alt` mô tả VAI TRÒ của ảnh, không mô tả nội dung: ứng dụng không đọc được chữ trên
          ảnh, nên một `alt` kể tên và số điện thoại sẽ là một câu bịa. */}
      <img className="tn__anh-hinh" src={duong_dan} alt={SO_HOA_THIEP.nhan_anh} onError={onHong} />
      <figcaption className="tn__anh-chu">{SO_HOA_THIEP.nhan_anh}</figcaption>
    </figure>
  );
}

export function SoHoaThiepGiay() {
  const [quyen, datQuyen] = useState<TrangThai<boolean>>({ kieu: "chua-goi" });
  const [anh, datAnh] = useState<TrangThai<readonly string[]>>({ kieu: "chua-goi" });
  const [anh_hong, datAnhHong] = useState(false);

  async function chonAnh() {
    datAnhHong(false);
    datAnh({ kieu: "dang-cho" });
    const ket_qua = await chonAnhTuMay();
    datAnh(ket_qua);
    if (ket_qua.kieu === "xong" && ket_qua.du_lieu.length > 0) await rungMotNhip();
  }

  const dang_chon = anh.kieu === "dang-cho";
  const duong_dan = anh.kieu === "xong" ? anh.du_lieu[0] : undefined;

  return (
    <KhungTinhNang
      ma="so-hoa-thiep"
      trang_thai={quyen}
      onBam={() => {
        void (async () => {
          datQuyen({ kieu: "dang-cho" });
          datQuyen(await xinQuyenMayAnh());
        })();
      }}
      cap_tieu_de="h2"
      glyph={<TargetGlyph className="tn__glyph" />}
      dan_nhap={SO_HOA_THIEP.dan_nhap}
      veKetQua={(cho_phep) => <TraLoiQuyenMayAnh cho_phep={cho_phep} />}
      duoi_cung={
        <>
          <button
            type="button"
            className="tn__nut-phu"
            onClick={() => void chonAnh()}
            disabled={dang_chon}
            aria-busy={dang_chon}
          >
            {dang_chon ? SO_HOA_THIEP.dang_chon_anh : SO_HOA_THIEP.nut_chon_anh}
          </button>

          <div className="tn__ket-qua" role="status">
            {duong_dan !== undefined && (
              <AnhDanhThiep
                duong_dan={duong_dan}
                hong={anh_hong}
                onHong={() => datAnhHong(true)}
              />
            )}
            {anh.kieu === "xong" && duong_dan === undefined && (
              <p className="tn__loi">{SO_HOA_THIEP.khong_chon_anh}</p>
            )}
            {anh.kieu === "tu-choi" && <p className="tn__loi">{SO_HOA_THIEP.khong_chon_anh}</p>}
          </div>

          {/* HAI CÂU NÀY LUÔN HIỆN, KHÔNG CHỜ TỚI LÚC CÓ ẢNH. Người quyết định có đưa ảnh của
              một người khác vào hay không cần đọc chúng TRƯỚC, không phải sau. */}
          <p className="tn__ranh-gioi">{SO_HOA_THIEP.chua_doc_duoc_chu}</p>
          <p className="tn__rieng-tu">{SO_HOA_THIEP.anh_khong_roi_may}</p>
        </>
      }
    />
  );
}
