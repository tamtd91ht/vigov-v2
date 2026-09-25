/**
 * MÀN "PHẢN ÁNH CỦA TÔI" — danh sách phiếu của CHÍNH người dân, mới nhất trước.
 *
 * ⚠ CHỈ PHIẾU CỦA MÌNH, VÀ MÁY CHỦ QUYẾT ĐIỀU ĐÓ: lời gọi không mang một tham số nào nói "của ai"
 * hay "xã nào" (`api/goi-vigov.ts` `phanAnhCuaToi`). Công dân và xã lấy từ phiên (luật 4, bất biến 2).
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG GỌI MẠNG — hôm nay là luôn luôn (`api/phien-vigov.ts`). Màn nói
 * "kênh chưa mở" như hai màn kia.
 *
 * ⚠ NÚT "XEM THÊM", KHÔNG CUỘN VÔ HẠN (`skills/accessibility-elderly`): danh sách tự dài ra khi
 * ngón tay chỉ định kéo xuống là danh sách người lớn tuổi không tìm lại được chỗ mình đang đọc.
 *
 * ⚠ KHÔNG LƯU XUỐNG MÁY. Danh sách sống trong `useState` và mất khi đóng màn (`ranh-gioi-hai-nua`
 * §3b). Không ghi mã tra cứu vào nhật ký.
 *
 * ⚠ KHÔNG CÓ "QUÁ HẠN N NGÀY": quá hạn đếm bằng giờ làm việc (ADR 0007) và chỉ `identity` đếm được.
 * Thẻ chỉ hiện mốc hạn cố định máy chủ đã ghi.
 */
import { useEffect, useRef, useState } from "react";

import { type KetQuaDanhSach, phanAnhCuaToi } from "../api/goi-vigov";
import type { PhieuCuaToiTomTat } from "../api/hop-dong-phan-anh";
import { layPhienViGov } from "../api/phien-vigov";

import { BangXa, KenhChuaMo, nhanLinhVuc } from "./khung";
import { CUA_TOI, GUI, nhanTrangThai, QUAY_LAI, THE_PHIEU } from "./noi-dung";
import { THOI_DIEM_KHONG_DOC_DUOC, thoiDiemVN } from "./thoi-diem";

/** Ba câu lỗi người dân đọc được. Mọi mã lạ khác rơi vào `loi-may-chu`. */
export type LoiDanhSach = "het-phien" | "loi-mang" | "loi-may-chu";

/** Trạng thái màn, THUẦN — test dựng thẳng từng bước mà không cần DOM. */
export type DanhSach = {
  readonly muc: readonly PhieuCuaToiTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
  /** Đã nhận trang đầu. Trước đó, "không có dòng nào" chưa có nghĩa là "chưa gửi phản ánh nào". */
  readonly da_co_trang_dau: boolean;
  readonly dang_tai: boolean;
  readonly loi: LoiDanhSach | null;
  /** Máy chủ nói chưa có phiên / chưa cấu hình — màn thành "kênh chưa mở". */
  readonly kenh_dong: boolean;
};

export const DANH_SACH_DAU: DanhSach = {
  muc: [],
  con_tro: "",
  con_nua: false,
  da_co_trang_dau: false,
  dang_tai: true,
  loi: null,
  kenh_dong: false,
};

export function batDauTai(ds: DanhSach): DanhSach {
  return { ...ds, dang_tai: true, loi: null };
}

/**
 * Kết quả một lần tải → trạng thái kế. Trang mới NỐI VÀO SAU, không thay: "Xem thêm" là thêm.
 *
 * BỎ DÒNG TRÙNG MÃ: con trỏ không trả trùng, nhưng hai lần tải chồng nhau (bấm nhanh, dựng lại
 * màn) thì có thể — và hai thẻ cùng mã là người dân tưởng mình gửi hai lần.
 */
export function sauKhiTai(ds: DanhSach, kq: KetQuaDanhSach): DanhSach {
  switch (kq.kieu) {
    case "xong": {
      const da_co = new Set(ds.muc.map((p) => p.ma_tra_cuu));
      const moi = kq.trang.muc.filter((p) => !da_co.has(p.ma_tra_cuu));
      return {
        ...ds,
        muc: [...ds.muc, ...moi],
        con_tro: kq.trang.con_tro,
        con_nua: kq.trang.con_nua,
        da_co_trang_dau: true,
        dang_tai: false,
        loi: null,
      };
    }
    case "chua-co-phien":
    case "chua-cau-hinh":
      return { ...ds, dang_tai: false, kenh_dong: true };
    case "het-phien":
    case "loi-mang":
      return { ...ds, dang_tai: false, loi: kq.kieu };
    default:
      return { ...ds, dang_tai: false, loi: "loi-may-chu" };
  }
}

function moc(iso: string): string {
  return `${thoiDiemVN(iso) ?? THOI_DIEM_KHONG_DOC_DUOC} ${THE_PHIEU.gio_vn}`;
}

/**
 * MỘT THẺ = MỘT NÚT to bằng cả thẻ: chạm đâu cũng mở phiếu. Mã tra cứu to nhất, đứng đầu (luật 10,
 * bất biến 1). Trạng thái bằng CHỮ, không bằng màu (README §Non-negotiables #6).
 *
 * "Hạn xử lý xong" chỉ hiện khi đã có: `null` là CHƯA CÓ cam kết, và danh sách không phải chỗ giải
 * thích điều đó — màn tra cứu có câu riêng.
 */
