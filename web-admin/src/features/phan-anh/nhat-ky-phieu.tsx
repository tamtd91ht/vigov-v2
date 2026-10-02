"use client";

import { ChevronDown, History, LockKeyhole, NotebookPen } from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { cn } from "@/lib/cn";
import { khoaSauLanGhi } from "@/features/thu-chi/nhan-thu-chi";
import type { KetQua } from "@/lib/api/goi";
import { ghiNhatKyPhieu, layNhatKyPhieu } from "@/lib/api/phieu-phan-anh";
import type {
  page_Result_petitions_nhatKyPhieuRa,
  petitions_nhatKyPhieuRa,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_NHAT_KY,
  demKyTu,
  GHI_CHU_TOI_DA,
  GOI_Y_GHI_NHAT_KY,
  laDongPhanCong,
  logActorLabel,
  loiGhiChuNhatKy,
  NHAC_DU_LIEU_CA_NHAN,
  NHAN_BO_PHAN_PHU_TRACH,
  NHAN_NGUOI_THUC_HIEN,
  NHAN_NUT_GHI_NHAT_KY,
  NHAN_O_GHI_NHAT_KY,
  NHAN_XEM_THEM_NHAT_KY,
  NHAT_KY_RONG,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanThaoTacNhatKy,
  nhanThoiDiem,
  nhanTrangThai,
  TIEU_DE_NHAT_KY,
  type DanhBaTheoMa,
} from "./nhan-phieu";
import {
  BusyLabel,
  buttonClass,
  Glyph,
  HINT_CLASS,
  LABEL_CLASS,
  PetitionStatusBadge,
  SectionTitle,
  TEXTAREA_CLASS,
} from "./petition-ui";

/**
 * Nhật ký xử lý của MỘT phiếu (§8.7) — dòng thời gian mới nhất trước, "Xem thêm" theo con trỏ, và
 * nút `Ghi nhật ký`.
 *
 * NHẬT KÝ LÀ BẢN GHI NGHIỆP VỤ, KHÔNG PHẢI `audit_log`: cán bộ trong xã đọc nó để biết ai đã làm gì.
 * Máy chủ tự ghi một dòng ở mỗi thao tác xử lý; dòng `ghi-chu` là dòng cán bộ tự viết.
 *
 * NỘI DUNG GHI CHÚ HIỆN BẰNG TEXT NODE CỦA REACT, KHÔNG BAO GIỜ `dangerouslySetInnerHTML`: đó là chữ
 * tự do do người gõ, và một thẻ `<img onerror>` trong đó sẽ chạy trên máy của mọi cán bộ mở phiếu.
 * Xuống dòng giữ bằng CSS (`.ghi-chu-nhat-ky`), không bằng cách chèn `<br>`.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100, mặc định 20. */
const SO_DONG_MOI_TRANG = 20;

type TrangDoc =
  | { khoa: string; ok: true; dong: readonly petitions_nhatKyPhieuRa[]; conTro: string; conNua: boolean }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_nhatKyPhieuRa>): TrangDoc {
  return kq.ok
    ? {
        khoa,
        ok: true,
        dong: kq.duLieu.items,
        conTro: kq.duLieu.next_cursor,
        conNua: kq.duLieu.has_more,
      }
    : { khoa, ok: false, thongBao: kq.thongBao };
}

