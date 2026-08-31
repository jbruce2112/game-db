import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, formatUSD, valueCents } from "../api";
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
  const [yearPlatform, setYearPlatform] = useState("");
  const [cumulativePlatform, setCumulativePlatform] = useState("");
  const yearRows = useMemo(() => countsByYear(items, yearPlatform), [items, yearPlatform]);
  const cumulativeRows = useMemo(
    () => cumulative(countsByYear(items, cumulativePlatform)),
    [items, cumulativePlatform],
  );
  const qc = useQueryClient();
  const [clearing, setClearing] = useState(false);
  const [clearError, setClearError] = useState("");
  const [clearNote, setClearNote] = useState("");

  async function clearCache() {
    if (
      !confirm(
        "Clear cached covers, eBay prices, and lookup data? Games, dates, and notes are kept. Covers and prices will download again.",
      )
    ) {
      return;
    }
    setClearError("");
    setClearNote("");
    setClearing(true);
    try {
      const result = await api.clearCache();
      await qc.invalidateQueries({ queryKey: ["library"] });
      setClearNote(
        `Cleared ${result.covers} covers, ${result.prices} prices, ${result.barcodes} barcode lookups.`,
      );
    } catch (err) {
      setClearError(err instanceof Error ? err.message : "Clear failed");
    } finally {
      setClearing(false);
    }
  }

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

          <YearSection
            title="Added by year"
            footer="Copies added in each year. Empty years are shown as zero."
            filterLabel="Filter added-by-year chart by platform"
            platforms={stats.platforms.map((p) => p.name)}
            platform={yearPlatform}
            onPlatform={setYearPlatform}
            rows={yearRows}
          />
          <YearSection
            title="Shelf size"
            footer="Running total of copies on the shelf by the end of each year."
            filterLabel="Filter shelf-size chart by platform"
            platforms={stats.platforms.map((p) => p.name)}
            platform={cumulativePlatform}
            onPlatform={setCumulativePlatform}
            rows={cumulativeRows}
          />

          {stats.priced > 0 && stats.shelfCents != null && (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <Hero label="Shelf value" value={formatUSD(stats.shelfCents) ?? "—"} />
                <Hero label="Median asking" value={formatUSD(stats.medianCents) ?? "—"} />
                <Hero label="Priced" value={pct(stats.priced, stats.total)} />
                <Hero
                  label={stats.highest ? `Highest · ${stats.highest.title}` : "Highest"}
                  value={formatUSD(stats.highest?.cents) ?? "—"}
                />
              </div>
              <p className="text-xs text-[#9aa3b2]">
                eBay asking prices. Completeness “unknown” uses the CIB quote when present.
              </p>
              <MoneySection title="Value by platform" rows={stats.valueByPlatform} totalCents={stats.shelfCents} />
              <Ranked title="Most expensive" rows={stats.mostExpensive} />
            </>
          )}

          <Section title="Platforms" rows={stats.platforms} total={stats.total} />
          <Section title="Region" rows={stats.regions} total={stats.total} />
          <Section title="Completeness" rows={stats.completeness} total={stats.total} />
        </div>
      )}

      <CacheClear
        clearing={clearing}
        error={clearError}
        note={clearNote}
        onClear={clearCache}
      />
    </div>
  );
}

function CacheClear({
  clearing,
  error,
  note,
  onClear,
}: {
  clearing: boolean;
  error: string;
  note: string;
  onClear: () => void;
}) {
  return (
    <section className="mt-16 border-t border-[#2a2e38] pt-8">
      <h2 className="text-sm font-medium text-[#9aa3b2]">Cache</h2>
      <p className="mt-2 text-sm text-[#9aa3b2]">
        Remove downloaded covers, eBay asking prices, and barcode lookup cache. Core library
        fields stay. Covers and prices are fetched again afterward.
      </p>
      <button
        type="button"
        onClick={onClear}
        disabled={clearing}
        className="mt-4 rounded-lg border border-[#5c2a2a] px-3 py-2 text-sm text-[#f0a0a0] disabled:opacity-40"
      >
        {clearing ? "Clearing…" : "Clear cached covers and prices"}
      </button>
      {note && <p className="mt-3 text-sm text-[#e2b14a]">{note}</p>}
      {error && <p className="mt-3 text-sm text-red-400">{error}</p>}
    </section>
  );
}

