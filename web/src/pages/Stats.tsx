import { useMemo } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { LibraryItem } from "../types";

const regionLabels: Record<string, string> = {
  us: "USA",
  eu: "Europe",
  jp: "Japan",
  au: "Australia",
  other: "Other",
};

const completenessOrder = ["cib", "loose", "new", "unknown"] as const;
const completenessLabels: Record<string, string> = {
  cib: "CIB",
  loose: "Loose",
  new: "New",
  unknown: "Unknown",
};

export default function Stats() {
  const lib = useQuery({
    queryKey: ["library", "title"],
    queryFn: () => api.library({ sort: "title" }),
  });
  const items = lib.data?.items ?? [];
  const stats = useMemo(() => buildStats(items), [items]);

  return (
    <div className="mx-auto max-w-3xl px-4 py-6">
      <Link to="/" className="text-sm text-[#9aa3b2] hover:text-[#e2b14a]">
        ← Game Library
      </Link>
      <h1 className="mt-4 text-2xl font-semibold tracking-tight">Statistics</h1>
      <p className="mt-1 text-sm text-[#9aa3b2]">
        {lib.isLoading ? "Loading…" : `${stats.total} game${stats.total === 1 ? "" : "s"} on the shelf`}
      </p>

      {lib.isError && <p className="mt-10 text-red-400">Could not load library.</p>}

      {!lib.isLoading && stats.total === 0 && (
        <p className="mt-16 text-center text-[#9aa3b2]">Nothing on the shelf yet.</p>
      )}

      {stats.total > 0 && (
        <div className="mt-8 space-y-10">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Hero label="Games" value={stats.total} />
            <Hero label="Platforms" value={stats.platforms.length} />
            <Hero label="With cover" value={pct(stats.withCover, stats.total)} />
            <Hero label="With barcode" value={pct(stats.withBarcode, stats.total)} />
          </div>

          <Section title="Platforms" rows={stats.platforms} total={stats.total} />
          <Section title="Region" rows={stats.regions} total={stats.total} />
          <Section title="Completeness" rows={stats.completeness} total={stats.total} />
        </div>
      )}
    </div>
  );
}

function Hero({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border border-[#2a2e38] bg-[#16181f] px-4 py-3">
      <div className="text-2xl font-semibold tabular-nums text-[#e2b14a]">{value}</div>
      <div className="mt-1 text-xs text-[#9aa3b2]">{label}</div>
    </div>
  );
}

function Section({
  title,
  rows,
  total,
}: {
  title: string;
  rows: { name: string; count: number }[];
  total: number;
}) {
  return (
    <section>
      <h2 className="text-sm font-medium text-[#9aa3b2]">{title}</h2>
      <ul className="mt-3 space-y-2">
        {rows.map((row) => (
          <li key={row.name}>
            <div className="flex items-baseline justify-between gap-3 text-sm">
              <span className="truncate">{row.name}</span>
              <span className="shrink-0 tabular-nums text-[#9aa3b2]">
                {row.count}
                <span className="ml-2 text-xs">{pct(row.count, total)}</span>
              </span>
            </div>
            <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-[#2a2e38]">
              <div
                className="h-full rounded-full bg-[#e2b14a]"
                style={{ width: `${total ? (row.count / total) * 100 : 0}%` }}
              />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function pct(n: number, total: number) {
  if (!total) return "0%";
  return `${Math.round((n / total) * 100)}%`;
}

export function buildStats(items: LibraryItem[]) {
  const live = items.filter((item) => !item.deleted_at);
  const total = live.length;
  const platforms = countBy(live, (i) => i.platform);
  const regionMap = new Map<string, number>();
  const completeMap = new Map<string, number>();
  let withBarcode = 0;
  let withCover = 0;
  for (const item of live) {
    const region = item.region && regionLabels[item.region] ? item.region : "";
    regionMap.set(region, (regionMap.get(region) ?? 0) + 1);
    const c = completenessLabels[item.completeness] ? item.completeness : "unknown";
    completeMap.set(c, (completeMap.get(c) ?? 0) + 1);
    if (item.barcode) withBarcode += 1;
    if (item.cover_id || item.cover_url) withCover += 1;
  }
  const regionOrder = ["us", "eu", "jp", "au", "other", ""];
  return {
    total,
    withBarcode,
    withCover,
    platforms: platforms.sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)),
    regions: regionOrder
      .filter((k) => regionMap.has(k))
      .map((k) => ({ name: k ? regionLabels[k] : "Unset", count: regionMap.get(k) ?? 0 })),
    completeness: completenessOrder
      .filter((k) => completeMap.has(k))
      .map((k) => ({ name: completenessLabels[k], count: completeMap.get(k) ?? 0 })),
  };
}

function countBy(items: LibraryItem[], key: (i: LibraryItem) => string) {
  const map = new Map<string, number>();
  for (const item of items) {
    const k = key(item);
    map.set(k, (map.get(k) ?? 0) + 1);
  }
  return [...map.entries()].map(([name, count]) => ({ name, count }));
}
