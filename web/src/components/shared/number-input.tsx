// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as React from "react";

import {cn} from "@/lib/utils";
import {Input} from "@/components/ui/input";

export interface NumberInputProps
  extends Omit<React.ComponentProps<"input">, "value" | "onChange" | "type"> {
  value: number | undefined | null;
  onChange: (value: number) => void;
  /** Unit rendered against the right edge of the field, e.g. "seconds". */
  addonAfter?: React.ReactNode;
}

/**
 * Reports numbers rather than strings and clamps to `min`/`max` on blur, so a
 * page never has to re-validate what it stores.
 */
const NumberInput = React.forwardRef<HTMLInputElement, NumberInputProps>(function NumberInput(
  {className, value, onChange, addonAfter, min, max, ...props},
  ref,
) {
  const clamp = (next: number) => {
    let result = next;
    if (min !== undefined && result < Number(min)) {
      result = Number(min);
    }
    if (max !== undefined && result > Number(max)) {
      result = Number(max);
    }
    return result;
  };

  const field = (
    <Input
      ref={ref}
      type="number"
      min={min}
      max={max}
      value={value ?? ""}
      onChange={event => {
        const parsed = parseInt(event.target.value, 10);
        onChange(isNaN(parsed) ? 0 : parsed);
      }}
      onBlur={event => {
        const parsed = parseInt(event.target.value, 10);
        onChange(clamp(isNaN(parsed) ? 0 : parsed));
      }}
      className={cn(
        "tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none",
        addonAfter && "rounded-r-none",
        className,
      )}
      {...props}
    />
  );

  if (!addonAfter) {
    return field;
  }

  return (
    <div className="flex">
      {field}
      <span className="border-input bg-muted text-muted-foreground inline-flex h-9 items-center rounded-r-md border border-l-0 px-3 text-sm whitespace-nowrap">
        {addonAfter}
      </span>
    </div>
  );
});

export {NumberInput};
