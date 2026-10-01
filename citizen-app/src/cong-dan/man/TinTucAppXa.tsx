/**
 * TIN TỨC CỦA XÃ — giao diện theo prototype khách (`PROTOTYPE.md` §6.5, đợt 1 30/09/2026): mọi bài là cùng
 * một thẻ (ô hình · tiêu đề hai dòng · "x ngày trước"), không còn thẻ nổi bật; chạm để đọc toàn văn.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `TinTucXaScreen.tsx` — cùng trạng thái thuần (`TIN_DAU`,
 * `sauKhiTaiTin`, `sauKhiTaiBai`), cùng tuyến công khai theo tên miền, cùng "Xem thêm" thay cho cuộn
 * vô hạn. VĂN BẢN THUẦN: thân tin vẽ bằng nút chữ của React, chia đoạn theo dòng trống; không HTML.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { baiTinCuaXa, newsCategories, tinCuaXa } from "../api/goi-vigov";
import {
  type BaiTinXa as BaiTinXaData,
  chiaDoan,
  type NewsCategory,
  type NewsType,
  type TinXaTomTat,
} from "../api/hop-dong-cong-khai";
import { NGAY_KHONG_DOC_DUOC, ngayVN, thoiDiemVN } from "../../lib/thoi-diem";
import { NEWS_TYPE_LABEL, TIN_XA, XA_TN } from "./noi-dung";
import {
  batDauTaiTin,
  type DanhSachTin,
  sauKhiTaiBai,
  sauKhiTaiTin,
  TIN_DAU,
  type TrangBai,
} from "./TinTucXaScreen";

import { BieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";

const ngay = (s: string) => ngayVN(s) ?? NGAY_KHONG_DOC_DUOC;

/** An instant in Vietnam time (+07, pinned — `lib/thoi-diem.ts`), split into its day and its time; `null` if unreadable. */
function vnDayTime(iso: string): { day: string; time: string } | null {
  const s = thoiDiemVN(iso); // "dd/MM/yyyy HH:mm"
  if (s === null) return null;
  const [day, time] = s.split(" ") as [string, string];
  return { day, time };
}

/**
 * When the article was published, for the detail's meta line: "09:07 ngày 25/09/2026" (§6.5) when the first
 * publish instant is known, else the publication day alone. PURE.
 *
 * THE TIME IS SHOWN ONLY WHEN IT FALLS ON `ngay_dang`. `ngay_dang` is the day the commune chose to display (the
 * cards count "x ngày trước" from it); `publishedAt` is when it was first published. When the two disagree —
 * a backdated item — printing the time beside `ngay_dang` would state an instant that never happened, and
 * printing `publishedAt`'s own day would contradict the card the citizen just tapped. So the day alone.
 */
export function publishedLabel(tin: TinXaTomTat): string {
  const at = tin.publishedAt === undefined ? null : vnDayTime(tin.publishedAt);
  const day = ngayVN(tin.ngay_dang);
  if (at === null || day === null || at.day !== day) return ngay(tin.ngay_dang);
  return XA_TN.time_on_day(at.time, at.day);
}

function dongPhu(tin: TinXaTomTat): string {
  return tin.chuyen_muc !== "" ? `${tin.chuyen_muc} · ${publishedLabel(tin)}` : publishedLabel(tin);
}

/**
 * An event's window in words. PURE. Start only → "08:00 ngày 05/10/2026"; same day → "08:00 – 11:00 ngày
 * 05/10/2026"; two days → "08:00 ngày 05/10/2026 – 17:00 ngày 06/10/2026". No readable start → `null` (the
 * block then shows no time row). An unreadable end → the start alone, never a guessed end.
 */
export function eventTimeLabel(startsAt: string | undefined, endsAt: string | undefined): string | null {
  const start = startsAt === undefined ? null : vnDayTime(startsAt);
  if (start === null) return null;
  const end = endsAt === undefined ? null : vnDayTime(endsAt);
  if (end === null) return XA_TN.time_on_day(start.time, start.day);
  if (end.day === start.day) return XA_TN.time_on_day(`${start.time} – ${end.time}`, start.day);
  return `${XA_TN.time_on_day(start.time, start.day)} – ${XA_TN.time_on_day(end.time, end.day)}`;
}

