import { FormEvent, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api";
import type { Completeness, Region, SearchGame } from "../types";

export default function AddGame({ igdb }: { igdb: boolean }) {
  const [tab, setTab] = useState<"search" | "manual">(igdb ? "search" : "manual");
  return (
    <div className="mx-auto max-w-3xl px-4 py-6">
      <Link to="/" className="text-sm text-[#9aa3b2] hover:text-[#e2b14a]">
        ← Library
      </Link>
      <h1 className="mt-4 text-2xl font-semibold">Add game</h1>
      <div className="mt-4 flex gap-2 text-sm">
        <button
          disabled={!igdb}
          onClick={() => setTab("search")}
          className={`rounded-lg px-3 py-1.5 ${tab === "search" ? "bg-[#e2b14a] text-[#111]" : "border border-[#2a2e38] text-[#9aa3b2]"}`}
        >
          Search IGDB
        </button>
        <button
          onClick={() => setTab("manual")}
          className={`rounded-lg px-3 py-1.5 ${tab === "manual" ? "bg-[#e2b14a] text-[#111]" : "border border-[#2a2e38] text-[#9aa3b2]"}`}
        >
          Manual
        </button>
      </div>
      {!igdb && (
        <p className="mt-3 text-sm text-[#9aa3b2]">
          Set IGDB_CLIENT_ID and IGDB_CLIENT_SECRET on the server to enable search.
        </p>
      )}
      <div className="mt-6">{tab === "search" && igdb ? <SearchAdd /> : <ManualAdd />}</div>
    </div>
  );
}

function SearchAdd() {
  const [q, setQ] = useState("");
  const [platformId, setPlatformId] = useState(0);
  const [submitted, setSubmitted] = useState<{ q: string; platformId: number } | null>(null);
  const nav = useNavigate();
  const qc = useQueryClient();
  const platforms = useQuery({
    queryKey: ["platforms"],
    queryFn: api.platforms,
  });
  const search = useQuery({
    queryKey: ["search", submitted?.q, submitted?.platformId],
    queryFn: () => api.search(submitted!.q, submitted!.platformId || undefined),
    enabled: !!submitted?.q,
  });
  const [picked, setPicked] = useState<SearchGame | null>(null);

  return (
    <div>
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const title = q.trim();
          if (!title) return;
          setSubmitted({ q: title, platformId });
          setPicked(null);
        }}
      >
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Title"
          className="min-w-48 flex-1 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
        />
        <select
          value={platformId}
          onChange={(e) => setPlatformId(Number(e.target.value))}
          className="min-w-44 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
          aria-label="Platform"
        >
          <option value={0}>All platforms</option>
          {(platforms.data?.platforms ?? []).map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        <button className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111]">
          Search
        </button>
      </form>
      {platforms.isError && (
        <p className="mt-2 text-xs text-[#9aa3b2]">Could not load the platform list. Search still works for all platforms.</p>
      )}
      {search.isLoading && <p className="mt-4 text-[#9aa3b2]">Searching…</p>}
      {search.isError && <p className="mt-4 text-red-400">Search failed.</p>}
      {search.isSuccess && !picked && (search.data.games ?? []).length === 0 && (
        <p className="mt-4 text-[#9aa3b2]">No matches{submitted?.platformId ? " for that platform" : ""}.</p>
      )}
      {!picked && (
        <ul className="mt-4 space-y-2">
          {(search.data?.games ?? []).map((g) => (
            <li key={g.igdb_id}>
              <button
                onClick={() => setPicked(g)}
                className="flex w-full items-center gap-3 rounded-lg border border-[#2a2e38] bg-[#16181f] p-2 text-left hover:border-[#e2b14a]"
              >
                <div className="h-16 w-12 overflow-hidden rounded bg-[#0e0f12]">
                  {g.cover_url ? (
                    <img src={g.cover_url} alt="" className="h-full w-full object-cover" />
                  ) : null}
                </div>
                <div>
                  <div className="font-medium">{g.name}</div>
                  <div className="text-xs text-[#9aa3b2]">
                    {g.platforms.map((p) => p.name).join(", ") || "No platforms"}
                  </div>
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
      {picked && (
        <ConfirmIGDB
          game={picked}
          preferredPlatformId={submitted?.platformId || undefined}
          onCancel={() => setPicked(null)}
          onAdded={async (id) => {
            await qc.invalidateQueries({ queryKey: ["library"] });
            nav(`/game/${id}`);
          }}
        />
      )}
    </div>
  );
}

function ConfirmIGDB({
  game,
  preferredPlatformId,
  onCancel,
  onAdded,
}: {
  game: SearchGame;
  preferredPlatformId?: number;
  onCancel: () => void;
  onAdded: (id: string) => void;
}) {
  const [platformId, setPlatformId] = useState(() => {
    if (preferredPlatformId && game.platforms.some((p) => p.id === preferredPlatformId)) {
      return preferredPlatformId;
    }
    return game.platforms[0]?.id ?? 0;
  });
  const [region, setRegion] = useState("us");
  const [completeness, setCompleteness] = useState<Completeness>("unknown");
  const add = useMutation({
    mutationFn: () =>
      api.create({
        igdb_game_id: game.igdb_id,
        igdb_platform_id: platformId || undefined,
        region: region || undefined,
        completeness,
      }),
    onSuccess: (item) => onAdded(item.id),
  });

  return (
    <div className="mt-4 rounded-xl border border-[#2a2e38] bg-[#16181f] p-4">
      <div className="font-medium">{game.name}</div>
      {game.summary && (
        <p className="mt-2 line-clamp-4 text-sm text-[#9aa3b2]">{game.summary}</p>
      )}
      <label className="mt-4 block text-sm text-[#9aa3b2]">
        Platform
        <select
          value={platformId}
          onChange={(e) => setPlatformId(Number(e.target.value))}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#0e0f12] px-3 py-2 text-sm"
        >
          {game.platforms.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </label>
      <div className="mt-3 grid grid-cols-2 gap-3">
        <label className="text-sm text-[#9aa3b2]">
          Region
          <select
            value={region}
            onChange={(e) => setRegion(e.target.value)}
            className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#0e0f12] px-3 py-2 text-sm"
          >
            <option value="">—</option>
            {(["us", "eu", "jp", "au", "other"] as Region[]).map((r) => (
              <option key={r} value={r}>
                {r.toUpperCase()}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-[#9aa3b2]">
          Completeness
          <select
            value={completeness}
            onChange={(e) => setCompleteness(e.target.value as Completeness)}
            className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#0e0f12] px-3 py-2 text-sm"
          >
            <option value="unknown">Unknown</option>
            <option value="loose">Loose</option>
            <option value="cib">CIB</option>
            <option value="new">New / sealed</option>
          </select>
        </label>
      </div>
      {add.isError && <p className="mt-3 text-sm text-red-400">Could not add game.</p>}
      <div className="mt-4 flex gap-2">
        <button
          onClick={() => add.mutate()}
          disabled={add.isPending || !platformId}
          className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111]"
        >
          Add to library
        </button>
        <button onClick={onCancel} className="text-sm text-[#9aa3b2]">
          Back
        </button>
      </div>
    </div>
  );
}

function ManualAdd() {
  const [title, setTitle] = useState("");
  const [platform, setPlatform] = useState("");
  const [region, setRegion] = useState("us");
  const [completeness, setCompleteness] = useState<Completeness>("unknown");
  const [notes, setNotes] = useState("");
  const nav = useNavigate();
  const qc = useQueryClient();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const item = await api.create({
      title,
      platform,
      region: region || undefined,
      completeness,
      notes,
    });
    await qc.invalidateQueries({ queryKey: ["library"] });
    nav(`/game/${item.id}`);
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <label className="block text-sm text-[#9aa3b2]">
        Title
        <input
          required
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        />
      </label>
      <label className="block text-sm text-[#9aa3b2]">
        Platform
        <input
          required
          value={platform}
          onChange={(e) => setPlatform(e.target.value)}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        />
      </label>
      <label className="block text-sm text-[#9aa3b2]">
        Region
        <select
          value={region}
          onChange={(e) => setRegion(e.target.value)}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        >
          <option value="">—</option>
          {(["us", "eu", "jp", "au", "other"] as Region[]).map((r) => (
            <option key={r} value={r}>
              {r.toUpperCase()}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm text-[#9aa3b2]">
        Completeness
        <select
          value={completeness}
          onChange={(e) => setCompleteness(e.target.value as Completeness)}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        >
          <option value="unknown">Unknown</option>
          <option value="loose">Loose</option>
          <option value="cib">CIB</option>
          <option value="new">New / sealed</option>
        </select>
      </label>
      <label className="block text-sm text-[#9aa3b2]">
        Notes
        <textarea
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          rows={3}
          className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm"
        />
      </label>
      <button className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111]">
        Add to library
      </button>
    </form>
  );
}
