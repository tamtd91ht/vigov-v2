"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { AUTHORITY_KIND } from "@/components/commune-identity";
import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { APP_VERSION_LABEL, PRODUCT_NAME } from "@/components/side-nav";
import { rememberedMenuPermissions } from "@/components/sidebar-state";
import { buttonVariants } from "@/components/ui/button";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { cn } from "@/lib/cn";

import { GUIDE_SECTIONS, GUIDE_TITLE, type GuideBlock, type GuideSection, type GuideText } from "./guide-content";
import { guideAccess, visibleGuide } from "./guide-visibility";

export const GUIDE_TOC_LABEL = "Nội dung";

/**
 * `/huong-dan` — the end-user guide (owner 17/10/2026): the prototype guide's look (cover, two-column
 * table of contents, numbered sections with a left accent, steps, note boxes, tables) WITHOUT its
 * screenshots, and a row of buttons per flow into the real screen instead.
 *
 * WHO SEES WHAT: every signed-in account opens the page; a section whose menu item this account cannot
 * see is left out, and so is a button needing a key it lacks (`guide-visibility.ts`). The permissions are
 * read exactly as the sidebar reads them (`DauTrang`): the session's list, or — while this page's session
 * read is in flight — the previous screen's, so moving here from another screen does not empty the page
 * and refill it. On the very first screen of a tab that is `null`, and only the sections tied to no menu
 * show until the session arrives: sections can appear, never appear and then vanish.
 *
 * THE COMMUNE'S NAME comes from the runtime configuration (`useCauHinhXa`), never a constant (rule 1,
 * invariant 10). The section numbers are counted over the VISIBLE sections, so a reader never sees a gap.
 */
export function UserGuide() {
  const commune = useCauHinhXa();
  const session = usePhien();
  const read = session === null ? null : session.ok ? session.duLieu.permissions : [];
  const permissions = read ?? rememberedMenuPermissions();
  const sections = visibleGuide(GUIDE_SECTIONS, guideAccess(permissions, session));
  return <UserGuideView commune={commune} sections={sections} />;
}

/** The page body for a given commune and the sections already filtered — pure, so tests render it. */
export function UserGuideView({ commune, sections }: { commune: CauHinhXaHienThi; sections: readonly GuideSection[] }) {
  return (
    <div className="user-guide mx-auto w-full max-w-[62rem] text-[14px] leading-relaxed text-ink">
      <header className="mb-8 rounded-[12px] border-b-[5px] border-solid border-brand bg-navy px-5 py-8 text-white sm:px-8 print:[print-color-adjust:exact]">
        <h1 className="m-0 mb-2 text-[26px] leading-tight font-bold sm:text-[32px]">{GUIDE_TITLE}</h1>
        <p className="m-0 max-w-[46rem] text-white/85">
          Tài liệu dành cho cán bộ, công chức và lãnh đạo {AUTHORITY_KIND} <span className="guide-commune">{commune.displayName}</span>
          {commune.parentAuthority !== "" && <>, {commune.parentAuthority}</>} — đọc từ trên xuống là làm được việc. Mỗi phần có nút mở
          thẳng màn hình tương ứng.
        </p>
        <p className="m-0 mt-5 text-[12.5px] text-white/70">
          {PRODUCT_NAME} · {APP_VERSION_LABEL}
        </p>
      </header>

      <nav aria-label={GUIDE_TOC_LABEL} className="mb-10 rounded-[12px] border border-solid border-line bg-white px-5 py-4 print:break-after-page">
        <h2 className="m-0 mb-2 text-[12px] font-semibold tracking-wider text-ink-muted uppercase">{GUIDE_TOC_LABEL}</h2>
        <ol className="m-0 columns-1 gap-8 pl-5 sm:columns-2">
          {sections.map((s) => (
            <li key={s.id} className="my-1 break-inside-avoid">
              <a href={`#${s.id}`} className="text-navy no-underline hover:underline">
                {s.title}
              </a>
            </li>
          ))}
        </ol>
      </nav>

      {sections.map((s, i) => (
        <GuideSectionView key={s.id} section={s} number={i + 1} />
      ))}
    </div>
  );
}

function GuideSectionView({ section, number }: { section: GuideSection; number: number }) {
  const headingId = `${section.id}-tieu-de`;
  return (
    <section id={section.id} aria-labelledby={headingId} className="mb-12 scroll-mt-20 print:break-inside-avoid-page">
      <h2 id={headingId} className="m-0 border-l-[5px] border-solid border-brand pl-3 text-[21px] leading-tight font-bold text-navy">
        <span className="mr-1.5 font-semibold text-ink-muted">{number}.</span>
        {section.title}
      </h2>
      <p className="m-0 mt-2 mb-4 max-w-[46rem]">
        <RichText text={section.lede} />
      </p>
      {section.actions.length > 0 && (
        <div className="guide-actions mb-4 flex flex-wrap gap-2 print:hidden">
          {section.actions.map((a) => (
            <Link
              key={a.href}
              href={a.href}
              className={cn(buttonVariants({ variant: a.primary === true ? "primary" : "secondary", size: "lg" }), "px-3.5")}
            >
              {a.label}
              {a.primary === true && <ArrowRight aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            </Link>
          ))}
        </div>
      )}
      {section.blocks.map((b, i) => (
        <GuideBlockView key={i} block={b} />
      ))}
    </section>
  );
}

function GuideBlockView({ block }: { block: GuideBlock }) {
  switch (block.kind) {
    case "steps":
    case "bullets": {
      const items = block.items.map((t, i) => (
        <li key={i} className="my-1.5">
          <RichText text={t} />
        </li>
      ));
      return (
        <>
          {block.title !== undefined && <h3 className="m-0 mt-6 mb-1.5 text-[15px] font-semibold text-navy">{block.title}</h3>}
          {block.kind === "steps" ? (
            <ol className="m-0 mt-2 list-decimal pl-5">{items}</ol>
          ) : (
            <ul className="m-0 mt-2 list-disc pl-5">{items}</ul>
          )}
        </>
      );
    }
    case "paragraph":
      return (
        <p className="m-0 my-3">
          <RichText text={block.text} />
        </p>
      );
    case "note":
      return (
        <div
          className={cn(
            "my-4 rounded-[10px] border border-solid px-4 py-3 text-[13.5px]",
            block.tone === "warn" ? "border-warning-500/40 bg-warning-50" : "border-line bg-canvas",
          )}
          data-tone={block.tone}
        >
          <RichText text={block.text} />
        </div>
      );
    case "table":
      return (
        <div className="my-4 overflow-x-auto">
          <table className="w-full border-collapse text-[13.5px]">
            <thead>
              <tr>
                {block.head.map((h) => (
                  <th key={h} scope="col" className="bg-navy px-3 py-2 text-left font-semibold text-white">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {block.rows.map((r, i) => (
                <tr key={i} className="even:bg-canvas">
                  <td className="border-b border-solid border-line px-3 py-2 align-top">
                    <RichText text={r[0]} />
                  </td>
                  <td className="border-b border-solid border-line px-3 py-2 align-top">
                    <RichText text={r[1]} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
  }
}

/**
 * `**…**` → bold runs, everything else plain text. React nodes only — the guide never injects HTML
 * (rule 13, forbidden #3), so a stray `<` in a sentence is printed, not parsed.
 */
export function RichText({ text }: { text: GuideText }): ReactNode {
  const parts = text.split(/\*\*(.+?)\*\*/g);
  return parts.map((p, i) =>
    i % 2 === 1 ? (
      <strong key={i} className="font-semibold text-navy">
        {p}
      </strong>
    ) : (
      p
    ),
  );
}
