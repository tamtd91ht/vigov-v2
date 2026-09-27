/**
 * MỞ PHIÊN CÔNG DÂN ViGov SAU KHI CÔNG DÂN XÁC NHẬN XÃ — qua một hàm TIÊM VÀO, không qua một đường nhập.
 *
 * VÌ SAO TIÊM (dependency injection), KHÔNG NHẬP:
 *
 *   Phiên ViGov chỉ mở được qua `vihat-miniapp` (`POST /api/v1/sessions` với `communeHostHint`,
 *   `communeConfirmed` — ADR 0045 · 0047 câu 7), bằng `accessToken` của `zmp-sdk`. `zmp-sdk` chỉ
 *   được nhập trong `features/tinh-nang/`, và nửa nhà nước KHÔNG được nhập nửa thương mại
 *   (`ranh-gioi-hai-nua.test.ts` §3a, `cong-dan.test.tsx`). Nên nửa này chỉ KHAI hình dạng của hàm
 *   nó cần (`MoPhienViGov`); `App.tsx` — lớp vỏ, được nhập cả hai nửa — dựng hàm ấy từ client đăng
 *   nhập thương mại và truyền xuống. Không cạnh nhập cấm nào xuất hiện.
 *
 * ⚠ XÃ CHỈ VÀO PHIÊN BẰNG HÀNH VI XÁC NHẬN (ADR 0047 điều kiện dừng #4). `communeConfirmed` là hằng
 *   `true` ở đây vì hàm này CHỈ được gọi từ nút "Đúng, tiếp tục" — không có đường nào gọi nó khi mở app.
 *
 * ⚠ TÊN XÃ CỦA PHIÊN LÀ TÊN MÁY CHỦ TRẢ CÙNG PHIÊN, không phải tên màn xác nhận vừa hiện (ADR 0047
 *   §Trả lời mục 4). Hai tên có thể khác nhau trong một ca hiếm đã được chấp nhận; khi ấy phiên nói thật.
 *
 * ⚠ BEARER KHÔNG ĐI NGƯỢC LÊN. Kết quả trả cho màn hình chỉ có TÊN XÃ; bearer vào `phien-vigov.ts` và
 *   chỉ `goi-vigov.ts` đọc nó ra.
 */
import { datPhienViGov } from "./phien-vigov";
import { laTenMien } from "../../lib/launch-params";

/** Thứ nửa nhà nước gửi cho hàm mở phiên. Tên trường là tên trên dây của `vihat-miniapp`. */
export type YeuCauMoPhien = {
  readonly communeHostHint: string;
  readonly communeConfirmed: true;
};

/**
 * Bốn nhánh, mỗi nhánh một việc người dân làm tiếp:
 *
 *   `xong`        có phiên ViGov (bearer + tên xã)
 *   `chua-mo`     cầu phiên tắt, xã chưa sẵn sàng, hoặc máy chủ trả một phiên KHÔNG có bearer —
 *                 bấm lại không đổi được gì; về phần giới thiệu kèm một câu
 *   `thu-lai`     mạng, Zalo, hoặc máy chủ bận — bấm lại có thể được
 *   `ngoai-zalo`  không chạy trong Zalo nên không có mã phiên Zalo nào
 */
export type KetQuaMoPhien =
  | {
      readonly kieu: "xong";
      readonly token: string;
      readonly ten_xa: string;
      /**
       * Tên miền công khai chính của xã CỦA PHIÊN (`communePrimaryHost`), hoặc `null`. Chỉ làm khoá tra
       * `?host=` cho tin tức và danh bạ — không vào `phien-vigov.ts`, không gửi đi làm "xã của tôi".
       */
      readonly ten_mien: string | null;
    }
  | { readonly kieu: "chua-mo" }
  | { readonly kieu: "thu-lai" }
  | { readonly kieu: "ngoai-zalo" };

export type MoPhienViGov = (yc: YeuCauMoPhien) => Promise<KetQuaMoPhien>;

/** Kết quả cho màn hình — KHÔNG mang bearer. */
export type KetQuaXacNhan =
  | { kieu: "da-mo"; ten_xa: string; ten_mien: string | null }
  | { kieu: "chua-mo" }
  | { kieu: "thu-lai" }
  | { kieu: "ngoai-zalo" };

/**
 * Gọi hàm mở phiên đã tiêm với tên miền công dân vừa xác nhận, rồi ghi phiên vào bộ nhớ.
 *
 * Không ném ra ngoài: một hàm tiêm vào ném lỗi là `thu-lai`, không phải một màn hình đứng im. Một
 * `xong` thiếu bearer hoặc thiếu tên xã là `chua-mo` — KHÔNG BAO GIỜ giả vờ đã có phiên.
 */
export async function moPhienSauXacNhan(mo: MoPhienViGov, ten_mien: string): Promise<KetQuaXacNhan> {
  let kq: KetQuaMoPhien;
  try {
    kq = await mo({ communeHostHint: ten_mien, communeConfirmed: true });
  } catch {
    return { kieu: "thu-lai" };
  }
  if (kq.kieu !== "xong") return { kieu: kq.kieu };
  if (kq.token === "" || kq.ten_xa.trim() === "") return { kieu: "chua-mo" };
  datPhienViGov({ token: kq.token, ten_xa: kq.ten_xa });
  // KIỂM KHUÔN LẠI Ở ĐÂY dù nửa thương mại đã kiểm: hàm này nhận từ BÊN NGOÀI nửa nhà nước, và một tên
  // miền sai khuôn đi vào `?host=` là để máy chủ phân tích một thứ không phải tên miền.
  const ten_mien_phien = typeof kq.ten_mien === "string" && laTenMien(kq.ten_mien) ? kq.ten_mien : null;
  return { kieu: "da-mo", ten_xa: kq.ten_xa, ten_mien: ten_mien_phien };
}
