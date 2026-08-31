import { useEffect, useState } from "react";
import { coverSrc } from "./api";
import { useCoverArt } from "./coverArt";
import type { LibraryItem } from "./types";

const chrome = "overflow-hidden rounded-lg border border-[#2a2e38] bg-[#16181f]";

type Layout = "grid" | "thumb" | "detail";

/** Poster mode fills a 3:4 tile. Box mode sizes the frame to the scan so PS1, GB, N64, etc. are uncropped. */
export function CoverImage({ item, layout = "grid" }: { item: LibraryItem; layout?: Layout }) {
  const [coverArt] = useCoverArt();
  const preferred = coverSrc(item, coverArt);
  const fallback = coverArt === "box" ? coverSrc(item, "poster") : null;
  const [src, setSrc] = useState<string | null>(preferred);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setSrc(preferred);
    setFailed(false);
  }, [preferred]);

  const box = coverArt === "box";
  if (!src || failed) {
    return <div className={`${placeholderClass(layout)} ${chrome} grid place-items-center px-2 text-center text-xs text-[#9aa3b2]`}>{item.title}</div>;
  }

  if (box) {
    return (
      <img
        src={src}
        alt=""
        className={`${boxSizeClass(layout)} ${chrome} block`}
        onError={() => {
          if (fallback && src !== fallback) {
            setSrc(fallback);
            return;
          }
          setFailed(true);
        }}
      />
    );
  }

  return (
    <div className={`${posterFrameClass(layout)} ${chrome}`}>
      <img
        src={src}
        alt=""
        className="h-full w-full object-cover"
        onError={() => {
          if (fallback && src !== fallback) {
            setSrc(fallback);
            return;
          }
          setFailed(true);
        }}
      />
    </div>
  );
}

function boxSizeClass(layout: Layout): string {
  switch (layout) {
    case "thumb":
      return "h-14 w-auto max-w-[6rem] shrink-0";
    case "detail":
      return "max-h-[28rem] w-auto max-w-full";
    default:
      return "h-auto w-full";
  }
}

function posterFrameClass(layout: Layout): string {
  switch (layout) {
    case "thumb":
      return "h-14 w-10 shrink-0";
    case "detail":
      return "aspect-[3/4] w-full max-w-[180px]";
    default:
      return "aspect-[3/4] w-full";
  }
}

function placeholderClass(layout: Layout): string {
  if (layout === "thumb") return "h-14 w-10 shrink-0";
  if (layout === "detail") return "aspect-[3/4] w-full max-w-[180px]";
  return "aspect-[3/4] w-full";
}
