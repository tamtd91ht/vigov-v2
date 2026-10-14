/**
 * MÀN "YÊU CẦU CỦA TÔI" — chỉ yêu cầu của chính người đang đăng nhập.
 *
 * ⚠ ĐỊNH DANH ĐẾN TỪ PHIÊN, KHÔNG TỪ MỘT THAM SỐ (luật 4, bất biến 2). Tuyến GET không nhận một
 * tham số nào nói "của ai", và việc KHÔNG CÓ chúng chính là tính năng — xem `api/goi-may-chu.ts`.
 *
 * ⚠ CHƯA ĐĂNG NHẬP THÌ KHÔNG ĐỌC, VÀ CŨNG KHÔNG HIỆN MỘT DANH SÁCH RỖNG. Một danh sách rỗng khi
 * chưa đăng nhập đọc ra thành "tôi chưa gửi yêu cầu nào" — một câu SAI, và người đọc nó sẽ gửi
 * lại lần nữa. `MoiDangNhap` thay chỗ.
 *
 * ⚠ DANH SÁCH RỖNG CÓ MỘT CÂU, KHÔNG PHẢI MỘT MÀN TRẮNG. Một màn trắng và một màn hỏng trông
 * giống hệt nhau, và người lớn tuổi đọc cả hai thành "app hỏng".
 *
 * 12/10/2026 (owner, Zalo resubmission): loading is a skeleton under a visible sentence, a failure says what happened
 * and offers "Thử lại", the empty list names the next action, the status is a pill with an icon AND words, and a pull
 * from the top re-reads the list — keeping the old list on screen when the re-read fails.
 */
import { useEffect, useRef, useState } from "react";

import { docYeuCauCuaToi, type KetQuaDoc } from "../../api/goi-may-chu";
import type { MaTrangThai, YeuCauDaGui } from "../../api/hop-dong-yeu-cau";
import { LOGIN_FROM_REQUESTS, type ThamSoMan } from "../company-intro/dieu-huong";
import { AlertGlyph, CheckGlyph, ClockGlyph, GridGlyph, HandshakeGlyph, InboxGlyph } from "../company-intro/icons";
import { bearerCua, dungPhien, forgetSessionOn401 } from "../dang-nhap/kho-phien";

import { IntroPullToRefresh } from "./intro-pull-to-refresh";
import { MoiDangNhap } from "./MoiDangNhap";
import { YEU_CAU_CUA_TOI } from "./noi-dung";
import { GIAI_THICH_TRANG_THAI, ngayDoc, NHAN_LOAI, NHAN_TRANG_THAI, STATUS_TONE, type StatusTone } from "./trang-thai";

const STATUS_GLYPH: Readonly<Record<StatusTone, typeof ClockGlyph>> = {
  received: InboxGlyph,
  working: ClockGlyph,
  contacted: HandshakeGlyph,
  done: CheckGlyph,
};

/**
 * THE STATUS PILL: an icon and the words, on the status's tone. The words carry the meaning (README §Non-negotiables
 * #6); the icon and the tone repeat it for the eye. PURE.
 */
export function RequestStatusChip({ status }: { status: MaTrangThai }) {
  const tone = STATUS_TONE[status];
  const Glyph = STATUS_GLYPH[tone];
  return (
    <p className={`vh-status vh-status--${tone}`}>
      <Glyph className="vh-status__glyph" />
      <span>{NHAN_TRANG_THAI[status]}</span>
    </p>
  );
}

/** Một dòng của danh sách, THUẦN. */
export function DongYeuCau({ yeu_cau }: { yeu_cau: YeuCauDaGui }) {
  const luc = ngayDoc(yeu_cau.tao_luc);

  return (
    <li className="card">
      <div className="vh-request__head">
        <h3 className="card__title">{NHAN_LOAI[yeu_cau.loai]}</h3>
        <RequestStatusChip status={yeu_cau.trang_thai} />
      </div>
      <ul className="the-tt__danh-sach">
        <li className="the-tt__dong">
          <span className="the-tt__nhan">{YEU_CAU_CUA_TOI.nhan_ma}</span>
          <span className="the-tt__gia-tri">{yeu_cau.ma}</span>
        </li>
        {luc !== "" && (
          <li className="the-tt__dong">
            <span className="the-tt__nhan">{YEU_CAU_CUA_TOI.nhan_gui_luc}</span>
            <span className="the-tt__gia-tri">{luc}</span>
          </li>
        )}
        {/* A count, no thumbnails (owner 12/10/2026): it tells the person their photos arrived without fetching
            any image — the image route needs the bearer and would cost one call per photo per opening. */}
        {yeu_cau.photo_count > 0 && (
          <li className="the-tt__dong">
            <span className="the-tt__nhan">{YEU_CAU_CUA_TOI.photos_label}</span>
            <span className="the-tt__gia-tri">{YEU_CAU_CUA_TOI.photo_count(yeu_cau.photo_count)}</span>
          </li>
        )}
      </ul>
      {/* A sentence under the pill: "Đã hoàn tất" alone can read as a refusal to exactly the reader least likely to
          ask again. */}
      <p className="card__body">{GIAI_THICH_TRANG_THAI[yeu_cau.trang_thai]}</p>
    </li>
  );
}

