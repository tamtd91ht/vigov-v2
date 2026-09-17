"use client";

import { useCallback, useState, useSyncExternalStore } from "react";

import { dangXuat } from "@/lib/api/phien";

import { docSid, xoaSid } from "./sid-phien";

/**
 * Nút `Đăng xuất` ở khối người dùng — `15-phu-luc-giao-dien-chung` §3.3.
 *
 * KHI KHÔNG CÓ `sid`: hợp đồng không có route nào kết thúc "phiên hiện tại" mà không cần `sid`,
 * và cũng không có route nào hỏi lại `sid` của phiên đang mang. Nên ở ca này giao diện nói
 * thẳng ra thay vì hiện một cái nút bấm vào không xảy ra gì — đăng nhập lại sẽ cấp một `sid`
 * mới. Xem `sid-phien.ts` và phần lỗ hổng hợp đồng trong báo cáo.
 */

/**
 * `sessionStorage` là một kho NGOÀI React. `useSyncExternalStore` là cách React đọc một kho như
 * vậy mà vẫn đúng khi trang được dựng sẵn ở máy chủ: bản chụp phía máy chủ luôn là `null`, nên
 * HTML gửi đi không bao giờ chứa `sid` của ai cả.
 *
 * Không đăng ký lắng nghe gì: `sessionStorage` không phát sự kiện cho chính tab đang ghi, và
 * `sid` chỉ đổi đúng hai lần trong đời một tab — lúc đăng nhập và lúc đăng xuất, cả hai đều kéo
 * theo một lần tải lại trang.
 */
const KHONG_LANG_NGHE = () => () => {};
const SID_O_MAY_CHU = () => null;

export function NutDangXuat() {
  const docSidOnDinh = useCallback(() => docSid(), []);
  const sid = useSyncExternalStore(KHONG_LANG_NGHE, docSidOnDinh, SID_O_MAY_CHU);

  const [dangGui, datDangGui] = useState(false);
  const [thongBaoLoi, datThongBaoLoi] = useState<string | null>(null);

  async function bam() {
    if (sid === null || dangGui) return;
    datDangGui(true);
    datThongBaoLoi(null);

    const ketQua = await dangXuat(sid);
    if (!ketQua.ok) {
      datThongBaoLoi(ketQua.thongBao);
      datDangGui(false);
      return;
    }

    xoaSid();
    // Cookie đã bị máy chủ xoá bằng Set-Cookie trong chính phản hồi 204. Ở đây rời trang bằng
    // CẢ TRANG, không phải điều hướng phía client.
    //
    // ESLint của Next đề nghị `useRouter().push()` cho đường nội bộ, và ở phần lớn ứng dụng thì
    // đúng. Ở nút đăng xuất thì không: điều hướng phía client giữ nguyên tiến trình, tức là giữ
    // nguyên mọi thứ người vừa đăng xuất còn để lại trong bộ nhớ trang — trên một máy dùng chung
    // ở bộ phận một cửa, người kế tiếp ngồi vào chính cái máy đó. Tải lại cả trang là cách duy
    // nhất bỏ sạch số ấy.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.assign("/dang-nhap");
  }

  if (sid === null) {
    return (
      <div className="khoi-dang-xuat">
        <a className="nut-phu" href="/dang-nhap">
          Đăng nhập lại
        </a>
        <p className="ghi-chu">Không xác định được phiên trên thiết bị này.</p>
      </div>
    );
  }

  return (
    <div className="khoi-dang-xuat">
      <button type="button" className="nut-phu" onClick={bam} disabled={dangGui} aria-busy={dangGui}>
        {dangGui ? "Đang đăng xuất…" : "Đăng xuất"}
      </button>
      {thongBaoLoi !== null && (
        <p className="thong-bao-loi" role="alert" aria-live="assertive">
          {thongBaoLoi}
        </p>
      )}
    </div>
  );
}
