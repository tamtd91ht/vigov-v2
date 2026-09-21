import { useState } from "react";

import { COMPANY } from "../../content/company-profile";
import { moRaNgoai } from "../tinh-nang/mo-ra-ngoai";
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung";

import { GlobeGlyph } from "./icons";

/**
 * NÚT MỞ TRANG WEB CHÍNH THỨC — một chỗ viết, hai chỗ dùng (màn Liên hệ · trang chi tiết giải pháp).
 *
 * ⚠ ĐÃ ĐỔI TỪ `<a target="_blank">` SANG MỘT NÚT ĐI QUA `openWebview` — 21/09/2026, và lý do là
 * hành vi bên trong Zalo chứ không phải khẩu vị:
 *
 *   Một liên kết ngoài mở bằng thẻ `a` trong Zalo hiện ra KHÔNG có thanh điều hướng và KHÔNG có
 *   đường quay lại Mini App — người dùng phải đóng cả app để về chỗ cũ. `openWebview` là API nền
 *   tảng dựng đúng cho việc này và nó trả người dùng về đúng chỗ họ đang đứng. Ứng dụng đã mở mọi
 *   liên kết khác bằng đường ấy (bản đồ, mã QR); hai neo website là hai chỗ CUỐI CÙNG còn đi bằng
 *   đường kia, và giữ lại hai hình dạng cho cùng một việc là giữ lại chỗ để chúng lệch nhau.
 *
 *   Hệ quả nói thẳng: ngoài Zalo (trình duyệt trên máy tính) nút này KHÔNG mở được, trong khi cái
 *   neo cũ mở được. Nên nhánh hỏng nói ra bằng đúng câu mà mọi chỗ mở ngoài khác của app dùng —
 *   `LOI_MO_NGOAI` — chứ không im lặng.
 *
 * ⚠ VẮNG MẶT KHI KHÔNG CÓ ĐỊA CHỈ. `COMPANY.website` từng trống hai ngày sau khi đổi pháp nhân;
 * một nút "Website" bấm vào không đi đâu tệ hơn hẳn một nút không có mặt.
 */
export function NutWebsite({ lop_them }: { lop_them?: string }) {
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);
  const dia_chi = COMPANY.website;

  async function mo(duong_dan: string) {
    datKhongMoDuoc(!(await moRaNgoai("trang-chu", duong_dan)));
  }

  if (dia_chi === undefined) return null;

  return (
    <>
      <button
        type="button"
        className={`action action--mo${lop_them === undefined ? "" : ` ${lop_them}`}`}
        onClick={() => void mo(dia_chi)}
      >
        <span className="tile tile--soft" aria-hidden="true">
          <GlobeGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Website</span>
          {/* ĐỊA CHỈ HIỆN RA NGUYÊN VĂN: người dùng phải đọc được mình sắp đi đâu TRƯỚC khi bấm,
              nhất là khi cái mở ra là một trình duyệt bên trong một ứng dụng khác. */}
          <span className="action__value">{dia_chi}</span>
        </span>
      </button>
      {khong_mo_duoc && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
    </>
  );
}
