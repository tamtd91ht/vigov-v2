import { TIEU_DE_CHINH_SACH } from "../../content/chinh-sach-rieng-tu";
import { COMPANY } from "../../content/company-profile";
import { KHAI_BAO_LOI_GOI, type KhaiBaoLoiGoi } from "../tinh-nang/zalo-api";

import { DocumentGlyph, ShieldGlyph } from "./icons";

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

/**
 * THE ROWS THIS SCREEN SHOWS — the ViHAT intro's calls only, in the intro's own words (owner 11/10/2026: to the user
 * the intro and the commune interface are two separate apps, so this screen never names a commune, a petition or
 * ViGov). Derived from the one table, never a second list:
 *
 *   `intro_app` object             → that view
 *   `intro_app: null`              → hidden (the intro does not make the call)
 *   no `intro_app`, `thuong-mai`   → the main fields, which are the intro's words
 *   anything else (`nha-nuoc`, a `ca-hai` row without a view) → hidden; §3c refuses the second shape
 */
export function introPermissionRows(table: readonly KhaiBaoLoiGoi[]): KhaiBaoLoiGoi[] {
  return table.flatMap((row) => {
    if (row.intro_app === null) return [];
    if (row.intro_app !== undefined) return [{ ...row, ...row.intro_app }];
    return row.nua === "thuong-mai" ? [row] : [];
  });
}

export function QuanLyQuyenScreen(props: {
  onQuayLai: () => void;
  /**
   * Opens the privacy policy (12/10/2026: the policy left the bottom of Liên hệ for its own screen, and stays one
   * tap from here — this screen says what the app CALLS, the policy says where data GOES). Optional only so a test
   * can render the screen alone; `ContactScreen` always passes it.
   */
  onOpenPolicy?: () => void;
}) {
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
        {introPermissionRows(KHAI_BAO_LOI_GOI).map((quyen) => (
          <li className="quyen" key={quyen.api}>
            <h2 className="quyen__ten">{quyen.tinh_nang}</h2>
            {/* "Dùng ở:" thay cho "Màn " — 21/09/2026. `openWebview` nay khai `man` là "Mọi màn"
                (nút Chat nổi có mặt trên mọi màn), và "Màn Mọi màn" là một câu không ai viết ra
                được. Tiền tố mới đọc xuôi với mọi giá trị, kể cả một danh sách nhiều màn. */}
            <p className="quyen__vi-tri">
              Dùng ở: {quyen.man}
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

      {props.onOpenPolicy !== undefined && (
        <button type="button" className="action action--mo" onClick={props.onOpenPolicy}>
          <span className="tile tile--soft" aria-hidden="true">
            <DocumentGlyph className="tile__glyph" />
          </span>
          <span>
            <span className="action__label">Dữ liệu của bạn được dùng thế nào</span>
            <span className="action__value">{TIEU_DE_CHINH_SACH}</span>
          </span>
        </button>
      )}
    </>
  );
}
