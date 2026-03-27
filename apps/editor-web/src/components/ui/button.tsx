import type { ButtonHTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

type ButtonVariant = "default" | "secondary" | "ghost";

type ButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: ButtonVariant;
  }
>;

const variantClasses: Record<ButtonVariant, string> = {
  default:
    "bg-accent text-accent-foreground shadow-[0_10px_30px_rgba(16,185,129,0.16)] hover:opacity-95",
  secondary: "border border-border bg-white/[0.04] text-foreground hover:bg-white/[0.08]",
  ghost: "text-slate-300 hover:bg-white/5 hover:text-white",
};

export function Button({
  className,
  variant = "default",
  children,
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex h-10 items-center justify-center rounded-[var(--radius-control)] px-4 text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-0",
        variantClasses[variant],
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}
