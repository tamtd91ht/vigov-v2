/**
 * MÀN "GỢI Ý GIẢI PHÁP" — ba câu hỏi, một danh sách gợi ý, BA hành động. Chạy hết trên máy.
 *
 * ⚠ MỖI LỰA CHỌN LÀ MỘT `<button>`, KHÔNG PHẢI `<input type="radio">` HAY `<select>`.
 *
 *   `phase1-collects-nothing.test.ts` cấm `<form|input|textarea|select>`, và từ 22/09/2026 nó miễn
 *   cho ĐÚNG MỘT TỆP — ô ghi chú của màn "Tư vấn và báo giá". MÀN NÀY KHÔNG NẰM TRONG NGOẠI LỆ ẤY,
 *   và cũng không cần: một bộ chọn ba bước dựng bằng nút là hình dạng ĐÚNG cho nó, không phải một
 *   cách lách. Mỗi bước một câu hỏi, mỗi lựa chọn một đích chạm 44px, không bàn phím, không con
 *   trỏ. Đổi chúng thành radio là thêm một điểm thu thập để không được gì.
 *
 * ⚠ KHÔNG MỘT BYTE NÀO RỜI KHỎI MÀN NÀY. Trạng thái sống trong `useState`: không `localStorage`,
 *   không `sessionStorage`, không lời gọi mạng, không `console.log`. Ba câu trả lời ở đây KHÔNG đi
 *   theo người dùng sang màn "Tư vấn và báo giá" — họ chọn lại ở đó, và đó là chủ đích: chuyền
 *   chúng sang nghĩa là gửi lên máy chủ những câu người dùng trả lời cho một màn họ tưởng là chạy
 *   trên máy.
 *
 * ⚠ BA HÀNH ĐỘNG CUỐI, VÀ KHÔNG HÀNH ĐỘNG NÀO HỨA THỨ CHƯA CÓ.
 *
 *   Giai đoạn A có hai: đọc tiếp trên màn Giải pháp, và gọi hotline. Giai đoạn B thêm cái thứ ba —
 *   "Gửi yêu cầu tư vấn" — và nó CHỈ ĐƯỢC PHÉP TỒN TẠI TỪ HÔM NAY, vì hôm nay mới có một tuyến
 *   nhận (`POST /api/v1/requests`). "Đặt lịch" thì vẫn KHÔNG, và mọi cam kết thời gian cũng vậy:
 *   chúng vẫn chưa có đích đến nào.
 */
import { useState } from "react";

import { CONTACT } from "../../content/company-profile";
import { type ThamSoMan } from "../company-intro/dieu-huong";
import { CompassGlyph, HandshakeGlyph, PhoneGlyph, SOLUTION_GLYPHS } from "../company-intro/icons";

import {
  CAU_HOI_NGANH,
  CAU_HOI_QUY_MO,
  CAU_HOI_VIEC,
  giaiPhapGoiY,
  type LuaChon,
  type MaNganh,
  type MaQuyMo,
  type MaViec,
  NGANH,
  nhanCua,
  QUY_MO,
  VIEC,
} from "./anh-xa";

