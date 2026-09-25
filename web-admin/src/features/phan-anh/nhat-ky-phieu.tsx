"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { khoaSauLanGhi } from "@/features/thu-chi/nhan-thu-chi";
import type { KetQua } from "@/lib/api/goi";
import { ghiNhatKyPhieu, layNhatKyPhieu } from "@/lib/api/phieu-phan-anh";
import type {
  page_Result_petitions_nhatKyPhieuRa,
  petitions_nhatKyPhieuRa,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_NHAT_KY,
  demKyTu,
  GHI_CHU_TOI_DA,
  GOI_Y_GHI_NHAT_KY,
  laDongPhanCong,
  loiGhiChuNhatKy,
  NHAC_DU_LIEU_CA_NHAN,
  NHAN_BO_PHAN_PHU_TRACH,
  NHAN_NGUOI_THUC_HIEN,
  NHAN_NUT_GHI_NHAT_KY,
  NHAN_O_GHI_NHAT_KY,
  NHAN_XEM_THEM_NHAT_KY,
  NHAT_KY_RONG,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanThaoTacNhatKy,
  nhanThoiDiem,
  nhanTrangThai,
  TIEU_DE_NHAT_KY,
  type DanhBaTheoMa,
} from "./nhan-phieu";

/**
 * Nhật ký xử lý của MỘT phiếu (§8.7) — dòng thời gian mới nhất trước, "Xem thêm" theo con trỏ, và
 * nút `Ghi nhật ký`.
 *
 * NHẬT KÝ LÀ BẢN GHI NGHIỆP VỤ, KHÔNG PHẢI `audit_log`: cán bộ trong xã đọc nó để biết ai đã làm gì.
 * Máy chủ tự ghi một dòng ở mỗi thao tác xử lý; dòng `ghi-chu` là dòng cán bộ tự viết.
 *
 * NỘI DUNG GHI CHÚ HIỆN BẰNG TEXT NODE CỦA REACT, KHÔNG BAO GIỜ `dangerouslySetInnerHTML`: đó là chữ
 * tự do do người gõ, và một thẻ `<img onerror>` trong đó sẽ chạy trên máy của mọi cán bộ mở phiếu.
 * Xuống dòng giữ bằng CSS (`.ghi-chu-nhat-ky`), không bằng cách chèn `<br>`.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100, mặc định 20. */
const SO_DONG_MOI_TRANG = 20;

