"use client";

import { useEffect, useState } from "react";

import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import { layHangChoLuiHan, quyetDinhLuiHan } from "@/lib/api/nhiem-vu";
import type {
  page_Result_petitions_deNghiChoDuyetRa,
  petitions_deNghiChoDuyetRa,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_HANG_CHO,
  ID_HANG_CHO,
  LOC_HANG_CHO_MAC_DINH,
  NHAN_LOC_CHO_TOI,
  NHAN_LOC_TOAN_XA,
  NHAN_XEM_THEM_HANG_CHO,
  TIEU_DE_HANG_CHO,
  boDeNghiDaQuyet,
  cauHangChoRong,
  gopTrangNhatKy,
  hienDongHangCho,
  thamSoHangCho,
  type LocHangCho,
} from "./nhan-nhiem-vu";

/**
 * Hàng chờ duyệt lùi hạn §5.8 — một mục trên sổ `/nhiem-vu`, không phải trong drawer. Vì sao: khối
 * chú thích `HÀNG CHỜ DUYỆT LÙI HẠN` ở `nhan-nhiem-vu.ts`.
 *
 * KHÔNG CÓ CÂU TỪ CHỐI NÀO VIẾT Ở ĐÂY. Quyết định đi qua `task.extend` ở cổng rồi phép so
 * `lanh_dao_giao_viec_ma` trong giao dịch (ADR 0038); cả hai lần từ chối (403, 409 khi đề nghị đã
 * được người khác quyết) ra NGUYÊN VĂN dưới đúng dòng, `role="alert"`.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100. */
const SO_DONG_MOI_TRANG = 20;

