/**
 * The three Tổng quan files — PDF, XLSX, PPTX — built IN THE BROWSER from a `DashboardExport`
 * (`export-model.ts`), which is itself the figures already on screen. Nothing here reads a server,
 * computes a figure or reads the clock: what goes in is what the page drew. `/bao-cao` uses the same
 * builders (owner 09/10/2026, D3); its `tables` render after the blocks, and only when present.
 *
 * Layout and wording follow the prototype's exporters (`vigov-require/apps/api/app/modules/reports/
 * exporters.py` `to_pdf` / `to_xlsx` / `to_pptx`): title, commune, period, "tính đến", one band per
 * block, the metric cards, the notes; footer with the commune and the page number.
 *
 * EVERY LIBRARY IS A DYNAMIC IMPORT, on the click: jsPDF, pptxgenjs and write-excel-file never enter
 * the dashboard's own bundle, and a reader who never exports never downloads them. None of the three
 * needs `eval` / `new Function` (checked in their builds, 06/10/2026), so a CSP without 'unsafe-eval'
 * keeps working when web-admin gets one (`tools/security_debt.json`, missing-security-headers).
 *
 * VIETNAMESE: XLSX and PPTX store text as UTF-8 XML and name Arial, the face the prototype chose because
 * the communes' Office installs and meeting-room projectors all have it with every diacritic. A PDF has
 * no such fallback — the font must travel INSIDE the file — so jsPDF embeds Roboto TTF (the UI's own
 * face, `@expo-google-fonts/roboto`, copied to `public/pdf-fonts/` by `scripts/copy-pdf-fonts.mjs`).
 * jsPDF's 14 standard fonts are WinAnsi and would print "Tổng" as "T?ng".
 */

import type { Cell, CellObject, SheetData } from "write-excel-file/universal";

import type { DashboardExport, ExportBlock, ExportFigure, ExportFormat, ExportTable } from "./export-model";
import { EXPORT_MIME } from "./export-model";

/** ViHAT palette of the prototype's exporters (hex without `#`). */
const NAVY = "102B43";
const BRAND = "2FB1F9";
const DANGER = "C3373C";
const SURFACE = "F2F6F9";
const LINE = "D8E0E8";
const INK = "43607B";
const MUTED = "6B7C8C";
const WHITE = "FFFFFF";

/** The face named in XLSX / PPTX — see the header. */
const OFFICE_FONT = "Arial";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * XLSX
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

type XlsxCell = Cell;

const XLSX_COLUMNS = 4;

function spanRow(cell: CellObject): XlsxCell[] {
  return [{ ...cell, columnSpan: XLSX_COLUMNS, wrap: true }, ...Array<XlsxCell>(XLSX_COLUMNS - 1).fill(null)];
}

const BORDER: CellObject = { borderStyle: "thin", borderColor: `#${LINE}` };

/**
 * One figure row. The VALUE is the server's number when there is one (an accountant sorts and totals
 * this column — a column of text that looks like numbers does neither), otherwise the tile's text.
 * An amount goes in to the đồng; the tile's shortened "9,64 tỷ đồng" is the same figure, rounded.
 */
function figureRow(f: ExportFigure): XlsxCell[] {
  const value: CellObject =
    f.number !== undefined
      ? {
          value: f.number,
          type: Number,
          format: f.unit === "dong" ? '#,##0" đồng"' : "#,##0",
          align: "right",
        }
      : { value: f.value, type: String, align: f.noFigure === true ? "left" : "right", wrap: true };
  return [
    { value: f.label, type: String, wrap: true, ...BORDER },
    {
      ...value,
      ...BORDER,
      fontWeight: f.noFigure === true ? undefined : "bold",
      textColor: f.alert === true ? `#${DANGER}` : f.noFigure === true ? `#${MUTED}` : undefined,
    },
    { value: f.comparison ?? "", type: String, wrap: true, ...BORDER },
    { value: f.note ?? "", type: String, wrap: true, ...BORDER },
  ];
}