/**
 * The event block of a `su-kien` article (§6.5 detail: "Thời gian / Địa điểm" on the purple tone the Sự kiện
 * tile already uses). Each row only when the commune set it; neither → nothing at all. PLAIN TEXT. PURE.
 */
export function EventDetails({ tin }: { tin: TinXaTomTat }) {
  const time = eventTimeLabel(tin.eventStartsAt, tin.eventEndsAt);
  const place = tin.eventPlace;
  if (time === null && place === undefined) return null;
  return (
    <section className="xa-bai__event" aria-label={XA_TN.event_details}>
      <dl className="xa-bai__event-list">
        {time !== null && (
          <div>
            <dt>{XA_TN.event_time}</dt>
            <dd>{time}</dd>
          </div>
        )}
        {place !== undefined && (
          <div>
            <dt>{XA_TN.event_place}</dt>
            <dd>{place}</dd>
          </div>
        )}
      </dl>
    </section>
  );
}

/**
 * OPENS A VIDEO LINK OUTSIDE THE APP — `true` when it opened. Injected by the shell (`App.tsx` `AppRieng`, which
 * opens it through the one declared door under the destination "video"): this half may not import `features/` or
 * `zmp-sdk` (`ranh-gioi-hai-nua.test.ts` §3a), the same way it receives the Zalo name bridge. Not injected (tests,
 * the shared app) → no button.
 */
export type OpenVideo = (url: string) => Promise<boolean>;

/**
 * One tap on "Xem video": hand the commune's link to the opener, unchanged, and answer whether it FAILED. A
 * rejected promise is a failure too — never an unhandled rejection under a button. PURE apart from `open`.
 */
export async function videoOpenFailed(open: OpenVideo, url: string): Promise<boolean> {
  return !(await open(url).catch(() => false));
}

/**
 * The "Xem video" button (red primary `xa-nut`: ≥44px, body-size text) and, after a failed tap, one sentence
 * saying what to do next (`xa-error-box`, `role="alert"`, words — not colour alone). PURE: `failed` is the
 * caller's state.
 */
export function WatchVideo(props: { failed: boolean; onTap: () => void }) {
  return (
    <div className="xa-bai__video">
      <button type="button" className="xa-nut" onClick={props.onTap}>
        {XA_TN.watch_video}
      </button>
      {props.failed && (
        <p className="xa-error-box" role="alert">
          {XA_TN.watch_video_failed}
        </p>
      )}
    </div>
  );
}

