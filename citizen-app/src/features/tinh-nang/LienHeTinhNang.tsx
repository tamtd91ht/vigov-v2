/**
 * KHỐI ĐĂNG NHẬP (`getPhoneNumber`) VÀ DANH SÁCH VĂN PHÒNG CỦA MÀN LIÊN HỆ.
 *
 * ⚠ "TÌM VĂN PHÒNG GẦN TÔI" ĐÃ TRỞ LẠI — VÀ LẦN NÀY XẾP ĐƯỢC THẬT (12/10/2026, chủ dự án). Ngày 11/10 nút
 * ấy bị gỡ vì `getLocation` chỉ trả một mã và màn này không đổi được mã thành toạ độ: xin quyền mà không
 * xếp được gì. Nay mã được đổi Ở MÁY CHỦ (`vihat-miniapp` `POST /api/v1/location`, cùng client với trang
 * xã — `nearby-offices.ts`), khoảng cách tính trên máy, danh sách xếp gần nhất lên đầu. Ba văn phòng và
 * nút "Chỉ đường" vẫn hiện thẳng trước khi bấm gì: từ chối hay hỏng thì danh sách vẫn nguyên thứ tự cũ.
 *
 * ⚠ KHỐI `getPhoneNumber` LÀ KHỐI ĐĂNG NHẬP, KHÔNG PHẢI "ĐĂNG KÝ NHẬN TƯ VẤN" NỮA (ADR 0020):
 *
 *   Đăng nhập MỘT CHẠM bằng chính số Zalo. **Không OTP, không ô nhập sáu số, không màn chờ mã** —
 *   ADR 0020 bác đường ấy vì màn nhập sáu số là rào thật với người cao tuổi, và kênh này người
 *   dân không chọn. Ai định thêm một bước nhập mã vào đây phải mở lại ADR ấy trước.
 *
 *   Khối này có mặt ở CẢ HAI biến thể, **và cả hai đều gọi máy chủ thật** — kể cả BẢN NỘP. Một
 *   nút đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút nói "bản này chưa nối máy
 *   chủ" (điều 3.3.4). Máy chủ là `vihat-miniapp`, backend riêng của Tập đoàn ViHAT Group
 *   (bên vận hành, câu mở #28 — 25/09/2026).
 *
 * ⚠ `getPhoneNumber` chỉ trả token. Đổi token cần khoá bí mật của Mini App, và khoá ấy chỉ
 *   được nằm ở máy chủ (ADR 0020, bất biến 2) — nên không có bước đổi mã nào ở client, ở bất kỳ
 *   biến thể nào.
 *
 * ⚠ HAI ĐƯỜNG LIÊN HỆ Ở LẠI NGAY DƯỚI NÚT, VÀ CHÚNG KHÔNG PHẢI TRANG TRÍ: người từ chối chia sẻ
 * số điện thoại vẫn phải liên hệ được ngay. "Từ chối không làm mất tính năng" có ca test riêng.
 */
import { CONTACT, OFFICES } from "../../content/company-profile";
import { CompassGlyph, HandshakeGlyph, MailGlyph, PhoneGlyph, PinGlyph } from "../company-intro/icons";
import { MOC_TIM_VAN_PHONG } from "../company-intro/dieu-huong";
import { PhatHanhPhien } from "../dang-nhap/PhatHanhPhien";

import { HotlineLink } from "./HotlineLink";
import { TinhNangCoTrangThai } from "./khung";
import { moRaNgoai } from "./mo-ra-ngoai";
import { createNearbyFlow, formatDistanceKm, type NearbyRow, type NearbyState } from "./nearby-offices";
import { DANG_NHAP, LOI_MO_NGOAI, MA_RONG, NEARBY_OFFICES, VAN_PHONG } from "./noi-dung";
import { type MaDangNhap, xinMaDangNhap } from "./zalo-api";
import { useEffect, useRef, useState } from "react";

/**
 * Đường mở bản đồ cho một địa chỉ văn phòng.
 *
 * Địa chỉ là dữ liệu doanh nghiệp đã công bố (`content/company-profile.ts`), không phải dữ liệu
 * cá nhân — nên đưa nó vào một URL là hợp lệ. KHÔNG có gì của người dùng đi kèm: không toạ độ,
 * không token, không mã định danh nào. Bản đồ chỉ nhận đúng địa chỉ văn phòng.
 */
