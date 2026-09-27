/**
 * TRANG CỦA XÃ — màn gốc của APP RIÊNG một xã (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Mong muốn của chủ dự án (28/09/2026): chọn app của xã trên Zalo là thấy NGAY giao diện của xã ấy;
 * xã sau (Sông Hàn…) y hệt, không sửa mã. Nên màn này:
 *
 *   1. tra tên xã theo tên miền của bản dựng (`GET identity /api/v1/communes?host=`, công khai);
 *   2. hiện kênh công dân của xã — tin tức, danh bạ đọc được ngay, KHÔNG đăng nhập, KHÔNG qua
 *      `vihat-miniapp`.
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP. Mở phiên qua cầu của app chung với `communeConfirmed=true` tự động làm
 * máy chủ đổi xã đã nhớ, thu hồi phiên xã khác và ghi vết một lần xác nhận không ai bấm (ADR 0047 §6).
 * Đăng nhập thuộc về lúc người dân làm việc cá nhân, và đường của nó cho app riêng — xã từ App ID đã
 * xác minh — CHƯA DỰNG; cho tới lúc ấy ba lối phản ánh nói "chưa đăng nhập được" (`CHUA_DANG_NHAP_XA`).
 *
 * Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log. Tên xã là thứ máy chủ trả, không bao giờ dựng
 * từ tên miền. Không đúng một xã → fail closed với một câu và nút "Thử lại".
 */
import { useEffect, useState } from "react";

import { type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import type { XaTraDuoc } from "../api/hop-dong-cong-khai";

import { KenhCongDan } from "./KenhCongDan";
import { APP_RIENG } from "./noi-dung";

export type XaCuaApp = { readonly ten: string; readonly tinh: string };

/** Kết quả tra xã → xã của app, hoặc câu báo lỗi. THUẦN. Đúng MỘT xã, tên không rỗng. */
export function xaTuKetQuaTra(
  kq: KetQuaCongKhai<readonly XaTraDuoc[]>,
): { readonly xa: XaCuaApp } | { readonly loi: string } {
  if (kq.kieu === "xong" && kq.gia_tri.length === 1 && kq.gia_tri[0]!.ten.trim() !== "") {
    return { xa: kq.gia_tri[0]! };
  }
  if (kq.kieu === "xong" || kq.kieu === "khong-hop-le" || kq.kieu === "khong-thay") {
    return { loi: APP_RIENG.khong_thay };
  }
  return { loi: APP_RIENG.chua_ket_noi };
}

type Trang =
  | { readonly kieu: "dang-tra" }
  | { readonly kieu: "loi"; readonly cau: string }
  | { readonly kieu: "xong" };

export function TrangXa(props: {
  /** Tên miền xã nung vào bản dựng (`lib/xa-co-dinh.ts`). */
  ten_mien: string;
  /** Báo tên xã lên header của lớp vỏ — lớp vỏ không được nhập client ViGov. */
  onXa: (xa: XaCuaApp) => void;
}) {
  const { ten_mien, onXa } = props;
  const [trang, datTrang] = useState<Trang>({ kieu: "dang-tra" });
  /** Mỗi lần bấm "Thử lại" tăng một — hiệu ứng tra chạy lại đúng một lần cho mỗi giá trị. */
  const [lan, datLan] = useState(0);

  useEffect(() => {
    let con_song = true;
    void traXaTheoTenMien(ten_mien).then((kq) => {
      if (!con_song) return;
      const kq_xa = xaTuKetQuaTra(kq);
      if ("xa" in kq_xa) {
        onXa(kq_xa.xa);
        datTrang({ kieu: "xong" });
      } else {
        datTrang({ kieu: "loi", cau: kq_xa.loi });
      }
    });
    return () => {
      con_song = false;
    };
    // Tên miền là hằng của bản dựng; chỉ `lan` đổi.
  }, [lan]);

  if (trang.kieu === "xong") return <KenhCongDan ten_mien={ten_mien} />;

  if (trang.kieu === "loi") {
    return (
      <section className="cd-man">
        <p className="cd-loi" role="status">
          {trang.cau}
        </p>
        <button
          type="button"
          className="cd-nut"
          onClick={() => {
            datTrang({ kieu: "dang-tra" });
            datLan((n) => n + 1);
          }}
        >
          {APP_RIENG.thu_lai}
        </button>
      </section>
    );
  }

  return (
    <section className="goi-y" aria-busy="true">
      <p className="goi-y__tiep" role="status">
        {APP_RIENG.dang_mo}
      </p>
    </section>
  );
}
