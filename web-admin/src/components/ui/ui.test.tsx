import { Pencil, Send } from "lucide-react";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { cn } from "@/lib/cn";

import { Badge } from "./badge";
import { Button } from "./button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "./card";
import { EmptyState } from "./empty-state";
import { Field, Toolbar } from "./field";
import { IconButton } from "./icon-button";
import { Notice } from "./notice";
import { PageHeader } from "./page-header";
import { Segmented } from "./segmented";
import { NO_DATA_CAPTION, StatCard } from "./stat-card";
import { Tab, TabList } from "./tabs";

/**
 * The ui components are PRESENTATIONAL. What these tests hold is the contract a screen card relies
 * on when it swaps its markup for them: the native element is still there, the legacy hook class is
 * still there, every prop the screen passed still reaches the element, and a segmented control
 * emits exactly the values it was given. Looks are not tested — they change; this contract must not.
 */

/** Depth-first walk of an UNRENDERED element tree (components with no hooks only). */
function findAll(node: ReactNode, pick: (el: ReactElement<Record<string, unknown>>) => boolean): ReactElement<Record<string, unknown>>[] {
  const found: ReactElement<Record<string, unknown>>[] = [];
  const walk = (n: ReactNode) => {
    if (Array.isArray(n)) return n.forEach(walk);
    if (!isValidElement<Record<string, unknown>>(n)) return;
    if (pick(n)) found.push(n);
    walk(n.props.children as ReactNode);
  };
  walk(node);
  return found;
}

describe("cn", () => {
  it("lets the later utility win and keeps legacy class names", () => {
    expect(cn("nut-phu px-4", "px-2")).toBe("nut-phu px-2");
    expect(cn("rounded-control", "rounded-lg")).toBe("rounded-lg");
    expect(cn("text-sm text-ink-700")).toBe("text-sm text-ink-700");
  });
});