/** Loading: the sentence (`role="status"`) and three flat blocks in the shape of a request card. PURE. */
export function RequestListSkeleton() {
  return (
    <>
      <p className="screen-lead" role="status">
        {YEU_CAU_CUA_TOI.dang_doc}
      </p>
      <div className="vh-skeleton" aria-hidden="true">
        {[0, 1, 2].map((row) => (
          <div className="skel-row" key={row}>
            <div className="skel-lines">
              <span className="skel skel--title" />
              <span className="skel skel--short" />
              <span className="skel" />
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

/** Thân màn theo từng nhánh kết quả, THUẦN — dựng được mọi nhánh mà không cần một máy chủ nào. */
export function ThanDanhSach({
  ket_qua,
  onDangNhapLai,
  onGuiMoi,
  onThuLai,
}: {
  ket_qua: KetQuaDoc | null;
  onDangNhapLai: () => void;
  onGuiMoi: () => void;
  /** Re-read after a failure. Optional so a branch can be rendered alone; the screen always passes it. */
  onThuLai?: () => void;
}) {
  if (ket_qua === null) return <RequestListSkeleton />;

  if (ket_qua.kieu === "chua-dang-nhap") {
    return (
      <div className="vh-state" role="alert">
        <p>{YEU_CAU_CUA_TOI.phien_het_han}</p>
        <button type="button" className="tn__nut" onClick={onDangNhapLai}>
          {YEU_CAU_CUA_TOI.nut_toi_dang_nhap}
        </button>
      </div>
    );
  }

  if (ket_qua.kieu === "chua-khai-host" || ket_qua.kieu === "khong-goi-duoc" || ket_qua.kieu === "server-error") {
    // Three failures, three next steps: the build (say "gián đoạn"), the phone's network, the server (wait).
    const sentence =
      ket_qua.kieu === "chua-khai-host"
        ? YEU_CAU_CUA_TOI.chua_khai_host
        : ket_qua.kieu === "server-error"
          ? YEU_CAU_CUA_TOI.server_error
          : YEU_CAU_CUA_TOI.khong_doc_duoc;
    return (
      <div className="vh-state vh-state--error" role="alert">
        <span className="vh-state__circle" aria-hidden="true">
          <AlertGlyph />
        </span>
        <p className="vh-state__title">{YEU_CAU_CUA_TOI.tieu_de_loi}</p>
        <p>{sentence}</p>
        {onThuLai !== undefined && (
          <button type="button" className="tn__nut-phu" onClick={onThuLai}>
            {YEU_CAU_CUA_TOI.nut_thu_lai}
          </button>
        )}
      </div>
    );
  }

  if (ket_qua.danh_sach.length === 0) {
    return (
      <div className="vh-state">
        <span className="vh-state__circle" aria-hidden="true">
          <InboxGlyph />
        </span>
        <p className="vh-state__title">{YEU_CAU_CUA_TOI.rong}</p>
        <p>{YEU_CAU_CUA_TOI.rong_goi_y}</p>
        <button type="button" className="tn__nut" onClick={onGuiMoi}>
          {YEU_CAU_CUA_TOI.nut_gui_moi}
        </button>
      </div>
    );
  }

  return (
    <ul className="vh-request-list">
      {ket_qua.danh_sach.map((mot) => (
        <DongYeuCau key={mot.ma} yeu_cau={mot} />
      ))}
    </ul>
  );
}

/**
 * The login invitation's words; `expired` = a read just answered 401 and the session was forgotten. PURE.
 */
export function requestsLoginInvite(expired: boolean): { title: string; reason: string } {
  return expired
    ? { title: YEU_CAU_CUA_TOI.session_expired_title, reason: YEU_CAU_CUA_TOI.phien_het_han }
    : { title: YEU_CAU_CUA_TOI.can_dang_nhap_tieu_de, reason: YEU_CAU_CUA_TOI.can_dang_nhap };
}

/**
 * WHAT A PULL DOES TO WHAT IS ON SCREEN. PURE.
 *
 *   fresh list, or "signed out"        → shown, and the pull succeeded (a signed-out session must be said)
 *   a failure while a LIST is shown    → the list stays; the pull failed, and the band says so
 *   a failure while nothing useful is  → the new failure is shown (it carries its own "Thử lại")
 */
export function afterPullRefresh(shown: KetQuaDoc | null, fresh: KetQuaDoc): { shown: KetQuaDoc; ok: boolean } {
  if (fresh.kieu === "xong" || fresh.kieu === "chua-dang-nhap") return { shown: fresh, ok: true };
  if (shown !== null && shown.kieu === "xong") return { shown, ok: false };
  return { shown: fresh, ok: true };
}

export function YeuCauCuaToiScreen({ onDi }: ThamSoMan) {
  const di = onDi ?? (() => {});
  const { phien, datPhien } = dungPhien();
  const bearer = bearerCua(phien);

  const [ket_qua, datKetQua] = useState<KetQuaDoc | null>(null);
  /** A read answered 401 and the session was forgotten — the login card then says it expired. Memory only. */
  const [expired, setExpired] = useState(false);
  const forget = () => {
    setExpired(true);
    datPhien(null);
  };
  /** Đếm số lần bấm "đọc lại" — không có nó thì bấm hai lần liên tiếp chỉ đọc một lần. */
  const [lan_doc, datLanDoc] = useState(0);

  useEffect(() => {
    if (bearer === "") return;
    let con_tren_man = true;
    datKetQua(null);
    void docYeuCauCuaToi(bearer).then((ra) => {
      if (con_tren_man) datKetQua(forgetSessionOn401(ra, forget));
    });
    return () => {
      con_tren_man = false;
    };
    // `forget` is left out on purpose: it is a new function every render, and listing it would re-read the list
    // on every render. It only calls two state setters, which React keeps stable.
  }, [bearer, lan_doc]);

  /**
   * What is on screen NOW, for the pull below. A ref, not the updater form of `datKetQua`: an updater may run after
   * this function returns (React batches), and the pull must answer its success from the same decision it renders.
   */
  const shown = useRef(ket_qua);
  shown.current = ket_qua;

  /** The pull: re-read WITHOUT blanking the list into a skeleton; `afterPullRefresh` decides what stays. */
  async function pullRefresh(): Promise<boolean> {
    if (bearer === "") return true;
    const next = afterPullRefresh(shown.current, forgetSessionOn401(await docYeuCauCuaToi(bearer), forget));
    datKetQua(next.shown);
    return next.ok;
  }

  const banner = (
    <section className="banner">
      <div className="banner__content">
        <span className="tile tile--lon" aria-hidden="true">
          <GridGlyph className="tile__glyph" />
        </span>
        <h1 className="banner__title">{YEU_CAU_CUA_TOI.tieu_de}</h1>
      </div>
    </section>
  );

  // `LOGIN_FROM_REQUESTS`: once logged in, Liên hệ offers "Quay lại Yêu cầu của tôi" back here.
  const toLogin = () => di({ man: "contact", moc: LOGIN_FROM_REQUESTS });

  if (phien === null) {
    const invite = requestsLoginInvite(expired);
    return (
      <>
        {banner}
        <MoiDangNhap
          tieu_de={invite.title}
          li_do={invite.reason}
          nhan_nut={YEU_CAU_CUA_TOI.nut_toi_dang_nhap}
          onDangNhap={toLogin}
        />
      </>
    );
  }

  return (
    <IntroPullToRefresh onRefresh={pullRefresh}>
      {banner}
      <p className="screen-lead">{YEU_CAU_CUA_TOI.dan_nhap}</p>
      <ThanDanhSach
        ket_qua={ket_qua}
        onDangNhapLai={toLogin}
        onGuiMoi={() => di({ man: "tu-van" })}
        onThuLai={() => datLanDoc((n) => n + 1)}
      />
      {/* The button stays for whoever cannot pull — a screen-reader user, or someone who does not know the gesture. */}
      {ket_qua !== null && ket_qua.kieu === "xong" && ket_qua.danh_sach.length > 0 && (
        <button type="button" className="tn__nut-phu" onClick={() => datLanDoc((n) => n + 1)}>
          {YEU_CAU_CUA_TOI.nut_doc_lai}
        </button>
      )}
    </IntroPullToRefresh>
  );
}
