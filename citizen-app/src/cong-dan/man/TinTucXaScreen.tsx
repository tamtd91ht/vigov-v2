/**
 * MÀN "TIN TỨC CỦA XÃ" — tin xã đã đăng cho người dân, mới nhất trước; chạm một tin để đọc toàn văn.
 *
 * ⚠ CÔNG KHAI, KHÔNG CẦN PHIÊN (`api/goi-vigov.ts` `tinCuaXa` · `baiTinCuaXa`). Cổng là TÊN MIỀN xã
 *   của lần mở này — màn chỉ được mở khi đã có nó (`KenhCongDan.tsx`).
 *
 * ⚠ KHÔNG BAO GIỜ HTML. Thân tin được vẽ bằng phần tử React (tự thoát ký tự) — từ `body_blocks` có cấu
 *   trúc khi máy chủ gửi (ADR 0067 §1), hoặc từ `body` văn bản thuần chia đoạn theo dòng trống. Không
 *   một lối chèn HTML thô nào ở bất kỳ đâu (luật 13 cấm #3): một thẻ lọt vào thân tin mà được vẽ ra là mã
 *   lạ chạy trong một app mang tên cơ quan nhà nước.
 *
 * ⚠ NÚT "XEM THÊM", KHÔNG CUỘN VÔ HẠN (`skills/accessibility-elderly`), cùng lối "Phản ánh của tôi".
 */
import { useEffect, useRef, useState } from "react";

import { baiTinCuaXa, type NewsReadResult, tinCuaXa } from "../api/goi-vigov";
import { type BaiTinXa, type TinXaTomTat, type TrangTinXa } from "../api/hop-dong-cong-khai";
import { layPhienViGov } from "../api/phien-vigov";

import { ArticleBody, NewsSapo } from "./article-body";
import { BieuTuong } from "./BieuTuong";
import { BangXa, EmptyNotice, LoadingNotice } from "./khung";
import { QUAY_LAI, TIN_XA } from "./noi-dung";
import { NGAY_KHONG_DOC_DUOC, ngayVN } from "../../lib/thoi-diem";

/**
 * Bốn câu lỗi. `khong-hop-le` không mời thử lại: bấm lại không đổi được gì. `rate-limited` (429, 02/10/2026)
 * keeps the "Thử lại" button but never retries by itself: a screen that re-asks on its own is the loop the limit
 * exists to stop.
 */
export type LoiTin = "loi-mang" | "loi-may-chu" | "khong-hop-le" | "rate-limited";

const CAU_LOI: Readonly<Record<Exclude<LoiTin, "rate-limited">, string>> = {
  "loi-mang": TIN_XA.loi_mang,
  "loi-may-chu": TIN_XA.loi_may_chu,
  "khong-hop-le": TIN_XA.khong_hop_le,
};

/** The wait in words: seconds up to 90, whole minutes (rounded up) beyond. PURE. */
export function waitWords(seconds: number): string {
  return seconds <= 90 ? `${seconds} giây` : `${Math.ceil(seconds / 60)} phút`;
}

/** The sentence for an error; `retryAfterSeconds` matters only to `rate-limited`. PURE. */
export function errorSentence(loi: LoiTin, retryAfterSeconds: number | null = null): string {
  if (loi !== "rate-limited") return CAU_LOI[loi];
  return retryAfterSeconds === null || retryAfterSeconds <= 0
    ? TIN_XA.rate_limited
    : TIN_XA.rate_limited_wait(waitWords(retryAfterSeconds));
}

function loiCua(kieu: Exclude<NewsReadResult<unknown>["kieu"], "xong">): LoiTin {
  if (kieu === "loi-mang") return "loi-mang";
  if (kieu === "khong-hop-le") return "khong-hop-le";
  if (kieu === "rate-limited") return "rate-limited";
  return "loi-may-chu";
}

