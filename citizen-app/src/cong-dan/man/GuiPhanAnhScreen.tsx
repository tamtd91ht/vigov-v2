/**
 * MÀN "GỬI PHẢN ÁNH" — hành vi DUY NHẤT một công dân ghi vào hệ thống này (luật 10).
 *
 * BỐN BƯỚC, MỖI MÀN MỘT VIỆC (`skills/accessibility-elderly` #4):
 *
 *   nhập  →  xác nhận xã  →  đang gửi  →  mã tra cứu   (hoặc câu lỗi kèm việc cần làm)
 *
 * ⚠ BƯỚC XÁC NHẬN XÃ LÀ LUẬT NGHIỆP VỤ, KHÔNG PHẢI TRANG TRÍ (README §Non-negotiables #5): gửi nhầm
 * xã là xã ấy nhận việc ngoài địa bàn, phải chuyển hoặc từ chối, còn người dân chờ vô ích. Tên xã ở
 * bước ấy là `ten_xa` CỦA PHIÊN — đúng xã máy chủ sẽ ghi phiếu.
 *
 * ⚠ KHÔNG CÓ Ô CHỌN LĨNH VỰC: lĩnh vực do cán bộ chốt (ADR 0028, #23). Không có ô chọn xã: xã lấy
 * từ phiên (ADR 0022). Không có ảnh: chưa có kho lưu ảnh, và màn hình nói thẳng điều đó.
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG VẼ BIỂU MẪU và KHÔNG GỌI MẠNG — hôm nay là luôn luôn
 * (`api/phien-vigov.ts`).
 */
import { type ReactNode, useState } from "react";

import { guiPhanAnh, type KetQuaGoi } from "../api/goi-vigov";
import {
  DO_DAI_TOI_DA,
  type PhanAnhMoi,
  type PhieuCuaToi,
  thanGuiPhanAnh,
} from "../api/hop-dong-phan-anh";
import { type LanGui, taoLanGui } from "../api/lan-gui";
import { layPhienViGov } from "../api/phien-vigov";

import { BangXa, KenhChuaMo, ThePhieu } from "./khung";
import { GUI, KHAN_CAP, LOI_GUI, nhanTrangThai, QUAY_LAI } from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import { thoiDiemVN } from "./thoi-diem";

export const PHAN_ANH_TRONG: PhanAnhMoi = {
  noi_dung: "",
  dia_chi: "",
  ho_ten: "",
  dien_thoai: "",
  an_danh: false,
};

/** Câu cần sửa, hoặc `null` khi gửi được. Đếm theo KÝ TỰ như máy chủ, không theo byte. */
export function kiemPhanAnh(pa: PhanAnhMoi): string | null {
  const dai = (s: string) => [...s.trim()].length;
  if (dai(pa.noi_dung) === 0) return GUI.thieu_noi_dung;
  if (dai(pa.noi_dung) > DO_DAI_TOI_DA.noi_dung) return GUI.qua_dai("Nội dung", DO_DAI_TOI_DA.noi_dung);
  if (dai(pa.dia_chi) > DO_DAI_TOI_DA.dia_chi) return GUI.qua_dai("Nơi xảy ra", DO_DAI_TOI_DA.dia_chi);
  if (!pa.an_danh && dai(pa.ho_ten) > DO_DAI_TOI_DA.ho_ten) {
    return GUI.qua_dai("Họ và tên", DO_DAI_TOI_DA.ho_ten);
  }
  if (!pa.an_danh && dai(pa.dien_thoai) > DO_DAI_TOI_DA.dien_thoai) {
    return GUI.qua_dai("Số điện thoại", DO_DAI_TOI_DA.dien_thoai);
  }
  return null;
}

/* ─────────────────────────── bước 1: nhập ─────────────────────────── */

