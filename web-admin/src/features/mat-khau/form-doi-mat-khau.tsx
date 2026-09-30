"use client";

import { useId, useState, type FormEvent } from "react";

import { DAI_TOI_THIEU, doiMatKhau, type NhapDoiMatKhau } from "./doi-mat-khau";

/**
 * Biểu mẫu tự đổi mật khẩu — `15-phu-luc-giao-dien-chung` §1, tuyến
 * `PUT /api/v1/staff/current/password`.
 *
 * MỘT VIỆC TRÊN MỘT MÀN HÌNH (`skills/accessibility-elderly`, REQUIRED #4): ba ô, một nút, một
 * vùng thông báo. Nhãn nằm TRÊN ô, không bao giờ chỉ có placeholder — một placeholder biến mất
 * ngay khi người ta bắt đầu gõ, và ở màn này thì cả ba ô đều gõ mù.
 *
 * KHÔNG CÓ Ô NÀO MANG ĐỊNH DANH NGƯỜI DÙNG, và không được thêm: `current` là của máy chủ, lấy từ
 * phiên. Một ô "tài khoản" ở đây là một ô đổi mật khẩu người khác (`lib/api/tai-khoan.ts`).
 *
 * TOÀN BỘ PHẦN QUYẾT ĐỊNH NẰM Ở `doi-mat-khau.ts`, kể cả đích đến sau khi đổi xong. Tệp này chỉ
 * giữ ba chuỗi đang gõ và cái CÁCH rời trang. Lý do ở đầu tệp kia: môi trường kiểm của kho này là
 * Node, nên thứ nằm trong một `onSubmit` là thứ không phép kiểm nào với tới.
 *
 * KHÔNG `console.*`, KHÔNG `localStorage`, KHÔNG `sessionStorage`: ba giá trị ở đây LÀ thông tin
 * đăng nhập. Bề mặt duy nhất giữ chúng là state của chính thành phần này, và nó biến mất cùng
 * trang — đó cũng là một lý do nữa để rời đi bằng CẢ TRANG.
 */

