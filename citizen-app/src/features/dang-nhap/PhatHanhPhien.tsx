/**
 * BƯỚC MÁY CHỦ CỦA KHỐI ĐĂNG NHẬP — đổi hai mã của Zalo lấy một phiên.
 *
 * ⚠ KHÔNG CÒN CỬA BIẾN THỂ Ở ĐÂY, VÀ VIỆC GỠ NÓ LÀ CÓ CHỦ ĐÍCH — 20/09/2026.
 *
 *   Bản đầu tiên của khối này đặt lời gọi máy chủ sau một cửa `bien-the/dang-nhap`, để bản nộp
 *   không gọi mạng. Tiền đề ấy đã đổi: **cả hai biến thể nay gọi máy chủ thật**, vì một nút
 *   đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút nói "bản này chưa nối máy chủ"
 *   (chính sách Mini App, điều 3.3.4).
 *
 *   Nên cửa ấy bị gỡ, chứ không giữ lại "cho chắc". Hai biến thể nói cùng một thứ là hai biến
 *   thể sẽ lệch nhau — đúng bài học của biến thể `quyen` đã ghi trong README. Hai cửa
 *   `bien-the/kham-pha` và `bien-the/chan-doan` thì GIỮ NGUYÊN: chúng vẫn có việc thật, vì bản
 *   nộp không được mang tên đơn vị hành chính nào.
 *
 * ⚠ CÁI KHÔNG ĐỔI: `phase1-collects-nothing.test.ts` vẫn cấm mọi lời gọi mạng ở MỌI TỆP, miễn
 * cho đúng một — `goi-may-chu.ts`. Tệp đang đọc nằm NGAY CẠNH nó và vẫn bị cấm; có một ca kiểm
 * cho lệnh cấm ăn một `fetch` đặt ở chính tệp này để chứng minh ngoại lệ hẹp đúng bằng một tệp.
 *
 * → ADR 0020 (một chạm, không OTP) · `hop-dong.ts` (khuôn của dây) · README §"Hai biến thể"
 */
import { useEffect, useState } from "react";

import type { MaDangNhap } from "../tinh-nang/zalo-api";

import { type KetQuaPhien, phatHanhPhien } from "./goi-may-chu";

/**
 * CHỮ CỦA BƯỚC MÁY CHỦ — nằm cạnh mã vẽ nó, không ở `noi-dung.ts`.
 *
 * `noi-dung.ts` giữ chữ của SÁU TÍNH NĂNG: đó là những câu người duyệt Zalo đọc để quyết định
 * cấp quyền, và chúng có một người chủ. Những câu dưới đây là trạng thái của một lời gọi mạng —
 * không ai đọc chúng để cấp quyền, và chúng đổi cùng nhịp với `goi-may-chu.ts` chứ không cùng
 * nhịp với bản mô tả quyền.
 *
 * KHÔNG CÂU NÀO NHẮC TÊN ĐƯỜNG DẪN, TÊN TRƯỜNG HAY MÃ TRẠNG THÁI. Người dùng cần biết việc đang
 * xảy ra và việc phải làm tiếp; một `401` trên màn hình là thứ README §Error message shape cấm.
 */
const CHU_PHIEN = {
  dang_gui: "Đang mở phiên đăng nhập…",
  xong: "Bạn đã đăng nhập.",
  giu_trong_bo_nho:
    "Phiên này chỉ nằm trong bộ nhớ của ứng dụng: nó không được ghi xuống máy bạn, và mất đi khi bạn đóng ứng dụng. Lần sau mở lại, bạn đăng nhập bằng một lần chạm như vừa rồi.",
  ma_het_han:
    "Mã đăng nhập của Zalo chỉ dùng được trong hai phút và đã quá hạn. Bạn hãy bấm lại nút đăng nhập bên trên để lấy mã mới.",
  // 502 — KHÁC HẲN `ma_het_han`, và khác ở đúng chỗ quan trọng: bấm lại ngay cũng hỏng y như
  // vậy. Câu này nói CHỜ rồi thử lại, và nói ra rằng lỗi không nằm ở phía người dùng.
  zalo_khong_tra_loi:
    "Zalo đang không trả lời phần xác thực, nên lần đăng nhập này chưa thành công. Việc này không do máy bạn. Bạn hãy chờ một lát rồi bấm lại nút đăng nhập bên trên.",
  // ⚠ KHÔNG ĐƯỢC MỞ ĐẦU BẰNG "Chưa mở…", DÙ ĐÓ LÀ CÂU TIẾNG VIỆT TỰ NHIÊN NHẤT Ở ĐÂY. `Chưa mở`
  // là nhãn của lớp khám phá (dịch vụ xã chưa mở), và `bundle-for-zalo.test.ts` khẳng định bản
  // NỘP không chứa một chuỗi nào của lớp ấy. Câu đầu tiên viết ở đây đã làm ca ấy đỏ — dây bẫy
  // hoạt động đúng như thiết kế. Ghi lại để lần sau không ai viết lại nó rồi đi tìm nguyên nhân.
  khong_goi_duoc:
    "Phiên đăng nhập chưa được tạo. Bạn hãy kiểm tra kết nối mạng rồi bấm lại nút đăng nhập bên trên.",
  // Câu này thực chất nói với NGƯỜI DỰNG BẢN — một bản dựng quên khai địa chỉ máy chủ. Nó vẫn
  // nói việc cần làm tiếp bằng tiếng Việt, không phải một mã lỗi. `scripts/deploy.mjs` chặn
  // đường đẩy khi biến chưa khai, nên câu này không được phép tới tay người dùng thật.
  chua_khai_host:
    "Bản dựng này chưa được khai địa chỉ máy chủ, nên chưa đăng nhập được. Người dựng bản cần đặt biến VIGOV_API_HOST rồi dựng lại.",
} as const;

