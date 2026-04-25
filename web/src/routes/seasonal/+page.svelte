<script lang="ts">
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Skeleton } from "$lib/components/ui/skeleton/index.js";
  import {
    Tabs,
    TabsList,
    TabsTrigger,
  } from "$lib/components/ui/tabs/index.js";
  import type { FollowState, Series } from "$lib/types.js";
  import { Eye, Lock } from "@lucide/svelte";
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";

  
  interface SeasonalSeries extends Series {
    is_adult?: boolean;
    next_airing?: { episode: number; airing_at: string };
  }

  let series = $state<SeasonalSeries[]>([]);
  let loading = $state(true);

  let filterFollow = $state("all");
  let search = $state("");
  let selectedGenre = $state("");
  let showNsfw = $state(false);

  onMount(async () => {
    await reload();
  });

  async function reload(): Promise<void> {
    loading = true;
    const res = await fetch("/api/series");
    series = res.ok ? await res.json() : [];
    loading = false;
  }

  async function setFollow(id: number, state: FollowState): Promise<void> {
    const action = state === 1 ? "follow" : "ignore";
    await fetch(`/api/series/${id}/${action}`, { method: "POST" });
    series = series.map((s) =>
      s.id === id ? { ...s, follow_state: state } : s,
    );
  }

  let allGenres = $derived(
    [...new Set(series.flatMap((s) => s.genres ?? []))].sort(),
  );

  const displayed = $derived.by((): SeasonalSeries[] => {
    const q = search.trim().toLowerCase();
    return series.filter((s) => {
      if (filterFollow === "active" && s.follow_state !== 1) return false;
      if (filterFollow === "ignored" && s.follow_state !== 0) return false;
      if (q) {
        const t = (
          (s.title_english ?? "") + (s.title_romaji ?? "")
        ).toLowerCase();
        if (!t.includes(q)) return false;
      }
      if (selectedGenre && !(s.genres ?? []).includes(selectedGenre))
        return false;
      return true;
    });
  });

  function title(s: SeasonalSeries): string {
    return s.title_english || s.title_romaji;
  }

  function fmtDate(iso: string): string {
    return new Date(iso).toLocaleDateString("fr-FR", {
      day: "numeric",
      month: "short",
    });
  }

  function isBlurred(s: SeasonalSeries): boolean {
    return (
      !showNsfw && (s.is_adult === true || (s.genres ?? []).includes("Hentai"))
    );
  }
</script>

<div class="flex flex-col h-full page-enter">
  <!-- Header -->
  <div class="shrink-0 border-b border-border px-5 py-3.5 space-y-2.5">
    <div class="flex items-center gap-3">
      <p class="font-display text-lg font-bold leading-none flex-1">
        Saison en cours
      </p>
      {#if !loading}
        <p class="text-xs text-muted-foreground">
          {series.filter((s) => s.follow_state === 1).length} suivis · {series.length}
          au catalogue
        </p>
      {/if}
    </div>

    <!-- Filters row -->
    <div class="flex flex-wrap items-center gap-3">
      <Tabs bind:value={filterFollow}>
        <TabsList variant="line">
          <TabsTrigger value="all">Tout</TabsTrigger>
          <TabsTrigger value="active">Suivis</TabsTrigger>
          <TabsTrigger value="ignored">Non suivis</TabsTrigger>
        </TabsList>
      </Tabs>

      <Button
        variant={showNsfw ? "default" : "outline"}
        size="sm"
        onclick={() => (showNsfw = !showNsfw)}
        title={showNsfw
          ? "Masquer le contenu adulte"
          : "Afficher le contenu adulte"}
        >{#if showNsfw}<Eye size={14} class="mr-1" />Visible{:else}<Lock
            size={14}
            class="mr-1"
          />Flou{/if}</Button
      >
    </div>

    <!-- Search + genre -->
    <div class="flex flex-wrap gap-2">
      <Input
        bind:value={search}
        placeholder="Rechercher un anime…"
        class="h-8 flex-1 min-w-48 text-sm"
      />
      {#if allGenres.length > 0}
        <select
          bind:value={selectedGenre}
          class="h-8 rounded-md border border-border bg-background px-2.5 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring"
        >
          <option value="">Tous les genres</option>
          {#each allGenres as g}
            <option value={g}>{g}</option>
          {/each}
        </select>
      {/if}
    </div>
  </div>

  <!-- Body -->
  <div class="flex-1 overflow-y-auto px-5 py-4">
    {#if loading}
      <div
        class="grid gap-3 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
      >
        {#each { length: 18 } as _}
          <div class="space-y-2">
            <Skeleton class="aspect-[2/3] w-full rounded-lg" />
            <Skeleton class="h-3 w-3/4 rounded" />
            <Skeleton class="h-6 w-full rounded" />
          </div>
        {/each}
      </div>
    {:else if displayed.length === 0}
      <div class="flex h-40 items-center justify-center">
        <p class="text-sm text-muted-foreground">Aucune série trouvée.</p>
      </div>
    {:else}
      <ul
        class="grid gap-3 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
      >
        {#each displayed as s, index (s.id)}
          {@const blurred = isBlurred(s)}
          <li animate:flip={{ duration: 300 }}>
            <Card.Root
              class="group flex h-full flex-col gap-0 py-0 overflow-hidden p-0"
            >
              <!-- Cover -->
              <a href="/series/{s.id}" class="flex flex-col h-full">
                <div class="relative aspect-[2/3] bg-muted">
                  {#if s.cover_url}
                    <img
                      src={s.cover_url}
                      alt={blurred ? "" : title(s)}
                      class="h-full w-full object-cover transition-[filter] {blurred
                        ? 'blur-xl'
                        : ''}"
                      loading="lazy"
                    />
                  {:else}
                    <div
                      class="flex h-full items-center justify-center p-2 text-center text-xs text-muted-foreground"
                    >
                      {#if blurred}<Lock size={20} />{:else}{title(s)}{/if}
                    </div>
                  {/if}
                  {#if blurred}
                    <div
                      class="absolute inset-0 flex items-center justify-center"
                    >
                      <Lock size={28} class="select-none opacity-80" />
                    </div>
                  {/if}
                  {#if s.follow_state === 1}
                    <span class="absolute right-2 top-2"
                      ><Badge>Suivi</Badge></span
                    >
                  {/if}
                </div>

                <!-- Info -->
                <div class="flex flex-1 flex-col p-2.5">
                  <p
                    class="mb-1 line-clamp-2 text-xs font-medium leading-snug no-underline hover:underline"
                  >
                    {blurred ? "???" : title(s)}
                  </p>
                  {#if s.next_airing}
                    <p class="mb-2 text-xs text-muted-foreground">
                      Ép. {s.next_airing.episode} · {fmtDate(
                        s.next_airing.airing_at,
                      )}
                    </p>
                  {:else if s.total_episodes}
                    <p class="mb-2 text-xs text-muted-foreground">
                      {s.total_episodes} éps
                    </p>
                  {:else}
                    <div class="flex-1"></div>
                  {/if}
                  <div class="mt-auto">
                    {#if s.follow_state === 1}
                      <Button
                        variant="outline"
                        size="sm"
                        class="w-full text-xs h-7"
                        onclick={() => setFollow(s.id, 0)}
                      >
                        Ne plus suivre
                      </Button>
                    {:else}
                      <Button
                        size="sm"
                        class="w-full text-xs h-7"
                        onclick={() => setFollow(s.id, 1)}
                      >
                        Suivre
                      </Button>
                    {/if}
                  </div>
                </div>
              </a>
            </Card.Root>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>