export function BuocNhap(props: {
  pa: PhanAnhMoi;
  loi: string | null;
  onDoi: (pa: PhanAnhMoi) => void;
  onTiep: () => void;
}) {
  const { pa, onDoi } = props;
  return (
    <div className="cd-buoc">
      <p className="cd-khan-cap" role="note">
        {KHAN_CAP}
      </p>

      <ONhapDoan
        id="cd-noi-dung"
        nhan={GUI.nhan_noi_dung}
        goi_y={GUI.goi_y_noi_dung}
        gia_tri={pa.noi_dung}
        toi_da={DO_DAI_TOI_DA.noi_dung}
        bat_buoc
        onDoi={(v) => onDoi({ ...pa, noi_dung: v })}
      />
      <ONhapDong
        id="cd-dia-chi"
        nhan={GUI.nhan_dia_chi}
        goi_y={GUI.goi_y_dia_chi}
        gia_tri={pa.dia_chi}
        toi_da={DO_DAI_TOI_DA.dia_chi}
        onDoi={(v) => onDoi({ ...pa, dia_chi: v })}
      />

      {/* NÚT BẬT/TẮT, KHÔNG PHẢI Ô ĐÁNH DẤU: trạng thái nói bằng CHỮ ("Đang bật"), không bằng một
          dấu tích nhỏ hay một màu (README §Non-negotiables #6), và đích chạm to bằng cả dòng. */}
      <button
        type="button"
        className="cd-cong-tac"
        aria-pressed={pa.an_danh}
        onClick={() => onDoi({ ...pa, an_danh: !pa.an_danh })}
      >
        <span className="cd-cong-tac__ten">{GUI.an_danh}</span>
        <span className="cd-cong-tac__trang-thai">{pa.an_danh ? GUI.an_danh_bat : GUI.an_danh_tat}</span>
      </button>
      <p className="cd-ghi-chu">{GUI.an_danh_giai_thich}</p>

      {!pa.an_danh && (
        <>
          <ONhapDong
            id="cd-ho-ten"
            nhan={GUI.nhan_ho_ten}
            gia_tri={pa.ho_ten}
            toi_da={DO_DAI_TOI_DA.ho_ten}
            onDoi={(v) => onDoi({ ...pa, ho_ten: v })}
          />
          <ONhapDong
            id="cd-dien-thoai"
            nhan={GUI.nhan_dien_thoai}
            gia_tri={pa.dien_thoai}
            toi_da={DO_DAI_TOI_DA.dien_thoai}
            kieu_ban_phim="tel"
            onDoi={(v) => onDoi({ ...pa, dien_thoai: v })}
          />
        </>
      )}

      <p className="cd-ghi-chu">{GUI.chua_ho_tro_anh}</p>

      {props.loi !== null && (
        <p className="cd-loi" role="alert">
          {props.loi}
        </p>
      )}

      <button type="button" className="cd-nut" onClick={props.onTiep}>
        {GUI.nut_tiep}
      </button>
    </div>
  );
}

/* ─────────────────────── bước 2: xác nhận xã ─────────────────────── */

export function BuocXacNhan(props: { ten_xa: string; onGui: () => void; onSua: () => void }) {
  return (
    <div className="cd-buoc" aria-labelledby="cd-xac-nhan-tieu-de">
      <h2 className="cd-tieu-de-phu" id="cd-xac-nhan-tieu-de">
        {GUI.xac_nhan_tieu_de}
      </h2>
      <p className="cd-cau">{GUI.xac_nhan_cau}</p>
      <p className="cd-xa-xac-nhan">{props.ten_xa}</p>
      <p className="cd-ghi-chu">{GUI.xac_nhan_hau_qua}</p>
      <button type="button" className="cd-nut" onClick={props.onGui}>
        {GUI.nut_gui(props.ten_xa)}
      </button>
      <button type="button" className="cd-nut-phu" onClick={props.onSua}>
        {GUI.nut_sua}
      </button>
    </div>
  );
}

/* ─────────────────────── bước 4: mã tra cứu ─────────────────────── */

/**
 * MÃ TRA CỨU TO, ĐỨNG ĐẦU (luật 10, bất biến 1): đó là thứ duy nhất người dân có để hỏi lại về
 * phiếu này. Dưới nó là tình trạng và mốc cán bộ phải xem phiếu — `acknowledge_due`, giờ Việt Nam.
 */
export function KetQuaGui(props: { phieu: PhieuCuaToi; onGuiKhac: () => void }) {
  const { phieu } = props;
  const han_xem = phieu.han_tiep_nhan === null ? null : thoiDiemVN(phieu.han_tiep_nhan);
  return (
    <div className="cd-buoc" role="status">
      <h2 className="cd-tieu-de-phu">{GUI.xong_tieu_de}</h2>
      <p className="cd-cau">{GUI.xong_ma}</p>
      <p className="cd-ma-tra-cuu">{phieu.ma_tra_cuu}</p>
      <p className="cd-ghi-chu">{GUI.xong_giu_ma}</p>
      <p className="cd-cau">
        <strong>{nhanTrangThai(phieu.trang_thai)}</strong>
      </p>
      {han_xem !== null && <p className="cd-cau">{GUI.se_xem_truoc(han_xem)}</p>}
      <ThePhieu phieu={phieu} />
      <button type="button" className="cd-nut-phu" onClick={props.onGuiKhac}>
        {GUI.gui_phieu_khac}
      </button>
    </div>
  );
}

type NhanhLoi = keyof typeof LOI_GUI;

/** Câu lỗi kèm việc cần làm. "Gửi lại" dùng lại CÙNG lần gửi — cùng thân, cùng khoá. */
export function LoiGui(props: { nhanh: NhanhLoi; onGuiLai: () => void; onSua: () => void }) {
  const loi = LOI_GUI[props.nhanh];
  return (
    <div className="cd-buoc">
      <p className="cd-loi" role="alert">
        {loi.cau}
      </p>
      {loi.co_the_gui_lai && (
        <button type="button" className="cd-nut" onClick={props.onGuiLai}>
          {GUI.nut_gui_lai}
        </button>
      )}
      <button type="button" className="cd-nut-phu" onClick={props.onSua}>
        {GUI.nut_sua}
      </button>
    </div>
  );
}

