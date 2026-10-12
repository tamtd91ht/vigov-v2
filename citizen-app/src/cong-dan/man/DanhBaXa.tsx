/**
 * DANH BẠ CÁN BỘ XÃ — giao diện theo prototype khách (`PROTOTYPE.md` §6.9, đợt 1 30/09/2026): ô tìm đứng
 * yên ở đầu vùng cuộn, nhóm theo bộ phận, mỗi cán bộ một thẻ, mỗi số một nút "Gọi" rộng hết thẻ.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `DanhBaCanBoScreen.tsx`: chỉ người đã đồng ý công khai (#12). Tìm kiếm lọc NGAY
 * TRÊN MÁY trong danh sách đã tải; từ khoá không gửi đi đâu.
 *
 * THẺ CÁN BỘ, bảng lỗi App Xã 12/10/2026 dòng 11, 12, 14 (chủ dự án): họ tên, chức vụ, MỘT số điện thoại, nút "GỌI",
 * nút "NHẮN ZALO" — như `DirectoryPage.tsx:97-131` của bản mẫu. Hai nút đi qua lời gọi nền tảng do lớp vỏ tiêm
 * (`DialNumber`, `OpenZaloChat`): webview của Mini App bỏ qua liên kết `tel:`, nên nút "Gọi" cũ bấm không ra gì. Số
 * điện thoại chỉ đi vào màn quay số hoặc đường dẫn zalo.me — không log, không lưu, không đi vào khoá hay thống kê nào.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { danhBaCanBoXa } from "../api/goi-vigov";
import type { CanBoCongKhai } from "../api/hop-dong-cong-khai";
import { dichGoi, sauKhiTaiDanhBa, type TrangDanhBa } from "./DanhBaCanBoScreen";
import { DANH_BA, XA_GIAO_DIEN, XA_PA, XA_TN } from "./noi-dung";

import { BieuTuong } from "./BieuTuong";
import { KhoiTrangThai } from "./khung-xa";
import { OTimKiem } from "./o-nhap"; // vi-name-ok: existing module name, not renamed (rule 12 #3)

/** Bỏ dấu tiếng Việt và hoa thường — người lớn tuổi hay gõ không dấu ("chu tich"). */
export function boDau(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();
}

/**
 * Lọc trên máy, không phân biệt hoa thường, không phân biệt dấu, và theo SỐ điện thoại (bỏ dấu cách, chấm
 * trong số) — như danh bạ của prototype. Không dấu hiệu nào của từ khoá rời khỏi máy.
 */
export function locCanBo(ds: readonly CanBoCongKhai[], tu_khoa: string): readonly CanBoCongKhai[] {
  const q = boDau(tu_khoa.trim());
  if (q === "") return ds;
  const so = q.replace(/\D/g, "");
  // Chỉ so theo số khi từ khoá TOÀN là số (cho phép dấu cách, chấm, gạch) — "tổ 3" không khớp mọi số có chữ 3.
  const theo_so = so.length >= 3 && /^[\d\s.\-+]+$/.test(q);
  return ds.filter(
    (cb) =>
      // The units a person heads are searchable too: "ha lam" finds the Trưởng thôn Hà Lam.
      [cb.ho_ten, cb.chuc_vu, cb.bo_phan, ...cb.residential_units_headed].some((o) => boDau(o).includes(q)) ||
      (theo_so && [cb.so_co_quan, cb.di_dong].some((o) => o.replace(/\D/g, "").includes(so))),
  );
}

/**
 * The commune's own order first: people with a `display_order` ascending, then everybody without one in
 * the order the server sent. STABLE — equal positions keep the server's order. The server already sends
 * this order (`danh_ba_cong_khai.go:54-56`); sorting here is the guard for the day it does not, so the
 * commune's chosen order is what the citizen sees either way. PURE.
 */
