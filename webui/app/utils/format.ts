// Spell out numbers 1-20 — Theater voice idiom: "Twenty-Two Services"
const WORDS: Record<number, string> = {
  0: "Zero", 1: "One", 2: "Two", 3: "Three", 4: "Four", 5: "Five",
  6: "Six", 7: "Seven", 8: "Eight", 9: "Nine", 10: "Ten",
  11: "Eleven", 12: "Twelve", 13: "Thirteen", 14: "Fourteen", 15: "Fifteen",
  16: "Sixteen", 17: "Seventeen", 18: "Eighteen", 19: "Nineteen", 20: "Twenty",
};

const TENS: Record<number, string> = {
  20: "Twenty", 30: "Thirty", 40: "Forty", 50: "Fifty",
  60: "Sixty", 70: "Seventy", 80: "Eighty", 90: "Ninety",
};

export function spellOut(n: number): string {
  if (n in WORDS) return WORDS[n]!;
  if (n < 100) {
    const t = Math.floor(n / 10) * 10;
    const o = n % 10;
    return o === 0 ? TENS[t]! : `${TENS[t]}-${WORDS[o]?.toLowerCase()}`;
  }
  return String(n);
}

export function shortSha(sha: string): string {
  return sha.slice(0, 7);
}

export function shortRef(ref: string): string {
  return ref.replace(/^refs\/heads\//, "").replace(/^refs\/tags\//, "tag: ");
}

export function ago(iso: string, now = Date.now()): string {
  const ms = now - new Date(iso).getTime();
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}

export function duration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.floor(s % 60);
  return `${m}m ${rem}s`;
}

export function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(2)} GB`;
}

export function pct(n: number): string {
  return `${Math.round(n * 100)}%`;
}

// Uptime in seconds → "14d 06h 22m"
export function uptime(seconds: number): string {
  const d = Math.floor(seconds / 86_400);
  const h = Math.floor((seconds % 86_400) / 3_600);
  const m = Math.floor((seconds % 3_600) / 60);
  if (d > 0) return `${d}d ${String(h).padStart(2, "0")}h ${String(m).padStart(2, "0")}m`;
  if (h > 0) return `${h}h ${String(m).padStart(2, "0")}m`;
  return `${m}m`;
}

// Prometheus histogram seconds → human, picking the right unit
export function secs(s: number): string {
  if (s < 0.001) return `${(s * 1_000_000).toFixed(0)}µs`;
  if (s < 1)     return `${(s * 1_000).toFixed(0)}ms`;
  if (s < 60)    return `${s.toFixed(2)}s`;
  const m = Math.floor(s / 60);
  const r = Math.floor(s % 60);
  return `${m}m ${r}s`;
}

// Sum the counts across a series of {count: number} buckets
export function sumCount<T extends { count: number }>(rows: T[]): number {
  return rows.reduce((acc, r) => acc + r.count, 0);
}
