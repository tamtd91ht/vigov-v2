/**
 * MÀN "GỬI PHẢN ÁNH" — hành vi DUY NHẤT một công dân ghi vào hệ thống này (luật 10).
 *
 * NĂM BƯỚC, MỖI MÀN MỘT VIỆC (`skills/accessibility-elderly` #4):
 *
 *   lĩnh vực  →  nhập  →  xác nhận xã  →  đang gửi  →  mã tra cứu   (hoặc câu lỗi kèm việc cần làm)
 *
 * ⚠ BƯỚC XÁC NHẬN XÃ LÀ LUẬT NGHIỆP VỤ, KHÔNG PHẢI TRANG TRÍ (README §Non-negotiables #5): gửi nhầm
 * xã là xã ấy nhận việc ngoài địa bàn, phải chuyển hoặc từ chối, còn người dân chờ vô ích. Tên xã ở
 * bước ấy là `ten_xa` CỦA PHIÊN — đúng xã máy chủ sẽ ghi phiếu.
 *
 * ⚠ LĨNH VỰC LÀ BƯỚC ĐẦU VÀ BẮT BUỘC (ADR 0050 #1, #9; chủ dự án 01/10/2026): danh mục là CỦA XÃ
 * (`GET /my-citizen-report-fields`, tải khi màn mở và CHỈ khi có phiên), mã chọn được đi lên thành
 * `field`. "Tiếp tục" tắt tới khi chọn. Không có danh sách dự phòng (ADR 0060 §3): danh mục lỗi hay rỗng
 * là một câu, kèm "Thử lại" khi bấm lại có ích. 400 `field_not_offered` (xã vừa đổi danh mục) → về bước
 * lĩnh vực, tải lại, chọn lại; nội dung đã viết giữ nguyên. Máy chủ vẫn để `field` tuỳ chọn — bắt buộc
 * là luật của màn này, không phải của `petitions`. Cán bộ vẫn chốt lĩnh vực cuối (ADR 0028).
 *
 * Không có ô chọn xã: xã lấy từ phiên (ADR 0022). Không có ảnh: chưa có kho lưu ảnh, và màn hình nói
 * thẳng điều đó.
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG VẼ BIỂU MẪU và KHÔNG GỌI MẠNG (`api/phien-vigov.ts`). The session
 * bridge exists end to end — `vihat-miniapp` opens a ViGov citizen session for a login carrying
 * `communeHostHint` and forwards `vigovSession`, which `api/mo-phien-vigov.ts` records. Its Zalo
 * account-id step is, since `vihat-miniapp` 4114f00, a real call (`LayMaTaiKhoan`, `GET
 * graph.zalo.me/v2.0/me?fields=id`, `internal/zalo/ma_tai_khoan.go`) — not yet measured against real
 * Zalo, so whether a session is issued on a real phone is still unknown.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";

import { citizenReportFields, guiPhanAnh, type KetQuaGoi, myResidentialUnits } from "../api/goi-vigov";
import {
  DO_DAI_TOI_DA,
  type PhanAnhMoi,
  type PhieuCuaToi,
  type SceneLocation,
  thanGuiPhanAnh,
} from "../api/hop-dong-phan-anh";
import { type LanGui, taoLanGui } from "../api/lan-gui";
import type { ReopenWithPhone } from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { type Catalogue, fieldLabelOf, offeredCodes, readCatalogueAnswer } from "./field-catalogue";
import { BangXa, KenhChuaMo, LoadingNotice, ThePhieu } from "./khung";
import {
  CUA_TOI,
  GUI,
  KHAN_CAP,
  LOI_GUI,
  nhanTrangThai,
  QUAY_LAI,
  SEND_LOCATION_WORDS,
  SEND_RESIDENTIAL_UNIT_WORDS,
  TYPED_CONTACT,
} from "./noi-dung";
import { ONhapDoan, ONhapDong } from "./o-nhap";
import { PhoneVerificationPanel, usePhoneVerification } from "./phone-verification";
import {
  ResidentialUnitField,
  type ResidentialUnitList,
  residentialUnitName,
  useResidentialUnits,
} from "./residential-unit-picker";
import { isPlausiblePhone } from "./trai-nghiem";
import {
  formatCoordinates,
  type GetSceneLocation,
  SceneLocationControl,
  type SceneLocationFailure,
  useSceneLocation,
} from "./scene-location";
import type { ZaloFailure } from "../api/mo-phien-vigov";
import { thoiDiemVN } from "../../lib/thoi-diem";

export const PHAN_ANH_TRONG: PhanAnhMoi = {
  noi_dung: "",
  dia_chi: "",
  ho_ten: "",
  dien_thoai: "",
  an_danh: false,
};

/**
 * Câu cần sửa, hoặc `null` khi gửi được. Đếm theo KÝ TỰ như máy chủ, không theo byte. The field comes
 * first: it is step 1 and required on this screen (the server keeps it optional — see the header).
 *
 * `typedContact` (ADR 0080): Zalo gave no number and the citizen chose to type a name and a number — both are
 * then required, the number must look like a Vietnamese phone, and anonymous does not apply.
 */