export function duongDanBanDo(dia_chi: string): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(dia_chi)}`;
}

/**
 * Ba văn phòng, THUẦN. `rows` absent = the published order with no distance; present = nearest first, each with
 * "Cách khoảng X km" (the only trace of the person's location, `nearby-offices.ts`).
 */
export function DanhSachVanPhong({
  onChiDuong,
  rows,
}: {
  onChiDuong: (dia_chi: string) => void;
  rows?: readonly NearbyRow[];
}) {
  const ordered = rows ?? OFFICES.map((_, index) => ({ index, km: null as number | null }));
  return (
    <ul className="van-phong">
      {ordered.map(({ index, km }) => {
        const office = OFFICES[index]!;
        return (
          <li className="van-phong__mot" key={office.name}>
            <PinGlyph className="van-phong__glyph" />
            <span className="van-phong__chu">
              <strong className="van-phong__ten">{office.name}</strong>
              <span className="van-phong__dia-chi">{office.address}</span>
              {km !== null && (
                <span className="van-phong__khoang-cach">{NEARBY_OFFICES.distance(formatDistanceKm(km))}</span>
              )}
            </span>
            <button type="button" className="tn-hanh-dong" onClick={() => onChiDuong(office.address)}>
              {VAN_PHONG.nut_chi_duong}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/** The in-page question before Zalo's dialog — the purpose, then "Cho phép" / "Không". Focus lands on "Không". */
function NearbyQuestion({ onAllow, onDeny }: { onAllow: () => void; onDeny: () => void }) {
  const denyRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    denyRef.current?.focus();
  }, []);
  const id = "tn-van-phong-cau-hoi";
  return (
    <div className="vh-photo-question" role="group" aria-labelledby={id}>
      <p className="vh-photo-question__why" id={id}>
        {NEARBY_OFFICES.purpose}
      </p>
      <button type="button" className="tn__nut" onClick={onAllow}>
        {NEARBY_OFFICES.allow}
      </button>
      <button type="button" className="tn__nut-phu" ref={denyRef} onClick={onDeny}>
        {NEARBY_OFFICES.deny}
      </button>
    </div>
  );
}

/**
 * The block, PURE apart from the callbacks — the tests render every state without a DOM.
 *
 * The find button sits above the list in every state but `asking` (where the question takes its place), and is
 * disabled while locating, the state said in WORDS on the button, not by colour.
 */
export function NearbyOfficesView(props: {
  state: NearbyState;
  onFind: () => void;
  onAllow: () => void;
  onDeny: () => void;
  onChiDuong: (dia_chi: string) => void;
  cannotOpen: boolean;
}) {
  const { state } = props;
  return (
    <section className="tn hien-len" aria-labelledby={MOC_TIM_VAN_PHONG}>
      <div className="tn__dau">
        <span className="tn__huy-hieu" aria-hidden="true">
          <CompassGlyph className="tn__glyph" />
        </span>
        <h2 className="tn__tieu-de" id={MOC_TIM_VAN_PHONG}>
          {VAN_PHONG.tieu_de}
        </h2>
      </div>
      {state.step === "asking" ? (
        <NearbyQuestion onAllow={props.onAllow} onDeny={props.onDeny} />
      ) : (
        <button
          type="button"
          className="tn__nut"
          disabled={state.step === "locating"}
          aria-busy={state.step === "locating"}
          onClick={props.onFind}
        >
          {state.step === "locating" ? NEARBY_OFFICES.locating : NEARBY_OFFICES.find}
        </button>
      )}
      <div role="status">
        {state.step === "sorted" && <p className="tn__nhac">{NEARBY_OFFICES.sorted}</p>}
        {state.step === "default" && <p className="tn__loi">{state.note}</p>}
      </div>
      <DanhSachVanPhong onChiDuong={props.onChiDuong} rows={state.step === "sorted" ? state.rows : undefined} />
      {props.cannotOpen && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
    </section>
  );
}

export function TimVanPhong() {
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);
  const [state, setState] = useState<NearbyState>({ step: "idle" });
  // One flow per screen opening: "Cho phép" is remembered until the screen unmounts, never written down.
  const [flow] = useState(() => createNearbyFlow(setState));

  // Qua CỬA KHAI BÁO (`mo-ra-ngoai.ts`), không gọi thẳng `moTrangWeb`: `"ban-do"` là một đích đã
  // khai trong `content/dich-ra-ngoai.ts`, tức một dòng đã có trong câu khai của chính sách.
  async function chiDuong(dia_chi: string) {
    datKhongMoDuoc(!(await moRaNgoai("ban-do", duongDanBanDo(dia_chi))));
  }

  // `id` của tiêu đề là mỏ neo mục "Văn phòng" của menu nhanh (`MOC_TIM_VAN_PHONG`).
  return (
    <NearbyOfficesView
      state={state}
      onFind={flow.find}
      onAllow={() => void flow.allow()}
      onDeny={flow.deny}
      onChiDuong={(dia_chi) => void chiDuong(dia_chi)}
      cannotOpen={khong_mo_duoc}
    />
  );
}

/**
 * Hai đường liên hệ chạy được ngay bây giờ, đặt ngay dưới nút đăng nhập.
 *
 * Chúng lặp lại hotline và email ở đầu màn Liên hệ, và việc lặp lại ấy là CÓ CHỦ ĐÍCH: người
 * vừa từ chối chia sẻ số điện thoại, hoặc vừa đọc câu "bản này chưa mở được phiên", phải thấy
 * ngay một đường khác ở đúng chỗ mắt họ đang nhìn. Bắt họ cuộn ngược lên là bắt một người đang
 * cần liên hệ tự đi tìm cách liên hệ.
 */
function DuongLienHeNgay() {
  return (
    <>
      <p className="tn__nhac">{DANG_NHAP.nhac_lien_he}</p>
      <div className="tn__doi-nut">
        {/* Through `HotlineLink`: inside Zalo a bare `tel:` does nothing (owner 12/10/2026). */}
        <HotlineLink className="tn-hanh-dong tn-hanh-dong--rong">
          <PhoneGlyph className="tn-hanh-dong__glyph" />
          Gọi {CONTACT.hotlineLabel}
        </HotlineLink>
        <a className="tn-hanh-dong tn-hanh-dong--rong" href={`mailto:${CONTACT.email}`}>
          <MailGlyph className="tn-hanh-dong__glyph" />
          {CONTACT.email}
        </a>
      </div>
    </>
  );
}

/**
 * Kết quả một lần đăng nhập: bước máy chủ đổi mã lấy phiên. Mã không hiện lên màn hình.
 *
 * MÃ RỖNG THÌ DỪNG Ở ĐÂY, KHÔNG GỌI MÁY CHỦ. `zmp-sdk` trả mã rỗng ở môi trường phát triển
 * (`MA_RONG` nói ra điều đó). Gửi một mã rỗng đi thì máy chủ từ chối, và người đọc nhận một câu
 * "mã đã quá hạn" — một câu SAI, chỉ ra một việc phải làm không giải quyết được gì.
 */
export function KetQuaDangNhap({ ma, onBusyChange }: { ma: MaDangNhap; onBusyChange?: (busy: boolean) => void }) {
  if (ma.ma_so_dien_thoai === "") return <p className="tn__loi">{MA_RONG}</p>;
  return <PhatHanhPhien ma={ma} onBusyChange={onBusyChange} />;
}

/**
 * KHỐI ĐĂNG NHẬP ĐỊNH DANH — `getPhoneNumber` + `getAccessToken`, một lần chạm.
 *
 * Vẫn đúng khuôn `TinhNangCoTrangThai` như năm khối kia: một tiêu đề, một lý do đọc được, một
 * nút, một chỗ hiện kết quả, và bốn nhánh — xong · từ chối · ngoài Zalo · không lấy được. Từ
 * chối là ĐƯỜNG BÌNH THƯỜNG (`code === -201`), không phải lỗi.
 *
 * Since 12/10/2026 it is the first action of the "Kết nối với ViHAT" card (`heading="h3"`,
 * `company-intro/ConnectWithVihat.tsx`), which shows it only while no session is held.
 */
export function KhoiDangNhap({ heading = "h2" }: { heading?: "h2" | "h3" }) {
  // Zalo answering is half of the login; the server exchange is the other half. The button waits for both.
  const [exchanging, setExchanging] = useState(false);
  return (
    <TinhNangCoTrangThai
      ma="dang-nhap"
      xin={xinMaDangNhap}
      cap_tieu_de={heading}
      glyph={<HandshakeGlyph className="tn__glyph" />}
      busy={exchanging}
      veKetQua={(ma) => <KetQuaDangNhap ma={ma} onBusyChange={setExchanging} />}
      duoi_cung={<DuongLienHeNgay />}
    />
  );
}