/** Today's date in Vietnam, `YYYY-MM-DD` — the one clock read of the news cards. */
export function todayVN(now: number = Date.now()): string {
  return new Date(now + 7 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

/**
 * "Hôm nay" · "Hôm qua" · "N ngày trước" for a publication DAY (`YYYY-MM-DD`, no time of day — so no
 * hours or minutes are ever claimed), against `today` in the same shape. PURE.
 *
 * Past 30 days, or a day after `today` (a clock set wrong on the phone), the plain date instead: "45 ngày
 * trước" makes the reader count, and "-2 ngày trước" is nonsense. An unreadable day → `NGAY_KHONG_DOC_DUOC`.
 */
export function relativeDay(day: string, today: string): string {
  if (ngayVN(day) === null) return NGAY_KHONG_DOC_DUOC;
  const ms = (d: string) => Date.UTC(Number(d.slice(0, 4)), Number(d.slice(5, 7)) - 1, Number(d.slice(8, 10)));
  if (ngayVN(today) === null) return ngay(day);
  const days = Math.round((ms(today) - ms(day)) / 86_400_000);
  if (days === 0) return TIN_XA.today;
  if (days === 1) return TIN_XA.yesterday;
  if (days > 1 && days <= 30) return TIN_XA.days_ago(days);
  return ngay(day);
}

/**
 * The card's picture slot. PURE — `failed` is the caller's state, so both outcomes render without a DOM.
 *
 * The cover when the item has one and it has not failed; otherwise the newspaper ICON (owner, 01/10/2026):
 * the slot never disappears — cards of one list keep one shape, and a missing picture must not read as a
 * card that broke. Decoration either way (`alt=""` + `aria-hidden`): the title is the card's words.
 */
export function NewsThumb(props: { imageUrl: string | undefined; failed: boolean; onFail: () => void }) {
  const show = props.imageUrl !== undefined && !props.failed;
  return (
    <span className="xa-hang-tin__o" aria-hidden="true">
      {show ? (
        <img
          className="xa-hang-tin__image"
          src={props.imageUrl}
          alt=""
          loading="lazy"
          decoding="async"
          onError={props.onFail}
        />
      ) : (
        <BieuTuong ten="newspaper" co={30} />
      )}
    </span>
  );
}

/**
 * One news card (`PROTOTYPE.md` §6.5 NewsCard) — on the news tab, the home screen and "Tin liên quan": the
 * picture slot (`NewsThumb`), the title on two lines, how long ago. No view count (owner). `compact`: the
 * home screen's smaller slot (88×72 against 96×80, §6.5).
 */
export function HangTin({
  tin,
  onMo,
  today = todayVN(),
  compact = false,
}: {
  tin: TinXaTomTat;
  onMo: (id: string) => void;
  today?: string;
  compact?: boolean;
}) {
  const [thumbFailed, setThumbFailed] = useState(false);
  return (
    <button type="button" className={`xa-the xa-hang-tin${compact ? " xa-hang-tin--compact" : ""}`} onClick={() => onMo(tin.id)}>
      <NewsThumb imageUrl={tin.imageUrl} failed={thumbFailed} onFail={() => setThumbFailed(true)} />
      <span className="xa-hang-tin__chu">
        <strong className="xa-hang-tin__tieu-de xa-cat-2">{tin.tieu_de}</strong>
        <span className="xa-phu">{relativeDay(tin.ngay_dang, today)}</span>
      </span>
    </button>
  );
}

const CAU_LOI = {
  "loi-mang": TIN_XA.loi_mang,
  "loi-may-chu": TIN_XA.loi_may_chu,
  "khong-hop-le": TIN_XA.khong_hop_le,
} as const;

/**
 * The type tabs of the news tab — exactly the prototype's three (`NewsPage.tsx` TABS: Tin tức · Sự kiện ·
 * Thông báo), each a SERVER filter (`?type=`, comms b22bf76), not a guess from the free-text category.
 * NO "Tất cả" (owner, 30/09/2026): the tab opens on Tin tức. The home screen's "Tin mới" still loads every
 * type (`useTinXa` without a type). Truyền thanh and Video have their own tiles; banners are not articles.
 */
export const NEWS_TABS: readonly NewsType[] = ["tin-tuc", "su-kien", "thong-bao"];

/** Tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3. THUẦN. */
export function tinLienQuan(ds: readonly TinXaTomTat[], dang_doc: TinXaTomTat | null, toi_da = 3): TinXaTomTat[] {
  if (dang_doc === null || dang_doc.chuyen_muc === "") return [];
  return ds.filter((t) => t.id !== dang_doc.id && t.chuyen_muc === dang_doc.chuyen_muc).slice(0, toi_da);
}

/**
 * Danh sách tin — thân của tab "Tin tức – Sự kiện": a tablist of `NEWS_TABS`, and under it the chosen
 * type's own server-filtered list (`NewsOfType`, keyed by type so each tab loads, pages and "Xem thêm"s on
 * its own). The tabs are `role="tab"` + `aria-selected`, and the chosen one carries a bar as well as a
 * colour — never colour alone.
 */
export function DanhSachTinXa(props: { ten_mien: string; onMo: (id: string) => void }) {
  const [type, setType] = useState<NewsType>("tin-tuc");
  return (
    <>
      <div className="xa-tabs-tin" role="tablist" aria-label={XA_TN.loc_loai_tin}>
        {NEWS_TABS.map((t) => (
          <button
            key={t}
            id={`xa-tab-tin-${t}`}
            type="button"
            role="tab"
            aria-selected={type === t}
            aria-controls="xa-tin-theo-loai"
            className={`xa-tabs-tin__muc${type === t ? " xa-tabs-tin__muc--on" : ""}`}
            onClick={() => setType(t)}
          >
            {NEWS_TYPE_LABEL[t]}
          </button>
        ))}
      </div>
      {/* Keyed by type: changing tab remounts the panel, which resets BOTH chip rows and reloads the
          categories with the new tab's `type` (owner, 30/09/2026). */}
      <div id="xa-tin-theo-loai" role="tabpanel" aria-labelledby={`xa-tab-tin-${type}`}>
        <NewsTypePanel key={type} ten_mien={props.ten_mien} type={type} onMo={props.onMo} />
      </div>
    </>
  );
}

/**
 * The chip rows as the screen needs them: roots in order, and each root's DIRECT children in order. PURE.
 *
 * A category whose `parent_id` names nothing in the list (an orphan) is not shown, nor is anything under
 * it: the server always sends ancestors, so an orphan is an inconsistent answer, and a chip with no place
 * in the tree cannot be reached honestly. Deeper levels are not flattened into row 2 — choosing a child
 * filters its whole subtree on the server.
 */
export type CategoryRows = {
  readonly roots: readonly NewsCategory[];
  readonly children: ReadonlyMap<string, readonly NewsCategory[]>;
};

export function groupCategories(list: readonly NewsCategory[]): CategoryRows {
  // Stable sort by `order`: the server already orders the flat list, this only keeps each row ordered
  // if it ever does not.
  const sorted = [...list].sort((a, b) => a.order - b.order);
  const roots = sorted.filter((c) => c.parent_id === null);
  const children = new Map<string, NewsCategory[]>();
  for (const r of roots) children.set(r.id, []);
  for (const c of sorted) {
    if (c.parent_id !== null) children.get(c.parent_id)?.push(c);
  }
  return { roots, children };
}

/**
 * The two chip rows (prototype `NewsPage.tsx:76-131`). Row 1: "Tất cả" + the roots. Row 2, only once a
 * root with children is chosen: "Tất cả mục này" + that root's direct children, indented and lighter so it
 * reads as belonging to row 1. The chosen chip says so with `aria-pressed`, a fill and a heavier weight —
 * never colour alone. `null` or no roots → nothing at all, never a row of empty chips. PURE.
 */
export function CategoryChips(props: {
  rows: CategoryRows | null;
  root: string | null;
  child: string | null;
  onRoot: (id: string | null) => void;
  onChild: (id: string | null) => void;
}) {
  const { rows, root, child } = props;
  if (rows === null || rows.roots.length === 0) return null;
  const chosenRoot = rows.roots.find((r) => r.id === root) ?? null;
  const kids = chosenRoot === null ? [] : (rows.children.get(chosenRoot.id) ?? []);
  return (
    <>
      <div className="xa-chips" role="group" aria-label={XA_TN.news_category_filter}>
        <button
          type="button"
          className={`xa-chip${root === null ? " xa-chip--on" : ""}`}
          aria-pressed={root === null}
          onClick={() => props.onRoot(null)}
        >
          {XA_TN.loc_tat_ca}
        </button>
        {rows.roots.map((r) => (
          <button
            key={r.id}
            type="button"
            className={`xa-chip${root === r.id ? " xa-chip--on" : ""}`}
            aria-pressed={root === r.id}
            onClick={() => props.onRoot(r.id)}
          >
            {r.name}
          </button>
        ))}
      </div>
      {chosenRoot !== null && kids.length > 0 && (
        <div
          className="xa-chips xa-chips--phu"
          role="group"
          aria-label={XA_TN.news_subcategory_filter(chosenRoot.name)}
        >
          <button
            type="button"
            className={`xa-chip xa-chip--phu${child === null ? " xa-chip--on" : ""}`}
            aria-pressed={child === null}
            onClick={() => props.onChild(null)}
          >
            {XA_TN.news_category_all_in_root}
          </button>
          {kids.map((c) => (
            <button
              key={c.id}
              type="button"
              className={`xa-chip xa-chip--phu${child === c.id ? " xa-chip--on" : ""}`}
              aria-pressed={child === c.id}
              onClick={() => props.onChild(c.id)}
            >
              {c.name}
            </button>
          ))}
        </div>
      )}
    </>
  );
}

/**
 * One type tab's body: the category chips, then the list for (type, category). The list is keyed by the
 * chosen category, so each selection is ONE fresh load through `useTinXa`'s StrictMode-safe ref guard, with
 * its own paging and "Xem thêm". Choosing a root filters by the root (the server includes descendants);
 * choosing a child filters by the child.
 */
function NewsTypePanel(props: { ten_mien: string; type: NewsType; onMo: (id: string) => void }) {
  const rows = useNewsCategories(props.ten_mien, props.type);
  const [root, setRoot] = useState<string | null>(null);
  const [child, setChild] = useState<string | null>(null);
  const category = child ?? root;
  const chosen = rows === null || category === null ? undefined : findCategory(rows, category);
  return (
    <>
      <CategoryChips
        rows={rows}
        root={root}
        child={child}
        onRoot={(id) => {
          setRoot(id);
          setChild(null);
        }}
        onChild={setChild}
      />
      <NewsOfType
        key={category ?? ""}
        ten_mien={props.ten_mien}
        type={props.type}
        category={category}
        onMo={props.onMo}
        empty={chosen !== undefined ? XA_TN.news_category_empty(chosen.name) : XA_TN.news_type_empty(NEWS_TYPE_LABEL[props.type])}
      />
    </>
  );
}

function findCategory(rows: CategoryRows, id: string): NewsCategory | undefined {
  return rows.roots.find((r) => r.id === id) ?? [...rows.children.values()].flat().find((c) => c.id === id);
}

/**
 * The chip rows for one type, loaded once when the tab's panel mounts (same ref guard as `useTinXa`).
 *
 * ⚠ A FAILED LOAD SHOWS NO CHIPS, AND SAYS NOTHING — deliberately (owner, 30/09/2026). The chips are an
 * optional filter over a list that loads and works on its own; an error block here would sit above a
 * working list and tell the citizen something is broken when nothing they need is. Every failure branch
 * (network, 5xx, malformed, bad domain) therefore ends in `null`, exactly like "no categories".
 */
function useNewsCategories(ten_mien: string, type: NewsType): CategoryRows | null {
  const [rows, setRows] = useState<CategoryRows | null>(null);
  const loaded = useRef(false);
  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void newsCategories(ten_mien, type).then((kq) => {
      if (kq.kieu === "xong") setRows(groupCategories(kq.gia_tri));
    });
  }, []);
  return rows;
}

