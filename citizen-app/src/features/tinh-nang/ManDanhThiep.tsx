/**
 * TÍNH NĂNG QUÉT DANH THIẾP SỐ — `scanQRCode`.
 *
 * Đây là tính năng CHẠY TRỌN VẸN của bản dựng này, và nó chạy trọn vẹn được vì `scanQRCode` là
 * API duy nhất của nền tảng trả về **dữ liệu thật** chứ không phải một token phải đổi ở máy chủ
 * (`zalo-api.ts`). Quét xong là có nội dung, bóc tách xong là có một tấm thiếp đọc được và một
 * nút gọi bấm được — không cần một dòng máy chủ nào.
 *
 * ⚠ NỘI DUNG MỘT TẤM THIẾP LÀ DỮ LIỆU CÁ NHÂN CỦA NGƯỜI KHÁC (luật 3, Nghị định 13/2023/NĐ-CP).
 * Hiện lên màn hình thì được — đó là toàn bộ việc người dùng vừa yêu cầu. Ngoài ra thì không:
 * không `console.log`, không chỗ lưu nào, không tên tệp, không khoá bộ nhớ đệm. Nó sống trong
 * `useState` và mất đi khi quét mã khác hoặc rời màn hình.
 *
 * VỀ THẺ `mailto:` — CÓ CÂN NHẮC, KHÔNG PHẢI QUÊN. Luật 3 cấm dữ liệu cá nhân nằm trong URL, và
 * điều nó nhắm tới là những URL ĐƯỢC GHI LẠI ở đâu đó: đường gọi mạng, tên tệp, khoá bộ nhớ đệm.
 * `mailto:` không gọi mạng và không được ghi lại: nó là một ý định cục bộ, giao cho ứng dụng thư
 * của chính người dùng, đúng bằng việc họ vừa bấm. Số điện thoại thì đi qua `openPhone` của nền
 * tảng chứ không qua một `tel:` nào.
 */
import { useState } from "react";

import { NamecardGlyph } from "../company-intro/icons";

import { type DanhThiep, docMaQR, thiepCoNoiDung } from "./danh-thiep";
import { KhungTinhNang, type TrangThai } from "./khung";
import { DANH_THIEP, LOI_MO_NGOAI } from "./noi-dung";
import { moCuocGoi, moTrangWeb, quetMaQR } from "./zalo-api";

/** Một dòng thông tin của tấm thiếp: nhãn, giá trị, và (có thể) một nút hành động. */
function DongThiep({
  nhan,
  gia_tri,
  hanh_dong,
}: {
  nhan: string;
  gia_tri: string;
  hanh_dong?: React.ReactNode;
}) {
  return (
    <li className="the-tt__dong">
      <span className="the-tt__nhan">{nhan}</span>
      <span className="the-tt__gia-tri">{gia_tri}</span>
      {hanh_dong}
    </li>
  );
}

/**
 * Tấm thiếp đã bóc tách, THUẦN — nhận cả hai hàm mở ngoài qua tham số.
 *
 * Thuần để `tinh-nang.test.tsx` dựng được nó với mọi hình dạng thiếp (đủ trường, thiếu trường,
 * nhiều số điện thoại) mà không cần một chiếc điện thoại nào.
 */
export function TheDanhThiep({
  thiep,
  onGoi,
  onMoLienKet,
}: {
  thiep: DanhThiep;
  onGoi: (so: string) => void;
  onMoLienKet: (duong_dan: string) => void;
}) {
  if (!thiepCoNoiDung(thiep)) return <p className="tn__loi">{DANH_THIEP.thiep_rong}</p>;

  return (
    <article className="the-tt hien-len">
      <h3 className="the-tt__tieu-de">{DANH_THIEP.tieu_de_ket_qua}</h3>
      <ul className="the-tt__danh-sach">
        {thiep.ho_ten !== "" && <DongThiep nhan={DANH_THIEP.nhan_ho_ten} gia_tri={thiep.ho_ten} />}
        {thiep.chuc_danh !== "" && (
          <DongThiep nhan={DANH_THIEP.nhan_chuc_danh} gia_tri={thiep.chuc_danh} />
        )}
        {thiep.to_chuc !== "" && <DongThiep nhan={DANH_THIEP.nhan_to_chuc} gia_tri={thiep.to_chuc} />}

        {thiep.dien_thoai.map((so) => (
          <DongThiep
            key={`tel-${so}`}
            nhan={DANH_THIEP.nhan_dien_thoai}
            gia_tri={so}
            hanh_dong={
              <button type="button" className="tn-hanh-dong" onClick={() => onGoi(so)}>
                {DANH_THIEP.nut_goi}
              </button>
            }
          />
        ))}

        {thiep.email.map((dia_chi) => (
          <DongThiep
            key={`mail-${dia_chi}`}
            nhan={DANH_THIEP.nhan_email}
            gia_tri={dia_chi}
            hanh_dong={
              <a className="tn-hanh-dong" href={`mailto:${dia_chi}`}>
                {DANH_THIEP.nut_email}
              </a>
            }
          />
        ))}

        {thiep.trang_web.map((duong_dan) => (
          <DongThiep
            key={`url-${duong_dan}`}
            nhan={DANH_THIEP.nhan_trang_web}
            gia_tri={duong_dan}
            hanh_dong={
              <button
                type="button"
                className="tn-hanh-dong"
                onClick={() => onMoLienKet(duong_dan)}
              >
                {DANH_THIEP.nut_mo_lien_ket}
              </button>
            }
          />
        ))}
      </ul>
    </article>
  );
}