export function NhatKyPhieu({
  maTraCuu,
  tenBoPhan,
  danhBa,
  coNutGhi,
  lanLamMoi = 0,
}: {
  maTraCuu: string;
  tenBoPhan: ReadonlyMap<string, string>;
  /** Danh bạ tra theo mã cán bộ, cho ô `Phụ trách` của dòng `phan-cong`. `null` = chưa có. */
  danhBa: DanhBaTheoMa | null;
  /**
   * Có vẽ nút `Ghi nhật ký` không. CHỈ LÀ TIỆN DỤNG: người gọi truyền `feedback.read` của phiên.
   * Máy chủ mới quyết ai ghi được (người được phân công, hoặc có `feedback.resolve` / `.assign` /
   * `.classify`) và câu 403 của nó ra nguyên văn.
   */
  coNutGhi: boolean;
  /** Tăng lên sau mỗi thao tác xử lý thành công — máy chủ vừa ghi thêm một dòng. */
  lanLamMoi?: number;
}) {
  const [lanTaiLai, datLanTaiLai] = useState(0);
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<string | null>(null);

  const [moGhi, datMoGhi] = useState(false);
  const [noiDung, datNoiDung] = useState("");
  /**
   * Khoá chống trùng của BẢN NHÁP đang gõ. Giữ nguyên qua mọi lần gửi lại sau lỗi (kể cả lỗi mạng mà
   * lần đầu thật ra đã tới máy chủ); thay mới sau một lần 201 — `khoaSauLanGhi`.
   */
  const [khoa, datKhoa] = useState(khoaChongTrungMoi);
  const [dangGui, datDangGui] = useState(false);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const nutGhi = useRef<HTMLButtonElement>(null);

  const khoaDoc = `${maTraCuu}|${lanLamMoi}|${lanTaiLai}`;

  useEffect(() => {
    let bo = false;
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG }).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [maTraCuu, khoaDoc]);

  // Kết quả của một lần đọc CŨ (phiếu khác, trước lần làm mới) không được vẽ như của lần này.
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG, cursor: truoc.conTro }).then((kq) => {
      datDangTaiThem(false);
      if (!kq.ok) {
        datLoiThem(kq.thongBao);
        return;
      }
      datLoiThem(null);
      datDoc((d) =>
        d === null || d.khoa !== truoc.khoa || !d.ok
          ? d
          : {
              ...d,
              dong: [...d.dong, ...kq.duLieu.items],
              conTro: kq.duLieu.next_cursor,
              conNua: kq.duLieu.has_more,
            },
      );
    });
  }

  function dongBieuMau(): void {
    datMoGhi(false);
    // Bản nháp và khoá của nó ĐƯỢC GIỮ: mở lại là gõ tiếp, và gửi lại vẫn là cùng một lần ghi.
    nutGhi.current?.focus();
  }

  function gui(): void {
    datDangGui(true);
    ghiNhatKyPhieu(maTraCuu, noiDung, khoa).then((kq) => {
      datDangGui(false);
      datKhoa((k) => khoaSauLanGhi(k, kq.ok, khoaChongTrungMoi));
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ — 403 nói đúng vì sao tài khoản này không ghi được vào phiếu này.
        datLoiGhi(kq.thongBao);
        return;
      }
      datLoiGhi(null);
      datNoiDung("");
      datMoGhi(false);
      datLanTaiLai((n) => n + 1);
      nutGhi.current?.focus();
    });
  }

  return (
    <Card as="section" aria-labelledby={`tieu-de-nhat-ky-${maTraCuu}`}>
      <CardHeader className="justify-between">
        <SectionTitle icon={History} id={`tieu-de-nhat-ky-${maTraCuu}`}>
          {TIEU_DE_NHAT_KY}
        </SectionTitle>
        {coNutGhi && (
          <button
            ref={nutGhi}
            type="button"
            className={buttonClass("secondary", "sm")}
            aria-expanded={moGhi}
            aria-controls={`bieu-mau-nhat-ky-${maTraCuu}`}
            onClick={() => (moGhi ? dongBieuMau() : datMoGhi(true))}
          >
            <Glyph icon={NotebookPen} />
            {NHAN_NUT_GHI_NHAT_KY}
          </button>
        )}
      </CardHeader>

      <CardContent className="flex flex-col gap-3 [&>p]:m-0">
      {coNutGhi && moGhi && (
        <BieuMauGhiNhatKy
          id={`bieu-mau-nhat-ky-${maTraCuu}`}
          noiDung={noiDung}
          datNoiDung={datNoiDung}
          dangGui={dangGui}
          loi={loiGhi}
          gui={gui}
          huy={dongBieuMau}
        />
      )}

      {hienTai === null && (
        <p role="status" className="text-sm text-ink-500">
          {DANG_TAI_NHAT_KY}
        </p>
      )}
      {hienTai !== null && !hienTai.ok && (
        <p className="thong-bao-loi" role="alert">
          {hienTai.thongBao}
        </p>
      )}
      {hienTai !== null && hienTai.ok && (
        <>
          <DanhSachNhatKy dong={hienTai.dong} tenBoPhan={tenBoPhan} danhBa={danhBa} />
          {loiThem !== null && (
            <p className="thong-bao-loi" role="alert">
              {loiThem}
            </p>
          )}
          {/* Hai điều kiện, không một: `has_more` nói còn dòng, `next_cursor` là đường đi tới đó. */}
          {hienTai.conNua && hienTai.conTro !== "" && (
            <div>
              <button
                type="button"
                className={buttonClass("secondary", "sm")}
                disabled={dangTaiThem}
                onClick={xemThem}
              >
                <Glyph icon={ChevronDown} />
                {NHAN_XEM_THEM_NHAT_KY}
              </button>
            </div>
          )}
        </>
      )}
      </CardContent>
    </Card>
  );
}

