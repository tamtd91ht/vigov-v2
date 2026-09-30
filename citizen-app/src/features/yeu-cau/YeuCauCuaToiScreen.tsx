/**
 * MÀN "YÊU CẦU CỦA TÔI" — chỉ yêu cầu của chính người đang đăng nhập.
 *
 * ⚠ ĐỊNH DANH ĐẾN TỪ PHIÊN, KHÔNG TỪ MỘT THAM SỐ (luật 4, bất biến 2). Tuyến GET không nhận một
 * tham số nào nói "của ai", và việc KHÔNG CÓ chúng chính là tính năng — xem `api/goi-may-chu.ts`.
 *
 * ⚠ CHƯA ĐĂNG NHẬP THÌ KHÔNG ĐỌC, VÀ CŨNG KHÔNG HIỆN MỘT DANH SÁCH RỖNG. Một danh sách rỗng khi
 * chưa đăng nhập đọc ra thành "tôi chưa gửi yêu cầu nào" — một câu SAI, và người đọc nó sẽ gửi
 * lại lần nữa. `MoiDangNhap` thay chỗ.
 *
 * ⚠ DANH SÁCH RỖNG CÓ MỘT CÂU, KHÔNG PHẢI MỘT MÀN TRẮNG. Một màn trắng và một màn hỏng trông
 * giống hệt nhau, và người lớn tuổi đọc cả hai thành "app hỏng".
 */
import { useEffect, useState } from "react";

import { docYeuCauCuaToi, type KetQuaDoc } from "../../api/goi-may-chu";
import type { YeuCauDaGui } from "../../api/hop-dong-yeu-cau";
import { MOC_DANG_NHAP, type ThamSoMan } from "../company-intro/dieu-huong";
import { GridGlyph } from "../company-intro/icons";
import { bearerCua, dungPhien } from "../dang-nhap/kho-phien";

import { MoiDangNhap } from "./MoiDangNhap";
import { PHIEN_KHONG_LUU, YEU_CAU_CUA_TOI } from "./noi-dung";
import { GIAI_THICH_TRANG_THAI, ngayDoc, NHAN_LOAI, NHAN_TRANG_THAI } from "./trang-thai";

/** Một dòng của danh sách, THUẦN. */
export function DongYeuCau({ yeu_cau }: { yeu_cau: YeuCauDaGui }) {
  const luc = ngayDoc(yeu_cau.tao_luc);

  return (
    <li className="card">
      <h3 className="card__title">{NHAN_LOAI[yeu_cau.loai]}</h3>
      {/* TÌNH TRẠNG NÓI BẰNG CHỮ, KHÔNG BẰNG RIÊNG MÀU CỦA MỘT CHẤM TRÒN (README §Non-negotiables
          #6). Và có một câu giải thích dưới nhãn: "Đã khép lại" đứng một mình đọc ra như bị từ
          chối, với đúng người ít kiên nhẫn nhất để hỏi lại. */}
      <ul className="the-tt__danh-sach">
        <li className="the-tt__dong">
          <span className="the-tt__nhan">Tình trạng</span>
          <span className="the-tt__gia-tri">{NHAN_TRANG_THAI[yeu_cau.trang_thai]}</span>
        </li>
        <li className="the-tt__dong">
          <span className="the-tt__nhan">{YEU_CAU_CUA_TOI.nhan_ma}</span>
          <span className="the-tt__gia-tri">{yeu_cau.ma}</span>
        </li>
        {luc !== "" && (
          <li className="the-tt__dong">
            <span className="the-tt__nhan">{YEU_CAU_CUA_TOI.nhan_gui_luc}</span>
            <span className="the-tt__gia-tri">{luc}</span>
          </li>
        )}
      </ul>
      <p className="card__body">{GIAI_THICH_TRANG_THAI[yeu_cau.trang_thai]}</p>
    </li>
  );
}