/**
 * One type's list, loaded from the server with `?type=` (and `?category=` when given) when mounted. Used by
 * the type tabs and the Sự kiện tile.
 */
export function NewsOfType(props: {
  ten_mien: string;
  type: NewsType;
  category?: string | null;
  onMo: (id: string) => void;
  empty: string;
}) {
  const news = useTinXa(props.ten_mien, props.type, props.category ?? null);
  return <NewsListBody ds={news.ds} onMo={props.onMo} onTai={news.taiTiep} empty={props.empty} />;
}

/** Loading · failed · empty · the list with "Xem thêm". PURE apart from the callbacks. */
export function NewsListBody(props: { ds: DanhSachTin; onMo: (id: string) => void; onTai: () => void; empty: string }) {
  const { ds } = props;
  if (!ds.da_co_trang_dau && ds.dang_tai) {
    return <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai} dang_tai />;
  }
  if (!ds.da_co_trang_dau && ds.loi !== null) {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={CAU_LOI[ds.loi]}
        nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onTai } : undefined}
      />
    );
  }
  if (ds.muc.length === 0) return <KhoiTrangThai bieu_tuong="news" cau={props.empty} />;
  return (
    <>
      {/* Every item the same card — no featured first card (wave 1): a big coloured block with no picture in it
          read as the most important news, when it was only the newest. */}
      <ul className="xa-ds">
        {ds.muc.map((t) => (
          <li key={t.id}>
            <HangTin tin={t} onMo={props.onMo} />
          </li>
        ))}
      </ul>
      {ds.loi !== null && (
        <KhoiTrangThai
          bieu_tuong="alert"
          loi
          cau={CAU_LOI[ds.loi]}
          nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onTai } : undefined}
        />
      )}
      {ds.loi === null && ds.con_nua && (
        <button type="button" className="xa-nut xa-nut--phu" disabled={ds.dang_tai} onClick={props.onTai}>
          {ds.dang_tai ? TIN_XA.dang_tai_them : TIN_XA.nut_xem_them}
        </button>
      )}
      {!ds.con_nua && <p className="xa-phu xa-giua">{TIN_XA.het_danh_sach}</p>}
    </>
  );
}

