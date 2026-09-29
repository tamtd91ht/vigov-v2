/**
 * CÁC MÀN CÒN LẠI CỦA APP RIÊNG — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026: "làm đủ
 * các màn như bản mẫu"): tra cứu hồ sơ, truyền thanh, video, bản đồ, thông báo, cá nhân. Không màn định
 * danh, không đăng nhập (bỏ 28/09/2026). Họ tên ở Cá nhân CHỈ ĐỂ HIỂN THỊ (29/09/2026): nó được lấy từ
 * Zalo một lần lúc mở app (`CommuneHome`), và không có thì hiện "Chưa xác định" — màn này không gọi Zalo,
 * vì hỏi lại ở đây là hỏi lần hai một câu bà con đã trả lời lúc mở app.
 *
 * MÀN NÀO CHƯA CÓ DỮ LIỆU THẬT thì hiện trạng thái trống bằng lời ("xã chưa cập nhật…"), KHÔNG dữ liệu
 * giả: một bản tin bịa trong app mang tên cơ quan nhà nước là một thông tin sai do xã phát hành.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: quét căn cước (dữ liệu định danh — luật 3, điều kiện dừng), đăng nhập, đăng xuất, lưu
 * cài đặt xuống máy (`localStorage` cấm ở nửa này — `two-halves-boundary.test.ts` §3b; cỡ chữ sống trong
 * bộ nhớ của lần mở; công tắc thông báo bỏ tới khi có thông báo thật).
 */
import { useState } from "react";

import { Icon, type IconName } from "./Icon";
import { SubScreenHeader, StatusBlock, IconTile, SubPage } from "./commune-frame";
import { MY_REPORTS, COMMUNE_APP_SCREENS } from "./copy";
import { TextField } from "./input-field";
import { initials } from "./commune-app-model";

/* ═══════════════════════════════ TRA CỨU HỒ SƠ ═══════════════════════════════ */

/**
 * Tra cứu hồ sơ một cửa — theo spec kho yêu cầu (`05-nghiep-vu.md:148`): SỐ ĐIỆN THOẠI ĐẦY ĐỦ + 4 SỐ CUỐI
 * của số hồ sơ, cả hai bắt buộc. CHƯA có hệ thống một cửa nào nối vào ViGov, nên mọi lần tra nói thật điều
 * ấy — không bao giờ trả một kết quả dựng ra. Hai ô gõ vào không rời máy.
 */
export function CommuneRecordLookup({ onBack }: { onBack: () => void }) {
  const [phone, setPhone] = useState("");
  const [lastFour, setLastFour] = useState("");
  const [result, setResult] = useState<string | null>(null);
  const complete = phone.replace(/\D/g, "").length >= 9 && /^\d{4}$/.test(lastFour.trim());
  return (
    <>
      <SubScreenHeader title={COMMUNE_APP_SCREENS.lookup_title} onBack={onBack} />
      <SubPage>
        <div className="xa-the xa-the--dem xa-khoi">
          <TextField id="xa-sdt-ho-so" label={COMMUNE_APP_SCREENS.field_profile_phone} suggestion={COMMUNE_APP_SCREENS.hint_profile_phone} value={phone} max={20} input_mode="tel" onChange={setPhone} />
          <TextField id="xa-bon-so-cuoi" label={COMMUNE_APP_SCREENS.field_last_four} suggestion={COMMUNE_APP_SCREENS.hint_last_four} value={lastFour} max={4} input_mode="tel" onChange={setLastFour} />
          <button
            type="button"
            className="xa-nut"
            onClick={() => setResult(complete ? COMMUNE_APP_SCREENS.lookup_not_connected : COMMUNE_APP_SCREENS.lookup_needs_code)}
          >
            <Icon name="search" size={20} />
            {COMMUNE_APP_SCREENS.lookup_tile_button}
          </button>
        </div>
        {result !== null && <StatusBlock icon="info" text={result} />}
      </SubPage>
    </>
  );
}

/* ═══════════════════════════════ MÀN CHƯA CÓ DỮ LIỆU ═══════════════════════════════ */

