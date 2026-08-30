import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api, coverSrc, formatUSD, valueCents } from "../api";
import type { LibraryItem } from "../types";

export default function Library({ igdb, pricecharting = false }: { igdb: boolean; pricecharting?: boolean }) {
  const [q, setQ] = useState("");
  const [platform, setPlatform] = useState("");
  const [sort, setSort] = useState("title");
  const [view, setView] = useState<"grid" | "list">("grid");
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState("");
  const nav = useNavigate();
  const qc = useQueryClient();

  const lib = useQuery({
    queryKey: ["library", sort],
    queryFn: () => api.library({ sort }),
    refetchInterval: (query) => {
      if (!pricecharting) return false;
      const items = query.state.data?.items ?? [];
      if (items.length === 0) return false;
      if (items.every((i) => i.value)) return false;
      return 10_000;
    },
  });

  const allItems = lib.data?.items ?? [];
  const platformCounts = useMemo(() => {
    const map = new Map<string, number>();
    for (const item of allItems) {
      map.set(item.platform, (map.get(item.platform) ?? 0) + 1);
    }
    return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  }, [allItems]);

  const items = useMemo(() => {
    const query = q.trim().toLowerCase();
    return allItems.filter((item) => {
      if (platform && item.platform !== platform) return false;
      if (query && !item.title.toLowerCase().includes(query)) return false;
      return true;
    });
  }, [allItems, platform, q]);

  const shelfValue = useMemo(() => {
    let sum = 0;
    let n = 0;
    for (const item of items) {
      const cents = valueCents(item);
      if (cents != null) {
        sum += cents;
        n++;
      }
    }
    return n > 0 ? sum : null;
  }, [items]);

  async function logout() {
    await api.logout();
    await qc.invalidateQueries({ queryKey: ["me"] });
    nav("/login");
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Game Library</h1>
          <p className="text-sm text-[#9aa3b2]">
            {items.length} game{items.length === 1 ? "" : "s"}
            {platform ? ` · ${platform}` : ""}
            {shelfValue != null ? ` · ${formatUSD(shelfValue)}` : ""}
            {!igdb && " · IGDB not configured"}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => api.exportCSV({ q, platform, sort })}
            disabled={items.length === 0}
            className="rounded-lg border border-[#2a2e38] px-3 py-2 text-sm text-[#9aa3b2] disabled:opacity-40"
          >
            Export CSV
          </button>
          <label className="rounded-lg border border-[#2a2e38] px-3 py-2 text-sm text-[#9aa3b2] disabled:opacity-40 cursor-pointer">
            {importing ? "Importing…" : "Import CSV"}
            <input
              type="file"
              accept=".csv,text/csv"
              className="hidden"
              disabled={importing}
              onChange={async (e) => {
                const file = e.target.files?.[0];
                e.target.value = "";
                if (!file) return;
                if (
                  !confirm(
                    "Import replaces your entire library with this CSV. This cannot be undone. Continue?",
                  )
                ) {
                  return;
                }
                setImportError("");
                setImporting(true);
                try {
                  await api.importCSV(file);
                  await qc.invalidateQueries({ queryKey: ["library"] });
                } catch (err) {
                  setImportError(err instanceof Error ? err.message : "Import failed");
                } finally {
                  setImporting(false);
                }
              }}
            />
          </label>
          <Link
            to="/add"
            className="rounded-lg bg-[#e2b14a] px-3 py-2 text-sm font-medium text-[#111]"
          >
            Add game
          </Link>
          <button
            onClick={logout}
            className="rounded-lg border border-[#2a2e38] px-3 py-2 text-sm text-[#9aa3b2]"
          >
            Sign out
          </button>
        </div>
      </header>

      <div className="mt-6 flex flex-col gap-6 md:flex-row md:items-start">
        {allItems.length > 0 && (
          <PlatformSidebar
            total={allItems.length}
            platforms={platformCounts}
            selected={platform}
            onSelect={setPlatform}
          />
        )}

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap gap-3">
            <input
              placeholder="Search titles"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              className="min-w-48 flex-1 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
            />
            <select
              value={sort}
              onChange={(e) => setSort(e.target.value)}
              className="rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
            >
              <option value="title">Title</option>
              <option value="added">Date added</option>
            </select>
            <div className="flex overflow-hidden rounded-lg border border-[#2a2e38] text-sm">
              <button
                type="button"
                className={`px-3 py-2 ${view === "grid" ? "bg-[#16181f] text-[#e2b14a]" : "text-[#9aa3b2]"}`}
                onClick={() => setView("grid")}
              >
                Grid
              </button>
              <button
                type="button"
                className={`px-3 py-2 ${view === "list" ? "bg-[#16181f] text-[#e2b14a]" : "text-[#9aa3b2]"}`}
                onClick={() => setView("list")}
              >
                List
              </button>
            </div>
          </div>

          {lib.isLoading && <p className="mt-10 text-[#9aa3b2]">Loading…</p>}
          {lib.isError && <p className="mt-10 text-red-400">Could not load library.</p>}
          {importError && <p className="mt-4 text-red-400">{importError}</p>}

          {!lib.isLoading && allItems.length === 0 && (
            <div className="mt-16 text-center text-[#9aa3b2]">
              <p className="text-lg text-[#e8eaef]">Nothing on the shelf yet.</p>
              <p className="mt-1">Add a game from IGDB or enter one manually.</p>
            </div>
          )}

          {!lib.isLoading && allItems.length > 0 && items.length === 0 && (
            <p className="mt-10 text-[#9aa3b2]">No games match this filter.</p>
          )}

          {items.length > 0 && view === "grid" && (
            <ul className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
              {items.map((item) => (
                <li key={item.id}>
                  <CoverCard item={item} />
                </li>
              ))}
            </ul>
          )}
          {items.length > 0 && view === "list" && (
            <ul className="mt-6 divide-y divide-[#2a2e38] rounded-xl border border-[#2a2e38]">
              {items.map((item) => (
                <li key={item.id}>
                  <Link
                    to={`/game/${item.id}`}
                    className="flex items-center gap-4 px-3 py-2 hover:bg-[#16181f]"
                  >
                    <CoverThumb item={item} />
                    <div className="min-w-0">
                      <div className="truncate font-medium">{item.title}</div>
                      <div className="text-sm text-[#9aa3b2]">
                        {item.platform}
                        {item.region ? ` · ${item.region.toUpperCase()}` : ""}
                        {` · ${item.completeness}`}
                        {formatUSD(valueCents(item)) ? ` · ${formatUSD(valueCents(item))}` : ""}
                      </div>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>

      <p className="mt-12 text-center text-xs text-[#9aa3b2]">
        Game data from IGDB.com. Values from PriceCharting when configured.
      </p>
    </div>
  );
}

function PlatformSidebar({
  total,
  platforms,
  selected,
  onSelect,
}: {
  total: number;
  platforms: [string, number][];
  selected: string;
  onSelect: (platform: string) => void;
}) {
  return (
    <nav
      aria-label="Platforms"
      className="max-h-48 overflow-y-auto rounded-xl border border-[#2a2e38] md:sticky md:top-4 md:max-h-[calc(100vh-6rem)] md:w-56 md:shrink-0 divide-y divide-[#2a2e38]"
    >
      <PlatformRow label="All games" count={total} active={selected === ""} onClick={() => onSelect("")} />
      {platforms.map(([name, count]) => (
        <PlatformRow
          key={name}
          label={name}
          count={count}
          active={selected === name}
          onClick={() => onSelect(name)}
        />
      ))}
    </nav>
  );
}

function PlatformRow({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={active ? "true" : undefined}
      title={label}
      className={`flex w-full items-center justify-between gap-3 border-l-2 px-3 py-2 text-left text-sm ${
        active
          ? "border-[#e2b14a] bg-[#16181f] text-[#e2b14a]"
          : "border-transparent text-[#e8eaef] hover:bg-[#16181f]"
      }`}
    >
      <span className="truncate">{label}</span>
      <span className={`tabular-nums ${active ? "text-[#e2b14a]" : "text-[#9aa3b2]"}`}>{count}</span>
    </button>
  );
}

function CoverCard({ item }: { item: LibraryItem }) {
  return (
    <Link to={`/game/${item.id}`} className="group block">
      <div className="aspect-[3/4] overflow-hidden rounded-lg border border-[#2a2e38] bg-[#16181f]">
        <CoverImg item={item} />
      </div>
      <div className="mt-2 truncate text-sm font-medium group-hover:text-[#e2b14a]">
        {item.title}
      </div>
      <div className="truncate text-xs text-[#9aa3b2]">{item.platform}</div>
      {formatUSD(valueCents(item)) && (
        <div className="truncate text-xs text-[#e2b14a]">{formatUSD(valueCents(item))}</div>
      )}
    </Link>
  );
}

function CoverThumb({ item }: { item: LibraryItem }) {
  return (
    <div className="h-14 w-10 shrink-0 overflow-hidden rounded border border-[#2a2e38] bg-[#16181f]">
      <CoverImg item={item} />
    </div>
  );
}

function CoverImg({ item }: { item: LibraryItem }) {
  const src = coverSrc(item);
  const [failed, setFailed] = useState(false);
  if (!src || failed) {
    return (
      <div className="grid h-full w-full place-items-center px-2 text-center text-xs text-[#9aa3b2]">
        {item.title}
      </div>
    );
  }
  return (
    <img
      src={src}
      alt=""
      className="h-full w-full object-cover"
      onError={() => setFailed(true)}
    />
  );
}
