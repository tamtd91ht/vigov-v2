/**
 * LỚP KHÁM PHÁ NÓI CHUYỆN VỚI MÁY CHỦ — tra tên xã theo tên miền trên QR, hỏi công dân, rồi mở phiên.
 *
 * VÌ SAO Ở `citizen/`, KHÔNG Ở `features/kham-pha/`: `two-halves-boundary.test.ts` (khối chú thích
 * trên `ZONES`) cho lớp khám phá đứng ở vùng trung lập CHỈ CHỪNG NÀO nó chưa gọi máy chủ. Phần gọi
 * máy chủ sinh ra ở đây, trong nửa nhà nước, và dùng lại màn xác nhận thuần của lớp khám phá
 * (`GoiYXaScreen`) cùng quy tắc mức tin (`phanGiaiGoiY`) — hai thứ ấy vẫn không gọi gì.
 *
 * LUỒNG (ADR 0047 §Trả lời mục 4):
 *
 *   1. `GET identity /api/v1/communes?host=<d>` — tên và tỉnh. Rỗng / 400 / 503 / mất mạng → FAIL
 *      CLOSED: về phần giới thiệu kèm một câu. Không bao giờ hiện một cái tên dựng từ tham số.
 *   2. Công dân bấm "Đúng, tiếp tục" → hàm mở phiên TIÊM VÀO (`api/open-vigov-session.ts`) với
 *      `communeHostHint=<d>`, `communeConfirmed=true`.
 *   3. Có phiên → báo lên với TÊN XÃ CỦA PHIÊN. Không có (cầu tắt, phiên không bearer, ngoài Zalo) →
 *      báo lên "đã xác nhận, không phiên" với tên + tỉnh `/communes` vừa trả: hai màn công khai mở,
 *      gửi phản ánh thì không. Không bao giờ giả vờ đã có phiên.
 *
 * ⚠ TÊN MIỀN KHÔNG ĐƯỢC VẼ RA, KHÔNG GHI LOG. Nó lộ người này đang làm việc với xã nào.
 */
import { useEffect, useRef, useState } from "react";

import { type PublicResult, lookupCommuneByDomain } from "../api/vigov-client";
import type { FoundCommune } from "../api/public-contract";
import { type ConfirmationResult, type OpenVigovSession, openSessionAfterConfirmation } from "../api/open-vigov-session";

import { COMMUNE_CONFIRMATION } from "./copy";
import { GoiYXaScreen, phanGiaiGoiY, type XaGoiY } from "../../features/kham-pha";

/** Trạng thái của màn, THUẦN — test dựng thẳng từng bước mà không cần DOM. */
export type ConfirmationState =
  | { readonly kind: "dang-tra" }
  | { readonly kind: "hoi"; readonly commune: XaGoiY; readonly error_text?: string }
  | { readonly kind: "dang-mo"; readonly commune: XaGoiY };

/**
 * Cách lớp khám phá KẾT THÚC — thứ duy nhất đi lên `App.tsx`. Không có bearer.
 *
 *   `da-mo`                 có phiên ViGov; `commune_name` là tên máy chủ trả CÙNG phiên; `domain` là
 *                           `communePrimaryHost` của phiên (hoặc `null`)
 *   `xac-nhan-khong-phien`  công dân ĐÃ bấm xác nhận, nhưng không mở được phiên (cầu tắt, xã chưa
 *                           sẵn sàng, ngoài Zalo). `commune` là tên + tỉnh `/communes` vừa trả — đủ để mở
 *                           hai màn công khai (27/09/2026, quyết định của chủ sản phẩm); gửi phản ánh
 *                           vẫn cần phiên
 *   `ve-gioi-thieu`         về phần giới thiệu; `text` là câu nói vì sao (hoặc `null` khi công dân tự
 *                           bấm "Không phải xã này" — họ biết vì sao)
 *
 * Tên miền `d` KHÔNG đi lên ở đây: `App.tsx` đã có nó (chính nó truyền `domain` xuống màn này).
 */
export type ConfirmationOutcome =
  | { readonly kind: "da-mo"; readonly commune_name: string; readonly domain: string | null }
  | { readonly kind: "xac-nhan-khong-phien"; readonly commune: XaGoiY }
  | { readonly kind: "ve-gioi-thieu"; readonly text: string | null };

export type Step = { readonly page: ConfirmationState } | { readonly outcome: ConfirmationOutcome };

/**
 * Kết quả tra xã → bước kế. Đúng MỘT xã, tên không rỗng, nguồn đủ tin → hỏi. Mọi thứ khác → về giới
 * thiệu. `phanGiaiGoiY` giữ quy tắc mức tin; ở đây không viết lại nó.
 */