function YearSection({
  title,
  footer,
  filterLabel,
  platforms,
  platform,
  onPlatform,
  rows,
}: {
  title: string;
  footer: string;
  filterLabel: string;
  platforms: string[];
  platform: string;
  onPlatform: (value: string) => void;
  rows: { year: number; count: number }[];
}) {
  return (
    <section>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h2 className="text-sm font-medium text-[#9aa3b2]">{title}</h2>
        <select
          value={platform}
          onChange={(e) => onPlatform(e.target.value)}
          className="rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-1.5 text-sm"
          aria-label={filterLabel}
        >
          <option value="">All platforms</option>
          {platforms.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </select>
      </div>
      {rows.length === 0 ? (
        <p className="mt-3 text-sm text-[#9aa3b2]">No added dates to chart.</p>
      ) : (
        <>
          <YearChart rows={rows} />
          <p className="mt-2 text-xs text-[#9aa3b2]">{footer}</p>
        </>
      )}
    </section>
  );
}

function YearChart({ rows }: { rows: { year: number; count: number }[] }) {
  const w = 640;
  const h = 220;
  const padL = 44;
  const padR = 16;
  const padT = 16;
  const padB = 28;
  const max = Math.max(...rows.map((r) => r.count), 1);
  const innerW = w - padL - padR;
  const innerH = h - padT - padB;
  const xAt = (i: number) => (rows.length === 1 ? padL + innerW / 2 : padL + (i / (rows.length - 1)) * innerW);
  const yAt = (count: number) => padT + innerH - (count / max) * innerH;
  const points = rows.map((row, i) => `${xAt(i)},${yAt(row.count)}`).join(" ");
  const labelEvery = rows.length > 8 ? Math.ceil(rows.length / 8) : 1;
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      className="mt-3 w-full overflow-visible rounded-xl border border-[#2a2e38] bg-[#16181f]"
      role="img"
      aria-label={rows.map((r) => `${r.year}: ${r.count}`).join(", ")}
    >
      <line x1={padL} y1={padT} x2={padL} y2={h - padB} stroke="#2a2e38" />
      <line x1={padL} y1={h - padB} x2={w - padR} y2={h - padB} stroke="#2a2e38" />
      <text x={4} y={padT + 4} fill="#9aa3b2" fontSize="11">
        {max}
      </text>
      <text x={4} y={h - padB} fill="#9aa3b2" fontSize="11">
        0
      </text>
      <polyline fill="none" stroke="#e2b14a" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" points={points} />
      {rows.map((row, i) => (
        <g key={row.year}>
          <circle cx={xAt(i)} cy={yAt(row.count)} r="3.5" fill="#e2b14a" />
          {i % labelEvery === 0 && (
            <text x={xAt(i)} y={h - 8} textAnchor="middle" fill="#9aa3b2" fontSize="11">
              {row.year}
            </text>
          )}
        </g>
      ))}
    </svg>
  );
}

