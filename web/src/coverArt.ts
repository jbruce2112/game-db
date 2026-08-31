import { useEffect, useState } from "react";
import type { CoverArt } from "./api";

const KEY = "coverArt";
const EVENT = "coverArt-change";

export function loadCoverArt(): CoverArt {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "box" || v === "poster") return v;
  } catch {
    /* ignore */
  }
  return "poster";
}

export function saveCoverArt(art: CoverArt) {
  try {
    localStorage.setItem(KEY, art);
  } catch {
    /* ignore */
  }
  window.dispatchEvent(new Event(EVENT));
}

export function useCoverArt(): [CoverArt, (art: CoverArt) => void] {
  const [art, setArt] = useState<CoverArt>(loadCoverArt);
  useEffect(() => {
    const onChange = () => setArt(loadCoverArt());
    window.addEventListener(EVENT, onChange);
    window.addEventListener("storage", onChange);
    return () => {
      window.removeEventListener(EVENT, onChange);
      window.removeEventListener("storage", onChange);
    };
  }, []);
  return [
    art,
    (next) => {
      saveCoverArt(next);
      setArt(next);
    },
  ];
}
