import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, formatUSD } from "../api";
import BarcodeScanner, { canScanBarcode } from "../BarcodeScanner";
import type { PriceCheck as PriceCheckResult, SearchGame } from "../types";

export default function PriceCheck({ igdb, prices }: { igdb: boolean; prices: boolean }) {
  const [tab, setTab] = useState<"search" | "barcode">(igdb ? "search" : "barcode");

  return (
    <div className="mx-auto max-w-3xl px-4 py-6">
      <Link to="/" className="text-sm text-[#9aa3b2] hover:text-[#e2b14a]">
        ← Game Library
      </Link>
      <h1 className="mt-4 text-2xl font-semibold">Price check</h1>
      <p className="mt-1 text-sm text-[#9aa3b2]">
        Scan or search a game to see Loose, CIB, and New asking prices. Nothing is added to the
        library.
      </p>
      {!prices && (
        <p className="mt-3 text-sm text-red-400">
          Pricing is not configured on the server. Set eBay or PriceCharting keys to look up values.
        </p>
      )}
      <div className="mt-4 flex gap-2 text-sm">
        <button
          onClick={() => setTab("search")}
          className={`rounded-lg px-3 py-1.5 ${tab === "search" ? "bg-[#e2b14a] text-[#111]" : "border border-[#2a2e38] text-[#9aa3b2]"}`}
        >
          Search
        </button>
        <button
          onClick={() => setTab("barcode")}
          className={`rounded-lg px-3 py-1.5 ${tab === "barcode" ? "bg-[#e2b14a] text-[#111]" : "border border-[#2a2e38] text-[#9aa3b2]"}`}
        >
          Barcode
        </button>
      </div>
      {!igdb && (
        <p className="mt-3 text-sm text-[#9aa3b2]">
          Set IGDB keys on the server to search by title. You can still scan or type a barcode, or
          enter a title below.
        </p>
      )}
      <div className="mt-6">
        {tab === "search" ? <SearchCheck igdb={igdb} enabled={prices} /> : <BarcodeCheck enabled={prices} />}
      </div>
    </div>
  );
}

