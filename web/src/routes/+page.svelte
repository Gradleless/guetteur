<script lang="ts">
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Progress } from "$lib/components/ui/progress/index.js";
  import { Skeleton } from "$lib/components/ui/skeleton/index.js";
  import * as m from "$lib/paraglide/messages.js";
  import { getLocale } from "$lib/paraglide/runtime.js";
  import { onEvent } from "$lib/sse.js";
  import type {
    DownloadProgress,
    DownloadState,
    ScheduleEntry,
  } from "$lib/types.js";
  import { ArrowRight, Check, Play } from "@lucide/svelte";
  import { onMount } from "svelte";
  import { toast } from "svelte-sonner";

  let schedule = $state<ScheduleEntry[]>([]);
  let activeDownloads = $state<DownloadState[]>([]);
  let loading = $state(true);

  onMount(() => {
    (async () => {
      const lastDay = weekDays[6]!;
      const firstDay = weekDays[0]!;
      const weekEnd = new Date(lastDay);
      weekEnd.setDate(weekEnd.getDate() + 1);
      const scheduleURL = `/api/schedule?from=${firstDay.toISOString()}&to=${weekEnd.toISOString()}`;

      const [sRes, dRes] = await Promise.all([
        fetch(scheduleURL),
        fetch("/api/downloads"),
      ]);
      schedule = sRes.ok ? await sRes.json() : [];
      activeDownloads = dRes.ok ? await dRes.json() : [];
      loading = false;
    })();

    const unsubProgress = onEvent(
      "download_progress",
      (d: DownloadProgress) => {
        const p = d as DownloadProgress;
        activeDownloads = activeDownloads.map((dl) =>
          dl.info_hash === p.info_hash
            ? { ...dl, progress: p.progress, speed_bps: p.speed_bps }
            : dl,
        );
      },
    );
    const unsubStatus = onEvent("download_status_changed", async () => {
      const res = await fetch("/api/downloads");
      if (res.ok) activeDownloads = await res.json();
    });
    const unsubRelease = onEvent("release_detected", async () => {
      const res = await fetch("/api/downloads");
      if (res.ok) activeDownloads = await res.json();
    });
    return () => {
      unsubProgress();
      unsubStatus();
      unsubRelease();
    };
  });

  function getWeekDays(): Date[] {
    const today = new Date();
    const monday = new Date(today);
    monday.setDate(today.getDate() - ((today.getDay() + 6) % 7));
    monday.setHours(0, 0, 0, 0);
    return Array.from({ length: 7 }, (_, i) => {
      const d = new Date(monday);
      d.setDate(monday.getDate() + i);
      return d;
    });
  }

  const weekDays = getWeekDays();
  const todayStr = new Date().toDateString();
  let selectedIdx = $state(
    weekDays.findIndex((d) => d.toDateString() === todayStr) ?? 0,
  );

  const DAY_SHORT = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(2024, 0, 8 + i); // Jan 8 2024 = Monday
    return new Intl.DateTimeFormat(getLocale(), { weekday: "short" }).format(d);
  });

  const selectedDay = $derived(weekDays[selectedIdx] ?? weekDays[0]!);

  const todayLabel = new Date().toLocaleDateString(getLocale(), {
    weekday: "long",
    day: "numeric",
    month: "long",
  });

  let dayEpisodes = $derived(
    schedule
      .filter(
        (ep) =>
          new Date(ep.airing_at).toDateString() === selectedDay.toDateString(),
      )
      .sort(
        (a, b) =>
          new Date(a.airing_at).getTime() - new Date(b.airing_at).getTime(),
      ),
  );

  function countForDay(d: Date): number {
    return schedule.filter(
      (ep) => new Date(ep.airing_at).toDateString() === d.toDateString(),
    ).length;
  }

  function fmtTime(iso: string): string {
    return new Date(iso).toLocaleTimeString(getLocale(), {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  const inProgress = $derived(
    activeDownloads.filter(
      (d) => d.status === "downloading" || d.status === "queued",
    ),
  );

  function dlForEp(ep: ScheduleEntry): DownloadState | undefined {
    return activeDownloads.find(
      (d) => d.series_id === ep.series_id && d.episode === ep.episode,
    );
  }

  function fmtSpeed(bps: number | undefined): string | null {
    if (!bps || bps <= 0) return null;
    if (bps >= 1_000_000) return (bps / 1_000_000).toFixed(1) + " MB/s";
    if (bps >= 1_000) return (bps / 1_000).toFixed(0) + " KB/s";
    return bps + " B/s";
  }

  function copyURL(dl: DownloadState): void {
    navigator.clipboard.writeText(
      window.location.origin + (dl.stream_url ?? `/stream/${dl.info_hash}`),
    );
    toast.success(m.toast_url_copied());
  }
</script>

<div class="flex flex-col h-full page-enter">
  <!-- Header -->
  <div class="shrink-0 border-b border-border px-5 py-3.5">
    <p class="font-display text-lg font-bold leading-none">
      {m.nav_dashboard()}
    </p>
    <p class="mt-0.5 text-xs text-muted-foreground">
      {todayLabel}{#if !loading && dayEpisodes.length > 0}&nbsp;{m.dashboard_episodes_today({ count: dayEpisodes.length, s: dayEpisodes.length > 1 ? "s" : "" })}{/if}
    </p>
  </div>

  <!-- Body -->
  <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4">
    {#if loading}
      <div class="flex gap-1.5">
        {#each { length: 7 } as _}
          <Skeleton class="flex-1 h-14 rounded-lg" />
        {/each}
      </div>
      <div class="space-y-2">
        {#each { length: 3 } as _}
          <Skeleton class="h-16 w-full rounded-xl" />
        {/each}
      </div>
    {:else}
      <!-- Week strip -->
      <div class="flex gap-1.5">
        {#each weekDays as day, i}
          {@const count = countForDay(day)}
          {@const isToday = day.toDateString() === todayStr}
          {@const active = selectedIdx === i}
          <button
            onclick={() => (selectedIdx = i)}
            class="relative flex flex-1 flex-col items-center gap-0 rounded-lg border px-1 py-2 text-center transition-colors duration-[120ms]"
            class:bg-primary={active}
            class:border-primary={active}
            class:text-primary-foreground={active}
            class:border-border={!active && !isToday}
            class:hover:bg-accent={!active}
            style={!active && isToday ? "border-color: var(--violet-mid)" : ""}
          >
            <span
              class="text-xs font-bold uppercase tracking-[0.5px]"
              class:text-primary-foreground={active}
              class:text-primary={!active && isToday}
              class:text-muted-foreground={!active && !isToday}
              >{DAY_SHORT[i]}</span
            >
            <span
              class="font-display text-lg font-extrabold leading-none"
              class:text-primary-foreground={active}>{day.getDate()}</span
            >
            <span
              class="h-1 w-1 rounded-full"
              class:bg-primary={isToday && !active}
              class:bg-transparent={!(isToday && !active)}
            ></span>
            {#if count > 0 && !active}
              <span
                class="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-primary text-sm font-bold text-primary-foreground"
              >
                {count}
              </span>
            {/if}
          </button>
        {/each}
      </div>

      <!-- Episodes for selected day -->
      <div>
        <p
          class="mb-2 text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
        >
          {selectedDay.toLocaleDateString(getLocale(), {
            weekday: "long",
            day: "numeric",
            month: "long",
          })}
        </p>

        {#if dayEpisodes.length === 0}
          <Card.Root class="py-8">
            <Card.Content
              class="flex flex-col items-center gap-2 p-0 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {m.dashboard_no_episodes()}
              </p>
              {#if schedule.length === 0}
                <Button variant="link" size="sm" href="/seasonal" class="gap-1">
                  {m.btn_follow_series()} <ArrowRight size={13} />
                </Button>
              {/if}
            </Card.Content>
          </Card.Root>
        {:else}
          <Card.Root class="gap-0 py-0 overflow-hidden">
            <ul class="divide-y divide-border">
              {#each dayEpisodes as ep}
                {@const dl = dlForEp(ep)}
                {@const isDone = dl?.status === "completed"}
                {@const isDL = dl?.status === "downloading"}
                <li
                  class="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-accent/40
									{isDone ? 'bg-emerald-500/5' : ''}"
                >
                  <!-- Cover -->
                  <a href="/series/{ep.series_id ?? ''}" class="shrink-0">
                    {#if ep.cover_url}
                      <img
                        src={ep.cover_url}
                        alt=""
                        class="h-12 w-8 rounded object-cover"
                      />
                    {:else}
                      <div class="h-12 w-8 rounded bg-muted"></div>
                    {/if}
                  </a>

                  <!-- Info -->
                  <div class="min-w-0 flex-1">
                    <a
                      href="/series/{ep.series_id ?? ''}"
                      class="block truncate text-sm font-semibold no-underline hover:underline"
                    >
                      {ep.title}
                    </a>
                    <p class="text-xs text-muted-foreground">
                      {m.lbl_episode_n({ n: ep.episode })} · {fmtTime(ep.airing_at)}
                    </p>
                    {#if isDL}
                      <Progress
                        value={(dl.progress ?? 0) * 100}
                        class="mt-1.5 h-1"
                      />
                      <p
                        class="mt-0.5 text-xs tabular-nums text-muted-foreground"
                      >
                        {((dl.progress ?? 0) * 100).toFixed(0)}%{fmtSpeed(
                          dl.speed_bps,
                        )
                          ? " · " + fmtSpeed(dl.speed_bps)
                          : ""}
                        {m.eta_in_progress()}
                      </p>
                    {/if}
                  </div>

                  <!-- Status + actions -->
                  <div class="flex shrink-0 items-center gap-1.5">
                    {#if isDone}
                      <a
                        href="vlc:{dl.stream_url ?? `/stream/${dl.info_hash}`}"
                        target="_blank"
                        class="inline-flex items-center h-7 px-2.5 rounded-md text-xs font-medium no-underline transition-opacity hover:opacity-80"
                        style="background: var(--green-lo); border: 1px solid var(--green-lo); color: var(--green)"
                      >
                        <Play size={11} class="shrink-0" fill="currentColor" strokeWidth={0} /> VLC
                      </a>
                      <button
                        onclick={() => copyURL(dl)}
                        class="inline-flex items-center h-7 px-2.5 rounded-md text-xs font-medium transition-opacity hover:opacity-80"
                        style="background: var(--card2); border: 1px solid var(--border); color: var(--color-muted-foreground)"
                      >
                        URL
                      </button>
                      <Badge class="bg-emerald-500/10 text-emerald-400 border-transparent shrink-0"
                        ><Check size={11} /></Badge
                      >
                    {:else if isDL}
                      <button
                        onclick={() => copyURL(dl)}
                        class="inline-flex items-center gap-1 h-7 px-2.5 rounded-md text-xs font-medium transition-opacity hover:opacity-80"
                        style="background: var(--violet-lo); border: 1px solid var(--violet-mid); color: var(--color-primary)"
                      >
                        <Play size={11} class="shrink-0" fill="currentColor" strokeWidth={0} /> Stream
                      </button>
                      <Badge class="shrink-0"
                        >{((dl.progress ?? 0) * 100).toFixed(0)}%</Badge
                      >
                    {:else}
                      <span
                        class="text-xs px-2.5 py-1 rounded-md"
                        style="background: oklch(1 0 0 / 5%); color: var(--color-muted-foreground)"
                      >
                        {fmtTime(ep.airing_at)}
                      </span>
                    {/if}
                  </div>
                </li>
              {/each}
            </ul>
          </Card.Root>
        {/if}
      </div>

      <!-- Active downloads -->
      {#if inProgress.length > 0}
        <div>
          <div class="mb-2 flex items-center justify-between">
            <p
              class="text-xs font-bold uppercase tracking-[0.9px] text-muted-foreground"
            >
              {m.dashboard_active_downloads()}
            </p>
            <Button
              variant="link"
              size="sm"
              href="/downloads"
              class="h-auto p-0 text-xs gap-0.5"
              >{m.btn_see_all()} <ArrowRight size={12} /></Button
            >
          </div>
          <Card.Root class="gap-0 py-0 overflow-hidden">
            <ul class="divide-y divide-border">
              {#each inProgress as dl}
                <li class="px-4 py-3">
                  <div class="flex items-center justify-between gap-2 mb-1.5">
                    <p class="truncate text-sm font-medium">
                      {dl.series_title ?? dl.raw_title}
                    </p>
                    {#if dl.status === "queued"}
                      <Badge variant="secondary" class="shrink-0"
                        >{m.status_queued_badge()}</Badge
                      >
                    {:else if dl.status === "downloading"}
                      <span
                        class="shrink-0 tabular-nums text-xs font-semibold"
                        style="color: var(--color-primary)"
                      >
                        {((dl.progress ?? 0) * 100).toFixed(0)}%
                      </span>
                    {/if}
                  </div>
                  {#if dl.status === "downloading"}
                    <div
                      class="relative h-[2.5px] overflow-hidden rounded-full"
                      style="background: oklch(1 0 0 / 7%)"
                    >
                      <div
                        class="h-full rounded-full bg-primary bar-animated transition-all"
                        style="width: {((dl.progress ?? 0) * 100).toFixed(0)}%"
                      ></div>
                    </div>
                    {#if fmtSpeed(dl.speed_bps)}
                      <p
                        class="mt-1 text-xs text-muted-foreground tabular-nums"
                      >
                        {fmtSpeed(dl.speed_bps)}
                      </p>
                    {/if}
                  {/if}
                </li>
              {/each}
            </ul>
          </Card.Root>
        </div>
      {/if}
    {/if}
  </div>
</div>