/**
 * Kết quả một lần quét, THUẦN: ba nhánh của `docMaQR` cộng nhánh mã rỗng.
 *
 * Mã không phải danh thiếp vẫn được hiện NGUYÊN VĂN. Người vừa quét cần biết mã ấy ghi gì —
 * giấu nó đi vì "không đúng định dạng" là biến một tính năng thành một cánh cửa đóng.
 */
export function KetQuaQuet({
  noi_dung_qr,
  onGoi,
  onMoLienKet,
}: {
  noi_dung_qr: string;
  onGoi: (so: string) => void;
  onMoLienKet: (duong_dan: string) => void;
}) {
  const doc = docMaQR(noi_dung_qr);

  if (doc.loai === "rong") return <p className="tn__loi">{DANH_THIEP.ma_rong}</p>;

  if (doc.loai === "danh-thiep")
    return (
      <>
        <TheDanhThiep thiep={doc} onGoi={onGoi} onMoLienKet={onMoLienKet} />
        <p className="tn__rieng-tu">{DANH_THIEP.rieng_tu}</p>
      </>
    );

  if (doc.loai === "lien-ket")
    return (
      <article className="the-tt hien-len">
        <h3 className="the-tt__tieu-de">{DANH_THIEP.tieu_de_lien_ket}</h3>
        <p className="the-tt__tho">{doc.duong_dan}</p>
        <button
          type="button"
          className="tn-hanh-dong"
          onClick={() => onMoLienKet(doc.duong_dan)}
        >
          {DANH_THIEP.nut_mo_lien_ket}
        </button>
      </article>
    );

  return (
    <article className="the-tt hien-len">
      <h3 className="the-tt__tieu-de">{DANH_THIEP.tieu_de_van_ban}</h3>
      <p className="the-tt__nhan-tho">{DANH_THIEP.nguyen_van}</p>
      <p className="the-tt__tho">{doc.noi_dung}</p>
    </article>
  );
}

/** Màn quét danh thiếp — một tab của ứng dụng. */
export function ManDanhThiep() {
  const [trang_thai, datTrangThai] = useState<TrangThai>({ kieu: "chua-goi" });
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);

  async function quet() {
    datKhongMoDuoc(false);
    datTrangThai({ kieu: "dang-cho" });
    datTrangThai(await quetMaQR());
  }

  /** Mở ngoài (gọi điện, mở trang) chỉ có hai kết cục đáng nói: được, hoặc chưa được. */
  async function moNgoai(chay: () => Promise<{ kieu: string }>) {
    const ket_qua = await chay();
    datKhongMoDuoc(ket_qua.kieu !== "xong");
  }

  return (
    <KhungTinhNang
      ma="danh-thiep"
      trang_thai={trang_thai}
      onBam={() => void quet()}
      glyph={<NamecardGlyph className="tn__glyph" />}
      dan_nhap={DANH_THIEP.dan_nhap}
      veKetQua={(noi_dung_qr) => (
        <>
          <KetQuaQuet
            noi_dung_qr={noi_dung_qr}
            onGoi={(so) => void moNgoai(() => moCuocGoi(so))}
            onMoLienKet={(duong_dan) => void moNgoai(() => moTrangWeb(duong_dan))}
          />
          {khong_mo_duoc && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
          <button type="button" className="tn__nut-phu" onClick={() => void quet()}>
            {DANH_THIEP.nut_quet_lai}
          </button>
        </>
      )}
    />
  );
}