function SearchCheck({ igdb, enabled }: { igdb: boolean; enabled: boolean }) {
  const [q, setQ] = useState("");
  const [platformId, setPlatformId] = useState(0);
  const [submitted, setSubmitted] = useState<{ q: string; platformId: number } | null>(null);
  const [picked, setPicked] = useState<SearchGame | null>(null);
  const [platformName, setPlatformName] = useState("");
  const [manualTitle, setManualTitle] = useState("");
  const [manualPlatform, setManualPlatform] = useState("");
  const [lookup, setLookup] = useState<{ title: string; platform: string } | null>(null);

  const platforms = useQuery({
    queryKey: ["platforms"],
    queryFn: api.platforms,
    enabled: igdb,
  });
  const search = useQuery({
    queryKey: ["search", submitted?.q, submitted?.platformId],
    queryFn: () => api.search(submitted!.q, submitted!.platformId || undefined),
    enabled: igdb && !!submitted?.q,
  });

  function pickGame(game: SearchGame) {
    setPicked(game);
    const preferred = preferredPlatform(game, submitted?.platformId);
    const name = game.platforms.find((p) => p.id === preferred)?.name ?? game.platforms[0]?.name ?? "";
    setPlatformName(name);
    if (enabled && game.name) {
      setLookup({ title: game.name, platform: name });
    }
  }

  return (
    <div>
      {igdb ? (
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const title = q.trim();
            if (!title) return;
            setSubmitted({ q: title, platformId });
            setPicked(null);
            setLookup(null);
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
      ) : (
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const title = manualTitle.trim();
            if (!title || !enabled) return;
            setLookup({ title, platform: manualPlatform.trim() });
          }}
        >
          <input
            value={manualTitle}
            onChange={(e) => setManualTitle(e.target.value)}
            placeholder="Title"
            className="min-w-48 flex-1 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
          />
          <input
            value={manualPlatform}
            onChange={(e) => setManualPlatform(e.target.value)}
            placeholder="Platform"
            className="min-w-44 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
          />
          <button
            disabled={!enabled}
            className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111] disabled:opacity-40"
          >
            Check prices
          </button>
        </form>
      )}

      {search.isLoading && <p className="mt-4 text-[#9aa3b2]">Searching…</p>}
      {search.isError && <p className="mt-4 text-red-400">Search failed.</p>}
      {search.isSuccess && !picked && (search.data.games ?? []).length === 0 && (
        <p className="mt-4 text-[#9aa3b2]">No matches{submitted?.platformId ? " for that platform" : ""}.</p>
      )}
      {igdb && (
        <ul className="mt-4 space-y-2">
          {(search.data?.games ?? []).map((g) => (
            <li key={g.igdb_id}>
              <button
                onClick={() => pickGame(g)}
                className={`flex w-full items-center gap-3 rounded-lg border bg-[#16181f] p-2 text-left hover:border-[#e2b14a] ${
                  picked?.igdb_id === g.igdb_id ? "border-[#e2b14a]" : "border-[#2a2e38]"
                }`}
              >
                <CoverThumb url={g.cover_url} />
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
      {picked && picked.platforms.length > 1 && (
        <label className="mt-4 block text-sm text-[#9aa3b2]">
          Platform
          <select
            value={platformName}
            onChange={(e) => {
              const name = e.target.value;
              setPlatformName(name);
              if (enabled) setLookup({ title: picked.name, platform: name });
            }}
            className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm text-[#e8eaef]"
          >
            {picked.platforms.map((p) => (
              <option key={p.id} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      )}
      <PricePanel lookup={lookup} coverUrl={picked?.cover_url} enabled={enabled} />
    </div>
  );
}

function BarcodeCheck({ enabled }: { enabled: boolean }) {
  const [q, setQ] = useState("");
  const [scanning, setScanning] = useState(false);
  const [lookupCode, setLookupCode] = useState("");
  const [picked, setPicked] = useState<SearchGame | null>(null);
  const [platformName, setPlatformName] = useState("");
  const [priceLookup, setPriceLookup] = useState<{
    title: string;
    platform: string;
    barcode: string;
  } | null>(null);

  const result = useQuery({
    queryKey: ["barcode", lookupCode],
    queryFn: () => api.searchBarcode(lookupCode),
    enabled: !!lookupCode,
  });

  useEffect(() => {
    if (!result.data || !lookupCode) return;
    const games = result.data.games ?? [];
    const game = games[0] ?? null;
    setPicked(game);
    const platform =
      (game && preferredPlatformName(game, result.data.platform_hint)) ||
      result.data.platform ||
      result.data.platform_hint ||
      "";
    setPlatformName(platform);
    const title = game?.name || result.data.query || result.data.product_title || "";
    if (title || lookupCode) {
      setPriceLookup({ title, platform, barcode: result.data.barcode || lookupCode });
    }
  }, [result.data, lookupCode]);

  function submit(code: string) {
    const digits = code.replace(/\D/g, "");
    if (!digits) return;
    setQ(digits);
    setLookupCode(digits);
    setPicked(null);
    setPriceLookup(null);
    setScanning(false);
  }

  function chooseGame(game: SearchGame) {
    setPicked(game);
    const platform = preferredPlatformName(game, result.data?.platform_hint);
    setPlatformName(platform);
    if (result.data) {
      setPriceLookup({ title: game.name, platform, barcode: result.data.barcode });
    }
  }

  return (
    <div>
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          submit(q);
        }}
      >
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="UPC / EAN"
          inputMode="numeric"
          className="min-w-48 flex-1 rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm outline-none focus:border-[#e2b14a]"
        />
        {canScanBarcode() && (
          <button
            type="button"
            onClick={() => setScanning(true)}
            className="rounded-lg border border-[#2a2e38] px-4 py-2 text-sm text-[#9aa3b2]"
          >
            Camera
          </button>
        )}
        <button className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111]">
          Lookup
        </button>
      </form>
      {scanning && (
        <BarcodeScanner
          onCode={(code) => submit(code)}
          onClose={() => setScanning(false)}
        />
      )}
      {result.isLoading && <p className="mt-4 text-[#9aa3b2]">Looking up barcode…</p>}
      {result.isError && <p className="mt-4 text-red-400">Barcode lookup failed.</p>}
      {result.data?.lookup_error && (
        <p className="mt-4 text-sm text-[#9aa3b2]">{result.data.lookup_error}</p>
      )}
      {result.data && (result.data.games ?? []).length > 0 && (
        <ul className="mt-4 space-y-2">
          {result.data.games.map((g) => (
            <li key={g.igdb_id}>
              <button
                type="button"
                onClick={() => chooseGame(g)}
                className={`flex w-full items-center gap-3 rounded-lg border bg-[#16181f] p-2 text-left hover:border-[#e2b14a] ${
                  picked?.igdb_id === g.igdb_id ? "border-[#e2b14a]" : "border-[#2a2e38]"
                }`}
              >
                <CoverThumb url={g.cover_url} />
                <div>
                  <div className="font-medium">{g.name}</div>
                  <div className="text-xs text-[#9aa3b2]">{g.platforms.map((p) => p.name).join(", ")}</div>
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
      {picked && picked.platforms.length > 1 && (
        <label className="mt-4 block text-sm text-[#9aa3b2]">
          Platform
          <select
            value={platformName}
            onChange={(e) => {
              const name = e.target.value;
              setPlatformName(name);
              if (result.data) {
                setPriceLookup({ title: picked.name, platform: name, barcode: result.data.barcode });
              }
            }}
            className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm text-[#e8eaef]"
          >
            {picked.platforms.map((p) => (
              <option key={p.id} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      )}
      <PricePanel
        lookup={priceLookup}
        coverUrl={picked?.cover_url}
        enabled={enabled}
      />
    </div>
  );
}

function PricePanel({
  lookup,
  coverUrl,
  enabled,
}: {
  lookup: { title: string; platform?: string; barcode?: string } | null;
  coverUrl?: string | null;
  enabled: boolean;
}) {
  const quote = useQuery({
    queryKey: ["price-check", lookup?.title, lookup?.platform, lookup?.barcode],
    queryFn: () =>
      api.checkPrice({
        title: lookup!.title || undefined,
        platform: lookup!.platform || undefined,
        barcode: lookup!.barcode || undefined,
      }),
    enabled: enabled && !!lookup && !!(lookup.title || lookup.barcode),
  });

  if (!lookup) return null;

  return (
    <div className="mt-6 rounded-xl border border-[#2a2e38] bg-[#16181f] p-4">
      <div className="flex gap-3">
        <CoverThumb url={coverUrl} large />
        <div className="min-w-0">
          <div className="font-medium">{lookup.title || "Unknown title"}</div>
          {lookup.platform && <div className="text-sm text-[#9aa3b2]">{lookup.platform}</div>}
          {lookup.barcode && <div className="text-xs text-[#9aa3b2]">{lookup.barcode}</div>}
        </div>
      </div>
      {quote.isLoading && <p className="mt-4 text-sm text-[#9aa3b2]">Looking up prices…</p>}
      {quote.isError && (
        <p className="mt-4 text-sm text-red-400">
          {quote.error instanceof Error ? quote.error.message : "Price lookup failed."}
        </p>
      )}
      {quote.data && <PriceTiles result={quote.data} />}
    </div>
  );
}

function PriceTiles({ result }: { result: PriceCheckResult }) {
  const v = result.value;
  if (result.status !== "ok" || !v) {
    return <p className="mt-4 text-sm text-[#9aa3b2]">No listings found for this title.</p>;
  }
  return (
    <div className="mt-4">
      <div className="text-xs text-[#9aa3b2]">
        {v.source === "ebay" ? "eBay asking" : "PriceCharting"}
        {v.listings ? ` · ${v.listings} listings` : ""}
        {v.product_name ? ` · ${v.product_name}` : ""}
        {v.console_name ? ` · ${v.console_name}` : ""}
      </div>
      <div className="mt-3 grid grid-cols-3 gap-2">
        <PriceTile label="Loose" cents={v.loose_cents} />
        <PriceTile label="CIB" cents={v.cib_cents} />
        <PriceTile label="New" cents={v.new_cents} />
      </div>
      {v.url && (
        <a
          href={v.url}
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-block text-xs text-[#e2b14a] hover:underline"
        >
          {v.source === "ebay" ? "See listings on eBay" : "Open on PriceCharting"}
        </a>
      )}
    </div>
  );
}

function PriceTile({ label, cents }: { label: string; cents: number | null | undefined }) {
  return (
    <div className="rounded-lg border border-[#2a2e38] bg-[#0e0f12] px-3 py-3 text-center">
      <div className="text-xs text-[#9aa3b2]">{label}</div>
      <div className="mt-1 text-lg font-semibold text-[#e2b14a]">{formatUSD(cents) ?? "—"}</div>
    </div>
  );
}

function CoverThumb({ url, large }: { url?: string | null; large?: boolean }) {
  const cls = large ? "h-24 w-[72px]" : "h-16 w-12";
  return (
    <div className={`${cls} shrink-0 overflow-hidden rounded bg-[#0e0f12]`}>
      {url ? <img src={url} alt="" className="h-full w-full object-cover" /> : null}
    </div>
  );
}

function preferredPlatform(game: SearchGame, preferred?: number) {
  if (preferred && game.platforms.some((p) => p.id === preferred)) return preferred;
  return game.platforms[0]?.id ?? 0;
}

function preferredPlatformName(game: SearchGame, hint?: string) {
  if (hint) {
    const h = hint.toLowerCase();
    const match = game.platforms.find(
      (p) => p.name.toLowerCase().includes(h) || h.includes(p.name.toLowerCase()),
    );
    if (match) return match.name;
  }
  return game.platforms[0]?.name ?? "";
}