export function kiemPhanAnh(pa: PhanAnhMoi, typedContact = false): string | null {
  const dai = (s: string) => [...s.trim()].length;
  const anonymous = !typedContact && pa.an_danh;
  if ((pa.field ?? "").trim() === "") return GUI.field_missing;
  if (dai(pa.noi_dung) === 0) return GUI.thieu_noi_dung;
  if (dai(pa.noi_dung) > DO_DAI_TOI_DA.noi_dung) return GUI.qua_dai("Nội dung", DO_DAI_TOI_DA.noi_dung);
  if (dai(pa.dia_chi) > DO_DAI_TOI_DA.dia_chi) return GUI.qua_dai("Nơi xảy ra", DO_DAI_TOI_DA.dia_chi);
  if (typedContact && dai(pa.ho_ten) === 0) return TYPED_CONTACT.name_missing;
  if (!anonymous && dai(pa.ho_ten) > DO_DAI_TOI_DA.ho_ten) {
    return GUI.qua_dai("Họ và tên", DO_DAI_TOI_DA.ho_ten);
  }
  if (!anonymous && dai(pa.dien_thoai) > DO_DAI_TOI_DA.dien_thoai) {
    return GUI.qua_dai("Số điện thoại", DO_DAI_TOI_DA.dien_thoai);
  }
  if (typedContact && dai(pa.dien_thoai) === 0) return TYPED_CONTACT.phone_missing;
  if (typedContact && !isPlausiblePhone(pa.dien_thoai)) return TYPED_CONTACT.phone_invalid;
  return null;
}

/**
 * CHỖ ĐẶT TIÊU ĐIỂM KHI ĐỔI BƯỚC — một `id` cho mỗi bước, phần tử mang nó có `tabIndex={-1}`.
 *
 * VÌ SAO: bấm "Tiếp tục" / "Gửi tới …" là nút vừa bấm BIẾN MẤT cùng bước cũ. Người dùng trình đọc
 * màn hình bị bỏ lại trên một nút không còn, và không biết màn đã sang bước nào. Đưa tiêu điểm tới
 * đầu bước mới thì trình đọc đọc ngay bước ấy nói gì (`skills/accessibility-elderly`).
 *
 * Bước "nhập" trỏ vào tiêu đề chung của màn: không trỏ vào ô nhập, vì tiêu điểm ở ô nhập là bàn
 * phím điện thoại bật lên che nửa màn.
 */
export const ID_DAU_BUOC = {
  field: "cd-field-title",
  nhap: "cd-gui-tieu-de",
  "xac-nhan": "cd-xac-nhan-tieu-de",
  "dang-gui": "cd-dang-gui",
  xong: "cd-xong-tieu-de",
  loi: "cd-loi-gui",
  "can-so": "cd-xac-thuc-so",
} as const;

/* ─────────────────────────── step 1: field ─────────────────────────── */

/** The shared app's catalogue failures: the commune app's three, plus a 401 said on the step itself. */
export type SharedCatalogueFailure = "unavailable" | "network" | "server" | "expired";

export function sharedCatalogueFailureText(f: SharedCatalogueFailure): string {
  switch (f) {
    case "unavailable":
      return GUI.field_unavailable;
    case "network":
      return GUI.field_network;
    case "server":
      return GUI.field_server;
    case "expired":
      return GUI.field_expired;
  }
}

/**
 * Step 1 — the commune's own fields, in its order, as full-width tiles (one column: long Vietnamese names
 * wrap instead of shrinking). `<button role="radio">` in a `radiogroup`, not an `<input>`: this app's only
 * input controls live in `o-nhap.tsx` (`phase1-collects-nothing.test.ts`). The picked tile says "Đã chọn"
 * in words — status is never shown by a border or a colour alone (README §Non-negotiables #6).
 *
 * "Tiếp tục" is DISABLED until a field the catalogue offers is picked, with the sentence saying why right
 * above it. Loading, failure and an empty catalogue are words; none of them draws a tile or "Tiếp tục".
 */
