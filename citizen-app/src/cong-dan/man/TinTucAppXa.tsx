/**
 * TIN TỨC CỦA XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/news/*`): bài đầu là thẻ nổi
 * bật có dải màu, các bài sau là hàng có ô vuông; chạm để đọc toàn văn.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `TinTucXaScreen.tsx` — cùng trạng thái thuần (`TIN_DAU`,
 * `sauKhiTaiTin`, `sauKhiTaiBai`), cùng tuyến công khai theo tên miền, cùng "Xem thêm" thay cho cuộn
 * vô hạn. VĂN BẢN THUẦN: thân tin vẽ bằng nút chữ của React, chia đoạn theo dòng trống; không HTML.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { baiTinCuaXa, newsCategories, tinCuaXa } from "../api/goi-vigov";
import { chiaDoan, type NewsCategory, type NewsType, type TinXaTomTat } from "../api/hop-dong-cong-khai";
import { NGAY_KHONG_DOC_DUOC, ngayVN } from "../../lib/thoi-diem";
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

function dongPhu(tin: TinXaTomTat): string {
  return tin.chuyen_muc !== "" ? `${tin.chuyen_muc} · ${ngay(tin.ngay_dang)}` : ngay(tin.ngay_dang);
}

/** Một hàng tin: ô vuông + tiêu đề + chuyên mục · ngày. Dùng cả ở trang chủ. */
export function HangTin({ tin, onMo }: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={() => onMo(tin.id)}>
      <span className="xa-hang-tin__o">
        <BieuTuong ten="news" co={26} />
      </span>
      <span className="xa-hang-tin__chu">
        <strong className="xa-hang-tin__tieu-de">{tin.tieu_de}</strong>
        <span className="xa-phu">{dongPhu(tin)}</span>
      </span>
    </button>
  );
}

/** Bài đầu danh sách: dải màu lớn + tiêu đề + tóm tắt. */
function TheNoiBat({ tin, onMo }: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-noi-bat" onClick={() => onMo(tin.id)}>
      <span className="xa-noi-bat__bia" aria-hidden="true">
        <BieuTuong ten="news" co={96} />
      </span>
      <span className="xa-noi-bat__chu">
        <strong className="xa-noi-bat__tieu-de">{tin.tieu_de}</strong>
        {tin.tom_tat !== "" && <span className="xa-noi-bat__tom-tat">{tin.tom_tat}</span>}
        <span className="xa-phu">{dongPhu(tin)}</span>
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
      <ul className="xa-ds">
        {ds.muc.map((t, i) => (
          <li key={t.id}>{i === 0 ? <TheNoiBat tin={t} onMo={props.onMo} /> : <HangTin tin={t} onMo={props.onMo} />}</li>
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

/** Một bài — dải bìa + tiêu đề + ngày + toàn văn. Màn con, có nút quay lại. */
export function BaiTinXa(props: {
  ten_mien: string;
  id: string;
  onQuayLai: () => void;
  /** Tin đã tải ở danh sách — nguồn "tin liên quan" (không gọi thêm mạng). */
  ds?: readonly TinXaTomTat[];
  onMo?: (id: string) => void;
}) {
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
    <>
      <DauManCon tieu_de={TIN_XA.tieu_de} onQuayLai={props.onQuayLai} />
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
          <article className="xa-bai">
            <div className="xa-bai__bia" aria-hidden="true">
              <BieuTuong ten="news" co={110} />
            </div>
            <h2 className="xa-bai__tieu-de">{trang.bai.tieu_de}</h2>
            <p className="xa-phu">{dongPhu(trang.bai)}</p>
            <div className="xa-ke" />
            {chiaDoan(trang.bai.noi_dung).map((doan, i) => (
              <p key={i} className="xa-bai__doan">
                {doan}
              </p>
            ))}
            <TinLienQuan ds={props.ds ?? []} bai={trang.bai} onMo={props.onMo} />
          </article>
        )}
      </TrangCon>
    </>
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
