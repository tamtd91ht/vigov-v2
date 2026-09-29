/**
 * MÀN "DANH BẠ CÁN BỘ XÃ" — những cán bộ xã đã công khai, mỗi số điện thoại là một liên kết gọi.
 *
 * ⚠ CÔNG KHAI, KHÔNG CẦN PHIÊN (`api/vigov-client.ts` `communeStaffDirectory`). Cổng là TÊN MIỀN xã của lần mở
 *   này — màn chỉ được mở khi đã có nó (`CitizenChannel.tsx`).
 *
 * ⚠ SỐ DI ĐỘNG LÀ DỮ LIỆU CÁ NHÂN, công khai theo đồng ý từng người (#12). Nó chỉ được hiện ra và
 *   thành liên kết `tel:` — thứ mở màn quay số của máy, không đi qua mạng. Không log, không lưu, không
 *   gửi đi đâu, không đưa vào khoá hay tên tệp nào (luật 3).
 *
 * ⚠ "CÓ DÙNG ZALO" LÀ CHỮ, KHÔNG PHẢI MỘT BIỂU TƯỢNG MÀU (README §Non-negotiables #6), và chỉ hiện khi
 *   máy chủ nói có. Không có nút "nhắn Zalo": chưa có lời gọi nền tảng nào cho việc ấy ở nửa này.
 */
import { useEffect, useRef, useState } from "react";

import { communeStaffDirectory, type PublicResult } from "../api/vigov-client";
import type { PublicStaff } from "../api/public-contract";
import { getVigovSession } from "../api/vigov-session";

import { CommuneBanner } from "./frame";
import { DIRECTORY, BACK } from "./copy";

export type DirectoryError = "loi-mang" | "loi-may-chu" | "khong-hop-le";

export type DirectoryState =
  | { readonly kind: "dang-tai" }
  | { readonly kind: "xong"; readonly staff: readonly PublicStaff[] }
  | { readonly kind: "loi"; readonly error: DirectoryError };

export function afterDirectoryLoad(result: PublicResult<readonly PublicStaff[]>): DirectoryState {
  switch (result.kind) {
    case "xong":
      return { kind: "xong", staff: result.value };
    case "loi-mang":
      return { kind: "loi", error: "loi-mang" };
    case "khong-hop-le":
      return { kind: "loi", error: "khong-hop-le" };
    default:
      return { kind: "loi", error: "loi-may-chu" };
  }
}

const ERROR_TEXT: Readonly<Record<DirectoryError, string>> = {
  "loi-mang": DIRECTORY.network_error,
  "loi-may-chu": DIRECTORY.server_error,
  "khong-hop-le": DIRECTORY.invalid,
};

/**
 * Số điện thoại → đích `tel:`, hoặc `null` khi không còn chữ số nào để quay.
 *
 * GIỮ CHỮ SỐ VÀ MỘT DẤU `+` ĐẦU, BỎ MỌI THỨ KHÁC: số do xã nhập có thể mang dấu cách, chấm, gạch
 * (dạng "0900.000 000"). Một `tel:` mang ký tự lạ là liên kết máy quay số từ chối — và người lớn tuổi
 * bấm một số không gọi được sẽ không bấm lần hai.
 */
export function dialTarget(number: string): string | null {
  const trimmed = number.trim();
  const digits = trimmed.replace(/[^\d]/g, "");
  if (digits.length < 3) return null;
  return `tel:${trimmed.startsWith("+") ? "+" : ""}${digits}`;
}

function PhoneLine({ label, number }: { label: string; number: string }) {
  if (number.trim() === "") return null;
  const target = dialTarget(number);
  return (
    <div className="cd-can-bo__so">
      <span className="cd-can-bo__nhan">{label}</span>
      {target === null ? (
        <span>{number}</span>
      ) : (
        <a className="cd-goi" href={target}>
          {DIRECTORY.call(number.trim())}
        </a>
      )}
    </div>
  );
}

export function StaffCard({ staff }: { staff: PublicStaff }) {
  return (
    <li className="cd-can-bo">
      <p className="cd-can-bo__ten">{staff.full_name}</p>
      {staff.position !== "" && (
        <p className="cd-can-bo__dong">
          {DIRECTORY.position}: {staff.position}
        </p>
      )}
      {staff.org_unit !== "" && (
        <p className="cd-can-bo__dong">
          {DIRECTORY.org_unit}: {staff.org_unit}
        </p>
      )}
      <PhoneLine label={DIRECTORY.office_phone} number={staff.office_phone} />
      <PhoneLine label={DIRECTORY.mobile} number={staff.mobile} />
      {staff.has_zalo && <p className="cd-can-bo__dong cd-can-bo__zalo">{DIRECTORY.has_zalo}</p>}
    </li>
  );
}

export function DirectoryBody(props: { page: DirectoryState; onLoad: () => void }) {
  const { page } = props;
  if (page.kind === "dang-tai") {
    return (
      <p className="cd-cau" role="status">
        {DIRECTORY.loading}
      </p>
    );
  }
  if (page.kind === "loi") {
    return (
      <div className="cd-buoc">
        <p className="cd-loi" role="alert">
          {ERROR_TEXT[page.error]}
        </p>
        {page.error !== "khong-hop-le" && (
          <button type="button" className="cd-nut" onClick={props.onLoad}>
            {DIRECTORY.retry_button}
          </button>
        )}
      </div>
    );
  }
  if (page.staff.length === 0) return <p className="cd-cau">{DIRECTORY.empty}</p>;
  return (
    <>
      <p className="cd-cau">{DIRECTORY.intro}</p>
      <ul className="cd-cua-toi">
        {page.staff.map((staff, i) => (
          // Không có mã nào trong hợp đồng (cố ý: danh bạ công khai không lộ mã nội bộ), nên khoá là
          // vị trí. Danh sách không sắp lại phía client, nên vị trí đứng yên suốt một lần xem.
          <StaffCard key={i} staff={staff} />
        ))}
      </ul>
    </>
  );
}

export function StaffDirectoryScreen(props: { domain: string; onBack: () => void }) {
  const [commune_name] = useState(() => getVigovSession()?.commune_name ?? null);
  const [page, setPage] = useState<DirectoryState>({ kind: "dang-tai" });
  const loaded = useRef(false);

  async function load() {
    setPage({ kind: "dang-tai" });
    setPage(afterDirectoryLoad(await communeStaffDirectory(props.domain)));
  }

  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void load();
  }, []);

  return (
    <section className="cd-man" aria-label={DIRECTORY.title}>
      <button type="button" className="quay-lai" onClick={props.onBack}>
        {BACK}
      </button>
      {commune_name !== null && <CommuneBanner commune_name={commune_name} />}
      <h1 className="cd-tieu-de">{DIRECTORY.title}</h1>
      <DirectoryBody page={page} onLoad={() => void load()} />
    </section>
  );
}
