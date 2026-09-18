/**
 * Bản rỗng của bảng chẩn đoán — biến thể `goc`. Xem `features/kham-pha/index.rong.ts` để biết vì
 * sao mọi `import` ở đây là `import type`.
 */
import type { ComponentProps, ComponentType } from "react";

import type * as DayDu from "./index";

export const LaunchParamsPanel: ComponentType<
  ComponentProps<typeof DayDu.LaunchParamsPanel>
> = () => null;