/**
 * Tải danh sách tin của xã; trạng thái sống ở đây để tab và trang chủ dùng chung một lần tải. `type`
 * (tuỳ chọn): danh sách lọc theo loại ở máy chủ — một lần tải riêng, cho tab loại tin và ô Sự kiện.
 */
export function useTinXa(ten_mien: string, type: NewsType | null = null, category: string | null = null) {
  const [ds, datDs] = useState<DanhSachTin>(TIN_DAU);
  const da_tai = useRef(false);

  async function tai(con_tro: string) {
    datDs(batDauTaiTin);
    const kq = await tinCuaXa(ten_mien, con_tro, type, category);
    datDs((truoc) => sauKhiTaiTin(truoc, kq));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai("");
  }, []);

  return {
    ds,
    taiTiep: () => {
      if (!ds.dang_tai) void tai(ds.da_co_trang_dau ? ds.con_tro : "");
    },
  };
}

/** Một bài — ảnh bìa (khi có) + tiêu đề + ngày + toàn văn. Màn con, có nút quay lại. */
export function BaiTinXa(props: {
  ten_mien: string;
  id: string;
  onQuayLai: () => void;
  /** Tin đã tải ở danh sách — nguồn "tin liên quan" (không gọi thêm mạng). */
  ds?: readonly TinXaTomTat[];
  onMo?: (id: string) => void;
  /** The shell's opener for a video link (`OpenVideo`). Absent → no "Xem video" button. */
  openVideo?: OpenVideo;
}) {
  const [trang, datTrang] = useState<TrangBai>({ kieu: "dang-tai" });
  const [coverFailed, setCoverFailed] = useState(false);
  const [videoFailed, setVideoFailed] = useState(false);
  const da_tai = useRef(false);
  const openVideo = props.openVideo;

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiBai(await baiTinCuaXa(props.ten_mien, props.id)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  // The header names the item's TYPE (`PROTOTYPE.md` §6: "back, loại tin"): from the loaded item, or — while it
  // loads — from the list row it was opened from, so the title does not change under the reader. No type
  // known → the old general title.
  const type = trang.kieu === "xong" ? trang.bai.type : (props.ds?.find((t) => t.id === props.id)?.type ?? null);
  const title = type !== null ? NEWS_TYPE_LABEL[type] : TIN_XA.tieu_de;

  // The button exists only with BOTH a link on the item and an opener from the shell. A new tap clears the old
  // failure first, so the sentence never stands under a tap that is still opening.
  const videoUrl = trang.kieu === "xong" ? trang.bai.videoUrl : undefined;
  const onWatchVideo =
    openVideo === undefined || videoUrl === undefined
      ? undefined
      : () => {
          setVideoFailed(false);
          void videoOpenFailed(openVideo, videoUrl).then(setVideoFailed);
        };

  return (
    <>
      <DauManCon tieu_de={title} onQuayLai={props.onQuayLai} />
      <TrangCon>
        {trang.kieu === "dang-tai" && <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai_bai} dang_tai />}
        {trang.kieu === "khong-thay" && <KhoiTrangThai bieu_tuong="news" loi cau={TIN_XA.khong_thay} />}
        {trang.kieu === "loi" && (
          <KhoiTrangThai
            bieu_tuong="alert"
            loi
            cau={CAU_LOI[trang.loi]}
            nut={trang.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: () => void tai() } : undefined}
          />
        )}
        {trang.kieu === "xong" && (
          <NewsArticle
            bai={trang.bai}
            coverFailed={coverFailed}
            onCoverFail={() => setCoverFailed(true)}
            ds={props.ds ?? []}
            onMo={props.onMo}
            videoFailed={videoFailed}
            onWatchVideo={onWatchVideo}
          />
        )}
      </TrangCon>
    </>
  );
}

