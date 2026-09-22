/**
 * MÀN "TƯ VẤN & BÁO GIÁ" — bề mặt DUY NHẤT của ứng dụng gửi dữ liệu bán hàng đi.
 *
 * ⚠ CHƯA ĐĂNG NHẬP THÌ KHÔNG VẼ BIỂU MẪU. Xem `MoiDangNhap`.
 *
 * ⚠ MÃ GỬI ĐI LẤY TỪ `SOLUTIONS` VÀ TỪ DANH SÁCH QUY MÔ ĐÃ CÓ — KHÔNG DỰNG DANH SÁCH THỨ HAI.
 *
 *   Danh mục sản phẩm sống ở `content/company-profile.ts`; bốn mức quy mô sống ở
 *   `features/goi-y-giai-phap/anh-xa.ts`, nơi màn "Gợi ý" đã hỏi đúng câu hỏi ấy. Chép chúng sang
 *   đây là dựng hai danh sách cho một sự thật, và ngày ai đó đổi danh mục thì màn này gửi lên một
 *   mã máy chủ trả 400 — hoặc tệ hơn, một mã cũ vẫn hợp lệ nhưng chỉ vào một sản phẩm đã đổi tên.
 *
 * ⚠ MỖI LỰA CHỌN LÀ MỘT `<button>`, KHÔNG PHẢI `<input type="checkbox">`. Ô nhập duy nhất của cả
 * ứng dụng là ô ghi chú, và nó sống ở một tệp riêng (`OGhiChu.tsx`) để lệnh cấm còn hẹp được.
 *
 * ⚠ HAI NÚT GỬI, HAI `kind` KHÁC NHAU, VÀ CHÚNG KHÔNG PHẢI HAI CÁCH NÓI MỘT VIỆC:
 *
 *   "Gửi yêu cầu tư vấn" (`consult`) — chúng tôi chuẩn bị nội dung rồi trả lời.
 *   "Đề nghị gọi lại"    (`callback`) — có người gọi vào số Zalo của bạn, TRẦN 3 LƯỢT/24 GIỜ, và
 *                        cái trần ấy được nói ra NGAY TRÊN NÚT chứ không để người dùng gặp nó lần
 *                        đầu dưới dạng một câu từ chối.
 *
 * ⚠ KHÔNG MỘT CAM KẾT THỜI GIAN NÀO TRÊN MÀN NÀY — xem khối đầu `noi-dung.ts`.
 */
import { useState } from "react";

import {
  conLaiGhiChu,
  type LoaiYeuCau,
  SO_QUAN_TAM_TOI_DA,
  type YeuCauMoi,
} from "../../api/hop-dong-yeu-cau";
import { guiYeuCau, type KetQuaGui } from "../../api/goi-may-chu";
import { KHOA_CHIEN_DICH, maChienDichHopLe } from "../../content/chien-dich";
import { SOLUTIONS } from "../../content/company-profile";
import { thamSo, thamSoMoApp } from "../../lib/launch-params";
import { MOC_DANG_NHAP, type ThamSoMan } from "../company-intro/dieu-huong";
import { HandshakeGlyph } from "../company-intro/icons";
import { bearerCua, dungPhien } from "../dang-nhap/kho-phien";
import { QUY_MO } from "../goi-y-giai-phap/anh-xa";

import { MoiDangNhap } from "./MoiDangNhap";
import { PHIEN_KHONG_LUU, TU_VAN } from "./noi-dung";
import { OGhiChu } from "./OGhiChu";

const ID_O_GHI_CHU = "yc-ghi-chu";

