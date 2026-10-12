import {
  CAU_DAU,
  MUC_CHINH_SACH,
  NGAY_HIEU_LUC,
  PHIEN_BAN_CHINH_SACH,
} from "../../content/chinh-sach-rieng-tu";

/**
 * Thân chính sách quyền riêng tư — vẽ trong màn con `PrivacyPolicyScreen` từ 12/10/2026 (trước đó ở
 * cuối màn Liên hệ). Tiêu đề nay là `<h1>` của màn con ấy, nên khối này không in lại nó.
 *
 * VÌ SAO KHÔNG PHẢI MỘT TAB THỨ NĂM: bốn tab đang nói bốn việc của một app giới thiệu. Thêm tab
 * thứ năm cho một văn bản tĩnh làm loãng cả thanh tab, và một người mở app để xem công ty làm gì
 * thì không bấm vào tab "Chính sách". Đường vào là một hàng trên Liên hệ và một hàng trên Quản lý quyền.
 *
 * VÌ SAO TỪNG MỤC VẪN LÀ `<details>`: mở/đóng là hành vi có sẵn của trình duyệt — bàn phím và
 * trình đọc màn hình đều đã hiểu nó, không cần một dòng JS nào. Câu quan trọng nhất nằm NGOÀI
 * `<details>`, vì một chính sách phải đọc được mà không cần bấm gì.
 *
 * VÌ SAO ĐÁNH SỐ Ở ĐÂY CHỨ KHÔNG VIẾT VÀO TIÊU ĐỀ: một con số viết cứng trong tiêu đề sẽ lệch
 * ngay lần thêm hoặc bớt một mục — lệch trong một văn bản pháp lý, và không có gì đỏ lên. Xem
 * `content/chinh-sach-rieng-tu.ts`.
 */
export function ChinhSachRiengTu() {
  const muc = MUC_CHINH_SACH;

  return (
    <section className="chinh-sach" id="chinh-sach-rieng-tu">
      {/* Câu này đứng ngoài mọi `<details>` có chủ đích: nó là điều quan trọng nhất văn bản
          phải nói, và nó đúng với cả hai biến thể bản dựng. */}
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