export function FormDoiMatKhau() {
  const idHienTai = useId();
  const idMoi = useId();
  const idNhapLai = useId();
  const idLoi = useId();
  const idGoiY = useId();

  const [hienTai, datHienTai] = useState("");
  const [moi, datMoi] = useState("");
  const [nhapLai, datNhapLai] = useState("");
  const [hienChu, datHienChu] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [thongBao, datThongBao] = useState<string | null>(null);

  async function guiDi(su: FormEvent<HTMLFormElement>) {
    su.preventDefault();
    if (dangGui) return;

    datDangGui(true);
    datThongBao(null);

    const nhap: NhapDoiMatKhau = { hienTai, moi, nhapLai };

    // Rời trang bằng CẢ TRANG. ESLint của Next đề nghị `useRouter().push()`; lý do từ chối giống
    // hệt nút đăng xuất (`features/auth/nut-dang-xuat.tsx`), và ở đây còn nặng hơn: điều hướng
    // phía client giữ nguyên tiến trình, tức giữ nguyên cả ba mật khẩu vừa gõ trong bộ nhớ trang,
    // trên một máy dùng chung ở bộ phận một cửa (câu mở #18).
    const ketCuc = await doiMatKhau(nhap, (duongDan) => {
      window.location.assign(duongDan);
    });

    if (ketCuc.pha === "loi") {
      datThongBao(ketCuc.thongBao);
      datDangGui(false);
      return;
    }

    // KHÔNG mở khoá biểu mẫu ở nhánh thành công: phiên đã bị thu hồi và trang mới đang tới. Một
    // biểu mẫu sống lại giữa chừng là một lần bấm thứ hai chắc chắn nhận 401.
  }

  // `type` của cả ba ô đổi cùng nhau. MỘT nút cho ba ô, không phải ba nút: ở 320px, ba biểu
  // tượng con mắt không nhãn là ba vùng chạm nhỏ nằm sát ô nhập, và bấm trượt vào chúng là
  // chuyện thường xuyên (`skills/accessibility-elderly`, REQUIRED #2).
  const kieuO = hienChu ? "text" : "password";

  return (
    <form className="form-doi-mat-khau" onSubmit={guiDi} noValidate>
      {/*
        NÓI TRƯỚC HỆ QUẢ, KHÔNG ĐỂ NÓ THÀNH BẤT NGỜ. Đổi xong là máy chủ thu hồi MỌI phiên, kể cả
        phiên vừa gọi — người dùng sẽ bị đưa về màn đăng nhập. Không nói trước thì lần bị đá ra
        ấy đọc y hệt một lần hệ thống hỏng ngay sau khi họ làm đúng.
      */}
      <p className="canh-bao-pham-vi">
        Sau khi đổi, hệ thống kết thúc tất cả phiên đang mở của bạn — kể cả phiên trên máy này.
        Bạn sẽ được đưa về màn đăng nhập và cần đăng nhập lại bằng mật khẩu mới.
      </p>

      <div className="o-nhap">
        <label htmlFor={idHienTai}>Mật khẩu hiện tại</label>
        <input
          id={idHienTai}
          name="current_password"
          type={kieuO}
          autoComplete="current-password"
          autoCapitalize="none"
          spellCheck={false}
          value={hienTai}
          onChange={(su) => datHienTai(su.target.value)}
          disabled={dangGui}
          aria-describedby={thongBao !== null ? idLoi : undefined}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor={idMoi}>Mật khẩu mới</label>
        <input
          id={idMoi}
          name="new_password"
          type={kieuO}
          autoComplete="new-password"
          autoCapitalize="none"
          spellCheck={false}
          value={moi}
          onChange={(su) => datMoi(su.target.value)}
          disabled={dangGui}
          aria-describedby={thongBao !== null ? `${idGoiY} ${idLoi}` : idGoiY}
        />
        {/*
          Con số đọc từ `DAI_TOI_THIEU`, không gõ lại: một câu gợi ý nói "8 ký tự" trong khi máy
          chủ đòi 12 là một câu hướng dẫn người dùng vào đúng một lần bị từ chối.
        */}
        <p id={idGoiY} className="ghi-chu">
          Ít nhất {DAI_TOI_THIEU} ký tự. Một câu dễ nhớ thì vừa dài vừa khó đoán hơn một chuỗi
          ngắn có ký tự đặc biệt.
        </p>
      </div>

      <div className="o-nhap">
        {/*
          Ô NÀY KHÔNG ĐI LÊN MÁY CHỦ. Nó ở đây vì một lần gõ nhầm mật khẩu mới là một tài khoản
          không đăng nhập lại được: máy chủ chỉ giữ chuỗi băm, không ai dựng lại được giá trị đã
          gõ, và đường ra duy nhất là nhờ quản trị viên đặt lại hộ (#17).
        */}
        <label htmlFor={idNhapLai}>Nhập lại mật khẩu mới</label>
        <input
          id={idNhapLai}
          name="xac-nhan-mat-khau-moi"
          type={kieuO}
          autoComplete="new-password"
          autoCapitalize="none"
          spellCheck={false}
          value={nhapLai}
          onChange={(su) => datNhapLai(su.target.value)}
          disabled={dangGui}
          aria-describedby={thongBao !== null ? idLoi : undefined}
        />
      </div>

      {/*
        NÚT CHỮ, KHÔNG PHẢI BIỂU TƯỢNG. Cán bộ lớn tuổi gõ sai nhiều hơn hẳn khi không nhìn thấy
        chữ mình gõ, và một con mắt gạch chéo là thứ phải đoán nghĩa. `aria-pressed` để người
        dùng trình đọc màn hình biết trạng thái hiện thời chứ không chỉ biết tên nút.
      */}
      <button
        type="button"
        className="nut-phu nut-hien-mat-khau"
        aria-pressed={hienChu}
        onClick={() => datHienChu((truoc) => !truoc)}
      >
        {hienChu ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
      </button>

      {/*
        Vùng thông báo LUÔN có mặt trong DOM, kể cả khi rỗng: thêm/bớt một phần tử làm cả biểu mẫu
        nhảy chỗ, và vùng aria-live thêm vào sau không phải trình đọc màn hình nào cũng đọc.

        MỘT VÙNG, MỘT CÂU — và câu ấy là NGUYÊN VĂN của máy chủ khi máy chủ từ chối. Không rẽ
        nhánh theo `code`, không hiện `trace_id`, không hiện số hiệu HTTP.
      */}
      <p id={idLoi} className="thong-bao-loi" role="alert" aria-live="assertive">
        {thongBao ?? ""}
      </p>

      <button type="submit" className="nut-chinh" disabled={dangGui} aria-busy={dangGui}>
        {dangGui ? "Đang đổi mật khẩu…" : "Đổi mật khẩu"}
      </button>
    </form>
  );
}