export function ThePhieuTomTat({ phieu, onMo }: { phieu: PhieuCuaToiTomTat; onMo: (ma: string) => void }) {
  return (
    <li className="cd-cua-toi__muc">
      <button type="button" className="cd-the-cua-toi" onClick={() => onMo(phieu.ma_tra_cuu)}>
        <span className="cd-the-cua-toi__ma">{phieu.ma_tra_cuu}</span>
        <span className="cd-the-cua-toi__dong">
          {THE_PHIEU.trang_thai}: <strong className="cd-the-cua-toi__trang-thai">{nhanTrangThai(phieu.trang_thai)}</strong>
        </span>
        <span className="cd-the-cua-toi__dong">
          {THE_PHIEU.linh_vuc}: {nhanLinhVuc(phieu.linh_vuc, phieu.nhan_linh_vuc)}
        </span>
        <span className="cd-the-cua-toi__trich">{phieu.trich_noi_dung}</span>
        <span className="cd-the-cua-toi__dong">
          {THE_PHIEU.gui_luc}: {moc(phieu.goc_dem_han)}
        </span>
        {phieu.han_xu_ly_xong !== null && (
          <span className="cd-the-cua-toi__dong">
            {THE_PHIEU.han_xu_ly}: {moc(phieu.han_xu_ly_xong)}
          </span>
        )}
        <span className="cd-the-cua-toi__xem">{CUA_TOI.nut_xem}</span>
      </button>
    </li>
  );
}

const CAU_LOI: Readonly<Record<LoiDanhSach, string>> = {
  "het-phien": CUA_TOI.het_phien,
  "loi-mang": CUA_TOI.loi_mang,
  "loi-may-chu": CUA_TOI.loi_may_chu,
};

/** Thân màn khi đã có phiên, THUẦN — nhận trạng thái và việc cần làm qua tham số. */
export function ThanDanhSach(props: {
  ds: DanhSach;
  onMo: (ma: string) => void;
  onTai: () => void;
  onGuiPhanAnh: () => void;
}) {
  const { ds } = props;
  return (
    <>
      {!ds.da_co_trang_dau && ds.dang_tai && (
        <p className="cd-cau" role="status">
          {CUA_TOI.dang_tai}
        </p>
      )}

      {ds.da_co_trang_dau && ds.muc.length === 0 && (
        <div className="cd-buoc">
          <p className="cd-cau">{CUA_TOI.trong}</p>
          <button type="button" className="cd-nut" onClick={props.onGuiPhanAnh}>
            {GUI.tieu_de}
          </button>
        </div>
      )}

      {ds.muc.length > 0 && (
        <ul className="cd-cua-toi">
          {ds.muc.map((p) => (
            <ThePhieuTomTat key={p.ma_tra_cuu} phieu={p} onMo={props.onMo} />
          ))}
        </ul>
      )}

      {ds.da_co_trang_dau && ds.dang_tai && (
        <p className="cd-cau" role="status">
          {CUA_TOI.dang_tai_them}
        </p>
      )}

      {ds.loi !== null && (
        <div className="cd-buoc">
          <p className="cd-loi" role="alert">
            {CAU_LOI[ds.loi]}
          </p>
          {/* Hết phiên thì bấm lại không đổi được gì — câu đã nói việc cần làm. */}
          {ds.loi !== "het-phien" && (
            <button type="button" className="cd-nut" disabled={ds.dang_tai} onClick={props.onTai}>
              {CUA_TOI.nut_thu_lai}
            </button>
          )}
        </div>
      )}

      {ds.loi === null && ds.con_nua && (
        <button type="button" className="cd-nut-phu" disabled={ds.dang_tai} onClick={props.onTai}>
          {CUA_TOI.nut_xem_them}
        </button>
      )}

      {ds.da_co_trang_dau && !ds.con_nua && ds.muc.length > 0 && (
        <p className="cd-ghi-chu">{CUA_TOI.het_danh_sach}</p>
      )}
    </>
  );
}

export function PhanAnhCuaToiScreen(props: {
  onQuayLai: () => void;
  onMoPhieu: (ma: string) => void;
  onGuiPhanAnh: () => void;
}) {
  // Đọc một lần lúc dựng. `null` hôm nay — xem `api/phien-vigov.ts`.
  const [phien] = useState(layPhienViGov);
  const [ds, datDs] = useState<DanhSach>(DANH_SACH_DAU);
  // Tải trang đầu ĐÚNG MỘT LẦN, kể cả khi React dựng hiệu ứng hai lần (StrictMode).
  const da_tai_dau = useRef(false);

  /** Tải trang kế — trang đầu khi `con_tro` còn rỗng. Dùng cho cả "Xem thêm" lẫn "Thử lại". */
  async function tai(con_tro: string) {
    datDs(batDauTai);
    const kq = await phanAnhCuaToi(con_tro);
    datDs((truoc) => sauKhiTai(truoc, kq));
  }

  useEffect(() => {
    if (phien === null || da_tai_dau.current) return;
    da_tai_dau.current = true;
    void tai("");
  }, [phien]);

  const nutQuayLai = (
    <button type="button" className="quay-lai" onClick={props.onQuayLai}>
      {QUAY_LAI}
    </button>
  );

  if (phien === null || ds.kenh_dong) {
    return (
      <section className="cd-man" aria-label={CUA_TOI.tieu_de}>
        {nutQuayLai}
        <h1 className="cd-tieu-de">{CUA_TOI.tieu_de}</h1>
        <KenhChuaMo />
      </section>
    );
  }

  return (
    <section className="cd-man" aria-label={CUA_TOI.tieu_de}>
      {nutQuayLai}
      <BangXa ten_xa={phien.ten_xa} />
      <h1 className="cd-tieu-de">{CUA_TOI.tieu_de}</h1>
      <ThanDanhSach
        ds={ds}
        onMo={props.onMoPhieu}
        onTai={() => {
          if (!ds.dang_tai) void tai(ds.con_tro);
        }}
        onGuiPhanAnh={props.onGuiPhanAnh}
      />
    </section>
  );
}