/** Thân màn theo từng nhánh kết quả, THUẦN — dựng được mọi nhánh mà không cần một máy chủ nào. */
export function ThanDanhSach({
  ket_qua,
  onDangNhapLai,
  onGuiMoi,
}: {
  ket_qua: KetQuaDoc | null;
  onDangNhapLai: () => void;
  onGuiMoi: () => void;
}) {
  if (ket_qua === null) return <p className="tn__ranh-gioi">{YEU_CAU_CUA_TOI.dang_doc}</p>;

  if (ket_qua.kieu === "chua-dang-nhap") {
    return (
      <>
        <p className="tn__loi">{YEU_CAU_CUA_TOI.phien_het_han}</p>
        <button type="button" className="tn__nut-phu" onClick={onDangNhapLai}>
          {YEU_CAU_CUA_TOI.nut_toi_dang_nhap}
        </button>
      </>
    );
  }

  if (ket_qua.kieu === "chua-khai-host") {
    return <p className="tn__loi">{YEU_CAU_CUA_TOI.chua_khai_host}</p>;
  }

  if (ket_qua.kieu === "khong-goi-duoc") {
    return <p className="tn__loi">{YEU_CAU_CUA_TOI.khong_doc_duoc}</p>;
  }

  if (ket_qua.danh_sach.length === 0) {
    return (
      <>
        <p className="tn__ranh-gioi">{YEU_CAU_CUA_TOI.rong}</p>
        <button type="button" className="tn__nut" onClick={onGuiMoi}>
          {YEU_CAU_CUA_TOI.nut_gui_moi}
        </button>
      </>
    );
  }

  return (
    <ul className="gygp-ds">
      {ket_qua.danh_sach.map((mot) => (
        <DongYeuCau key={mot.ma} yeu_cau={mot} />
      ))}
    </ul>
  );
}

export function YeuCauCuaToiScreen({ onDi }: ThamSoMan) {
  const di = onDi ?? (() => {});
  const { phien } = dungPhien();
  const bearer = bearerCua(phien);

  const [ket_qua, datKetQua] = useState<KetQuaDoc | null>(null);
  /** Đếm số lần bấm "đọc lại" — không có nó thì bấm hai lần liên tiếp chỉ đọc một lần. */
  const [lan_doc, datLanDoc] = useState(0);

  useEffect(() => {
    if (bearer === "") return;
    let con_tren_man = true;
    datKetQua(null);
    void docYeuCauCuaToi(bearer).then((ra) => {
      if (con_tren_man) datKetQua(ra);
    });
    return () => {
      con_tren_man = false;
    };
  }, [bearer, lan_doc]);

  return (
    <>
      <section className="banner">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <GridGlyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">{YEU_CAU_CUA_TOI.tieu_de}</h1>
        </div>
      </section>

      {phien === null ? (
        <MoiDangNhap
          tieu_de={YEU_CAU_CUA_TOI.can_dang_nhap_tieu_de}
          li_do={YEU_CAU_CUA_TOI.can_dang_nhap}
          nhan_nut={YEU_CAU_CUA_TOI.nut_toi_dang_nhap}
          onDangNhap={() => di({ man: "contact", moc: MOC_DANG_NHAP })}
        />
      ) : (
        <>
          <p className="screen-lead">{YEU_CAU_CUA_TOI.dan_nhap}</p>
          <div className="tn__ket-qua" role="status">
            <ThanDanhSach
              ket_qua={ket_qua}
              onDangNhapLai={() => di({ man: "contact", moc: MOC_DANG_NHAP })}
              onGuiMoi={() => di({ man: "tu-van" })}
            />
          </div>
          <button type="button" className="tn__nut-phu" onClick={() => datLanDoc((n) => n + 1)}>
            {YEU_CAU_CUA_TOI.nut_doc_lai}
          </button>
        </>
      )}

      <p className="tn__rieng-tu">{PHIEN_KHONG_LUU}</p>
    </>
  );
}
