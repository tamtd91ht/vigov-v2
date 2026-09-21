/**
 * Màn CHỌN XÃ — đường cuối cùng, và là đường LUÔN có mặt.
 *
 * `skills/zalo-miniapp-multi-tenant` xếp thứ tự cho công dân chứ không cho mô hình dữ liệu:
 * gần đây → GPS gợi ý → tìm theo tên → duyệt theo tỉnh. Bản này chỉ có bậc cuối, và cố ý:
 *
 *   • "Gần đây" cần một hồ sơ công dân trên máy chủ. Chưa có.
 *   • GPS thì **chưa tới lúc**, và khi tới thì nó GỢI Ý chứ không QUYẾT: toạ độ giả được, còn
 *     ranh giới trong đô thị chạy giữa lòng đường — người đứng nhầm bên vỉa hè không vì thế mà
 *     thuộc xã khác. Giai đoạn 1 lại còn không được đụng `navigator.geolocation` (xem
 *     `phase1-collects-nothing.test.ts`).
 *   • Ô tìm theo tên **cố tình không có**: một ô tìm kiếm là một thẻ `<input>`, mà giai đoạn 1
 *     không có thẻ nhập liệu nào — đó là thứ làm luật 3 đúng BẰNG CẤU TRÚC chứ không bằng lập
 *     luận. Tám dòng thì cuộn nhanh hơn gõ. Khi danh mục thật có hàng nghìn xã thì ô tìm kiếm
 *     quay lại cùng lúc với `ListTenants`, và lúc ấy nó được bàn tử tế.
 *
 * Danh sách là danh sách BẤM ĐƯỢC, không phải danh sách để đọc: mỗi dòng là một `<button>` cao
 * tối thiểu 44px, vì một mục tiêu nhỏ hơn thế là mục tiêu một bàn tay run không bấm trúng.
 */
import { DEMO_DANH_MUC_XA, DEMO_GHI_CHU, type XaDemo } from "./demo-danh-muc-xa";
import { LOI_NHAN, type LiDoPhaiChon } from "./goi-y";

type Props = {
  /**
   * Vì sao màn này hiện ra. `null` khi công dân tự bấm "Đổi xã" — lúc đó không cần giải thích
   * gì cả, chính họ vừa yêu cầu.
   */
  li_do: LiDoPhaiChon | null;
  onChon: (xa: XaDemo) => void;
  onXemGioiThieu: () => void;
};

/** Mũi tên chỉ hướng đi tiếp. Trang trí — chữ bên cạnh mới là nội dung. */
function GlyphTiep({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      aria-hidden="true"
    >
      <path d="m9 5 7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ChonXaScreen({ li_do, onChon, onXemGioiThieu }: Props) {
  return (
    <section className="chon-xa" aria-labelledby="chon-xa-tieu-de">
      <h1 className="goi-y__tieu-de" id="chon-xa-tieu-de">
        Chọn xã để tiếp tục
      </h1>

      {/* Câu này nói VIỆC CẦN LÀM, không nói mã lỗi (README §Error message shape). */}
      {li_do && <p className="chon-xa__li-do">{LOI_NHAN[li_do]}</p>}

      <ul className="chon-xa__ds">
        {DEMO_DANH_MUC_XA.map((xa) => (
          <li key={xa.id}>
            <button type="button" className="chon-xa__dong" onClick={() => onChon(xa)}>
              <span className="chon-xa__chu">
                <strong className="chon-xa__ten">{xa.ten}</strong>
                {/* Tỉnh/thành đứng dưới tên, không phải sau dấu phẩy: hai xã trùng tên ở hai
                    tỉnh là chuyện bình thường, và đây là dòng phân biệt chúng. */}
                <span className="chon-xa__tinh">{xa.tinh}</span>
              </span>
              <GlyphTiep className="chon-xa__tiep" />
            </button>
          </li>
        ))}
      </ul>

      {/* Nói thẳng đây là dữ liệu mẫu. Một danh mục xã đặt ra mà không ghi chú thì người xem có
          quyền hiểu là danh mục thật — và với một hệ thống mang tên cơ quan nhà nước thì hiểu
          nhầm ấy đắt hơn nhiều so với một dòng chữ nhỏ. */}
      <p className="demo-ghi-chu">{DEMO_GHI_CHU}</p>

      {/* Giai đoạn 1 vẫn phải tới được: đó là bản Zalo đã duyệt. Một tham số `t` trên đường
          liên kết không được phép khoá vĩnh viễn phần giới thiệu công ty. */}
      <button type="button" className="goi-y__nut goi-y__nut--nhat" onClick={onXemGioiThieu}>
        Xem giới thiệu ViHAT Group
      </button>
    </section>
  );
}
