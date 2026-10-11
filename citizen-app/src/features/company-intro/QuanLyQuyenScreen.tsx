import { COMPANY } from "../../content/company-profile";
import { KHAI_BAO_LOI_GOI, type NuaUngDung } from "../tinh-nang/zalo-api";

import { ShieldGlyph } from "./icons";

/**
 * Màn "Quản lý quyền" — app này xin nền tảng những gì, ở màn nào, để làm gì.
 *
 * ⚠ MÀN NÀY KHÔNG CÓ MỘT DANH SÁCH NÀO CỦA RIÊNG NÓ. Nó vẽ `KHAI_BAO_LOI_GOI`, bảng khai nằm
 * ngay trong `features/tinh-nang/zalo-api.ts` — tệp DUY NHẤT được phép gọi `zmp-sdk`.
 *
 *   Một danh sách chép tay ở đây sẽ đúng đúng một ngày: ngày nó được chép. Lời gọi thứ mười ba
 *   được thêm vào sáu tuần nữa, không ai nhớ quay lại màn này, và app xin một quyền mà không màn
 *   nào nói ra — trong chính hồ sơ đang xin những quyền đó. `ranh-gioi-hai-nua.test.ts` §3c đối
 *   chiếu bảng khai với mã nguồn và đỏ lên trước khi điều đó xảy ra.
 *
 * ⚠ TRẠNG THÁI KHÔNG BAO GIỜ CHỈ BẰNG MÀU. "Zalo sẽ hỏi bạn" và "Zalo không hỏi" là hai CÂU CHỮ,
 * không phải một chấm xanh và một chấm xám: người phân biệt được hai chấm ấy là người ít cần màn
 * này nhất.
 *
 * ⚠ MÀN NÀY KHÔNG BẬT/TẮT ĐƯỢC QUYỀN NÀO, VÀ NÓ NÓI THẲNG RA. Quyền do Zalo giữ, gỡ ở phần cài
 * đặt của Zalo. Vẽ một công tắc ở đây là vẽ một điều khiển không điều khiển gì — thứ tệ hơn hẳn
 * một câu hướng dẫn.
 */

/** Nhãn tiếng Việt của cột `nua`. Một bản đồ, để không có câu nào viết rải trong JSX. */
const NHAN_NUA: Record<NuaUngDung, string> = {
  "thuong-mai": "Phần giới thiệu doanh nghiệp",
  "nha-nuoc": "Phần dịch vụ công",
  "ca-hai": "Cả hai phần",
};

export function QuanLyQuyenScreen(props: { onQuayLai: () => void }) {
  return (
    <>
      <button type="button" className="quay-lai" onClick={props.onQuayLai}>
        ‹ Quay lại Liên hệ
      </button>

      <section className="banner">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <ShieldGlyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">Quản lý quyền</h1>
        </div>
      </section>

      <section className="card">
        <p className="card__body">
          Các chức năng của Zalo mà ứng dụng {COMPANY.name} sử dụng, mục đích sử dụng và dữ liệu liên
          quan.
        </p>
        <p className="card__body">
          Bạn có thể thu hồi quyền bất cứ lúc nào trong phần cài đặt của Zalo.
        </p>
      </section>

      <ul className="quyen-ds">
        {KHAI_BAO_LOI_GOI.map((quyen) => (
          <li className="quyen" key={quyen.api}>
            <h2 className="quyen__ten">{quyen.tinh_nang}</h2>
            {/* "Dùng ở:" thay cho "Màn " — 21/09/2026. `openWebview` nay khai `man` là "Mọi màn"
                (nút Chat nổi có mặt trên mọi màn), và "Màn Mọi màn" là một câu không ai viết ra
                được. Tiền tố mới đọc xuôi với mọi giá trị, kể cả một danh sách nhiều màn. */}
            <p className="quyen__vi-tri">
              Dùng ở: {quyen.man} · {NHAN_NUA[quyen.nua]}
            </p>
            <p className="quyen__de-lam-gi">{quyen.de_lam_gi}</p>
            <p className="quyen__hoi">
              {quyen.hoi_nguoi_dung
                ? "Cần bạn cho phép."
                : "Không truy cập dữ liệu cá nhân."}
            </p>
            <p className="quyen__ra-ngoai">
              {quyen.roi_khoi_may === ""
                ? "Dữ liệu không rời khỏi thiết bị."
                : quyen.roi_khoi_may}
            </p>
          </li>
        ))}
      </ul>
    </>
  );
}