export function FieldPickStep(props: {
  catalogue: Catalogue<SharedCatalogueFailure>;
  picked: string;
  /** The last send was refused with `field_not_offered`: say why before the list. */
  fieldChanged: boolean;
  onPick: (code: string) => void;
  onNext: () => void;
  onRetry: () => void;
}) {
  const { catalogue } = props;
  const offered = offeredCodes(catalogue);
  const canGoOn = offered !== null && props.picked !== "" && offered.includes(props.picked);

  let body: ReactNode;
  if (catalogue.kind === "loading") {
    body = <LoadingNotice cau={GUI.field_loading} shape="card" />;
  } else if (catalogue.kind === "failed") {
    body = (
      <>
        <p className="cd-loi" role="alert">
          {sharedCatalogueFailureText(catalogue.failure)}
        </p>
        {/* A 401 is not cured by pressing again — its sentence says to reopen the app instead. */}
        {catalogue.failure !== "expired" && (
          <button type="button" className="cd-nut" onClick={props.onRetry}>
            {CUA_TOI.nut_thu_lai}
          </button>
        )}
      </>
    );
  } else if (catalogue.fields.length === 0) {
    body = (
      <p className="cd-cau" role="status">
        {GUI.field_empty}
      </p>
    );
  } else {
    body = (
      <>
        <p className="cd-cau">{GUI.field_prompt}</p>
        <div className="cd-fields" role="radiogroup" aria-labelledby={ID_DAU_BUOC.field}>
          {catalogue.fields.map((f) => {
            const on = props.picked === f.code;
            return (
              <button
                key={f.code}
                type="button"
                role="radio"
                aria-checked={on}
                className={`cd-field${on ? " cd-field--picked" : ""}`}
                onClick={() => props.onPick(f.code)}
              >
                <span className="cd-field__label">{f.label}</span>
                {on && <span className="cd-field__state">{GUI.field_picked}</span>}
              </button>
            );
          })}
        </div>
        {!canGoOn && <p className="cd-ghi-chu">{GUI.field_pick_first}</p>}
        <button type="button" className="cd-nut" disabled={!canGoOn} onClick={props.onNext}>
          {GUI.nut_tiep}
        </button>
      </>
    );
  }

  return (
    <div className="cd-buoc">
      <h2 className="cd-tieu-de-phu" id={ID_DAU_BUOC.field} tabIndex={-1}>
        {GUI.field_title}
      </h2>
      {props.fieldChanged && (
        <p className="cd-loi" role="alert">
          {LOI_GUI["field-not-offered"].cau}
        </p>
      )}
      {body}
    </div>
  );
}

/* ─────────────────────────── bước 2: nhập ─────────────────────────── */

/** The field picked on step 1, shown on the writing step with the way back to change it. */
export type InputStepField = { label: string; onChange: () => void };

/**
 * The location control's state on the input step. `null` = the shell injected no location function
 * (outside Zalo, tests): no button at all, rather than a button that can only fail.
 */
export type InputStepLocation = {
  locating: boolean;
  failure: SceneLocationFailure | null;
  /** Zalo's refusal of the last tap, with its code (`scene-location.tsx`). */
  zalo?: ZaloFailure | null;
  onLocate: () => void;
} | null;

/** The thôn picker's state on the input step: the commune's list and the last 400's sentence, if any. */
export type InputStepResidentialUnit = {
  list: ResidentialUnitList;
  refusal: string | null;
  onPick: (id: string) => void;
};

