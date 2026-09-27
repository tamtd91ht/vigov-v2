/**
 * PHẢN ÁNH TRONG APP RIÊNG — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/my-feedback/*`,
 * `features/send-feedback/*`), NỘI DUNG theo hợp đồng thật của `service-petitions`
 * (`api/hop-dong-phan-anh.ts`): năm ô gửi đi, bảng trạng thái `TRANG_THAI`, lĩnh vực do cán bộ chốt,
 * hai mốc hạn, kết quả, lý do không tiếp nhận, cơ quan nhận. Chữ dùng lại `GUI` · `TRA_CUU` · `THE_PHIEU`
 * — hai app nói cùng một chữ cho cùng một thứ.
 *
 * PHIẾU CHỈ TRONG MÁY (`trai-nghiem.ts`): không một byte nào tới máy chủ của xã. Mọi màn gắn nhãn.
 *
 * KHÁC BẢN MẪU, CÓ CHỦ ĐÍCH:
 *   · bước 1 "chọn lĩnh vực gần đúng nhất" như prototype — là GỢI Ý cho cán bộ; lĩnh vực của phiếu và hạn
 *     vẫn do cán bộ chốt (ADR 0049, sửa một phần ADR 0028); danh mục tạm `LINH_VUC_TAM`, không số giờ;
 *   · không "cam kết xử lý trong N ngày" viết cứng — hạn đếm bằng giờ làm việc theo lịch từng xã
 *     (ADR 0007), chỉ máy chủ đếm được; bản trải nghiệm nói thẳng là chưa tính hạn;
 *   · không ảnh hiện trường — chưa có kho tệp và đường tải lên;
 *   · vị trí: nút "Lấy vị trí hiện tại" xin quyền và nhận MÃ vị trí; toạ độ cần máy chủ đổi, chưa có,
 *     nên không vẽ bản đồ, không đoán địa chỉ;
 *   · không sửa / thu hồi / đánh giá — các hành động ấy cần tuyến máy chủ chưa có.
 * Mọi ô nhập đi qua `o-nhap.tsx` — tệp duy nhất của nửa nhà nước được có ô nhập.
 */
import { type ReactNode, useMemo, useState } from "react";

import { thoiDiemVN } from "../../lib/thoi-diem";

import { BieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";
import { GUI, giaiThichTrangThai, KHAN_CAP, nhanTrangThai, THE_PHIEU, TRA_CUU, TRANG_THAI, XA_TN } from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import {
  type KetQuaLayTen,
  type KetQuaViTri,
  kiemNhapPhieu,
  LINH_VUC_TAM,
  type LayMaViTri,
  type LoiNhapPhieu,
  type LayTenZalo,
  maPhieuTraiNghiem,
  nhomCua,
  type NhomLoc,
  type NhapPhieu,
  type PhieuTN,
  taoPhieuTraiNghiem,
  traPhieuTraiNghiem,
  VONG_DOI,
} from "./trai-nghiem";
import { DO_DAI_TOI_DA } from "../api/hop-dong-phan-anh";

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

function lopTrangThai(tt: string): string {
  const nhom = nhomCua(tt);
  if (tt === "khong-tiep-nhan" || tt === "chuyen-cap-tren") return "xa-chip-tt xa-chip-tt--ket-thuc";
  return `xa-chip-tt xa-chip-tt--${nhom}`;
}

export function ChipTrangThai({ tt }: { tt: string }) {
  return <span className={lopTrangThai(tt)}>{nhanTrangThai(tt)}</span>;
}

const gio = (iso: string) => thoiDiemVN(iso) ?? "";

/** Một thẻ phiếu — dùng cả ở trang chủ ("phiếu mới nhất") và danh sách. */
export function ThePhieuTN({ phieu, onMo }: { phieu: PhieuTN; onMo: () => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={onMo}>
      <span className="xa-o-bt xa-mau--hong" aria-hidden="true">
        <BieuTuong ten="chat" co={24} />
      </span>
      <span className="xa-hang-tin__chu">
        <span className="xa-phu">{phieu.ma_tra_cuu}</span>
        <strong className="xa-hang-tin__tieu-de xa-cat-2">{phieu.noi_dung}</strong>
        <span className="xa-phu">
          {gio(phieu.goc_dem_han)}
          {phieu.dia_chi !== "" ? ` · ${phieu.dia_chi}` : ""}
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

const NHOM: ReadonlyArray<[NhomLoc, string]> = [
  ["tat-ca", XA_TN.loc_tat_ca],
  ["dang-cho", XA_TN.loc_dang_cho],
  ["dang-xu-ly", XA_TN.loc_dang_xu_ly],
  ["da-xong", XA_TN.loc_da_xong],
];

export function DanhSachPhieuTN(props: {
  phieu: readonly PhieuTN[];
  onMo: (ma: string) => void;
  onTraCuu: () => void;
}) {
  const { phieu } = props;
  const [loc, datLoc] = useState<NhomLoc>("tat-ca");
  const hien = useMemo(() => (loc === "tat-ca" ? phieu : phieu.filter((p) => nhomCua(p.trang_thai) === loc)), [phieu, loc]);
  const dem = (n: NhomLoc) => (n === "tat-ca" ? phieu.length : phieu.filter((p) => nhomCua(p.trang_thai) === n).length);

  return (
    <>
      <GhiChuTraiNghiem cau={XA_TN.ghi_chu_phieu_tn} />
      <button type="button" className="xa-the xa-hang xa-hang--vien" onClick={props.onTraCuu}>
        <span className="xa-o-bt xa-mau--xanh" aria-hidden="true">
          <BieuTuong ten="search" co={24} />
        </span>
        <span className="xa-hang__chu">
          <strong>{TRA_CUU.tieu_de}</strong>
          <span className="xa-phu">{TRA_CUU.goi_y_ma}</span>
        </span>
        <BieuTuong ten="right" co={20} />
      </button>
      <div className="xa-chips" role="group" aria-label={XA_TN.loc_nhom}>
        {NHOM.map(([k, n]) => (
          <button
            key={k}
            type="button"
            className={`xa-chip${loc === k ? " xa-chip--on" : ""}`}
            aria-pressed={loc === k}
            onClick={() => datLoc(k)}
          >
            {n} ({dem(k)})
          </button>
        ))}
      </div>
      {hien.length === 0 ? (
        <KhoiTrangThai bieu_tuong="chat" cau={phieu.length === 0 ? XA_TN.chua_co_phieu : XA_TN.loc_trong} />
      ) : (
        <ul className="xa-ds">
          {hien.map((p) => (
            <li key={p.ma_tra_cuu}>
              <ThePhieuTN phieu={p} onMo={() => props.onMo(p.ma_tra_cuu)} />
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

/* ═══════════════════════════════ CHI TIẾT ═══════════════════════════════ */

function Dong({ nhan, children }: { nhan: string; children: ReactNode }) {
  return (
    <>
      <p className="xa-nhan-o">{nhan}</p>
      <div>{children}</div>
    </>
  );
}

/** Dòng thời gian theo vòng đời thật: bước đã qua, bước hiện tại, bước sắp tới. */
export function DongThoiGian({ phieu }: { phieu: PhieuTN }) {
  const ket_thuc = phieu.trang_thai === "khong-tiep-nhan" || phieu.trang_thai === "chuyen-cap-tren";
  const buoc = ket_thuc ? ["da-tiep-nhan", "dang-phan-loai", phieu.trang_thai] : VONG_DOI;
  const hien_tai = buoc.indexOf(phieu.trang_thai);
  return (
    <ol className="xa-dong-tg">
      {buoc.map((tt, i) => {
        const lop = i < hien_tai ? "xa-dong-tg__buoc--qua" : i === hien_tai ? "xa-dong-tg__buoc--dang" : "xa-dong-tg__buoc--cho";
        return (
          <li key={tt} className={`xa-dong-tg__buoc ${lop}`} aria-current={i === hien_tai ? "step" : undefined}>
            <strong>{TRANG_THAI[tt]?.nhan ?? nhanTrangThai(tt)}</strong>
            {i === 0 && <span className="xa-phu">{gio(phieu.goc_dem_han)}</span>}
            {i === hien_tai && giaiThichTrangThai(tt) && <span className="xa-phu">{giaiThichTrangThai(tt)}</span>}
          </li>
        );
      })}
    </ol>
  );
}

export function ThanPhieuTN({ phieu }: { phieu: PhieuTN }) {
  const nguoi_gui = phieu.an_danh
    ? THE_PHIEU.an_danh
    : [phieu.ho_ten_da_che || THE_PHIEU.khong_ghi_ten, phieu.dien_thoai_da_che].filter(Boolean).join(" · ");
  return (
    <>
      <div className="xa-the xa-the--dem xa-khoi">
        <p className="xa-phu">
          {GUI.xong_ma}: <strong className="xa-ma">{phieu.ma_tra_cuu}</strong>
        </p>
        <Dong nhan={THE_PHIEU.trang_thai}>
          <ChipTrangThai tt={phieu.trang_thai} />
        </Dong>
        {phieu.linh_vuc_goi_y !== "" && <Dong nhan={XA_TN.linh_vuc_ban_chon}>{phieu.linh_vuc_goi_y}</Dong>}
        <Dong nhan={THE_PHIEU.linh_vuc}>{phieu.nhan_linh_vuc || THE_PHIEU.chua_phan_loai}</Dong>
        <Dong nhan={THE_PHIEU.gui_luc}>
          {gio(phieu.goc_dem_han)} {THE_PHIEU.gio_vn}
        </Dong>
        <Dong nhan={THE_PHIEU.han_xem}>{phieu.han_tiep_nhan ? gio(phieu.han_tiep_nhan) : XA_TN.chua_tinh_han}</Dong>
        <Dong nhan={THE_PHIEU.han_xu_ly}>{phieu.han_xu_ly_xong ? gio(phieu.han_xu_ly_xong) : THE_PHIEU.han_xu_ly_chua_co}</Dong>
        <div className="xa-ke" />
        <Dong nhan={THE_PHIEU.noi_dung}>
          <p className="xa-giu-dong">{phieu.noi_dung}</p>
        </Dong>
        <Dong nhan={THE_PHIEU.dia_chi}>{phieu.dia_chi || THE_PHIEU.dia_chi_trong}</Dong>
        <Dong nhan={THE_PHIEU.nguoi_gui}>{nguoi_gui}</Dong>
        {phieu.ket_qua !== "" && (
          <Dong nhan={THE_PHIEU.ket_qua}>
            <p className="xa-giu-dong">{phieu.ket_qua}</p>
          </Dong>
        )}
        {phieu.trang_thai === "khong-tiep-nhan" && (
          <Dong nhan={THE_PHIEU.ly_do_khong_tiep_nhan}>{phieu.ly_do || THE_PHIEU.chua_ghi}</Dong>
        )}
        {phieu.trang_thai === "chuyen-cap-tren" && (
          <>
            <Dong nhan={THE_PHIEU.co_quan_tiep_nhan}>{phieu.co_quan_nhan || THE_PHIEU.chua_ghi}</Dong>
            <Dong nhan={THE_PHIEU.ly_do_chuyen}>{phieu.ly_do || THE_PHIEU.chua_ghi}</Dong>
            <p className="xa-phu">{THE_PHIEU.lien_he_co_quan}</p>
          </>
        )}
      </div>
      <div className="xa-the xa-the--dem xa-khoi">
        <h2 className="xa-dau-khoi__tieu-de">{XA_TN.tien_trinh}</h2>
        <DongThoiGian phieu={phieu} />
      </div>
    </>
  );
}

export function ChiTietPhieuTN({ phieu, onQuayLai }: { phieu: PhieuTN | null; onQuayLai: () => void }) {
  return (
    <>
      <DauManCon tieu_de={XA_TN.chi_tiet_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        {phieu === null ? (
          <KhoiTrangThai bieu_tuong="chat" loi cau={TRA_CUU.khong_thay} />
        ) : (
          <>
            <GhiChuTraiNghiem cau={XA_TN.ghi_chu_phieu_tn} />
            <ThanPhieuTN phieu={phieu} />
          </>
        )}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ TRA CỨU PHIẾU ═══════════════════════════════ */

export function TraCuuPhieuTN({ phieu, onQuayLai }: { phieu: readonly PhieuTN[]; onQuayLai: () => void }) {
  const [ma, datMa] = useState("");
  const [kq, datKq] = useState<{ kieu: "thieu" } | { kieu: "khong-thay" } | { kieu: "thay"; p: PhieuTN } | null>(null);
  function tra() {
    if (ma.trim() === "") return datKq({ kieu: "thieu" });
    const p = traPhieuTraiNghiem(phieu, ma);
    datKq(p === null ? { kieu: "khong-thay" } : { kieu: "thay", p });
  }
  return (
    <>
      <DauManCon tieu_de={TRA_CUU.tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <GhiChuTraiNghiem cau={XA_TN.ghi_chu_tra_cuu_tn} />
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong id="xa-ma-tra-cuu" nhan={TRA_CUU.nhan_ma} goi_y={TRA_CUU.goi_y_ma} gia_tri={ma} toi_da={40} onDoi={datMa} />
          <button type="button" className="xa-nut" onClick={tra}>
            <BieuTuong ten="search" co={20} />
            {TRA_CUU.nut_tra}
          </button>
        </div>
        {kq?.kieu === "thieu" && <KhoiTrangThai bieu_tuong="info" loi cau={TRA_CUU.thieu_ma} />}
        {kq?.kieu === "khong-thay" && <KhoiTrangThai bieu_tuong="search" loi cau={TRA_CUU.khong_thay} />}
        {kq?.kieu === "thay" && (
          <>
            <p className="xa-phu" role="status">
              {TRA_CUU.tim_thay}
            </p>
            <ThanPhieuTN phieu={kq.p} />
          </>
        )}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ GỬI — 2 BƯỚC ═══════════════════════════════ */

function ThanhBuoc({ buoc }: { buoc: 1 | 2 | 3 }) {
  const nhan = [XA_TN.buoc_linh_vuc, XA_TN.buoc_noi_dung, XA_TN.buoc_xac_nhan];
  return (
    <ol className="xa-thanh-buoc" aria-label={XA_TN.buoc(buoc, 3)}>
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

const CAU_VI_TRI: Readonly<Record<KetQuaViTri, string>> = {
  "da-nhan-ma": XA_TN.vi_tri_da_nhan,
  "tu-choi": XA_TN.vi_tri_tu_choi,
  "ngoai-zalo": XA_TN.vi_tri_ngoai_zalo,
  "khong-lay-duoc": XA_TN.vi_tri_khong_lay_duoc,
};

const CAU_TEN: Readonly<Record<Exclude<KetQuaLayTen["kieu"], "xong">, string>> = {
  "tu-choi": XA_TN.ten_tu_choi,
  "ngoai-zalo": XA_TN.ten_ngoai_zalo,
  "khong-lay-duoc": XA_TN.ten_khong_lay_duoc,
};

/**
 * Nút "Lấy họ tên từ Zalo" — HÀNH ĐỘNG XIN QUYỀN DUY NHẤT (`LayTenZalo`). Zalo tự bật hộp hỏi; người dân
 * từ chối thì vẫn tự gõ được, và câu hiện ra nói đúng điều ấy.
 */
export function NutLayTen({ lay, onTen }: { lay: LayTenZalo; onTen: (ho_ten: string) => void }) {
  const [dang, datDang] = useState(false);
  const [cau, datCau] = useState<{ loi: boolean; chu: string } | null>(null);
  async function bam() {
    if (dang) return;
    datDang(true);
    const kq = await lay().catch((): KetQuaLayTen => ({ kieu: "khong-lay-duoc" }));
    datDang(false);
    if (kq.kieu === "xong") {
      onTen(kq.ho_ten);
      datCau({ loi: false, chu: XA_TN.ten_da_lay });
    } else {
      datCau({ loi: true, chu: CAU_TEN[kq.kieu] });
    }
  }
  return (
    <div className="xa-vi-tri">
      <button type="button" className="xa-nut xa-nut--phu" onClick={() => void bam()} disabled={dang}>
        <BieuTuong ten="user" co={20} />
        {dang ? XA_TN.ten_dang : XA_TN.ten_nut}
      </button>
      <p className="xa-phu">{XA_TN.ten_vi_sao}</p>
      {cau !== null && (
        <p className={cau.loi ? "xa-loi-o" : "xa-phu"} role="status">
          {cau.chu}
        </p>
      )}
    </div>
  );
}

/** Nút "Lấy vị trí hiện tại". Chỉ có khi lớp vỏ tiêm hàm lấy mã vị trí (chạy trong Zalo). */
function NutViTri({ lay }: { lay: LayMaViTri }) {
  const [dang, datDang] = useState(false);
  const [kq, datKq] = useState<KetQuaViTri | null>(null);
  async function bam() {
    if (dang) return;
    datDang(true);
    datKq(await lay().catch(() => "khong-lay-duoc" as const));
    datDang(false);
  }
  return (
    <div className="xa-vi-tri">
      <button type="button" className="xa-nut xa-nut--phu" onClick={() => void bam()} disabled={dang}>
        <BieuTuong ten="pin" co={20} />
        {dang ? XA_TN.vi_tri_dang_lay : XA_TN.vi_tri_nut}
      </button>
      <p className="xa-phu">{XA_TN.vi_tri_vi_sao}</p>
      {kq !== null && (
        <p className={kq === "da-nhan-ma" ? "xa-phu" : "xa-loi-o"} role="status">
          {CAU_VI_TRI[kq]}
        </p>
      )}
    </div>
  );
}

export function GuiPhanAnhTN(props: {
  ten_xa: string;
  /** Họ tên đã lấy từ Zalo ở lần mở này, hoặc `null`. */
  ho_ten: string | null;
  lay_ten?: LayTenZalo;
  onTen: (ho_ten: string) => void;
  lay_ma_vi_tri?: LayMaViTri;
  onQuayLai: () => void;
  onDaGui: (phieu: PhieuTN) => void;
  onXemPhieu: (ma: string) => void;
}) {
  const [buoc, datBuoc] = useState<1 | 2 | 3>(1);
  const [nhap, datNhap] = useState<NhapPhieu>({
    linh_vuc_goi_y: "",
    noi_dung: "",
    dia_chi: "",
    ho_ten: props.ho_ten ?? "",
    dien_thoai: "",
    an_danh: false,
  });
  const [loi, datLoi] = useState<LoiNhapPhieu>({});
  const [xong, datXong] = useState<PhieuTN | null>(null);
  const [hoi_huy, datHoiHuy] = useState(false);
  const doi = (k: "noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai") => (v: string) => datNhap((t) => ({ ...t, [k]: v }));
  const co_noi_dung = nhap.linh_vuc_goi_y !== "" || nhap.noi_dung.trim() !== "" || nhap.dia_chi.trim() !== "";

  if (xong !== null) {
    return (
      <>
        <DauManCon tieu_de={GUI.tieu_de} onQuayLai={props.onQuayLai} />
        <TrangCon>
          <div className="xa-ket-qua">
            <span className="xa-ket-qua__dau" aria-hidden="true">
              <BieuTuong ten="check" co={46} />
            </span>
            <h2 className="xa-bai__tieu-de">{XA_TN.xong_tieu_de}</h2>
            <p className="xa-phu">{XA_TN.xong_mo_ta}</p>
            <div className="xa-the xa-the--dem xa-ket-qua__ma">
              <p className="xa-phu">{GUI.xong_ma}</p>
              <strong>{xong.ma_tra_cuu}</strong>
              <p className="xa-phu">{GUI.xong_giu_ma}</p>
            </div>
            <button type="button" className="xa-nut xa-nut--hong" onClick={() => props.onXemPhieu(xong.ma_tra_cuu)}>
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
    const l = kiemNhapPhieu(nhap, { thieu: GUI.thieu_noi_dung, qua_dai: (n) => GUI.qua_dai(XA_TN.o_nay, n) });
    datLoi(l);
    if (Object.keys(l).length === 0) datBuoc(3);
  }

  function gui() {
    const phieu = taoPhieuTraiNghiem(nhap, new Date().toISOString(), maPhieuTraiNghiem());
    props.onDaGui(phieu);
    datXong(phieu);
  }

  function lui() {
    if (buoc === 3) return datBuoc(2);
    if (buoc === 2) return datBuoc(1);
    if (co_noi_dung) return datHoiHuy(true);
    props.onQuayLai();
  }

  return (
    <>
      <DauManCon tieu_de={GUI.tieu_de} onQuayLai={lui} />
      <ThanhBuoc buoc={buoc} />
      <TrangCon>
        <GhiChuTraiNghiem cau={XA_TN.ghi_chu_gui_tn} />
        {hoi_huy && (
          <div className="xa-the xa-the--dem xa-khoi" role="alertdialog" aria-label={XA_TN.hoi_huy_tieu_de}>
            <h2 className="xa-dau-khoi__tieu-de">{XA_TN.hoi_huy_tieu_de}</h2>
            <p>{XA_TN.hoi_huy_cau}</p>
            <button type="button" className="xa-nut" onClick={() => datHoiHuy(false)}>
              {XA_TN.tiep_tuc_nhap}
            </button>
            <button type="button" className="xa-nut xa-nut--phu xa-nut--do-vien" onClick={props.onQuayLai}>
              {XA_TN.huy_bo}
            </button>
          </div>
        )}
        {buoc === 1 && (
          <div className="xa-the xa-the--dem xa-khoi">
            <p>{XA_TN.chon_linh_vuc}</p>
            <div className="xa-luoi-lv" role="radiogroup" aria-label={XA_TN.buoc_linh_vuc}>
              {LINH_VUC_TAM.map((lv) => (
                <button
                  key={lv}
                  type="button"
                  role="radio"
                  aria-checked={nhap.linh_vuc_goi_y === lv}
                  className={`xa-o-lv${nhap.linh_vuc_goi_y === lv ? " xa-o-lv--on" : ""}`}
                  onClick={() => {
                    datNhap((t) => ({ ...t, linh_vuc_goi_y: lv }));
                    datBuoc(2);
                  }}
                >
                  {lv}
                </button>
              ))}
            </div>
            <p className="xa-phu">{XA_TN.linh_vuc_la_goi_y}</p>
          </div>
        )}
        {buoc === 2 ? (
          <div className="xa-the xa-the--dem xa-khoi">
            <div className="xa-hang xa-hang--tinh xa-hang--sat">
              <span className="xa-hang__chu">
                <span className="xa-phu">{XA_TN.linh_vuc_ban_chon}</span>
                <strong>{nhap.linh_vuc_goi_y}</strong>
              </span>
              <button type="button" className="xa-dau-khoi__them" onClick={() => datBuoc(1)}>
                {XA_TN.doi}
              </button>
            </div>
            <div className="xa-ghi-chu">
              <BieuTuong ten="alert" co={22} />
              <p>{KHAN_CAP}</p>
            </div>
            <ONhapDoan id="xa-noi-dung" nhan={GUI.nhan_noi_dung} goi_y={GUI.goi_y_noi_dung} gia_tri={nhap.noi_dung} toi_da={DO_DAI_TOI_DA.noi_dung} bat_buoc onDoi={doi("noi_dung")} />
            <p className="xa-phu xa-dem-ky-tu">
              {[...nhap.noi_dung].length}/{DO_DAI_TOI_DA.noi_dung.toLocaleString("vi-VN")}
            </p>
            {loi.noi_dung && <p className="xa-loi-o" role="alert">{loi.noi_dung}</p>}
            <ONhapDong id="xa-dia-chi" nhan={GUI.nhan_dia_chi} goi_y={GUI.goi_y_dia_chi} gia_tri={nhap.dia_chi} toi_da={DO_DAI_TOI_DA.dia_chi} onDoi={doi("dia_chi")} />
            {loi.dia_chi && <p className="xa-loi-o" role="alert">{loi.dia_chi}</p>}
            {props.lay_ma_vi_tri && <NutViTri lay={props.lay_ma_vi_tri} />}
            <p className="xa-phu">{GUI.chua_ho_tro_anh}</p>
            <div className="xa-hang xa-hang--tinh xa-hang--sat">
              <span className="xa-hang__chu">
                <strong>{GUI.an_danh}</strong>
                <span className="xa-phu">{GUI.an_danh_giai_thich}</span>
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={nhap.an_danh}
                aria-label={GUI.an_danh}
                className={`xa-cong-tac${nhap.an_danh ? " xa-cong-tac--bat" : ""}`}
                onClick={() => datNhap((t) => ({ ...t, an_danh: !t.an_danh }))}
              >
                <span className="xa-cong-tac__nut" />
              </button>
            </div>
            {!nhap.an_danh && (
              <>
                {props.lay_ten && (
                  <NutLayTen
                    lay={props.lay_ten}
                    onTen={(t) => {
                      datNhap((x) => ({ ...x, ho_ten: t }));
                      props.onTen(t);
                    }}
                  />
                )}
                <ONhapDong id="xa-ho-ten" nhan={GUI.nhan_ho_ten} gia_tri={nhap.ho_ten} toi_da={DO_DAI_TOI_DA.ho_ten} onDoi={doi("ho_ten")} />
                {loi.ho_ten && <p className="xa-loi-o" role="alert">{loi.ho_ten}</p>}
                <p className="xa-phu">{XA_TN.so_tu_go}</p>
                <ONhapDong id="xa-dien-thoai" nhan={GUI.nhan_dien_thoai} gia_tri={nhap.dien_thoai} toi_da={DO_DAI_TOI_DA.dien_thoai} kieu_ban_phim="tel" onDoi={doi("dien_thoai")} />
                {loi.dien_thoai && <p className="xa-loi-o" role="alert">{loi.dien_thoai}</p>}
              </>
            )}
            <button type="button" className="xa-nut xa-nut--hong" onClick={tiep}>
              {GUI.nut_tiep}
            </button>
          </div>
        ) : buoc === 3 ? (
          <div className="xa-the xa-the--dem xa-khoi">
            <h2 className="xa-dau-khoi__tieu-de">{GUI.xac_nhan_tieu_de}</h2>
            <p>{GUI.xac_nhan_cau}</p>
            <p className="xa-xa-nhan">
              <strong>{props.ten_xa}</strong>
            </p>
            <p className="xa-phu">{GUI.xac_nhan_hau_qua}</p>
            <div className="xa-ke" />
            <Dong nhan={XA_TN.linh_vuc_ban_chon}>{nhap.linh_vuc_goi_y}</Dong>
            <Dong nhan={THE_PHIEU.noi_dung}>
              <p className="xa-giu-dong">{nhap.noi_dung.trim()}</p>
            </Dong>
            <Dong nhan={THE_PHIEU.dia_chi}>{nhap.dia_chi.trim() || THE_PHIEU.dia_chi_trong}</Dong>
            <Dong nhan={THE_PHIEU.nguoi_gui}>
              {nhap.an_danh ? THE_PHIEU.an_danh : [nhap.ho_ten.trim() || THE_PHIEU.khong_ghi_ten, nhap.dien_thoai.trim()].filter(Boolean).join(" · ")}
            </Dong>
            <button type="button" className="xa-nut xa-nut--hong" onClick={gui}>
              <BieuTuong ten="send" co={20} />
              {GUI.nut_gui(props.ten_xa)}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={() => datBuoc(2)}>
              {GUI.nut_sua}
            </button>
          </div>
        ) : null}
      </TrangCon>
    </>
  );
}
