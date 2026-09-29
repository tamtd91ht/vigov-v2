/**
 * MÀN "GỢI Ý GIẢI PHÁP" — ba câu hỏi, một danh sách gợi ý, BA hành động. Chạy hết trên máy.
 *
 * ⚠ MỖI LỰA CHỌN LÀ MỘT `<button>`, KHÔNG PHẢI `<input type="radio">` HAY `<select>`.
 *
 *   `phase1-collects-nothing.test.ts` cấm `<form|input|textarea|select>`, và từ 22/09/2026 nó miễn
 *   cho ĐÚNG MỘT TỆP — ô ghi chú của màn "Tư vấn và báo giá". MÀN NÀY KHÔNG NẰM TRONG NGOẠI LỆ ẤY,
 *   và cũng không cần: một bộ chọn ba bước dựng bằng nút là hình dạng ĐÚNG cho nó, không phải một
 *   cách lách. Mỗi bước một câu hỏi, mỗi lựa chọn một đích chạm 44px, không bàn phím, không con
 *   trỏ. Đổi chúng thành radio là thêm một điểm thu thập để không được gì.
 *
 * ⚠ KHÔNG MỘT BYTE NÀO RỜI KHỎI MÀN NÀY. Trạng thái sống trong `useState`: không `localStorage`,
 *   không `sessionStorage`, không lời gọi mạng, không `console.log`. Ba câu trả lời ở đây KHÔNG đi
 *   theo người dùng sang màn "Tư vấn và báo giá" — họ chọn lại ở đó, và đó là chủ đích: chuyền
 *   chúng sang nghĩa là gửi lên máy chủ những câu người dùng trả lời cho một màn họ tưởng là chạy
 *   trên máy.
 *
 * ⚠ BA HÀNH ĐỘNG CUỐI, VÀ KHÔNG HÀNH ĐỘNG NÀO HỨA THỨ CHƯA CÓ.
 *
 *   Giai đoạn A có hai: đọc tiếp trên màn Giải pháp, và gọi hotline. Giai đoạn B thêm cái thứ ba —
 *   "Gửi yêu cầu tư vấn" — và nó CHỈ ĐƯỢC PHÉP TỒN TẠI TỪ HÔM NAY, vì hôm nay mới có một tuyến
 *   nhận (`POST /api/v1/requests`). "Đặt lịch" thì vẫn KHÔNG, và mọi cam kết thời gian cũng vậy:
 *   chúng vẫn chưa có đích đến nào.
 */
import { useState } from "react";

import { CONTACT } from "../../content/company-profile";
import { type ThamSoMan } from "../company-intro/dieu-huong";
import { CompassGlyph, HandshakeGlyph, PhoneGlyph, SOLUTION_GLYPHS } from "../company-intro/icons";

import {
  INDUSTRY_QUESTION,
  SCALE_QUESTION,
  TASK_QUESTION,
  suggestedSolutions,
  type Option,
  type IndustryCode,
  type ScaleCode,
  type TaskCode,
  INDUSTRIES,
  labelOf,
  SCALES,
  TASKS,
} from "./mapping";

/** Chữ của màn, một chỗ. Nơi nào cần một câu thì đọc từ đây, không gõ vào JSX. */
export const WORDS = {
  title: "Gợi ý giải pháp",
  intro:
    "Ba câu hỏi ngắn, trả lời ngay trên máy bạn. Không có gì được lưu lại và không có gì được gửi đi.",
  step: (step_no: number, total: number) => `Bước ${step_no} / ${total}`,
  back_button: "‹ Quay lại bước trước",
  restart_button: "Làm lại từ đầu",
  result_title: "Dòng giải pháp nên hỏi trước",
  suggestion_basis:
    "Gợi ý dưới đây chọn theo việc bạn đang cần. Lĩnh vực và quy mô nằm trong câu tóm tắt bên dưới, để bạn đọc lại cho người trực hotline nghe.",
  summary_label: "Tóm tắt câu trả lời của bạn",
  field_label: "Lĩnh vực",
  scale_label: "Quy mô",
  task_label: "Đang cần",
  view_detail_button: "Xem chi tiết giải pháp",
  view_detail_note: "Mở màn Giải pháp",
  // ⚠ HÀNH ĐỘNG THỨ BA, THÊM 22/09/2026 — VÀ NÓ CHỈ ĐƯỢC PHÉP TỒN TẠI TỪ HÔM NAY.
  //
  //   Giai đoạn A cấm mọi nhãn kiểu "gửi yêu cầu" trên màn này, vì KHÔNG CÓ TUYẾN NÀO NHẬN, và
  //   một nút hứa thứ chưa có là thứ người duyệt của Zalo bấm đầu tiên. Giai đoạn B có tuyến
  //   (`POST /api/v1/requests`), nên lệnh cấm ấy được THU HẸP đúng bằng hai từ có đích đến —
  //   "đặt lịch" và mọi cam kết thời gian vẫn bị cấm, vì chúng vẫn chưa có đích. Xem ca kiểm
  //   "KHÔNG hứa thứ chưa có tuyến nào nhận" trong `solution-suggestion.test.tsx`.
  send_request_button: "Gửi yêu cầu tư vấn",
  send_request_note: "Chúng tôi liên hệ lại",
  call_button: "Gọi hotline",
  no_suggestion:
    "Chưa có dòng giải pháp nào khớp với lựa chọn này. Bạn hãy gọi hotline, chúng tôi trả lời trực tiếp.",
} as const;