export function byDisplayOrder(ds: readonly CanBoCongKhai[]): CanBoCongKhai[] {
  return ds
    .map((cb, i) => ({ cb, i }))
    .sort((a, b) => {
      const x = a.cb.display_order;
      const y = b.cb.display_order;
      if (x !== null && y !== null && x !== y) return x - y;
      if (x !== null && y === null) return -1;
      if (x === null && y !== null) return 1;
      return a.i - b.i;
    })
    .map((e) => e.cb);
}

/**
 * Nhóm theo BỘ PHẬN, theo thứ tự máy chủ trả (không sắp lại). Bản mẫu chia "Lãnh đạo UBND xã" /
 * "Bộ phận chuyên môn" bằng một trường nhóm ViGov không có; `bo_phan` là thứ xã thật sự nhập. THUẦN.
 *
 * Người không ghi bộ phận đứng ở CUỐI, trong một nhóm `bo_phan: null` — KHÔNG có tiêu đề (bảng lỗi App Xã 12/10/2026
 * dòng 15: "dư cụm ở cuối danh sách → bỏ"). Bỏ tiêu đề, không bỏ người: với nhiều bà con, thẻ ấy là đường duy nhất tới
 * số của cán bộ đó (ADR 0090 #2).
 */
export function nhomTheoBoPhan(ds: readonly CanBoCongKhai[]): { bo_phan: string | null; can_bo: CanBoCongKhai[] }[] {
  const nhom: { bo_phan: string | null; can_bo: CanBoCongKhai[] }[] = [];
  const khac: CanBoCongKhai[] = [];
  for (const cb of ds) {
    const bp = cb.bo_phan.trim();
    if (bp === "") {
      khac.push(cb);
      continue;
    }
    const co = nhom.find((n) => n.bo_phan === bp);
    if (co) co.can_bo.push(cb);
    else nhom.push({ bo_phan: bp, can_bo: [cb] });
  }
  if (khac.length > 0) nhom.push({ bo_phan: null, can_bo: khac });
  return nhom;
}

/**
 * THE ONE NUMBER A CARD SHOWS (owner, 12/10/2026, sheet row 14: "mobile if published, else office"). The mobile is
 * published only with the person's own consent (#12/#16 decided), and it is the number that reaches them; the office
 * line is the fallback. A value `dichGoi` cannot dial (too few digits) is passed over — a number the citizen cannot
 * call is not worth the space. `null` → the card shows no number and no button. PURE.
 */
export function publishedNumber(cb: CanBoCongKhai): { value: string; dial: string; mobile: boolean } | null {
  const mobile = cb.di_dong.trim();
  const mobileDial = mobile === "" ? null : dichGoi(mobile);
  if (mobileDial !== null) return { value: mobile, dial: mobileDial, mobile: true };
  const office = cb.so_co_quan.trim();
  const officeDial = office === "" ? null : dichGoi(office);
  return officeDial === null ? null : { value: office, dial: officeDial, mobile: false };
}

/**
 * "NHẮN ZALO" exists only for a PUBLISHED MOBILE whose owner the directory says uses Zalo (`has_zalo`): zalo.me finds
 * a Zalo account by its phone number, and an office line has none. PURE.
 */
export function offersZaloChat(cb: CanBoCongKhai, shown: { mobile: boolean } | null): boolean {
  return cb.co_zalo && shown !== null && shown.mobile;
}

/**
 * The shell's dialer (`openPhone`, `App.tsx`) — this half may not import zmp-sdk. Takes the digits `dichGoi` kept
 * (with one leading `+`); `true` when the phone's dial screen opened. Absent (tests, a browser) → a `tel:` link.
 */
export type DialNumber = (digits: string) => Promise<boolean>;
/** The shell's "Nhắn Zalo" (`openOutApp` with zalo.me, `App.tsx`). `true` when Zalo opened. Absent → no button. */
export type OpenZaloChat = (digits: string) => Promise<boolean>;

