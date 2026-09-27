"use client";

import { useEffect, useState } from "react";

import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import { layNhatKyNhiemVu } from "@/lib/api/nhiem-vu";
import type {
  page_Result_petitions_nhatKyNhiemVuRa,
  petitions_nhatKyNhiemVuRa,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_NHAT_KY_NHIEM_VU,
  NHAN_XEM_THEM_NHAT_KY_NHIEM_VU,
  NHAT_KY_RONG,
  TIEU_DE_NHAT_KY_NHIEM_VU,
  gopTrangNhatKy,
  hienDongNhatKy,
  type BangNhanTrangThai,
} from "./nhan-nhiem-vu";

/**
 * Nhật ký & Trao đổi của MỘT nhiệm vụ (§5.9) — NỬA ĐỌC: dòng thời gian mới nhất trước, `Xem thêm`
 * theo con trỏ. Không có ô ghi tay — xem `PHAN_CHUA_DUNG`.
 *
 * CÙNG KHUÔN `features/phan-anh/nhat-ky-phieu.tsx`, KHÔNG DÙNG CHUNG THÀNH PHẦN: hai hợp đồng khác
 * hình dạng (dòng phiếu có `action` và dòng `phan-cong` riêng; dòng nhiệm vụ không có `action`, còn
 * nhãn trạng thái là của XÃ qua `BangNhanTrangThai` chứ không phải bảng chép tay của phiếu). Gộp
 * hai cái thành một là một thành phần rẽ nhánh theo loại hồ sơ ở mọi dòng — lớn hơn cả hai bản.
 *
 * GHI CHÚ HIỆN BẰNG TEXT NODE CỦA REACT, KHÔNG BAO GIỜ `dangerouslySetInnerHTML`: đó là chữ tự do do
 * cán bộ gõ lúc đổi trạng thái, và một thẻ `<img onerror>` trong đó sẽ chạy trên máy mọi người mở
 * drawer.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100. */
const SO_DONG_MOI_TRANG = 20;

type TrangDoc =
  | {
      khoa: string;
      ok: true;
      dong: readonly petitions_nhatKyNhiemVuRa[];
      conTro: string;
      conNua: boolean;
    }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_nhatKyNhiemVuRa>): TrangDoc {
  return kq.ok
    ? {
        khoa,
        ok: true,
        dong: kq.duLieu.items,
        conTro: kq.duLieu.next_cursor,
        conNua: kq.duLieu.has_more,
      }
    : { khoa, ok: false, thongBao: kq.thongBao };
}

export function NhatKyNhiemVu({
  maNhiemVu,
  nhanTT,
  danhBa,
  tenBoPhan,
  lanLamMoi,
}: {
  maNhiemVu: string;
  nhanTT: BangNhanTrangThai;
  /** Danh bạ chọn người tra theo mã. `null` = chưa có (đang tải / tải hỏng) — dòng hiện mã. */
  danhBa: DanhBaTheoMa | null;
  tenBoPhan: ReadonlyMap<string, string>;
  /**
   * Đổi ở mỗi lần mở drawer và sau mỗi lần ghi thành công (`DrawerNhiemVu.luotDoc`) — máy chủ vừa
   * ghi thêm một dòng, nên trang đang hiện đã cũ.
   */
  lanLamMoi: number;
}) {
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<{ khoa: string; thongBao: string } | null>(null);

  const khoaDoc = `${maNhiemVu}|${lanLamMoi}`;

  useEffect(() => {
    let bo = false;
    layNhatKyNhiemVu(maNhiemVu, null, SO_DONG_MOI_TRANG).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [maNhiemVu, khoaDoc]);

  // Câu trả lời của một lần đọc CŨ (nhiệm vụ khác, trước lần làm mới) không được vẽ như của lần
  // này — đổi nhiệm vụ là về lại "đang tải", không bao giờ hiện nhật ký của việc vừa đóng.
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;
  const loiThemHienTai = loiThem !== null && loiThem.khoa === khoaDoc ? loiThem.thongBao : null;

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layNhatKyNhiemVu(maNhiemVu, truoc.conTro, SO_DONG_MOI_TRANG).then((kq) => {
      datDangTaiThem(false);
      if (!kq.ok) {
        datLoiThem({ khoa: truoc.khoa, thongBao: kq.thongBao });
        return;
      }
      datLoiThem(null);
      datDoc((d) =>
        d === null || d.khoa !== truoc.khoa || !d.ok
          ? d
          : {
              ...d,
              dong: gopTrangNhatKy(d.dong, kq.duLieu.items),
              conTro: kq.duLieu.next_cursor,
              conNua: kq.duLieu.has_more,
            },
      );
    });
  }

  return (
    <KhoiNhatKyNhiemVu
      maNhiemVu={maNhiemVu}
      tai={
        hienTai === null
          ? { pha: "dangTai" }
          : hienTai.ok
            ? { pha: "xong", dong: hienTai.dong, conNua: hienTai.conNua && hienTai.conTro !== "" }
            : { pha: "loi", thongBao: hienTai.thongBao }
      }
      nhanTT={nhanTT}
      danhBa={danhBa}
      tenBoPhan={tenBoPhan}
      loiThem={loiThemHienTai}
      dangTaiThem={dangTaiThem}
      xemThem={xemThem}
    />
  );
}

