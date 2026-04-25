import { writable } from "svelte/store";
import type { DownloadState, SSEEventType, SSEPayload } from "./types.js";

/** Map of infoHash → download state, kept live via SSE. */
export const downloads = writable<Record<string, DownloadState>>({});

type Callback<T = SSEPayload> = (payload: T) => void;

const listeners = new Map<SSEEventType, Set<Callback>>();

/** Register a callback for a specific SSE event type. Returns an unsubscribe fn. */
export function onEvent<T = SSEPayload>(
  type: SSEEventType,
  cb: Callback<T>,
): () => void {
  if (!listeners.has(type)) listeners.set(type, new Set());
  listeners.get(type)!.add(cb as Callback);
  return () => listeners.get(type)?.delete(cb as Callback);
}

function dispatch(type: SSEEventType, payload: SSEPayload): void {
  listeners.get(type)?.forEach((cb) => cb(payload));
}

let es: EventSource | null = null;

function connect(): void {
  if (es) return;
  es = new EventSource("/api/events");

  es.addEventListener("download_progress", (e: MessageEvent) => {
    const d = JSON.parse(e.data);
    downloads.update((m) => ({
      ...m,
      [d.info_hash]: { ...m[d.info_hash], ...d },
    }));
    dispatch("download_progress", d);
  });

  es.addEventListener("download_status_changed", (e: MessageEvent) => {
    const d = JSON.parse(e.data);
    downloads.update((m) => ({
      ...m,
      [d.info_hash]: { ...m[d.info_hash], ...d },
    }));
    dispatch("download_status_changed", d);
  });

  es.addEventListener("release_detected", (e: MessageEvent) => {
    dispatch("release_detected", JSON.parse(e.data));
  });

  es.addEventListener("series_updated", (e: MessageEvent) => {
    dispatch("series_updated", JSON.parse(e.data));
  });

  es.onerror = () => {
    es?.close();
    es = null;
    setTimeout(connect, 2000);
  };
}

/** Call once at app startup (e.g. in root +layout.svelte). */
export function startSSE(): void {
  if (typeof window !== "undefined") connect();
}