export type DirectoryActions = { dial?: DialNumber; openZaloChat?: OpenZaloChat };

/**
 * One person, as the prototype's `DirectoryRow` (`DirectoryPage.tsx:97-131`): name, title, ONE number, then "GỌI" and
 * — when `offersZaloChat` — "NHẮN ZALO" side by side. The department is the group's heading, so it is not repeated.
 * A failed platform call says what to do instead, in words (README §Non-negotiables #7): a button that does nothing
 * visible is pressed four more times and then given up on.
 */
function TheCanBoXa({ cb, actions }: { cb: CanBoCongKhai; actions: DirectoryActions }) {
  const [failed, setFailed] = useState<"call" | "zalo" | null>(null);
  const shown = publishedNumber(cb);
  const digits = shown === null ? "" : shown.dial.slice("tel:".length);
  const { dial, openZaloChat } = actions;
  const zalo = openZaloChat !== undefined && offersZaloChat(cb, shown);
  return (
    <li className="xa-the xa-staff-card">
      <strong className="xa-staff-card__name">{cb.ho_ten}</strong>
      {cb.chuc_vu !== "" && <span className="xa-phu">{cb.chuc_vu}</span>}
      {shown !== null && (
        <>
          <span className="xa-staff-card__digits">{shown.value}</span>
          <div className="xa-staff-card__actions">
            {dial === undefined ? (
              <a className="xa-staff-card__call" href={shown.dial} aria-label={XA_GIAO_DIEN.call_person(cb.ho_ten, shown.value)}>
                <BieuTuong ten="phone" co={20} />
                <span>{XA_GIAO_DIEN.goi}</span>
              </a>
            ) : (
              <button
                type="button"
                className="xa-staff-card__call"
                aria-label={XA_GIAO_DIEN.call_person(cb.ho_ten, shown.value)}
                onClick={() => {
                  setFailed(null);
                  void dial(digits).then((ok) => setFailed(ok ? null : "call"));
                }}
              >
                <BieuTuong ten="phone" co={20} />
                <span>{XA_GIAO_DIEN.goi}</span>
              </button>
            )}
            {zalo && (
              <button
                type="button"
                className="xa-staff-card__zalo"
                aria-label={XA_GIAO_DIEN.zalo_chat_person(cb.ho_ten)}
                onClick={() => {
                  setFailed(null);
                  void openZaloChat(digits).then((ok) => setFailed(ok ? null : "zalo"));
                }}
              >
                <BieuTuong ten="message-circle" co={20} />
                <span>{XA_GIAO_DIEN.zalo_chat}</span>
              </button>
            )}
          </div>
          {/* Always in the page, so a screen reader announces the sentence when it appears (a live region added
              together with its text is often not read). */}
          <p className="xa-phu xa-staff-card__failed" role="status">
            {failed === "call" ? XA_GIAO_DIEN.call_failed : failed === "zalo" ? XA_GIAO_DIEN.zalo_chat_failed : ""}
          </p>
        </>
      )}
    </li>
  );
}

const CAU_LOI = {
  "loi-mang": DANH_BA.loi_mang,
  "loi-may-chu": DANH_BA.loi_may_chu,
  "khong-hop-le": XA_PA.danh_ba_khong_hop_le,
} as const;

