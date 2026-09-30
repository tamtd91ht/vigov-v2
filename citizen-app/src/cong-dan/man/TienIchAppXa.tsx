/**
 * CÁC MÀN CÒN LẠI CỦA APP RIÊNG — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026: "làm đủ
 * các màn như bản mẫu"): tra cứu hồ sơ, truyền thanh, video, bản đồ, thông báo, cá nhân. Không màn định
 * danh, không đăng nhập (bỏ 28/09/2026). Họ tên ở Cá nhân CHỈ ĐỂ HIỂN THỊ (29/09/2026): nó được lấy từ
 * Zalo một lần lúc mở app (`TrangXa`), và không có thì hiện "Chưa xác định" — màn này không gọi Zalo,
 * vì hỏi lại ở đây là hỏi lần hai một câu bà con đã trả lời lúc mở app.
 *
 * MÀN NÀO CHƯA CÓ DỮ LIỆU THẬT thì hiện trạng thái trống bằng lời ("xã chưa cập nhật…"), KHÔNG dữ liệu
 * giả: một bản tin bịa trong app mang tên cơ quan nhà nước là một thông tin sai do xã phát hành.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: quét căn cước (dữ liệu định danh — luật 3, điều kiện dừng), đăng nhập, đăng xuất, lưu
 * cài đặt xuống máy (`localStorage` cấm ở nửa này — `ranh-gioi-hai-nua.test.ts` §3b; cỡ chữ sống trong
 * bộ nhớ của lần mở; công tắc thông báo bỏ tới khi có thông báo thật).
 */
import { useState } from "react";

import { BUILD_LABEL } from "../../lib/build-label";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";
import { CUA_TOI, XA_TN } from "./noi-dung";
import { ONhapDong } from "./o-nhap";

/* ═══════════════════════════════ TRA CỨU HỒ SƠ ═══════════════════════════════ */

/**
 * Tra cứu hồ sơ một cửa — theo spec kho yêu cầu (`05-nghiep-vu.md:148`): SỐ ĐIỆN THOẠI ĐẦY ĐỦ + 4 SỐ CUỐI
 * của số hồ sơ, cả hai bắt buộc. CHƯA có hệ thống một cửa nào nối vào ViGov, nên mọi lần tra nói thật điều
 * ấy — không bao giờ trả một kết quả dựng ra. Hai ô gõ vào không rời máy.
 */
/**
 * Both fields filled the way the one-stop lookup needs them (`PROTOTYPE.md` §6.8): at least 9 digits of
 * phone (any spaces or dots typed are ignored), and exactly 4 digits of the file number. PURE.
 */
export function dossierLookupReady(phone: string, lastFour: string): boolean {
  return phone.replace(/\D/g, "").length >= 9 && /^\d{4}$/.test(lastFour.trim());
}

