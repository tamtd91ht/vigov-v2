import { useState } from "react";

import { TIEU_DE_CHINH_SACH } from "../../content/chinh-sach-rieng-tu";
import { CONTACT } from "../../content/company-profile";
import { HotlineLink, TimVanPhong } from "../tinh-nang/index";

import { ConnectWithVihat } from "./ConnectWithVihat";
import { MOC_QUAN_LY_QUYEN, type ThamSoMan } from "./dieu-huong";
import { DocumentGlyph, MailGlyph, PhoneGlyph, ShieldGlyph } from "./icons";
import { NutWebsite } from "./NutWebsite";
import { PrivacyPolicyScreen } from "./PrivacyPolicyScreen";
import { QuanLyQuyenScreen } from "./QuanLyQuyenScreen";

/**
 * Màn Liên hệ — và là nơi hai trong ba tính năng thật của ứng dụng sống.
 *
 * THỨ TỰ TRÊN MÀN LÀ THỨ TỰ CHẠY ĐƯỢC NGAY → CẦN THÊM MỘT BƯỚC:
 *
 *   1. Hotline · email · website. Hotline và email chạy được ngay bây giờ, không cần quyền nào,
 *      không cần máy chủ nào. Người muốn nói chuyện với công ty phải gặp chúng trước tiên.
 *      ⚠ WEBSITE NAY LÀ MỘT NÚT, KHÔNG CÒN LÀ MỘT NEO `<a target="_blank">` (21/09/2026): bên
 *      trong Zalo, một liên kết mở bằng thẻ `a` không có đường quay lại app. Nó cũng chỉ hiện khi
 *      `COMPANY.website` có giá trị — xem `NutWebsite.tsx`.
 *   2. Thẻ "Kết nối với ViHAT" (12/10/2026) — ba việc xin số điện thoại, mỗi việc một câu mục
 *      đích đứng trước nút của nó: đăng nhập bằng số Zalo (`getPhoneNumber`, một lần chạm, không
 *      mã sáu số — ADR 0020), chat với chuyên viên, nhận ưu đãi qua SMS. Hai đường liên hệ chạy
 *      được ngay vẫn nằm ngay dưới nút đăng nhập, cho người từ chối chia sẻ số.
 *   3. Văn phòng — ba văn phòng và nút chỉ đường luôn hiện; không xin vị trí (11/10/2026).
 *
 *   Đặt hai tính năng cần quyền lên trước ba đường liên hệ chạy được là bắt một khách hàng đang
 *   cần gọi phải đi qua hai lời xin quyền trước đã. Không ai làm thế với một khách hàng.
 *
 * Ba neo đầu giữ lớp `.action`, có `min-height: var(--tap-min)` được `accessibility.test.ts`
 * ghim: một đích chạm dưới 44px là đích một bàn tay run không bấm trúng.
 *
 * Hotline và email là đầu mối doanh nghiệp đã công bố của ViHAT Group, không phải của một cá
 * nhân nào — nên chúng là dữ liệu doanh nghiệp, không phải dữ liệu cá nhân. Câu dẫn nói điều ấy
 * trên màn đã gỡ cùng hai ghi chú quy thuộc khác: nó nói "đây là đầu mối CỦA CÔNG TY MẸ", câu
 * chỉ có nghĩa khi bên phát hành app là công ty con.
 */
/**
 * ⚠ VÌ SAO "QUẢN LÝ QUYỀN" LÀ MÀN CON CỦA TAB LIÊN HỆ CHỨ KHÔNG PHẢI TAB THỨ SÁU — đo, không đoán:
 *
 *   `accessibility.test.ts` đo bề rộng thanh tab trên máy 320px, máy hẹp nhất còn bán được. Với
 *   sáu tab, mỗi tab còn ~45px chữ, tức tối đa 4 ký tự cho TỪ dài nhất của một nhãn. "Trang",
 *   "thiếp", "ViHAT", "Quyền" đều 5 ký tự. Tab thứ sáu chỉ vào được bằng cách rút ngắn nhãn của
 *   những tab đang có tới chỗ không còn đọc ra nghĩa, hoặc bằng cách thu nhỏ chữ — và thu nhỏ chữ
 *   là thứ phép đo ấy sinh ra để chặn.
 *
 *   Tab Liên hệ là đúng chỗ: hàng "Chính sách quyền riêng tư" ở đây (màn con riêng từ 12/10/2026),
 *   tức là nơi người tìm thông tin pháp lý — và người duyệt của Zalo — đã đứng sẵn.
 */