export function ThanDanhBaXa(props: { trang: TrangDanhBa; onTai: () => void } & DirectoryActions) {
  const [tu_khoa, datTuKhoa] = useState("");
  const { trang } = props;
  const actions: DirectoryActions = { dial: props.dial, openZaloChat: props.openZaloChat };
  const loc = useMemo(
    () => (trang.kieu === "xong" ? locCanBo(byDisplayOrder(trang.can_bo), tu_khoa) : []),
    [trang, tu_khoa],
  );

  // The search is ALWAYS there, as the prototype's (`DirectoryPage.tsx:48-62`, owner 11/10/2026) — also while the
  // list loads or failed: a box that appears later moves the screen under the citizen's finger.
  const search = (
    // Sticky at the top of the scrolling area (§6.9): a long directory scrolls under it, and the search stays one
    // tap away. It sits INSIDE the scroller, so it never covers the header or the tab bar.
    <div className="xa-tim">
      <OTimKiem
        id="xa-tim-can-bo"
        nhan={XA_GIAO_DIEN.tim_danh_ba}
        placeholder={XA_GIAO_DIEN.tim_danh_ba_goi_y}
        gia_tri={tu_khoa}
        toi_da={60}
        onDoi={datTuKhoa}
      />
    </div>
  );
  if (trang.kieu === "dang-tai") {
    return (
      <>
        {search}
        <KhoiTrangThai bieu_tuong="users" cau={DANH_BA.dang_tai} dang_tai shape="rows" />
      </>
    );
  }
  if (trang.kieu === "loi") {
    return (
      <>
        {search}
        <KhoiTrangThai
          bieu_tuong="alert"
          loi
          cau={CAU_LOI[trang.loi]}
          nut={trang.loi !== "khong-hop-le" ? { nhan: DANH_BA.nut_thu_lai, onBam: props.onTai } : undefined}
        />
      </>
    );
  }
  if (trang.can_bo.length === 0) {
    return (
      <>
        {search}
        <KhoiTrangThai bieu_tuong="users" cau={XA_PA.danh_ba_trong} />
      </>
    );
  }

  return (
    <>
      {search}
      {loc.length === 0 ? (
        <KhoiTrangThai bieu_tuong="users" cau={XA_GIAO_DIEN.khong_thay_can_bo} hint={XA_TN.directory_search_empty_hint} />
      ) : (
        nhomTheoBoPhan(loc).map((n) => (
          // The people with no department: no visible heading (sheet row 15), but the region keeps a name for a screen
          // reader, so they are not heard as members of the department just above.
          <section
            key={n.bo_phan ?? ""}
            className="xa-nhom"
            aria-label={n.bo_phan === null ? XA_TN.nhom_khac : undefined}
          >
            {n.bo_phan !== null && <h2 className="xa-dau-khoi__tieu-de xa-nhom__tieu-de">{n.bo_phan}</h2>}
            <ul className="xa-ds">
              {n.can_bo.map((cb, i) => (
                // Không có mã trong hợp đồng công khai (cố ý); danh sách không sắp lại, nên khoá là vị trí
                // trong nhóm cộng tên — đủ ổn cho một lần xem.
                <TheCanBoXa key={`${i}-${cb.ho_ten}`} cb={cb} actions={actions} />
              ))}
            </ul>
          </section>
        ))
      )}
    </>
  );
}

export function useDanhBaXa(ten_mien: string) {
  const [trang, datTrang] = useState<TrangDanhBa>({ kieu: "dang-tai" });
  const da_tai = useRef(false);

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiDanhBa(await danhBaCanBoXa(ten_mien)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  /** The pull: read again without blanking the list; what the citizen typed in the search box stays (`ThanDanhBaXa`). */
  async function refresh(): Promise<boolean> {
    if (trang.kieu === "dang-tai") return true;
    const next = sauKhiTaiDanhBa(await danhBaCanBoXa(ten_mien));
    datTrang((now) => directoryAfterRefresh(now, next));
    return next.kieu !== "loi";
  }

  return { trang, taiLai: () => void tai(), refresh };
}

/**
 * The directory after a pull's re-read. PURE. A failed re-read keeps a list on screen — a phone number the citizen
 * may be about to dial must not vanish because a refresh failed; the pull's own sentence says it is not fresh.
 * Anything else (fresh list, or a failure with no list to keep) is the re-read's answer.
 */
export function directoryAfterRefresh(now: TrangDanhBa, next: TrangDanhBa): TrangDanhBa {
  return next.kieu === "loi" && now.kieu === "xong" ? now : next;
}
