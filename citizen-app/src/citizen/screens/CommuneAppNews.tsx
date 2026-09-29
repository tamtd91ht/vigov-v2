/**
 * TIN TỨC CỦA XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/news/*`): bài đầu là thẻ nổi
 * bật có dải màu, các bài sau là hàng có ô vuông; chạm để đọc toàn văn.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `CommuneNewsScreen.tsx` — cùng trạng thái thuần (`FIRST_NEWS`,
 * `afterNewsLoad`, `afterArticleLoad`), cùng tuyến công khai theo tên miền, cùng "Xem thêm" thay cho cuộn
 * vô hạn. VĂN BẢN THUẦN: thân tin vẽ bằng nút chữ của React, chia đoạn theo dòng trống; không HTML.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { communeNewsArticle, communeNews } from "../api/vigov-client";
import { splitParagraphs, type NewsType, type CommuneNewsSummary } from "../api/public-contract";
import { UNREADABLE_DATE, vnDate } from "../../lib/date-time";
import { NEWS_TYPE_LABEL, COMMUNE_NEWS, COMMUNE_APP_SCREENS } from "./copy";
import {
  startLoadingNews,
  type NewsList,
  afterArticleLoad,
  afterNewsLoad,
  FIRST_NEWS,
  type ArticlePage,
} from "./CommuneNewsScreen";

import { Icon } from "./Icon";
import { SubScreenHeader, StatusBlock, SubPage } from "./commune-frame";

const date = (s: string) => vnDate(s) ?? UNREADABLE_DATE;

function subLine(news: CommuneNewsSummary): string {
  return news.category !== "" ? `${news.category} · ${date(news.published_on)}` : date(news.published_on);
}

/** Một hàng tin: ô vuông + tiêu đề + chuyên mục · ngày. Dùng cả ở trang chủ. */
export function NewsRow({ news, onOpen }: { news: CommuneNewsSummary; onOpen: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={() => onOpen(news.id)}>
      <span className="xa-hang-tin__o">
        <Icon name="news" size={26} />
      </span>
      <span className="xa-hang-tin__chu">
        <strong className="xa-hang-tin__tieu-de">{news.title}</strong>
        <span className="xa-phu">{subLine(news)}</span>
      </span>
    </button>
  );
}

/** Bài đầu danh sách: dải màu lớn + tiêu đề + tóm tắt. */
function FeaturedCard({ news, onOpen }: { news: CommuneNewsSummary; onOpen: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-noi-bat" onClick={() => onOpen(news.id)}>
      <span className="xa-noi-bat__bia" aria-hidden="true">
        <Icon name="news" size={96} />
      </span>
      <span className="xa-noi-bat__chu">
        <strong className="xa-noi-bat__tieu-de">{news.title}</strong>
        {news.summary !== "" && <span className="xa-noi-bat__tom-tat">{news.summary}</span>}
        <span className="xa-phu">{subLine(news)}</span>
      </span>
    </button>
  );
}

const ERROR_TEXT = {
  "loi-mang": COMMUNE_NEWS.network_error,
  "loi-may-chu": COMMUNE_NEWS.server_error,
  "khong-hop-le": COMMUNE_NEWS.invalid,
} as const;

/**
 * The type chips of the news tab — the prototype's three (Tin tức · Sự kiện · Thông báo), now real: each
 * chip is a SERVER filter (`?type=`, comms b22bf76), not a guess from the free-text category the old chips
 * used. Truyền thanh and Video have their own tiles; banners are not articles.
 */
export const NEWS_CHIPS: readonly NewsType[] = ["tin-tuc", "su-kien", "thong-bao"];

/** Tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3. THUẦN. */
export function relatedNews(list: readonly CommuneNewsSummary[], reading: CommuneNewsSummary | null, max = 3): CommuneNewsSummary[] {
  if (reading === null || reading.category === "") return [];
  return list.filter((t) => t.id !== reading.id && t.category === reading.category).slice(0, max);
}

/**
 * Danh sách tin — thân của tab "Tin tức". "Tất cả" is the list the home screen shares (`list`); a type chip
 * mounts its own server-filtered list (`NewsOfType`), with its own pages and its own "Xem thêm".
 */
export function CommuneNewsList(props: {
  domain: string;
  list: NewsList;
  onOpen: (id: string) => void;
  onLoad: () => void;
}) {
  const [type, setType] = useState<NewsType | null>(null);
  const chips = (
    <div className="xa-chips" role="group" aria-label={COMMUNE_APP_SCREENS.filter_news_type}>
      <button type="button" className={`xa-chip${type === null ? " xa-chip--on" : ""}`} aria-pressed={type === null} onClick={() => setType(null)}>
        {COMMUNE_APP_SCREENS.filter_all}
      </button>
      {NEWS_CHIPS.map((t) => (
        <button key={t} type="button" className={`xa-chip${type === t ? " xa-chip--on" : ""}`} aria-pressed={type === t} onClick={() => setType(t)}>
          {NEWS_TYPE_LABEL[t]}
        </button>
      ))}
    </div>
  );
  return (
    <>
      {chips}
      {type === null ? (
        <NewsListBody list={props.list} onOpen={props.onOpen} onLoad={props.onLoad} empty={COMMUNE_NEWS.empty} />
      ) : (
        <NewsOfType key={type} domain={props.domain} type={type} onOpen={props.onOpen} empty={COMMUNE_APP_SCREENS.news_type_empty(NEWS_TYPE_LABEL[type])} />
      )}
    </>
  );
}

