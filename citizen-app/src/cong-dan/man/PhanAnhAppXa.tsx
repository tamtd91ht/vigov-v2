/**
 * PHẢN ÁNH TRONG APP RIÊNG — bản trải nghiệm theo bản mẫu `vi-gov/zalo-miniapp`
 * (`features/my-feedback/*`, `features/send-feedback/*`).
 *
 * PHIẾU CHỈ TRONG MÁY (`trai-nghiem.ts`): không một byte nào tới máy chủ của xã. Mọi màn gắn nhãn.
 *
 * KHÁC BẢN MẪU, CÓ CHỦ ĐÍCH:
 *   · KHÔNG CÓ BƯỚC CHỌN LĨNH VỰC. Lĩnh vực do cán bộ chốt, không do người dân chọn (ADR 0028, #23);
 *     bản mẫu có bước "Danh mục" kèm số ngày xử lý viết cứng — trái cả ADR ấy lẫn luật 10 (hạn tính
 *     bằng GIỜ LÀM VIỆC, theo cấu hình từng xã). Luồng còn hai bước: Nội dung → Xác nhận.
 *   · KHÔNG CÂU "CAM KẾT XỬ LÝ TRONG N NGÀY": con số ấy là cấu hình của xã (ADR 0007), chưa đọc được.
 *   · KHÔNG ẢNH HIỆN TRƯỜNG, KHÔNG GPS: ảnh cần kho tệp và đường tải lên, chưa có; vị trí nhập bằng chữ.
 *   · KHÔNG SỬA / THU HỒI / ĐÁNH GIÁ: các hành động ấy thuộc phiếu thật trên máy chủ.
 * Mọi ô nhập đi qua `o-nhap.tsx` — tệp duy nhất của nửa nhà nước được có ô nhập.
 */
import { useMemo, useState } from "react";

import { thoiDiemVN } from "../../lib/thoi-diem";

import { BieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";
import { XA_TN } from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import {
  kiemNhapPhieu,
  type LoiNhapPhieu,
  maPhieuTraiNghiem,
  NHAN_TRANG_THAI_TN,
  type NhapPhieu,
  type PhieuTraiNghiem,
  taoPhieuTraiNghiem,
  TOI_DA_DIA_CHI,
  TOI_DA_NOI_DUNG,
  TOI_DA_TIEU_DE,
  type TrangThaiTraiNghiem,
} from "./trai-nghiem";

/** Nhãn "BẢN TRẢI NGHIỆM" — chữ, không chỉ màu. */
export function NhanTraiNghiem() {
  return <span className="xa-nhan-tn">{XA_TN.nhan_trai_nghiem}</span>;
}

/** Khối ghi chú màu cam của bản trải nghiệm. */
export function GhiChuTraiNghiem({ cau }: { cau: string }) {
  return (
    <div className="xa-the xa-the--dem xa-ghi-chu xa-ghi-chu--cam">
      <BieuTuong ten="alert" co={22} />
      <p>{cau}</p>
    </div>
  );
}

export function ChipTrangThai({ tt }: { tt: TrangThaiTraiNghiem }) {
  return <span className={`xa-chip-tt xa-chip-tt--${tt}`}>{NHAN_TRANG_THAI_TN[tt]}</span>;
}

/** Một thẻ phiếu — dùng cả ở trang chủ ("phiếu mới nhất") và danh sách. */
export function ThePhieuTN({ phieu, onMo }: { phieu: PhieuTraiNghiem; onMo: () => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={onMo}>
      <span className="xa-o-bt xa-mau--hong" aria-hidden="true">
        <BieuTuong ten="chat" co={24} />
      </span>
      <span className="xa-hang-tin__chu">
        <span className="xa-phu">{phieu.ma}</span>
        <strong className="xa-hang-tin__tieu-de">{phieu.tieu_de}</strong>
        <span className="xa-phu">
          {phieu.luc_gui} · {phieu.dia_chi}
        </span>
        <span>
          <ChipTrangThai tt={phieu.trang_thai} />
        </span>
      </span>
      <BieuTuong ten="right" co={20} />
    </button>
  );
}

/* ═══════════════════════════════ DANH SÁCH ═══════════════════════════════ */

type Loc = TrangThaiTraiNghiem | "tat-ca";
const THU_TU: readonly TrangThaiTraiNghiem[] = ["moi", "dang-xu-ly", "da-xu-ly"];

export function DanhSachPhieuTN({ phieu, onMo }: { phieu: readonly PhieuTraiNghiem[]; onMo: (ma: string) => void }) {
  const [loc, datLoc] = useState<Loc>("tat-ca");
  const hien = useMemo(() => (loc === "tat-ca" ? phieu : phieu.filter((p) => p.trang_thai === loc)), [phieu, loc]);
  const dem = (tt: TrangThaiTraiNghiem) => phieu.filter((p) => p.trang_thai === tt).length;

  return (
    <>
      <GhiChuTraiNghiem cau={XA_TN.ghi_chu_phieu_tn} />
      <div className="xa-chips" role="group" aria-label={XA_TN.loc_tat_ca}>
        <button type="button" className={`xa-chip${loc === "tat-ca" ? " xa-chip--on" : ""}`} aria-pressed={loc === "tat-ca"} onClick={() => datLoc("tat-ca")}>
          {XA_TN.loc_tat_ca} ({phieu.length})
        </button>
        {THU_TU.map((tt) => (
          <button key={tt} type="button" className={`xa-chip${loc === tt ? " xa-chip--on" : ""}`} aria-pressed={loc === tt} onClick={() => datLoc(tt)}>
            {NHAN_TRANG_THAI_TN[tt]} ({dem(tt)})
          </button>
        ))}
      </div>
      {hien.length === 0 ? (
        <KhoiTrangThai bieu_tuong="chat" cau={phieu.length === 0 ? XA_TN.chua_co_phieu : XA_TN.loc_trong} />
      ) : (
        <ul className="xa-ds">
          {hien.map((p) => (
            <li key={p.ma}>
              <ThePhieuTN phieu={p} onMo={() => onMo(p.ma)} />
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

/* ═══════════════════════════════ CHI TIẾT ═══════════════════════════════ */

export function ChiTietPhieuTN({ phieu, onQuayLai }: { phieu: PhieuTraiNghiem | null; onQuayLai: () => void }) {
  return (
    <>
      <DauManCon tieu_de={XA_TN.chi_tiet_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        {phieu === null ? (
          <KhoiTrangThai bieu_tuong="chat" loi cau={XA_TN.loc_trong} />
        ) : (
          <>
            <GhiChuTraiNghiem cau={XA_TN.ghi_chu_phieu_tn} />
            <div className="xa-the xa-the--dem xa-khoi">
              <p className="xa-phu">
                {XA_TN.ma_phieu}: <strong>{phieu.ma}</strong>
              </p>
              <h2 className="xa-bai__tieu-de">{phieu.tieu_de}</h2>
              <ChipTrangThai tt={phieu.trang_thai} />
              <div className="xa-ke" />
              <p className="xa-nhan-o">{XA_TN.mo_ta}</p>
              <p className="xa-giu-dong">{phieu.noi_dung}</p>
              <p className="xa-nhan-o">{XA_TN.noi_xay_ra}</p>
              <p>{phieu.dia_chi}</p>
              <p className="xa-nhan-o">{XA_TN.gui_luc}</p>
              <p>{phieu.luc_gui}</p>
            </div>
            <div className="xa-the xa-the--dem xa-khoi">
              <h2 className="xa-dau-khoi__tieu-de">{XA_TN.tien_trinh}</h2>
              <ol className="xa-dong-tg">
                <li className="xa-dong-tg__buoc">
                  <strong>{XA_TN.buoc_da_gui}</strong>
                  <span className="xa-phu">{phieu.luc_gui}</span>
                </li>
                <li className="xa-dong-tg__buoc xa-dong-tg__buoc--dang">
                  <strong>{XA_TN.buoc_cho_tiep_nhan}</strong>
                </li>
              </ol>
            </div>
          </>
        )}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ GỬI — 2 BƯỚC ═══════════════════════════════ */

function ThanhBuoc({ buoc }: { buoc: 1 | 2 }) {
  const nhan = [XA_TN.buoc_noi_dung, XA_TN.buoc_xac_nhan];
  return (
    <ol className="xa-thanh-buoc" aria-label={XA_TN.buoc(buoc, 2)}>
      {nhan.map((n, i) => {
        const so = i + 1;
        const tt = so < buoc ? "xong" : so === buoc ? "dang" : "cho";
        return (
          <li key={n} className={`xa-thanh-buoc__muc xa-thanh-buoc__muc--${tt}`} aria-current={tt === "dang" ? "step" : undefined}>
            <span className="xa-thanh-buoc__cham">{tt === "xong" ? <BieuTuong ten="check" co={16} /> : so}</span>
            <span>{n}</span>
          </li>
        );
      })}
    </ol>
  );
}

export function GuiPhanAnhTN(props: {
  onQuayLai: () => void;
  onDaGui: (phieu: PhieuTraiNghiem) => void;
  onXemPhieu: (ma: string) => void;
}) {
  const [buoc, datBuoc] = useState<1 | 2>(1);
  const [nhap, datNhap] = useState<NhapPhieu>({ tieu_de: "", noi_dung: "", dia_chi: "" });
  const [loi, datLoi] = useState<LoiNhapPhieu>({});
  const [xong, datXong] = useState<PhieuTraiNghiem | null>(null);
  const doi = (k: keyof NhapPhieu) => (v: string) => datNhap((t) => ({ ...t, [k]: v }));

  if (xong !== null) {
    return (
      <>
        <DauManCon tieu_de={XA_TN.nut_gui} onQuayLai={props.onQuayLai} />
        <TrangCon>
          <div className="xa-ket-qua">
            <span className="xa-ket-qua__dau" aria-hidden="true">
              <BieuTuong ten="check" co={46} />
            </span>
            <h2 className="xa-bai__tieu-de">{XA_TN.xong_tieu_de}</h2>
            <p className="xa-phu">{XA_TN.xong_mo_ta}</p>
            <div className="xa-the xa-the--dem xa-ket-qua__ma">
              <p className="xa-phu">{XA_TN.ma_phieu}</p>
              <strong>{xong.ma}</strong>
            </div>
            <button type="button" className="xa-nut xa-nut--hong" onClick={() => props.onXemPhieu(xong.ma)}>
              {XA_TN.nut_theo_doi}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onQuayLai}>
              {XA_TN.nut_ve_trang_chu}
            </button>
          </div>
        </TrangCon>
      </>
    );
  }

  function tiep() {
    const l = kiemNhapPhieu(nhap);
    datLoi(l);
    if (Object.keys(l).length === 0) datBuoc(2);
  }

  function gui() {
    const luc = thoiDiemVN(new Date().toISOString()) ?? "";
    const phieu = taoPhieuTraiNghiem(nhap, luc, maPhieuTraiNghiem());
    props.onDaGui(phieu);
    datXong(phieu);
  }

  return (
    <>
      <DauManCon tieu_de={XA_TN.nut_gui} onQuayLai={buoc === 2 ? () => datBuoc(1) : props.onQuayLai} />
      <ThanhBuoc buoc={buoc} />
      <TrangCon>
        <GhiChuTraiNghiem cau={XA_TN.ghi_chu_gui_tn} />
        {buoc === 1 ? (
          <div className="xa-the xa-the--dem xa-khoi">
            <ONhapDong id="xa-tieu-de" nhan={XA_TN.o_tieu_de} goi_y={XA_TN.goi_y_tieu_de} gia_tri={nhap.tieu_de} toi_da={TOI_DA_TIEU_DE} onDoi={doi("tieu_de")} />
            {loi.tieu_de && <p className="xa-loi-o" role="alert">{loi.tieu_de}</p>}
            <ONhapDoan id="xa-noi-dung" nhan={XA_TN.o_noi_dung} goi_y={XA_TN.goi_y_noi_dung} gia_tri={nhap.noi_dung} toi_da={TOI_DA_NOI_DUNG} bat_buoc onDoi={doi("noi_dung")} />
            {loi.noi_dung && <p className="xa-loi-o" role="alert">{loi.noi_dung}</p>}
            <ONhapDong id="xa-dia-chi" nhan={XA_TN.o_dia_chi} goi_y={XA_TN.goi_y_dia_chi} gia_tri={nhap.dia_chi} toi_da={TOI_DA_DIA_CHI} onDoi={doi("dia_chi")} />
            {loi.dia_chi && <p className="xa-loi-o" role="alert">{loi.dia_chi}</p>}
            <button type="button" className="xa-nut xa-nut--hong" onClick={tiep}>
              {XA_TN.nut_tiep}
            </button>
          </div>
        ) : (
          <div className="xa-the xa-the--dem xa-khoi">
            <h2 className="xa-dau-khoi__tieu-de">{XA_TN.kiem_tra_lai}</h2>
            <p className="xa-nhan-o">{XA_TN.o_tieu_de}</p>
            <p>
              <strong>{nhap.tieu_de.trim()}</strong>
            </p>
            <p className="xa-nhan-o">{XA_TN.o_noi_dung}</p>
            <p className="xa-giu-dong">{nhap.noi_dung.trim()}</p>
            <p className="xa-nhan-o">{XA_TN.o_dia_chi}</p>
            <p>{nhap.dia_chi.trim()}</p>
            <button type="button" className="xa-nut xa-nut--hong" onClick={gui}>
              <BieuTuong ten="send" co={20} />
              {XA_TN.nut_gui}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={() => datBuoc(1)}>
              {XA_TN.nut_lui}
            </button>
          </div>
        )}
      </TrangCon>
    </>
  );
}