type TrangDoc =
  | { khoa: string; ok: true; dong: readonly petitions_nhatKyPhieuRa[]; conTro: string; conNua: boolean }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_nhatKyPhieuRa>): TrangDoc {
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

export function NhatKyPhieu({
  maTraCuu,
  tenBoPhan,
  danhBa,
  coNutGhi,
  lanLamMoi = 0,
}: {
  maTraCuu: string;
  tenBoPhan: ReadonlyMap<string, string>;
  /** Danh bạ tra theo mã cán bộ, cho ô `Phụ trách` của dòng `phan-cong`. `null` = chưa có. */
  danhBa: DanhBaTheoMa | null;
  /**
   * Có vẽ nút `Ghi nhật ký` không. CHỈ LÀ TIỆN DỤNG: người gọi truyền `feedback.read` của phiên.
   * Máy chủ mới quyết ai ghi được (người được phân công, hoặc có `feedback.resolve` / `.assign` /
   * `.classify`) và câu 403 của nó ra nguyên văn.
   */
  coNutGhi: boolean;
  /** Tăng lên sau mỗi thao tác xử lý thành công — máy chủ vừa ghi thêm một dòng. */
  lanLamMoi?: number;
}) {
  const [lanTaiLai, datLanTaiLai] = useState(0);
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<string | null>(null);

  const [moGhi, datMoGhi] = useState(false);
  const [noiDung, datNoiDung] = useState("");
  /**
   * Khoá chống trùng của BẢN NHÁP đang gõ. Giữ nguyên qua mọi lần gửi lại sau lỗi (kể cả lỗi mạng mà
   * lần đầu thật ra đã tới máy chủ); thay mới sau một lần 201 — `khoaSauLanGhi`.
   */
  const [khoa, datKhoa] = useState(khoaChongTrungMoi);
  const [dangGui, datDangGui] = useState(false);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const nutGhi = useRef<HTMLButtonElement>(null);

  const khoaDoc = `${maTraCuu}|${lanLamMoi}|${lanTaiLai}`;

  useEffect(() => {
    let bo = false;
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG }).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [maTraCuu, khoaDoc]);

  // Kết quả của một lần đọc CŨ (phiếu khác, trước lần làm mới) không được vẽ như của lần này.
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG, cursor: truoc.conTro }).then((kq) => {
      datDangTaiThem(false);
      if (!kq.ok) {
        datLoiThem(kq.thongBao);
        return;
      }
      datLoiThem(null);
      datDoc((d) =>
        d === null || d.khoa !== truoc.khoa || !d.ok
          ? d
          : {
              ...d,
              dong: [...d.dong, ...kq.duLieu.items],
              conTro: kq.duLieu.next_cursor,
              conNua: kq.duLieu.has_more,
            },
      );
    });
  }

  function dongBieuMau(): void {
    datMoGhi(false);
    // Bản nháp và khoá của nó ĐƯỢC GIỮ: mở lại là gõ tiếp, và gửi lại vẫn là cùng một lần ghi.
    nutGhi.current?.focus();
  }

  function gui(): void {
    datDangGui(true);
    ghiNhatKyPhieu(maTraCuu, noiDung, khoa).then((kq) => {
      datDangGui(false);
      datKhoa((k) => khoaSauLanGhi(k, kq.ok, khoaChongTrungMoi));
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ — 403 nói đúng vì sao tài khoản này không ghi được vào phiếu này.
        datLoiGhi(kq.thongBao);
        return;
      }
      datLoiGhi(null);
      datNoiDung("");
      datMoGhi(false);
      datLanTaiLai((n) => n + 1);
      nutGhi.current?.focus();
    });
  }

  return (
    <section className="khoi-chi-tiet" aria-labelledby={`tieu-de-nhat-ky-${maTraCuu}`}>
      <div className="dau-khoi-chi-tiet">
        <h4 id={`tieu-de-nhat-ky-${maTraCuu}`}>{TIEU_DE_NHAT_KY}</h4>
        {coNutGhi && (
          <button
            ref={nutGhi}
            type="button"
            className="nut-phu"
            aria-expanded={moGhi}
            aria-controls={`bieu-mau-nhat-ky-${maTraCuu}`}
            onClick={() => (moGhi ? dongBieuMau() : datMoGhi(true))}
          >
            {NHAN_NUT_GHI_NHAT_KY}
          </button>
        )}
      </div>

      {coNutGhi && moGhi && (
        <BieuMauGhiNhatKy
          id={`bieu-mau-nhat-ky-${maTraCuu}`}
          noiDung={noiDung}
          datNoiDung={datNoiDung}
          dangGui={dangGui}
          loi={loiGhi}
          gui={gui}
          huy={dongBieuMau}
        />
      )}

      {hienTai === null && <p role="status">{DANG_TAI_NHAT_KY}</p>}
      {hienTai !== null && !hienTai.ok && (
        <p className="thong-bao-loi" role="alert">
          {hienTai.thongBao}
        </p>
      )}
      {hienTai !== null && hienTai.ok && (
        <>
          <DanhSachNhatKy dong={hienTai.dong} tenBoPhan={tenBoPhan} danhBa={danhBa} />
          {loiThem !== null && (
            <p className="thong-bao-loi" role="alert">
              {loiThem}
            </p>
          )}
          {/* Hai điều kiện, không một: `has_more` nói còn dòng, `next_cursor` là đường đi tới đó. */}
          {hienTai.conNua && hienTai.conTro !== "" && (
            <button type="button" className="nut-phu" disabled={dangTaiThem} onClick={xemThem}>
              {NHAN_XEM_THEM_NHAT_KY}
            </button>
          )}
        </>
      )}
    </section>
  );
}

/**
 * Các dòng nhật ký, đúng thứ tự máy chủ trả (mới nhất trước). Tách ra để kiểm bằng HTML tĩnh.
 *
 * NGƯỜI THỰC HIỆN LÀ MÃ CÁN BỘ, không họ tên: máy chủ không trả tên, và mã là thứ còn chỉ ra được
 * đúng một người nhiều năm sau (luật 6, bất biến 8). Rỗng thì hiện gạch, không để ô trống.
 */
