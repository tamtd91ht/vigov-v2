import { useEffect, useRef } from "react";

import { BieuTuong } from "./BieuTuong";
import { XA_GIAO_DIEN } from "./noi-dung";

/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC CÓ Ô NHẬP — `phase1-collects-nothing.test.ts` miễn lệnh cấm
 * `<input|textarea>` cho ĐÚNG tệp này (24/09/2026), cạnh `features/yeu-cau/OGhiChu.tsx`.
 *
 * Mọi ô chữ của hai màn "Gửi phản ánh" và "Tra cứu phiếu" đi qua hai component dưới, nên mỗi điểm
 * thu thập mới phải đi qua đúng chỗ này — và ba điều dưới không ai phải nhớ lại ở từng màn:
 *
 *   · NHÃN Ở TRÊN Ô, không chỉ là chữ mờ trong ô (`skills/accessibility-elderly` #4): chữ mờ biến
 *     mất khi bắt đầu gõ, và người lớn tuổi quên mất ô này hỏi gì.
 *   · `autoComplete="off"`: nội dung phản ánh và số điện thoại không đi vào bộ nhớ gợi ý của
 *     trình duyệt trên một máy có thể cho mượn.
 *   · `maxLength` đặt theo giới hạn máy chủ, để người dân không gõ xong một trang rồi bị từ chối.
 *
 * Giá trị sống trong `useState` của màn cha — không ghi xuống máy (`ranh-gioi-hai-nua.test.ts` §3b).
 * Đóng app là mất bản đang gõ: cái giá ấy được nói ra, không giấu (xem báo cáo lượt này).
 */
type ChungProps = {
  id: string;
  nhan: string;
  goi_y?: string;
  gia_tri: string;
  toi_da: number;
  onDoi: (gia_tri: string) => void;
  /**
   * The prototype's grey example inside the box (commune app, owner 11/10/2026). NEVER instead of the label:
   * the label stays (above, or `labelHidden` for a screen reader where the prototype shows a heading instead).
   */
  placeholder?: string;
  /**
   * The label is read by screen readers only (`xa-sr-only`) — ONLY where a visible heading right above the box
   * already says what it asks (the prototype's address block, the directory search with its magnifier).
   */
  labelHidden?: boolean;
};

/** The label line: visible above the box, or screen-reader-only (`labelHidden`). */
function Label(props: { id: string; nhan: string; hidden?: boolean; requiredMark?: boolean }) {
  return (
    <label className={props.hidden === true ? "xa-sr-only" : "cd-o__nhan"} htmlFor={props.id}>
      {props.nhan}
      {/* The required mark of the prototype (`NewFeedbackPage.tsx:333`) — decoration: `aria-required` says it. */}
      {props.requiredMark === true && (
        <>
          {" "}
          <span className="xa-required-mark" aria-hidden="true">
            *
          </span>
        </>
      )}
    </label>
  );
}

/**
 * `required`: OPTIONAL — only the typed-contact path asks for it (ADR 0080: name and number are the commune's
 * only way back to the citizen). It sets `aria-required`, so a screen reader says what the label says.
 * `autoFocus`: OPTIONAL — only the commune send screen's name box, when "Sửa" opens it (08/10/2026): the box
 * replaces the button just pressed, and without it focus would fall back to the page.
 */
export function ONhapDong(
  props: ChungProps & { kieu_ban_phim?: "text" | "tel" | "numeric"; required?: boolean; autoFocus?: boolean },
) {
  const id_goi_y = props.goi_y ? `${props.id}-goi-y` : undefined;
  const keyboard = props.kieu_ban_phim ?? "text";
  return (
    <div className="cd-o">
      <Label id={props.id} nhan={props.nhan} hidden={props.labelHidden} />
      {props.goi_y && (
        <p className="cd-o__goi-y" id={id_goi_y}>
          {props.goi_y}
        </p>
      )}
      <input
        className="cd-o__nhap"
        id={props.id}
        type={keyboard === "tel" ? "tel" : "text"}
        inputMode={keyboard}
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        maxLength={props.toi_da}
        required={props.required}
        aria-required={props.required}
        autoFocus={props.autoFocus === true}
        placeholder={props.placeholder}
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => props.onDoi(e.target.value)}
      />
    </div>
  );
}

/**
 * Height = the text's height, so the box grows line by line; `.cd-o__nhap--grow`'s `max-height` caps it at ten
 * lines, past which it scrolls. "auto" first, or a box could never shrink back after text is deleted. A measure
 * that is not a number (no layout: tests, a server render) leaves the CSS height alone.
 */
export function fitToContent(box: HTMLTextAreaElement): void {
  box.style.height = "auto";
  const content = box.scrollHeight;
  const borders = box.offsetHeight - box.clientHeight;
  if (!Number.isFinite(content) || content <= 0) return;
  box.style.height = `${content + (Number.isFinite(borders) ? borders : 0)}px`;
}

/**
 * `autoFocus`: OPTIONAL and off unless a caller asks — only the commune app's send screen does, right after the
 * citizen tapped a field (`PROTOTYPE.md` §6.2). The shared app never passes it, so its screens are unchanged.
 * `autoGrow`: OPTIONAL, same single caller (owner, 08/10/2026): three lines to start, growing with the text up to
 * ten (`fitToContent`). The shared app's form and the rating keep their six-line box.
 */