export function TraCuuHoSoXa({ onQuayLai }: { onQuayLai: () => void }) {
  const [phone, setPhone] = useState("");
  const [lastFour, setLastFour] = useState("");
  const [ket_qua, datKetQua] = useState<string | null>(null);
  const complete = dossierLookupReady(phone, lastFour);
  return (
    <>
      <DauManCon tieu_de={XA_TN.tra_cuu_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong id="xa-sdt-ho-so" nhan={XA_TN.o_so_dien_thoai_ho_so} goi_y={XA_TN.goi_y_so_dien_thoai_ho_so} gia_tri={phone} toi_da={20} kieu_ban_phim="tel" onDoi={setPhone} />
          <div className="xa-lookup-code">
            <ONhapDong id="xa-bon-so-cuoi" nhan={XA_TN.o_bon_so_cuoi} goi_y={XA_TN.goi_y_bon_so_cuoi} gia_tri={lastFour} toi_da={4} kieu_ban_phim="tel" onDoi={setLastFour} />
          </div>
          {/* Enabled only when both fields are complete (§6.8). A disabled button alone does not say why, so the
              sentence saying what is missing stands under it until then — words, not the grey alone. */}
          <button type="button" className="xa-nut" disabled={!complete} onClick={() => datKetQua(XA_TN.tra_cuu_chua_ket_noi)}>
            <BieuTuong ten="search" co={20} />
            {XA_TN.nut_tra_cuu}
          </button>
          {!complete && <p className="xa-phu">{XA_TN.tra_cuu_can_ma}</p>}
        </div>
        {ket_qua !== null && <KhoiTrangThai bieu_tuong="info" cau={ket_qua} />}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ MÀN CHƯA CÓ DỮ LIỆU ═══════════════════════════════ */

export function ManChuaCoDuLieu(props: {
  tieu_de: string;
  bieu_tuong: TenBieuTuong;
  cau: string;
  /** One muted sentence under `cau` (`KhoiTrangThai` `hint`), or none. */
  hint?: string;
  onQuayLai: () => void;
}) {
  return (
    <>
      <DauManCon tieu_de={props.tieu_de} onQuayLai={props.onQuayLai} />
      <TrangCon>
        <KhoiTrangThai bieu_tuong={props.bieu_tuong} cau={props.cau} hint={props.hint} />
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ CÁ NHÂN ═══════════════════════════════ */

export type CoChu = "vua" | "lon" | "rat-lon";

/**
 * Up to two letters for the avatar (`PROTOTYPE.md` §6.6, "Nguyễn Văn Hùng" → "NH"): the first letter of the
 * family name and of the given name — one letter for a one-word name. Upper-cased in Vietnamese. PURE.
 */
export function initials(fullName: string): string {
  const words = fullName.trim().split(/\s+/).filter((w) => w !== "");
  if (words.length === 0) return "";
  const first = words[0]!.charAt(0);
  const last = words.length > 1 ? words[words.length - 1]!.charAt(0) : "";
  return `${first}${last}`.toLocaleUpperCase("vi");
}

/** One row of Cá nhân: the 40px round icon on blue-50, a semibold label, a muted hint under it. */
function SettingText(props: { icon: TenBieuTuong; label: string; hint?: string }) {
  return (
    <>
      <span className="xa-setting-icon" aria-hidden="true">
        <BieuTuong ten={props.icon} co={22} />
      </span>
      <span className="xa-hang__chu">
        <strong className="xa-setting-label">{props.label}</strong>
        {props.hint !== undefined && <span className="xa-phu">{props.hint}</span>}
      </span>
    </>
  );
}

export function CaNhanXa(props: {
  /** Tên Zalo lấy lúc mở app, hoặc `null` → "Chưa xác định". */
  ho_ten: string | null;
  ten_xa: string;
  tinh: string;
  /**
   * Petitions loaded in this open: the count, and whether more pages exist; `null` when not loaded (no
   * session yet) — then no number is shown, never an invented 0.
   */
  so_phieu: { readonly count: number; readonly more: boolean } | null;
  co_chu: CoChu;
  onDoiCoChu: (c: CoChu) => void;
  onMoPhanAnh: () => void;
  onMoTraCuu: () => void;
  /** "Thông báo" — the screen the home bell used to open (decision 10: the bell is gone from the header). */
  onOpenNotifications: () => void;
}) {
  const co_chu: ReadonlyArray<[CoChu, string]> = [
    ["vua", XA_TN.co_chu_vua],
    ["lon", XA_TN.co_chu_lon],
    ["rat-lon", XA_TN.co_chu_rat_lon],
  ];
  const letters = props.ho_ten ? initials(props.ho_ten) : "";

  return (
    <div className="xa-trang xa-trang--tab">
      {/* The profile card (§6.6): what this app KNOWS — the Zalo name taken at entry and the commune of the app.
          No hamlet and no phone line: the app holds neither (a later card), and a guessed line is a wrong one. */}
      <div className="xa-the xa-the--dem xa-profile">
        <span className="xa-avatar" aria-hidden="true">
          {letters !== "" ? letters : <BieuTuong ten="user" co={30} />}
        </span>
        <span className="xa-hang__chu">
          <strong className="xa-profile__name">{props.ho_ten ?? XA_TN.chua_co_ten}</strong>
          <span className="xa-phu">
            {props.ten_xa}
            {props.tinh !== "" ? ` · ${props.tinh}` : ""}
          </span>
        </span>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.hien_thi}</h2>
      <div className="xa-the">
        {/* Three levels, not the prototype's one checkbox (owner: keep the three). */}
        <div className="xa-setting">
          <div className="xa-hang xa-hang--tinh">
            <SettingText icon="text" label={XA_TN.co_chu} hint={XA_TN.text_size_hint} />
          </div>
          <div className="xa-chips xa-setting__chips" role="group" aria-label={XA_TN.co_chu}>
            {co_chu.map(([k, n]) => (
              <button
                key={k}
                type="button"
                className={`xa-chip${props.co_chu === k ? " xa-chip--on" : ""}`}
                aria-pressed={props.co_chu === k}
                onClick={() => props.onDoiCoChu(k)}
              >
                {n}
              </button>
            ))}
          </div>
        </div>
        <div className="xa-ke xa-ke--sat" />
        {/* A statement, not a control: Vietnamese is the only language the app has. No "Nhận thông báo" switch
            either — a switch wired to nothing lets a citizen believe they opted out of messages the commune will
            later send; the real opt-out lives on the server, next to ZNS. */}
        <div className="xa-hang xa-hang--tinh">
          <SettingText icon="globe" label={XA_TN.language} hint={XA_TN.language_value} />
        </div>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.of_mine}</h2>
      <div className="xa-the">
        <button type="button" className="xa-hang" onClick={props.onMoPhanAnh}>
          <SettingText
            icon="message-square-warning"
            label={CUA_TOI.tieu_de}
            hint={
              props.so_phieu === null
                ? XA_TN.so_phieu_unknown
                : props.so_phieu.more
                  ? XA_TN.so_phieu_more(props.so_phieu.count)
                  : XA_TN.so_phieu(props.so_phieu.count)
            }
          />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onMoTraCuu}>
          <SettingText icon="file-search" label={XA_TN.o_tra_cuu_ho_so} hint={XA_TN.lich_su_tra_cuu_phu} />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onOpenNotifications}>
          <SettingText icon="bell" label={XA_TN.thong_bao} />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
      </div>

      {BUILD_LABEL !== "" && (
        <p className="xa-app-version">
          <BieuTuong ten="info" co={18} />
          <span>{XA_TN.app_version(BUILD_LABEL)}</span>
        </p>
      )}
    </div>
  );
}