/**
 * The article's cover: the item's picture, 16:9, the full width of the screen, above the title. PURE.
 *
 * NO PICTURE, OR ONE THAT FAILED TO LOAD → NO BAND AT ALL (owner, 01/10/2026; same stance as `CommuneBanner`):
 * a coloured or empty block where a photo belongs reads as a page that failed to load. `alt=""`: the picture
 * is decoration — the title right under it says what the article is.
 */
export function ArticleCover(props: { imageUrl: string | undefined; failed: boolean; onFail: () => void }) {
  if (props.imageUrl === undefined || props.failed) return null;
  return (
    <div className="xa-bai__cover">
      <img className="xa-bai__cover-image" src={props.imageUrl} alt="" decoding="async" onError={props.onFail} />
    </div>
  );
}

/**
 * A loaded article — cover, title, meta line (published at), the event block, the "Xem video" button, body,
 * related items. PURE: the cover's and the video's failures are the caller's.
 *
 * "Xem video" (owner, 01/10/2026, ADR 0047 §6): only when the caller passes `onWatchVideo`, which `BaiTinXa` does
 * only for an item with a `videoUrl` (a `video` item with an https link) AND an injected opener. The way out is
 * the declared destination `"video"` (`content/dich-ra-ngoai.ts`), so the privacy policy's counted sentence
 * names it.
 */
