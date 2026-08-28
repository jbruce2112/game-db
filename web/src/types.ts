export type Completeness = "unknown" | "loose" | "cib" | "new";
export type Region = "us" | "eu" | "jp" | "au" | "other";

export type LibraryItem = {
  id: string;
  title: string;
  platform: string;
  igdb_platform_id: number | null;
  region: Region | null;
  completeness: Completeness;
  notes: string;
  igdb_game_id: number | null;
  cover_id: string | null;
  cover_url: string | null;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
  sync_seq: number;
};

export type Platform = {
  id: number;
  name: string;
  slug?: string | null;
  abbreviation?: string | null;
};

export type SearchGame = {
  igdb_id: number;
  name: string;
  summary: string;
  cover_url: string | null;
  first_release_date: string | null;
  platforms: Platform[];
};