export function DanhSachNhatKy({
  dong,
  tenBoPhan,
  danhBa,
}: {
  dong: readonly petitions_nhatKyPhieuRa[];
  tenBoPhan: ReadonlyMap<string, string>;
  danhBa: DanhBaTheoMa | null;
}) {
  if (dong.length === 0) return <p className="trang-thai-rong">{NHAT_KY_RONG}</p>;

  return (
    <ol aria-label="Nhật ký xử lý, mới nhất trước">
      {dong.map((d) => (
        <li key={d.id}>
          <div className="dau-khoi-chi-tiet">
            <time dateTime={d.at}>{nhanThoiDiem(d.at)}</time>
            <strong>{nhanThaoTacNhatKy(d.action)}</strong>
            {d.status !== "" && <span className="chip chip-ngung">{nhanTrangThai(d.status)}</span>}
          </div>
          <dl className="danh-sach-truong">
            {laDongPhanCong(d.action) && (
              <>
                <dt>{NHAN_BO_PHAN_PHU_TRACH}</dt>
                <dd>
                  {nhanBoPhan(d.unit, tenBoPhan)} · {nhanCanBoXuLy(d.assignee, danhBa)}
                </dd>
              </>
            )}
            <dt>{NHAN_NGUOI_THUC_HIEN}</dt>
            <dd>{d.actor_code === "" ? "—" : d.actor_code}</dd>
          </dl>
          {d.note !== "" && <p className="ghi-chu-nhat-ky">{d.note}</p>}
        </li>
      ))}
    </ol>
  );
}

/**
 * Biểu mẫu `Ghi nhật ký`. Điều khiển từ ngoài (bản nháp sống ở `NhatKyPhieu`) để đóng rồi mở lại
 * không mất chữ và không đổi khoá chống trùng. `Esc` trong ô nhập là đóng.
 *
 * Nút gửi khoá khi trống hoặc quá 2000 ký tự — nhưng giới hạn thật ở máy chủ, và câu 400 của nó vẫn
 * ra nguyên văn nếu hai bên lệch nhau.
 */
export function BieuMauGhiNhatKy({
  id,
  noiDung,
  datNoiDung,
  dangGui,
  loi,
  gui,
  huy,
}: {
  id: string;
  noiDung: string;
  datNoiDung: (s: string) => void;
  dangGui: boolean;
  loi: string | null;
  gui: () => void;
  huy: () => void;
}) {
  const oNhap = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    oNhap.current?.focus();
  }, []);

  const loiO = loiGhiChuNhatKy(noiDung);
  const idO = `${id}-noi-dung`;

  function phim(e: KeyboardEvent<HTMLTextAreaElement>): void {
    if (e.key === "Escape") {
      e.preventDefault();
      huy();
    }
  }

  return (
    <form
      id={id}
      className="form-danh-muc"
      onSubmit={(e) => {
        e.preventDefault();
        if (loiO === null && !dangGui) gui();
      }}
    >
      <div className="o-nhap">
        <label htmlFor={idO}>{NHAN_O_GHI_NHAT_KY}</label>
        <textarea
          ref={oNhap}
          id={idO}
          name={idO}
          rows={4}
          value={noiDung}
          placeholder={GOI_Y_GHI_NHAT_KY}
          onChange={(e) => datNoiDung(e.target.value)}
          onKeyDown={phim}
          aria-describedby={`${idO}-dem ${idO}-nhac`}
        />
        <p className="ghi-chu" id={`${idO}-dem`} aria-live="polite">
          {demKyTu(noiDung)}/{GHI_CHU_TOI_DA} ký tự
          {loiO !== null && noiDung !== "" ? ` · ${loiO}` : ""}
        </p>
        <p className="ghi-chu" id={`${idO}-nhac`}>
          {NHAC_DU_LIEU_CA_NHAN}
        </p>
      </div>
      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}
      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui || loiO !== null}>
          Lưu vào nhật ký
        </button>
        <button type="button" className="nut-phu" onClick={huy} disabled={dangGui}>
          Huỷ
        </button>
      </div>
    </form>
  );
}
