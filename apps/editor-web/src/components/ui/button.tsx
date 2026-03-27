import type { ButtonHTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

type ButtonVariant = "default" | "secondary" | "ghost";
type ButtonSize = "sm" | "md" | "icon";

type ButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: ButtonVariant;
    size?: ButtonSize;
  }
>;

const variantClasses: Record<ButtonVariant, string> = {
  default: "bg-accent text-accent-foreground hover:brightness-105",
  secondary: "border border-border bg-panel-soft text-foreground hover:bg-panel-strong",
  ghost: "text-slate-300 hover:bg-white/6 hover:text-white",
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: "h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] px-2.5 text-[12px]",
  md: "h-[var(--ui-h-md)] rounded-[var(--ui-radius-md)] px-3 text-sm",
  icon: "h-[var(--ui-h-sm)] w-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] p-0 text-[12px]",
};

export function Button({
  className,
  variant = "default",
  size = "md",
  children,
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex items-center justify-center font-medium transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-offset-0 disabled:pointer-events-none disabled:opacity-50",
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}