type TrangDoc =
  | {
      khoa: string;
      ok: true;
      dong: readonly petitions_deNghiChoDuyetRa[];
      conTro: string;
      conNua: boolean;
    }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_deNghiChoDuyetRa>): TrangDoc {
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

export function HangChoLuiHan({
  danhBa,
  maNguoiDangNhap,
  lanLamMoi,
  moNhiemVu,
  daQuyet,
}: {
  /** Danh bạ chọn người tra theo mã. `null` = chưa có — dòng hiện mã. */
  danhBa: DanhBaTheoMa | null;
  /** `phien.staff.code`, rỗng khi chưa đọc được phiên. */
  maNguoiDangNhap: string;
  /** Đổi thì đọc lại trang đầu — sau một lần gửi đề nghị hay một lần ghi trên sổ. */
  lanLamMoi: number;
  /** Mở drawer của một mã. Trả `KetQua` để câu lỗi (404 một câu) hiện ngay tại hàng chờ. */
  moNhiemVu: (ma: string) => Promise<KetQua<unknown>>;
  /** Một quyết định thành công — hạn của nhiệm vụ `ma` có thể đã đổi. */
  daQuyet: (ma: string) => void;
}) {
  const [loc, datLoc] = useState<LocHangCho>(LOC_HANG_CHO_MAC_DINH);
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<{ khoa: string; thongBao: string } | null>(null);
  const [dangQuyet, datDangQuyet] = useState<string | null>(null);
  const [loiDong, datLoiDong] = useState<{ id: string; thongBao: string } | null>(null);
  const [loiMo, datLoiMo] = useState<string | null>(null);
  const [ghiChu, datGhiChu] = useState<Readonly<Record<string, string>>>({});

  const khoaDoc = `${loc}|${lanLamMoi}`;

  useEffect(() => {
    let bo = false;
    layHangChoLuiHan({ ...thamSoHangCho(loc), limit: SO_DONG_MOI_TRANG }).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [loc, khoaDoc]);

  // Câu trả lời của bộ lọc cũ không được vẽ dưới bộ lọc mới — đổi lọc là về lại "đang tải".
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;
  const loiThemHienTai = loiThem !== null && loiThem.khoa === khoaDoc ? loiThem.thongBao : null;

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layHangChoLuiHan({
      ...thamSoHangCho(loc),
      limit: SO_DONG_MOI_TRANG,
      cursor: truoc.conTro,
    }).then((kq) => {
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

  function quyetDinh(d: petitions_deNghiChoDuyetRa, duyet: boolean): void {
    datDangQuyet(d.id);
    datLoiDong(null);
    quyetDinhLuiHan(d.task_code, d.id, duyet, (ghiChu[d.id] ?? "").trim()).then((kq) => {
      datDangQuyet(null);
      if (!kq.ok) {
        datLoiDong({ id: d.id, thongBao: kq.thongBao });
        return;
      }
      // Bỏ dòng TẠI CHỖ, không đọc lại trang đầu: đọc lại sẽ vứt mọi trang `Xem thêm` đã mở.
      datDoc((t) => (t === null || !t.ok ? t : { ...t, dong: boDeNghiDaQuyet(t.dong, d.id) }));
      daQuyet(d.task_code);
    });
  }

  return (
    <KhoiHangChoLuiHan
      loc={loc}
      datLoc={(moi) => {
        datLoc(moi);
        datLoiDong(null);
        datLoiMo(null);
      }}
      tai={
        hienTai === null
          ? { pha: "dangTai" }
          : hienTai.ok
            ? { pha: "xong", dong: hienTai.dong, conNua: hienTai.conNua && hienTai.conTro !== "" }
            : { pha: "loi", thongBao: hienTai.thongBao }
      }
      danhBa={danhBa}
      maNguoiDangNhap={maNguoiDangNhap}
      dangQuyet={dangQuyet}
      loiDong={loiDong}
      loiMo={loiMo}
      loiThem={loiThemHienTai}
      dangTaiThem={dangTaiThem}
      ghiChu={ghiChu}
      datGhiChu={(id, giaTri) => datGhiChu((g) => ({ ...g, [id]: giaTri }))}
      quyetDinh={quyetDinh}
      moNhiemVu={(ma) => {
        datLoiMo(null);
        moNhiemVu(ma).then((kq) => {
          if (!kq.ok) datLoiMo(kq.thongBao);
        });
      }}
      xemThem={xemThem}
    />
  );
}

/** Ba pha của mục. `conNua` đã gộp hai điều kiện: `has_more` VÀ có `next_cursor` để đi tới. */
export type TaiHangCho =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; dong: readonly petitions_deNghiChoDuyetRa[]; conNua: boolean };

/**
 * Phần vẽ, không đọc mạng — tách ra để kiểm bằng `renderToStaticMarkup`.
 *
 * TẢI HỎNG KHÔNG VẼ CÂU RỖNG: "không có đề nghị nào chờ bạn" vì đọc hỏng là một lãnh đạo tin rằng
 * mình không nợ ai quyết định nào. Câu máy chủ ra NGUYÊN VĂN, `role="alert"`.
 */
export function KhoiHangChoLuiHan({
  loc,
  datLoc,
  tai,
  danhBa,
  maNguoiDangNhap,
  dangQuyet,
  loiDong,
  loiMo,
  loiThem,
  dangTaiThem,
  ghiChu,
  datGhiChu,
  quyetDinh,
  moNhiemVu,
  xemThem,
}: {
  loc: LocHangCho;
  datLoc: (loc: LocHangCho) => void;
  tai: TaiHangCho;
  danhBa: DanhBaTheoMa | null;
  maNguoiDangNhap: string;
  /** `id` của đề nghị đang chờ máy chủ trả lời, hoặc `null`. */
  dangQuyet: string | null;
  loiDong: { id: string; thongBao: string } | null;
  loiMo: string | null;
  loiThem: string | null;
  dangTaiThem: boolean;
  ghiChu: Readonly<Record<string, string>>;
  datGhiChu: (id: string, giaTri: string) => void;
  quyetDinh: (d: petitions_deNghiChoDuyetRa, duyet: boolean) => void;
  moNhiemVu: (ma: string) => void;
  xemThem: () => void;
}) {
  const idTieuDe = `tieu-de-${ID_HANG_CHO}`;
  return (
    <section id={ID_HANG_CHO} className="khoi-chi-tiet" aria-labelledby={idTieuDe}>
      <h3 id={idTieuDe}>{TIEU_DE_HANG_CHO}</h3>

      <div className="o-chon" role="group" aria-label="Lọc đề nghị lùi hạn">
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc === "cua-toi"}
          onClick={() => datLoc("cua-toi")}
        >
          {NHAN_LOC_CHO_TOI}
        </button>
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc === "toan-xa"}
          onClick={() => datLoc("toan-xa")}
        >
          {NHAN_LOC_TOAN_XA}
        </button>
      </div>

      {loiMo !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiMo}
        </p>
      )}

      {tai.pha === "dangTai" && <p role="status">{DANG_TAI_HANG_CHO}</p>}
      {tai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {tai.thongBao}
        </p>
      )}
      {tai.pha === "xong" && tai.dong.length === 0 && (
        <p className="trang-thai-rong">{cauHangChoRong(loc)}</p>
      )}
      {tai.pha === "xong" && tai.dong.length > 0 && (
        <ol aria-label={`${TIEU_DE_HANG_CHO}, chờ lâu nhất trước`}>
          {tai.dong.map((d) => {
            const h = hienDongHangCho(d, danhBa, maNguoiDangNhap);
            const idGhiChu = `ghi-chu-quyet-dinh-${h.id}`;
            const dangChay = dangQuyet === h.id;
            return (
              <li key={h.id}>
                <p>
                  <button
                    type="button"
                    className="nut-phu"
                    onClick={() => moNhiemVu(h.maNhiemVu)}
                    aria-label={`Mở ${h.maNhiemVu}: ${h.tieuDe}`}
                  >
                    <span className="ma-muc">{h.maNhiemVu}</span> {h.tieuDe}
                  </button>
                </p>
                <dl className="danh-sach-truong">
                  <dt>Hạn đang có</dt>
                  <dd>{h.hanHienTai}</dd>
                  <dt>Hạn đề nghị</dt>
                  <dd>{h.hanDeNghi}</dd>
                  <dt>Lý do</dt>
                  <dd>{h.lyDo}</dd>
                  <dt>Người đề nghị</dt>
                  <dd>
                    {h.nguoiDeNghi} · <time dateTime={h.lucDeNghiISO}>{h.lucDeNghi}</time>
                  </dd>
                  <dt>Lãnh đạo giao việc</dt>
                  <dd>{h.lanhDao}</dd>
                </dl>

                {h.cauChan !== null && <p className="trang-thai-rong">{h.cauChan}</p>}

                {h.cauChan === null && (
                  <>
                    <div className="o-nhap">
                      <label htmlFor={idGhiChu}>Ghi chú quyết định (không bắt buộc)</label>
                      <input
                        id={idGhiChu}
                        name={idGhiChu}
                        value={ghiChu[h.id] ?? ""}
                        autoComplete="off"
                        onChange={(e) => datGhiChu(h.id, e.target.value)}
                      />
                    </div>
                    <div className="cum-nut">
                      <button
                        type="button"
                        className="nut-chinh"
                        disabled={dangQuyet !== null}
                        onClick={() => quyetDinh(d, true)}
                        aria-label={`Duyệt lùi hạn ${h.maNhiemVu}`}
                      >
                        {dangChay ? "Đang gửi…" : "Duyệt"}
                      </button>
                      <button
                        type="button"
                        className="nut-phu"
                        disabled={dangQuyet !== null}
                        onClick={() => quyetDinh(d, false)}
                        aria-label={`Từ chối lùi hạn ${h.maNhiemVu}`}
                      >
                        Từ chối
                      </button>
                    </div>
                  </>
                )}

                {loiDong !== null && loiDong.id === h.id && (
                  <p className="thong-bao-loi" role="alert">
                    {loiDong.thongBao}
                  </p>
                )}
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
          {NHAN_XEM_THEM_HANG_CHO}
        </button>
      )}
    </section>
  );
}
