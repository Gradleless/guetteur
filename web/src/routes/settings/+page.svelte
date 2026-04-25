<script lang="ts">
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import type { HealthResponse, SettingsResponse } from "$lib/types.js";
  import { onMount } from "svelte";

  let discord = $state("");
  let ntfy = $state("");
  let defGroups = $state("");
  let prefQual = $state("");
  let mediaDir = $state("");
  let settingsSaving = $state(false);
  let settingsMsg = $state("");

  let health = $state<HealthResponse | null>(null);

  onMount(async () => {
    const [sRes, hRes] = await Promise.all([
      fetch("/api/settings"),
      fetch("/api/health"),
    ]);
    if (sRes.ok) {
      const d: SettingsResponse = await sRes.json();
      discord = d.discord_webhook ?? "";
      ntfy = d.ntfy_topic ?? "";
      defGroups = d.default_groups ?? "";
      prefQual = d.preferred_quality ?? "";
      mediaDir = d.media_dir ?? "";
    }
    if (hRes.ok) health = await hRes.json();
  });

  async function saveNotifSettings(): Promise<void> {
    settingsSaving = true;
    settingsMsg = "";
    const res = await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ discord_webhook: discord, ntfy_topic: ntfy }),
    });
    settingsMsg = res.ok ? "Sauvegardé." : "Erreur lors de la sauvegarde.";
    settingsSaving = false;
  }

  let prefSaving = $state(false);
  let prefMsg = $state("");

  async function savePrefSettings(): Promise<void> {
    prefSaving = true;
    prefMsg = "";
    const res = await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        default_groups: defGroups,
        preferred_quality: prefQual,
      }),
    });
    prefMsg = res.ok ? "Sauvegardé." : "Erreur lors de la sauvegarde.";
    prefSaving = false;
  }

  

  interface SearchResult {
    id: number;
    title: string;
    cover_url?: string;
    status: string;
    year?: number;
    title_romaji: string;
  }

  let searchQuery = $state("");
  let searchResults = $state<SearchResult[]>([]);
  let searching = $state(false);
  let importMsg = $state("");

  async function doSearch(): Promise<void> {
    if (!searchQuery.trim()) return;
    searching = true;
    searchResults = [];
    const res = await fetch(
      `/api/anilist/search?q=${encodeURIComponent(searchQuery)}`,
    );
    searchResults = res.ok ? await res.json() : [];
    searching = false;
  }

  async function importSeries(id: number): Promise<void> {
    importMsg = "";
    const res = await fetch("/api/anilist/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ anilist_id: id }),
    });
    if (res.ok) {
      const s: SearchResult = await res.json();
      importMsg = `"${s.title}" importé.`;
      searchResults = searchResults.filter((r) => r.id !== id);
    } else {
      importMsg = "Erreur lors de l'import.";
    }
  }

  

  function fmtBytes(b: number | null | undefined): string {
    if (!b) return "—";
    if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
    if (b >= 1e6) return (b / 1e6).toFixed(0) + " MB";
    return b + " B";
  }

  function fmtUptime(s: number | null | undefined): string {
    if (!s) return "—";
    const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (d > 0) return `${d}j ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }
</script>

<div class="flex flex-col h-full page-enter">
  <!-- Header -->
  <div class="shrink-0 border-b border-border px-5 py-3.5">
    <p class="font-display text-lg font-bold leading-none">Réglages</p>
    <p class="mt-0.5 text-xs text-muted-foreground">
      Notifications, préférences et système
    </p>
  </div>

  <!-- Body — 2 column layout on large screens -->
  <div class="flex-1 overflow-y-auto px-5 py-4">
    <div class="mx-auto max-w-4xl">
      <div class="grid gap-4 lg:grid-cols-2">
        <!-- ── Col 1 ──────────────────────────────────────────────────── -->
        <div class="space-y-4">
          <!-- Notifications -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display">Notifications</Card.Title>
              <Card.Description>Discord webhook et ntfy topic.</Card.Description
              >
            </Card.Header>
            <Card.Content class="space-y-3">
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="discord"
                  >Discord Webhook URL</label
                >
                <Input
                  id="discord"
                  type="url"
                  bind:value={discord}
                  placeholder="https://discord.com/api/webhooks/…"
                />
              </div>
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="ntfy"
                  >ntfy Topic</label
                >
                <Input
                  id="ntfy"
                  bind:value={ntfy}
                  placeholder="mon-topic ou https://ntfy.monserveur.fr/topic"
                />
                <p class="mt-1 text-xs text-muted-foreground">
                  Topic seul = ntfy.sh public. URL complète = serveur
                  self-hosted.
                </p>
              </div>
              {#if settingsMsg}
                <p
                  class="text-sm {settingsMsg.startsWith('Erreur')
                    ? 'text-destructive'
                    : 'text-emerald-400'}"
                >
                  {settingsMsg}
                </p>
              {/if}
            </Card.Content>
            <Card.Footer>
              <Button onclick={saveNotifSettings} disabled={settingsSaving}>
                {settingsSaving ? "Sauvegarde…" : "Sauvegarder"}
              </Button>
            </Card.Footer>
          </Card.Root>

          <!-- Préférences de téléchargement -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display"
                >Préférences de téléchargement</Card.Title
              >
              <Card.Description
                >Surcharge les valeurs de config.yaml.</Card.Description
              >
            </Card.Header>
            <Card.Content class="space-y-3">
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="groups"
                  >Groupes par défaut</label
                >
                <Input
                  id="groups"
                  bind:value={defGroups}
                  placeholder="SubsPlease, Erai-raws"
                />
                <p class="mt-1 text-xs text-muted-foreground">
                  Priorité décroissante, séparés par des virgules.
                </p>
              </div>
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="quality"
                  >Qualité préférée</label
                >
                <Input id="quality" bind:value={prefQual} placeholder="1080p" />
              </div>
              <div>
                <p class="mb-1.5 text-sm font-medium">Dossier de destination</p>
                <div
                  class="flex h-9 items-center rounded-md border border-border bg-muted/30 px-3 text-sm text-muted-foreground"
                >
                  {mediaDir || "/media"}
                </div>
                <p class="mt-1 text-xs text-muted-foreground">
                  Défini par la variable d'env <code
                    class="font-mono text-xs">MEDIA_DIR</code
                  >.
                </p>
              </div>
              {#if prefMsg}
                <p
                  class="text-sm {prefMsg.startsWith('Erreur')
                    ? 'text-destructive'
                    : 'text-emerald-400'}"
                >
                  {prefMsg}
                </p>
              {/if}
            </Card.Content>
            <Card.Footer>
              <Button onclick={savePrefSettings} disabled={prefSaving}>
                {prefSaving ? "Sauvegarde…" : "Sauvegarder"}
              </Button>
            </Card.Footer>
          </Card.Root>
        </div>

        <!-- ── Col 2 ──────────────────────────────────────────────────── -->
        <div class="space-y-4">
          <!-- Ajouter manuellement -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display"
                >Ajouter une série manuellement</Card.Title
              >
              <Card.Description>
                Pour les séries hors calendrier saisonnier (long-running shonen,
                OVA…).
              </Card.Description>
            </Card.Header>
            <Card.Content class="space-y-3">
              <div class="flex gap-2">
                <Input
                  bind:value={searchQuery}
                  placeholder="Rechercher sur AniList…"
                  class="flex-1"
                  onkeydown={(e: KeyboardEvent) =>
                    e.key === "Enter" && doSearch()}
                />
                <Button onclick={doSearch} disabled={searching}>
                  {searching ? "…" : "Rechercher"}
                </Button>
              </div>
              {#if importMsg}
                <p
                  class="text-sm {importMsg.startsWith('Erreur')
                    ? 'text-destructive'
                    : 'text-emerald-400'}"
                >
                  {importMsg}
                </p>
              {/if}
              {#if searchResults.length > 0}
                <ul class="space-y-2">
                  {#each searchResults as r}
                    <li
                      class="flex items-center gap-3 rounded-lg border border-border p-3"
                      style="background: var(--background)"
                    >
                      {#if r.cover_url}
                        <img
                          src={r.cover_url}
                          alt=""
                          class="h-14 w-10 shrink-0 rounded object-cover"
                        />
                      {:else}
                        <div class="h-14 w-10 shrink-0 rounded bg-muted"></div>
                      {/if}
                      <div class="flex-1 min-w-0">
                        <p class="truncate text-sm font-medium">{r.title}</p>
                        <div class="mt-1 flex items-center gap-2">
                          <Badge variant="outline" class="text-xs"
                            >{r.status}</Badge
                          >
                          {#if r.year}<span
                              class="text-xs text-muted-foreground"
                              >{r.year}</span
                            >{/if}
                        </div>
                      </div>
                      <Button size="sm" onclick={() => importSeries(r.id)}
                        >Importer</Button
                      >
                    </li>
                  {/each}
                </ul>
              {/if}
            </Card.Content>
          </Card.Root>

          <!-- Système -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display">Système</Card.Title>
            </Card.Header>
            <Card.Content>
              {#if health}
                <div class="space-y-2">
                  {#each [["Version daemon", health.version ?? "—"], ["Uptime", fmtUptime(health.uptime_seconds)], ["Base de données", fmtBytes(health.db_size_bytes)], ["Espace disque libre", fmtBytes(health.disk_free_bytes)], ["Tunnel VPN", health.vpn_ip ? "Actif · " + health.vpn_ip : "Non disponible"]] as [k, v]}
                    <div class="flex justify-between">
                      <span class="text-sm text-muted-foreground">{k}</span>
                      <span
                        class="text-sm tabular-nums
												{k === 'Tunnel VPN' && health.vpn_ip ? 'text-emerald-400' : ''}
												{k === 'Tunnel VPN' && !health.vpn_ip ? 'text-destructive' : ''}
											">{v}</span
                      >
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="text-sm text-muted-foreground">Chargement…</p>
              {/if}
            </Card.Content>
          </Card.Root>
        </div>
      </div>
    </div>
  </div>
</div>
