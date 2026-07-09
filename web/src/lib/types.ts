export type DownloadStatus =
  | "queued"
  | "downloading"
  | "completed"
  | "failed"
  | "deleted"
  | "skipped"
  | "superseded";

export type AniListStatus =
  | "RELEASING"
  | "FINISHED"
  | "NOT_YET_RELEASED"
  | "CANCELLED"
  | "HIATUS";

/** 0 = ignored, 1 = following, 2 = archived */
export type FollowState = 0 | 1 | 2;

export interface DownloadProgress {
  info_hash: string;
  progress: number;
  bytes_completed: number;
  speed_bps: number;
  seeders: number;
}

export interface DownloadStatusChanged {
  info_hash: string;
  status: DownloadStatus;
}

export interface ReleaseDetected {
  info_hash: string;
  raw_title: string;
  series_id: number;
  episode: number | null;
  resolution: string;
  group: string;
}

export interface SeriesUpdated {
  series_id: number;
}

export type SSEPayload =
  | DownloadProgress
  | DownloadStatusChanged
  | ReleaseDetected
  | SeriesUpdated;

export type SSEEventType =
  | "download_progress"
  | "download_status_changed"
  | "release_detected"
  | "series_updated";

export interface DownloadState {
  info_hash: string;
  series_id?: number;
  series_title?: string;
  raw_title?: string;
  episode?: number | null;
  status: DownloadStatus;
  progress?: number;
  speed_bps?: number;
  stream_url?: string;
}

export interface ScheduleEntry {
  series_id: number;
  title: string;
  episode: number;
  airing_at: string;
  cover_url: string | null;
  follow_state: FollowState;
}

export interface Release {
  info_hash: string;
  series_id?: number;
  raw_title: string;
  episode?: number | null;
  episode_end?: number | null;
  group_name?: string;
  resolution?: string;
  status: DownloadStatus;
  progress?: number;
  speed_bps?: number;
  stream_url?: string;
}

export interface AiringSlot {
  episode: number;
  airing_at: string;
}

export interface Series {
  id: number;
  title_romaji: string;
  title_english?: string;
  title_native?: string;
  description?: string;
  cover_url?: string;
  anilist_url?: string;
  follow_state: FollowState;
  status: AniListStatus;
  total_episodes?: number;
  score_anilist?: number;
  season_formatted?: string;
  studio?: string;
  source?: string;
  genres?: string[];
  characters?: Array<{ name: string; va?: string }>;
  relations?: Array<{ type: string; title: string; id: number }>;
  preferred_groups?: string[];
  aliases?: string[];
}

export interface SeriesDetailResponse {
  series: Series;
  airing_schedule: AiringSlot[];
  recent_releases: Release[];
  stats: {
    dl_count: number;
    total_bytes: number;
    first_dl: string | null;
    last_dl: string | null;
  } | null;
}

export interface HealthResponse {
  ok: boolean;
  version: string;
  uptime_seconds: number;
  vpn_ip: string | null;
  db_size_bytes: number;
  disk_free_bytes: number;
}

export interface SettingsResponse {
  discord_webhook?: string;
  ntfy_topic?: string;
  default_groups?: string;
  preferred_quality?: string;
  media_dir?: string;
}
