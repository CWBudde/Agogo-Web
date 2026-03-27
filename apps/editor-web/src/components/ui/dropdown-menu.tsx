import type { ButtonHTMLAttributes, HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

export function DropdownMenu({ children }: PropsWithChildren) {
  return <div className="relative inline-block">{children}</div>;
}

export function DropdownMenuTrigger({
  children,
  className,
  ...props
}: PropsWithChildren<ButtonHTMLAttributes<HTMLButtonElement>>) {
  return (
    <button type="button" className={cn("inline-flex items-center gap-2", className)} {...props}>
      {children}
    </button>
  );
}

export function DropdownMenuContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "absolute z-50 mt-2 min-w-48 rounded-2xl border border-white/10 bg-slate-950 p-2 shadow-2xl",
        className,
      )}
      {...props}
    />
  );
}

export function DropdownMenuItem({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cn(
        "flex w-full items-center rounded-xl px-3 py-2 text-left text-sm text-slate-200 hover:bg-white/5",
        className,
      )}
      {...props}
    />
  );
}