export function BuocNhap(props: {
  pa: PhanAnhMoi;
  loi: string | null;
  onDoi: (pa: PhanAnhMoi) => void;
  onTiep: () => void;
  field: InputStepField;
  location?: InputStepLocation;
  /** The typed-contact path (ADR 0080): name + number required, anonymous off with its reason. */
  typedContact?: boolean;
  /** The optional thôn picker (ADR 0088). Absent = no field (tests of other parts of the step). */
  residentialUnit?: InputStepResidentialUnit;
}) {
  const { pa, onDoi } = props;
  const location = props.location ?? null;
  const typedContact = props.typedContact === true;
  const anonymous = !typedContact && pa.an_danh;
  return (
    <div className="cd-buoc">
      {/* "Lĩnh vực: <tên> · Đổi" — what was picked on step 1, and a real 48px button back to it. */}
      <div className="cd-field-picked">
        <span className="cd-field-picked__text">
          {GUI.field_label}: <strong>{props.field.label}</strong>
        </span>
        <button
          type="button"
          className="cd-field-change"
          aria-label={GUI.field_change_name}
          onClick={props.field.onChange}
        >
          {GUI.field_change}
        </button>
      </div>

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
      {/* UNDER the address box, which stays optional and editable: the location adds a point, it never
          fills or replaces the words the citizen typed (`scene-location.tsx`). */}
      {location !== null && (
        <SceneLocationControl
          words={SEND_LOCATION_WORDS}
          look="shared"
          locating={location.locating}
          location={pa.scene_location ?? null}
          failure={location.failure}
          zalo={location.zalo ?? null}
          onLocate={location.onLocate}
        />
      )}
      {/* OPTIONAL thôn / tổ dân phố (ADR 0088), right after the address block. The prototype's send form has no
          such field (PROTOTYPE.md §6.2 AddressBlock), so it sits where the location information already is. */}
      {props.residentialUnit !== undefined && (
        <ResidentialUnitField
          id="cd-thon"
          list={props.residentialUnit.list}
          picked={pa.residential_unit_id ?? ""}
          refusal={props.residentialUnit.refusal}
          words={SEND_RESIDENTIAL_UNIT_WORDS}
          look="shared"
          onPick={props.residentialUnit.onPick}
        />
      )}

      {/* NÚT BẬT/TẮT, KHÔNG PHẢI Ô ĐÁNH DẤU: trạng thái nói bằng CHỮ ("Đang bật"), không bằng một
          dấu tích nhỏ hay một màu (README §Non-negotiables #6), và đích chạm to bằng cả dòng. */}
      {typedContact && (
        <p className="cd-cau" role="note">
          {TYPED_CONTACT.form_note}
        </p>
      )}
      <button
        type="button"
        className="cd-cong-tac"
        aria-pressed={anonymous}
        disabled={typedContact}
        onClick={() => onDoi({ ...pa, an_danh: !pa.an_danh })}
      >
        <span className="cd-cong-tac__ten">{GUI.an_danh}</span>
        <span className="cd-cong-tac__trang-thai">{anonymous ? GUI.an_danh_bat : GUI.an_danh_tat}</span>
      </button>
      <p className="cd-ghi-chu">{typedContact ? TYPED_CONTACT.anonymous_off : GUI.an_danh_giai_thich}</p>

      {!anonymous && (
        <>
          <ONhapDong
            id="cd-ho-ten"
            nhan={typedContact ? TYPED_CONTACT.name_label : GUI.nhan_ho_ten}
            gia_tri={pa.ho_ten}
            toi_da={DO_DAI_TOI_DA.ho_ten}
            required={typedContact || undefined}
            onDoi={(v) => onDoi({ ...pa, ho_ten: v })}
          />
          <ONhapDong
            id="cd-dien-thoai"
            nhan={typedContact ? TYPED_CONTACT.phone_label : GUI.nhan_dien_thoai}
            gia_tri={pa.dien_thoai}
            toi_da={DO_DAI_TOI_DA.dien_thoai}
            kieu_ban_phim="tel"
            required={typedContact || undefined}
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

/* ─────────────────────── bước 3: xác nhận xã ─────────────────────── */

export function BuocXacNhan(props: {
  ten_xa: string;
  /** The picked field's name, read again at the last step with the commune (owner, 01/10/2026). */
  fieldLabel: string;
  onGui: () => void;
  onSua: () => void;
  /** The location that goes with the petition, if any — said here, at the last step, before sending. */
  location?: SceneLocation | null;
  /** The picked thôn's name, read again here with the commune; "" or absent = none picked, no line. */
  residentialUnitName?: string;
}) {
  const location = props.location ?? null;
  const unitName = props.residentialUnitName ?? "";
  return (
    <div className="cd-buoc" aria-labelledby={ID_DAU_BUOC["xac-nhan"]}>
      <h2 className="cd-tieu-de-phu" id={ID_DAU_BUOC["xac-nhan"]} tabIndex={-1}>
        {GUI.xac_nhan_tieu_de}
      </h2>
      <p className="cd-cau">{GUI.xac_nhan_cau}</p>
      <p className="cd-xa-xac-nhan">{props.ten_xa}</p>
      <p className="cd-ghi-chu">{GUI.xac_nhan_hau_qua}</p>
      <p className="cd-cau">
        {GUI.field_label}: <strong>{props.fieldLabel}</strong>
      </p>
      {unitName !== "" && (
        <p className="cd-cau">
          {SEND_RESIDENTIAL_UNIT_WORDS.summary_label}: <strong>{unitName}</strong>
        </p>
      )}
      {location !== null && <p className="cd-cau">{GUI.confirm_location(formatCoordinates(location))}</p>}
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
 *
 * `role="status"` CHỈ bọc câu và mã, không bọc cả khối: tiêu điểm đã tới tiêu đề (`ID_DAU_BUOC`),
 * nên vùng thông báo chỉ cần đọc thêm mã. Bọc cả khối là trình đọc đọc lại cả thẻ phiếu lẫn nút.
 */
export function KetQuaGui(props: { phieu: PhieuCuaToi; onGuiKhac: () => void }) {
  const { phieu } = props;
  const han_xem = phieu.han_tiep_nhan === null ? null : thoiDiemVN(phieu.han_tiep_nhan);
  return (
    <div className="cd-buoc">
      <h2 className="cd-tieu-de-phu" id={ID_DAU_BUOC.xong} tabIndex={-1}>
        {GUI.xong_tieu_de}
      </h2>
      <div role="status">
        <p className="cd-cau">{GUI.xong_ma}</p>
        <p className="cd-ma-tra-cuu">{phieu.ma_tra_cuu}</p>
      </div>
      <p className="cd-ghi-chu">{GUI.xong_giu_ma}</p>
      {/* ADR 0080: an unverified petition is never notified — said under the code, from the server's field. */}
      {phieu.contact_unverified === true && (
        <p className="cd-cau" role="note">
          {TYPED_CONTACT.done_no_notice}
        </p>
      )}
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

/**
 * Câu lỗi kèm việc cần làm. "Gửi lại" dùng lại CÙNG lần gửi — cùng thân, cùng khoá. `message`: the server's
 * own sentence, shown instead of ours (`unverified-daily-limit` only, `api/goi-vigov.ts`).
 */
export function LoiGui(props: { nhanh: NhanhLoi; message?: string; onGuiLai: () => void; onSua: () => void }) {
  const loi = LOI_GUI[props.nhanh];
  return (
    <div className="cd-buoc">
      <p className="cd-loi" role="alert" id={ID_DAU_BUOC.loi} tabIndex={-1}>
        {props.message ?? loi.cau}
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
  /** Step 1. `changed`: the server refused the picked field (`field_not_offered`) — say so above the list. */
  | { kieu: "field"; changed: boolean }
  /** `unitRefusal`: a 400 `residential_unit_not_offered` sent the citizen back here — its sentence, for the field. */
  | { kieu: "nhap"; loi: string | null; unitRefusal?: string }
  | { kieu: "xac-nhan" }
  | { kieu: "dang-gui" }
  | { kieu: "xong"; phieu: PhieuCuaToi }
  | { kieu: "loi"; nhanh: NhanhLoi; message?: string }
  /** Máy chủ cần số điện thoại đã xác thực — khung `phone-verification.tsx`. Nháp `pa` vẫn nguyên. */
  | { kieu: "can-so" };

/** Thân bước "đang gửi". `role="status"`: người không nhìn màn hình nghe được là đang gửi. */
export function DangGui() {
  return (
    <p className="cd-cau" role="status" id={ID_DAU_BUOC["dang-gui"]} tabIndex={-1}>
      {GUI.dang_gui}
    </p>
  );
}

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
    case "can-xac-thuc-so":
      return { kieu: "can-so" };
    case "unverified-daily-limit":
      return kq.message === null ? { kieu: "loi", nhanh: kq.kieu } : { kieu: "loi", nhanh: kq.kieu, message: kq.message };
    case "residential-unit-not-offered":
      // Back to the writing step, the sentence AT the thôn field (the server's, or ours), the list reloaded by the
      // screen. Nothing was written; the next send has a different body.
      return { kieu: "nhap", loi: null, unitRefusal: kq.message ?? LOI_GUI[kq.kieu].cau };
    case "residential-unit-check-unavailable":
      return kq.message === null ? { kieu: "loi", nhanh: kq.kieu } : { kieu: "loi", nhanh: kq.kieu, message: kq.message };
    case "field-not-offered":
      // The commune changed its list since it was loaded: back to step 1 to pick again (the screen reloads
      // the catalogue). Sending the same body again would only meet the same answer.
      return { kieu: "field", changed: true };
    default:
      return { kieu: "loi", nhanh: kq.kieu };
  }
}

export function GuiPhanAnhScreen({
  onQuayLai,
  reopenWithPhone,
  getSceneLocation,
}: {
  onQuayLai: () => void;
  /** Hàm mở lại phiên kèm số do lớp vỏ tiêm vào (`api/mo-phien-vigov.ts`). Vắng = không có đường ấy. */
  reopenWithPhone?: ReopenWithPhone;
  /**
   * "Lấy vị trí hiện tại" — injected by the shell (`App.tsx`: Zalo codes + `vihat-miniapp` exchange).
   * Needs no ViGov session: the route is public. Absent = no location button.
   */
  getSceneLocation?: GetSceneLocation;
}) {
  // Đọc một lần lúc dựng. `null` until the bridge issues a session (see the header). Mở lại phiên kèm số không đổi
  // TÊN XÃ (`reopenSessionWithPhone` từ chối phiên khác xã), nên bản đọc một lần này vẫn đúng.
  const [phien] = useState(layPhienViGov);
  const phone = usePhoneVerification(reopenWithPhone);
  /**
   * The citizen chose "Gửi bằng họ tên và số điện thoại" after Zalo gave no number (ADR 0080). Applies only
   * while the session is still phone-less: a session reopened with a verified number sends as usual.
   */
  const [typedChosen, setTypedChosen] = useState(false);
  const typedContact = typedChosen && layPhienViGov()?.phone_verified !== true;
  const [pa, datPa] = useState<PhanAnhMoi>(PHAN_ANH_TRONG);
  const [buoc, datBuoc] = useState<Buoc>({ kieu: "field", changed: false });
  const [kenhDong, datKenhDong] = useState(false);
  const [catalogue, setCatalogue] = useState<Catalogue<SharedCatalogueFailure>>({ kind: "loading" });
  /** One catalogue call at a time: "Thử lại" pressed twice must not race two answers into the step. */
  const catalogueBusy = useRef(false);
  /** Lần gửi đang dở. Giữ qua "Gửi lại"; bỏ khi người dân quay lại sửa (`api/lan-gui.ts`). */
  const [lan, datLan] = useState<LanGui | null>(null);
  /**
   * The location lives IN the form (`pa.scene_location`) so the body is built from exactly what the
   * screen shows. Functional update: the citizen may type while the exchange runs, and their words must
   * not be overwritten by a copy of the form taken before the tap. A new location is a new body, so the
   * pending send (and its Idempotency-Key) is dropped, exactly as `onDoi` does for a typed change.
   */
  /**
   * The commune's thôn list — loaded with the catalogue, only with a session (no session = no call, as above).
   * `unitRefusal`: the last 400 `residential_unit_not_offered`'s sentence, shown at the field until a new pick.
   */
  const units = useResidentialUnits(myResidentialUnits, phien !== null);
  const [unitRefusal, setUnitRefusal] = useState<string | null>(null);
  const sceneLocation = useSceneLocation(getSceneLocation, (location) => {
    datPa((t) => ({ ...t, scene_location: location }));
    datLan(null);
  });

  // Đổi bước → tiêu điểm tới đầu bước mới (`ID_DAU_BUOC`). So với bước TRƯỚC, không dùng cờ "lần
  // đầu": StrictMode chạy hiệu ứng hai lần lúc gắn, và cờ ấy sẽ kéo tiêu điểm ngay khi mở màn.
  // Theo `kieu`, không theo cả `buoc`: bấm "Tiếp tục" mà còn thiếu nội dung vẫn là bước nhập, câu
  // lỗi tự đọc ra (`role="alert"`) và tiêu điểm nên ở lại nút vừa bấm.
  const buoc_truoc = useRef(buoc.kieu);
  useEffect(() => {
    if (buoc_truoc.current === buoc.kieu) return;
    buoc_truoc.current = buoc.kieu;
    document.getElementById(ID_DAU_BUOC[buoc.kieu])?.focus();
  }, [buoc.kieu]);

  /**
   * The commune's catalogue → step 1. The session-shaped answers say what the shared app says everywhere
   * else on this screen: no session / no address / intake not set up → the channel is closed; 401 → a
   * sentence on the step (reopen the app); 403 phone → the phone panel, then the catalogue again.
   */
  async function loadCatalogue() {
    if (catalogueBusy.current) return;
    catalogueBusy.current = true;
    setCatalogue({ kind: "loading" });
    const read = readCatalogueAnswer(await citizenReportFields());
    catalogueBusy.current = false;
    switch (read.kind) {
      case "no-session":
      case "not-configured":
      case "intake-closed":
        datKenhDong(true);
        return;
      case "expired":
        setCatalogue({ kind: "failed", failure: "expired" });
        return;
      case "phone-required":
        datBuoc({ kieu: "can-so" });
        phone.onPhoneRequired(() => {
          datBuoc({ kieu: "field", changed: false });
          void loadCatalogue();
        });
        return;
      default:
        setCatalogue(read);
    }
  }

  // Loaded once when the screen opens — and ONLY with a session: without one the screen draws "kênh chưa
  // mở" below and must not call anything (`hieu-ung-khong-phien.test.tsx`). Ref guard: StrictMode runs
  // effects twice on mount, and the citizen must not cost the commune two calls for one screen.
  const catalogueStarted = useRef(false);
  useEffect(() => {
    if (phien === null || catalogueStarted.current) return;
    catalogueStarted.current = true;
    void loadCatalogue();
  }, []);

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
    else if (tiep.kieu === "can-so") {
      // Gọi lại với CÙNG lần gửi — cùng thân, cùng `Idempotency-Key`: 403 trả về trước khi máy chủ ghi
      // gì, nên đây vẫn là lần gửi ấy, không phải một phiếu mới.
      datBuoc(tiep);
      phone.onPhoneRequired(() => void gui(lan_gui));
    } else if (tiep.kieu === "nhap") {
      // `residential_unit_not_offered`: the attempt goes (a new pick is a new body), the pick is cleared — the
      // unit is not offered any more — and the list reloaded. What the citizen wrote stays.
      datLan(null);
      datPa((t) => ({ ...t, residential_unit_id: null }));
      setUnitRefusal(tiep.unitRefusal ?? null);
      datBuoc({ kieu: "nhap", loi: null });
      units.reload();
    } else if (tiep.kieu === "field") {
      // `field_not_offered`: the attempt is dropped — the next send has a different body, so it is a
      // different act (`lan-gui.ts`). The picked code goes; what the citizen wrote stays.
      datLan(null);
      datPa((t) => ({ ...t, field: "" }));
      datBuoc(tiep);
      void loadCatalogue();
    } else {
      // A 201, or the daily ceiling of unverified petitions (nothing recorded — a later send is a new act).
      if (tiep.kieu === "xong" || (tiep.kieu === "loi" && tiep.nhanh === "unverified-daily-limit")) datLan(null);
      datBuoc(tiep);
    }
  }

  function guiLanDau() {
    // ADR 0080 decision 1: a session with no verified phone is INVITED to share the Zalo number before anything
    // is sent — the server would now accept the send as unverified, so no 403 asks any more. Read fresh: a
    // reopen with the number replaces the session, and the rerun below must see it.
    if (layPhienViGov()?.phone_verified !== true && !typedChosen) {
      datBuoc({ kieu: "can-so" });
      phone.onPhoneRequired(() => guiLanDau());
      return;
    }
    let lan_gui = lan;
    if (lan_gui === null) {
      try {
        lan_gui = taoLanGui(thanGuiPhanAnh(typedContact ? { ...pa, an_danh: false } : pa));
      } catch {
        datBuoc({ kieu: "loi", nhanh: "khong-tao-duoc-khoa" });
        return;
      }
      datLan(lan_gui);
    }
    void gui(lan_gui);
  }

  /** Back to editing — to step 1 when no field is picked (the phone panel can stand before step 1 too). */
  function suaLai() {
    phone.reset();
    datLan(null);
    datBuoc((pa.field ?? "") === "" ? { kieu: "field", changed: false } : { kieu: "nhap", loi: null });
  }

  const fieldLabel = fieldLabelOf(catalogue, pa.field ?? "");

  let than: ReactNode;
  switch (buoc.kieu) {
    case "field":
      than = (
        <FieldPickStep
          catalogue={catalogue}
          picked={pa.field ?? ""}
          fieldChanged={buoc.changed}
          onRetry={() => void loadCatalogue()}
          onPick={(code) => {
            if (code !== pa.field) datLan(null);
            datPa((t) => ({ ...t, field: code }));
            // A new pick answers the "field no longer offered" notice; same `kieu`, so focus stays on the tile.
            if (buoc.changed) datBuoc({ kieu: "field", changed: false });
          }}
          onNext={() => datBuoc({ kieu: "nhap", loi: null })}
        />
      );
      break;
    case "nhap":
      than = (
        <BuocNhap
          pa={pa}
          loi={buoc.loi}
          field={{ label: fieldLabel, onChange: () => datBuoc({ kieu: "field", changed: false }) }}
          onDoi={(moi) => {
            datPa(moi);
            // Nội dung đổi thì lần gửi cũ không còn đúng thân của nó.
            datLan(null);
          }}
          typedContact={typedContact}
          residentialUnit={{
            list: units.list,
            refusal: unitRefusal,
            onPick: (id) => {
              // A different unit is a different body: the pending attempt (and its key) goes.
              if (id !== (pa.residential_unit_id ?? "")) datLan(null);
              datPa((t) => ({ ...t, residential_unit_id: id === "" ? null : id }));
              setUnitRefusal(null);
            },
          }}
          onTiep={() => {
            const loi = kiemPhanAnh(pa, typedContact);
            // No field (or one the loaded catalogue no longer names): step 1 is where that is fixed.
            if (loi === GUI.field_missing || fieldLabel === "") datBuoc({ kieu: "field", changed: false });
            else datBuoc(loi === null ? { kieu: "xac-nhan" } : { kieu: "nhap", loi });
          }}
          location={
            getSceneLocation === undefined
              ? null
              : {
                  locating: sceneLocation.locating,
                  failure: sceneLocation.failure,
                  zalo: sceneLocation.zalo,
                  onLocate: () => void sceneLocation.locate(),
                }
          }
        />
      );
      break;
    case "xac-nhan":
      than = (
        <BuocXacNhan
          ten_xa={phien.ten_xa}
          fieldLabel={fieldLabel}
          onGui={guiLanDau}
          onSua={suaLai}
          location={pa.scene_location ?? null}
          residentialUnitName={residentialUnitName(units.list, pa.residential_unit_id ?? "")}
        />
      );
      break;
    case "dang-gui":
      than = <DangGui />;
      break;
    case "xong":
      than = (
        <KetQuaGui
          phieu={buoc.phieu}
          onGuiKhac={() => {
            datPa(PHAN_ANH_TRONG);
            setUnitRefusal(null);
            sceneLocation.reset();
            // A new petition starts at step 1: the field of the last one is not this one's.
            datBuoc({ kieu: "field", changed: false });
          }}
        />
      );
      break;
    case "loi":
      than = (
        <LoiGui
          nhanh={buoc.nhanh}
          {...(buoc.message === undefined ? {} : { message: buoc.message })}
          onGuiLai={() => {
            if (lan !== null) void gui(lan);
          }}
          onSua={suaLai}
        />
      );
      break;
    case "can-so":
      // `null` chỉ trong khoảnh khắc giữa "đã xác thực" và lần gọi lại — lần gọi ấy đưa màn sang
      // "đang gửi" ngay.
      than =
        phone.state === null ? (
          <DangGui />
        ) : (
          <>
            <PhoneVerificationPanel
              state={phone.state}
              task="submit"
              focusId={ID_DAU_BUOC["can-so"]}
              draftKept
              onAllow={() => void phone.allow()}
              onDecline={phone.decline}
              onManual={() => {
                // Back to the writing step with name + number required (ADR 0080); the attempt goes — the body
                // changes (anonymous off), so the next send is a new act. What was written stays.
                phone.reset();
                setTypedChosen(true);
                datLan(null);
                datBuoc({ kieu: "nhap", loi: null });
              }}
            />
            {phone.state.kieu !== "dang-xac-nhan" && (
              <button type="button" className="cd-nut-phu" onClick={suaLai}>
                {GUI.nut_sua}
              </button>
            )}
          </>
        );
      break;
  }

  return (
    <section className="cd-man" aria-label={GUI.tieu_de}>
      {nutQuayLai}
      <BangXa ten_xa={phien.ten_xa} />
      <h1 className="cd-tieu-de" id={ID_DAU_BUOC.nhap} tabIndex={-1}>
        {GUI.tieu_de}
      </h1>
      {than}
    </section>
  );
}
