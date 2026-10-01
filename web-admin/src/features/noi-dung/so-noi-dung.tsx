"use client";

import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import type { KetQua } from "@/lib/api/goi";
import {
  layDanhMucNoiDung,
  layMotNoiDung,
  laySoNoiDung,
  CONTENT_TYPE_BANNER,
  CONTENT_TYPE_BROADCAST,
  CONTENT_TYPE_EVENT,
  CONTENT_TYPE_VIDEO,
  deleteContentCategory,
  suaNoiDung,
  themDanhMucNoiDung,
  themNoiDung,
  TU_KHOA_TIM_TOI_DA,
  updateContentCategory,
  type BoLocNoiDung,
} from "@/lib/api/noi-dung";
import type {
  comms_coverImageOut,
  comms_danhMucRa,
  comms_danhSachDanhMucRa,
  comms_noiDungRa,
  page_Result_comms_noiDungRa,
} from "@/lib/api/schema.gen";

import {
  BANNER_APP_PATHS,
  BANNER_ARTICLE_PATH_HINT,
  BANNER_COVER_NOTICE,
  bannerLinkTapWarning,
  CANH_BAO_HTML_THO,
  CHUA_XEP_DANH_MUC,
  coThayDoi,
  DANG_TAI_SO,
  DANG_TAI_TOAN_VAN,
  DANH_MUC_RONG,
  DAU_GACH,
  DISPLAY_ORDER_HINT,
  DISPLAY_ORDER_MAX,
  dungCayDanhMuc,
  EVENT_PLACE_MAX_CHARS,
  EVENT_TIME_HINT,
  FORM_TRONG,
  GHI_CHU_KHONG_CO_XOA,
  giaTriTuHang,
  KHONG_CO_GI_DOI,
  LINK_TO_HINT,
  LINK_TO_MAX_CHARS,
  LOAI_MAC_DINH,
  lopChipTrangThai,
  MO_TA_FORM_THEM,
  MO_TA_THE_DANH_BA,
  MOI_DANH_MUC,
  MOI_LOAI,
  MOI_LOAI_NHAN,
  NHAN_DA_SUA_TAY,
  NHAN_NUT_DANH_MUC,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  nhanLoai,
  nhanMoc,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanNguon,
  nhanTepDinhKem,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
  publishedAtLabel,
  SLUG_DANH_MUC_TOI_DA,
  SO_RONG,
  TEN_DANH_MUC_TOI_DA,
  TIEU_DE_FORM_THEM,
  TIEU_DE_THE_DANH_BA,
  TIEU_DE_TOI_DA,
  THAN_BAI_RONG,
  THU_TU_DANH_MUC_TOI_DA,
  TIM_PLACEHOLDER,
  TOM_TAT_TOI_DA,
  tenDanhMuc,
  thanSua,
  thanThem,
  trichTomTat,
  URL_TOI_DA,
  validateTypeFields,
  VIDEO_URL_HINT,
  type GiaTriFormNoiDung,
} from "./nhan-noi-dung";
import {
  COVER_ACCEPT,
  COVER_HINT,
  COVER_NONE,
  COVER_PICK_BUTTON,
  COVER_PREVIEW_ALT,
  COVER_PREVIEW_MISSING,
  COVER_REMOVE_BUTTON,
  COVER_REPLACE_BUTTON,
  COVER_RETRY_BUTTON,
  COVER_WAIT_NOTE,
  COVER_WILL_DETACH,
  coverInFlight,
  coverPreviewSrc,
  coverStateText,
  retryCoverCompletion,
  runCoverUpload,
  savedCoverText,
  type CoverUploadState,
} from "./cover-image";
import { AUDIO_SAVE_FIRST, AUDIO_WAIT_NOTE, type AudioUploadState } from "./broadcast-audio";
import { BroadcastAudioField } from "./broadcast-audio-field";
import { CategoryAdmin } from "./category-admin";
import { RichTextEditor } from "./rich-text-editor";

/**
 * Màn "Nội dung Mini App" — `docs/ui-ux/11-noi-dung-mini-app.md` §2 (bố cục), §5 (sáu tab),
 * §6 (bảng), §7 (biểu mẫu).
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: **KHÔNG MỘT DÒNG NÀO Ở ĐÂY ĐƯA MỘT CHUỖI HTML VÀO TRANG**.
 *
 * `noi_dung` là HTML (§8). The server sanitises it on every write to the ADR 0067 §1 allow-list, and
 * the body is edited in Tiptap (`rich-text-editor.tsx`), which draws the document from its parsed
 * structure. So: NO `dangerouslySetInnerHTML` anywhere, not even for a preview — the editor IS the
 * preview. `ranh-gioi-html.test.ts` reads this folder's source and turns red if the string appears.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG CÓ NÚT XOÁ MỘT BÀI, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT CÂU TRẢ LỜI. §6 chỉ vẽ `✎`, hợp đồng không có
 * tuyến `DELETE` nào cho một mục nội dung. Gỡ một bài khỏi Mini App là tắt ô `Đăng lên Mini App` ở màn
 * sửa. (A CATEGORY has a soft delete with a reason — `category-admin.tsx`, ADR 0067 §3.)
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở CLIENT — xem `PHAN_CHUA_DUNG`. `service-comms` kiểm `content.read` /
 * `content.update` trên TỪNG lời gọi; tài khoản thiếu khoá nhận nguyên câu 403 ra màn hình. Ẩn một
 * nút chưa bao giờ là biện pháp (luật 5, cấm #1).
 *
 * TIÊU ĐỀ, TÓM TẮT VÀ THÂN BÀI LÀ TIN BÀI CỦA XÃ, thường nhắc tên và hoàn cảnh của một công dân cụ
 * thể: không dòng nào ở đây ghi chúng vào log, vào tên tệp hay vào một URL (luật 3, cấm #1 và #4).
 */