export function NoDataScreen(props: { title: string; icon: IconName; text: string; onBack: () => void }) {
  return (
    <>
      <SubScreenHeader title={props.title} onBack={props.onBack} />
      <SubPage>
        <StatusBlock icon={props.icon} text={props.text} />
      </SubPage>
    </>
  );
}

/* ═══════════════════════════════ CÁ NHÂN ═══════════════════════════════ */

export type FontSize = "vua" | "lon" | "rat-lon";

export function CommunePersonal(props: {
  /** Tên Zalo lấy lúc mở app, hoặc `null` → "Chưa xác định". */
  full_name: string | null;
  commune_name: string;
  province: string;
  /**
   * Petitions loaded in this open: the count, and whether more pages exist; `null` when not loaded (no
   * session yet) — then no number is shown, never an invented 0.
   */
  report_count: { readonly count: number; readonly more: boolean } | null;
  font_size: FontSize;
  onFontSizeChange: (c: FontSize) => void;
  onOpenReports: () => void;
  onOpenLookup: () => void;
}) {
  const font_sizes: ReadonlyArray<[FontSize, string]> = [
    ["vua", COMMUNE_APP_SCREENS.font_size_medium],
    ["lon", COMMUNE_APP_SCREENS.font_size_large],
    ["rat-lon", COMMUNE_APP_SCREENS.font_size_extra_large],
  ];

  return (
    <div className="xa-trang xa-trang--tab">
      <div className="xa-the xa-the--dem xa-khoi">
        <div className="xa-ho-so">
          <span className="xa-can-bo__chu-dau xa-ho-so__chu" aria-hidden="true">
            {props.full_name ? initials(props.full_name) : <Icon name="user" size={28} />}
          </span>
          <span className="xa-can-bo__chu">
            <strong className="xa-can-bo__ten">{props.full_name ?? COMMUNE_APP_SCREENS.no_name_yet}</strong>
          </span>
        </div>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.utilities}</h2>
      <div className="xa-the">
        <button type="button" className="xa-hang" onClick={props.onOpenReports}>
          <IconTile name="chat" color="hong" />
          <span className="xa-hang__chu">
            <strong>{MY_REPORTS.title}</strong>
            <span className="xa-phu">
              {props.report_count === null
                ? COMMUNE_APP_SCREENS.report_count_unknown
                : props.report_count.more
                  ? COMMUNE_APP_SCREENS.report_count_more(props.report_count.count)
                  : COMMUNE_APP_SCREENS.report_count(props.report_count.count)}
            </span>
          </span>
          <Icon name="right" size={20} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onOpenLookup}>
          <IconTile name="history" color="xanh" />
          <span className="xa-hang__chu">
            <strong>{COMMUNE_APP_SCREENS.lookup_history}</strong>
            <span className="xa-phu">{COMMUNE_APP_SCREENS.lookup_history_note}</span>
          </span>
          <Icon name="right" size={20} />
        </button>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.display}</h2>
      <div className="xa-the xa-the--dem">
        <p className="xa-nhan-o">{COMMUNE_APP_SCREENS.font_size}</p>
        <div className="xa-chips" role="group" aria-label={COMMUNE_APP_SCREENS.font_size}>
          {font_sizes.map(([k, n]) => (
            <button
              key={k}
              type="button"
              className={`xa-chip${props.font_size === k ? " xa-chip--on" : ""}`}
              aria-pressed={props.font_size === k}
              onClick={() => props.onFontSizeChange(k)}
            >
              {n}
            </button>
          ))}
        </div>
        <p className="xa-phu">{COMMUNE_APP_SCREENS.font_size_preview}</p>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.section_notices}</h2>
      {/* No switch until notifications exist: a switch wired to nothing lets a citizen believe they opted out
          of messages the commune will later send. The real opt-out lives on the server, next to ZNS. */}
      <div className="xa-the xa-the--dem">
        <p className="xa-phu">{COMMUNE_APP_SCREENS.notices_none_yet}</p>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.about_app}</h2>
      <div className="xa-the xa-the--dem">
        <p className="xa-nhan-o">{COMMUNE_APP_SCREENS.unit}</p>
        <p>
          {props.commune_name}
          {props.province !== "" ? ` · ${props.province}` : ""}
        </p>
      </div>
    </div>
  );
}
