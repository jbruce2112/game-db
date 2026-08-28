import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api, coverSrc } from "../api";
import type { LibraryItem } from "../types";

export default function Library({ igdb }: { igdb: boolean }) {
  const [q, setQ] = useState("");
  const [platform, setPlatform] = useState("");
  const [sort, setSort] = useState("title");
  const [view, setView] = useState<"grid" | "list">("grid");
  const nav = useNavigate();
  const qc = useQueryClient();

  const lib = useQuery({
    queryKey: ["library", q, platform, sort],
    queryFn: () => api.library({ q, platform, sort }),
  });

  const platforms = useMemo(() => {
    const set = new Set((lib.data?.items ?? []).map((i) => i.platform));
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [lib.data]);

  async function logout() {
    await api.logout();
    await qc.invalidateQueries({ queryKey: ["me"] });
    nav("/login");
  }

  const items = lib.data?.items ?? [];

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Library</h1>
          <p className="text-sm text-[#9aa3b2]">
            {items.length} game{items.length === 1 ? "" : "s"}
            {!igdb && " · IGDB not configured"}
          </p>
        </div>
        <div className="flex gap-2">
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

      <div className="mt-6 flex flex-wrap gap-3">
        <input
          placeholder="Search titles"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="min-w-48 flex-1 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
        />
        <select
          value={platform}
          onChange={(e) => setPlatform(e.target.value)}
          className="rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        >
          <option value="">All platforms</option>
          {platforms.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </select>
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
            className={`px-3 py-2 ${view === "grid" ? "bg-[#16181f] text-[#e2b14a]" : "text-[#9aa3b2]"}`}
            onClick={() => setView("grid")}
          >
            Grid
          </button>
          <button
            className={`px-3 py-2 ${view === "list" ? "bg-[#16181f] text-[#e2b14a]" : "text-[#9aa3b2]"}`}
            onClick={() => setView("list")}
          >
            List
          </button>
        </div>
      </div>

      {lib.isLoading && <p className="mt-10 text-[#9aa3b2]">Loading…</p>}
      {lib.isError && <p className="mt-10 text-red-400">Could not load library.</p>}

      {!lib.isLoading && items.length === 0 && (
        <div className="mt-16 text-center text-[#9aa3b2]">
          <p className="text-lg text-[#e8eaef]">Nothing on the shelf yet.</p>
          <p className="mt-1">Add a game from IGDB or enter one manually.</p>
        </div>
      )}

      {view === "grid" ? (
        <ul className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {items.map((item) => (
            <li key={item.id}>
              <CoverCard item={item} />
            </li>
          ))}
        </ul>
      ) : (
        <ul className="mt-8 divide-y divide-[#2a2e38] rounded-xl border border-[#2a2e38]">
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
                  </div>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <p className="mt-12 text-center text-xs text-[#9aa3b2]">
        Game data from IGDB.com
      </p>
    </div>
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
  if (!src) {
    return (
      <div className="grid h-full w-full place-items-center px-2 text-center text-xs text-[#9aa3b2]">
        {item.title}
      </div>
    );
  }
  return <img src={src} alt="" className="h-full w-full object-cover" />;
}
