import { AlarmClock, Hourglass } from "lucide-react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TodoList } from "./todo-list";

describe("TodoList", () => {
  it("a row with href is one link carrying the caller's name and the number's id", () => {
    const html = renderToStaticMarkup(
      <TodoList
        items={[
          {
            key: "a",
            icon: AlarmClock,
            tone: "danger",
            title: "Nhiệm vụ quá hạn",
            value: <span id="a-value">3</span>,
            alert: true,
            href: "/nhiem-vu?metric=overdue",
            linkLabel: "Xem danh sách đằng sau: Quá hạn",
            describedBy: "a-value",
          },
        ]}
      />,
    );
    expect(html).toContain('href="/nhiem-vu?metric=overdue"');
    expect(html).toContain('aria-label="Xem danh sách đằng sau: Quá hạn"');
    expect(html).toContain('aria-describedby="a-value"');
    expect(html).toContain("text-danger-600");
  });

  it("a row without href is not a link and draws no chevron", () => {
    const html = renderToStaticMarkup(
      <TodoList items={[{ key: "b", icon: Hourglass, title: "Văn bản", value: "5" }]} />,
    );
    expect(html).not.toContain("<a");
    expect(html).not.toContain("lucide-chevron-right");
    expect(html).not.toContain("text-danger-600");
  });
});