/** The server's wait, when the failure was a 429 that named one. */
function waitOf(kq: NewsReadResult<unknown>): number | null {
  return kq.kieu === "rate-limited" ? kq.retryAfterSeconds : null;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DANH SÁCH — trạng thái thuần
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type DanhSachTin = {
  readonly muc: readonly TinXaTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
  readonly da_co_trang_dau: boolean;
  readonly dang_tai: boolean;
  readonly loi: LoiTin | null;
  /** The server's wait with a `rate-limited` error (seconds); absent or `null` otherwise. */
  readonly retryAfterSeconds?: number | null;
};

export const TIN_DAU: DanhSachTin = {
  muc: [],
  con_tro: "",
  con_nua: false,
  da_co_trang_dau: false,
  dang_tai: true,
  loi: null,
};

export function batDauTaiTin(ds: DanhSachTin): DanhSachTin {
  return { ...ds, dang_tai: true, loi: null };
}

/** Trang mới NỐI VÀO SAU; tin trùng `id` (hai lần tải chồng nhau) bị bỏ. */
export function sauKhiTaiTin(ds: DanhSachTin, kq: NewsReadResult<TrangTinXa>): DanhSachTin {
  if (kq.kieu !== "xong") return { ...ds, dang_tai: false, loi: loiCua(kq.kieu), retryAfterSeconds: waitOf(kq) };
  const da_co = new Set(ds.muc.map((t) => t.id));
  return {
    muc: [...ds.muc, ...kq.gia_tri.muc.filter((t) => !da_co.has(t.id))],
    con_tro: kq.gia_tri.con_tro,
    con_nua: kq.gia_tri.con_nua,
    da_co_trang_dau: true,
    dang_tai: false,
    loi: null,
  };
}

const ngay = (s: string) => ngayVN(s) ?? NGAY_KHONG_DOC_DUOC;

/**
 * An eye and "1.234 lượt xem" at the end of a meta line, both apps (owner, 02/10/2026; prototype `NewsCard.tsx:31-34`). The
 * eye is decoration (`aria-hidden` in `BieuTuong`): the words carry the meaning, so a screen reader hears
 * "1.234 lượt xem" and nobody has to read a picture. Rendered only for a count the server sent (`viewCount`).
 */
export function ViewCount({ count }: { count: number }) {
  return (
    <span className="view-count">
      <BieuTuong ten="eye" co={16} />
      {TIN_XA.view_count(count)}
    </span>
  );
}

/**
 * Dòng phụ chung cho thẻ và bài: chuyên mục · ngày đăng — bằng chữ, có nhãn — then the view count when the server
 * sent one. Without a count the markup is exactly what it was.
 */
function DongPhu({ tin }: { tin: TinXaTomTat }) {
  return (
    <span className={tin.viewCount === undefined ? "cd-the-cua-toi__dong" : "cd-the-cua-toi__dong news-meta"}>
      {tin.chuyen_muc !== "" && `${TIN_XA.chuyen_muc}: ${tin.chuyen_muc} · `}
      {TIN_XA.ngay_dang}: {ngay(tin.ngay_dang)}
      {tin.viewCount !== undefined && <ViewCount count={tin.viewCount} />}
    </span>
  );
}

/** MỘT THẺ = MỘT NÚT to bằng cả thẻ. Tiêu đề to nhất, tóm tắt ngay dưới. */
export function TheTin({ tin, onMo }: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  return (
    <li className="cd-cua-toi__muc">
      <button type="button" className="cd-the-cua-toi" onClick={() => onMo(tin.id)}>
        <strong className="cd-tin__tieu-de">{tin.tieu_de}</strong>
        <DongPhu tin={tin} />
        {tin.tom_tat !== "" && <span className="cd-the-cua-toi__trich">{tin.tom_tat}</span>}
        <span className="cd-the-cua-toi__xem">{TIN_XA.nut_doc}</span>
      </button>
    </li>
  );
}

export function ThanTinXa(props: { ds: DanhSachTin; onMo: (id: string) => void; onTai: () => void }) {
  const { ds } = props;
  return (
    <>
      {!ds.da_co_trang_dau && ds.dang_tai && <LoadingNotice cau={TIN_XA.dang_tai} />}
      {ds.da_co_trang_dau && ds.muc.length === 0 && <EmptyNotice kind="news" cau={TIN_XA.trong} hint={TIN_XA.empty_hint} />}
      {ds.muc.length > 0 && (
        <ul className="cd-cua-toi">
          {ds.muc.map((t) => (
            <TheTin key={t.id} tin={t} onMo={props.onMo} />
          ))}
        </ul>
      )}
      {ds.da_co_trang_dau && ds.dang_tai && (
        <p className="cd-cau" role="status">
          {TIN_XA.dang_tai_them}
        </p>
      )}
      {ds.loi !== null && (
        <div className="cd-buoc">
          <p className="cd-loi" role="alert">
            {errorSentence(ds.loi, ds.retryAfterSeconds ?? null)}
          </p>
          {ds.loi !== "khong-hop-le" && (
            <button type="button" className="cd-nut" disabled={ds.dang_tai} onClick={props.onTai}>
              {TIN_XA.nut_thu_lai}
            </button>
          )}
        </div>
      )}
      {ds.loi === null && ds.con_nua && (
        <button type="button" className="cd-nut-phu" disabled={ds.dang_tai} onClick={props.onTai}>
          {TIN_XA.nut_xem_them}
        </button>
      )}
      {ds.da_co_trang_dau && !ds.con_nua && ds.muc.length > 0 && (
        <p className="cd-ghi-chu">{TIN_XA.het_danh_sach}</p>
      )}
    </>
  );
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * MỘT BÀI — toàn văn, văn bản thuần
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type TrangBai =
  | { readonly kieu: "dang-tai" }
  | { readonly kieu: "xong"; readonly bai: BaiTinXa }
  | { readonly kieu: "khong-thay" }
  | { readonly kieu: "loi"; readonly loi: LoiTin; readonly retryAfterSeconds?: number | null };

export function sauKhiTaiBai(kq: NewsReadResult<BaiTinXa>): TrangBai {
  if (kq.kieu === "xong") return { kieu: "xong", bai: kq.gia_tri };
  if (kq.kieu === "khong-thay") return { kieu: "khong-thay" };
  return { kieu: "loi", loi: loiCua(kq.kieu), retryAfterSeconds: waitOf(kq) };
}

/**
 * Toàn văn. Mỗi đoạn là MỘT `<p>` chứa CHỮ — React thoát mọi ký tự, kể cả `<script>`. Có `body_blocks` thì vẽ
 * định dạng từ cấu trúc (`article-body.tsx`); KHÔNG có trình mở liên kết ở app chung (chủ dự án 01/10/2026: mở
 * ra ngoài chỉ ở app của xã), nên chữ của một liên kết hiện như chữ thường.
 */
export function BaiTin({ bai }: { bai: BaiTinXa }) {
  return (
    <article className="cd-tin">
      <h2 className="cd-tieu-de-phu">{bai.tieu_de}</h2>
      <NewsSapo summary={bai.tom_tat} />
      <p className="cd-ghi-chu">
        <DongPhu tin={bai} />
      </p>
      <ArticleBody blocks={bai.bodyBlocks} text={bai.noi_dung} paragraphClass="cd-tin__doan" />
    </article>
  );
}

export function ThanBaiTin(props: { trang: TrangBai; onTai: () => void }) {
  const { trang } = props;
  if (trang.kieu === "dang-tai") return <LoadingNotice cau={TIN_XA.dang_tai_bai} shape="article" />;
  if (trang.kieu === "xong") return <BaiTin bai={trang.bai} />;
  if (trang.kieu === "khong-thay") {
    return (
      <p className="cd-loi" role="alert">
        {TIN_XA.khong_thay}
      </p>
    );
  }
  return (
    <div className="cd-buoc">
      <p className="cd-loi" role="alert">
        {errorSentence(trang.loi, trang.retryAfterSeconds ?? null)}
      </p>
      {trang.loi !== "khong-hop-le" && (
        <button type="button" className="cd-nut" onClick={props.onTai}>
          {TIN_XA.nut_thu_lai}
        </button>
      )}
    </div>
  );
}

/**
 * One open of an article: ONE counted read on mount (the ref survives StrictMode's second effect run), and one more
 * only when the citizen taps "Thử lại" — the failed read was not counted, so the retry is the open.
 */
export function ManBaiTin(props: { ten_mien: string; id: string; onQuayLai: () => void; ten_xa: string | null }) {
  const [trang, datTrang] = useState<TrangBai>({ kieu: "dang-tai" });
  const da_tai = useRef(false);

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiBai(await baiTinCuaXa(props.ten_mien, props.id)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  return (
    <section className="cd-man" aria-label={TIN_XA.tieu_de}>
      <button type="button" className="quay-lai" onClick={props.onQuayLai}>
        {QUAY_LAI}
      </button>
      {props.ten_xa !== null && <BangXa ten_xa={props.ten_xa} />}
      <ThanBaiTin trang={trang} onTai={() => void tai()} />
    </section>
  );
}

export function TinTucXaScreen(props: { ten_mien: string; onQuayLai: () => void }) {
  const [ten_xa] = useState(() => layPhienViGov()?.ten_xa ?? null);
  const [ds, datDs] = useState<DanhSachTin>(TIN_DAU);
  const [dang_doc, datDangDoc] = useState<string | null>(null);
  const da_tai_dau = useRef(false);

  async function tai(con_tro: string) {
    datDs(batDauTaiTin);
    const kq = await tinCuaXa(props.ten_mien, con_tro);
    datDs((truoc) => sauKhiTaiTin(truoc, kq));
  }

  useEffect(() => {
    if (da_tai_dau.current) return;
    da_tai_dau.current = true;
    void tai("");
  }, []);

  return (
    <>
      {/* Danh sách chỉ bị ẩn khi đọc một tin — "Quay lại" về đúng chỗ đang đọc, không tải lại. */}
      <section className="cd-man" aria-label={TIN_XA.tieu_de} hidden={dang_doc !== null}>
        <button type="button" className="quay-lai" onClick={props.onQuayLai}>
          {QUAY_LAI}
        </button>
        {ten_xa !== null && <BangXa ten_xa={ten_xa} />}
        <h1 className="cd-tieu-de">{TIN_XA.tieu_de}</h1>
        <ThanTinXa
          ds={ds}
          onMo={datDangDoc}
          onTai={() => {
            if (!ds.dang_tai) void tai(ds.con_tro);
          }}
        />
      </section>
      {dang_doc !== null && (
        <ManBaiTin
          key={dang_doc}
          ten_mien={props.ten_mien}
          id={dang_doc}
          ten_xa={ten_xa}
          onQuayLai={() => datDangDoc(null)}
        />
      )}
    </>
  );
}
