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
  barcode: string | null;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
  sync_seq: number;
  value?: PriceValue | null;
};

export type PriceValue = {
  pc_id: string;
  product_name: string;
  console_name: string;
  url: string;
  source?: string;
  listings?: number;
  loose_cents: number | null;
  cib_cents: number | null;
  new_cents: number | null;
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

export type OwnedCopy = {
  id: string;
  title: string;
  platform: string;
};

export type BarcodeSearch = {
  barcode: string;
  product_title: string;
  query: string;
  source: string;
  platform_hint?: string;
  platform?: string;
  lookup_error?: string;
  games: SearchGame[];
  owned: OwnedCopy[];
};