/** Ba pha của khối. `conNua` đã gộp hai điều kiện: `has_more` VÀ có `next_cursor` để đi tới. */
export type TaiNhatKyNhiemVu =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; dong: readonly petitions_nhatKyNhiemVuRa[]; conNua: boolean };

/**
 * Phần vẽ, không đọc mạng — tách ra để kiểm bằng `renderToStaticMarkup`.
 *
 * TẢI HỎNG KHÔNG VẼ CÂU "Chưa có ghi chép nào.": một nhật ký rỗng vì đọc hỏng trông y hệt một nhiệm
 * vụ chưa ai đụng tới. Câu máy chủ ra NGUYÊN VĂN, `role="alert"`.
 */
export function KhoiNhatKyNhiemVu({
  maNhiemVu,
  tai,
  nhanTT,
  danhBa,
  tenBoPhan,
  loiThem,
  dangTaiThem,
  xemThem,
}: {
  maNhiemVu: string;
  tai: TaiNhatKyNhiemVu;
  nhanTT: BangNhanTrangThai;
  danhBa: DanhBaTheoMa | null;
  tenBoPhan: ReadonlyMap<string, string>;
  loiThem: string | null;
  dangTaiThem: boolean;
  xemThem: () => void;
}) {
  const idTieuDe = `tieu-de-nhat-ky-nhiem-vu-${maNhiemVu}`;
  return (
    <section className="khoi-chi-tiet" aria-labelledby={idTieuDe}>
      <h4 id={idTieuDe}>{TIEU_DE_NHAT_KY_NHIEM_VU}</h4>

      {tai.pha === "dangTai" && <p role="status">{DANG_TAI_NHAT_KY_NHIEM_VU}</p>}
      {tai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {tai.thongBao}
        </p>
      )}
      {tai.pha === "xong" && tai.dong.length === 0 && (
        <p className="trang-thai-rong">{NHAT_KY_RONG}</p>
      )}
      {tai.pha === "xong" && tai.dong.length > 0 && (
        <ol aria-label="Nhật ký nhiệm vụ, mới nhất trước">
          {tai.dong.map((d) => {
            const h = hienDongNhatKy(d, nhanTT, danhBa, tenBoPhan);
            return (
              <li key={h.id}>
                <div className="dau-khoi-chi-tiet">
                  <time dateTime={h.luc}>{h.thoiDiem}</time>
                  <strong>{h.nguoi}</strong>
                  <span className="chip chip-ngung">{h.trangThai}</span>
                </div>
                {h.phanCong !== null && <p className="ghi-chu">{h.phanCong}</p>}
                <p className="ghi-chu-nhat-ky">{h.ghiChu}</p>
              </li>
            );
          })}
        </ol>
      )}
      {tai.pha === "xong" && loiThem !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiThem}
        </p>
      )}
      {tai.pha === "xong" && tai.conNua && (
        <button type="button" className="nut-phu" disabled={dangTaiThem} onClick={xemThem}>
          {NHAN_XEM_THEM_NHAT_KY_NHIEM_VU}
        </button>
      )}
    </section>
  );
}
