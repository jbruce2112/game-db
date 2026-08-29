import { useEffect, useRef, useState } from "react";

type Detector = {
  detect: (source: ImageBitmapSource) => Promise<{ rawValue: string }[]>;
};

function detector(): Detector | null {
  const Ctor = (window as unknown as { BarcodeDetector?: new (opts: { formats: string[] }) => Detector }).BarcodeDetector;
  if (!Ctor) return null;
  try {
    return new Ctor({ formats: ["ean_13", "ean_8", "upc_a", "upc_e"] });
  } catch {
    return null;
  }
}

export default function BarcodeScanner({ onCode, onClose }: { onCode: (code: string) => void; onClose: () => void }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const onCodeRef = useRef(onCode);
  onCodeRef.current = onCode;
  const [error, setError] = useState("");

  useEffect(() => {
    const det = detector();
    if (!det) {
      setError("This browser cannot scan barcodes. Type the digits instead.");
      return;
    }
    let stream: MediaStream | null = null;
    let stop = false;
    const video = videoRef.current;
    (async () => {
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: { ideal: "environment" } },
          audio: false,
        });
        if (!video) return;
        video.srcObject = stream;
        await video.play();
        const loop = async () => {
          if (stop || !video) return;
          try {
            const hits = await det.detect(video);
            const raw = hits[0]?.rawValue?.replace(/\D/g, "");
            if (raw) {
              stop = true;
              onCodeRef.current(raw);
              return;
            }
          } catch {
            /* keep scanning */
          }
          requestAnimationFrame(() => {
            void loop();
          });
        };
        void loop();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Camera permission denied");
      }
    })();
    return () => {
      stop = true;
      stream?.getTracks().forEach((t) => t.stop());
    };
  }, []);

  return (
    <div className="fixed inset-0 z-20 grid place-items-center bg-black/80 p-4">
      <div className="w-full max-w-md overflow-hidden rounded-xl border border-[#2a2e38] bg-[#16181f]">
        <video ref={videoRef} className="aspect-[3/4] w-full bg-black object-cover" muted playsInline />
        <div className="flex items-center justify-between p-3">
          <p className="text-sm text-[#9aa3b2]">{error || "Point at the box barcode"}</p>
          <button onClick={onClose} className="text-sm text-[#e2b14a]">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

export function canScanBarcode() {
  return typeof window !== "undefined" && "BarcodeDetector" in window && !!navigator.mediaDevices?.getUserMedia;
}
