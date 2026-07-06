export function cn(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}

/**
 * Parses the raw string of a numeric input field. Returns `fallback` for
 * empty, whitespace-only, or non-numeric text so that clearing a field never
 * dispatches 0 (`Number("") === 0`).
 */
export function parseNumericInput(raw: string, fallback: number): number {
  if (raw.trim() === "") {
    return fallback;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}
