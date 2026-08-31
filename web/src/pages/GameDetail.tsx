import { FormEvent, type ReactNode, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { CoverImage } from "../CoverImage";
import { api, formatAdded, formatUSD } from "../api";
import type { Completeness, Region } from "../types";

export default function GameDetail() {
  const { id } = useParams();
  const nav = useNavigate();
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["game", id],
    queryFn: () => api.get(id!),
    enabled: !!id,
  });

  const [title, setTitle] = useState("");
  const [platform, setPlatform] = useState("");
  const [region, setRegion] = useState<string>("");
  const [completeness, setCompleteness] = useState<Completeness>("unknown");
  const [notes, setNotes] = useState("");
  const [barcode, setBarcode] = useState("");

  useEffect(() => {
    if (!q.data) return;
    setTitle(q.data.title);
    setPlatform(q.data.platform);
    setRegion(q.data.region ?? "");
    setCompleteness(q.data.completeness);
    setNotes(q.data.notes);
    setBarcode(q.data.barcode ?? "");
  }, [q.data]);

  const save = useMutation({
    mutationFn: () =>
      api.patch(id!, {
        title,
        platform,
        region: region || null,
        completeness,
        notes,
        barcode: barcode || null,
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["library"] });
      nav("/");
    },
  });

  const del = useMutation({
    mutationFn: () => api.remove(id!),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["library"] });
      nav("/");
    },
  });

  if (q.isLoading) return <p className="p-8 text-[#9aa3b2]">Loading…</p>;
  if (q.isError || !q.data) return <p className="p-8 text-red-400">Not found.</p>;

  async function onSave(e: FormEvent) {
    e.preventDefault();
    await save.mutateAsync();
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-6">
      <Link to="/" className="text-sm text-[#9aa3b2] hover:text-[#e2b14a]">
        ← Game Library
      </Link>
      <div className="mt-6 grid gap-8 sm:grid-cols-[180px_1fr]">
        <div className="aspect-[3/4] overflow-hidden rounded-lg border border-[#2a2e38] bg-[#16181f]">
          <CoverImage item={q.data} />
        </div>
        <form onSubmit={onSave} className="space-y-3">
          <Field label="Title">
            <input value={title} onChange={(e) => setTitle(e.target.value)} className={inputCls} />
          </Field>
          <Field label="Platform">
            <input value={platform} onChange={(e) => setPlatform(e.target.value)} className={inputCls} />
          </Field>
          <Field label="Region">
            <select value={region} onChange={(e) => setRegion(e.target.value)} className={inputCls}>
              <option value="">—</option>
              {(["us", "eu", "jp", "au", "other"] as Region[]).map((r) => (
                <option key={r} value={r}>
                  {r.toUpperCase()}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Completeness">
            <select
              value={completeness}
              onChange={(e) => setCompleteness(e.target.value as Completeness)}
              className={inputCls}
            >
              <option value="unknown">Unknown</option>
              <option value="loose">Loose</option>
              <option value="cib">CIB</option>
              <option value="new">New / sealed</option>
            </select>
          </Field>
          <Field label="Barcode">
            <input value={barcode} onChange={(e) => setBarcode(e.target.value)} inputMode="numeric" className={inputCls} />
          </Field>
          <Field label="Notes">
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={4}
              className={inputCls}
            />
          </Field>
          {q.data.value && (
            <div className="rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm">
              <div className="text-[#9aa3b2]">
                {q.data.value.source === "ebay" ? "eBay asking" : "PriceCharting"}
                {q.data.value.listings ? ` · ${q.data.value.listings} listings` : ""}
              </div>
              <div className="mt-1 text-[#e8eaef]">
                {q.data.value.product_name}
                {q.data.value.console_name ? ` · ${q.data.value.console_name}` : ""}
              </div>
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[#9aa3b2]">
                <span className={q.data.completeness === "loose" ? "text-[#e2b14a]" : ""}>
                  Loose {formatUSD(q.data.value.loose_cents) ?? "—"}
                </span>
                <span className={q.data.completeness === "cib" ? "text-[#e2b14a]" : ""}>
                  CIB {formatUSD(q.data.value.cib_cents) ?? "—"}
                </span>
                <span className={q.data.completeness === "new" ? "text-[#e2b14a]" : ""}>
                  New {formatUSD(q.data.value.new_cents) ?? "—"}
                </span>
              </div>
              {q.data.value.url && (
                <a
                  href={q.data.value.url}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-2 inline-block text-xs text-[#e2b14a] hover:underline"
                >
                  {q.data.value.source === "ebay" ? "See listings on eBay" : "Open on PriceCharting"}
                </a>
              )}
            </div>
          )}
          <p className="text-xs text-[#9aa3b2]">
            Added to collection {formatAdded(q.data.created_at)}
          </p>
          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={save.isPending}
              className="rounded-lg bg-[#e2b14a] px-4 py-2 text-sm font-medium text-[#111]"
            >
              {save.isSuccess && !save.isPending ? "Saved" : "Save"}
            </button>
            <button
              type="button"
              onClick={() => {
                if (confirm("Delete this copy from the library?")) del.mutate();
              }}
              className="rounded-lg border border-red-900 px-4 py-2 text-sm text-red-400"
            >
              Delete
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block text-sm text-[#9aa3b2]">
      {label}
      <div className="mt-1">{children}</div>
    </label>
  );
}

const inputCls =
  "w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-sm text-[#e8eaef] outline-none focus:border-[#e2b14a]";