/** One type's list, loaded from the server with `?type=` when mounted. Used by the chips and the Sự kiện tile. */
export function NewsOfType(props: { domain: string; type: NewsType; onOpen: (id: string) => void; empty: string }) {
  const news = useCommuneNews(props.domain, props.type);
  return <NewsListBody list={news.list} onOpen={props.onOpen} onLoad={news.loadMore} empty={props.empty} />;
}

/** Loading · failed · empty · the list with "Xem thêm". PURE apart from the callbacks. */
export function NewsListBody(props: { list: NewsList; onOpen: (id: string) => void; onLoad: () => void; empty: string }) {
  const { list } = props;
  if (!list.has_first_page && list.loading) {
    return <StatusBlock icon="news" text={COMMUNE_NEWS.loading} loading />;
  }
  if (!list.has_first_page && list.error !== null) {
    return (
      <StatusBlock
        icon="alert"
        error
        text={ERROR_TEXT[list.error]}
        button={list.error !== "khong-hop-le" ? { label: COMMUNE_NEWS.retry_button, onPress: props.onLoad } : undefined}
      />
    );
  }
  if (list.entries.length === 0) return <StatusBlock icon="news" text={props.empty} />;
  return (
    <>
      <ul className="xa-ds">
        {list.entries.map((t, i) => (
          <li key={t.id}>{i === 0 ? <FeaturedCard news={t} onOpen={props.onOpen} /> : <NewsRow news={t} onOpen={props.onOpen} />}</li>
        ))}
      </ul>
      {list.error !== null && (
        <StatusBlock
          icon="alert"
          error
          text={ERROR_TEXT[list.error]}
          button={list.error !== "khong-hop-le" ? { label: COMMUNE_NEWS.retry_button, onPress: props.onLoad } : undefined}
        />
      )}
      {list.error === null && list.has_more && (
        <button type="button" className="xa-nut xa-nut--phu" disabled={list.loading} onClick={props.onLoad}>
          {list.loading ? COMMUNE_NEWS.loading_more : COMMUNE_NEWS.load_more_button}
        </button>
      )}
      {!list.has_more && <p className="xa-phu xa-giua">{COMMUNE_NEWS.end_of_list}</p>}
    </>
  );
}

/**
 * Tải danh sách tin của xã; trạng thái sống ở đây để tab và trang chủ dùng chung một lần tải. `type`
 * (tuỳ chọn): danh sách lọc theo loại ở máy chủ — một lần tải riêng, cho chip và ô Sự kiện.
 */
export function useCommuneNews(domain: string, type: NewsType | null = null) {
  const [list, setList] = useState<NewsList>(FIRST_NEWS);
  const loaded = useRef(false);

  async function load(cursor: string) {
    setList(startLoadingNews);
    const result = await communeNews(domain, cursor, type);
    setList((previous) => afterNewsLoad(previous, result));
  }

  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void load("");
  }, []);

  return {
    list,
    loadMore: () => {
      if (!list.loading) void load(list.has_first_page ? list.cursor : "");
    },
  };
}

/** Một bài — dải bìa + tiêu đề + ngày + toàn văn. Màn con, có nút quay lại. */
export function CommuneNewsArticle(props: {
  domain: string;
  id: string;
  onBack: () => void;
  /** Tin đã tải ở danh sách — nguồn "tin liên quan" (không gọi thêm mạng). */
  list?: readonly CommuneNewsSummary[];
  onOpen?: (id: string) => void;
}) {
  const [page, setPage] = useState<ArticlePage>({ kind: "dang-tai" });
  const loaded = useRef(false);

  async function load() {
    setPage({ kind: "dang-tai" });
    setPage(afterArticleLoad(await communeNewsArticle(props.domain, props.id)));
  }

  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void load();
  }, []);

  return (
    <>
      <SubScreenHeader title={COMMUNE_NEWS.title} onBack={props.onBack} />
      <SubPage>
        {page.kind === "dang-tai" && <StatusBlock icon="news" text={COMMUNE_NEWS.loading_article} loading />}
        {page.kind === "khong-thay" && <StatusBlock icon="news" error text={COMMUNE_NEWS.not_found} />}
        {page.kind === "loi" && (
          <StatusBlock
            icon="alert"
            error
            text={ERROR_TEXT[page.error]}
            button={page.error !== "khong-hop-le" ? { label: COMMUNE_NEWS.retry_button, onPress: () => void load() } : undefined}
          />
        )}
        {page.kind === "xong" && (
          <article className="xa-bai">
            <div className="xa-bai__bia" aria-hidden="true">
              <Icon name="news" size={110} />
            </div>
            <h2 className="xa-bai__tieu-de">{page.article.title}</h2>
            <p className="xa-phu">{subLine(page.article)}</p>
            <div className="xa-ke" />
            {splitParagraphs(page.article.content).map((paragraph, i) => (
              <p key={i} className="xa-bai__doan">
                {paragraph}
              </p>
            ))}
            <RelatedNews list={props.list ?? []} article={page.article} onOpen={props.onOpen} />
          </article>
        )}
      </SubPage>
    </>
  );
}

function RelatedNews(props: { list: readonly CommuneNewsSummary[]; article: CommuneNewsSummary; onOpen?: (id: string) => void }) {
  const related = relatedNews(props.list, props.article);
  if (related.length === 0 || props.onOpen === undefined) return null;
  const open = props.onOpen;
  return (
    <section className="xa-lien-quan">
      <h2 className="xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.related_news}</h2>
      <ul className="xa-ds">
        {related.map((t) => (
          <li key={t.id}>
            <NewsRow news={t} onOpen={open} />
          </li>
        ))}
      </ul>
    </section>
  );
}
