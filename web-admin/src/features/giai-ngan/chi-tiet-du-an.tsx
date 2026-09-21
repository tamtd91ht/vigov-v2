"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { layChiTietDuAn } from "@/lib/api/du-an";
import type { finance_duAnRa } from "@/lib/api/schema.gen";

import {
  CANH_BAO_KHONG_PHAI_KE_TOAN,
  lopTienDo,
  nhanNgay,
  nhanTien,
  nhanTienDo,
  nhanTyLeGiaiNgan,
  tienDoDuAn,
} from "./nhan-du-an";

/**
 * Trang chi tiết một dự án — `docs/ui-ux/06-giai-ngan.md §8`.
 *
 * BỐN TAB CỦA ĐẶC TẢ (Vướng mắc · Chứng từ · Biểu đồ · Trao đổi) KHÔNG CÓ Ở ĐÂY: không tab nào
 * có tuyến phía sau trong hợp đồng REST. Một thanh bốn tab mà bấm vào không ra gì là bốn lần hứa
 * suông với cán bộ — thanh tab chỉ mọc khi tuyến mọc.
 *
 * KHỐI "GIẢI NGÂN THEO NGUỒN VỐN" CŨNG KHÔNG: `phan_bo_nguon_von` chưa có tuyến nào, và một khối
 * rỗng gắn nhãn "0 đ / 0 đ" đọc thành "xã chưa gắn nguồn nào" — một khẳng định về dữ liệu của xã
 * mà màn hình này không có căn cứ để đưa ra.
 *
 * KHÔNG CÓ VẠCH "THỜI GIAN ĐÃ TRÔI QUA" (§8, thanh tiến độ). Con số ấy là một phép tính trên
 * đồng hồ và trên biên của năm ngân sách; máy chủ tính nó để ra `delay_score` nhưng KHÔNG gửi
 * nó về. Dựng lại phép tính ở trình duyệt là một bản thứ hai đọc đồng hồ của MÁY CÁN BỘ, và nó
 * sẽ lệch bản của máy chủ đúng vào hai đầu năm — thiếu ở hợp đồng, đã báo lên, không vá tạm.
 */

type TrangThaiChiTiet =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duAn: finance_duAnRa };

export function ChiTietDuAn({ id }: { id: string }) {
  const [trangThai, datTrangThai] = useState<TrangThaiChiTiet>({ pha: "dangTai" });

  useEffect(() => {
    let bo = false;
    layChiTietDuAn(id).then((kq) => {
      if (bo) return;
      datTrangThai(kq.ok ? { pha: "xong", duAn: kq.duLieu } : { pha: "loi", thongBao: kq.thongBao });
    });
    return () => {
      bo = true;
    };
  }, [id]);

  return (
    <section className="man-giai-ngan" aria-labelledby="tieu-de-chi-tiet-du-an">
      <p className="duong-lui">
        <Link href="/giai-ngan">← Theo dõi giải ngân</Link>
      </p>
      <h2 id="tieu-de-chi-tiet-du-an">Chi tiết dự án</h2>
      <p className="canh-bao-pham-vi">{CANH_BAO_KHONG_PHAI_KE_TOAN}</p>

      {trangThai.pha === "dangTai" && <p role="status">Đang tải dự án…</p>}

      {/* MỘT CÂU DUY NHẤT CHO CẢ "KHÔNG CÓ DỰ ÁN ẤY" LẪN "DỰ ÁN CỦA XÃ KHÁC": máy chủ trả cùng
          một 404 cho cả hai, và giao diện không dựng lại sự phân biệt ấy. */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && <ThongTinDuAn duAn={trangThai.duAn} />}
    </section>
  );
}

/** Phần thuần trình bày, tách ra để kết xuất được trong test mà không cần mạng. */
export function ThongTinDuAn({ duAn }: { duAn: finance_duAnRa }) {
  const tienDo = tienDoDuAn(duAn.delay_score, duAn.is_delayed);

  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3>{duAn.name}</h3>
        <span className={lopTienDo(tienDo)}>{nhanTienDo(tienDo)}</span>
      </div>
      <p className="ma-muc">{duAn.code}</p>

      <dl className="danh-sach-truong">
        <dt>Năm ngân sách</dt>
        <dd>{duAn.year}</dd>

        <dt>Kế hoạch vốn năm</dt>
        <dd>{nhanTien(duAn.planned_amount)}</dd>

        {/* "Tổng mức được duyệt" về đây đã áp sẵn quy tắc §9 cho ô để trống — máy chủ làm việc
            ấy, nên không có nhánh "để trống thì lấy bằng kế hoạch vốn" nào ở phía web. */}
        <dt>Tổng mức được duyệt</dt>
        <dd>{nhanTien(duAn.approved_amount)}</dd>

        <dt>Đã giải ngân</dt>
        <dd>{nhanTien(duAn.disbursed_amount)}</dd>

        <dt>Còn lại</dt>
        <dd>{nhanTien(duAn.remaining_amount)}</dd>

        <dt>Tỷ lệ giải ngân</dt>
        <dd>{nhanTyLeGiaiNgan(duAn.disbursed_ratio)}</dd>

        {/* HAI MỐC KHÁC NHAU, CỐ Ý ĐỂ CẠNH NHAU. §9 nói rõ: công trình xong tháng 3 vẫn có thể
            phải giải ngân trước 31/12, nên "ngày hoàn thành" không thay được "thời hạn giải
            ngân". Gộp hai dòng này làm một là mất đúng mốc bị hỏi khi quyết toán. */}
        <dt>Thời hạn giải ngân</dt>
        <dd>{nhanNgay(duAn.disbursement_deadline)}</dd>

        <dt>Ngày khởi công</dt>
        {/* Trường tuỳ chọn trong hợp đồng: vắng mặt nghĩa là máy chủ không nói gì, và `nhanNgay`
            nói ra điều đó bằng "Chưa đặt" chứ không bằng một ô trống. */}
        <dd>{nhanNgay(duAn.start_date ?? "")}</dd>

        <dt>Ngày hoàn thành</dt>
        <dd>{nhanNgay(duAn.completion_date ?? "")}</dd>
      </dl>

      {duAn.description !== undefined && duAn.description !== "" && (
        <p className="mo-ta-du-an">{duAn.description}</p>
      )}

      {/* ĐƠN VỊ THỰC HIỆN VÀ CÁN BỘ PHỤ TRÁCH KHÔNG HIỆN Ở ĐÂY. Hợp đồng chỉ trả ID nội bộ
          (`org_unit_id`, `assignee_id`); tra chúng thành tên bộ phận và tên cán bộ là việc của
          tuyến khác dưới quyền khác, và in một chuỗi ULID lên màn hình cán bộ không nói với ai
          điều gì. */}
    </div>
  );
}