const TOTAL_STEPS = 3;

type Answers = { industry?: IndustryCode; scale?: ScaleCode; task?: TaskCode };

/**
 * Một bước: câu hỏi và các lựa chọn. THUẦN — không giữ gì, nhận mọi thứ qua tham số.
 *
 * Tách ra để `solution-suggestion.test.tsx` dựng được từng bước bằng `react-dom/server`, nơi không có
 * DOM để bấm. Một bộ chọn chỉ kiểm được khi dựng thẳng được từng trạng thái của nó.
 */
export function ChoiceStep<T extends string>({
  step_no,
  question,
  options,
  onSelect,
}: {
  step_no: number;
  question: string;
  options: readonly Option<T>[];
  onSelect: (code: T) => void;
}) {
  return (
    <>
      {/* SỐ BƯỚC NÓI BẰNG CHỮ, KHÔNG BẰNG BA CHẤM TRÒN ĐỔI MÀU. Một chuỗi chấm chỉ khác nhau ở màu
          là thông tin truyền bằng riêng màu sắc — thứ người mù màu và người đọc ngoài nắng mất
          trắng (README §Non-negotiables #6). */}
      <p className="screen-lead">{WORDS.step(step_no, TOTAL_STEPS)}</p>
      <h2 className="section-title">{question}</h2>
      <ul className="gygp-ds">
        {options.map((item) => (
          <li key={item.ma}>
            {/* `.action action--mo` — đúng cái nút khối mà màn chủ và màn Quản lý quyền dùng: cao
                tối thiểu `--tap-min`, nền đã đo, viền và bóng đã có. Không dựng một lớp nút mới
                cho màn này: một lớp CSS không quy định gì là chỗ người sau gắn một màu chưa ai đo. */}
            <button
              type="button"
              className="action action--mo gygp-ds__nut"
              onClick={() => onSelect(item.ma)}
            >
              <span className="action__value">{item.nhan}</span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * Kết quả, THUẦN. Nhận cả ba câu trả lời đã có và hai hàm hành động.
 *
 * `task` là tham số BẮT BUỘC chứ không tuỳ chọn: không có nó thì không có gì để gợi ý, và một
 * nhánh "chưa chọn việc" ở đây sẽ là một màn hình trống không ai giải thích được. Nơi gọi chỉ dựng
 * khối này khi đã đủ ba câu trả lời — kiểu ép điều đó thay vì một chú thích.
 */
export function SuggestionResult({
  answers,
  task,
  onViewSolution,
  onSendRequest,
}: {
  answers: Answers;
  task: TaskCode;
  onViewSolution: () => void;
  /**
   * Đường sang màn "Tư vấn và báo giá" (giai đoạn B). TUỲ CHỌN, và khác biệt ấy có lý do: bộ test
   * dựng khối này một mình, và bắt nó khai một hàm nó không bấm tới là mời người sau truyền vào
   * đó một thứ khác. Vỏ màn luôn truyền.
   */
  onSendRequest?: () => void;
}) {
  const suggestion = suggestedSolutions(task);

  return (
    <>
      <h2 className="section-title">{WORDS.result_title}</h2>
      <p className="screen-lead">{WORDS.suggestion_basis}</p>

      {suggestion.length === 0 ? (
        <p className="tn__loi">{WORDS.no_suggestion}</p>
      ) : (
        <ul className="gygp-ds">
          {suggestion.map((solution) => {
            const Glyph = SOLUTION_GLYPHS[solution.id];
            return (
              <li className="card" key={solution.id}>
                <span className="tile tile--soft" aria-hidden="true">
                  <Glyph className="tile__glyph" />
                </span>
                {/* Tên sản phẩm chỉ hiện khi nguồn có nó — `product` là tuỳ chọn trong `SOLUTIONS`,
                    và suy ra một cái tên là gán một sản phẩm cho một pháp nhân bằng phỏng đoán. */}
                {solution.product ? (
                  <h3 className="card__title">{solution.product}</h3>
                ) : null}
                <p className="card__body">{solution.headline}</p>
                {solution.note ? <p className="card__body">{solution.note}</p> : null}
              </li>
            );
          })}
        </ul>
      )}

      {/* CÂU TÓM TẮT — chỗ DUY NHẤT hai câu trả lời đầu được đọc lại, và là lý do chúng được hỏi.
          Người dùng đọc nguyên câu này cho người trực hotline nghe; không có nó thì hai bước đầu
          là hai bước không làm gì. */}
      <section className="card">
        <h3 className="card__title">{WORDS.summary_label}</h3>
        <ul className="the-tt__danh-sach">
          {(
            [
              [WORDS.field_label, labelOf(INDUSTRIES, answers.industry)],
              [WORDS.scale_label, labelOf(SCALES, answers.scale)],
              [WORDS.task_label, labelOf(TASKS, answers.task)],
            ] as const
          )
            .filter(([, value]) => value !== null)
            .map(([label, value]) => (
              <li className="the-tt__dong" key={label}>
                <span className="the-tt__nhan">{label}</span>
                <span className="the-tt__gia-tri">{value}</span>
              </li>
            ))}
        </ul>
      </section>

      <button type="button" className="action action--mo" onClick={onViewSolution}>
        <span className="tile tile--soft" aria-hidden="true">
          <CompassGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">{WORDS.view_detail_note}</span>
          <span className="action__value">{WORDS.view_detail_button}</span>
        </span>
      </button>

      {onSendRequest !== undefined && (
        <button type="button" className="action action--mo" onClick={onSendRequest}>
          <span className="tile tile--soft" aria-hidden="true">
            <HandshakeGlyph className="tile__glyph" />
          </span>
          <span>
            <span className="action__label">{WORDS.send_request_note}</span>
            <span className="action__value">{WORDS.send_request_button}</span>
          </span>
        </button>
      )}

      {/* `tel:` — không phải một lời gọi nền tảng. `openPhone` đang được khai cho việc gọi một số
          vừa QUÉT được ở màn Danh thiếp; mượn nó ở đây là làm sai lệch chính bảng khai mà màn Quản
          lý quyền đọc ra. Một neo `tel:` chạy được cả ngoài Zalo và không cần khai thêm quyền nào. */}
      <a className="action action--primary" href={`tel:${CONTACT.hotlineDialable}`}>
        <span className="tile" aria-hidden="true">
          <PhoneGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">{WORDS.call_button}</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </a>
    </>
  );
}

export function SolutionSuggestionScreen({ onDi }: ThamSoMan) {
  /**
   * Vỏ app luôn truyền `onDi`. Giá trị lui này chỉ phục vụ những chỗ dựng màn một mình — bộ test
   * dựng bằng `renderToStaticMarkup` — và không bao giờ chạy trong app thật.
   */
  const go = onDi ?? (() => {});

  const [answers, setAnswers] = useState<Answers>({});

  /**
   * BƯỚC HIỆN TẠI SUY RA TỪ CÂU TRẢ LỜI, KHÔNG PHẢI MỘT BIẾN ĐẾM RIÊNG.
   *
   * Hai nguồn cho một sự thật thì một trong hai sẽ lệch, và lần lệch ấy hiện ra thành "bấm quay
   * lại một lần, màn hình vẫn ở bước cũ nhưng câu trả lời đã mất". Ở đây không có gì để lệch.
   */
  function goBack() {
    setAnswers((previous) => {
      if (previous.task !== undefined) return { industry: previous.industry, scale: previous.scale };
      if (previous.scale !== undefined) return { industry: previous.industry };
      return {};
    });
  }

  const started = answers.industry !== undefined;

  return (
    <>
      <section className="banner">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <CompassGlyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">{WORDS.title}</h1>
        </div>
      </section>

      <p className="screen-lead">{WORDS.intro}</p>

      {answers.industry === undefined ? (
        <ChoiceStep
          step_no={1}
          question={INDUSTRY_QUESTION}
          options={INDUSTRIES}
          onSelect={(code) => setAnswers((previous) => ({ ...previous, industry: code }))}
        />
      ) : answers.scale === undefined ? (
        <ChoiceStep
          step_no={2}
          question={SCALE_QUESTION}
          options={SCALES}
          onSelect={(code) => setAnswers((previous) => ({ ...previous, scale: code }))}
        />
      ) : answers.task === undefined ? (
        <ChoiceStep
          step_no={3}
          question={TASK_QUESTION}
          options={TASKS}
          onSelect={(code) => setAnswers((previous) => ({ ...previous, task: code }))}
        />
      ) : (
        <SuggestionResult
          answers={answers}
          task={answers.task}
          onViewSolution={() => go({ man: "solutions" })}
          onSendRequest={() => go({ man: "tu-van" })}
        />
      )}

      {/* HAI ĐƯỜNG LUI, VÀ CHÚNG CHỈ HIỆN KHI CÓ THỨ ĐỂ LUI VỀ. Ở bước 1 thì "quay lại bước trước"
          và "làm lại từ đầu" đều là hai nút không làm gì — và một nút bấm không phản hồi là cách
          nhanh nhất để người lớn tuổi kết luận rằng app hỏng. */}
      {started && (
        <div className="tn__doi-nut">
          <button type="button" className="tn-hanh-dong tn-hanh-dong--rong" onClick={goBack}>
            {WORDS.back_button}
          </button>
          <button
            type="button"
            className="tn-hanh-dong tn-hanh-dong--rong"
            onClick={() => setAnswers({})}
          >
            {WORDS.restart_button}
          </button>
        </div>
      )}
    </>
  );
}
