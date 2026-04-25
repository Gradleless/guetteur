<script lang="ts">
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Progress } from "$lib/components/ui/progress/index.js";
  import { Skeleton } from "$lib/components/ui/skeleton/index.js";
  import {
    Tabs,
    TabsList,
    TabsTrigger,
  } from "$lib/components/ui/tabs/index.js";
  import type { Series } from "$lib/types.js";
  import { ArrowRight } from "@lucide/svelte";
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";

  
  interface LibrarySeries extends Series {
    downloaded_episodes?: number;
    next_airing?: { episode: number; airing_at: string };
  }

  let allSeries = $state<LibrarySeries[]>([]);
  let loading = $state(true);
  let filter = $state("active");

  async function reload(): Promise<void> {
    loading = true;
    const res = await fetch("/api/series");
    const all: LibrarySeries[] = res.ok ? await res.json() : [];
    allSeries = all.filter((s) => s.follow_state === 1 || s.follow_state === 2);
    loading = false;
  }

  let series = $derived(
    filter === "all"
      ? allSeries
      : allSeries.filter((s) =>
          filter === "active" ? s.follow_state === 1 : s.follow_state === 2,
        ),
  );

  onMount(reload);

  function title(s: LibrarySeries): string {
    return s.title_english || s.title_romaji;
  }
</script>

<div class="flex flex-col h-full page-enter">
  <!-- Header -->
  <div class="shrink-0 border-b border-border px-5 py-3.5 space-y-2.5">
    <div class="flex items-center gap-3">
      <p class="font-display text-lg font-bold leading-none flex-1">
        Bibliothèque
      </p>
      {#if !loading}
        <p class="text-xs text-muted-foreground">
          {series.length} série{series.length > 1 ? "s" : ""}
        </p>
      {/if}
    </div>
    <Tabs bind:value={filter}>
      <TabsList variant="line">
        <TabsTrigger value="active">Suivis</TabsTrigger>
        <TabsTrigger value="archived">Archivés</TabsTrigger>
        <TabsTrigger value="all">Tout</TabsTrigger>
      </TabsList>
    </Tabs>
  </div>

  <!-- Body -->
  <div class="flex-1 overflow-y-auto px-5 py-4">
    {#if loading}
      <div
        class="grid gap-3 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
      >
        {#each { length: 12 } as _}
          <div class="space-y-2">
            <Skeleton class="aspect-[2/3] w-full rounded-lg" />
            <Skeleton class="h-3 w-3/4 rounded" />
            <Skeleton class="h-2 w-1/2 rounded" />
          </div>
        {/each}
      </div>
    {:else if series.length === 0}
      <div class="flex h-40 items-center justify-center">
        <p class="text-sm text-muted-foreground">
          Aucune série ici.
          <Button
            variant="link"
            size="sm"
            href="/seasonal"
            class="p-0 h-auto gap-1"
            >Suivre des séries <ArrowRight size={13} /></Button
          >
        </p>
      </div>
    {:else}
      <ul
        class="grid gap-3 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
      >
        {#each series as s, index (s.id)}
          <li animate:flip>
            <a href="/series/{s.id}" class="block h-full no-underline">
              <Card.Root
                class="h-full gap-0 py-0 overflow-hidden p-0 transition-all hover:ring-primary/30"
              >
                <!-- Cover -->
                <div class="relative aspect-[2/3] bg-muted">
                  {#if s.cover_url}
                    <img
                      src={s.cover_url}
                      alt={title(s)}
                      class="h-full w-full object-cover"
                      loading="lazy"
                    />
                  {:else}
                    <div
                      class="flex h-full items-center justify-center p-2 text-center text-xs text-muted-foreground"
                    >
                      {title(s)}
                    </div>
                  {/if}
                  {#if s.follow_state === 2}
                    <span class="absolute right-2 top-2"
                      ><Badge variant="secondary">Archivé</Badge></span
                    >
                  {/if}
                  <!-- Gradient overlay for progress -->
                  <div
                    class="absolute bottom-0 left-0 right-0 h-8 bg-gradient-to-t from-card to-transparent"
                  ></div>
                  {#if s.downloaded_episodes != null && s.total_episodes}
                    <div class="absolute bottom-2 left-2 right-2">
                      <Progress
                        value={(s.downloaded_episodes / s.total_episodes) * 100}
                        class="h-0.5 {s.downloaded_episodes === s.total_episodes
                          ? '[&>div]:bg-emerald-400'
                          : ''}"
                      />
                    </div>
                  {/if}
                </div>
                <!-- Info -->
                <div class="p-2.5">
                  <p class="line-clamp-2 text-xs font-medium leading-snug">
                    {title(s)}
                  </p>
                  <p class="mt-1 text-xs text-muted-foreground">
                    {#if s.next_airing}
                      Ép. {s.next_airing.episode} à venir
                    {:else if s.total_episodes}
                      {s.total_episodes} éps
                    {/if}
                  </p>
                </div>
              </Card.Root>
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>
