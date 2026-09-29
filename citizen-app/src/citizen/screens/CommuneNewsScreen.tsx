/**
 * MÀN "TIN TỨC CỦA XÃ" — tin xã đã đăng cho người dân, mới nhất trước; chạm một tin để đọc toàn văn.
 *
 * ⚠ CÔNG KHAI, KHÔNG CẦN PHIÊN (`api/vigov-client.ts` `communeNews` · `communeNewsArticle`). Cổng là TÊN MIỀN xã
 *   của lần mở này — màn chỉ được mở khi đã có nó (`CitizenChannel.tsx`).
 *
 * ⚠ VĂN BẢN THUẦN, KHÔNG BAO GIỜ HTML. Thân tin là chữ cán bộ gõ; nó được vẽ bằng nút chữ của React
 *   (tự thoát ký tự), chia đoạn theo dòng trống. Không `dangerouslySetInnerHTML` ở bất kỳ đâu: một
 *   thẻ lọt vào thân tin mà được vẽ ra là mã lạ chạy trong một app mang tên cơ quan nhà nước.
 *
 * ⚠ NÚT "XEM THÊM", KHÔNG CUỘN VÔ HẠN (`skills/accessibility-elderly`), cùng lối "Phản ánh của tôi".
 */
import { useEffect, useRef, useState } from "react";

import { communeNewsArticle, type PublicResult, communeNews } from "../api/vigov-client";
import { type CommuneNewsArticle, splitParagraphs, type CommuneNewsSummary, type CommuneNewsPage } from "../api/public-contract";
import { getVigovSession } from "../api/vigov-session";

import { CommuneBanner } from "./frame";
import { BACK, COMMUNE_NEWS } from "./copy";
import { UNREADABLE_DATE, vnDate } from "../../lib/date-time";

/** Ba câu lỗi. `khong-hop-le` không mời thử lại: bấm lại không đổi được gì. */
export type NewsError = "loi-mang" | "loi-may-chu" | "khong-hop-le";

const ERROR_TEXT: Readonly<Record<NewsError, string>> = {
  "loi-mang": COMMUNE_NEWS.network_error,
  "loi-may-chu": COMMUNE_NEWS.server_error,
  "khong-hop-le": COMMUNE_NEWS.invalid,
};