export function stepAfterCommuneLookup(result: PublicResult<readonly FoundCommune[]>, source: string): Step {
  if (result.kind === "xong") {
    const suggestion = phanGiaiGoiY(source, result.value.length === 1 ? result.value[0]! : null);
    if (suggestion.kieu === "chon-san") return { page: { kind: "hoi", commune: suggestion.xa } };
    return { outcome: { kind: "ve-gioi-thieu", text: COMMUNE_CONFIRMATION.not_found } };
  }
  if (result.kind === "khong-hop-le" || result.kind === "khong-thay") {
    return { outcome: { kind: "ve-gioi-thieu", text: COMMUNE_CONFIRMATION.not_found } };
  }
  return { outcome: { kind: "ve-gioi-thieu", text: COMMUNE_CONFIRMATION.not_connected } };
}

/**
 * Kết quả mở phiên → bước kế. Chỉ `thu-lai` ở lại màn xác nhận: đó là nhánh duy nhất bấm lại có ích.
 *
 * `chua-mo` / `ngoai-zalo` KHÔNG còn về phần giới thiệu (27/09/2026): công dân đã xác nhận đúng xã, nên
 * tin tức và danh bạ — hai tuyến công khai theo tên miền — mở ngay, không cần phiên. `commune` là thứ
 * `/communes` đã trả và công dân vừa đọc trên màn này, không phải một tên dựng từ tham số.
 */
export function stepAfterSessionOpen(commune: XaGoiY, result: ConfirmationResult): Step {
  switch (result.kind) {
    case "da-mo":
      return { outcome: { kind: "da-mo", commune_name: result.commune_name, domain: result.domain } };
    case "thu-lai":
      return { page: { kind: "hoi", commune, error_text: COMMUNE_CONFIRMATION.retry } };
    case "ngoai-zalo":
    case "chua-mo":
      return { outcome: { kind: "xac-nhan-khong-phien", commune } };
  }
}

/** Thân màn, THUẦN. */
export function CommuneConfirmationScreen(props: {
  page: ConfirmationState;
  source: string;
  onConfirm: () => void;
  onNotThis: () => void;
}) {
  const { page } = props;
  if (page.kind === "dang-tra") {
    return (
      <section className="goi-y" aria-busy="true">
        <p className="goi-y__tiep" role="status">
          {COMMUNE_CONFIRMATION.looking_up}
        </p>
      </section>
    );
  }
  return (
    <GoiYXaScreen
      xa={page.commune}
      nguon={props.source}
      onXacNhan={props.onConfirm}
      onKhongPhai={props.onNotThis}
      cau_dang_mo={page.kind === "dang-mo" ? COMMUNE_CONFIRMATION.opening : undefined}
      cau_loi={page.kind === "hoi" ? page.error_text : undefined}
    />
  );
}

export function CommuneConfirmation(props: {
  /** Tên miền `d` đã qua khuôn của `lib/launch-params.ts`. Chỉ làm khoá tra và gợi ý cho cầu phiên. */
  domain: string;
  source: string;
  /** Hàm mở phiên do `App.tsx` dựng từ client đăng nhập — xem `api/open-vigov-session.ts`. */
  openVigovSession: OpenVigovSession;
  onDone: (result: ConfirmationOutcome) => void;
}) {
  const { domain, source, openVigovSession, onDone } = props;
  const [page, setPage] = useState<ConfirmationState>({ kind: "dang-tra" });
  // Tra ĐÚNG MỘT LẦN, kể cả khi React chạy hiệu ứng hai lần (StrictMode).
  const looked_up = useRef(false);

  function goTo(b: Step) {
    if ("outcome" in b) onDone(b.outcome);
    else setPage(b.page);
  }

  useEffect(() => {
    if (looked_up.current) return;
    looked_up.current = true;
    void lookupCommuneByDomain(domain).then((result) => goTo(stepAfterCommuneLookup(result, source)));
    // Tên miền và nguồn đọc MỘT LẦN lúc mở app; không có gì đổi chúng giữa chừng.
  }, []);

  function confirm() {
    if (page.kind !== "hoi") return;
    const commune = page.commune;
    setPage({ kind: "dang-mo", commune });
    void openSessionAfterConfirmation(openVigovSession, domain).then((result) => goTo(stepAfterSessionOpen(commune, result)));
  }

  return (
    <CommuneConfirmationScreen
      page={page}
      source={source}
      onConfirm={confirm}
      onNotThis={() => onDone({ kind: "ve-gioi-thieu", text: null })}
    />
  );
}