/** Bao nhiêu hàng một trang của bảng §6. */
const SO_HANG_MOI_TRANG = 20;

type TrangThaiTai<T> =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: T };

function taiTu<T>(daTai: { khoa: string; kq: KetQua<T> } | null, khoa: string): TrangThaiTai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

/** Bộ lọc §6 mà màn giữ, KHÔNG gồm con trỏ — con trỏ do ngăn xếp phân trang giữ. */
type BoLocMan = {
  readonly loai: string;
  readonly danhMucID: string;
  readonly tim: string;
};

const LOC_TRONG: BoLocMan = { loai: "", danhMucID: "", tim: "" };

export function SoNoiDung() {
  const [loc, datLoc] = useState<BoLocMan>(LOC_TRONG);
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_comms_noiDungRa>;
  } | null>(null);

  const [danhMuc, datDanhMuc] = useState<KetQua<comms_danhSachDanhMucRa> | null>(null);
  const [lanTaiDanhMuc, datLanTaiDanhMuc] = useState(0);

  const [dangMoThem, datDangMoThem] = useState(false);
  const [dangMoDanhMuc, datDangMoDanhMuc] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loiForm, datLoiForm] = useState<string | null>(null);
  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của biểu mẫu, nên một lần ghi xong là một lần biểu
  // mẫu dựng lại từ đầu: các ô trống trở lại VÀ một khoá chống trùng mới được sinh. Lần ghi HỎNG
  // thì không tăng — biểu mẫu giữ nguyên chữ đã gõ và giữ nguyên khoá cũ, đúng điều
  // `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // Id của hàng đang sửa, KHÔNG phải cả bản ghi: hàng của DANH SÁCH không mang thân bài, nên mở
  // biểu mẫu sửa từ nó sẽ cho một ô nội dung rỗng và lần Lưu đầu tiên xoá trắng bài viết. Toàn văn
  // phải đi hỏi tuyến chi tiết.
  const [dangSuaID, datDangSuaID] = useState<string | null>(null);
  const [daTaiChiTiet, datDaTaiChiTiet] = useState<{
    khoa: string;
    kq: KetQua<comms_noiDungRa>;
  } | null>(null);

  const khoaSo = `${loc.loai}|${loc.danhMucID}|${loc.tim}|${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    const yc: BoLocNoiDung = {
      loai: loc.loai,
      danhMucID: loc.danhMucID,
      tim: loc.tim,
      limit: SO_HANG_MOI_TRANG,
      cursor: nganXep.hienTai,
    };
    laySoNoiDung(yc).then((kq) => {
      if (!bo) datDaTai({ khoa: khoaSo, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc.loai, loc.danhMucID, loc.tim, nganXep.hienTai, khoaSo]);

  useEffect(() => {
    let bo = false;
    layDanhMucNoiDung().then((kq) => {
      if (!bo) datDanhMuc(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanTaiDanhMuc]);

  const khoaChiTiet = `${dangSuaID ?? ""}|${lanGhiXong}`;

  useEffect(() => {
    if (dangSuaID === null) return;
    let bo = false;
    layMotNoiDung(dangSuaID).then((kq) => {
      if (!bo) datDaTaiChiTiet({ khoa: khoaChiTiet, kq });
    });
    return () => {
      bo = true;
    };
  }, [dangSuaID, khoaChiTiet]);

  const so = taiTu(daTai, khoaSo);
  const dsDanhMuc: readonly comms_danhMucRa[] =
    danhMuc !== null && danhMuc.ok ? danhMuc.duLieu.items : [];
  const chiTiet = dangSuaID === null ? null : taiTu(daTaiChiTiet, khoaChiTiet);

  /** Sau mỗi lần ghi xong: về trang đầu và đọc lại sổ. */
  function taiLaiSo(): void {
    datNganXep(TRANG_DAU);
    datLanTai((n) => n + 1);
  }

  function dongMoiBieuMau(): void {
    datDangMoThem(false);
    datDangMoDanhMuc(false);
    datDangSuaID(null);
    datLoiForm(null);
  }

  function themBai(gt: GiaTriFormNoiDung, khoa: string): void {
    datDangGui(true);
    themNoiDung(thanThem(gt), khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. Câu 400 của nó nói đúng trường sai và đúng quy tắc bị chạm —
        // viết lại ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangMoThem(false);
      datLanGhiXong((n) => n + 1);
      taiLaiSo();
    });
  }

  function suaBai(id: string, dau: GiaTriFormNoiDung, moi: GiaTriFormNoiDung): void {
    const than = thanSua(dau, moi);
    if (!coThayDoi(than)) {
      // KHÔNG GỌI `PATCH` RỖNG. Một lần Lưu không đổi gì vẫn là một lần ghi ở máy chủ, và §10.4
      // gắn cờ "đã sửa tay" lên bài — cờ ấy đưa bài ra khỏi lượt đồng bộ VĨNH VIỄN.
      datLoiForm(KHONG_CO_GI_DOI);
      return;
    }
    datDangGui(true);
    suaNoiDung(id, than).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangSuaID(null);
      datLanGhiXong((n) => n + 1);
      taiLaiSo();
    });
  }

  function themDanhMuc(ten: string, slug: string, chaID: string, thuTu: number, khoa: string): void {
    datDangGui(true);
    themDanhMucNoiDung({ name: ten, slug, parent_id: chaID, order: thuTu }, khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangMoDanhMuc(false);
      datLanGhiXong((n) => n + 1);
      datLanTaiDanhMuc((n) => n + 1);
    });
  }

  return (
    <section className="man-noi-dung" aria-labelledby="tieu-de-so-noi-dung">
      <h2 id="tieu-de-so-noi-dung">Sổ nội dung Mini App</h2>

      <KhoiChuaDung />
      <TheDanhBaChinhQuyen />

      <div className="cum-nut">
        <button
          type="button"
          className="nut-chinh"
          aria-expanded={dangMoThem}
          onClick={() => {
            dongMoiBieuMau();
            datDangMoThem(!dangMoThem);
          }}
        >
          {NHAN_NUT_THEM}
        </button>
        <button
          type="button"
          className="nut-phu"
          aria-expanded={dangMoDanhMuc}
          onClick={() => {
            dongMoiBieuMau();
            datDangMoDanhMuc(!dangMoDanhMuc);
          }}
        >
          {NHAN_NUT_DANH_MUC}
        </button>
      </div>

      {dangMoThem && (
        <FormNoiDung
          // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
          key={`them|${lanGhiXong}`}
          tieuDeForm={TIEU_DE_FORM_THEM}
          moTa={MO_TA_FORM_THEM}
          giaTriDau={FORM_TRONG}
          danhMuc={dsDanhMuc}
          dangGui={dangGui}
          loi={loiForm}
          huy={dongMoiBieuMau}
          luu={(gt, khoa) => themBai(gt, khoa)}
        />
      )}

      {dangMoDanhMuc && (
        <>
          <CategoryAdmin
            categories={dsDanhMuc}
            update={updateContentCategory}
            remove={deleteContentCategory}
            changed={() => datLanTaiDanhMuc((n) => n + 1)}
          />
          <FormDanhMuc
            key={`danh-muc|${lanGhiXong}`}
            danhMuc={dsDanhMuc}
            dangGui={dangGui}
            loi={loiForm}
            huy={dongMoiBieuMau}
            luu={themDanhMuc}
          />
        </>
      )}

      {chiTiet !== null && chiTiet.pha === "dangTai" && <p role="status">{DANG_TAI_TOAN_VAN}</p>}
      {chiTiet !== null && chiTiet.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {chiTiet.thongBao}
        </p>
      )}
      {chiTiet !== null && chiTiet.pha === "xong" && dangSuaID !== null && (
        <FormNoiDung
          key={`sua|${dangSuaID}|${lanGhiXong}`}
          tieuDeForm={`Sửa nội dung: ${chiTiet.duLieu.title}`}
          moTa={MO_TA_FORM_THEM}
          giaTriDau={giaTriTuHang(chiTiet.duLieu)}
          hang={chiTiet.duLieu}
          danhMuc={dsDanhMuc}
          dangGui={dangGui}
          loi={loiForm}
          huy={dongMoiBieuMau}
          luu={(gt) => suaBai(dangSuaID, giaTriTuHang(chiTiet.duLieu), gt)}
        />
      )}

      <ThanhTabLoai
        loai={loc.loai}
        datLoai={(l) => {
          datNganXep(TRANG_DAU);
          datLoc({ ...loc, loai: l });
        }}
      />

      <HangLocNoiDung
        danhMucID={loc.danhMucID}
        datDanhMucID={(id) => {
          datNganXep(TRANG_DAU);
          datLoc({ ...loc, danhMucID: id });
        }}
        tim={tim}
        datTim={datTim}
        timNgay={() => {
          datNganXep(TRANG_DAU);
          datLoc({ ...loc, tim: tim.trim() });
        }}
        danhMuc={dsDanhMuc}
      />

      {danhMuc !== null && !danhMuc.ok && (
        <p className="thong-bao-loi" role="alert">
          {danhMuc.thongBao}
        </p>
      )}

      {so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
      )}

      {so.pha === "xong" && (
        <>
          <BangNoiDung ds={so.duLieu.items} danhMuc={dsDanhMuc} sua={datDangSuaID} />
          <nav className="dieu-huong-trang" aria-label="Phân trang sổ nội dung Mini App">
            <button
              type="button"
              className="nut-phu"
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </button>
            <button
              type="button"
              className="nut-phu"
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
            </button>
          </nav>
        </>
      )}
    </section>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý
        do
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

/**
 * Thẻ §4 — Danh bạ chính quyền.
 *
 * LIÊN KẾT CÓ, CON SỐ KHÔNG. `Đang hiện 26 cán bộ cho bà con` đếm cờ `hien_tren_mini_app` trên danh
 * bạ, thứ `service-identity` sở hữu và không tuyến nào hôm nay trả về. Hiện một số 0 ở đó là nói
 * với xã rằng bà con không thấy cán bộ nào — xem `PHAN_CHUA_DUNG`.
 */
export function TheDanhBaChinhQuyen() {
  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3>📖 {TIEU_DE_THE_DANH_BA}</h3>
      </div>
      <p>{MO_TA_THE_DANH_BA}</p>
      <Link className="nut-phu" href="/danh-ba">
        Mở danh bạ cán bộ →
      </Link>
    </div>
  );
}

/**
 * Sáu tab loại nội dung §5, cộng một tab `Tất cả`.
 *
 * TAB `Tất cả` KHÔNG CÓ TRONG §5, và nó được thêm vào có chủ ý chứ không phải do nhầm: sổ phải mở
 * được ở trạng thái không lọc, vì `type` VẮNG là hình dạng duy nhất nói "mọi loại" — gửi `type=`
 * rỗng thì máy chủ vẫn coi là không lọc, nhưng màn hình sẽ không có nút nào để quay về đó.
 *
 * SÁU NÚT `aria-pressed`, KHÔNG PHẢI TAB THẬT: `globals.css` chưa có lớp nào cho tab và lượt này
 * không được thêm CSS. `.thanh-sap-xep` đã có sẵn kiểu cho một hàng nút bật/tắt.
 */
export function ThanhTabLoai({
  loai,
  datLoai,
}: {
  loai: string;
  datLoai: (l: string) => void;
}) {
  return (
    <div className="thanh-sap-xep" role="group" aria-label="Loại nội dung">
      <button
        type="button"
        className="nut-phu"
        aria-pressed={loai === ""}
        onClick={() => datLoai("")}
      >
        {MOI_LOAI_NHAN}
      </button>
      {MOI_LOAI.map((ma) => (
        <button
          key={ma}
          type="button"
          className="nut-phu"
          aria-pressed={loai === ma}
          onClick={() => datLoai(ma)}
        >
          {nhanLoai(ma)}
        </button>
      ))}
    </div>
  );
}

/** Hàng lọc §6 — ô tìm theo tiêu đề và ô chọn danh mục. */
export function HangLocNoiDung({
  danhMucID,
  datDanhMucID,
  tim,
  datTim,
  timNgay,
  danhMuc,
}: {
  danhMucID: string;
  datDanhMucID: (id: string) => void;
  tim: string;
  datTim: (s: string) => void;
  timNgay: () => void;
  danhMuc: readonly comms_danhMucRa[];
}) {
  function gui(e: FormEvent) {
    e.preventDefault();
    timNgay();
  }

  return (
    <div className="hang-loc">
      {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
          đang gõ vào một URL, và một URL đi vào mọi log truy cập (luật 3, cấm #4). */}
      <form className="form-tra-cuu" onSubmit={gui} role="search">
        <div className="o-nhap">
          <label htmlFor="tim-noi-dung">Tìm theo tiêu đề</label>
          <input
            id="tim-noi-dung"
            name="tim-noi-dung"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            autoComplete="off"
            // Máy chủ trả 400 khi quá trần, và câu từ chối CỐ Ý không nhắc lại chữ vừa gõ. Chặn ở ô
            // nhập để cán bộ thấy giới hạn thay vì thấy "không tải được".
            maxLength={TU_KHOA_TIM_TOI_DA}
            onChange={(e) => datTim(e.target.value)}
          />
        </div>
        <button className="nut-phu" type="submit">
          Tìm
        </button>
      </form>

      <div className="o-chon">
        <label htmlFor="loc-danh-muc">Danh mục</label>
        <select
          id="loc-danh-muc"
          value={danhMucID}
          onChange={(e) => datDanhMucID(e.target.value)}
        >
          <option value="">{MOI_DANH_MUC}</option>
          {dungCayDanhMuc(danhMuc).map((m) => (
            <option key={m.dm.id} value={m.dm.id}>
              {nhanMucDanhMuc(m)}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

/**
 * Bảng §6.
 *
 * CỘT HÀNH ĐỘNG CHỈ CÓ `✎`. Không nút xoá: không tuyến nào xoá được, và §6 cũng chỉ vẽ một ký hiệu.
 * Một dòng chữ dưới bảng nói vì sao, thay vì một nút mờ — một nút mờ nói "bạn không có quyền",
 * trong khi sự thật là chức năng không tồn tại, và hai câu ấy không được lẫn vào nhau.
 *
 * KHÔNG CỘT NÀO DỰNG HTML. `title` và `summary` là văn bản thuần ở đây; thân bài không có mặt
 * trong phản hồi danh sách và màn hình không đi đoán nó.
 */
export function BangNoiDung({
  ds,
  danhMuc,
  sua,
}: {
  ds: readonly comms_noiDungRa[];
  danhMuc: readonly comms_danhMucRa[];
  sua: (id: string) => void;
}) {
  if (ds.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    <>
      <div className="bang-cuon">
        <table className="bang-can-bo">
          <caption className="an-thi-giac">Sổ nội dung Mini App của xã</caption>
          <thead>
            <tr>
              <th scope="col">Tiêu đề</th>
              <th scope="col">Loại</th>
              <th scope="col">Chuyên mục</th>
              <th scope="col">Tệp đính kèm</th>
              <th scope="col">Ngày đăng</th>
              <th scope="col">Trạng thái</th>
              <th scope="col">Sửa</th>
            </tr>
          </thead>
          <tbody>
            {ds.map((nd) => {
              const trich = trichTomTat(nd.summary);
              const firstPublished = publishedAtLabel(nd);
              return (
                <tr key={nd.id}>
                  <td>
                    <span className="ten-can-bo">{nd.title}</span>
                    {trich !== "" && <span className="dong-phu">{trich}</span>}
                    {nd.hand_edited && <span className="dong-phu">{NHAN_DA_SUA_TAY}</span>}
                  </td>
                  <td>{nhanLoai(nd.type)}</td>
                  <td>{tenDanhMuc(nd.category_id, danhMuc)}</td>
                  <td>{nhanTepDinhKem(nd.has_image)}</td>
                  <td>
                    {nhanNgayDang(nd.published_on)}
                    {firstPublished !== null && <span className="dong-phu">{firstPublished}</span>}
                  </td>
                  <td>
                    <span className={lopChipTrangThai(nd.status)}>{nhanTrangThai(nd.status)}</span>
                  </td>
                  <td className="o-thao-tac">
                    <button
                      type="button"
                      className="nut-phu"
                      onClick={() => sua(nd.id)}
                      // Ký hiệu một mình không đọc được bằng trình đọc màn hình, và sáu hàng đều
                      // mang cùng một ký hiệu. Nhãn mang theo tiêu đề để nói rõ đang sửa bài nào.
                      aria-label={`${NHAN_NUT_SUA} Sửa: ${nd.title}`}
                    >
                      {NHAN_NUT_SUA}
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p className="ghi-chu">{GHI_CHU_KHONG_CO_XOA}</p>
    </>
  );
}

/**
 * Biểu mẫu §7, dùng cho cả THÊM và SỬA.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL. Ở đây nó là một khối nằm trong trang — KHÔNG phải một lớp phủ — vì một
 * lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này không được thêm CSS.
 *
 * Ô `Nội dung` IS THE TIPTAP EDITOR (`RichTextEditor`), limited to the server's allow-list. It reports
 * a change only when the officer edits, so `thanSua` never sends a body nobody touched.
 *
 * KHÔNG CÓ Ô `Trạng thái`: máy chủ suy trạng thái từ ô tích `Đăng lên Mini App`. Một thân tự khai
 * trạng thái là một bài đăng vượt qua bước duyệt mà §10.2 dành cho lượt đồng bộ.
 *
 * `khoaChongTrung` SINH LÚC MỞ BIỂU MẪU, không lúc gửi: bấm lại sau một lỗi mạng phải dùng LẠI
 * đúng khoá ấy, vì lần gửi đầu có thể đã tới máy chủ và đã đăng một bài lên Mini App của cả xã.
 * Biểu mẫu SỬA không cần khoá (hợp đồng không đòi), nhưng vẫn nhận cùng chữ ký để hai lối gọi
 * không rẽ nhánh ở đây.
 */
export function FormNoiDung({
  tieuDeForm,
  moTa,
  giaTriDau,
  hang,
  danhMuc,
  dangGui,
  loi,
  huy,
  luu,
  initialCoverState,
  initialAudioState,
}: {
  tieuDeForm: string;
  moTa: string;
  giaTriDau: GiaTriFormNoiDung;
  /** Hàng gốc, chỉ có khi đang SỬA — dùng cho khối thông tin chỉ đọc. */
  hang?: comms_noiDungRa;
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormNoiDung, khoaChongTrung: string) => void;
  /** Tests only: render the cover block in a given upload state (this suite has no DOM events). */
  initialCoverState?: CoverUploadState;
  /** Tests only: render the audio block in a given upload state. */
  initialAudioState?: AudioUploadState;
}) {
  const [gt, datGT] = useState<GiaTriFormNoiDung>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const [cover, setCover] = useState<CoverUploadState>(initialCoverState ?? { kind: "idle" });
  const [audioBusy, setAudioBusy] = useState(false);

  const tieuDeGon = gt.title.trim();
  // MỘT ĐIỀU KIỆN, KHÔNG BA. Hợp đồng đánh dấu `title` và `type` bắt buộc; `type` luôn có giá trị
  // vì ô chọn mặc định `Tin tức` và danh sách đóng. Máy chủ vẫn là nơi từ chối thật; nút tắt chỉ
  // để không phải gõ lại.
  //
  // The per-type check is shown LIVE next to the boxes and holds Lưu off: the author sees "end before
  // start" while fixing it, not after a round trip. It mirrors the server; the server's own 400 still
  // reaches `loi` word for word.
  // The edit form passes its starting values: two banner rules depend on what the item WAS.
  const typeFieldError = validateTypeFields(gt, hang === undefined ? undefined : giaTriDau);
  // A cover still moving holds Lưu: saving now would drop the image the officer just chose.
  const coverBusy = coverInFlight(cover);
  // An audio upload still moving holds Lưu too: closing the form mid-upload would hide its outcome.
  const duDieuKien = tieuDeGon !== "" && typeFieldError === null && !coverBusy && !audioBusy;
  // The audio is uploaded for a SAVED broadcast only (the server requires `content_item_id`).
  const savedAsBroadcast = hang !== undefined && hang.type === CONTENT_TYPE_BROADCAST;
  const bannerLinkWarning = gt.type === CONTENT_TYPE_BANNER ? bannerLinkTapWarning(gt.link_to) : null;

  // Each state report; a `ready` one makes its id the form's cover. Functional update: the upload
  // resolves after renders the officer may have made meanwhile (typing the title), and those stay.
  function onCoverState(s: CoverUploadState): void {
    setCover(s);
    if (s.kind === "ready") datGT((g) => ({ ...g, cover_image_file_id: s.id }));
  }

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    luu(gt, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={guiNgay} aria-labelledby="tieu-de-form-noi-dung">
      <h3 id="tieu-de-form-noi-dung">{tieuDeForm}</h3>
      <p className="ghi-chu">{moTa}</p>

      {hang !== undefined && <ThongTinChiDoc hang={hang} />}

      <div className="o-chon">
        <label htmlFor="loai-noi-dung">Loại nội dung</label>
        <select
          id="loai-noi-dung"
          value={gt.type === "" ? LOAI_MAC_DINH : gt.type}
          onChange={(e) => datGT({ ...gt, type: e.target.value })}
        >
          {MOI_LOAI.map((ma) => (
            <option key={ma} value={ma}>
              {nhanLoai(ma)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="danh-muc-noi-dung">Danh mục</label>
        <select
          id="danh-muc-noi-dung"
          value={gt.category_id}
          onChange={(e) => datGT({ ...gt, category_id: e.target.value })}
        >
          <option value="">{CHUA_XEP_DANH_MUC}</option>
          {dungCayDanhMuc(danhMuc).map((m) => (
            <option key={m.dm.id} value={m.dm.id}>
              {nhanMucDanhMuc(m)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-nhap">
        <label htmlFor="tieu-de-noi-dung">Tiêu đề *</label>
        <input
          id="tieu-de-noi-dung"
          name="tieu-de-noi-dung"
          value={gt.title}
          maxLength={TIEU_DE_TOI_DA}
          autoComplete="off"
          onChange={(e) => datGT({ ...gt, title: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="tom-tat-noi-dung">Tóm tắt</label>
        <textarea
          id="tom-tat-noi-dung"
          name="tom-tat-noi-dung"
          rows={3}
          value={gt.summary}
          maxLength={TOM_TAT_TOI_DA}
          onChange={(e) => datGT({ ...gt, summary: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <span id="than-bai-noi-dung-nhan" className="nhan-o">
          Nội dung
        </span>
        {/* The editor starts from the body as it was when the form opened (`giaTriDau`), never from
            `gt.body`: feeding its own output back in would reset the cursor on every keystroke. */}
        <RichTextEditor
          id="than-bai-noi-dung"
          labelId="than-bai-noi-dung-nhan"
          initialHtml={giaTriDau.body}
          disabled={dangGui}
          onChange={(body) => datGT((g) => ({ ...g, body }))}
        />
        <p className="ghi-chu" id="than-bai-noi-dung-hint">
          {CANH_BAO_HTML_THO}
        </p>
        {gt.body === "" && <p className="ghi-chu">{THAN_BAI_RONG}</p>}
      </div>

      <CoverImageField
        fileId={gt.cover_image_file_id}
        savedFileId={hang?.cover_image_file_id ?? ""}
        savedCover={hang?.cover_image}
        state={cover}
        disabled={dangGui}
        onPick={(file) => void runCoverUpload(file, hang?.id, onCoverState)}
        onRetry={(id) => void retryCoverCompletion(id, onCoverState)}
        onRemove={() => {
          setCover({ kind: "idle" });
          datGT((g) => ({ ...g, cover_image_file_id: "" }));
        }}
      />

      {/* :131 of the spec, ADR 0047 §6. Rendered only for the type that can carry them — the server
          refuses an event field on any other type, and a box that is shown is a box somebody fills. */}
      {gt.type === CONTENT_TYPE_EVENT && (
        <>
          <div className="o-nhap">
            <label htmlFor="bat-dau-su-kien">Bắt đầu</label>
            <input
              id="bat-dau-su-kien"
              name="bat-dau-su-kien"
              type="datetime-local"
              value={gt.event_starts_local}
              onChange={(e) => datGT({ ...gt, event_starts_local: e.target.value })}
            />
          </div>
          <div className="o-nhap">
            <label htmlFor="ket-thuc-su-kien">Kết thúc</label>
            <input
              id="ket-thuc-su-kien"
              name="ket-thuc-su-kien"
              type="datetime-local"
              value={gt.event_ends_local}
              onChange={(e) => datGT({ ...gt, event_ends_local: e.target.value })}
            />
            <p className="ghi-chu">{EVENT_TIME_HINT}</p>
          </div>
          <div className="o-nhap">
            <label htmlFor="dia-diem-su-kien">Địa điểm</label>
            <input
              id="dia-diem-su-kien"
              name="dia-diem-su-kien"
              value={gt.event_place}
              maxLength={EVENT_PLACE_MAX_CHARS}
              autoComplete="off"
              onChange={(e) => datGT({ ...gt, event_place: e.target.value })}
            />
          </div>
        </>
      )}

      {gt.type === CONTENT_TYPE_VIDEO && (
        <div className="o-nhap">
          <label htmlFor="lien-ket-video">Liên kết video</label>
          <input
            id="lien-ket-video"
            name="lien-ket-video"
            type="url"
            value={gt.video_url}
            maxLength={URL_TOI_DA}
            placeholder="https://"
            autoComplete="off"
            onChange={(e) => datGT({ ...gt, video_url: e.target.value })}
          />
          <p className="ghi-chu">{VIDEO_URL_HINT}</p>
        </div>
      )}

      {/* ADR 0067 §4. The audio is uploaded for a SAVED broadcast: on the create form, or on an item
          saved as another type, the officer is told to save first instead of being shown a picker the
          server would refuse (400 / 422 `audio_only_for_truyen_thanh`). */}
      {gt.type === CONTENT_TYPE_BROADCAST && !savedAsBroadcast && (
        <p className="ghi-chu" role="note">
          {AUDIO_SAVE_FIRST}
        </p>
      )}
      {savedAsBroadcast && (
        <BroadcastAudioField
          itemId={hang.id}
          initialAudio={hang.audio ?? null}
          currentType={gt.type}
          disabled={dangGui}
          onBusyChange={setAudioBusy}
          initialState={initialAudioState}
        />
      )}

      {/* ADR 0067 §5. Only for `Banner`: the server refuses `link_to` / `display_order` on any other type
          (422), and a type change away from banner clears both there — so these boxes are not sent then. */}
      {gt.type === CONTENT_TYPE_BANNER && (
        <>
          <p className="ghi-chu">{BANNER_COVER_NOTICE}</p>
          <div className="o-nhap">
            <label htmlFor="lien-ket-banner">Liên kết khi bấm</label>
            <input
              id="lien-ket-banner"
              name="lien-ket-banner"
              value={gt.link_to}
              maxLength={LINK_TO_MAX_CHARS}
              placeholder="/tin-tuc hoặc https://"
              autoComplete="off"
              list="lien-ket-banner-duong-app"
              aria-describedby={
                bannerLinkWarning === null ? "lien-ket-banner-goi-y" : "lien-ket-banner-goi-y lien-ket-banner-canh-bao"
              }
              onChange={(e) => datGT({ ...gt, link_to: e.target.value })}
            />
            {/* The in-app paths the citizen Mini App can open (`BANNER_APP_PATHS`); free https:// still typed. */}
            <datalist id="lien-ket-banner-duong-app">
              {BANNER_APP_PATHS.map((p) => (
                <option key={p.path} value={p.path} label={p.label} />
              ))}
            </datalist>
            <p className="ghi-chu" id="lien-ket-banner-goi-y">
              {LINK_TO_HINT} {BANNER_ARTICLE_PATH_HINT}
            </p>
            {bannerLinkWarning !== null && (
              // A WARNING, not a refusal: the server accepts any in-app path, and the Mini App may learn
              // the screen later. The officer just must not believe it is tappable today.
              <p className="ghi-chu" role="status" id="lien-ket-banner-canh-bao">
                {bannerLinkWarning}
              </p>
            )}
          </div>
          <div className="o-nhap">
            <label htmlFor="thu-tu-banner">Thứ tự hiển thị</label>
            <input
              id="thu-tu-banner"
              name="thu-tu-banner"
              type="number"
              inputMode="numeric"
              min={0}
              max={DISPLAY_ORDER_MAX}
              step={1}
              value={gt.display_order}
              aria-describedby="thu-tu-banner-goi-y"
              onChange={(e) => datGT({ ...gt, display_order: e.target.value })}
            />
            <p className="ghi-chu" id="thu-tu-banner-goi-y">
              {DISPLAY_ORDER_HINT}
            </p>
          </div>
        </>
      )}

      {typeFieldError !== null && (
        <p className="thong-bao-loi" role="alert">
          {typeFieldError}
        </p>
      )}

      <div className="o-nhap">
        <label htmlFor="dang-len-mini-app">
          <input
            id="dang-len-mini-app"
            name="dang-len-mini-app"
            type="checkbox"
            checked={gt.publish}
            onChange={(e) => datGT({ ...gt, publish: e.target.checked })}
          />{" "}
          {NHAN_O_DANG}
        </label>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      {coverBusy && <p className="ghi-chu">{COVER_WAIT_NOTE}</p>}
      {audioBusy && <p className="ghi-chu">{AUDIO_WAIT_NOTE}</p>}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || !duDieuKien}>
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * §7 `Ảnh đại diện` — `Chọn tệp từ máy` · `JPG, PNG hoặc WebP — tối đa 50MB`. Presentational: the
 * form owns the id and the upload state (`cover-image.ts` runs the flow).
 *
 * THE PREVIEW IS THE SERVER'S SIGNED LINK AND NOTHING ELSE (`coverPreviewSrc`), shown only for the
 * cover already saved on the article. A just-uploaded file shows a sentence, not a picture: there is
 * no server preview for it until the article is saved, and a `blob:` of the officer's own file would
 * be a second, unchecked source of `src` on this screen.
 *
 * Same control shape as `📎 Đính kèm` (`task-attachments-ui.tsx`): the file input is visually hidden
 * and stays in the tab order, its label is the visible button, ringed on focus; `.task-attachments`
 * gives every control 44 px and 16 px text (reused rather than a new CSS rule).
 */
export function CoverImageField({
  fileId,
  savedFileId,
  savedCover,
  state,
  disabled,
  onPick,
  onRetry,
  onRemove,
}: {
  /** The form's current `cover_image_file_id`, `""` = none. */
  fileId: string;
  /** The article's cover when the form opened, `""` on the create form. */
  savedFileId: string;
  /** The detail route's cover block, edit form only. */
  savedCover: comms_coverImageOut | null | undefined;
  state: CoverUploadState;
  disabled: boolean;
  onPick: (file: File) => void;
  onRetry: (id: string) => void;
  onRemove: () => void;
}) {
  const busy = coverInFlight(state);
  const showingSaved = fileId !== "" && fileId === savedFileId;
  const previewSrc = showingSaved ? coverPreviewSrc(savedCover) : null;
  const stateText = coverStateText(state);
  const refused = state.kind === "refused" || state.kind === "retry";

  return (
    <div className="o-nhap task-attachments" role="group" aria-labelledby="anh-noi-dung-nhan">
      <span id="anh-noi-dung-nhan">Ảnh đại diện</span>

      {showingSaved && (
        <>
          <p>{savedCoverText(savedCover)}</p>
          {previewSrc !== null ? (
            // A plain <img>, not next/image: the optimiser would fetch this presigned link server-side
            // and cache a bearer credential under the commune's own origin.
            // eslint-disable-next-line @next/next/no-img-element
            <img src={previewSrc} alt={COVER_PREVIEW_ALT} width={240} />
          ) : (
            <p className="ghi-chu">{COVER_PREVIEW_MISSING}</p>
          )}
        </>
      )}
      {fileId === "" && savedFileId !== "" && <p>{COVER_WILL_DETACH}</p>}
      {fileId === "" && savedFileId === "" && <p className="ghi-chu">{COVER_NONE}</p>}

      {/* The input FIRST, visually hidden but focusable; the label right after it is the visible
          button (`input:focus-visible + label`). Enter/Space on the focused input open the picker. */}
      <input
        id="anh-noi-dung"
        name="anh-noi-dung"
        type="file"
        accept={COVER_ACCEPT}
        className="an-thi-giac"
        aria-describedby="anh-noi-dung-goi-y"
        disabled={disabled || busy}
        onChange={(e) => {
          const f = e.target.files?.[0];
          e.target.value = ""; // the same file can be chosen again after a refusal
          if (f !== undefined) onPick(f);
        }}
      />
      <label htmlFor="anh-noi-dung" className="nut-phu">
        {fileId === "" ? COVER_PICK_BUTTON : COVER_REPLACE_BUTTON}
        <span className="an-thi-giac"> — Ảnh đại diện</span>
      </label>
      <p className="ghi-chu" id="anh-noi-dung-goi-y">
        {COVER_HINT}
      </p>

      {stateText !== "" && (
        <p role={refused ? "alert" : "status"} className={refused ? "thong-bao-loi" : undefined}>
          {stateText}
        </p>
      )}

      {(state.kind === "retry" || fileId !== "") && (
        <div className="cum-nut">
          {state.kind === "retry" && (
            <button type="button" className="nut-phu" disabled={disabled} onClick={() => onRetry(state.id)}>
              {COVER_RETRY_BUTTON}
            </button>
          )}
          {fileId !== "" && (
            <button type="button" className="nut-phu" disabled={disabled || busy} onClick={onRemove}>
              {COVER_REMOVE_BUTTON}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Khối chỉ đọc của biểu mẫu SỬA — những thứ máy chủ quyết và biểu mẫu không đổi được.
 *
 * KHÔNG CÓ `luot_xem`: hệ thống không đếm lượt xem (ADR 0047), người dùng bỏ cột ấy 01/10/2026 —
 * xem `PHAN_CHUA_DUNG`. `hand_edited` là nửa nhìn thấy được của §10.4, và
 * đây là chỗ duy nhất màn hình biết được rằng bản sửa vừa rồi nay được bảo vệ khỏi lượt đồng bộ
 * sau. `source_url` hiện dưới dạng CHỮ, không phải một liên kết bấm được: máy chủ chỉ nhận
 * `http(s)` khi GHI, nhưng một hàng cũ trong CSDL không có gì bảo đảm điều đó, và một `href` dựng
 * từ dữ liệu chưa kiểm là đúng lỗ hổng danh sách trắng lược đồ sinh ra để chặn.
 */
export function ThongTinChiDoc({ hang }: { hang: comms_noiDungRa }) {
  const firstPublished = publishedAtLabel(hang);
  return (
    <dl className="danh-sach-truong">
      <div>
        <dt>Trạng thái</dt>
        <dd>
          <span className={lopChipTrangThai(hang.status)}>{nhanTrangThai(hang.status)}</span>
        </dd>
      </div>
      <div>
        <dt>Ngày đăng</dt>
        <dd>
          {nhanNgayDang(hang.published_on)}
          {firstPublished !== null && <> · {firstPublished}</>}
        </dd>
      </div>
      <div>
        <dt>Nguồn</dt>
        <dd>
          {nhanNguon(hang.source)}
          {hang.hand_edited && <> · {NHAN_DA_SUA_TAY}</>}
        </dd>
      </div>
      <div>
        <dt>Liên kết bài gốc</dt>
        <dd>{hang.source_url === "" ? DAU_GACH : hang.source_url}</dd>
      </div>
      {/* The LEGACY image link (`image_url`) has no box any more; a row that still carries one shows
          it as TEXT, never as a `src` — same reason as `source_url` above. */}
      {hang.image_url !== "" && (
        <div>
          <dt>Liên kết ảnh (cách cũ)</dt>
          <dd>{hang.image_url}</dd>
        </div>
      )}
      <div>
        <dt>Người soạn</dt>
        {/* MÃ NGHIỆP VỤ (`CB-2026-7K3M9Q`), không phải họ tên và không phải id nội bộ (luật 6,
            bất biến 8). `service-comms` không sở hữu danh bạ cán bộ nên không có tên để nối. */}
        <dd>{hang.author_code === "" ? DAU_GACH : hang.author_code}</dd>
      </div>
      <div>
        <dt>Cập nhật lúc</dt>
        <dd>{nhanMoc(hang.updated_at)}</dd>
      </div>
    </dl>
  );
}

/**
 * Biểu mẫu `⊞ Danh mục tin` §6 — THÊM một danh mục. Sửa, ẩn/hiện và xoá nằm ở `CategoryAdmin`
 * (`category-admin.tsx`, ADR 0067 §3), drawn right above this form.
 */
export function FormDanhMuc({
  danhMuc,
  dangGui,
  loi,
  huy,
  luu,
}: {
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (ten: string, slug: string, chaID: string, thuTu: number, khoa: string) => void;
}) {
  const [ten, datTen] = useState("");
  const [slug, datSlug] = useState("");
  const [chaID, datChaID] = useState("");
  const [thuTu, datThuTu] = useState("0");
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tenGon = ten.trim();
  const slugGon = slug.trim();
  const duDieuKien = tenGon !== "" && slugGon !== "";

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    // Số không đọc được thành 0: ô là `type=number`, nhưng một ô số rỗng cho ra chuỗi rỗng, và
    // `Number("")` là 0 — viết ra để không ai phải đoán.
    const n = Number.parseInt(thuTu, 10);
    luu(tenGon, slugGon, chaID, Number.isNaN(n) ? 0 : n, khoaChongTrung);
  }

  const cay = dungCayDanhMuc(danhMuc);

  return (
    <form className="form-danh-muc" onSubmit={guiNgay} aria-labelledby="tieu-de-form-danh-muc">
      <h3 id="tieu-de-form-danh-muc">Thêm danh mục tin</h3>

      {cay.length === 0 && <p className="ghi-chu">{DANH_MUC_RONG}</p>}

      <div className="o-nhap">
        <label htmlFor="ten-danh-muc">Tên danh mục *</label>
        <input
          id="ten-danh-muc"
          name="ten-danh-muc"
          value={ten}
          maxLength={TEN_DANH_MUC_TOI_DA}
          autoComplete="off"
          onChange={(e) => datTen(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="slug-danh-muc">Slug *</label>
        <input
          id="slug-danh-muc"
          name="slug-danh-muc"
          value={slug}
          maxLength={SLUG_DANH_MUC_TOI_DA}
          autoComplete="off"
          onChange={(e) => datSlug(e.target.value)}
        />
        {/* MÁY CHỦ TỪ CHỐI CHỨ KHÔNG TỰ HẠ CHỮ HOA: `Chuyen-Doi-So` bị trả 400 thay vì lặng lẽ
            thành `chuyen-doi-so`, vì mã lưu xuống phải đúng mã người ta thấy lúc gõ. Nói trước
            điều đó thay vì để họ gõ xong mới biết. */}
        <p className="ghi-chu">
          Chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ <code>chuyen-doi-so</code>. Slug đã
          cấp thì KHÔNG cấp lại, kể cả khi danh mục mang slug đó đã bị xoá.
        </p>
      </div>

      <div className="o-chon">
        <label htmlFor="cha-danh-muc">Danh mục cha</label>
        <select id="cha-danh-muc" value={chaID} onChange={(e) => datChaID(e.target.value)}>
          <option value="">— Không có danh mục cha —</option>
          {cay.map((m) => (
            <option key={m.dm.id} value={m.dm.id}>
              {nhanMucDanhMuc(m)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-nhap">
        <label htmlFor="thu-tu-danh-muc">Thứ tự hiển thị</label>
        <input
          id="thu-tu-danh-muc"
          name="thu-tu-danh-muc"
          type="number"
          min={0}
          max={THU_TU_DANH_MUC_TOI_DA}
          value={thuTu}
          onChange={(e) => datThuTu(e.target.value)}
        />
      </div>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || !duDieuKien}>
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}
