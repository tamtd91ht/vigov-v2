"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { layHangMucKeHoachVon } from "@/lib/api/danh-muc-nghiep-vu";
import { layChiTietDuAn } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type { finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { KhoiChungTu } from "./chung-tu-du-an";
import { KhoiSuaXoaDuAn } from "./ghi-du-an";
import { KhoiChuaDungGhi } from "./khoi-chua-dung-ghi";
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
 * Trang chi tiết một dự án — `docs/ui-ux/06-giai-ngan.md §8`, cộng `[✎ Sửa dự án]`, `🗑 Gỡ dự án`
 * và khối "Chứng từ" của §8.2.
 *
 * BA TRONG BỐN TAB CỦA ĐẶC TẢ VẪN KHÔNG CÓ (Vướng mắc · Biểu đồ · Trao đổi): không tab nào có
 * tuyến phía sau trong hợp đồng REST. Một thanh tab mà bấm vào không ra gì là những lần hứa suông
 * với cán bộ — tab chỉ mọc khi tuyến mọc.
 *
 * TAB "CHỨNG TỪ" NAY CÓ SÁU TUYẾN GHI VÀ KHÔNG CÓ TUYẾN ĐỌC. Khối chứng từ dưới đây vì thế chỉ giữ
 * được những chứng từ của chính phiên làm việc này, và nó NÓI RA điều đó — xem `chung-tu-du-an.tsx`
 * và mục đầu của `PHAN_CHUA_DUNG_GHI`.
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
  /**
   * KẾT QUẢ LƯU KÈM KHOÁ ĐÃ SINH RA NÓ, và "đang tải" SUY RA từ chỗ hai khoá lệch nhau — cùng
   * khuôn `bang-du-an.tsx`, và cùng lý do: một `setState` thẳng trong thân effect vừa là thứ React
   * Compiler cấm, vừa để lại một cửa sổ ở đó số của dự án CŨ còn đứng trên màn hình sau khi vừa
   * ghi xong một chứng từ làm chúng thay đổi.
   */
  const [daTai, datDaTai] = useState<{ khoa: string; kq: KetQua<finance_duAnRa> } | null>(null);
  const [lanTai, datLanTai] = useState(0);
  const [daXoa, datDaXoa] = useState(false);
  const [danhMuc, datDanhMuc] = useState<readonly finance_hangMucRa[]>([]);
  const khoa = `${id}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    layChiTietDuAn(id).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [id, khoa]);

  /** Danh mục hạng mục cho ô chọn của biểu mẫu sửa. Hỏng danh mục KHÔNG làm hỏng trang. */
  useEffect(() => {
    let bo = false;
    layHangMucKeHoachVon().then((kq) => {
      if (!bo && kq.ok) datDanhMuc(kq.duLieu.items);
    });
    return () => {
      bo = true;
    };
  }, []);

  const trangThai: TrangThaiChiTiet =
    daTai === null || daTai.khoa !== khoa
      ? { pha: "dangTai" }
      : daTai.kq.ok
        ? { pha: "xong", duAn: daTai.kq.duLieu }
        : { pha: "loi", thongBao: daTai.kq.thongBao };

  // FAIL CLOSED — xem `bang-du-an.tsx`. HAI khoá đọc riêng, và chúng không suy ra nhau.
  const phien = usePhien();
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);
  const coXacNhan = coQuyen(dsQuyen, QUYEN_XAC_NHAN_NGAN_SACH);

  return (
    <section className="man-giai-ngan" aria-labelledby="tieu-de-chi-tiet-du-an">
      <p className="duong-lui">
        <Link href="/giai-ngan">← Theo dõi giải ngân</Link>
      </p>
      <h2 id="tieu-de-chi-tiet-du-an">Chi tiết dự án</h2>
      <p className="canh-bao-pham-vi">{CANH_BAO_KHONG_PHAI_KE_TOAN}</p>

      {/* DỰ ÁN VỪA BỊ GỠ THÌ KHÔNG DỰNG LẠI NÓ. Máy chủ trả 204 không thân, và đọc lại sẽ ra 404 —
          một câu "Không tìm thấy dự án" ngay sau một thao tác thành công đọc như một lỗi. */}
      {daXoa ? (
        <p className="trang-thai-rong">
          Đã gỡ dự án. Bản ghi vẫn còn trong hệ thống kèm người gỡ và lý do (xoá mềm), và mã dự án
          không quay lại dãy. <Link href="/giai-ngan">Về danh sách dự án</Link>
        </p>
      ) : (
        <>
          <KhoiChuaDungGhi />

          {trangThai.pha === "dangTai" && <p role="status">Đang tải dự án…</p>}

          {/* MỘT CÂU DUY NHẤT CHO CẢ "KHÔNG CÓ DỰ ÁN ẤY" LẪN "DỰ ÁN CỦA XÃ KHÁC": máy chủ trả cùng
              một 404 cho cả hai, và giao diện không dựng lại sự phân biệt ấy. */}
          {trangThai.pha === "loi" && (
            <p className="thong-bao-loi" role="alert">
              {trangThai.thongBao}
            </p>
          )}

          {trangThai.pha === "xong" && (
            <>
              <ThongTinDuAn duAn={trangThai.duAn} />

              <KhoiSuaXoaDuAn
                duAn={trangThai.duAn}
                danhMuc={danhMuc}
                coGhi={coGhi}
                coXacNhan={coXacNhan}
                daSuaXong={() => datLanTai((n) => n + 1)}
                daXoaXong={() => datDaXoa(true)}
              />

              {/* MỖI LẦN GHI CHỨNG TỪ XONG LÀ MỘT LẦN ĐỌC LẠI DỰ ÁN: `disbursed_amount`,
                  `remaining_amount`, `disbursed_ratio` và `delay_score` đều suy ra từ chứng từ. */}
              <KhoiChungTu
                duAnID={trangThai.duAn.id}
                coGhi={coGhi}
                coXacNhan={coXacNhan}
                daGhiXong={() => datLanTai((n) => n + 1)}
              />
            </>
          )}
        </>
      )}
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