function Hero({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border border-[#2a2e38] bg-[#16181f] px-4 py-3">
      <div className="truncate text-2xl font-semibold tabular-nums text-[#e2b14a]">{value}</div>
      <div className="mt-1 truncate text-xs text-[#9aa3b2]">{label}</div>
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
            <Bar share={total ? row.count / total : 0} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function MoneySection({
  title,
  rows,
  totalCents,
}: {
  title: string;
  rows: { name: string; cents: number }[];
  totalCents: number;
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
                {formatUSD(row.cents)}
                <span className="ml-2 text-xs">{pct(row.cents, totalCents)}</span>
              </span>
            </div>
            <Bar share={totalCents ? row.cents / totalCents : 0} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function Ranked({
  title,
  rows,
}: {
  title: string;
  rows: { id: string; title: string; platform: string; cents: number }[];
}) {
  return (
    <section>
      <h2 className="text-sm font-medium text-[#9aa3b2]">{title}</h2>
      <ol className="mt-3 divide-y divide-[#2a2e38] overflow-hidden rounded-xl border border-[#2a2e38]">
        {rows.map((row, i) => (
          <li key={row.id}>
            <Link to={`/game/${row.id}`} className="flex items-baseline gap-3 px-3 py-2 hover:bg-[#16181f]">
              <span className="w-5 shrink-0 tabular-nums text-xs text-[#9aa3b2]">{i + 1}</span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm">{row.title}</span>
                <span className="block truncate text-xs text-[#9aa3b2]">{row.platform}</span>
              </span>
              <span className="shrink-0 tabular-nums text-sm text-[#e2b14a]">{formatUSD(row.cents)}</span>
            </Link>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Bar({ share }: { share: number }) {
  return (
    <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-[#2a2e38]">
      <div
        className="h-full rounded-full bg-[#e2b14a]"
        style={{ width: `${Math.min(Math.max(share, 0), 1) * 100}%` }}
      />
    </div>
  );
}

function pct(n: number, total: number) {
  if (!total) return "0%";
  return `${Math.round((n / total) * 100)}%`;
}

function median(values: number[]) {
  if (!values.length) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 1) return sorted[mid];
  return Math.round((sorted[mid - 1] + sorted[mid]) / 2);
}

export function buildStats(items: LibraryItem[]) {
  const live = items.filter((item) => !item.deleted_at);
  const total = live.length;
  const platforms = countBy(live, (i) => i.platform);
  const regionMap = new Map<string, number>();
  const completeMap = new Map<string, number>();
  const platValue = new Map<string, number>();
  const pricedRows: { id: string; title: string; platform: string; cents: number }[] = [];
  let withBarcode = 0;
  let withCover = 0;
  for (const item of live) {
    const region = item.region && regionLabels[item.region] ? item.region : "";
    regionMap.set(region, (regionMap.get(region) ?? 0) + 1);
    const c = completenessLabels[item.completeness] ? item.completeness : "unknown";
    completeMap.set(c, (completeMap.get(c) ?? 0) + 1);
    if (item.barcode) withBarcode += 1;
    if (item.cover_id || item.cover_url) withCover += 1;
    const cents = valueCents(item);
    if (cents != null) {
      platValue.set(item.platform, (platValue.get(item.platform) ?? 0) + cents);
      pricedRows.push({ id: item.id, title: item.title, platform: item.platform, cents });
    }
  }
  const regionOrder = ["us", "eu", "jp", "au", "other", ""];
  const shelfCents = pricedRows.length ? pricedRows.reduce((s, r) => s + r.cents, 0) : null;
  return {
    total,
    withBarcode,
    withCover,
    priced: pricedRows.length,
    shelfCents,
    medianCents: median(pricedRows.map((r) => r.cents)),
    highest: pricedRows.slice().sort((a, b) => b.cents - a.cents || a.title.localeCompare(b.title))[0] ?? null,
    platforms: platforms.sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)),
    regions: regionOrder
      .filter((k) => regionMap.has(k))
      .map((k) => ({ name: k ? regionLabels[k] : "Unset", count: regionMap.get(k) ?? 0 })),
    completeness: completenessOrder
      .filter((k) => completeMap.has(k))
      .map((k) => ({ name: completenessLabels[k], count: completeMap.get(k) ?? 0 })),
    valueByPlatform: [...platValue.entries()]
      .filter(([, cents]) => cents > 0)
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      .map(([name, cents]) => ({ name, cents })),
    mostExpensive: pricedRows
      .slice()
      .sort((a, b) => b.cents - a.cents || a.title.localeCompare(b.title))
      .slice(0, 10),
  };
}

export function countsByYear(items: LibraryItem[], platform = "") {
  const live = items.filter((item) => !item.deleted_at);
  const filtered = platform ? live.filter((item) => item.platform === platform) : live;
  const map = new Map<number, number>();
  for (const item of filtered) {
    const y = yearOf(item.created_at);
    if (y == null) continue;
    map.set(y, (map.get(y) ?? 0) + 1);
  }
  if (map.size === 0) return [];
  const years = [...map.keys()];
  const minY = Math.min(...years);
  const maxY = Math.max(...years);
  const rows: { year: number; count: number }[] = [];
  for (let y = minY; y <= maxY; y++) {
    rows.push({ year: y, count: map.get(y) ?? 0 });
  }
  return rows;
}

export function cumulative(rows: { year: number; count: number }[]) {
  let sum = 0;
  return rows.map((row) => {
    sum += row.count;
    return { year: row.year, count: sum };
  });
}

function yearOf(iso: string) {
  if (!iso || iso.length < 4) return null;
  const y = Number(iso.slice(0, 4));
  if (!Number.isInteger(y) || y < 1970 || y > 2100) return null;
  return y;
}

function countBy(items: LibraryItem[], key: (i: LibraryItem) => string) {
  const map = new Map<string, number>();
  for (const item of items) {
    const k = key(item);
    map.set(k, (map.get(k) ?? 0) + 1);
  }
  return [...map.entries()].map(([name, count]) => ({ name, count }));
}
