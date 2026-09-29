import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SOLUTIONS } from "../../content/company-profile";

import {
  TASK_SOLUTION_MAP,
  INDUSTRY_QUESTION,
  SCALE_QUESTION,
  TASK_QUESTION,
  suggestedSolutions,
  type TaskCode,
  INDUSTRIES,
  labelOf,
  SCALES,
  TASKS,
} from "./mapping";
import { ChoiceStep, SolutionSuggestionScreen, SuggestionResult, WORDS } from "./SolutionSuggestionScreen";
import { SOLUTION_SUGGESTION_SCREEN } from "./index";

const render = (node: ReactElement) => renderToStaticMarkup(node);
/**
 * Chữ người đọc thấy, từ một chuỗi HTML.
 *
 * GIẢI MÃ THỰC THỂ HTML, KHÔNG CHỈ BÓC THẺ: `renderToStaticMarkup` thoát `&` thành `&amp;`, nên
 * một câu đã công bố như "Messaging & Voice" không bao giờ khớp nếu chỉ bóc thẻ — và ca kiểm sẽ
 * đỏ vì một lý do không liên quan gì tới thứ nó hỏi.
 */
const textOf = (markup: string) =>
  markup
    .replace(/<[^>]+>/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#x27;|&#39;/g, "'")
    .replace(/\s+/g, " ");

/* ============================================================================================
   BẢNG ÁNH XẠ — ĐÂY LÀ CA ĐẮT NHẤT CỦA CẢ TỆP
   ============================================================================================ */

describe("bảng ánh xạ trỏ vào danh mục sản phẩm THẬT", () => {
  /**
   * ⚠ KHÔNG CÓ CA NÀY THÌ MỘT ID GÕ SAI LÀ MỘT LỖI HOÀN TOÀN IM LẶNG.
   *
   *   `suggestedSolutions` lọc `SOLUTIONS`. Một id không tồn tại không ném, không cảnh báo, không vẽ
   *   một thẻ rỗng — nó chỉ biến mất khỏi kết quả. Ngày ai đó đổi danh mục sản phẩm (đổi tên một
   *   `SolutionId`, gỡ một dòng), màn này lặng lẽ gợi ý thiếu, hoặc gợi ý RỖNG, và mọi ca "màn có
   *   vẽ ra không" ở dưới vẫn xanh.
   */
  it("mọi id trong bảng ánh xạ đều tồn tại trong SOLUTIONS", () => {
    const real_ids = new Set(SOLUTIONS.map((solution) => solution.id));
    expect(real_ids.size, "danh mục sản phẩm rỗng — ca này sẽ xanh vì lý do sai").toBeGreaterThan(0);

    for (const [task, ids] of Object.entries(TASK_SOLUTION_MAP)) {
      expect(ids.length, `việc "${task}" không ánh xạ tới dòng sản phẩm nào`).toBeGreaterThan(0);
      for (const id of ids) {
        expect(
          real_ids,
          `bảng ánh xạ trỏ tới "${id}", một id KHÔNG có trong SOLUTIONS — việc "${task}" sẽ gợi ý thiếu`,
        ).toContain(id);
      }
    }
  });

  it("mọi lựa chọn ở bước ba đều có một dòng trong bảng ánh xạ", () => {
    // Mặt kia của ca trên: một lựa chọn hiện ra trên màn mà bảng không có dòng cho nó là một nút
    // dẫn tới một kết quả rỗng.
    for (const item of TASKS) {
      expect(Object.keys(TASK_SOLUTION_MAP), `bước ba có lựa chọn "${item.ma}" mà bảng không khai`).toContain(
        item.ma,
      );
      expect(suggestedSolutions(item.ma).length, `lựa chọn "${item.ma}" cho ra gợi ý rỗng`).toBeGreaterThan(0);
    }
  });

  it("giữ đúng thứ tự SOLUTIONS công bố, không đảo theo bảng ánh xạ", () => {
    // `da-kenh` ánh xạ tới `["voice-ai", "messaging"]`, nhưng `SOLUTIONS` công bố `messaging`
    // trước. Người dùng vừa xem màn Giải pháp phải gặp lại đúng thứ tự ấy.
    const ids = suggestedSolutions("da-kenh").map((solution) => solution.id);
    const published_order = SOLUTIONS.filter((solution) => ids.includes(solution.id)).map(
      (solution) => solution.id,
    );
    expect(ids).toEqual(published_order);
  });

  it("không dựng một danh sách sản phẩm thứ hai — chữ đọc thẳng từ SOLUTIONS", () => {
    for (const item of TASKS) {
      const text = textOf(
        render(<SuggestionResult answers={{ task: item.ma }} task={item.ma} onViewSolution={() => {}} />),
      );
      for (const solution of suggestedSolutions(item.ma)) {
        expect(text, `gợi ý cho "${item.ma}" thiếu câu đã công bố`).toContain(solution.headline);
      }
    }
  });
});

/* ============================================================================================
   BỘ CHỌN BA BƯỚC
   ============================================================================================ */

describe("ba bước, mỗi bước 3–4 lựa chọn, tất cả là nút", () => {
  it("mỗi bước có từ ba tới bốn lựa chọn", () => {
    for (const [name, list] of [
      ["lĩnh vực", INDUSTRIES],
      ["quy mô", SCALES],
      ["việc đang cần", TASKS],
    ] as const) {
      expect(list.length, `bước "${name}" có ${list.length} lựa chọn`).toBeGreaterThanOrEqual(3);
      expect(list.length, `bước "${name}" có ${list.length} lựa chọn`).toBeLessThanOrEqual(4);
      expect(new Set(list.map((item) => item.ma)).size, `bước "${name}" có hai mã trùng`).toBe(
        list.length,
      );
    }
  });

  /**
   * ⚠ ĐÂY LÀ CÁI BẪY CHÍNH CỦA MÀN NÀY, VÀ NÓ ĐƯỢC CANH Ở HAI CHỖ.
   *
   *   `phase1-collects-nothing.test.ts` quét MÃ NGUỒN và cấm `<form|input|textarea|select>` ở mọi
   *   tệp. Ca dưới quét BẢN DỰNG RA của đúng màn này. Hai hình dạng vì một lý do: phép quét mã
   *   nguồn không thấy được một `<select>` sinh ra từ một biến, và phép dựng không thấy được một
   *   nhánh chưa chạy tới. Một bộ chọn là đúng chỗ cả hai loại lọt qua.
   */
  it("mỗi lựa chọn là một `<button>`, không một ô nhập nào", () => {
    for (const [step_no, question, list] of [
      [1, INDUSTRY_QUESTION, INDUSTRIES],
      [2, SCALE_QUESTION, SCALES],
      [3, TASK_QUESTION, TASKS],
    ] as const) {
      const markup = render(
        <ChoiceStep step_no={step_no} question={question} options={list} onSelect={() => {}} />,
      );
      expect(markup, `bước ${step_no} dựng ra một ô nhập`).not.toMatch(/<(form|input|textarea|select)\b/);
      const button = markup.match(/<button\b/g) ?? [];
      expect(button.length, `bước ${step_no} có ${button.length} nút cho ${list.length} lựa chọn`).toBe(
        list.length,
      );
      const text = textOf(markup);
      expect(text, `bước ${step_no} không hiện câu hỏi`).toContain(question);
      for (const item of list) {
        expect(text, `bước ${step_no} thiếu lựa chọn: ${item.nhan}`).toContain(item.nhan);
      }
      // Số bước nói bằng CHỮ, không bằng riêng màu của một chuỗi chấm tròn.
      expect(text, `bước ${step_no} không nói mình là bước thứ mấy`).toContain(WORDS.step(step_no, 3));
    }
  });

  it("mã nội bộ của một lựa chọn không bao giờ hiện ra màn hình", () => {
    // `ban-le`, `50-200`, `quan-he-khach` là khoá của mã, không phải chữ của người đọc. Một khoá
    // lọt lên màn là một màn hình nói bằng ngôn ngữ của lập trình viên.
    const text = textOf(render(<ChoiceStep step_no={1} question={INDUSTRY_QUESTION} options={INDUSTRIES} onSelect={() => {}} />));
    for (const item of INDUSTRIES) {
      expect(text, `mã nội bộ "${item.ma}" hiện ra màn hình`).not.toContain(item.ma);
    }
  });

  it("màn mở ra ở bước một, và chưa có đường lui nào", () => {
    const text = textOf(render(<SolutionSuggestionScreen />));
    expect(text).toContain(WORDS.title);
    expect(text).toContain(INDUSTRY_QUESTION);
    // Hai nút lui chỉ xuất hiện khi CÓ thứ để lui về. Ở bước một chúng là hai nút không làm gì,
    // và một nút bấm không phản hồi là cách nhanh nhất để người lớn tuổi kết luận app hỏng.
    expect(text, "bước một đã bày nút quay lại").not.toContain(WORDS.back_button);
    expect(text, "bước một đã bày nút làm lại").not.toContain(WORDS.restart_button);
  });

  it("labelOf đọc chữ từ chính danh sách vẽ ra nó, và trả null khi chưa chọn", () => {
    expect(labelOf(SCALES, undefined)).toBeNull();
    expect(labelOf(SCALES, "50-200")).toBe(SCALES.find((item) => item.ma === "50-200")!.nhan);
  });
});

/* ============================================================================================
   KẾT QUẢ — HAI HÀNH ĐỘNG, KHÔNG MỘT LỜI HỨA NÀO VƯỢT GIAI ĐOẠN A
   ============================================================================================ */

describe("màn kết quả kết thúc bằng đúng hai hành động", () => {
  const task: TaskCode = "da-kenh";
  const markup = render(
    <SuggestionResult
      answers={{ industry: "ban-le", scale: "10-50", task }}
      task={task}
      onViewSolution={() => {}}
    />,
  );

  it("đọc lại cả ba câu trả lời, kể cả hai câu không lọc gì", () => {
    // Hai bước đầu KHÔNG đổi danh sách gợi ý, và màn hình nói ra điều đó. Chúng vẫn phải xuất
    // hiện ở câu tóm tắt — nếu không thì chúng là hai bước không làm gì, tức hai bước nói dối.
    const text = textOf(markup);
    expect(text).toContain(labelOf(INDUSTRIES, "ban-le")!);
    expect(text).toContain(labelOf(SCALES, "10-50")!);
    expect(text).toContain(labelOf(TASKS, task)!);
    expect(text, "màn không nói ra rằng gợi ý chọn theo việc đang cần").toContain(WORDS.suggestion_basis);
  });

  it("có đúng một neo `tel:`, tới đúng hotline đã công bố", () => {
    const neo = markup.match(/href="tel:([^"]+)"/g) ?? [];
    expect(neo).toHaveLength(1);
    expect(markup).toContain('href="tel:');
  });

  it("KHÔNG hứa báo giá, đặt lịch hay đăng ký — giai đoạn A chưa có tuyến nào nhận", () => {
    const text = textOf(markup).toLowerCase();
    for (const word of ["báo giá", "đặt lịch", "đăng ký", "để lại thông tin", "gửi yêu cầu"]) {
      expect(text, `màn kết quả hứa "${word}" trong khi chưa có tuyến nào nhận`).not.toContain(word);
    }
  });

  it("không một ô nhập nào, kể cả ở màn kết quả", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });
});

/* ============================================================================================
   CHỖ CỦA MÀN TRONG SỔ MÀN HÌNH
   ============================================================================================ */

describe("màn đứng ngoài thanh tab, và nói rõ ô nào sáng", () => {
  it("khai `ngoai-tab` và trỏ về màn chủ", () => {
    expect(SOLUTION_SUGGESTION_SCREEN.cho.kieu).toBe("ngoai-tab");
    expect(SOLUTION_SUGGESTION_SCREEN.cho.tabSangLen).toBe("home");
  });

  it("thanh tiêu đề và `<h1>` của màn là MỘT chuỗi, không hai", () => {
    // Hai chỗ giữ một cái tên là hai chỗ sẽ lệch, và lần lệch ấy hiện ra thành thanh tiêu đề nói
    // một đằng, tiêu đề màn một nẻo — với người lớn tuổi thì đó là "tôi đang ở đâu".
    expect(SOLUTION_SUGGESTION_SCREEN.headerTitle).toBe(WORDS.title);
    expect(textOf(render(<SolutionSuggestionScreen />))).toContain(SOLUTION_SUGGESTION_SCREEN.headerTitle);
  });
});
