/**
 * LỚP KHÁM PHÁ NÓI CHUYỆN VỚI MÁY CHỦ — tra tên xã theo tên miền trên QR, hỏi công dân, rồi mở phiên.
 *
 * VÌ SAO Ở `cong-dan/`, KHÔNG Ở `features/kham-pha/`: `ranh-gioi-hai-nua.test.ts` (khối chú thích
 * trên `KHU_VUC`) cho lớp khám phá đứng ở vùng trung lập CHỈ CHỪNG NÀO nó chưa gọi máy chủ. Phần gọi
 * máy chủ sinh ra ở đây, trong nửa nhà nước, và dùng lại màn xác nhận thuần của lớp khám phá
 * (`GoiYXaScreen`) cùng quy tắc mức tin (`phanGiaiGoiY`) — hai thứ ấy vẫn không gọi gì.
 *
 * LUỒNG (ADR 0047 §Trả lời mục 4):
 *
 *   1. `GET identity /api/v1/communes?host=<d>` — tên và tỉnh. Rỗng / 400 / 503 / mất mạng → FAIL
 *      CLOSED: về phần giới thiệu kèm một câu. Không bao giờ hiện một cái tên dựng từ tham số.
 *   2. Công dân bấm "Đúng, tiếp tục" → hàm mở phiên TIÊM VÀO (`api/mo-phien-vigov.ts`) với
 *      `communeHostHint=<d>`, `communeConfirmed=true`.
 *   3. Có phiên → báo lên với TÊN XÃ CỦA PHIÊN. Không có (cầu tắt, phiên không bearer, ngoài Zalo) →
 *      báo lên "đã xác nhận, không phiên" với tên + tỉnh `/communes` vừa trả: hai màn công khai mở,
 *      gửi phản ánh thì không. Không bao giờ giả vờ đã có phiên.
 *
 * ⚠ TÊN MIỀN KHÔNG ĐƯỢC VẼ RA, KHÔNG GHI LOG. Nó lộ người này đang làm việc với xã nào.
 */
import { useEffect, useRef, useState } from "react";

