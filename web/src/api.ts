import type { LibraryItem, Platform, SearchGame } from "./types";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const err = new Error(data?.error || res.statusText);
    (err as Error & { status: number }).status = res.status;
    throw err;
  }
  return data as T;
}

export const api = {
  me: () => req<{ igdb_configured: boolean }>("/v1/auth/me"),
  login: (password: string) =>
    req<{ token: string }>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),
  logout: () => req<void>("/v1/auth/logout", { method: "POST" }),
  library: (params?: { q?: string; platform?: string; sort?: string }) => {
    const qs = new URLSearchParams();
    if (params?.q) qs.set("q", params.q);
    if (params?.platform) qs.set("platform", params.platform);
    if (params?.sort) qs.set("sort", params.sort);
    const s = qs.toString();
    return req<{ items: LibraryItem[] }>(`/v1/library${s ? `?${s}` : ""}`);
  },
  get: (id: string) => req<LibraryItem>(`/v1/library/${id}`),
  create: (body: Record<string, unknown>) =>
    req<LibraryItem>("/v1/library", { method: "POST", body: JSON.stringify(body) }),
  patch: (id: string, body: Record<string, unknown>) =>
    req<LibraryItem>(`/v1/library/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  remove: (id: string) =>
    req<LibraryItem>(`/v1/library/${id}`, { method: "DELETE" }),
  platforms: () => req<{ platforms: Platform[] }>("/v1/platforms"),
  importCSV: async (file: File) => {
    const res = await fetch("/v1/library/import", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "text/csv" },
      body: await file.text(),
    });
    const text = await res.text();
    const data = text ? JSON.parse(text) : null;
    if (!res.ok) {
      throw new Error(data?.error || "Import failed");
    }
    return data as { imported: number };
  },
  exportCSV: async (params?: { q?: string; platform?: string; sort?: string }) => {
    const qs = new URLSearchParams();
    if (params?.q) qs.set("q", params.q);
    if (params?.platform) qs.set("platform", params.platform);
    if (params?.sort) qs.set("sort", params.sort);
    const s = qs.toString();
    const res = await fetch(`/v1/library.csv${s ? `?${s}` : ""}`, { credentials: "include" });
    if (!res.ok) {
      throw new Error("Export failed");
    }
    const blob = await res.blob();
    const disp = res.headers.get("Content-Disposition") || "";
    const match = /filename="([^"]+)"/.exec(disp);
    const name = match?.[1] || "game-db.csv";
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
  },
  search: (q: string, platform?: number) => {
    const qs = new URLSearchParams({ q });
    if (platform) qs.set("platform", String(platform));
    return req<{ games: SearchGame[] }>(`/v1/search/games?${qs}`);
  },
};

export function coverSrc(item: { cover_url?: string | null }) {
  return item.cover_url ?? null;
}