/** Một hàng nút chọn, THUẦN. Dùng cho cả "quan tâm" (nhiều) lẫn "quy mô" (một). */
export function HangChon({
  cau_hoi,
  huong_dan,
  lua_chon,
  dang_chon,
  onBam,
}: {
  cau_hoi: string;
  huong_dan?: string;
  lua_chon: readonly { ma: string; nhan: string }[];
  dang_chon: readonly string[];
  onBam: (ma: string) => void;
}) {
  return (
    <>
      <h2 className="section-title">{cau_hoi}</h2>
      {huong_dan !== undefined && <p className="screen-lead">{huong_dan}</p>}
      <ul className="gygp-ds">
        {lua_chon.map((mot) => {
          const chon = dang_chon.includes(mot.ma);
          return (
            <li key={mot.ma}>
              {/* TRẠNG THÁI ĐƯỢC CHỌN NÓI BẰNG CHỮ VÀ BẰNG `aria-pressed`, KHÔNG BẰNG RIÊNG MÀU
                  NỀN (README §Non-negotiables #6). Dấu "✓" là một ký tự chữ, đọc được ngoài nắng
                  và đọc được bởi một người mù màu. */}
              <button
                type="button"
                className={chon ? "action action--mo gygp-ds__nut la-chon" : "action action--mo gygp-ds__nut"}
                aria-pressed={chon}
                onClick={() => onBam(mot.ma)}
              >
                <span className="la-chon__dau" aria-hidden="true">
                  {chon ? "✓" : "＋"}
                </span>
                <span className="action__value">{mot.nhan}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </>
  );
}

/** Màn kết quả sau khi gửi xong, THUẦN — nhận mã yêu cầu qua tham số. */
export function DaGuiXong({
  ma_yeu_cau,
  onXemDanhSach,
  onGuiTiep,
}: {
  ma_yeu_cau: string;
  onXemDanhSach: () => void;
  onGuiTiep: () => void;
}) {
  return (
    <section className="card" role="status">
      <h2 className="card__title">{TU_VAN.xong_tieu_de}</h2>
      {/* MÃ YÊU CẦU HIỆN RA, VÌ ĐÓ LÀ THỨ DUY NHẤT NGƯỜI DÙNG CẦM ĐƯỢC SAU KHI GỬI. Không có nó
          thì "đã gửi" là một lời nói suông, và người gọi hotline sau đó không có gì để đọc cho
          người trực nghe. */}
      <p className="card__body">
        <span className="the-tt__nhan">{TU_VAN.nhan_ma}</span>{" "}
        <strong className="the-tt__gia-tri">{ma_yeu_cau}</strong>
      </p>
      {/* ⚠ CÂU NÀY ĐÚNG Ở CẢ HAI CA — có tin ZNS và không có. ZNS có trần và có thể tắt. */}
      <p className="card__body">{TU_VAN.xong_xem_lai}</p>
      <button type="button" className="tn__nut" onClick={onXemDanhSach}>
        {TU_VAN.nut_xem_yeu_cau}
      </button>
      <button type="button" className="tn__nut-phu" onClick={onGuiTiep}>
        {TU_VAN.nut_gui_tiep}
      </button>
    </section>
  );
}

export function TuVanBaoGiaScreen({ onDi }: ThamSoMan) {
  const di = onDi ?? (() => {});
  const { phien } = dungPhien();

  const [quan_tam, datQuanTam] = useState<readonly string[]>([]);
  const [quy_mo, datQuyMo] = useState("");
  const [ghi_chu, datGhiChu] = useState("");
  const [dang_gui, datDangGui] = useState(false);
  const [ket_qua, datKetQua] = useState<KetQuaGui | null>(null);

  /**
   * `source` CHỈ ĐƯỢC GỬI KHI MÃ KHỚP MỘT DÒNG CÓ THẬT trong `content/chien-dich.ts`.
   *
   * Tham số mở app là dữ liệu client tự đặt: gửi thẳng nó lên là để người ngoài ghi chữ vào CSDL
   * của máy chủ — và một chuỗi lệch khuôn làm máy chủ trả 400, tức làm NÚT GỬI của người dùng
   * hỏng vì một thứ không liên quan gì tới họ. Xem `maChienDichHopLe`.
   */
  const nguon = maChienDichHopLe(thamSo(thamSoMoApp())[KHOA_CHIEN_DICH] ?? "");

  if (phien === null) {
    return (
      <>
        <BangTieuDe />
        <MoiDangNhap
          tieu_de={TU_VAN.can_dang_nhap_tieu_de}
          li_do={TU_VAN.can_dang_nhap}
          nhan_nut={TU_VAN.nut_toi_dang_nhap}
          onDangNhap={() => di({ man: "contact", moc: MOC_DANG_NHAP })}
        />
      </>
    );
  }

  if (ket_qua?.kieu === "xong") {
    return (
      <>
        <BangTieuDe />
        <DaGuiXong
          ma_yeu_cau={ket_qua.ma_yeu_cau}
          onXemDanhSach={() => di({ man: "yeu-cau" })}
          onGuiTiep={() => {
            // ĐẶT LẠI TRỌN VẸN, kể cả ô ghi chú: người bấm "gửi một yêu cầu khác" đang bắt đầu
            // một việc mới, và để lại chữ cũ trong ô là mời họ gửi nhầm hai lần cùng một câu.
            datKetQua(null);
            datQuanTam([]);
            datQuyMo("");
            datGhiChu("");
          }}
        />
      </>
    );
  }

  const con_lai = conLaiGhiChu(ghi_chu);
  const qua_dai = con_lai < 0;
  const qua_nhieu = quan_tam.length > SO_QUAN_TAM_TOI_DA;
  const chua_chon = quan_tam.length === 0;
  const chan_gui = dang_gui || qua_dai || qua_nhieu || chua_chon;

  async function gui(loai: LoaiYeuCau) {
    datDangGui(true);
    const yc: YeuCauMoi = { loai, quan_tam, quy_mo, ghi_chu, nguon };
    // `bearerCua` chứ không `phien.token`: một chỗ quyết định "phiếu nào gửi được", xem
    // `kho-phien.tsx`. `phien` không thể là `null` ở đây — nhánh trên đã trả về — nhưng hàm vẫn
    // nhận `null` để không có một `!` nào trong mã sản phẩm.
    datKetQua(await guiYeuCau(bearerCua(phien), yc));
    datDangGui(false);
  }

  return (
    <>
      <BangTieuDe />
      <p className="screen-lead">{TU_VAN.dan_nhap}</p>

      <HangChon
        cau_hoi={TU_VAN.cau_hoi_quan_tam}
        huong_dan={TU_VAN.huong_dan_quan_tam}
        // ĐỌC TỪ `SOLUTIONS`: `id` là mã gửi đi, `product ?? headline` là chữ người dùng đọc.
        // Hai thứ ấy KHÔNG được đổi chỗ cho nhau — `headline` là một câu đã công bố, không phải
        // một mã, và `id` không bao giờ được hiện ra màn hình.
        lua_chon={SOLUTIONS.map((gp) => ({ ma: gp.id, nhan: gp.product ?? gp.headline }))}
        dang_chon={quan_tam}
        onBam={(ma) =>
          datQuanTam((truoc) =>
            truoc.includes(ma) ? truoc.filter((mot) => mot !== ma) : [...truoc, ma],
          )
        }
      />

      <HangChon
        cau_hoi={TU_VAN.cau_hoi_quy_mo}
        lua_chon={QUY_MO}
        dang_chon={quy_mo === "" ? [] : [quy_mo]}
        // MỘT LỰA CHỌN: bấm lại chính mục đang chọn là bỏ chọn. Không có nút "xoá lựa chọn"
        // riêng — một nút chỉ để huỷ một lựa chọn là một nút nữa phải đọc.
        onBam={(ma) => datQuyMo((truoc) => (truoc === ma ? "" : ma))}
      />

      <OGhiChu id={ID_O_GHI_CHU} gia_tri={ghi_chu} onDoi={datGhiChu} />

      {/* BA CÂU CHẶN, MỖI CÂU NÓI VIỆC PHẢI LÀM TIẾP — không phải một nút xám không giải thích.
          Một nút bị vô hiệu mà không nói vì sao là thứ người lớn tuổi bấm ba lần rồi bỏ đi. */}
      {chua_chon && <p className="tn__loi">{TU_VAN.chua_chon_gi}</p>}
      {qua_nhieu && <p className="tn__loi">{TU_VAN.qua_nhieu(SO_QUAN_TAM_TOI_DA)}</p>}

      <button
        type="button"
        className="tn__nut"
        disabled={chan_gui}
        aria-busy={dang_gui}
        onClick={() => void gui("consult")}
      >
        {dang_gui ? TU_VAN.dang_gui : TU_VAN.nut_gui}
      </button>

      {/* TRẦN NÓI TRƯỚC, NGAY TRÊN NÚT GỌI LẠI. Gặp cái trần lần đầu dưới dạng một câu từ chối là
          cách nhanh nhất làm người dùng nghĩ mình bị chặn nhầm. */}
      <p className="tn__giai-thich">{TU_VAN.tran_goi_lai}</p>
      <button
        type="button"
        className="tn__nut-phu"
        disabled={chan_gui}
        aria-busy={dang_gui}
        onClick={() => void gui("callback")}
      >
        {TU_VAN.nut_goi_lai}
      </button>

      <div className="tn__ket-qua" role="status">
        <CauTraLoi ket_qua={ket_qua} onDangNhapLai={() => di({ man: "contact", moc: MOC_DANG_NHAP })} />
      </div>

      <p className="tn__rieng-tu">{PHIEN_KHONG_LUU}</p>
    </>
  );
}

/** Dải tiêu đề của màn. Tách ra vì cả ba nhánh trên đều vẽ nó, và `<h1>` phải có đúng một. */
function BangTieuDe() {
  return (
    <section className="banner">
      <div className="banner__content">
        <span className="tile tile--lon" aria-hidden="true">
          <HandshakeGlyph className="tile__glyph" />
        </span>
        <h1 className="banner__title">{TU_VAN.tieu_de}</h1>
      </div>
    </section>
  );
}

/**
 * Câu trả lời cho bốn nhánh KHÔNG thành công, THUẦN.
 *
 * ⚠ NHÁNH `tu-choi` HIỆN NGUYÊN VĂN CÂU CỦA MÁY CHỦ. 400 · 429 · 503 đều kèm một câu tiếng Việt
 * đã nói việc cần làm tiếp, và câu ấy mang những con số của máy chủ (trần 3 lượt/24 giờ, lời mời
 * gọi hotline khi tổng đài chưa cấu hình). Dịch lại ở đây là dựng chỗ thứ hai giữ cùng một câu —
 * và ngày máy chủ đổi trần, chỗ thứ hai vẫn nói số cũ. Câu lui chỉ dùng khi máy chủ KHÔNG nói gì.
 */
export function CauTraLoi({
  ket_qua,
  onDangNhapLai,
}: {
  ket_qua: KetQuaGui | null;
  onDangNhapLai: () => void;
}) {
  if (ket_qua === null || ket_qua.kieu === "xong") return null;

  if (ket_qua.kieu === "chua-dang-nhap") {
    return (
      <>
        <p className="tn__loi">{TU_VAN.phien_het_han}</p>
        <button type="button" className="tn__nut-phu" onClick={onDangNhapLai}>
          {TU_VAN.nut_toi_dang_nhap}
        </button>
      </>
    );
  }

  if (ket_qua.kieu === "tu-choi") {
    return <p className="tn__loi">{ket_qua.cau === "" ? TU_VAN.cau_lui_tu_choi : ket_qua.cau}</p>;
  }

  const CAU: Record<"chua-khai-host" | "khong-goi-duoc", string> = {
    "chua-khai-host": TU_VAN.chua_khai_host,
    "khong-goi-duoc": TU_VAN.khong_goi_duoc,
  };
  return <p className="tn__loi">{CAU[ket_qua.kieu]}</p>;
}