function blockRows(b: ExportBlock): XlsxCell[][] {
  const rows: XlsxCell[][] = [
    spanRow({ value: b.title, fontWeight: "bold", fontSize: 12, textColor: `#${WHITE}`, backgroundColor: `#${NAVY}` }),
    ["Chỉ số", "Giá trị", "So với kỳ trước", "Ghi chú"].map((h): CellObject => ({
      value: h,
      fontWeight: "bold",
      textColor: `#${NAVY}`,
      backgroundColor: `#${SURFACE}`,
      ...BORDER,
    })),
    ...b.figures.map(figureRow),
  ];
  if (b.pending !== undefined) rows.push(spanRow({ value: `Chưa có số liệu: ${b.pending}`, textColor: `#${MUTED}` }));
  for (const e of b.errors) rows.push(spanRow({ value: e, textColor: `#${DANGER}` }));
  for (const n of b.notes) rows.push(spanRow({ value: n, textColor: `#${MUTED}` }));
  rows.push([null]);
  return rows;
}

/**
 * A page table (`/bao-cao`'s unit table and comparison): a title band, a header row, one row per line —
 * a count goes in as a NUMBER cell, everything else as the screen's text.
 */
function tableRows(t: ExportTable): XlsxCell[][] {
  const span = Math.max(XLSX_COLUMNS, t.columns.length);
  const wide = (cell: CellObject): XlsxCell[] => [
    { ...cell, columnSpan: span, wrap: true },
    ...Array<XlsxCell>(span - 1).fill(null),
  ];
  const rows: XlsxCell[][] = [
    wide({ value: t.title, fontWeight: "bold", fontSize: 12, textColor: `#${WHITE}`, backgroundColor: `#${NAVY}` }),
    t.columns.map((h): CellObject => ({
      value: h,
      fontWeight: "bold",
      textColor: `#${NAVY}`,
      backgroundColor: `#${SURFACE}`,
      wrap: true,
      ...BORDER,
    })),
    ...t.rows.map((r) =>
      r.map((c, i): CellObject =>
        c.number !== undefined
          ? { value: c.number, type: Number, format: "#,##0", align: "right", ...BORDER }
          : { value: c.text, type: String, wrap: true, align: i === 0 ? "left" : "right", ...BORDER },
      ),
    ),
  ];
  if (t.rows.length === 0) rows.push(wide({ value: t.empty, textColor: `#${MUTED}` }));
  for (const e of t.errors) rows.push(wide({ value: e, textColor: `#${DANGER}` }));
  for (const n of t.notes) rows.push(wide({ value: n, textColor: `#${MUTED}` }));
  rows.push([null]);
  return rows;
}

export async function buildXlsx(doc: DashboardExport): Promise<Blob> {
  const { default: writeXlsxFile } = await import("write-excel-file/universal");
  const tables = doc.tables ?? [];
  const rows: XlsxCell[][] = [
    spanRow({ value: doc.title, fontWeight: "bold", fontSize: 15, textColor: `#${NAVY}` }),
    spanRow({ value: doc.communeName, fontWeight: "bold", fontSize: 12, textColor: `#${NAVY}` }),
    ...(doc.parentAuthority === "" ? [] : [spanRow({ value: doc.parentAuthority, textColor: `#${INK}` })]),
    spanRow({ value: doc.periodLabel, textColor: `#${INK}` }),
    spanRow({ value: doc.asOfLine, textColor: `#${INK}` }),
    [null],
    ...doc.blocks.flatMap(blockRows),
    ...tables.flatMap(tableRows),
  ];
  if (doc.urgent !== null) {
    rows.push(spanRow({ value: doc.urgent.title, fontWeight: "bold", fontSize: 12, textColor: `#${WHITE}`, backgroundColor: `#${DANGER}` }));
    for (const l of doc.urgent.lines) rows.push(spanRow({ value: l }));
    rows.push([null]);
  }
  for (const n of doc.footnotes) rows.push(spanRow({ value: n, textColor: `#${MUTED}` }));
  rows.push(spanRow({ value: doc.generatedLine, textColor: `#${MUTED}` }));

  const sheet: SheetData = rows;
  const blob = await writeXlsxFile(sheet, {
    sheet: "Tổng quan",
    // A fifth column only when a table needs it — Tổng quan's workbook keeps its four.
    columns: [
      { width: 42 },
      { width: 24 },
      { width: 30 },
      { width: 70 },
      ...(tables.some((t) => t.columns.length > XLSX_COLUMNS) ? [{ width: 18 }] : []),
    ],
  }, { fontFamily: OFFICE_FONT, fontSize: 11 }).toBlob();
  return new Blob([blob], { type: EXPORT_MIME.xlsx });
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PDF
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Same-origin static files, copied from `@expo-google-fonts/roboto` at build (`scripts/copy-pdf-fonts.mjs`). */
export const PDF_FONT_URLS = {
  normal: "/pdf-fonts/Roboto-Regular.ttf",
  bold: "/pdf-fonts/Roboto-Bold.ttf",
} as const;

export type PdfFonts = { readonly normal: Uint8Array; readonly bold: Uint8Array };

async function fetchFont(url: string): Promise<Uint8Array> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`PDF font ${url}: HTTP ${res.status}`);
  return new Uint8Array(await res.arrayBuffer());
}