export function ONhapDoan(
  props: ChungProps & {
    bat_buoc?: boolean;
    autoFocus?: boolean;
    autoGrow?: boolean;
    /** Lines the box starts with (default: 3 growing, 6 fixed). The prototype's writing box starts at 5. */
    rows?: number;
    /** Show the prototype's red "*" after the label of a required box. */
    requiredMark?: boolean;
  },
) {
  const id_goi_y = props.goi_y ? `${props.id}-goi-y` : undefined;
  const autoGrow = props.autoGrow === true;
  const box = useRef<HTMLTextAreaElement>(null);
  // A value set from outside (a restored draft, a step drawn again) is fitted too, not only typing.
  useEffect(() => {
    if (autoGrow && box.current !== null) fitToContent(box.current);
  }, [autoGrow, props.gia_tri]);
  return (
    <div className="cd-o">
      <Label id={props.id} nhan={props.nhan} hidden={props.labelHidden} requiredMark={props.requiredMark} />
      {props.goi_y && (
        <p className="cd-o__goi-y" id={id_goi_y}>
          {props.goi_y}
        </p>
      )}
      <textarea
        ref={box}
        className={autoGrow ? "cd-o__nhap cd-o__nhap--doan cd-o__nhap--grow" : "cd-o__nhap cd-o__nhap--doan"}
        id={props.id}
        autoComplete="off"
        maxLength={props.toi_da}
        rows={props.rows ?? (autoGrow ? 3 : 6)}
        required={props.bat_buoc}
        aria-required={props.bat_buoc}
        autoFocus={props.autoFocus === true}
        placeholder={props.placeholder}
        value={props.gia_tri}
        aria-describedby={id_goi_y}
        onChange={(e) => {
          // Fitted in the handler, before the next paint — an effect would draw one frame with a scrollbar.
          if (autoGrow) fitToContent(e.currentTarget);
          props.onDoi(e.target.value);
        }}
      />
    </div>
  );
}

/**
 * THE COMMUNE APP'S SEARCH BOX — the prototype's (`DirectoryPage.tsx:48-61`): a magnifier inside the field, the label
 * for screen readers only, the example as placeholder. The directory uses it (`DanhBaXa.tsx` `locCanBo`); any list
 * that filters on the phone can (the Phản ánh tab, TASK-04) — what is typed goes nowhere.
 *
 * Customer sheet App Xã 12/10/2026 row 13:
 *   · NO FLASH ON TAP. `type="text"`, not `"search"`: WebKit draws its own search-field look and cancel button on a
 *     `search` input, and the Zalo webview repainted it on focus. The keyboard still shows "search"
 *     (`inputMode`, `enterKeyHint`). The tap highlight is off in CSS; focus shows ONLY as the border colour
 *     (`styles.css` `.xa-search:focus-within`), as `.cd-o__nhap` does since row 4.
 *   · AN "X" AT THE RIGHT once something is typed: clears and puts the cursor back, so the citizen types again
 *     without a second tap. A 48px target (`.xa-search__clear`), named in words for a screen reader.
 */
export function OTimKiem(props: { id: string; nhan: string; placeholder: string; gia_tri: string; toi_da: number; onDoi: (v: string) => void }) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <div className="xa-search">
      <label className="xa-sr-only" htmlFor={props.id}>
        {props.nhan}
      </label>
      <span className="xa-search__icon" aria-hidden="true">
        <BieuTuong ten="search" co={22} />
      </span>
      <input
        ref={input}
        className="xa-search__input"
        id={props.id}
        type="text"
        inputMode="search"
        enterKeyHint="search"
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        maxLength={props.toi_da}
        placeholder={props.placeholder}
        value={props.gia_tri}
        onChange={(e) => props.onDoi(e.target.value)}
      />
      {props.gia_tri !== "" && (
        <button
          type="button"
          className="xa-search__clear"
          aria-label={XA_GIAO_DIEN.clear_search}
          onClick={() => {
            props.onDoi("");
            input.current?.focus();
          }}
        >
          <BieuTuong ten="x" co={20} />
        </button>
      )}
    </div>
  );
}

/**
 * THE BROADCAST BAR'S SEEK (owner 11/10/2026: the prototype's `<input type="range">`, `BroadcastPage.tsx:265`). It
 * lives in THIS file because it is an input control (`phase1-collects-nothing.test.ts`), though it takes no text:
 * the value is a position in seconds and never leaves the screen. Its name and its value are said in words
 * (`aria-label`, `aria-valuetext`), and the track is a full 44px touch target (`styles.css` `.xa-seek`).
 */
export function SeekBar(props: {
  label: string;
  valueText: string;
  max: number;
  value: number;
  disabled?: boolean;
  onSeek: (seconds: number) => void;
}) {
  return (
    <input
      className="xa-seek"
      type="range"
      min={0}
      max={Math.max(props.max, 0)}
      step={1}
      value={Math.min(Math.max(Math.round(props.value), 0), Math.max(props.max, 0))}
      disabled={props.disabled}
      aria-label={props.label}
      aria-valuetext={props.valueText}
      onChange={(e) => props.onSeek(Number(e.target.value))}
    />
  );
}