function errorOf(kind: Exclude<PublicResult<unknown>["kind"], "xong">): NewsError {
  if (kind === "loi-mang") return "loi-mang";
  if (kind === "khong-hop-le") return "khong-hop-le";
  return "loi-may-chu";
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DANH SÁCH — trạng thái thuần
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type NewsList = {
  readonly entries: readonly CommuneNewsSummary[];
  readonly cursor: string;
  readonly has_more: boolean;
  readonly has_first_page: boolean;
  readonly loading: boolean;
  readonly error: NewsError | null;
};

export const FIRST_NEWS: NewsList = {
  entries: [],
  cursor: "",
  has_more: false,
  has_first_page: false,
  loading: true,
  error: null,
};

export function startLoadingNews(list: NewsList): NewsList {
  return { ...list, loading: true, error: null };
}

/** Trang mới NỐI VÀO SAU; tin trùng `id` (hai lần tải chồng nhau) bị bỏ. */
export function afterNewsLoad(list: NewsList, result: PublicResult<CommuneNewsPage>): NewsList {
  if (result.kind !== "xong") return { ...list, loading: false, error: errorOf(result.kind) };
  const seen = new Set(list.entries.map((t) => t.id));
  return {
    entries: [...list.entries, ...result.value.entries.filter((t) => !seen.has(t.id))],
    cursor: result.value.cursor,
    has_more: result.value.has_more,
    has_first_page: true,
    loading: false,
    error: null,
  };
}

const date = (s: string) => vnDate(s) ?? UNREADABLE_DATE;

/** Dòng phụ chung cho thẻ và bài: chuyên mục · ngày đăng — bằng chữ, có nhãn. */
function SubLine({ news }: { news: CommuneNewsSummary }) {
  return (
    <span className="cd-the-cua-toi__dong">
      {news.category !== "" && `${COMMUNE_NEWS.category}: ${news.category} · `}
      {COMMUNE_NEWS.published_on}: {date(news.published_on)}
    </span>
  );
}

/** MỘT THẺ = MỘT NÚT to bằng cả thẻ. Tiêu đề to nhất, tóm tắt ngay dưới. */
export function NewsCard({ news, onOpen }: { news: CommuneNewsSummary; onOpen: (id: string) => void }) {
  return (
    <li className="cd-cua-toi__muc">
      <button type="button" className="cd-the-cua-toi" onClick={() => onOpen(news.id)}>
        <strong className="cd-tin__tieu-de">{news.title}</strong>
        <SubLine news={news} />
        {news.summary !== "" && <span className="cd-the-cua-toi__trich">{news.summary}</span>}
        <span className="cd-the-cua-toi__xem">{COMMUNE_NEWS.read_button}</span>
      </button>
    </li>
  );
}

export function CommuneNewsBody(props: { list: NewsList; onOpen: (id: string) => void; onLoad: () => void }) {
  const { list } = props;
  return (
    <>
      {!list.has_first_page && list.loading && (
        <p className="cd-cau" role="status">
          {COMMUNE_NEWS.loading}
        </p>
      )}
      {list.has_first_page && list.entries.length === 0 && <p className="cd-cau">{COMMUNE_NEWS.empty}</p>}
      {list.entries.length > 0 && (
        <ul className="cd-cua-toi">
          {list.entries.map((t) => (
            <NewsCard key={t.id} news={t} onOpen={props.onOpen} />
          ))}
        </ul>
      )}
      {list.has_first_page && list.loading && (
        <p className="cd-cau" role="status">
          {COMMUNE_NEWS.loading_more}
        </p>
      )}
      {list.error !== null && (
        <div className="cd-buoc">
          <p className="cd-loi" role="alert">
            {ERROR_TEXT[list.error]}
          </p>
          {list.error !== "khong-hop-le" && (
            <button type="button" className="cd-nut" disabled={list.loading} onClick={props.onLoad}>
              {COMMUNE_NEWS.retry_button}
            </button>
          )}
        </div>
      )}
      {list.error === null && list.has_more && (
        <button type="button" className="cd-nut-phu" disabled={list.loading} onClick={props.onLoad}>
          {COMMUNE_NEWS.load_more_button}
        </button>
      )}
      {list.has_first_page && !list.has_more && list.entries.length > 0 && (
        <p className="cd-ghi-chu">{COMMUNE_NEWS.end_of_list}</p>
      )}
    </>
  );
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * MỘT BÀI — toàn văn, văn bản thuần
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type ArticlePage =
  | { readonly kind: "dang-tai" }
  | { readonly kind: "xong"; readonly article: CommuneNewsArticle }
  | { readonly kind: "khong-thay" }
  | { readonly kind: "loi"; readonly error: NewsError };

export function afterArticleLoad(result: PublicResult<CommuneNewsArticle>): ArticlePage {
  if (result.kind === "xong") return { kind: "xong", article: result.value };
  if (result.kind === "khong-thay") return { kind: "khong-thay" };
  return { kind: "loi", error: errorOf(result.kind) };
}

/** Toàn văn. Mỗi đoạn là MỘT `<p>` chứa CHỮ — React thoát mọi ký tự, kể cả `<script>`. */
export function NewsArticle({ article }: { article: CommuneNewsArticle }) {
  return (
    <article className="cd-tin">
      <h2 className="cd-tieu-de-phu">{article.title}</h2>
      <p className="cd-ghi-chu">
        <SubLine news={article} />
      </p>
      {splitParagraphs(article.content).map((paragraph, i) => (
        <p key={i} className="cd-tin__doan">
          {paragraph}
        </p>
      ))}
    </article>
  );
}

export function ArticleBody(props: { page: ArticlePage; onLoad: () => void }) {
  const { page } = props;
  if (page.kind === "dang-tai") {
    return (
      <p className="cd-cau" role="status">
        {COMMUNE_NEWS.loading_article}
      </p>
    );
  }
  if (page.kind === "xong") return <NewsArticle article={page.article} />;
  if (page.kind === "khong-thay") {
    return (
      <p className="cd-loi" role="alert">
        {COMMUNE_NEWS.not_found}
      </p>
    );
  }
  return (
    <div className="cd-buoc">
      <p className="cd-loi" role="alert">
        {ERROR_TEXT[page.error]}
      </p>
      {page.error !== "khong-hop-le" && (
        <button type="button" className="cd-nut" onClick={props.onLoad}>
          {COMMUNE_NEWS.retry_button}
        </button>
      )}
    </div>
  );
}

function ArticleScreen(props: { domain: string; id: string; onBack: () => void; commune_name: string | null }) {
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
    <section className="cd-man" aria-label={COMMUNE_NEWS.title}>
      <button type="button" className="quay-lai" onClick={props.onBack}>
        {BACK}
      </button>
      {props.commune_name !== null && <CommuneBanner commune_name={props.commune_name} />}
      <ArticleBody page={page} onLoad={() => void load()} />
    </section>
  );
}

export function CommuneNewsScreen(props: { domain: string; onBack: () => void }) {
  const [commune_name] = useState(() => getVigovSession()?.commune_name ?? null);
  const [list, setList] = useState<NewsList>(FIRST_NEWS);
  const [reading, setReading] = useState<string | null>(null);
  const first_loaded = useRef(false);

  async function load(cursor: string) {
    setList(startLoadingNews);
    const result = await communeNews(props.domain, cursor);
    setList((previous) => afterNewsLoad(previous, result));
  }

  useEffect(() => {
    if (first_loaded.current) return;
    first_loaded.current = true;
    void load("");
  }, []);

  return (
    <>
      {/* Danh sách chỉ bị ẩn khi đọc một tin — "Quay lại" về đúng chỗ đang đọc, không tải lại. */}
      <section className="cd-man" aria-label={COMMUNE_NEWS.title} hidden={reading !== null}>
        <button type="button" className="quay-lai" onClick={props.onBack}>
          {BACK}
        </button>
        {commune_name !== null && <CommuneBanner commune_name={commune_name} />}
        <h1 className="cd-tieu-de">{COMMUNE_NEWS.title}</h1>
        <CommuneNewsBody
          list={list}
          onOpen={setReading}
          onLoad={() => {
            if (!list.loading) void load(list.cursor);
          }}
        />
      </section>
      {reading !== null && (
        <ArticleScreen
          key={reading}
          domain={props.domain}
          id={reading}
          commune_name={commune_name}
          onBack={() => setReading(null)}
        />
      )}
    </>
  );
}
