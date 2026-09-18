import { MUC_CHINH_SACH_QUYEN } from "bien-the/quyen";

import {
  CAU_DAU,
  MUC_SAU_QUYEN,
  MUC_TRUOC_QUYEN,
  NGAY_HIEU_LUC,
  PHIEN_BAN_CHINH_SACH,
  TIEU_DE_CHINH_SACH,
} from "../../content/chinh-sach-rieng-tu";

/**
 * Chính sách quyền riêng tư, vẽ ở cuối màn Liên hệ.
 *
 * VÌ SAO KHÔNG PHẢI MỘT TAB THỨ NĂM: bốn tab đang nói bốn việc của một app giới thiệu. Thêm tab
 * thứ năm cho một văn bản tĩnh làm loãng cả thanh tab, và một người mở app để xem công ty làm gì
 * thì không bấm vào tab "Chính sách". Màn Liên hệ là chỗ người ta đã tìm thông tin pháp lý.
 *
 * VÌ SAO `<details>` CHỨ KHÔNG PHẢI MỘT MÀN RIÊNG: mở/đóng là hành vi có sẵn của trình duyệt —
 * bàn phím, trình đọc màn hình và nút Back của Zalo đều đã hiểu nó, không cần một dòng JS nào và
 * không cần thêm trạng thái vào vỏ app. Tiêu đề và câu quan trọng nhất nằm NGOÀI `<details>`, vì
 * một chính sách phải đọc được mà không cần bấm gì: người duyệt cuộn tới là thấy.
 *
 * VÌ SAO ĐÁNH SỐ Ở ĐÂY CHỨ KHÔNG VIẾT VÀO TIÊU ĐỀ: bản `goc` có 7 mục, bản `quyen` có 8 vì mục
 * ba quyền chèn vào giữa. Một con số viết cứng sẽ đúng ở một bản và sai ở bản kia — sai trong
 * một văn bản pháp lý, và không có gì đỏ lên. Xem `content/chinh-sach-rieng-tu.ts`.
 */
export function ChinhSachRiengTu() {
  const muc = [...MUC_TRUOC_QUYEN, ...MUC_CHINH_SACH_QUYEN, ...MUC_SAU_QUYEN];

  return (
    <section className="chinh-sach" id="chinh-sach-rieng-tu">
      <h2 className="section-title">{TIEU_DE_CHINH_SACH}</h2>

      {/* Câu này đứng ngoài mọi `<details>` có chủ đích: nó là điều quan trọng nhất văn bản
          phải nói, và nó đúng với cả ba biến thể bản dựng. */}
      <p className="chinh-sach__cau-dau">{CAU_DAU}</p>

      <p className="chinh-sach__hieu-luc">
        Hiệu lực từ {NGAY_HIEU_LUC} · Phiên bản {PHIEN_BAN_CHINH_SACH}
      </p>

      <ol className="chinh-sach__muc">
        {muc.map((m, chi_so) => (
          <li key={m.ma}>
            <details className="chinh-sach__phan">
              <summary className="chinh-sach__tieu-de">
                {chi_so + 1}. {m.tieu_de}
              </summary>
              {m.doan.map((doan, so_doan) => (
                // Khoá theo vị trí: các đoạn của một mục là một danh sách tĩnh, không bao giờ
                // được sắp xếp lại hay chèn thêm lúc chạy, nên vị trí là khoá ổn định.
                <p className="chinh-sach__doan" key={`${m.ma}-${so_doan}`}>
                  {doan}
                </p>
              ))}
            </details>
          </li>
        ))}
      </ol>
    </section>
  );
}