export function NewsArticle(props: {
  bai: BaiTinXaData;
  coverFailed: boolean;
  onCoverFail: () => void;
  ds: readonly TinXaTomTat[];
  onMo?: (id: string) => void;
  onWatchVideo?: () => void;
  videoFailed?: boolean;
}) {
  const { bai } = props;
  return (
    <article className="xa-bai">
      <ArticleCover imageUrl={bai.imageUrl} failed={props.coverFailed} onFail={props.onCoverFail} />
      <h2 className="xa-bai__tieu-de">{bai.tieu_de}</h2>
      <p className="xa-phu">{dongPhu(bai)}</p>
      <EventDetails tin={bai} />
      {props.onWatchVideo !== undefined && <WatchVideo failed={props.videoFailed ?? false} onTap={props.onWatchVideo} />}
      <div className="xa-ke" />
      {chiaDoan(bai.noi_dung).map((doan, i) => (
        <p key={i} className="xa-bai__doan">
          {doan}
        </p>
      ))}
      <TinLienQuan ds={props.ds} bai={bai} onMo={props.onMo} />
    </article>
  );
}

function TinLienQuan(props: { ds: readonly TinXaTomTat[]; bai: TinXaTomTat; onMo?: (id: string) => void }) {
  const lq = tinLienQuan(props.ds, props.bai);
  if (lq.length === 0 || props.onMo === undefined) return null;
  const mo = props.onMo;
  return (
    <section className="xa-lien-quan">
      <h2 className="xa-dau-khoi__tieu-de">{XA_TN.tin_lien_quan}</h2>
      <ul className="xa-ds">
        {lq.map((t) => (
          <li key={t.id}>
            <HangTin tin={t} onMo={mo} />
          </li>
        ))}
      </ul>
    </section>
  );
}
