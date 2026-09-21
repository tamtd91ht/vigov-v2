import { useState } from "react";

import { GHI_CHU_TIN, ngayDoc, NUT_MO_BAI, TIEU_DE_KHOI_TIN, TIN_VIHAT } from "../../content/tin-tuc";
import { moRaNgoai } from "../tinh-nang/mo-ra-ngoai";
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung";

import { MessagingGlyph } from "./icons";

/**
 * KHỐI "TIN ViHAT" TRÊN MÀN CHỦ — hai bài mới nhất, dưới dạng **ảnh chụp**.
 *
 * ⚠ KHÔNG CÓ MỘT LỜI GỌI MẠNG NÀO Ở ĐÂY, và đó là điều kiện chứ không phải một sự lười. Lý do đầy
 * đủ nằm ở `content/tin-tuc.ts` — tóm tắt: giai đoạn 1 có ĐÚNG MỘT đích mạng, và thêm một đích là
 * sửa một lời khai trong chính sách quyền riêng tư, tức một thay đổi pháp lý.
 *
 * ⚠ NGÀY ĐĂNG HIỆN CẠNH TIÊU ĐỀ, VÀ CÂU GHI CHÚ NÓI RÕ ĐÂY LÀ BẢN CHỤP NGÀY NÀO. Một khối tin
 * không ghi ngày trông như tin trực tiếp; người đọc không có cách nào biết mình đang xem một thứ
 * đã cũ, và đó là chỗ một khối trang trí biến thành một lời nói sai.
 *
 * MỞ BÀI ĐI QUA `moRaNgoai("tin-tuc", …)` — cửa duy nhất ra ngoài, và `"tin-tuc"` là một đích đã
 * khai trong `content/dich-ra-ngoai.ts`, tức một dòng đã có trong câu khai của chính sách.
 */
export function KhoiTin() {
  /**
   * Chưa mở được trang: một câu, không phải một mã lỗi. Trạng thái CHUNG cho cả khối chứ không
   * cho từng bài — hai câu giống hệt nhau hiện cùng lúc dưới hai thẻ là hai lần nói một điều.
   */
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);

  async function moBai(duong_dan: string) {
    datKhongMoDuoc(!(await moRaNgoai("tin-tuc", duong_dan)));
  }

  return (
    <>
      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <MessagingGlyph />
        </span>
        {TIEU_DE_KHOI_TIN}
      </h2>

      <ul className="tin-ds">
        {TIN_VIHAT.map((bai) => (
          <li className="tin" key={bai.ma}>
            {/* NGÀY ĐỨNG TRƯỚC TIÊU ĐỀ: người đọc quyết định có đọc tiếp hay không bằng đúng con
                số ấy. `<time dateTime>` để trình đọc màn hình đọc ra một ngày, không đọc "hai
                mươi gạch chéo không bảy". */}
            <time className="tin__ngay" dateTime={bai.ngay}>
              {ngayDoc(bai.ngay)}
            </time>
            <h3 className="tin__tieu-de">{bai.tieu_de}</h3>
            <p className="tin__trich">{bai.trich}</p>
            <button
              type="button"
              className="tn-hanh-dong"
              onClick={() => void moBai(bai.duong_dan)}
            >
              {NUT_MO_BAI}
            </button>
          </li>
        ))}
      </ul>

      {khong_mo_duoc && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
      <p className="tin__ghi-chu">{GHI_CHU_TIN}</p>
    </>
  );
}