import { type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import type { XaTraDuoc } from "../api/hop-dong-cong-khai";
import { type KetQuaXacNhan, type MoPhienViGov, moPhienSauXacNhan } from "../api/mo-phien-vigov";

import { confirmCommuneZaloFailed, XAC_NHAN_XA, zaloFailureSentence } from "./noi-dung";
import { GoiYXaScreen, phanGiaiGoiY, type XaGoiY } from "../../features/kham-pha";

/** Trạng thái của màn, THUẦN — test dựng thẳng từng bước mà không cần DOM. */
export type TrangXacNhan =
  | { readonly kieu: "dang-tra" }
  | { readonly kieu: "hoi"; readonly xa: XaGoiY; readonly cau_loi?: string }
  | { readonly kieu: "dang-mo"; readonly xa: XaGoiY };

/**
 * Cách lớp khám phá KẾT THÚC — thứ duy nhất đi lên `App.tsx`. Không có bearer.
 *
 *   `da-mo`                 có phiên ViGov; `ten_xa` là tên máy chủ trả CÙNG phiên; `ten_mien` là
 *                           `communePrimaryHost` của phiên (hoặc `null`)
 *   `xac-nhan-khong-phien`  công dân ĐÃ bấm xác nhận, nhưng không mở được phiên (cầu tắt, xã chưa
 *                           sẵn sàng, ngoài Zalo). `xa` là tên + tỉnh `/communes` vừa trả — đủ để mở
 *                           hai màn công khai (27/09/2026, quyết định của chủ sản phẩm); gửi phản ánh
 *                           vẫn cần phiên
 *   `ve-gioi-thieu`         về phần giới thiệu; `cau` là câu nói vì sao (hoặc `null` khi công dân tự
 *                           bấm "Không phải xã này" — họ biết vì sao)
 *
 * Tên miền `d` KHÔNG đi lên ở đây: `App.tsx` đã có nó (chính nó truyền `ten_mien` xuống màn này).
 */
export type KetThucXacNhan =
  | { readonly kieu: "da-mo"; readonly ten_xa: string; readonly ten_mien: string | null }
  | { readonly kieu: "xac-nhan-khong-phien"; readonly xa: XaGoiY }
  | { readonly kieu: "ve-gioi-thieu"; readonly cau: string | null };

export type Buoc = { readonly trang: TrangXacNhan } | { readonly ket_thuc: KetThucXacNhan };

/**
 * Kết quả tra xã → bước kế. Đúng MỘT xã, tên không rỗng, nguồn đủ tin → hỏi. Mọi thứ khác → về giới
 * thiệu. `phanGiaiGoiY` giữ quy tắc mức tin; ở đây không viết lại nó.
 */
export function buocSauTraXa(kq: KetQuaCongKhai<readonly XaTraDuoc[]>, nguon: string): Buoc {
  if (kq.kieu === "xong") {
    const goi_y = phanGiaiGoiY(nguon, kq.gia_tri.length === 1 ? kq.gia_tri[0]! : null);
    if (goi_y.kieu === "chon-san") return { trang: { kieu: "hoi", xa: goi_y.xa } };
    return { ket_thuc: { kieu: "ve-gioi-thieu", cau: XAC_NHAN_XA.khong_thay } };
  }
  if (kq.kieu === "khong-hop-le" || kq.kieu === "khong-thay") {
    return { ket_thuc: { kieu: "ve-gioi-thieu", cau: XAC_NHAN_XA.khong_thay } };
  }
  return { ket_thuc: { kieu: "ve-gioi-thieu", cau: XAC_NHAN_XA.chua_ket_noi } };
}

/**
 * Kết quả mở phiên → bước kế. Chỉ `thu-lai` ở lại màn xác nhận: đó là nhánh duy nhất bấm lại có ích.
 *
 * `chua-mo` / `ngoai-zalo` KHÔNG còn về phần giới thiệu (27/09/2026): công dân đã xác nhận đúng xã, nên
 * tin tức và danh bạ — hai tuyến công khai theo tên miền — mở ngay, không cần phiên. `xa` là thứ
 * `/communes` đã trả và công dân vừa đọc trên màn này, không phải một tên dựng từ tham số.
 */
export function buocSauMoPhien(xa: XaGoiY, kq: KetQuaXacNhan): Buoc {
  switch (kq.kieu) {
    case "da-mo":
      return { ket_thuc: { kieu: "da-mo", ten_xa: kq.ten_xa, ten_mien: kq.ten_mien } };
    case "thu-lai":
      // Zalo refused the session code with a code: say which, and whether pressing again can help.
      return {
        trang: {
          kieu: "hoi",
          xa,
          cau_loi:
            kq.zalo === undefined
              ? XAC_NHAN_XA.thu_lai
              : confirmCommuneZaloFailed(zaloFailureSentence(kq.zalo), kq.zalo.transient),
        },
      };
    case "ngoai-zalo":
    case "chua-mo":
      return { ket_thuc: { kieu: "xac-nhan-khong-phien", xa } };
  }
}

/** Thân màn, THUẦN. */
export function ManXacNhanXa(props: {
  trang: TrangXacNhan;
  nguon: string;
  onXacNhan: () => void;
  onKhongPhai: () => void;
}) {
  const { trang } = props;
  if (trang.kieu === "dang-tra") {
    return (
      <section className="goi-y" aria-busy="true">
        <p className="goi-y__tiep" role="status">
          {XAC_NHAN_XA.dang_tra}
        </p>
      </section>
    );
  }
  return (
    <GoiYXaScreen
      xa={trang.xa}
      nguon={props.nguon}
      onXacNhan={props.onXacNhan}
      onKhongPhai={props.onKhongPhai}
      cau_dang_mo={trang.kieu === "dang-mo" ? XAC_NHAN_XA.dang_mo : undefined}
      cau_loi={trang.kieu === "hoi" ? trang.cau_loi : undefined}
    />
  );
}

export function XacNhanXa(props: {
  /** Tên miền `d` đã qua khuôn của `lib/launch-params.ts`. Chỉ làm khoá tra và gợi ý cho cầu phiên. */
  ten_mien: string;
  nguon: string;
  /** Hàm mở phiên do `App.tsx` dựng từ client đăng nhập — xem `api/mo-phien-vigov.ts`. */
  moPhienViGov: MoPhienViGov;
  onKetThuc: (kq: KetThucXacNhan) => void;
}) {
  const { ten_mien, nguon, moPhienViGov, onKetThuc } = props;
  const [trang, datTrang] = useState<TrangXacNhan>({ kieu: "dang-tra" });
  // Tra ĐÚNG MỘT LẦN, kể cả khi React chạy hiệu ứng hai lần (StrictMode).
  const da_tra = useRef(false);

  function sang(b: Buoc) {
    if ("ket_thuc" in b) onKetThuc(b.ket_thuc);
    else datTrang(b.trang);
  }

  useEffect(() => {
    if (da_tra.current) return;
    da_tra.current = true;
    void traXaTheoTenMien(ten_mien).then((kq) => sang(buocSauTraXa(kq, nguon)));
    // Tên miền và nguồn đọc MỘT LẦN lúc mở app; không có gì đổi chúng giữa chừng.
  }, []);

  function xacNhan() {
    if (trang.kieu !== "hoi") return;
    const xa = trang.xa;
    datTrang({ kieu: "dang-mo", xa });
    void moPhienSauXacNhan(moPhienViGov, ten_mien).then((kq) => sang(buocSauMoPhien(xa, kq)));
  }

  return (
    <ManXacNhanXa
      trang={trang}
      nguon={nguon}
      onXacNhan={xacNhan}
      onKhongPhai={() => onKetThuc({ kieu: "ve-gioi-thieu", cau: null })}
    />
  );
}
