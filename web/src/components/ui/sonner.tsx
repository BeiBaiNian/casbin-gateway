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

import {Toaster as Sonner, type ToasterProps} from "sonner";

import {TooltipProvider} from "@/components/ui/tooltip";

// Sonner is themed through CSS variables rather than its own light/dark prop, so
// it follows the `.dark` class on <html> exactly like the rest of the UI and
// never disagrees with the page behind it.
function Toaster({...props}: ToasterProps) {
  // The toaster hangs outside <App/>, so the copy button on a message needs its
  // own tooltip context.
  return (
    <TooltipProvider>
      <Sonner
        className="toaster group"
        position="top-center"
        richColors
        closeButton
        toastOptions={{
          classNames: {
            toast:
              "group toast group-[.toaster]:bg-popover group-[.toaster]:text-popover-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg",
            description: "group-[.toast]:text-muted-foreground",
            actionButton: "group-[.toast]:bg-primary group-[.toast]:text-primary-foreground",
            cancelButton: "group-[.toast]:bg-muted group-[.toast]:text-muted-foreground",
          },
        }}
        {...props}
      />
    </TooltipProvider>
  );
}

export {Toaster};