/** Chữ của màn, một chỗ. Nơi nào cần một câu thì đọc từ đây, không gõ vào JSX. */
export const LOI = {
  tieu_de: "Gợi ý giải pháp",
  dan_nhap:
    "Ba câu hỏi ngắn, trả lời ngay trên máy bạn. Không có gì được lưu lại và không có gì được gửi đi.",
  buoc: (thu: number, tong: number) => `Bước ${thu} / ${tong}`,
  nut_quay_lai: "‹ Quay lại bước trước",
  nut_lam_lai: "Làm lại từ đầu",
  tieu_de_ket_qua: "Dòng giải pháp nên hỏi trước",
  co_so_goi_y:
    "Gợi ý dưới đây chọn theo việc bạn đang cần. Lĩnh vực và quy mô nằm trong câu tóm tắt bên dưới, để bạn đọc lại cho người trực hotline nghe.",
  nhan_tom_tat: "Tóm tắt câu trả lời của bạn",
  nhan_linh_vuc: "Lĩnh vực",
  nhan_quy_mo: "Quy mô",
  nhan_viec: "Đang cần",
  nut_xem_chi_tiet: "Xem chi tiết giải pháp",
  phu_xem_chi_tiet: "Mở màn Giải pháp",
  // ⚠ HÀNH ĐỘNG THỨ BA, THÊM 22/09/2026 — VÀ NÓ CHỈ ĐƯỢC PHÉP TỒN TẠI TỪ HÔM NAY.
  //
  //   Giai đoạn A cấm mọi nhãn kiểu "gửi yêu cầu" trên màn này, vì KHÔNG CÓ TUYẾN NÀO NHẬN, và
  //   một nút hứa thứ chưa có là thứ người duyệt của Zalo bấm đầu tiên. Giai đoạn B có tuyến
  //   (`POST /api/v1/requests`), nên lệnh cấm ấy được THU HẸP đúng bằng hai từ có đích đến —
  //   "đặt lịch" và mọi cam kết thời gian vẫn bị cấm, vì chúng vẫn chưa có đích. Xem ca kiểm
  //   "KHÔNG hứa thứ chưa có tuyến nào nhận" trong `goi-y-giai-phap.test.tsx`.
  nut_gui_yeu_cau: "Gửi yêu cầu tư vấn",
  phu_gui_yeu_cau: "Chúng tôi liên hệ lại",
  nut_goi: "Gọi hotline",
  khong_co_goi_y:
    "Chưa có dòng giải pháp nào khớp với lựa chọn này. Bạn hãy gọi hotline, chúng tôi trả lời trực tiếp.",
} as const;

const TONG_SO_BUOC = 3;

type TraLoi = { nganh?: MaNganh; quy_mo?: MaQuyMo; viec?: MaViec };

/**
 * Một bước: câu hỏi và các lựa chọn. THUẦN — không giữ gì, nhận mọi thứ qua tham số.
 *
 * Tách ra để `goi-y-giai-phap.test.tsx` dựng được từng bước bằng `react-dom/server`, nơi không có
 * DOM để bấm. Một bộ chọn chỉ kiểm được khi dựng thẳng được từng trạng thái của nó.
 */