/* ───────────────────────────── màn ───────────────────────────── */

type Buoc =
  | { kieu: "nhap"; loi: string | null }
  | { kieu: "xac-nhan" }
  | { kieu: "dang-gui" }
  | { kieu: "xong"; phieu: PhieuCuaToi }
  | { kieu: "loi"; nhanh: NhanhLoi };

/** Nhánh kết quả của lớp gọi → bước tiếp theo của màn. */
export function buocSauKhiGui(kq: KetQuaGoi): Buoc | "kenh-chua-mo" {
  switch (kq.kieu) {
    case "xong":
      return { kieu: "xong", phieu: kq.phieu };
    case "chua-co-phien":
    case "chua-cau-hinh":
      return "kenh-chua-mo";
    case "khong-thay":
      return { kieu: "loi", nhanh: "loi-may-chu" };
    default:
      return { kieu: "loi", nhanh: kq.kieu };
  }
}

export function GuiPhanAnhScreen({ onQuayLai }: { onQuayLai: () => void }) {
  // Đọc một lần lúc dựng. `null` hôm nay — xem `api/phien-vigov.ts`.
  const [phien] = useState(layPhienViGov);
  const [pa, datPa] = useState<PhanAnhMoi>(PHAN_ANH_TRONG);
  const [buoc, datBuoc] = useState<Buoc>({ kieu: "nhap", loi: null });
  const [kenhDong, datKenhDong] = useState(false);
  /** Lần gửi đang dở. Giữ qua "Gửi lại"; bỏ khi người dân quay lại sửa (`api/lan-gui.ts`). */
  const [lan, datLan] = useState<LanGui | null>(null);

  const nutQuayLai = (
    <button type="button" className="quay-lai" onClick={onQuayLai}>
      {QUAY_LAI}
    </button>
  );

  if (phien === null || kenhDong) {
    return (
      <section className="cd-man" aria-label={GUI.tieu_de}>
        {nutQuayLai}
        <h1 className="cd-tieu-de">{GUI.tieu_de}</h1>
        <KenhChuaMo />
      </section>
    );
  }

  async function gui(lan_gui: LanGui) {
    datBuoc({ kieu: "dang-gui" });
    const tiep = buocSauKhiGui(await guiPhanAnh(lan_gui));
    if (tiep === "kenh-chua-mo") datKenhDong(true);
    else {
      if (tiep.kieu === "xong") datLan(null);
      datBuoc(tiep);
    }
  }

  function guiLanDau() {
    let lan_gui = lan;
    if (lan_gui === null) {
      try {
        lan_gui = taoLanGui(thanGuiPhanAnh(pa));
      } catch {
        datBuoc({ kieu: "loi", nhanh: "khong-tao-duoc-khoa" });
        return;
      }
      datLan(lan_gui);
    }
    void gui(lan_gui);
  }

  function suaLai() {
    datLan(null);
    datBuoc({ kieu: "nhap", loi: null });
  }

  let than: ReactNode;
  switch (buoc.kieu) {
    case "nhap":
      than = (
        <BuocNhap
          pa={pa}
          loi={buoc.loi}
          onDoi={(moi) => {
            datPa(moi);
            // Nội dung đổi thì lần gửi cũ không còn đúng thân của nó.
            datLan(null);
          }}
          onTiep={() => {
            const loi = kiemPhanAnh(pa);
            datBuoc(loi === null ? { kieu: "xac-nhan" } : { kieu: "nhap", loi });
          }}
        />
      );
      break;
    case "xac-nhan":
      than = <BuocXacNhan ten_xa={phien.ten_xa} onGui={guiLanDau} onSua={suaLai} />;
      break;
    case "dang-gui":
      than = (
        <p className="cd-cau" role="status">
          {GUI.dang_gui}
        </p>
      );
      break;
    case "xong":
      than = (
        <KetQuaGui
          phieu={buoc.phieu}
          onGuiKhac={() => {
            datPa(PHAN_ANH_TRONG);
            datBuoc({ kieu: "nhap", loi: null });
          }}
        />
      );
      break;
    case "loi":
      than = (
        <LoiGui
          nhanh={buoc.nhanh}
          onGuiLai={() => {
            if (lan !== null) void gui(lan);
          }}
          onSua={suaLai}
        />
      );
      break;
  }

  return (
    <section className="cd-man" aria-label={GUI.tieu_de}>
      {nutQuayLai}
      <BangXa ten_xa={phien.ten_xa} />
      <h1 className="cd-tieu-de">{GUI.tieu_de}</h1>
      {than}
    </section>
  );
}
