"use client";

import { useCallback, type KeyboardEvent } from "react";

import { traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import {
  DAN_CHUYEN_XU_LY,
  DANG_TAI_CHI_TIET,
  DANG_TAI_LICH_SU,
  LICH_SU_RONG,
  LOI_THIEU_LY_DO_CHUYEN,
  NUT_DONG_CHI_TIET,
  NUT_XAC_NHAN_CHUYEN,
  O_CAN_BO_XU_LY,
  O_CO_QUAN_BAN_HANH,
  O_DEN_BO_PHAN,
  O_LY_DO_CHUYEN,
  TIEU_DE_DONG_THOI_GIAN,
  TIEU_DE_KHOI_CHUYEN,
  lopHanVanBan,
  nhanBoPhanDangGiu,
  nhanCanBo,
  nhanDoKhan,
  nhanHanVanBan,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanTieuDeVanBanDen,
  nhanTrangThai,
  nhanTuBoPhan,
  trangThaiHanVanBan,
  type BanChuyen,
} from "./nhan-van-ban";

/**
 * Ngăn chi tiết MỘT văn bản đến — `docs/ui-ux/05-van-ban-don-thu.md §3.5`, phần áp được cho văn bản
 * đến: tiêu đề, trích yếu, hàng chip, các ô thông tin, dòng thời gian chuyển tiếp CHỈ ĐỌC, và khối
 * "Chuyển cho bộ phận khác".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MỘT KHỐI TRONG LUỒNG TRANG, KHÔNG PHẢI LỚP PHỦ — cùng khuôn `khoi-chi-tiet` mà ngăn chi tiết nhiệm
 * vụ và phiếu phản ánh đang dùng, vì ứng dụng chưa có thành phần lớp phủ nào để dùng lại. Vì không
 * phải hộp thoại nên KHÔNG có bẫy tiêu điểm: bẫy tiêu điểm trong một vùng không che phần còn lại của
 * trang là nhốt người dùng bàn phím khỏi một trang họ vẫn nhìn thấy. Thay vào đó: mở thì tiêu điểm
 * sang tiêu đề ngăn, `Esc` khi tiêu điểm ở trong ngăn thì đóng, đóng thì tiêu điểm về nút "Xem chi
 * tiết" của đúng dòng ấy (bên gọi làm việc ấy — nó biết dòng nào).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ NÚT ĐỔI TRẠNG THÁI, KHÔNG CÓ "CHUYỂN THÀNH NHIỆM VỤ", có chủ ý: bộ trạng thái của văn bản
 * đến còn là câu hỏi mở của khách hàng (C2), và biến văn bản thành nhiệm vụ cần một hợp đồng giữa
 * hai dịch vụ chưa có. Một nút ở đây là một quyết định nghiệp vụ không ai đưa ra.
 *
 * KHÔNG CÓ NÚT NÀO SỬA HAY XOÁ MỘT DÒNG LỊCH SỬ: bảng lịch sử chỉ-thêm ở tầng CSDL (trigger
 * `lich_su_chuyen_chi_them`) và hợp đồng không có tuyến nào làm việc ấy (luật 7, cấm #5).
 */
export function NganVanBanDen({
  vb,
  lichSu,
  bayGio,
  traLoai,
  traBoPhan,
  coQuyenChuyen,
  ban,
  datBan,
  loi,
  dangGui,
  cauDaXong,
  tieuDiemChuyen,
  onGui,
  onDong,
}: {
  /** `null` = đang đọc. Lỗi hiện NGUYÊN câu máy chủ — 404 là một câu chung cho mọi ca. */
  vb: KetQua<documents_vanBanDenRa> | null;
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  /** Thời điểm hiện tại TRUYỀN VÀO: quá hạn là phép so sánh lúc vẽ, không phải trường lưu sẵn. */
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
  /** `document.route` — TIỆN DỤNG, không phải biện pháp: máy chủ kiểm lại trên từng yêu cầu. */
  coQuyenChuyen: boolean;
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  loi: string;
  dangGui: boolean;
  cauDaXong: string;
  /** Mở từ nút "Chuyển xử lý" của dòng: tiêu điểm vào ô bộ phận thay vì tiêu đề. */
  tieuDiemChuyen: boolean;
  onGui: () => void;
  onDong: () => void;
}) {
  const coVanBan = vb !== null && vb.ok;
  const tieuDe = coVanBan
    ? nhanTieuDeVanBanDen(vb.duLieu.number, vb.duLieu.year, vb.duLieu.received_date)
    : "Chi tiết văn bản đến";

  // TIÊU ĐIỂM LÚC MỞ, bằng ref gọi lại ỔN ĐỊNH. Một hàm viết thẳng trong JSX là hàm MỚI mỗi lần
  // vẽ, React gọi lại nó ở mỗi lần vẽ, và tiêu điểm bị giật về tiêu đề sau mỗi phím gõ vào ô lý do.
  // Khi mở để chuyển xử lý, ô bộ phận nhận tiêu điểm thay — xem `KhoiChuyenXuLy`.
  const tieuDiemTieuDe = useCallback(
    (el: HTMLHeadingElement | null) => {
      if (el !== null && !tieuDiemChuyen) el.focus();
    },
    [tieuDiemChuyen],
  );

  function banPhim(e: KeyboardEvent<HTMLElement>) {
    if (e.key === "Escape") {
      e.stopPropagation();
      onDong();
    }
  }

  return (
    <section
      className="khoi-chi-tiet"
      aria-labelledby="tieu-de-ngan-van-ban-den"
      onKeyDown={banPhim}
    >
      <div className="dau-khoi-chi-tiet">
        <h3
          id="tieu-de-ngan-van-ban-den"
          tabIndex={-1}
          ref={tieuDiemTieuDe}
        >
          {tieuDe}
        </h3>
        <button type="button" className="nut-phu" onClick={onDong} aria-label={NUT_DONG_CHI_TIET}>
          ✕
        </button>
      </div>

      {vb === null && <p role="status">{DANG_TAI_CHI_TIET}</p>}
      {vb !== null && !vb.ok && (
        <p className="thong-bao-loi" role="alert">
          {vb.thongBao}
        </p>
      )}

      {coVanBan && (
        <>
          <ThongTinVanBanDen vb={vb.duLieu} bayGio={bayGio} traLoai={traLoai} traBoPhan={traBoPhan} />

          <DongThoiGianChuyen lichSu={lichSu} traBoPhan={traBoPhan} />

          {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}

          {coQuyenChuyen && (
            <KhoiChuyenXuLy
              ban={ban}
              datBan={datBan}
              traBoPhan={traBoPhan}
              loi={loi}
              dangGui={dangGui}
              tieuDiem={tieuDiemChuyen}
              onGui={onGui}
            />
          )}
        </>
      )}
    </section>
  );
}

/** Trích yếu, hàng chip, và các ô thông tin. Xuất ra để bài kiểm vẽ được từng phần. */
export function ThongTinVanBanDen({
  vb,
  bayGio,
  traLoai,
  traBoPhan,
}: {
  vb: documents_vanBanDenRa;
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
}) {
  // SUY RA LÚC VẼ, không lưu (luật 10, bất biến 3) — cùng hàm cột "Hạn xử lý" của bảng dùng.
  const han = trangThaiHanVanBan(vb.due_at, bayGio);
  const soKyHieu = vb.reference_no === undefined || vb.reference_no === "" ? "Không ghi" : vb.reference_no;

  return (
    <>
      <p className="o-trich-yeu">{vb.summary}</p>

      <p className="cum-nut" aria-label="Trạng thái và thuộc tính">
        <span className="chip chip-hoat-dong">{nhanTrangThai(vb.status)}</span>{" "}
        <span className="chip chip-ngung">{nhanLoaiVanBan(traTen(traLoai, vb.document_type))}</span>{" "}
        <span className="chip chip-ngung">{nhanDoKhan(vb.urgency ?? "")}</span>
      </p>

      <dl className="danh-sach-truong">
        <dt>{O_CO_QUAN_BAN_HANH}</dt>
        <dd>{vb.issuing_body}</dd>

        <dt>Số, ký hiệu · ngày văn bản</dt>
        <dd>
          {soKyHieu} · {nhanNgayCoThe(vb.document_date ?? "")}
        </dd>

        <dt>Bộ phận đang giữ</dt>
        <dd>
          {nhanBoPhanDangGiu(traTen(traBoPhan, vb.holding_unit ?? ""))} ·{" "}
          {nhanCanBo(vb.assignee ?? "")}
        </dd>

        <dt>Hạn xử lý</dt>
        <dd>
          <span className={lopHanVanBan(han)}>{nhanHanVanBan(han)}</span>
        </dd>

        <dt>Người vào sổ</dt>
        <dd>{vb.created_by}</dd>
      </dl>
    </>
  );
}

/**
 * Dòng thời gian chuyển tiếp — CHỈ ĐỌC, GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ (cũ nhất trước).
 *
 * Không sắp lại ở client: thứ tự là thứ tự các lần chuyển thật, và máy chủ đã xếp theo `routed_at`.
 * Người chuyển và cán bộ được giao hiện bằng MÃ CÁN BỘ — xem `nhanCanBo`.
 */
export function DongThoiGianChuyen({
  lichSu,
  traBoPhan,
}: {
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  traBoPhan: BangTraDanhMuc;
}) {
  return (
    <div aria-labelledby="tieu-de-dong-thoi-gian">
      <h4 id="tieu-de-dong-thoi-gian">{TIEU_DE_DONG_THOI_GIAN}</h4>
      {lichSu === null && <p role="status">{DANG_TAI_LICH_SU}</p>}
      {lichSu !== null && !lichSu.ok && (
        <p className="thong-bao-loi" role="alert">
          {lichSu.thongBao}
        </p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length === 0 && (
        <p className="trang-thai-rong">{LICH_SU_RONG}</p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length > 0 && (
        <ol className="danh-sach-truong">
          {lichSu.duLieu.items.map((d) => (
            <DongLichSu key={d.id} d={d} traBoPhan={traBoPhan} />
          ))}
        </ol>
      )}
    </div>
  );
}

function DongLichSu({ d, traBoPhan }: { d: documents_lichSuChuyenRa; traBoPhan: BangTraDanhMuc }) {
  return (
    <li>
      <p>
        <strong>{d.routed_by}</strong> · <time dateTime={d.routed_at}>{nhanThoiDiem(d.routed_at)}</time>
      </p>
      <p>
        <span className="chip chip-ngung">{nhanTrangThai(d.status)}</span>
      </p>
      <p>
        {nhanTuBoPhan(traTen(traBoPhan, d.from_unit ?? ""))} →{" "}
        {nhanBoPhanDangGiu(traTen(traBoPhan, d.to_unit))}
      </p>
      <p>Phụ trách: {nhanCanBo(d.assignee ?? "")}</p>
      <p>{d.reason}</p>
    </li>
  );
}

/**
 * Khối "Chuyển cho bộ phận khác" — chuyển từ biểu mẫu trong trang của sổ vào đây, giữ nguyên ba ô
 * và phép kiểm (`guiChuyenVanBan` ở `thao-tac-van-ban.ts`).
 *
 * `reason` LÀ CHỮ TỰ DO CÓ THỂ NHẮC TÊN MỘT CÔNG DÂN: nó chỉ sống trong trạng thái React và trong
 * thân `POST`, không vào log, URL, `localStorage` hay một thuộc tính nào (luật 3).
 */
export function KhoiChuyenXuLy({
  ban,
  datBan,
  traBoPhan,
  loi,
  dangGui,
  tieuDiem,
  onGui,
}: {
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  traBoPhan: BangTraDanhMuc;
  loi: string;
  dangGui: boolean;
  tieuDiem: boolean;
  onGui: () => void;
}) {
  // Ổn định vì cùng lý do với tiêu đề ngăn: không giật tiêu điểm khỏi ô đang gõ.
  const tieuDiemO = useCallback(
    (el: HTMLSelectElement | null) => {
      if (el !== null && tieuDiem) el.focus();
    },
    [tieuDiem],
  );

  return (
    <form
      className="form-danh-muc"
      aria-labelledby="tieu-de-khoi-chuyen"
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4 id="tieu-de-khoi-chuyen">{TIEU_DE_KHOI_CHUYEN}</h4>
      <p className="canh-bao-pham-vi">{DAN_CHUYEN_XU_LY}</p>

      <div className="o-nhap">
        <label htmlFor="o-den-bo-phan">{O_DEN_BO_PHAN}</label>
        <select
          id="o-den-bo-phan"
          name="denBoPhan"
          value={ban.denBoPhan}
          ref={tieuDiemO}
          onChange={(e) => datBan({ ...ban, denBoPhan: e.target.value })}
        >
          <option value="">— Chọn bộ phận —</option>
          {traBoPhan.pha === "xong" &&
            [...traBoPhan.ten].map(([id, ten]) => (
              <option key={id} value={id}>
                {ten}
              </option>
            ))}
        </select>
      </div>

      <div className="o-nhap">
        <label htmlFor="o-can-bo-xu-ly">{O_CAN_BO_XU_LY}</label>
        {/* Ô CHỮ, KHÔNG PHẢI Ô CHỌN CÁN BỘ: hợp đồng nhận `assignee` là MÃ CÁN BỘ (`CB-00123`),
            và danh bạ cán bộ đòi khoá `admin.user` — một người có `document.route` chưa chắc
            đọc được danh bạ. Vẽ một ô chọn rỗng cho họ là vẽ một ô không bao giờ dùng được. */}
        <input
          id="o-can-bo-xu-ly"
          name="canBoXuLy"
          value={ban.canBoXuLy}
          autoComplete="off"
          onChange={(e) => datBan({ ...ban, canBoXuLy: e.target.value })}
          placeholder="Để trống nếu để bộ phận tự phân công"
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="o-ly-do-chuyen">{O_LY_DO_CHUYEN}</label>
        <input
          id="o-ly-do-chuyen"
          name="lyDoChuyen"
          required
          autoComplete="off"
          value={ban.lyDo}
          onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
          aria-invalid={loi === LOI_THIEU_LY_DO_CHUYEN}
        />
      </div>

      {/* CÂU TỪ CHỐI RA NGUYÊN VĂN, dù từ phép kiểm ở client hay từ máy chủ. */}
      {loi !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {NUT_XAC_NHAN_CHUYEN}
        </button>
      </div>
    </form>
  );
}