export function BuocChon<T extends string>({
  thu,
  cau_hoi,
  lua_chon,
  onChon,
}: {
  thu: number;
  cau_hoi: string;
  lua_chon: readonly LuaChon<T>[];
  onChon: (ma: T) => void;
}) {
  return (
    <>
      {/* SỐ BƯỚC NÓI BẰNG CHỮ, KHÔNG BẰNG BA CHẤM TRÒN ĐỔI MÀU. Một chuỗi chấm chỉ khác nhau ở màu
          là thông tin truyền bằng riêng màu sắc — thứ người mù màu và người đọc ngoài nắng mất
          trắng (README §Non-negotiables #6). */}
      <p className="screen-lead">{LOI.buoc(thu, TONG_SO_BUOC)}</p>
      <h2 className="section-title">{cau_hoi}</h2>
      <ul className="gygp-ds">
        {lua_chon.map((mot) => (
          <li key={mot.ma}>
            {/* `.action action--mo` — đúng cái nút khối mà màn chủ và màn Quản lý quyền dùng: cao
                tối thiểu `--tap-min`, nền đã đo, viền và bóng đã có. Không dựng một lớp nút mới
                cho màn này: một lớp CSS không quy định gì là chỗ người sau gắn một màu chưa ai đo. */}
            <button
              type="button"
              className="action action--mo gygp-ds__nut"
              onClick={() => onChon(mot.ma)}
            >
              <span className="action__value">{mot.nhan}</span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * Kết quả, THUẦN. Nhận cả ba câu trả lời đã có và hai hàm hành động.
 *
 * `viec` là tham số BẮT BUỘC chứ không tuỳ chọn: không có nó thì không có gì để gợi ý, và một
 * nhánh "chưa chọn việc" ở đây sẽ là một màn hình trống không ai giải thích được. Nơi gọi chỉ dựng
 * khối này khi đã đủ ba câu trả lời — kiểu ép điều đó thay vì một chú thích.
 */
export function KetQuaGoiY({
  tra_loi,
  viec,
  onXemGiaiPhap,
  onGuiYeuCau,
}: {
  tra_loi: TraLoi;
  viec: MaViec;
  onXemGiaiPhap: () => void;
  /**
   * Đường sang màn "Tư vấn và báo giá" (giai đoạn B). TUỲ CHỌN, và khác biệt ấy có lý do: bộ test
   * dựng khối này một mình, và bắt nó khai một hàm nó không bấm tới là mời người sau truyền vào
   * đó một thứ khác. Vỏ màn luôn truyền.
   */
  onGuiYeuCau?: () => void;
}) {
  const goi_y = giaiPhapGoiY(viec);

  return (
    <>
      <h2 className="section-title">{LOI.tieu_de_ket_qua}</h2>
      <p className="screen-lead">{LOI.co_so_goi_y}</p>

      {goi_y.length === 0 ? (
        <p className="tn__loi">{LOI.khong_co_goi_y}</p>
      ) : (
        <ul className="gygp-ds">
          {goi_y.map((giai_phap) => {
            const Glyph = SOLUTION_GLYPHS[giai_phap.id];
            return (
              <li className="card" key={giai_phap.id}>
                <span className="tile tile--soft" aria-hidden="true">
                  <Glyph className="tile__glyph" />
                </span>
                {/* Tên sản phẩm chỉ hiện khi nguồn có nó — `product` là tuỳ chọn trong `SOLUTIONS`,
                    và suy ra một cái tên là gán một sản phẩm cho một pháp nhân bằng phỏng đoán. */}
                {giai_phap.product ? (
                  <h3 className="card__title">{giai_phap.product}</h3>
                ) : null}
                <p className="card__body">{giai_phap.headline}</p>
                {giai_phap.note ? <p className="card__body">{giai_phap.note}</p> : null}
              </li>
            );
          })}
        </ul>
      )}

      {/* CÂU TÓM TẮT — chỗ DUY NHẤT hai câu trả lời đầu được đọc lại, và là lý do chúng được hỏi.
          Người dùng đọc nguyên câu này cho người trực hotline nghe; không có nó thì hai bước đầu
          là hai bước không làm gì. */}
      <section className="card">
        <h3 className="card__title">{LOI.nhan_tom_tat}</h3>
        <ul className="the-tt__danh-sach">
          {(
            [
              [LOI.nhan_linh_vuc, nhanCua(NGANH, tra_loi.nganh)],
              [LOI.nhan_quy_mo, nhanCua(QUY_MO, tra_loi.quy_mo)],
              [LOI.nhan_viec, nhanCua(VIEC, tra_loi.viec)],
            ] as const
          )
            .filter(([, gia_tri]) => gia_tri !== null)
            .map(([nhan, gia_tri]) => (
              <li className="the-tt__dong" key={nhan}>
                <span className="the-tt__nhan">{nhan}</span>
                <span className="the-tt__gia-tri">{gia_tri}</span>
              </li>
            ))}
        </ul>
      </section>

      <button type="button" className="action action--mo" onClick={onXemGiaiPhap}>
        <span className="tile tile--soft" aria-hidden="true">
          <CompassGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">{LOI.phu_xem_chi_tiet}</span>
          <span className="action__value">{LOI.nut_xem_chi_tiet}</span>
        </span>
      </button>

      {onGuiYeuCau !== undefined && (
        <button type="button" className="action action--mo" onClick={onGuiYeuCau}>
          <span className="tile tile--soft" aria-hidden="true">
            <HandshakeGlyph className="tile__glyph" />
          </span>
          <span>
            <span className="action__label">{LOI.phu_gui_yeu_cau}</span>
            <span className="action__value">{LOI.nut_gui_yeu_cau}</span>
          </span>
        </button>
      )}

      {/* `tel:` — không phải một lời gọi nền tảng. `openPhone` đang được khai cho việc gọi một số
          vừa QUÉT được ở màn Danh thiếp; mượn nó ở đây là làm sai lệch chính bảng khai mà màn Quản
          lý quyền đọc ra. Một neo `tel:` chạy được cả ngoài Zalo và không cần khai thêm quyền nào. */}
      <a className="action action--primary" href={`tel:${CONTACT.hotlineDialable}`}>
        <span className="tile" aria-hidden="true">
          <PhoneGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">{LOI.nut_goi}</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </a>
    </>
  );
}

export function GoiYGiaiPhapScreen({ onDi }: ThamSoMan) {
  /**
   * Vỏ app luôn truyền `onDi`. Giá trị lui này chỉ phục vụ những chỗ dựng màn một mình — bộ test
   * dựng bằng `renderToStaticMarkup` — và không bao giờ chạy trong app thật.
   */
  const di = onDi ?? (() => {});

  const [tra_loi, datTraLoi] = useState<TraLoi>({});

  /**
   * BƯỚC HIỆN TẠI SUY RA TỪ CÂU TRẢ LỜI, KHÔNG PHẢI MỘT BIẾN ĐẾM RIÊNG.
   *
   * Hai nguồn cho một sự thật thì một trong hai sẽ lệch, và lần lệch ấy hiện ra thành "bấm quay
   * lại một lần, màn hình vẫn ở bước cũ nhưng câu trả lời đã mất". Ở đây không có gì để lệch.
   */
  function quayLai() {
    datTraLoi((truoc) => {
      if (truoc.viec !== undefined) return { nganh: truoc.nganh, quy_mo: truoc.quy_mo };
      if (truoc.quy_mo !== undefined) return { nganh: truoc.nganh };
      return {};
    });
  }

  const da_bat_dau = tra_loi.nganh !== undefined;

  return (
    <>
      <section className="banner">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <CompassGlyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">{LOI.tieu_de}</h1>
        </div>
      </section>

      <p className="screen-lead">{LOI.dan_nhap}</p>

      {tra_loi.nganh === undefined ? (
        <BuocChon
          thu={1}
          cau_hoi={CAU_HOI_NGANH}
          lua_chon={NGANH}
          onChon={(ma) => datTraLoi((truoc) => ({ ...truoc, nganh: ma }))}
        />
      ) : tra_loi.quy_mo === undefined ? (
        <BuocChon
          thu={2}
          cau_hoi={CAU_HOI_QUY_MO}
          lua_chon={QUY_MO}
          onChon={(ma) => datTraLoi((truoc) => ({ ...truoc, quy_mo: ma }))}
        />
      ) : tra_loi.viec === undefined ? (
        <BuocChon
          thu={3}
          cau_hoi={CAU_HOI_VIEC}
          lua_chon={VIEC}
          onChon={(ma) => datTraLoi((truoc) => ({ ...truoc, viec: ma }))}
        />
      ) : (
        <KetQuaGoiY
          tra_loi={tra_loi}
          viec={tra_loi.viec}
          onXemGiaiPhap={() => di({ man: "solutions" })}
          onGuiYeuCau={() => di({ man: "tu-van" })}
        />
      )}

      {/* HAI ĐƯỜNG LUI, VÀ CHÚNG CHỈ HIỆN KHI CÓ THỨ ĐỂ LUI VỀ. Ở bước 1 thì "quay lại bước trước"
          và "làm lại từ đầu" đều là hai nút không làm gì — và một nút bấm không phản hồi là cách
          nhanh nhất để người lớn tuổi kết luận rằng app hỏng. */}
      {da_bat_dau && (
        <div className="tn__doi-nut">
          <button type="button" className="tn-hanh-dong tn-hanh-dong--rong" onClick={quayLai}>
            {LOI.nut_quay_lai}
          </button>
          <button
            type="button"
            className="tn-hanh-dong tn-hanh-dong--rong"
            onClick={() => datTraLoi({})}
          >
            {LOI.nut_lam_lai}
          </button>
        </div>
      )}
    </>
  );
}