/**
 * Which sub-screen of Liên hệ is open. The policy remembers where it was opened from, so its back button returns
 * there (Quản lý quyền links to it too, 12/10/2026). UI state only — never written to the device.
 */
type ContactView = "contact" | "permissions" | "policy-from-contact" | "policy-from-permissions";

export function ContactScreen({ moc }: ThamSoMan) {
  /**
   * Màn con đang mở. Trạng thái giao diện thuần, không ghi xuống máy.
   *
   * ⚠ GIÁ TRỊ BAN ĐẦU ĐỌC TỪ `moc`, và điều đó chỉ đúng vì vỏ app DỰNG LẠI màn này mỗi lần điều
   * hướng có mốc (`key` trong `App.tsx`). Không có bước dựng lại ấy thì mục "Quyền" trên màn chủ
   * chỉ mở được đúng một lần trong cả phiên: lần thứ hai `useState` giữ nguyên giá trị cũ và
   * người bấm thấy đầu tab Liên hệ — một nút im lặng không làm gì.
   */
  const [view, setView] = useState<ContactView>(moc === MOC_QUAN_LY_QUYEN ? "permissions" : "contact");

  if (view === "permissions") {
    return (
      <QuanLyQuyenScreen
        onQuayLai={() => setView("contact")}
        onOpenPolicy={() => setView("policy-from-permissions")}
      />
    );
  }

  if (view === "policy-from-contact" || view === "policy-from-permissions") {
    const fromPermissions = view === "policy-from-permissions";
    return (
      <PrivacyPolicyScreen
        backLabel={fromPermissions ? "Quay lại Quản lý quyền" : "Quay lại Liên hệ"}
        onBack={() => setView(fromPermissions ? "permissions" : "contact")}
      />
    );
  }

  return (
    <>
      <section className="banner">
        <div className="banner__content">
          <h1 className="banner__title">Liên hệ</h1>
        </div>
      </section>

      {/* `HotlineLink`: inside Zalo a bare `tel:` does nothing; it dials through the platform (12/10/2026). */}
      <HotlineLink className="action action--primary hien-len">
        <span className="tile" aria-hidden="true">
          <PhoneGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gọi hotline</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </HotlineLink>

      <a className="action hien-len hien-len--2" href={`mailto:${CONTACT.email}`}>
        <span className="tile tile--soft" aria-hidden="true">
          <MailGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gửi email</span>
          <span className="action__value">{CONTACT.email}</span>
        </span>
      </a>

      <NutWebsite lop_them="hien-len hien-len--3" />

      {/* EVERY ASK OF THE PHONE NUMBER, IN ONE CARD (12/10/2026): log in, chat with a specialist, SMS offers — each
          its own action with its own purpose sentence before its button (`ConnectWithVihat.tsx`). Below the three
          contacts that need nothing, by the rule at the top of this file. The floating "Chat Zalo" stays as is. */}
      <ConnectWithVihat />

      <TimVanPhong />

      {/* ĐƯỜNG VÀO MÀN QUẢN LÝ QUYỀN, ĐẶT NGAY TRÊN CHÍNH SÁCH QUYỀN RIÊNG TƯ. Hai thứ trả lời hai
          nửa của cùng một câu hỏi — "app này lấy gì của tôi": màn quyền nói app GỌI những gì của
          nền tảng, chính sách nói dữ liệu ĐI ĐÂU. Để chúng cạnh nhau thì người đọc không phải tìm
          nửa còn lại. */}
      <button type="button" className="action action--mo" onClick={() => setView("permissions")}>
        <span className="tile tile--soft" aria-hidden="true">
          <ShieldGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Quản lý quyền</span>
          <span className="action__value">Quyền truy cập và mục đích sử dụng</span>
        </span>
      </button>

      {/* THE POLICY IS ONE ROW HERE, ITS TEXT ON ITS OWN SCREEN (owner 12/10/2026: no text walls). The row stays
          on Liên hệ — where people and the Zalo reviewer look for legal information — right under Quản lý quyền,
          the other half of "what does this app take from me". */}
      <button type="button" className="action action--mo" onClick={() => setView("policy-from-contact")}>
        <span className="tile tile--soft" aria-hidden="true">
          <DocumentGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Dữ liệu của bạn được dùng thế nào</span>
          <span className="action__value">{TIEU_DE_CHINH_SACH}</span>
        </span>
      </button>
    </>
  );
}