/** Giờ hết hạn cho người đọc. Giá trị lạ thì bỏ hẳn dòng ấy, không hiện một chuỗi kỹ thuật. */
function gioHetHan(han: string): string | null {
  const luc = new Date(han);
  return Number.isNaN(luc.getTime()) ? null : luc.toLocaleString("vi-VN");
}

/**
 * Đổi hai mã lấy một phiên, ngay sau khi người dùng vừa đồng ý.
 *
 * MỘT CHẠM, KHÔNG PHẢI HAI: người dùng đã bấm "Đăng nhập" và đã đồng ý trên Zalo. Bắt họ bấm
 * thêm một nút "Gửi" nữa là thêm lại đúng cái rào mà ADR 0020 gỡ đi.
 *
 * ⚠ BEARER GIỮ TRONG `useState`, KHÔNG GHI XUỐNG MÁY VÀ KHÔNG VẼ RA MÀN HÌNH. Đây là chứng từ
 * của một phiên: ghi xuống máy là để nó ở lại sau khi người dùng đóng app — trên một thiết bị
 * có thể cho mượn — còn vẽ ra là mời người đứng cạnh chụp lại. Dây bẫy cấm
 * `localStorage`/`sessionStorage`/`cookie`/`indexedDB` không được nới cho tệp nào.
 *
 * ⚠ VÀ NÓ LÀ PHIẾU TẠM: ADR 0005 nói phiên được PHÁT HÀNH LẠI sau khi công dân chọn xã. Không
 * một dòng nào ở đây được dựng trên giả định "đăng nhập một lần rồi giữ mãi" — trạng thái sống
 * đúng bằng đời của khối này trên màn hình, và mất khi rời màn.
 */
export function PhatHanhPhien({ ma }: { ma: MaDangNhap }) {
  const [trang_thai, datTrangThai] = useState<KetQuaPhien | { kieu: "dang-gui" }>({
    kieu: "dang-gui",
  });

  useEffect(() => {
    // `con_tren_man`: màn hình có thể bị gỡ trước khi câu trả lời về. Đặt trạng thái cho một
    // component đã rời màn là một cảnh báo của React — không phải lỗi người dùng thấy, nhưng
    // là thứ che mất những cảnh báo thật.
    let con_tren_man = true;
    void phatHanhPhien(ma).then((ket_qua) => {
      if (con_tren_man) datTrangThai(ket_qua);
    });
    return () => {
      con_tren_man = false;
    };
  }, [ma]);

  if (trang_thai.kieu === "dang-gui") {
    return <p className="tn__ranh-gioi">{CHU_PHIEN.dang_gui}</p>;
  }

  if (trang_thai.kieu === "xong") {
    const gio = gioHetHan(trang_thai.phien.het_han);
    return (
      <>
        <p className="tn__xong">{CHU_PHIEN.xong}</p>
        {gio !== null && <p className="tn__do">Phiên có hiệu lực tới {gio}</p>}
        <p className="tn__giai-thich">{CHU_PHIEN.giu_trong_bo_nho}</p>
      </>
    );
  }

  // Bốn nhánh còn lại, mỗi nhánh một việc phải làm tiếp. Bảng tra thay cho một chuỗi `? :` lồng
  // nhau: thêm một nhánh ở `goi-may-chu.ts` mà quên câu chữ ở đây thì `tsc` đỏ, chứ không phải
  // một nhánh im lặng rơi vào câu chung chung.
  const CAU: Record<Exclude<KetQuaPhien["kieu"], "xong">, string> = {
    "ma-het-han": CHU_PHIEN.ma_het_han,
    "zalo-khong-tra-loi": CHU_PHIEN.zalo_khong_tra_loi,
    "chua-khai-host": CHU_PHIEN.chua_khai_host,
    "khong-goi-duoc": CHU_PHIEN.khong_goi_duoc,
  };

  return <p className="tn__loi">{CAU[trang_thai.kieu]}</p>;
}
