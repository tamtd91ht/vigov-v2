/**
 * Màn GỢI Ý XÃ — thứ hiện ra khi app được mở kèm tham số `t` (ADR 0005, lớp KHÁM PHÁ).
 *
 * ĐÂY KHÔNG PHẢI "ĐÃ CHỌN XÃ", VÀ KHÁC BIỆT ẤY LÀ TOÀN BỘ LÝ DO MÀN NÀY TỒN TẠI.
 *
 * ADR 0005 tách ba lớp và cấm gộp:
 *
 *   Khám phá  — công dân MUỐN làm việc với xã nào.  QR · deep link · GPS · picker.  KHÔNG tin được.
 *   Phiên     — phiên này ĐANG thao tác ở xã nào.   Server ghi sau khi công dân xác nhận.  Tin được.
 *   Uỷ quyền  — công dân này được đọc/ghi gì ở đó.  Quan hệ công dân↔xã + luật 4.  Tin được.
 *
 * Tham số trên QR là **dữ liệu client cung cấp**. Luật 1 cấm #2 cấm nhận xã từ client: một
 * client tự khai xã là một client tự cấp quyền cho mình. Nên màn này **dẫn giao diện**, và
 * không hơn — nó nói "có vẻ bạn muốn làm việc với xã này", chờ một chạm xác nhận, và việc ghi
 * xã vào phiên là của server.
 *
 * VÌ SAO NÓ CHƯA GỌI ĐƯỢC SERVER: chưa có server. `service-identity` mới có phía cán bộ; sáu
 * bảng kênh công dân đã có schema nhưng chưa chạy trên CSDL thật nào, chưa có route, chưa có
 * kho đọc. Nên nút xác nhận ở đây **cố ý không làm gì** ngoài việc nói ra bước kế tiếp là gì.
 * Một nút giả vờ đã chọn xã sẽ là đúng cái lỗi ADR 0005 dựng ba lớp để chặn.
 *
 * MỨC TIN THEO NGUỒN (ADR 0005): `qr` và `zns` thì chọn sẵn, một chạm xác nhận — người đang
 * đứng ở trụ sở xã không nên bị bắt đi tìm lại chính xã đó. `share` thì LUÔN bắt chọn tường
 * minh, vì nguồn gốc không xác định: một liên kết chuyển tay không nói lên ý định của người
 * nhận.
 */

type Props = {
  /** Mã xã do deep link mang tới. KHÔNG phải xã đã chọn — chỉ là gợi ý. */
  maXa: string;
  /** `qr` · `zns` · `share` · rỗng nếu không khai. Quyết định mức tin, xem chú thích đầu tệp. */
  nguon: string;
  onBoQua: () => void;
};

const TIN_DUOC_CHON_SAN = new Set(["qr", "zns"]);

export function GoiYXaScreen({ maXa, nguon, onBoQua }: Props) {
  const chonSan = TIN_DUOC_CHON_SAN.has(nguon);

  return (
    <section className="goi-y" aria-labelledby="goi-y-tieu-de">
      <p className="goi-y__nhan">Liên kết bạn vừa mở</p>
      <h1 className="goi-y__tieu-de" id="goi-y-tieu-de">
        {chonSan ? "Bạn muốn làm việc với xã này?" : "Chọn xã để tiếp tục"}
      </h1>

      <dl className="goi-y__bang">
        <div className="goi-y__dong">
          <dt>Mã xã</dt>
          <dd>{maXa}</dd>
        </div>
        <div className="goi-y__dong">
          <dt>Nguồn</dt>
          <dd>{nguon || "(không khai)"}</dd>
        </div>
      </dl>

      {/* TÊN XÃ CHƯA HIỆN ĐƯỢC, và nói thẳng ra thay vì để trống.
          Đổi mã sang tên là việc của sổ đăng ký phía server (ListTenants), và nó chưa có cài
          đặt. Hiện một cái tên bịa ra ở đây tệ hơn nhiều so với thừa nhận chưa tra được: công
          dân xác nhận theo TÊN xã, không theo một chuỗi ULID không đọc được. */}
      <p className="goi-y__canh-bao">
        Chưa tra được tên xã từ mã này — sổ đăng ký phía máy chủ chưa hoạt động. Công dân phải
        thấy <strong>tên xã</strong> trước khi xác nhận, nên bước này chưa hoàn chỉnh.
      </p>

      {!chonSan && (
        <p className="goi-y__canh-bao">
          Nguồn <code>{nguon || "không khai"}</code> không đủ tin để chọn sẵn. Liên kết chuyển
          tay không nói lên ý định của người nhận, nên xã phải được chọn tường minh.
        </p>
      )}

      <p className="goi-y__tiep">
        Bước kế tiếp thuộc về máy chủ: xác nhận ở đây sẽ yêu cầu máy chủ ghi xã vào phiên, và
        chính phiên đó — không phải liên kết này — mới là thứ quyết định bạn đang thao tác ở xã
        nào.
      </p>

      <button type="button" className="goi-y__nut" onClick={onBoQua}>
        Bỏ qua, xem giới thiệu công ty
      </button>
    </section>
  );
}
