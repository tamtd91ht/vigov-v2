"use client";

import { LockKeyhole, LogIn, Mail, ShieldCheck } from "lucide-react";
import { useId, useState, type FormEvent } from "react";

import { dangNhap } from "@/lib/api/phien";

import { duongDanTiepTuc } from "./duong-dan-tiep-tuc";
import { luuSid } from "./sid-phien";

/**
 * Biểu mẫu đăng nhập — `15-phu-luc-giao-dien-chung` §1.
 *
 * MỘT THÔNG BÁO, KHÔNG HAI: sai địa chỉ thư, sai mật khẩu và bỏ trống đều hiện đúng câu mà
 * máy chủ trả về. Dịch vụ identity cố ý trả cùng một `code` và cùng một `message` cho cả ba
 * (`handler.go`, hàm `tuChoiDangNhap`); phân biệt chúng ở đây là dựng lại đúng thứ máy chủ vừa
 * giấu — tức là để người ngoài dò ra thư điện tử công vụ nào có thật trên tên miền của xã.
 *
 * VÌ SAO KHÔNG ĐẶT `required` TRÊN Ô NHẬP: trình duyệt sẽ tự báo "hãy điền trường này", và đó
 * là một thông báo thứ hai, khác câu của máy chủ, nói về một trường cụ thể. Máy chủ đã nhận
 * việc trả lời ca bỏ trống, bằng đúng câu chung ấy. Để nó trả lời.
 *
 * COOKIE: không có dòng nào ở đây đụng tới cookie. Cookie phiên do dịch vụ identity đặt bằng
 * Set-Cookie, httpOnly, host-only. JavaScript không đọc được và không cần đọc.
 *
 * KHÔNG CÓ Ô "GHI NHỚ ĐĂNG NHẬP" dù §1 có vẽ: hợp đồng `identity.thanDangNhap` chỉ có `email`
 * và `password`, không có chỗ nào nhận lựa chọn ấy, và thời hạn phiên do máy chủ quyết. Một ô
 * tích không nối vào đâu là một lời hứa với người dùng mà hệ thống không giữ.
 */
export function FormDangNhap({ tiepTuc }: { tiepTuc: string | null }) {
  const idEmail = useId();
  const idMatKhau = useId();
  const idLoi = useId();

  const [email, datEmail] = useState("");
  const [matKhau, datMatKhau] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [thongBaoLoi, datThongBaoLoi] = useState<string | null>(null);

  async function guiDi(su: FormEvent<HTMLFormElement>) {
    su.preventDefault();
    if (dangGui) return;

    datDangGui(true);
    datThongBaoLoi(null);

    const ketQua = await dangNhap({ email, password: matKhau });

    if (!ketQua.ok) {
      datThongBaoLoi(ketQua.thongBao);
      datDangGui(false);
      return;
    }

    luuSid(ketQua.duLieu.sid);

    // Điều hướng bằng CẢ TRANG, không phải điều hướng phía client: yêu cầu kế tiếp phải là một
    // yêu cầu thật tới máy chủ, để cookie phiên vừa được đặt đi cùng nó và proxy nhìn thấy —
    // và để không còn mật khẩu vừa gõ nằm lại trong state của một component đã rời màn hình.
    // Giữ `dangGui` bật cho tới lúc trang mới thay chỗ: biểu mẫu không được sống lại giữa chừng.
    //
    // ESLint của Next đề nghị `useRouter().push()`; xem lý do từ chối ở `nut-dang-xuat.tsx`.
    window.location.assign(duongDanTiepTuc(tiepTuc));
  }

  return (
    <form className="form-dang-nhap" onSubmit={guiDi} noValidate>
      <h1 className="tieu-de-form">Đăng nhập hệ thống</h1>
      <p className="login-card-sub">Dành cho cán bộ, công chức của xã</p>

      <div className="o-nhap">
        <label htmlFor={idEmail}>Thư điện tử công vụ</label>
        <div className="login-input">
          <Mail aria-hidden="true" focusable="false" strokeWidth={1.8} />
          <input
            id={idEmail}
            name="email"
            type="email"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            value={email}
            onChange={(su) => datEmail(su.target.value)}
            disabled={dangGui}
            aria-describedby={thongBaoLoi ? idLoi : undefined}
          />
        </div>
      </div>

      <div className="o-nhap">
        <label htmlFor={idMatKhau}>Mật khẩu</label>
        <div className="login-input">
          <LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} />
          <input
            id={idMatKhau}
            name="password"
            type="password"
            autoComplete="current-password"
            value={matKhau}
            onChange={(su) => datMatKhau(su.target.value)}
            disabled={dangGui}
            aria-describedby={thongBaoLoi ? idLoi : undefined}
          />
        </div>
      </div>

      {/*
        `role="alert"` + `aria-live` để người dùng trình đọc màn hình nghe được câu từ chối mà
        không phải đi tìm. Chỉ có một vùng thông báo, vì chỉ có một thông báo.
      */}
      <p id={idLoi} className="thong-bao-loi" role="alert" aria-live="assertive">
        {thongBaoLoi ?? ""}
      </p>

      <button type="submit" className="nut-chinh" disabled={dangGui} aria-busy={dangGui}>
        <LogIn aria-hidden="true" focusable="false" strokeWidth={1.8} />
        {dangGui ? "Đang đăng nhập…" : "Đăng nhập"}
      </button>

      {/* Trust line requested by the owner (spec §8.4, 02/10/2026). It is a statement about the
          transport: the session cookie is `Secure`, so a sign-in over plain HTTP cannot complete.
          HSTS is NOT sent yet (`tools/security_debt.json`, web-admin headers) — closing that debt
          is what makes this sentence true for the very first request too. */}
      <p className="login-trust">
        <ShieldCheck aria-hidden="true" focusable="false" strokeWidth={1.8} />
        Kết nối được mã hoá
      </p>
    </form>
  );
}
