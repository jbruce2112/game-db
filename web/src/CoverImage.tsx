import { useEffect, useState } from "react";
import { coverSrc } from "./api";
import { useCoverArt } from "./coverArt";
import type { LibraryItem } from "./types";

/** Portrait fills the 3:4 tile; landscape (N64 etc.) is letterboxed so the full box is visible. */
export function CoverImage({ item }: { item: LibraryItem }) {
  const [coverArt] = useCoverArt();
  const preferred = coverSrc(item, coverArt);
  const fallback = coverArt === "box" ? coverSrc(item, "poster") : null;
  const [src, setSrc] = useState<string | null>(preferred);
  const [failed, setFailed] = useState(false);
  const [landscape, setLandscape] = useState(false);
  useEffect(() => {
    setSrc(preferred);
    setFailed(false);
    setLandscape(false);
  }, [preferred]);
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
      className={`h-full w-full ${landscape ? "object-contain" : "object-cover"}`}
      onLoad={(e) => {
        const img = e.currentTarget;
        setLandscape(img.naturalWidth > img.naturalHeight);
      }}
      onError={() => {
        if (fallback && src !== fallback) {
          setSrc(fallback);
          setLandscape(false);
          return;
        }
        setFailed(true);
      }}
    />
  );
}
