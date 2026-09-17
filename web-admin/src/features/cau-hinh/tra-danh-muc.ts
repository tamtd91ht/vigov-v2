/**
 * Tra một id trong danh mục ra tên người đọc được — bộ phận và vai trò dùng chung một phép tra.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO LÀ HÀM THUẦN NHẬN SẴN BẢNG TRA, KHÔNG PHẢI HÀM TỰ ĐI HỎI MÁY CHỦ:
 *
 * Chữ ký của `traTen` không có chỗ nào để gọi mạng, nên mẫu "mỗi dòng danh bạ một lời gọi"
 * không viết ra được kể cả khi ai đó muốn. Danh mục đọc MỘT lần cho cả màn hình
 * (`lib/api/danh-muc.ts`), rồi bảng tra đi xuống từng dòng như một tham số.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * NĂM KẾT QUẢ, KHÔNG PHẢI HAI, và bốn trong năm ca ấy KHÔNG phải lỗi lập trình — chúng là
 * những trạng thái có thật, khác nhau, và gộp bất kỳ hai cái nào cũng là báo sai một sự thật
 * về một cán bộ:
 *
 *   `dangDoc`        chưa có câu trả lời nào. Không được trông giống "chưa gán".
 *   `chuaGan`        id rỗng — người này CHƯA được phân bộ phận / chưa được gán vai trò.
 *   `coTen`          tra được.
 *   `khongTraDuoc`   id có thật trên dòng danh bạ nhưng danh mục không cho ra tên nào. Ca hay
 *                    gặp nhất là bộ phận đã bị xoá mềm (danh mục lọc `deleted_at IS NULL`) mà
 *                    dòng danh bạ vẫn trỏ tới; một mục có tên rỗng cũng rơi vào đây. Đây là dữ
 *                    liệu lệch, và người quản trị là người duy nhất sửa được — nên màn hình
 *                    phải NÓI RA, không được để ô trống. Một ô trống trông y hệt "chưa phân bộ
 *                    phận", mà hai thứ ấy cần hai hành động khác nhau. Câu chữ cố ý KHÔNG nói
 *                    "đã bị xoá": phía web chỉ biết là không tra được, không biết vì sao.
 *   `khongCoDanhMuc` không đọc được chính danh mục. Lúc này KHÔNG id nào tra được, nên nói
 *                    "không có trong danh mục" về từng dòng là vu cho hai mươi dòng cùng một
 *                    lỗi dữ liệu không ai gây ra.
 */

import type { KetQua } from "@/lib/api/goi";

/** Bảng tra id → tên của MỘT danh mục, ở một trong ba pha đọc. */
export type BangTraDanhMuc =
  /** Chưa đọc xong. Khác hẳn "đọc xong và hỏng" — ba pha, không hai. */
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; ten: ReadonlyMap<string, string> };

export type KetTra =
  | { loai: "dangDoc" }
  | { loai: "chuaGan" }
  | { loai: "coTen"; ten: string }
  | { loai: "khongTraDuoc" }
  | { loai: "khongCoDanhMuc" };

/**
 * Dựng bảng tra từ kết quả đọc một danh mục.
 *
 * Nhận `KetQua` chứ không nhận mảng: nhánh hỏng phải đi cùng bảng tra chứ không rụng lại ở chỗ
 * gọi, nếu không thì "danh mục hỏng" và "danh mục rỗng" đến đây thành một — mà một xã vừa
 * onboard có danh mục rỗng thật.
 *
 * Kiểu tham số cố ý chỉ đòi `id` và `name`: hai danh mục có hình dạng khác nhau (bộ phận có
 * `parent_id`, vai trò có `is_leader`) và phép tra này không quan tâm, nên nó không nhắc tới
 * trường nào ngoài hai trường nó thật sự đọc.
 */
export function bangTraTuKetQua(
  kq: KetQua<{ items: readonly { id: string; name: string }[] }> | null,
): BangTraDanhMuc {
  if (kq === null) return { pha: "dangDoc" };
  if (!kq.ok) return { pha: "loi", thongBao: kq.thongBao };

  const ten = new Map<string, string>();
  for (const muc of kq.duLieu.items) ten.set(muc.id, muc.name);
  return { pha: "xong", ten };
}

/**
 * Tra một id. Hoàn toàn đồng bộ — không mạng, không hiệu ứng phụ, gọi được trong lúc dựng dòng.
 *
 * Thứ tự hai phép kiểm đầu là có chủ ý: id rỗng được trả lời là "chưa gán" NGAY CẢ KHI danh mục
 * chưa đọc xong hoặc đọc hỏng. "Người này chưa được phân bộ phận" là điều đã biết chắc từ chính
 * dòng danh bạ; nó không cần danh mục nào để khẳng định, và bắt người đọc chờ một lời gọi mạng
 * để biết là thừa.
 */
export function traTen(bang: BangTraDanhMuc, id: string): KetTra {
  if (id === "") return { loai: "chuaGan" };
  if (bang.pha === "dangDoc") return { loai: "dangDoc" };
  if (bang.pha === "loi") return { loai: "khongCoDanhMuc" };

  const ten = bang.ten.get(id);
  if (ten === undefined || ten === "") return { loai: "khongTraDuoc" };
  return { loai: "coTen", ten };
}