describe("Button", () => {
  it("renders a native <button> carrying the legacy hook class of each variant", () => {
    expect(renderToStaticMarkup(<Button variant="primary">Lưu</Button>)).toMatch(/^<button class="nut-chinh /);
    expect(renderToStaticMarkup(<Button variant="secondary">Huỷ</Button>)).toMatch(/^<button class="nut-phu /);
    expect(renderToStaticMarkup(<Button variant="danger">Xoá</Button>)).toMatch(/^<button class="nut-phu nut-xoa /);
    expect(renderToStaticMarkup(<Button variant="ghost">Bỏ</Button>)).toMatch(/^<button class="nut-phu /);
  });

  it("sets NO default type — a button inside a form keeps the browser's submit default", () => {
    expect(renderToStaticMarkup(<Button>Gửi</Button>)).not.toContain("type=");
    expect(renderToStaticMarkup(<Button type="submit">Gửi</Button>)).toContain('type="submit"');
  });

  it("forwards every native prop and the handler untouched", () => {
    const onClick = vi.fn();
    const el = Button({ onClick, disabled: true, name: "hanh-dong", value: "duyet", "aria-busy": true, children: "Duyệt" });
    expect(el.type).toBe("button");
    const props = el.props as Record<string, unknown>;
    expect(props.onClick).toBe(onClick);
    expect(props.disabled).toBe(true);
    expect(props.name).toBe("hanh-dong");
    expect(props.value).toBe("duyet");
    expect(props["aria-busy"]).toBe(true);
  });

  it("draws the icon before the label", () => {
    const html = renderToStaticMarkup(
      <Button variant="primary" icon={<Send aria-hidden="true" />}>
        Công khai nhiều người
      </Button>,
    );
    expect(html.indexOf("<svg")).toBeGreaterThan(-1);
    expect(html.indexOf("<svg")).toBeLessThan(html.indexOf("Công khai nhiều người"));
  });
});

describe("IconButton", () => {
  it("puts the same label on aria-label AND title", () => {
    const html = renderToStaticMarkup(
      <IconButton label="Sửa" onClick={() => {}} type="button">
        <Pencil aria-hidden="true" />
      </IconButton>,
    );
    expect(html).toContain('aria-label="Sửa"');
    expect(html).toContain('title="Sửa"');
    expect(html).toMatch(/^<button /);
  });

  it("cannot be written without a label (type-level)", () => {
    // @ts-expect-error — `label` is required: an icon-only button with no accessible name.
    const missing = <IconButton>{null}</IconButton>;
    // @ts-expect-error — `aria-label` alone is refused; it would let `title` drift from it.
    const ariaOnly = <IconButton aria-label="Sửa" label="Sửa">{null}</IconButton>;
    expect(missing).toBeTruthy();
    expect(ariaOnly).toBeTruthy();
  });
});

describe("Badge", () => {
  it("always carries an icon next to the word — never colour alone", () => {
    for (const tone of ["success", "neutral", "warning", "danger", "info"] as const) {
      const html = renderToStaticMarkup(<Badge tone={tone}>Đang hiện</Badge>);
      expect(html).toContain("<svg");
      expect(html).toContain('aria-hidden="true"');
      expect(html).toContain("Đang hiện");
    }
  });
});

describe("Segmented", () => {
  const options = [
    { value: "", label: "Tất cả" },
    { value: "hien", label: "Đang hiện" },
    { value: "an", label: "Chưa hiện" },
  ];

  it("radio mode: native radios in a fieldset with a legend, one checked", () => {
    const html = renderToStaticMarkup(
      <Segmented legend="Trạng thái hiển thị" name="loc-hien" value="hien" options={options} onChange={() => {}} />,
    );
    expect(html).toContain("<fieldset");
    expect(html).toContain('<legend class="an-thi-giac">Trạng thái hiển thị</legend>');
    expect(html.match(/type="radio"/g)).toHaveLength(3);
    expect(html.match(/checked=""/g)).toHaveLength(1);
    expect(html).toMatch(/<input (?=[^>]*value="hien")(?=[^>]*checked="")[^>]*>/);
  });

  it("radio mode: emits EXACTLY the option's value through the given onChange", () => {
    const onChange = vi.fn();
    const tree = Segmented({ legend: "Trạng thái", name: "loc", value: "", options, onChange });
    const radios = findAll(tree, (el) => el.type === "input");
    expect(radios.map((r) => r.props.value)).toEqual(["", "hien", "an"]);
    (radios[2]!.props.onChange as (e: { target: { value: string } }) => void)({ target: { value: "an" } });
    (radios[0]!.props.onChange as (e: { target: { value: string } }) => void)({ target: { value: "" } });
    expect(onChange.mock.calls).toEqual([["an"], [""]]);
  });

  it("buttons mode: native toggle buttons with aria-pressed, same values", () => {
    const onChange = vi.fn();
    const tree = Segmented({ legend: "Phạm vi", name: "pham-vi", value: "cho-toi", mode: "buttons", onChange, options: [
      { value: "cho-toi", label: "Chờ tôi duyệt" },
      { value: "toan-xa", label: "Toàn xã" },
    ] });
    const buttons = findAll(tree, (el) => el.type === "button");
    expect(buttons.map((b) => [b.props.type, b.props["aria-pressed"]])).toEqual([
      ["button", true],
      ["button", false],
    ]);
    (buttons[1]!.props.onClick as () => void)();
    expect(onChange).toHaveBeenCalledWith("toan-xa");
    expect(renderToStaticMarkup(tree)).toContain('role="group" aria-label="Phạm vi"');
  });
});

describe("Field / Toolbar", () => {
  it("wraps the screen's own control: id, name and handler reach the DOM unchanged", () => {
    const html = renderToStaticMarkup(
      <Toolbar>
        <Field label="Tìm cán bộ" htmlFor="tim-can-bo" grow="search">
          <input id="tim-can-bo" name="q" type="search" />
        </Field>
        <Field label="Khối / đơn vị" htmlFor="loc-khoi" kind="select" hideLabel>
          <select id="loc-khoi" name="khoi">
            <option value="">Tất cả</option>
          </select>
        </Field>
      </Toolbar>,
    );
    expect(html).toContain('<label for="tim-can-bo"');
    const input = /<input [^>]*\/>/.exec(html)?.[0] ?? "";
    for (const a of ['id="tim-can-bo"', 'name="q"', 'type="search"']) expect(input).toContain(a);
    expect(html).toMatch(/<select (?=[^>]*id="loc-khoi")(?=[^>]*name="khoi")[^>]*>/);
    // A hidden label is still a <label>: the control keeps its accessible name.
    expect(html).toContain('<label for="loc-khoi" class="an-thi-giac">Khối / đơn vị</label>');
  });
});

describe("Tabs", () => {
  it("Tab is a native tab button and passes the screen's id / aria-controls / handler", () => {
    const onClick = vi.fn();
    const html = renderToStaticMarkup(
      <TabList aria-label="Phạm vi nhiệm vụ">
        <Tab selected id="tab-toan-xa" aria-controls="panel" onClick={onClick}>
          Toàn xã
        </Tab>
        <Tab selected={false} id="tab-cua-toi">
          Giao cho tôi
        </Tab>
      </TabList>,
    );
    expect(html).toMatch(/<div (?=[^>]*role="tablist")(?=[^>]*aria-label="Phạm vi nhiệm vụ")[^>]*>/);
    expect(html).toContain('id="tab-toan-xa"');
    expect(html).toContain('aria-controls="panel"');
    expect(html.match(/role="tab" aria-selected="true"/g)).toHaveLength(1);
    expect(html.match(/role="tab" aria-selected="false"/g)).toHaveLength(1);
    expect(Tab({ selected: true, onClick, children: "x" }).props.onClick).toBe(onClick);
  });
});

describe("StatCard", () => {
  it("draws the figure, and '—' + 'Chưa có dữ liệu' when there is none", () => {
    expect(renderToStaticMarkup(<StatCard icon={Pencil} label="Đang làm" value={12} />)).toContain(">12</p>");
    const empty = renderToStaticMarkup(<StatCard icon={Pencil} label="Đang làm" value={null} />);
    expect(empty).toContain(">—</p>");
    expect(empty).toContain(NO_DATA_CAPTION);
  });

  it("is red only when the caller says so", () => {
    expect(renderToStaticMarkup(<StatCard icon={Pencil} label="Quá hạn" value={3} />)).not.toContain("text-danger-600");
    expect(renderToStaticMarkup(<StatCard icon={Pencil} label="Quá hạn" value={3} alert />)).toContain("text-danger-600");
  });
});

describe("EmptyState / Notice / Card / PageHeader", () => {
  it("EmptyState shows the screen's sentence verbatim with a decorative icon", () => {
    const html = renderToStaticMarkup(<EmptyState title="Chưa có nhiệm vụ nào." role="status" />);
    expect(html).toContain('role="status"');
    expect(html).toContain("Chưa có nhiệm vụ nào.");
    expect(html).toContain('aria-hidden="true"');
  });

  it("Notice passes role through and has no red tone", () => {
    const html = renderToStaticMarkup(
      <Notice tone="legal" role="note" title="Dữ liệu cá nhân – NĐ 13/2023/NĐ-CP.">
        Không sao chép ra ngoài cơ quan.
      </Notice>,
    );
    expect(html).toContain('role="note"');
    expect(html).toContain("Không sao chép ra ngoài cơ quan.");
    expect(html).not.toContain("danger");
  });

  it("Card keeps the element it is given — a form stays a form", () => {
    const onSubmit = vi.fn();
    const html = renderToStaticMarkup(
      <Card as="form" aria-label="Lọc">
        <CardHeader>
          <CardTitle>Danh sách</CardTitle>
        </CardHeader>
        <CardContent>nội dung</CardContent>
        <CardFooter>chân</CardFooter>
      </Card>,
    );
    expect(html).toMatch(/^<form /);
    expect(html).toContain("<h2");
    expect(Card({ as: "form", onSubmit, children: null }).props.onSubmit).toBe(onSubmit);
  });

  it("PageHeader renders one <h1> with the title and the actions slot", () => {
    const html = renderToStaticMarkup(
      <PageHeader icon={Pencil} title="Danh bạ cán bộ" titleId="tieu-de" subtitle="Một dòng" actions={<Button>Hành động</Button>} />,
    );
    expect(html.match(/<h1/g)).toHaveLength(1);
    expect(html).toContain('<h1 id="tieu-de"');
    expect(html).toContain("Danh bạ cán bộ");
    expect(html).toContain("Hành động");
  });
});
