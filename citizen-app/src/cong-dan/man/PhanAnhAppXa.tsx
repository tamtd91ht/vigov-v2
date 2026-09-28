/**
 * PHẢN ÁNH TRONG APP RIÊNG — theo ADR 0050: chỗ xung đột theo kho yêu cầu, còn lại theo prototype khách
 * (`../vigov-require/apps/miniapp` — `NewFeedbackPage`, `FeedbackDetailPage`, `StatusChip`, `RatingBlock`).
 *
 *   · Gửi: Lĩnh vực → Mô tả (gửi ngay ở đây) → Đã gửi. Lĩnh vực dân chọn LÀ lĩnh vực của phiếu (ADR 0050
 *     #1); máy chủ đặt hạn từ nó — Mini App không tự tính hạn (luật 10 cấm #2).
 *   · Công tắc "Gửi ẩn danh" (#3, SRS M4.2): bật thì ẩn ô tên và số, phiếu không giữ hai ô ấy; tắt thì họ
 *     tên bắt buộc (`kiemNhapPhieu`). Tên xin quyền Zalo (`NutLayTen`).
 *   · Người dân thấy BỐN nhóm trạng thái; dòng thời gian chỉ các bước ĐÃ QUA, nhãn của prototype (#5).
 *   · Chấm 1–5 sao khi phiếu đã xử lý xong; 1–2 sao mở lại phiếu (#2).
 *   · Xưng "bà con" (#6).
 *   · Nháp đang soạn giữ trên máy (#7, chủ dự án 28/09/2026) — qua `draftStore` lớp vỏ tiêm; tệp này không
 *     chạm kho lưu trữ nào. Mở màn gửi mà có nháp thì hỏi "Tiếp tục" / "Bỏ nháp"; gửi xong hoặc huỷ là xoá.
 *
 * Tên xã hiện ngay trên nút gửi — người dân đọc lại nơi nhận ở bước cuối (`skills/zalo-miniapp-multi-
 * tenant` REQUIRED #5), dù prototype không có bước xác nhận riêng.
 *
 * PHIẾU CHỈ TRONG MÁY (`trai-nghiem.ts`): không một byte nào tới máy chủ của xã. Mọi màn gắn nhãn.
 * Mọi ô nhập đi qua `o-nhap.tsx` — tệp duy nhất của nửa nhà nước được có ô nhập.
 */
import { type ReactNode, useEffect, useMemo, useState } from "react";

import { DO_DAI_TOI_DA, RATING_COMMENT_MAX_LEN, type SceneLocation } from "../api/hop-dong-phan-anh";
import { thoiDiemVN } from "../../lib/thoi-diem";

import { BieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";
import { GUI, giaiThichTrangThai, KHAN_CAP, nhanTrangThai, THE_PHIEU, TRA_CUU, XA_PA, XA_TN } from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import {
  type GetSceneLocation,
  SceneLocationControl,
  type SceneLocationWords,
  useSceneLocation,
} from "./scene-location";
import { StarPicker } from "./star-picker";
import {
  duocDanhGia,
  type FeedbackDraftStore,
  type KetQuaLayTen,
  kiemNhapPhieu,
  LINH_VUC_TAM,
  type LayTenZalo,
  type LoiNhapPhieu,
  maPhieuTraiNghiem,
  NHAN_BUOC,
  NHAN_NHOM,
  nhomCua,
  type NhomLoc,
  type NhapPhieu,
  type PhieuTN,
  taoPhieuTraiNghiem,
  traPhieuTraiNghiem,
  STAFF_CONDUCT_FIELD,
  VONG_DOI,
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

/**
 * Nhãn một trong BỐN nhóm người dân thấy (ADR 0050 #5). An unknown code gets the shared app's neutral
 * sentence (`nhanTrangThai`) and a neutral style — never a guessed group, never the raw code.
 */
export function ChipTrangThai({ tt }: { tt: string }) {
  const nhom = nhomCua(tt);
  if (nhom === null) return <span className="xa-chip-tt xa-chip-tt--unknown">{nhanTrangThai(tt)}</span>;
  return <span className={`xa-chip-tt xa-chip-tt--${nhom}`}>{NHAN_NHOM[nhom]}</span>;
}

const gio = (iso: string) => thoiDiemVN(iso) ?? "";

/** Một thẻ phiếu — dùng cả ở trang chủ và danh sách. */
export function ThePhieuTN({ phieu, onMo }: { phieu: PhieuTN; onMo: () => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={onMo}>
      <span className="xa-o-bt xa-mau--hong" aria-hidden="true">
        <BieuTuong ten="chat" co={24} />
      </span>
      <span className="xa-hang-tin__chu">
        <span className="xa-phu">
          #{phieu.ma_tra_cuu} · {phieu.nhan_linh_vuc}
        </span>
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

const NHOM: readonly NhomLoc[] = ["tat-ca", "da-tiep-nhan", "dang-xu-ly", "da-xu-ly-xong", "da-dong"];
const nhanLoc = (n: NhomLoc) => (n === "tat-ca" ? XA_TN.loc_tat_ca : NHAN_NHOM[n]);

export function DanhSachPhieuTN(props: { phieu: readonly PhieuTN[]; onMo: (ma: string) => void; onTraCuu: () => void }) {
  const { phieu } = props;
  const [loc, datLoc] = useState<NhomLoc>("tat-ca");
  const hien = useMemo(() => (loc === "tat-ca" ? phieu : phieu.filter((p) => nhomCua(p.trang_thai) === loc)), [phieu, loc]);
  const dem = (n: NhomLoc) => (n === "tat-ca" ? phieu.length : phieu.filter((p) => nhomCua(p.trang_thai) === n).length);

  return (
    <>
      <GhiChuTraiNghiem cau={XA_PA.ghi_chu_phieu} />
      <button type="button" className="xa-the xa-hang xa-hang--vien" onClick={props.onTraCuu}>
        <span className="xa-o-bt xa-mau--xanh" aria-hidden="true">
          <BieuTuong ten="search" co={24} />
        </span>
        <span className="xa-hang__chu">
          <strong>{XA_PA.tra_cuu_tieu_de}</strong>
          <span className="xa-phu">{XA_PA.tra_cuu_goi_y}</span>
        </span>
        <BieuTuong ten="right" co={20} />
      </button>
      <div className="xa-chips" role="group" aria-label={XA_TN.loc_nhom}>
        {NHOM.map((k) => (
          <button
            key={k}
            type="button"
            className={`xa-chip${loc === k ? " xa-chip--on" : ""}`}
            aria-pressed={loc === k}
            onClick={() => datLoc(k)}
          >
            {nhanLoc(k)} ({dem(k)})
          </button>
        ))}
      </div>
      {hien.length === 0 ? (
        <KhoiTrangThai bieu_tuong="chat" cau={phieu.length === 0 ? XA_PA.chua_co_phieu : XA_TN.loc_trong} />
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

/**
 * Dòng thời gian — CHỈ các bước đã qua (prototype: sự kiện đã xảy ra, `feedback-adapter.ts:128-137`), nhãn
 * của prototype. Hai nhánh kết thúc dừng sau "Đang phân loại". THUẦN, xuất để test.
 */
export function buocDaQua(tt: string): string[] {
  if (tt === "khong-tiep-nhan" || tt === "chuyen-cap-tren") return ["da-tiep-nhan", "dang-phan-loai", tt];
  const i = VONG_DOI.indexOf(tt);
  return i < 0 ? [tt] : VONG_DOI.slice(0, i + 1);
}

export function DongThoiGian({ phieu }: { phieu: PhieuTN }) {
  const buoc = buocDaQua(phieu.trang_thai);
  return (
    <ol className="xa-dong-tg">
      {buoc.map((tt, i) => {
        const cuoi = i === buoc.length - 1;
        return (
          <li
            key={tt}
            className={`xa-dong-tg__buoc ${cuoi ? "xa-dong-tg__buoc--dang" : "xa-dong-tg__buoc--qua"}`}
            aria-current={cuoi ? "step" : undefined}
          >
            <strong>{NHAN_BUOC[tt] ?? tt}</strong>
            <span className="xa-phu">{XA_PA.don_vi_xu_ly}</span>
            {i === 0 && <span className="xa-phu">{gio(phieu.goc_dem_han)}</span>}
            {cuoi && giaiThichTrangThai(tt) && <span className="xa-phu">{giaiThichTrangThai(tt)}</span>}
          </li>
        );
      })}
      {phieu.so_lan_mo_lai > 0 && (
        <li className="xa-dong-tg__buoc xa-dong-tg__buoc--dang">
          <strong>{XA_PA.da_mo_lai(phieu.so_lan_mo_lai)}</strong>
        </li>
      )}
    </ol>
  );
}

/**
 * Đánh giá — theo prototype (`RatingBlock.tsx`): năm sao, mỗi sao một vùng chạm riêng, nhãn bằng chữ, nhận
 * xét tuỳ ý. Chỉ đọc khi đã chấm. Hàng sao là `StarPicker`, dùng chung với khối đánh giá thật
 * (`PetitionRating.tsx`) — một bản, không hai.
 */
export function KhoiDanhGia(props: { phieu: PhieuTN; onGui?: (sao: number, nhan_xet: string) => void }) {
  // Có `onGui` là đang chấm (lần đầu, hoặc chấm lại sau khi mở lại) — ô trống, không điền sẵn lần cũ.
  const chi_doc = props.onGui === undefined;
  const da = chi_doc ? props.phieu.danh_gia : null;
  const [sao, datSao] = useState(da?.sao ?? 0);
  const [nhan_xet, datNhanXet] = useState(da?.nhan_xet ?? "");
  return (
    <section className="xa-the xa-the--dem xa-khoi">
      <h2 className="xa-dau-khoi__tieu-de">{chi_doc ? XA_PA.da_danh_gia : XA_PA.danh_gia_tieu_de}</h2>
      {!chi_doc && <p className="xa-phu">{XA_PA.danh_gia_vi_sao}</p>}
      <StarPicker stars={sao} onPick={chi_doc ? undefined : datSao} />
      {chi_doc ? (
        da?.nhan_xet ? <p className="xa-giu-dong">“{da.nhan_xet}”</p> : null
      ) : (
        <>
          <ONhapDoan id="xa-nhan-xet" nhan={XA_PA.nhan_xet} gia_tri={nhan_xet} toi_da={RATING_COMMENT_MAX_LEN} onDoi={datNhanXet} />
          <button type="button" className="xa-nut" disabled={sao === 0} onClick={() => props.onGui?.(sao, nhan_xet)}>
            {XA_PA.gui_danh_gia}
          </button>
        </>
      )}
    </section>
  );
}

export function ThanPhieuTN({ phieu, onDanhGia }: { phieu: PhieuTN; onDanhGia?: (sao: number, nhan_xet: string) => void }) {
  const nguoi_gui = [phieu.ho_ten_da_che || XA_PA.giau_ten, phieu.dien_thoai_da_che].filter(Boolean).join(" · ");
  return (
    <>
      <div className="xa-the xa-the--dem xa-khoi">
        <p className="xa-phu">
          {XA_PA.ma_phieu}: <strong className="xa-ma">#{phieu.ma_tra_cuu}</strong>
        </p>
        <Dong nhan={THE_PHIEU.trang_thai}>
          <ChipTrangThai tt={phieu.trang_thai} />
        </Dong>
        <Dong nhan={THE_PHIEU.linh_vuc}>{phieu.nhan_linh_vuc}</Dong>
        <Dong nhan={THE_PHIEU.gui_luc}>{gio(phieu.goc_dem_han)}</Dong>
        <Dong nhan={XA_PA.du_kien_xong}>{phieu.han_xu_ly_xong ? gio(phieu.han_xu_ly_xong) : XA_TN.chua_tinh_han}</Dong>
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
          <Dong nhan={THE_PHIEU.ly_do_khong_tiep_nhan}>{phieu.ly_do || XA_PA.chua_ghi}</Dong>
        )}
        {phieu.trang_thai === "chuyen-cap-tren" && (
          <>
            <Dong nhan={THE_PHIEU.co_quan_tiep_nhan}>{phieu.co_quan_nhan || XA_PA.chua_ghi}</Dong>
            <Dong nhan={THE_PHIEU.ly_do_chuyen}>{phieu.ly_do || XA_PA.chua_ghi}</Dong>
          </>
        )}
      </div>
      <div className="xa-the xa-the--dem xa-khoi">
        <h2 className="xa-dau-khoi__tieu-de">{XA_PA.tien_trinh}</h2>
        <DongThoiGian phieu={phieu} />
      </div>
      {duocDanhGia(phieu) && onDanhGia ? (
        <KhoiDanhGia key={`cham-${phieu.so_lan_mo_lai}`} phieu={phieu} onGui={onDanhGia} />
      ) : (
        phieu.danh_gia !== null && <KhoiDanhGia phieu={phieu} />
      )}
    </>
  );
}

export function ChiTietPhieuTN(props: {
  phieu: PhieuTN | null;
  onQuayLai: () => void;
  onDanhGia?: (ma: string, sao: number, nhan_xet: string) => void;
}) {
  const { phieu } = props;
  return (
    <>
      <DauManCon tieu_de={XA_PA.chi_tiet_tieu_de} onQuayLai={props.onQuayLai} />
      <TrangCon>
        {phieu === null ? (
          <KhoiTrangThai bieu_tuong="chat" loi cau={XA_PA.khong_thay_phieu} />
        ) : (
          <>
            <GhiChuTraiNghiem cau={XA_PA.ghi_chu_phieu} />
            <ThanPhieuTN
              phieu={phieu}
              onDanhGia={props.onDanhGia ? (sao, nx) => props.onDanhGia!(phieu.ma_tra_cuu, sao, nx) : undefined}
            />
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
    const p = traPhieuTraiNghiem(phieu, ma.trim().replace(/^#/, ""));
    datKq(p === null ? { kieu: "khong-thay" } : { kieu: "thay", p });
  }
  return (
    <>
      <DauManCon tieu_de={XA_PA.tra_cuu_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <GhiChuTraiNghiem cau={XA_TN.ghi_chu_tra_cuu_tn} />
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong id="xa-ma-tra-cuu" nhan={XA_PA.ma_phieu} goi_y={XA_PA.tra_cuu_goi_y} gia_tri={ma} toi_da={40} onDoi={datMa} />
          <button type="button" className="xa-nut" onClick={tra}>
            <BieuTuong ten="search" co={20} />
            {TRA_CUU.nut_tra}
          </button>
        </div>
        {kq?.kieu === "thieu" && <KhoiTrangThai bieu_tuong="info" loi cau={XA_PA.thieu_ma} />}
        {kq?.kieu === "khong-thay" && <KhoiTrangThai bieu_tuong="search" loi cau={XA_PA.khong_thay_phieu} />}
        {kq?.kieu === "thay" && <ThanPhieuTN phieu={kq.p} />}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ GỬI — Lĩnh vực → Mô tả → Đã gửi ═══════════════════════════════ */

function ThanhBuoc({ buoc }: { buoc: 1 | 2 | 3 }) {
  const nhan = [XA_TN.buoc_linh_vuc, XA_PA.buoc_mo_ta, XA_PA.buoc_xong];
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

const CAU_TEN: Readonly<Record<Exclude<KetQuaLayTen["kieu"], "xong">, string>> = {
  "tu-choi": XA_TN.ten_tu_choi,
  "ngoai-zalo": XA_TN.ten_ngoai_zalo,
  "khong-lay-duoc": XA_TN.ten_khong_lay_duoc,
};

/**
 * Nút "Lấy họ tên từ Zalo" — HÀNH ĐỘNG XIN QUYỀN DUY NHẤT (`LayTenZalo`). Zalo tự bật hộp hỏi; bà con từ
 * chối thì vẫn tự gõ được, và câu hiện ra nói đúng điều ấy.
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

/** "Bà con" words for the shared location control (`scene-location.tsx`), from `XA_TN` / `XA_PA`. */
export const COMMUNE_LOCATION_WORDS: SceneLocationWords = {
  button: XA_TN.vi_tri_nut,
  button_again: XA_TN.location_again,
  locating: XA_TN.vi_tri_dang_lay,
  why: XA_PA.vi_tri_vi_sao,
  found: XA_TN.location_found,
  failures: {
    "tu-choi": XA_TN.vi_tri_tu_choi,
    "ngoai-zalo": XA_TN.vi_tri_ngoai_zalo,
    "qua-nhieu-lan": XA_TN.location_rate_limited,
    "thu-lai": XA_TN.vi_tri_khong_lay_duoc,
    "tam-ngung": XA_TN.location_unavailable,
  },
};

/**
 * "Lấy vị trí hiện tại" of the experience form — only when the shell injects the function (inside Zalo).
 *
 * THE COORDINATES STAY ON THIS SCREEN: the experience ticket is the real contract's `PhieuCuaToi`, which
 * carries no location, and the ticket never reaches the commune anyway (`trai-nghiem.ts`). They are
 * shown so the citizen sees the exchange worked, and they are NOT put in the draft on the phone (the
 * draft keeps the six `NhapPhieu` fields, like the prototype's `saveDraft`, which keeps no `coords`).
 */
function CommuneLocation({ get }: { get: GetSceneLocation }) {
  const [location, setLocation] = useState<SceneLocation | null>(null);
  const { locating, failure, locate } = useSceneLocation(get, setLocation);
  return (
    <SceneLocationControl
      words={COMMUNE_LOCATION_WORDS}
      look="commune"
      locating={locating}
      location={location}
      failure={failure}
      onLocate={() => void locate()}
    />
  );
}

/**
 * "Tiếp tục" on a found draft → the form to show and the step to open. PURE, exported for tests.
 *
 * A field no longer offered (the catalogue changed) is not restored: the citizen picks again on step 1.
 * Otherwise step 2, the writing step, as the prototype does (`NewFeedbackPage.tsx:222`). An empty name in
 * the draft (an anonymous one keeps none) falls back to the name taken from Zalo in this session.
 */
export function restoreDraft(draft: NhapPhieu, zaloName: string | null): { form: NhapPhieu; step: 1 | 2 } {
  const field = LINH_VUC_TAM.includes(draft.linh_vuc) ? draft.linh_vuc : "";
  return {
    form: {
      linh_vuc: field,
      noi_dung: draft.noi_dung,
      dia_chi: draft.dia_chi,
      ho_ten: draft.ho_ten !== "" ? draft.ho_ten : (zaloName ?? ""),
      dien_thoai: draft.dien_thoai,
      an_danh: draft.an_danh,
    },
    step: field !== "" ? 2 : 1,
  };
}

export function GuiPhanAnhTN(props: {
  ten_xa: string;
  /** Họ tên đã lấy từ Zalo ở lần mở này, hoặc `null`. */
  ho_ten: string | null;
  lay_ten?: LayTenZalo;
  onTen: (ho_ten: string) => void;
  /** The location exchange, injected by the shell; absent = no location button (outside Zalo, tests). */
  getSceneLocation?: GetSceneLocation;
  /**
   * Draft kept on the phone (ADR 0050 #7) — injected by the shell for the commune's own app only. Absent:
   * no draft at all, nothing survives closing the app (shared app, tests).
   */
  draftStore?: FeedbackDraftStore;
  onQuayLai: () => void;
  onDaGui: (phieu: PhieuTN) => void;
  onXemPhieu: (ma: string) => void;
}) {
  const { draftStore } = props;
  const [buoc, datBuoc] = useState<1 | 2 | 3>(1);
  /**
   * A draft found when the screen opened, until the citizen picks "Tiếp tục" or "Bỏ nháp". While it is
   * pending the form is hidden and nothing is saved, so the old draft cannot be overwritten by a new one
   * before the citizen has answered (the prototype asks first, `NewFeedbackPage.tsx:79`).
   */
  const [draftOffer, setDraftOffer] = useState<NhapPhieu | null>(() => draftStore?.load() ?? null);
  const [nhap, datNhap] = useState<NhapPhieu>({
    linh_vuc: "",
    noi_dung: "",
    dia_chi: "",
    ho_ten: props.ho_ten ?? "",
    dien_thoai: "",
    // Gửi ẩn danh là tuỳ chọn của bà con (SRS M4.2, ADR 0050 #3): bật thì không gửi họ tên, số điện thoại.
    an_danh: false,
  });
  const [loi, datLoi] = useState<LoiNhapPhieu>({});
  const [xong, datXong] = useState<PhieuTN | null>(null);
  const [hoi_huy, datHoiHuy] = useState(false);
  const doi = (k: "noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai") => (v: string) => datNhap((t) => ({ ...t, [k]: v }));
  const co_noi_dung = nhap.linh_vuc !== "" || nhap.noi_dung.trim() !== "" || nhap.dia_chi.trim() !== "";

  // Save as the citizen types, on the writing step only (the prototype's rule, `NewFeedbackPage.tsx:124`).
  // Not while a found draft is still waiting for an answer, and not after sending (step 3).
  useEffect(() => {
    if (draftStore === undefined || draftOffer !== null || buoc !== 2 || nhap.linh_vuc === "") return;
    draftStore.save(nhap);
  }, [draftStore, draftOffer, buoc, nhap]);

  function resumeDraft() {
    if (draftOffer === null) return;
    const restored = restoreDraft(draftOffer, props.ho_ten);
    datNhap(restored.form);
    setDraftOffer(null);
    datBuoc(restored.step);
  }

  function discardDraft() {
    draftStore?.clear();
    setDraftOffer(null);
  }

  /** "Huỷ bỏ" in the cancel dialog: what was typed is dropped — on the phone too. */
  function cancelFeedback() {
    draftStore?.clear();
    props.onQuayLai();
  }

  function gui() {
    const l = kiemNhapPhieu(nhap, {
      thieu: XA_PA.thieu_mo_ta,
      thieu_nguoi_gui: XA_PA.thieu_nguoi_gui,
      qua_dai: (n) => GUI.qua_dai(XA_TN.o_nay, n),
    });
    datLoi(l);
    if (Object.keys(l).length > 0) return;
    const phieu = taoPhieuTraiNghiem(nhap, new Date().toISOString(), maPhieuTraiNghiem());
    draftStore?.clear();
    props.onDaGui(phieu);
    datXong(phieu);
    datBuoc(3);
  }

  function lui() {
    if (buoc === 2) return datBuoc(1);
    if (buoc === 1 && co_noi_dung) return datHoiHuy(true);
    props.onQuayLai();
  }

  return (
    <>
      <DauManCon tieu_de={GUI.tieu_de} onQuayLai={buoc === 3 ? props.onQuayLai : lui} />
      <ThanhBuoc buoc={buoc} />
      <TrangCon>
        {/* Kể cả màn "Đã lưu": bà con không được tưởng một phiếu chưa gửi đã tới xã. */}
        <GhiChuTraiNghiem cau={buoc === 3 ? XA_PA.ghi_chu_phieu : XA_PA.ghi_chu_gui} />
        {hoi_huy && (
          <div className="xa-the xa-the--dem xa-khoi" role="alertdialog" aria-label={XA_TN.hoi_huy_tieu_de}>
            <h2 className="xa-dau-khoi__tieu-de">{XA_TN.hoi_huy_tieu_de}</h2>
            <p>{XA_PA.hoi_huy_cau}</p>
            <button type="button" className="xa-nut" onClick={() => datHoiHuy(false)}>
              {XA_TN.tiep_tuc_nhap}
            </button>
            <button type="button" className="xa-nut xa-nut--phu xa-nut--do-vien" onClick={cancelFeedback}>
              {XA_TN.huy_bo}
            </button>
          </div>
        )}

        {draftOffer !== null && (
          <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-nhap-tieu-de">
            <h2 className="xa-dau-khoi__tieu-de" id="xa-nhap-tieu-de">
              {XA_PA.draft_title}
            </h2>
            <p>{XA_PA.draft_body}</p>
            <p className="xa-phu">{XA_PA.draft_kept_on_phone}</p>
            <button type="button" className="xa-nut" onClick={resumeDraft}>
              {XA_PA.draft_resume}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={discardDraft}>
              {XA_PA.draft_discard}
            </button>
          </section>
        )}

        {buoc === 1 && draftOffer === null && (
          <div className="xa-the xa-the--dem xa-khoi">
            <p>{XA_PA.chon_linh_vuc}</p>
            <div className="xa-luoi-lv" role="radiogroup" aria-label={XA_TN.buoc_linh_vuc}>
              {LINH_VUC_TAM.map((lv) => (
                <button
                  key={lv}
                  type="button"
                  role="radio"
                  aria-checked={nhap.linh_vuc === lv}
                  className={`xa-o-lv${nhap.linh_vuc === lv ? " xa-o-lv--on" : ""}`}
                  onClick={() => {
                    datNhap((t) => ({ ...t, linh_vuc: lv }));
                    datBuoc(2);
                  }}
                >
                  {lv}
                </button>
              ))}
            </div>
          </div>
        )}

        {buoc === 2 && (
          <div className="xa-the xa-the--dem xa-khoi">
            <ONhapDoan id="xa-noi-dung" nhan={XA_PA.su_viec} goi_y={XA_PA.goi_y_su_viec} gia_tri={nhap.noi_dung} toi_da={DO_DAI_TOI_DA.noi_dung} bat_buoc onDoi={doi("noi_dung")} />
            {loi.noi_dung && <p className="xa-loi-o" role="alert">{loi.noi_dung}</p>}
            <div className="xa-hang xa-hang--tinh xa-hang--sat xa-lv-dang">
              <span className="xa-hang__chu">
                <span>
                  {XA_PA.linh_vuc}: <strong>{nhap.linh_vuc}</strong>
                </span>
              </span>
              <button type="button" className="xa-dau-khoi__them" onClick={() => datBuoc(1)}>
                {XA_TN.doi}
              </button>
            </div>
            {nhap.linh_vuc === STAFF_CONDUCT_FIELD && (
              <div className="xa-ghi-chu">
                <BieuTuong ten="shield" co={22} />
                <p>{XA_PA.tac_phong_rieng}</p>
              </div>
            )}
            {/* SRS M4.2 bắt buộc ảnh/video và vị trí trên bản đồ. Ảnh CHƯA có; vị trí hiện tại lấy được
                (29/09/2026) nhưng chưa có bản đồ. Không chặn nút gửi vì hai ô ấy (xem `kiemNhapPhieu`). */}
            <p className="xa-nhan-o">{XA_PA.anh_bat_buoc}</p>
            <p className="xa-phu">{XA_PA.anh_sap_co}</p>
            <p className="xa-nhan-o">{XA_PA.vi_tri_bat_buoc}</p>
            <p className="xa-phu">{XA_PA.vi_tri_sap_co}</p>
            <ONhapDong id="xa-dia-chi" nhan={XA_PA.dia_chi} goi_y={XA_PA.goi_y_dia_chi} gia_tri={nhap.dia_chi} toi_da={DO_DAI_TOI_DA.dia_chi} onDoi={doi("dia_chi")} />
            {loi.dia_chi && <p className="xa-loi-o" role="alert">{loi.dia_chi}</p>}
            {props.getSceneLocation && <CommuneLocation get={props.getSceneLocation} />}
            <div className="xa-hang xa-hang--tinh xa-hang--sat">
              <span className="xa-hang__chu">
                <strong>{XA_PA.an_danh}</strong>
                <span className="xa-phu">{XA_PA.an_danh_giai_thich}</span>
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={nhap.an_danh}
                aria-label={XA_PA.an_danh}
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
                <ONhapDong id="xa-ho-ten" nhan={XA_PA.ten_nguoi_pa} goi_y={XA_PA.goi_y_ten} gia_tri={nhap.ho_ten} toi_da={DO_DAI_TOI_DA.ho_ten} onDoi={doi("ho_ten")} />
                {loi.ho_ten && <p className="xa-loi-o" role="alert">{loi.ho_ten}</p>}
                <ONhapDong id="xa-dien-thoai" nhan={XA_PA.so_dien_thoai} goi_y={XA_PA.goi_y_so} gia_tri={nhap.dien_thoai} toi_da={DO_DAI_TOI_DA.dien_thoai} kieu_ban_phim="tel" onDoi={doi("dien_thoai")} />
                {loi.dien_thoai && <p className="xa-loi-o" role="alert">{loi.dien_thoai}</p>}
              </>
            )}
            <p className="xa-phu">{nhap.an_danh ? XA_PA.bat_buoc_an_danh : XA_PA.bat_buoc}</p>
            {draftStore !== undefined && <p className="xa-phu">{XA_PA.draft_kept_on_phone}</p>}
            <div className="xa-ghi-chu">
              <BieuTuong ten="alert" co={22} />
              <p>{KHAN_CAP}</p>
            </div>
            <p className="xa-xa-nhan">{XA_PA.gui_toi(props.ten_xa)}</p>
            <button type="button" className="xa-nut xa-nut--hong" onClick={gui} disabled={nhap.noi_dung.trim() === ""}>
              <BieuTuong ten="send" co={20} />
              {GUI.tieu_de}
            </button>
          </div>
        )}

        {buoc === 3 && xong !== null && (
          <div className="xa-ket-qua">
            <span className="xa-ket-qua__dau" aria-hidden="true">
              <BieuTuong ten="check" co={46} />
            </span>
            <h2 className="xa-bai__tieu-de">{XA_PA.xong_tieu_de}</h2>
            <p className="xa-phu">{XA_PA.xong_mo_ta}</p>
            <div className="xa-the xa-the--dem xa-ket-qua__ma">
              <p className="xa-phu">{XA_PA.ma_phieu_cua_ba_con}</p>
              <strong>#{xong.ma_tra_cuu}</strong>
            </div>
            <p className="xa-phu">{XA_TN.chua_tinh_han}</p>
            <button type="button" className="xa-nut xa-nut--hong" onClick={() => props.onXemPhieu(xong.ma_tra_cuu)}>
              {XA_PA.theo_doi}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onQuayLai}>
              {XA_TN.nut_ve_trang_chu}
            </button>
          </div>
        )}
      </TrangCon>
    </>
  );
}
