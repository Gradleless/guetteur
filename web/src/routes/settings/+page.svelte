<script lang="ts">
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import * as m from "$lib/paraglide/messages.js";
  import type { HealthResponse, SettingsResponse } from "$lib/types.js";
  import { onMount } from "svelte";
  import { toast } from "svelte-sonner";

  let discord = $state("");
  let ntfy = $state("");
  let defGroups = $state("");
  let prefQual = $state("");
  let mediaDir = $state("");
  let settingsSaving = $state(false);

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
    const res = await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ discord_webhook: discord, ntfy_topic: ntfy }),
    });
    if (res.ok) toast.success(m.msg_saved()); else toast.error(m.err_save());
    settingsSaving = false;
  }

  let prefSaving = $state(false);

  async function savePrefSettings(): Promise<void> {
    prefSaving = true;
    const res = await fetch("/api/settings", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        default_groups: defGroups,
        preferred_quality: prefQual,
      }),
    });
    if (res.ok) toast.success(m.msg_saved()); else toast.error(m.err_save());
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
    const res = await fetch("/api/anilist/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ anilist_id: id }),
    });
    if (res.ok) {
      const s: SearchResult = await res.json();
      toast.success(m.settings_imported({ title: s.title }));
      searchResults = searchResults.filter((r) => r.id !== id);
    } else {
      toast.error(m.err_import());
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
    const days = Math.floor(s / 86400);
    const hours = Math.floor((s % 86400) / 3600);
    const mins = Math.floor((s % 3600) / 60);
    if (days > 0) return m.uptime_days({ d: days, h: hours, min: mins });
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m`;
  }
</script>

<div class="flex flex-col h-full page-enter">
  <!-- Header -->
  <div class="shrink-0 border-b border-border px-5 py-3.5">
    <p class="font-display text-lg font-bold leading-none">{m.nav_settings()}</p>
    <p class="mt-0.5 text-xs text-muted-foreground">
      {m.settings_subtitle()}
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
              <Card.Title class="font-display">{m.settings_notif_title()}</Card.Title>
              <Card.Description>{m.settings_notif_desc()}</Card.Description>
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
                  {m.settings_ntfy_hint()}
                </p>
              </div>
            </Card.Content>
            <Card.Footer>
              <Button onclick={saveNotifSettings} disabled={settingsSaving}>
                {settingsSaving ? m.btn_saving() : m.btn_save()}
              </Button>
            </Card.Footer>
          </Card.Root>

          <!-- Download preferences -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display">{m.settings_dl_title()}</Card.Title>
              <Card.Description>{m.settings_dl_desc()}</Card.Description>
            </Card.Header>
            <Card.Content class="space-y-3">
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="groups"
                  >{m.settings_groups_label()}</label
                >
                <Input
                  id="groups"
                  bind:value={defGroups}
                  placeholder="SubsPlease, Erai-raws"
                />
                <p class="mt-1 text-xs text-muted-foreground">
                  {m.settings_groups_hint()}
                </p>
              </div>
              <div>
                <label class="mb-1.5 block text-sm font-medium" for="quality"
                  >{m.settings_quality_label()}</label
                >
                <Input id="quality" bind:value={prefQual} placeholder="1080p" />
              </div>
              <div>
                <p class="mb-1.5 text-sm font-medium">{m.settings_media_dir_label()}</p>
                <div
                  class="flex h-9 items-center rounded-md border border-border bg-muted/30 px-3 text-sm text-muted-foreground"
                >
                  {mediaDir || "/media"}
                </div>
                <p class="mt-1 text-xs text-muted-foreground">
                  {m.settings_media_dir_hint()}
                </p>
              </div>
            </Card.Content>
            <Card.Footer>
              <Button onclick={savePrefSettings} disabled={prefSaving}>
                {prefSaving ? m.btn_saving() : m.btn_save()}
              </Button>
            </Card.Footer>
          </Card.Root>
        </div>

        <!-- ── Col 2 ──────────────────────────────────────────────────── -->
        <div class="space-y-4">
          <!-- Add manually -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display">{m.settings_import_title()}</Card.Title>
              <Card.Description>{m.settings_import_desc()}</Card.Description>
            </Card.Header>
            <Card.Content class="space-y-3">
              <div class="flex gap-2">
                <Input
                  bind:value={searchQuery}
                  placeholder={m.settings_search_placeholder()}
                  class="flex-1"
                  onkeydown={(e: KeyboardEvent) =>
                    e.key === "Enter" && doSearch()}
                />
                <Button onclick={doSearch} disabled={searching}>
                  {searching ? "…" : m.btn_search()}
                </Button>
              </div>
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
                          <Badge variant="outline" class="text-xs">{r.status}</Badge>
                          {#if r.year}<span class="text-xs text-muted-foreground">{r.year}</span>{/if}
                        </div>
                      </div>
                      <Button size="sm" onclick={() => importSeries(r.id)}>{m.btn_import()}</Button>
                    </li>
                  {/each}
                </ul>
              {/if}
            </Card.Content>
          </Card.Root>

          <!-- System -->
          <Card.Root>
            <Card.Header>
              <Card.Title class="font-display">{m.settings_system_title()}</Card.Title>
            </Card.Header>
            <Card.Content>
              {#if health}
                <div class="space-y-2">
                  {#each [
                    [m.settings_health_version(), health.version ?? "—"],
                    [m.settings_health_uptime(), fmtUptime(health.uptime_seconds)],
                    [m.settings_health_db(), fmtBytes(health.db_size_bytes)],
                    [m.settings_health_disk(), fmtBytes(health.disk_free_bytes)],
                    [m.settings_health_vpn(), health.vpn_ip ? m.settings_vpn_active({ ip: health.vpn_ip }) : m.settings_vpn_inactive()],
                  ] as [k, v]}
                    <div class="flex justify-between">
                      <span class="text-sm text-muted-foreground">{k}</span>
                      <span
                        class="text-sm tabular-nums
												{k === m.settings_health_vpn() && health.vpn_ip ? 'text-emerald-400' : ''}
												{k === m.settings_health_vpn() && !health.vpn_ip ? 'text-destructive' : ''}
											">{v}</span
                      >
                    </div>
                  {/each}
                </div>
              {:else}
                <p class="text-sm text-muted-foreground">{m.settings_loading()}</p>
              {/if}
            </Card.Content>
          </Card.Root>
        </div>
      </div>
    </div>
  </div>
</div>