/**
 * Các dòng nhật ký, đúng thứ tự máy chủ trả (mới nhất trước). Tách ra để kiểm bằng HTML tĩnh.
 *
 * NGƯỜI THỰC HIỆN LÀ MÃ CÁN BỘ, không họ tên: máy chủ không trả tên, và mã là thứ còn chỉ ra được
 * đúng một người nhiều năm sau (luật 6, bất biến 8). Rỗng thì hiện gạch, không để ô trống.
 * Rows the citizen caused carry the marker `cong-dan` and read "Người dân" (`logActorLabel`).
 */
export function DanhSachNhatKy({
  dong,
  tenBoPhan,
  danhBa,
}: {
  dong: readonly petitions_nhatKyPhieuRa[];
  tenBoPhan: ReadonlyMap<string, string>;
  danhBa: DanhBaTheoMa | null;
}) {
  if (dong.length === 0) {
    return (
      <p className="m-0 inline-flex items-center gap-2 text-sm text-ink-500">
        <Glyph icon={History} className="size-4 shrink-0" />
        {NHAT_KY_RONG}
      </p>
    );
  }

  // A clean vertical timeline (spec v2 §8b "Minh bạch"): a dot and a hairline on the left, then the
  // time, the act, the status pill, who did it and the note — the same fields, in the same order.
  return (
    <ol aria-label="Nhật ký xử lý, mới nhất trước" className="m-0 list-none p-0">
      {dong.map((d) => (
        <li
          key={d.id}
          className="relative pb-4 pl-6 last:pb-0 before:absolute before:top-1.5 before:left-[5px] before:h-full before:w-px before:bg-line last:before:hidden"
        >
          <span
            aria-hidden="true"
            className="absolute top-1.5 left-0 size-[11px] rounded-full border-2 border-brand-500 bg-surface"
          />
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
            <time dateTime={d.at} className="text-xs text-ink-500 tabular-nums">
              {nhanThoiDiem(d.at)}
            </time>
            <strong>{nhanThaoTacNhatKy(d.action)}</strong>
            {d.status !== "" && (
              <PetitionStatusBadge status={d.status}>{nhanTrangThai(d.status)}</PetitionStatusBadge>
            )}
          </div>
          <dl className="mt-1 mb-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-0.5 text-[13px] [&>dd]:m-0 [&>dt]:text-ink-500">
            {laDongPhanCong(d.action) && (
              <>
                <dt>{NHAN_BO_PHAN_PHU_TRACH}</dt>
                <dd>
                  {nhanBoPhan(d.unit, tenBoPhan)} · {nhanCanBoXuLy(d.assignee, danhBa)}
                </dd>
              </>
            )}
            <dt>{NHAN_NGUOI_THUC_HIEN}</dt>
            {/* `cong-dan` reads "Người dân" and is never looked up in the staff directory. */}
            <dd>{logActorLabel(d.actor_code)}</dd>
          </dl>
          {/* The class stays exactly `ghi-chu-nhat-ky` (it keeps the line breaks; a test reads it);
              the frame comes from the wrapper. */}
          {d.note !== "" && (
            <div className="mt-1.5 rounded-lg bg-surface-muted px-3 py-2 text-sm [&>p]:m-0">
              <p className="ghi-chu-nhat-ky">{d.note}</p>
            </div>
          )}
        </li>
      ))}
    </ol>
  );
}

