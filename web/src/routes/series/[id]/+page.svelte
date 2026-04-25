<script lang="ts">
  import { page } from "$app/stores";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Progress } from "$lib/components/ui/progress/index.js";
  import {
    ResizableHandle,
    ResizablePane,
    ResizablePaneGroup,
  } from "$lib/components/ui/resizable/index.js";
  import { Skeleton } from "$lib/components/ui/skeleton/index.js";
  import { onEvent } from "$lib/sse.js";
  import type {
    AiringSlot,
    DownloadProgress,
    FollowState,
    Release,
    Series,
    SeriesDetailResponse,
  } from "$lib/types.js";
  import { onMount } from "svelte";
  import { Check, ExternalLink, Star, LoaderCircle, Play, X } from "@lucide/svelte";

  let series = $state<Series | null>(null);
  let schedule = $state<AiringSlot[]>([]);
  let releases = $state<Release[]>([]);
  let stats = $state<SeriesDetailResponse["stats"]>(null);
  let loading = $state(true);
  let error = $state("");

  let editing = $state(false);
  let editGroups = $state("");
  let editAliases = $state("");
  let saving = $state(false);
  let saveError = $state("");

  let toast = $state("");

  const id = $derived(parseInt($page.params["id"] ?? "0"));

  onMount(() => {
    (async () => {
      const res = await fetch(`/api/series/${id}`);
      if (!res.ok) {
        error = "Série introuvable.";
        loading = false;
        return;
      }
      const data: SeriesDetailResponse = await res.json();
      series = data.series;
      schedule = data.airing_schedule ?? [];
      releases = data.recent_releases ?? [];
      stats = data.stats ?? null;
      editGroups = (series.preferred_groups ?? []).join(", ");
      editAliases = (series.aliases ?? []).join(", ");
      loading = false;
    })();

    const unsubProgress = onEvent(
      "download_progress",
      (d: DownloadProgress) => {
        const p = d as DownloadProgress;
        releases = releases.map((r) =>
          r.info_hash === p.info_hash ? { ...r, progress: p.progress } : r,
        );
      },
    );
    const unsubStatus = onEvent("download_status_changed", async () => {
      const r = await fetch(`/api/series/${id}`);
      if (r.ok) {
        const d: SeriesDetailResponse = await r.json();
        releases = d.recent_releases ?? [];
        stats = d.stats;
      }
    });
    return () => {
      unsubProgress();
      unsubStatus();
    };
  });

  

  function title(s: Series | null): string {
    return s?.title_english || s?.title_romaji || "";
  }

  function fmtShortDate(iso: string | null | undefined): string {
    if (!iso) return "";
    return new Date(iso).toLocaleDateString("fr-FR", {
      day: "numeric",
      month: "short",
    });
  }

  function fmtDateTime(iso: string | null | undefined): string {
    if (!iso) return "";
    return new Date(iso).toLocaleString("fr-FR", {
      weekday: "short",
      day: "numeric",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function fmtBytes(b: number | null | undefined): string | null {
    if (!b) return null;
    if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
    if (b >= 1e6) return (b / 1e6).toFixed(0) + " MB";
    return (b / 1e3).toFixed(0) + " KB";
  }

  function sourceFR(src: string | null | undefined): string {
    const map: Record<string, string> = {
      MANGA: "Manga",
      LIGHT_NOVEL: "Light Novel",
      ORIGINAL: "Original",
      WEB_NOVEL: "Web Novel",
      NOVEL: "Roman",
      VIDEO_GAME: "Jeu vidéo",
      VISUAL_NOVEL: "Visual Novel",
      COMIC: "Comic",
      DOUJINSHI: "Doujinshi",
      ANIME: "Anime",
      OTHER: "Autre",
    };
    return src && map[src] ? map[src]! : (src ?? "—");
  }

  function airDayTime(slots: AiringSlot[]): string | null {
    if (!slots || slots.length === 0) return null;
    const first = new Date(slots[0]!.airing_at);
    const dayFR = ["Dim", "Lun", "Mar", "Mer", "Jeu", "Ven", "Sam"];
    const h = first.getHours().toString().padStart(2, "0");
    const m = first.getMinutes().toString().padStart(2, "0");
    return `${dayFR[first.getDay()]} ${h}h${m}`;
  }

  

  interface EpisodeRow {
    episode: number;
    airing_at: string | null;
    rel: Release | null;
  }

  const episodes = $derived.by((): EpisodeRow[] => {
    const relByEp = new Map<number, Release>();
    for (const r of releases) {
      if (r.episode != null) relByEp.set(r.episode, r);
    }
    const fromSched: EpisodeRow[] = schedule.map((slot) => ({
      episode: slot.episode,
      airing_at: slot.airing_at,
      rel: relByEp.get(slot.episode) ?? null,
    }));
    const schedEps = new Set(schedule.map((s) => s.episode));
    const extra: EpisodeRow[] = releases
      .filter((r) => r.episode != null && !schedEps.has(r.episode!))
      .map((r) => ({ episode: r.episode!, airing_at: null, rel: r }));
    return [...fromSched, ...extra].sort((a, b) => a.episode - b.episode);
  });

  function copyStream(rel: Release): void {
    navigator.clipboard.writeText(
      window.location.origin + (rel.stream_url ?? `/stream/${rel.info_hash}`),
    );
    toast = "URL copiée !";
    setTimeout(() => (toast = ""), 2000);
  }

  

  async function setFollow(state: FollowState): Promise<void> {
    const action = state === 1 ? "follow" : state === 2 ? "archive" : "ignore";
    await fetch(`/api/series/${id}/${action}`, { method: "POST" });
    if (series) series = { ...series, follow_state: state };
  }

  

  async function saveEdits(): Promise<void> {
    saving = true;
    saveError = "";
    const body = {
      preferred_groups: editGroups
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      aliases: editAliases
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    };
    const res = await fetch(`/api/series/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (res.ok) {
      series = await res.json();
      editGroups = (series!.preferred_groups ?? []).join(", ");
      editAliases = (series!.aliases ?? []).join(", ");
      editing = false;
    } else {
      saveError = "Erreur lors de la sauvegarde.";
    }
    saving = false;
  }

  const statusColors: Record<string, string> = {
    RELEASING: "text-emerald-400",
    FINISHED: "text-muted-foreground",
    NOT_YET_RELEASED: "text-blue-400",
    CANCELLED: "text-destructive",
    HIATUS: "text-amber-400",
  };

  const statusFR: Record<string, string> = {
    RELEASING: "En cours",
    FINISHED: "Terminé",
    NOT_YET_RELEASED: "À venir",
    CANCELLED: "Annulé",
    HIATUS: "En pause",
  };
</script>

<!-- Toast -->
{#if toast}
  <div
    class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 rounded-lg border border-primary/30 bg-primary/10 px-4 py-2 text-sm text-primary shadow-lg"
  >
    {toast}
  </div>
{/if}

<div class="flex flex-col h-full page-enter overflow-hidden">
  {#if loading}
    <!-- Skeleton -->
    <div class="shrink-0 h-24 bg-muted/30"></div>
    <div class="p-5 space-y-4">
      <div class="flex gap-4">
        <Skeleton class="h-16 w-11 shrink-0 rounded-lg" />
        <div class="flex-1 space-y-2">
          <Skeleton class="h-5 w-2/3 rounded" />
          <Skeleton class="h-3.5 w-1/3 rounded" />
          <div class="flex gap-2 mt-3">
            {#each { length: 3 } as _}
              <Skeleton class="h-7 w-16 rounded-md" />
            {/each}
          </div>
        </div>
      </div>
    </div>
  {:else if error}
    <div class="flex flex-1 items-center justify-center">
      <p class="text-sm text-destructive">{error}</p>
    </div>
  {:else if series}
    <!-- ── Hero ───────────────────────────────────────────────────────────── -->
    <div
      class="shrink-0 border-b border-border"
      style="background: var(--sidebar)"
    >
      <!-- Gradient banner with JP title watermark -->
      <div
        class="relative h-24 overflow-hidden"
        style="background: linear-gradient(135deg,
					oklch(0.18 0.09 {(series.id * 37) % 360}) 0%,
					oklch(0.12 0.06 {(series.id * 37 + 50) % 360}) 60%,
					oklch(0.09 0.03 {(series.id * 37 + 110) % 360}) 100%)"
      >
        <div
          class="absolute inset-0"
          style="background: radial-gradient(ellipse at 20% 50%, oklch(1 0 0 / 6%) 0%, transparent 60%)"
        ></div>
        <div
          class="absolute inset-0 flex items-center justify-end pr-6 overflow-hidden"
          style="opacity: 0.07"
        >
          <span
            class="font-display text-6xl font-extrabold text-white whitespace-nowrap"
          >
            {series.title_native ?? series.title_romaji ?? ""}
          </span>
        </div>
      </div>
      <div
        class="absolute h-12 left-0 right-0"
        style="margin-top: -48px; position: relative; background: linear-gradient(transparent, var(--sidebar))"
      ></div>

      <!-- Cover + info -->
      <div
        class="flex gap-4 px-5 pb-4 items-end"
        style="margin-top: -36px; position: relative; z-index: 1;"
      >
        {#if series.cover_url}
          <img
            src={series.cover_url}
            alt={title(series)}
            class="h-[84px] w-[60px] shrink-0 rounded-lg object-cover shadow-xl"
            style="outline: 3px solid var(--sidebar)"
          />
        {:else}
          <div
            class="h-[84px] w-[60px] shrink-0 rounded-lg bg-muted shadow-xl"
            style="outline: 3px solid var(--sidebar)"
          ></div>
        {/if}

        <div class="flex-1 min-w-0 pb-0.5">
          <h1 class="font-display text-base font-bold leading-tight truncate">
            {title(series)}
          </h1>
          <p class="text-xs text-muted-foreground mt-0.5 truncate">
            {[
              series.title_romaji !== title(series)
                ? series.title_romaji
                : null,
              series.season_formatted,
              series.total_episodes ? series.total_episodes + " éps" : null,
              series.studio,
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
          <div class="mt-2 flex flex-wrap gap-1.5 items-center">
            <!-- Follow state buttons — one clear action per state -->
            {#if series.follow_state === 1}
              <Button size="sm" variant="secondary" onclick={() => setFollow(0)} class="gap-1.5">
                <Check size={13} strokeWidth={2.5} /> Suivi
              </Button>
              <Button size="sm" variant="ghost" onclick={() => setFollow(2)}>Archiver</Button>
            {:else if series.follow_state === 2}
              <Button size="sm" onclick={() => setFollow(1)}>+ Suivre</Button>
              <Button size="sm" variant="secondary" onclick={() => setFollow(0)}>Archivé</Button>
            {:else}
              <Button size="sm" onclick={() => setFollow(1)}>+ Suivre</Button>
              <Button size="sm" variant="ghost" onclick={() => setFollow(2)}>Archiver</Button>
            {/if}
            {#if series.anilist_url}
              <Button size="sm" variant="ghost" href={series.anilist_url} target="_blank" rel="noopener" class="gap-1">
                AniList <ExternalLink size={12} />
              </Button>
            {/if}
          </div>
        </div>

        <!-- Score -->
        {#if series.score_anilist}
          <div class="shrink-0 text-center pb-1">
            <div
              class="font-display text-2xl font-extrabold leading-none"
              style="color: var(--color-primary)"
            >
              {series.score_anilist.toFixed(1)}
            </div>
            <div class="inline-flex items-center gap-0.5 text-xs text-muted-foreground mt-0.5">
              <Star size={10} fill="currentColor" strokeWidth={0} /> AniList
            </div>
          </div>
        {/if}
      </div>
    </div>

    <!-- ── 3-column body ──────────────────────────────────────────────────── -->
    <ResizablePaneGroup
      direction="horizontal"
      class="flex-1 overflow-hidden min-h-0"
    >
      <!-- ── Col 1 : AniList info ───────────────────────────────────────── -->
      <ResizablePane defaultSize={22} minSize={15} class="hidden lg:flex">
        <div
          class="flex flex-col gap-4 border-r border-border overflow-y-auto px-4 py-3 w-full"
        >
          <!-- Synopsis -->
          {#if series.description}
            <div>
              <p
                class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
              >
                Synopsis
              </p>
              <p
                class="text-sm leading-relaxed text-muted-foreground"
                style="text-wrap: pretty"
              >
                {series.description
                  .replace(/<[^>]+>/g, "")
                  .slice(0, 320)}{series.description.length > 320 ? "…" : ""}
              </p>
            </div>
          {/if}

          <!-- Informations -->
          <div>
            <p
              class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
            >
              Informations
            </p>
            <div class="space-y-1.5">
              {#each [["Studio", series.studio], ["Saison", series.season_formatted], ["Épisodes", series.total_episodes ? series.total_episodes + " × 23 min" : null], ["Statut", statusFR[series.status] ?? series.status], ["Diffusion", airDayTime(schedule)], ["Score", series.score_anilist ? series.score_anilist.toFixed(1) + " / 10 ★" : null], ["Source", sourceFR(series.source)]].filter(([, v]) => v) as [label, value]}
                <div class="flex justify-between gap-2">
                  <span class="text-xs text-muted-foreground shrink-0"
                    >{label}</span
                  >
                  <span
                    class="text-xs text-right truncate
									{label === 'Statut' ? (statusColors[series.status] ?? '') : ''}
									{label === 'Score' ? 'text-primary font-semibold' : ''}
								">{value}</span
                  >
                </div>
              {/each}
            </div>
          </div>

          <!-- Genres -->
          {#if (series.genres?.length ?? 0) > 0}
            <div>
              <p
                class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
              >
                Genres
              </p>
              <div class="flex flex-wrap gap-1">
                {#each series.genres as g}
                  <span
                    class="text-xs rounded px-1.5 py-0.5 border border-border text-muted-foreground"
                    style="background: var(--card2)">{g}</span
                  >
                {/each}
              </div>
            </div>
          {/if}

          <!-- Personnages -->
          {#if (series.characters?.length ?? 0) > 0}
            <div>
              <p
                class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
              >
                Personnages
              </p>
              <div class="space-y-2">
                {#each series.characters as c}
                  <div class="flex items-center gap-2">
                    <div
                      class="h-6 w-6 shrink-0 rounded-full"
                      style="background: linear-gradient(135deg, oklch(0.25 0.07 {(series.id *
                        37) %
                        360}) 0%, oklch(0.18 0.05 {(series.id * 37 + 60) %
                        360}) 100%)"
                    ></div>
                    <div>
                      <p class="text-sm font-medium leading-none">{c.name}</p>
                      {#if c.va}<p class="text-xs text-muted-foreground mt-0.5">
                          {c.va}
                        </p>{/if}
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Relations -->
          {#if (series.relations?.length ?? 0) > 0}
            <div>
              <p
                class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
              >
                Relations
              </p>
              <div class="space-y-1.5">
                {#each (series.relations ?? []).filter( (r) => ["PREQUEL", "SEQUEL", "SIDE_STORY", "PARENT", "ALTERNATIVE"].includes(r.type), ) as rel}
                  {@const typeLabel: Record<string, string> = { PREQUEL: 'Préquelle', SEQUEL: 'Suite', SIDE_STORY: 'Spin-off', PARENT: 'Série mère', ALTERNATIVE: 'Alternative' }}
                  <div class="flex items-center gap-2">
                    <Badge
                      variant="ghost"
                      class="text-xs shrink-0 h-5 px-1.5"
                      style="background: var(--blue-lo); color: var(--blue); border-color: transparent"
                    >
                      {typeLabel[rel.type] ?? rel.type}
                    </Badge>
                    <span class="text-xs text-muted-foreground truncate"
                      >{rel.title}</span
                    >
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </ResizablePane>
      <ResizableHandle withHandle />

      <!-- ── Col 2 : Episodes ────────────────────────────────────────────── -->
      <ResizablePane defaultSize={56} minSize={30} class="overflow-y-auto">
        <div
          class="overflow-y-auto h-full px-4 py-3 flex flex-col gap-2 min-w-0"
        >
          <div class="flex items-baseline gap-2 mb-1">
            <p
              class="text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
            >
              Épisodes
            </p>
            {#if series.total_episodes}
              <span class="text-xs text-muted-foreground">
                · {releases.filter((r) => r.status === "completed")
                  .length}/{series.total_episodes} téléchargés
              </span>
            {/if}
          </div>

          {#if episodes.length === 0}
            <div class="flex items-center justify-center py-12">
              <p class="text-sm text-muted-foreground">
                Aucun épisode dans le calendrier.
              </p>
            </div>
          {:else}
            {#each episodes as ep}
              {@const rel = ep.rel}
              {@const done = rel?.status === "completed"}
              {@const active = rel?.status === "downloading"}
              {@const queued = rel?.status === "queued"}
              {@const failed = rel?.status === "failed"}
              <div
                class="flex items-center gap-2.5 rounded-lg border px-3 py-2.5 transition-colors hover:bg-accent/30"
                style={done
                  ? "background: var(--green-lo); border-color: var(--green-lo)"
                  : "border-color: var(--border)"}
              >
                <!-- Ep number bubble -->
                <div
                  class="h-7 w-7 shrink-0 rounded-full flex items-center justify-center text-xs font-bold"
                  style="background: {done
                    ? 'oklch(0.72 0.14 155 / 18%)'
                    : active
                      ? 'var(--violet-lo)'
                      : 'oklch(1 0 0 / 5%)'};
								       color: {done
                    ? 'var(--green)'
                    : active
                      ? 'var(--color-primary)'
                      : 'var(--dim)'}"
                >
                  {String(ep.episode).padStart(2, "0")}
                </div>

                <!-- Info -->
                <div class="flex-1 min-w-0">
                  {#if rel}
                    <p class="text-sm font-medium">Épisode {ep.episode}</p>
                  {:else if ep.airing_at}
                    <p class="text-xs text-muted-foreground">
                      Diffusion : {fmtDateTime(ep.airing_at)}
                    </p>
                  {:else}
                    <p class="text-xs text-muted-foreground">
                      Épisode {ep.episode}
                    </p>
                  {/if}
                  {#if active}
                    <Progress
                      value={(rel.progress ?? 0) * 100}
                      class="mt-1.5 h-1"
                    />
                    <p
                      class="mt-0.5 text-xs tabular-nums"
                      style="color: var(--color-primary)"
                    >
                      {((rel.progress ?? 0) * 100).toFixed(0)}% · En cours…
                    </p>
                  {:else if done}
                    <p class="text-xs" style="color: var(--green)">
                      Téléchargé
                    </p>
                  {:else if queued}
                    <p class="text-xs text-muted-foreground">
                      En file d'attente
                    </p>
                  {:else if failed}
                    <p class="text-xs text-destructive">
                      Échec de téléchargement
                    </p>
                  {/if}
                </div>

                <!-- Actions -->
                {#if done || active}
                  <div class="flex items-center gap-1.5 shrink-0">
                    {#if done}
                      <a
                        href="vlc:{rel.stream_url ?? `/stream/${rel.info_hash}`}"
                        target="_blank"
                        class="inline-flex items-center h-7 px-2.5 rounded text-xs font-medium no-underline"
                        style="background: var(--green-lo); border: 1px solid var(--green-lo); color: var(--green)"
                      >
                        <Play size={11} fill="currentColor" strokeWidth={0} /> VLC
                      </a>
                    {/if}
                    <button
                      onclick={() => copyStream(rel)}
                      class="inline-flex items-center h-7 px-2.5 rounded text-xs font-medium"
                      style="background: var(--card2); border: 1px solid var(--border); color: var(--color-muted-foreground)"
                    >
                      URL
                    </button>
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        </div>
      </ResizablePane>
      <ResizableHandle withHandle />

      <!-- ── Col 3 : Match rules + stats ───────────────────────────────────── -->
      <ResizablePane defaultSize={22} minSize={14} class="hidden lg:flex">
        <div
          class="flex flex-col gap-4 border-l border-border overflow-y-auto px-3.5 py-3 w-full"
        >
          <!-- Correspondance Nyaa -->
          <div>
            <p
              class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
            >
              Correspondance Nyaa
            </p>
            <div
              class="rounded-lg border border-border px-3 py-2.5 flex flex-col gap-2.5"
              style="background: var(--card)"
            >
              {#each [["Groupes", editGroups || "Défauts globaux"], ["Alias", editAliases || "Aucun"]] as [k, v]}
                <div>
                  <p class="text-xs mb-0.5 text-muted-foreground">{k}</p>
                  <p class="text-sm truncate">{v}</p>
                </div>
              {/each}
              {#if !editing}
                <button
                  onclick={() => (editing = true)}
                  class="mt-1 w-full h-7 rounded text-xs transition-colors"
                  style="background: var(--card2); border: 1px solid var(--border); color: var(--color-muted-foreground)"
                >
                  Modifier
                </button>
              {:else}
                <div class="flex flex-col gap-2">
                  <div>
                    <p class="text-xs mb-1 text-muted-foreground">Groupes</p>
                    <Input
                      bind:value={editGroups}
                      placeholder="SubsPlease, Erai-raws"
                      class="h-8 text-xs"
                    />
                  </div>
                  <div>
                    <p class="text-xs mb-1 text-muted-foreground">Alias</p>
                    <Input
                      bind:value={editAliases}
                      placeholder="Autre titre…"
                      class="h-8 text-xs"
                    />
                  </div>
                  {#if saveError}
                    <p class="text-xs text-destructive">{saveError}</p>
                  {/if}
                  <div class="flex gap-1">
                    <Button
                      size="sm"
                      class="flex-1"
                      onclick={saveEdits}
                      disabled={saving}
                    >
                      {#if saving}<LoaderCircle size={12} class="animate-spin" />{:else}OK{/if}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      class="flex-1"
                      onclick={() => {
                        editing = false;
                        saveError = "";
                      }}
                    >
                      <X size={14} />
                    </Button>
                  </div>
                </div>
              {/if}
            </div>
          </div>

          <!-- Statistiques -->
          {#if stats}
            <div>
              <p
                class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
              >
                Statistiques
              </p>
              <div
                class="rounded-lg border border-border px-3 py-2.5"
                style="background: var(--card)"
              >
                {#each [["DL total", stats.dl_count > 0 ? stats.dl_count + " fichier" + (stats.dl_count > 1 ? "s" : "") : "—"], ["Taille totale", fmtBytes(stats.total_bytes) ?? "—"], ["Premier DL", stats.first_dl ? fmtShortDate(stats.first_dl) : "—"], ["Dernier DL", stats.last_dl ? fmtShortDate(stats.last_dl) : "—"]] as [k, v]}
                  <div class="flex justify-between mb-1.5 last:mb-0">
                    <span class="text-xs text-muted-foreground">{k}</span>
                    <span class="text-xs tabular-nums">{v}</span>
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Dossier -->
          <div>
            <p
              class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
            >
              Dossier
            </p>
            <div
              class="rounded-lg border border-border px-3 py-2.5"
              style="background: var(--card)"
            >
              <p class="text-xs mb-1 text-muted-foreground">Chemin local</p>
              <p class="text-xs break-all leading-relaxed">
                /media/{title(series)
                  .replace(/[<>:"/\\|?*]/g, "")
                  .slice(0, 40)}/
              </p>
            </div>
          </div>
        </div>
      </ResizablePane>
    </ResizablePaneGroup>

    <!-- ── Mobile: synopsis + info accordion (below cols, only visible < lg) ── -->
    <div
      class="lg:hidden shrink-0 border-t border-border overflow-y-auto px-4 py-3 space-y-4"
    >
      {#if series.description}
        <div>
          <p
            class="mb-1.5 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
          >
            Synopsis
          </p>
          <p class="text-sm text-muted-foreground leading-relaxed">
            {series.description.replace(/<[^>]+>/g, "").slice(0, 280)}…
          </p>
        </div>
      {/if}
      {#if (series.genres?.length ?? 0) > 0}
        <div class="flex flex-wrap gap-1">
          {#each series.genres as g}
            <span
              class="text-xs rounded px-2 py-0.5 border border-border text-muted-foreground"
              style="background: var(--card2)">{g}</span
            >
          {/each}
        </div>
      {/if}
      <!-- Match rules on mobile -->
      <Card.Root>
        <Card.Header class="flex flex-row items-center justify-between pb-2">
          <Card.Title class="font-display text-sm"
            >Règles de correspondance</Card.Title
          >
          {#if !editing}
            <Button size="sm" variant="outline" onclick={() => (editing = true)}
              >Modifier</Button
            >
          {/if}
        </Card.Header>
        <Card.Content class="space-y-2">
          {#if editing}
            <div>
              <label class="mb-1 block text-xs font-medium" for="groups-m"
                >Groupes préférés</label
              >
              <Input
                id="groups-m"
                bind:value={editGroups}
                placeholder="SubsPlease, Erai-raws"
                class="h-8"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium" for="aliases-m"
                >Alias</label
              >
              <Input
                id="aliases-m"
                bind:value={editAliases}
                placeholder="Autre titre…"
                class="h-8"
              />
            </div>
            {#if saveError}<p class="text-xs text-destructive">
                {saveError}
              </p>{/if}
            <div class="flex gap-2">
              <Button size="sm" onclick={saveEdits} disabled={saving}
                >{saving ? "…" : "Sauvegarder"}</Button
              >
              <Button
                size="sm"
                variant="outline"
                onclick={() => {
                  editing = false;
                  saveError = "";
                }}>Annuler</Button
              >
            </div>
          {:else}
            <p class="text-sm">
              <span class="font-medium">Groupes :</span>
              <span class="text-muted-foreground"
                >{editGroups || "Défauts globaux"}</span
              >
            </p>
            <p class="text-sm">
              <span class="font-medium">Alias :</span>
              <span class="text-muted-foreground">{editAliases || "Aucun"}</span
              >
            </p>
          {/if}
        </Card.Content>
      </Card.Root>
    </div>
  {/if}
</div>