/** Fetches the two TTFs. A platform asset, identical for every commune — nothing commune-specific. */
export async function loadPdfFonts(): Promise<PdfFonts> {
  const [normal, bold] = await Promise.all([fetchFont(PDF_FONT_URLS.normal), fetchFont(PDF_FONT_URLS.bold)]);
  return { normal, bold };
}

/** jsPDF's VFS takes the TTF as a binary string (it recognises the `00 01 00 00` TTF header). */
function binaryString(bytes: Uint8Array): string {
  let s = "";
  const CHUNK = 0x8000;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    s += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return s;
}

const PDF_FONT = "Roboto";

function rgb(hex: string): [number, number, number] {
  return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16)];
}

export async function buildPdf(
  doc: DashboardExport,
  fonts: () => Promise<PdfFonts> = loadPdfFonts,
): Promise<Blob> {
  const [{ jsPDF }, f] = await Promise.all([import("jspdf"), fonts()]);
  const pdf = new jsPDF({ unit: "mm", format: "a4", orientation: "portrait" });
  pdf.addFileToVFS("Roboto-Regular.ttf", binaryString(f.normal));
  pdf.addFont("Roboto-Regular.ttf", PDF_FONT, "normal", "Identity-H");
  pdf.addFileToVFS("Roboto-Bold.ttf", binaryString(f.bold));
  pdf.addFont("Roboto-Bold.ttf", PDF_FONT, "bold", "Identity-H");
  pdf.setProperties({ title: `${doc.title} — ${doc.communeName}`, subject: doc.periodLabel });
  pdf.setLanguage("vi");

  const W = pdf.internal.pageSize.getWidth();
  const H = pdf.internal.pageSize.getHeight();
  const M = 14;
  const BOTTOM = H - 16;
  const LH = 0.42; // mm per pt of font size, line height ≈ 1.2
  let y = 0;

  const text = (
    s: string,
    x: number,
    top: number,
    o: { size: number; bold?: boolean; color?: string; width: number; align?: "left" | "right" },
  ): number => {
    pdf.setFont(PDF_FONT, o.bold === true ? "bold" : "normal");
    pdf.setFontSize(o.size);
    pdf.setTextColor(...rgb(o.color ?? NAVY));
    const lines: string[] = pdf.splitTextToSize(s, o.width);
    const lh = o.size * LH;
    lines.forEach((line, i) => {
      pdf.text(line, o.align === "right" ? x + o.width : x, top + lh * (i + 0.8), { align: o.align ?? "left" });
    });
    return lines.length * lh;
  };
  const measure = (s: string, size: number, width: number, bold = false): number => {
    pdf.setFont(PDF_FONT, bold ? "bold" : "normal");
    pdf.setFontSize(size);
    return (pdf.splitTextToSize(s, width) as string[]).length * size * LH;
  };
  const ensure = (h: number) => {
    if (y + h > BOTTOM) {
      pdf.addPage();
      y = M;
    }
  };

  // Cover band — the prototype's `.cover`: navy, commune and period in light blue, brand rule below.
  // The band is as tall as its lines: a long commune name wraps instead of running out of the band.
  const coverLines: { s: string; size: number; bold?: boolean; color: string; gap: number }[] = [
    { s: doc.title, size: 20, bold: true, color: WHITE, gap: 1 },
    { s: doc.communeName, size: 13, color: "BCD4E6", gap: 0 },
    ...(doc.parentAuthority === "" ? [] : [{ s: doc.parentAuthority, size: 10, color: "BCD4E6", gap: 0 }]),
    { s: `${doc.periodLabel} · ${doc.asOfLine}`, size: 10, color: "8FB4D0", gap: 1 },
  ];
  const bandH =
    7 + coverLines.reduce((h, l) => h + measure(l.s, l.size, W - 2 * M, l.bold === true) + l.gap, 0) + 6;
  pdf.setFillColor(...rgb(NAVY));
  pdf.rect(0, 0, W, bandH, "F");
  pdf.setFillColor(...rgb(BRAND));
  pdf.rect(0, bandH, W, 1.6, "F");
  let cy = 7;
  for (const l of coverLines) {
    cy += text(l.s, M, cy, { size: l.size, bold: l.bold, color: l.color, width: W - 2 * M }) + l.gap;
  }
  y = bandH + 7;

  const COL_LABEL = 66;
  const COL_VALUE = 44;
  const COL_REST = W - 2 * M - COL_LABEL - COL_VALUE;
  const PAD = 1.6;

  const heading = (title: string, colour = BRAND) => {
    ensure(18);
    y += 2;
    pdf.setFillColor(...rgb(colour));
    pdf.rect(M, y, 1.4, 6, "F");
    text(title, M + 3.5, y - 0.3, { size: 12.5, bold: true, width: W - 2 * M - 4 });
    y += 9;
  };
  const sentence = (s: string, color: string, size = 9) => {
    const h = measure(s, size, W - 2 * M) + 1;
    ensure(h);
    text(s, M, y, { size, color, width: W - 2 * M });
    y += h;
  };

  for (const b of doc.blocks) {
    heading(b.title);
    for (const fig of b.figures) {
      const side = [fig.comparison, fig.note].filter((s): s is string => s !== undefined && s !== "");
      const value = fig.exact !== undefined && fig.exact !== fig.value ? `${fig.value}\n(${fig.exact})` : fig.value;
      const h =
        Math.max(
          measure(fig.label, 9.5, COL_LABEL - 2 * PAD),
          measure(value, fig.noFigure === true ? 9 : 11, COL_VALUE - 2 * PAD, true),
          side.reduce((sum, s) => sum + measure(s, 8.5, COL_REST - 2 * PAD), 0),
        ) +
        2 * PAD;
      ensure(h);
      text(fig.label, M + PAD, y + PAD, { size: 9.5, color: INK, width: COL_LABEL - 2 * PAD });
      text(value, M + COL_LABEL + PAD, y + PAD, {
        size: fig.noFigure === true ? 9 : 11,
        bold: fig.noFigure !== true,
        color: fig.alert === true ? DANGER : fig.noFigure === true ? MUTED : NAVY,
        width: COL_VALUE - 2 * PAD,
      });
      let sy = y + PAD;
      for (const s of side) sy += text(s, M + COL_LABEL + COL_VALUE + PAD, sy, { size: 8.5, color: MUTED, width: COL_REST - 2 * PAD });
      y += h;
      pdf.setDrawColor(...rgb(LINE));
      pdf.setLineWidth(0.25);
      pdf.line(M, y, W - M, y);
    }
    y += 1.5;
    if (b.pending !== undefined) sentence(`Chưa có số liệu: ${b.pending}`, MUTED);
    for (const e of b.errors) sentence(e, DANGER);
    for (const n of b.notes) sentence(n, MUTED);
    y += 2;
  }

  // Page tables (`/bao-cao`): the first column takes the room a unit name needs, the rest share the
  // remainder; the header row repeats on a new page so a continued table still reads.
  for (const t of doc.tables ?? []) {
    heading(t.title);
    const full = W - 2 * M;
    const first = t.columns.length > 1 ? full * 0.36 : full;
    const rest = t.columns.length > 1 ? (full - first) / (t.columns.length - 1) : 0;
    const widths = t.columns.map((_, i) => (i === 0 ? first : rest));
    const row = (cells: readonly string[], head: boolean) => {
      const h =
        Math.max(...cells.map((c, i) => measure(c, 9, (widths[i] ?? rest) - 2 * PAD, head))) + 2 * PAD;
      if (y + h > BOTTOM) {
        pdf.addPage();
        y = M;
        if (!head) row(t.columns, true);
      }
      if (head) {
        pdf.setFillColor(...rgb(SURFACE));
        pdf.rect(M, y, full, h, "F");
      }
      let x = M;
      cells.forEach((c, i) => {
        const w = widths[i] ?? rest;
        text(c, x + PAD, y + PAD, {
          size: 9,
          bold: head,
          color: head ? NAVY : INK,
          width: w - 2 * PAD,
          align: i === 0 ? "left" : "right",
        });
        x += w;
      });
      y += h;
      pdf.setDrawColor(...rgb(LINE));
      pdf.setLineWidth(0.25);
      pdf.line(M, y, W - M, y);
    };
    ensure(18);
    row(t.columns, true);
    for (const r of t.rows) row(r.map((c) => c.text), false);
    y += 1.5;
    if (t.rows.length === 0) sentence(t.empty, MUTED);
    for (const e of t.errors) sentence(e, DANGER);
    for (const n of t.notes) sentence(n, MUTED);
    y += 2;
  }

  if (doc.urgent !== null) {
    heading(doc.urgent.title, DANGER);
    for (const l of doc.urgent.lines) sentence(l, NAVY, 9.5);
    y += 2;
  }

  ensure(8);
  y += 3;
  pdf.setDrawColor(...rgb(LINE));
  pdf.line(M, y, W - M, y);
  y += 2;
  for (const n of [...doc.footnotes, doc.generatedLine]) sentence(n, MUTED, 8.5);

  // Footer on every page — the prototype's `@page` margin boxes: commune left, "Trang i/n" right.
  const pages = pdf.getNumberOfPages();
  for (let p = 1; p <= pages; p++) {
    pdf.setPage(p);
    text(doc.communeName, M, H - 11, { size: 8, color: "8AA2B8", width: (W - 2 * M) / 2 });
    text(`Trang ${p}/${pages}`, W / 2, H - 11, { size: 8, color: "8AA2B8", width: W / 2 - M, align: "right" });
  }

  return new Blob([pdf.output("arraybuffer")], { type: EXPORT_MIME.pdf });
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PPTX
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const SLIDE_W = 13.333;
const SLIDE_H = 7.5;
const SM = 0.7;
const CARDS_PER_SLIDE = 8;

export async function buildPptx(doc: DashboardExport): Promise<Blob> {
  const { default: PptxGenJS } = await import("pptxgenjs");
  const deck = new PptxGenJS();
  deck.layout = "LAYOUT_WIDE";
  deck.title = `${doc.title} — ${doc.communeName}`;
  deck.theme = { headFontFace: OFFICE_FONT, bodyFontFace: OFFICE_FONT };

  type Slide = ReturnType<typeof deck.addSlide>;
  let page = 0;
  const box = (
    slide: Slide,
    s: string,
    o: { x: number; y: number; w: number; h: number; size: number; bold?: boolean; color?: string; align?: "left" | "right"; valign?: "top" | "middle" },
  ) => {
    slide.addText(s, {
      x: o.x,
      y: o.y,
      w: o.w,
      h: o.h,
      fontFace: OFFICE_FONT,
      fontSize: o.size,
      bold: o.bold === true,
      color: o.color ?? NAVY,
      align: o.align ?? "left",
      valign: o.valign ?? "top",
      margin: 0,
      fit: "shrink",
    });
  };
  const furniture = (title: string, subtitle: string, band = NAVY): Slide => {
    const slide = deck.addSlide();
    page += 1;
    slide.addShape("rect", { x: 0, y: 0, w: SLIDE_W, h: 1.1, fill: { color: band }, line: { color: band } });
    slide.addShape("rect", { x: 0, y: 1.1, w: SLIDE_W, h: 0.06, fill: { color: BRAND }, line: { color: BRAND } });
    box(slide, title, { x: SM, y: 0.24, w: 8.4, h: 0.62, size: 26, bold: true, color: WHITE, valign: "middle" });
    box(slide, subtitle, { x: SLIDE_W - SM - 4.2, y: 0.3, w: 4.2, h: 0.5, size: 12, color: "BCD4E6", align: "right", valign: "middle" });
    slide.addShape("rect", { x: 0, y: SLIDE_H - 0.5, w: SLIDE_W, h: 0.02, fill: { color: LINE }, line: { color: LINE } });
    box(slide, doc.communeName, { x: SM, y: SLIDE_H - 0.44, w: 7, h: 0.3, size: 10, color: "8AA2B8" });
    box(slide, String(page), { x: SLIDE_W - SM - 1, y: SLIDE_H - 0.44, w: 1, h: 0.3, size: 10, color: "8AA2B8", align: "right" });
    return slide;
  };

  // Cover.
  const cover = deck.addSlide();
  page += 1;
  cover.background = { color: NAVY };
  cover.addShape("rect", { x: 0, y: SLIDE_H - 0.9, w: SLIDE_W, h: 0.9, fill: { color: "0B2036" }, line: { color: "0B2036" } });
  cover.addShape("rect", { x: SM, y: 2.25, w: 1.6, h: 0.1, fill: { color: BRAND }, line: { color: BRAND } });
  box(cover, doc.title, { x: SM, y: 2.6, w: 11, h: 1.1, size: 44, bold: true, color: WHITE });
  box(cover, doc.communeName, { x: SM, y: 3.9, w: 11, h: 0.55, size: 24, color: "BCD4E6" });
  let coverY = 4.5;
  if (doc.parentAuthority !== "") {
    box(cover, doc.parentAuthority, { x: SM, y: coverY, w: 11, h: 0.4, size: 16, color: "8FB4D0" });
    coverY += 0.5;
  }
  box(cover, doc.periodLabel, { x: SM, y: coverY, w: 11, h: 0.45, size: 18, color: "8FB4D0" });
  box(cover, doc.asOfLine, { x: SM, y: SLIDE_H - 0.62, w: 7, h: 0.35, size: 11, color: "8FB4D0" });

  // One slide per block with figures; an unbuilt block has no figure to project, so it is named on the
  // notes slide instead of filling a slide with "Chưa có số liệu" (the prototype skipped empty blocks).
  for (const b of doc.blocks.filter((x) => x.pending === undefined)) {
    for (let start = 0; start < b.figures.length; start += CARDS_PER_SLIDE) {
      const shown = b.figures.slice(start, start + CARDS_PER_SLIDE);
      const slide = furniture(start === 0 ? b.title : `${b.title} (tiếp)`, doc.periodLabel);
      const perRow = Math.min(4, shown.length);
      const gap = 0.3;
      const w = (SLIDE_W - 2 * SM - gap * (perRow - 1)) / perRow;
      shown.forEach((fig, i) => {
        const row = Math.floor(i / perRow);
        const col = i % perRow;
        const x = SM + col * (w + gap);
        const top = 1.55 + row * 2.3;
        slide.addShape("roundRect", { x, y: top, w, h: 2.1, fill: { color: SURFACE }, line: { color: LINE, width: 1 }, rectRadius: 0.08 });
        slide.addShape("rect", { x, y: top, w: 0.07, h: 2.1, fill: { color: BRAND }, line: { color: BRAND } });
        const sentenceValue = fig.noFigure === true || fig.value.length > 16;
        box(slide, fig.value, {
          x: x + 0.3,
          y: top + 0.2,
          w: w - 0.5,
          h: 0.8,
          size: sentenceValue ? 13 : fig.value.length <= 9 ? 30 : 24,
          bold: fig.noFigure !== true,
          color: fig.alert === true ? DANGER : fig.noFigure === true ? MUTED : NAVY,
          valign: "middle",
        });
        box(slide, fig.label, { x: x + 0.3, y: top + 1.05, w: w - 0.5, h: 0.4, size: 13, color: INK });
        const side = [fig.comparison, fig.note].filter((s): s is string => s !== undefined && s !== "").join("\n");
        if (side !== "") box(slide, side, { x: x + 0.3, y: top + 1.47, w: w - 0.5, h: 0.58, size: 9, color: MUTED });
      });
      if (start + CARDS_PER_SLIDE >= b.figures.length) {
        const extra = [...b.errors, ...b.notes];
        if (extra.length > 0) {
          box(slide, extra.join("\n"), {
            x: SM,
            y: SLIDE_H - 1.45,
            w: SLIDE_W - 2 * SM,
            h: 0.85,
            size: 10,
            color: b.errors.length > 0 ? DANGER : MUTED,
          });
        }
      }
    }
  }

  // Page tables (`/bao-cao`): one slide per ROWS_PER_SLIDE rows, the header row on each.
  const ROWS_PER_SLIDE = 10;
  for (const t of doc.tables ?? []) {
    const chunks: ExportTable["rows"][] = t.rows.length === 0 ? [[]] : [];
    for (let i = 0; i < t.rows.length; i += ROWS_PER_SLIDE) chunks.push(t.rows.slice(i, i + ROWS_PER_SLIDE));
    chunks.forEach((chunk, n) => {
      const slide = furniture(n === 0 ? t.title : `${t.title} (tiếp)`, doc.periodLabel);
      const w = SLIDE_W - 2 * SM;
      const first = t.columns.length > 1 ? w * 0.36 : w;
      const rest = t.columns.length > 1 ? (w - first) / (t.columns.length - 1) : 0;
      type Rows = Parameters<Slide["addTable"]>[0];
      const head: Rows[number] = t.columns.map((c, i) => ({
        text: c,
        options: { bold: true, color: NAVY, fill: { color: SURFACE }, align: i === 0 ? "left" : "right" },
      }));
      const body: Rows = chunk.map((r) =>
        r.map((c, i) => ({ text: c.text, options: { color: INK, align: i === 0 ? "left" : "right" } })),
      );
      if (chunk.length > 0) {
        slide.addTable([head, ...body], {
          x: SM,
          y: 1.5,
          w,
          colW: t.columns.map((_, i) => (i === 0 ? first : rest)),
          fontFace: OFFICE_FONT,
          fontSize: 13,
          border: { type: "solid", pt: 0.75, color: LINE },
          margin: 0.06,
        });
      } else {
        box(slide, t.empty, { x: SM, y: 1.6, w, h: 0.6, size: 16, color: MUTED });
      }
      if (n === chunks.length - 1) {
        const extra = [...t.errors, ...t.notes];
        if (extra.length > 0) {
          box(slide, extra.join("\n"), {
            x: SM,
            y: SLIDE_H - 1.45,
            w,
            h: 0.85,
            size: 10,
            color: t.errors.length > 0 ? DANGER : MUTED,
          });
        }
      }
    });
  }

  if (doc.urgent !== null) {
    const slide = furniture(doc.urgent.title, doc.periodLabel, NAVY);
    box(slide, doc.urgent.lines.join("\n\n"), { x: SM, y: 1.6, w: SLIDE_W - 2 * SM, h: 4.8, size: 16 });
  }

  const notes = furniture("Ghi chú", doc.periodLabel);
  const pending = doc.blocks.filter((x) => x.pending !== undefined);
  const lines = [
    ...pending.map((x) => `${x.title} — chưa có số liệu: ${x.pending}`),
    ...doc.footnotes,
    doc.generatedLine,
  ];
  box(notes, lines.join("\n\n"), { x: SM, y: 1.6, w: SLIDE_W - 2 * SM, h: 5.2, size: 13, color: INK });

  const out = (await deck.write({ outputType: "arraybuffer" })) as ArrayBuffer;
  return new Blob([out], { type: EXPORT_MIME.pptx });
}

/** One builder per format — what the buttons call. */
export const EXPORT_BUILDERS: Readonly<Record<ExportFormat, (doc: DashboardExport) => Promise<Blob>>> = {
  pdf: (doc) => buildPdf(doc),
  xlsx: buildXlsx,
  pptx: buildPptx,
};