/**
 * Biểu mẫu `Ghi nhật ký`. Điều khiển từ ngoài (bản nháp sống ở `NhatKyPhieu`) để đóng rồi mở lại
 * không mất chữ và không đổi khoá chống trùng. `Esc` trong ô nhập là đóng.
 *
 * Nút gửi khoá khi trống hoặc quá 2000 ký tự — nhưng giới hạn thật ở máy chủ, và câu 400 của nó vẫn
 * ra nguyên văn nếu hai bên lệch nhau.
 */
export function BieuMauGhiNhatKy({
  id,
  noiDung,
  datNoiDung,
  dangGui,
  loi,
  gui,
  huy,
}: {
  id: string;
  noiDung: string;
  datNoiDung: (s: string) => void;
  dangGui: boolean;
  loi: string | null;
  gui: () => void;
  huy: () => void;
}) {
  const oNhap = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    oNhap.current?.focus();
  }, []);

  const loiO = loiGhiChuNhatKy(noiDung);
  const idO = `${id}-noi-dung`;

  function phim(e: KeyboardEvent<HTMLTextAreaElement>): void {
    if (e.key === "Escape") {
      e.preventDefault();
      huy();
    }
  }

  return (
    <form
      id={id}
      className="form-danh-muc m-0 flex flex-col gap-3 rounded-xl border border-line bg-surface-muted p-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (loiO === null && !dangGui) gui();
      }}
    >
      <div>
        <label htmlFor={idO} className={LABEL_CLASS}>
          {NHAN_O_GHI_NHAT_KY}
        </label>
        <textarea
          ref={oNhap}
          id={idO}
          name={idO}
          rows={4}
          className={cn(TEXTAREA_CLASS, loiO !== null && noiDung !== "" && "border-danger-600")}
          value={noiDung}
          placeholder={GOI_Y_GHI_NHAT_KY}
          onChange={(e) => datNoiDung(e.target.value)}
          onKeyDown={phim}
          aria-describedby={`${idO}-dem ${idO}-nhac`}
        />
        <p
          className={cn(HINT_CLASS, loiO !== null && noiDung !== "" && "text-danger-600")}
          id={`${idO}-dem`}
          aria-live="polite"
        >
          {demKyTu(noiDung)}/{GHI_CHU_TOI_DA} ký tự
          {loiO !== null && noiDung !== "" ? ` · ${loiO}` : ""}
        </p>
        <p className={cn(HINT_CLASS, "inline-flex items-start gap-1")} id={`${idO}-nhac`}>
          <Glyph icon={LockKeyhole} className="mt-0.5 size-3.5 shrink-0" />
          {NHAC_DU_LIEU_CA_NHAN}
        </p>
      </div>
      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
      <div className="cum-nut justify-end">
        {/* Busy = THIS form's own send (`dangGui` is the log block's state, not the drawer's). */}
        <button type="submit" className={buttonClass("primary")} disabled={dangGui || loiO !== null}>
          <BusyLabel busy={dangGui}>Lưu vào nhật ký</BusyLabel>
        </button>
        <button type="button" className={buttonClass("secondary")} onClick={huy} disabled={dangGui}>
          Huỷ
        </button>
      </div>
    </form>
  );
}
