<script lang="ts">
  import { page } from "$app/state";
  import LanguageSelector from "$lib/components/language-selector.svelte";
  import { Toaster } from "$lib/components/ui/sonner/index.js";
  import * as m from "$lib/paraglide/messages.js";
  import { onEvent, startSSE } from "$lib/sse.js";
  import type {
    DownloadProgress,
    DownloadState,
    DownloadStatusChanged,
    HealthResponse,
    ReleaseDetected,
  } from "$lib/types.js";
  import { toast } from "svelte-sonner";
  import {
    CalendarDays,
    Download,
    Home,
    Library,
    Settings,
  } from "@lucide/svelte";
  import type { Component } from "svelte";
  import { onMount } from "svelte";
  import "./layout.css";

  let { children } = $props();
  let activeDownloads = $state<DownloadState[]>([]);
  let vpnIP = $state<string | null>(null);
  let vpnLoaded = $state(false);

  onMount(() => {
    startSSE();

    (async () => {
      const [dlRes, hRes] = await Promise.all([
        fetch("/api/downloads?status=active"),
        fetch("/api/health"),
      ]);
      if (dlRes.ok) activeDownloads = await dlRes.json();
      if (hRes.ok) {
        const h: HealthResponse = await hRes.json();
        vpnIP = h.vpn_ip ?? null;
      }
      vpnLoaded = true;
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

    const unsubStatus = onEvent("download_status_changed", async (raw) => {
      const d = raw as DownloadStatusChanged;
      const snap = activeDownloads.find((dl) => dl.info_hash === d.info_hash);

      const r = await fetch("/api/downloads?status=active");
      if (r.ok) activeDownloads = await r.json();

      const fresh = activeDownloads.find((dl) => dl.info_hash === d.info_hash);
      const title =
        fresh?.series_title ?? fresh?.raw_title ??
        snap?.series_title ?? snap?.raw_title ??
        d.info_hash.substring(0, 8);

      if (d.status === "downloading") {
        toast.loading(title, { id: d.info_hash, description: m.toast_download_started() });
      } else if (d.status === "completed") {
        const streamUrl = snap?.stream_url ?? fresh?.stream_url;
        toast.success(title, {
          id: d.info_hash,
          description: m.toast_stream_ready(),
          duration: 15_000,
          action: streamUrl
            ? { label: m.toast_open_vlc(), onClick: () => window.open(`vlc:${streamUrl}`, "_blank") }
            : undefined,
        });
      } else if (d.status === "failed") {
        toast.error(title, { id: d.info_hash, description: m.dl_status_failed_text() });
      }
    });

    const unsubRelease = onEvent("release_detected", (raw) => {
      const d = raw as ReleaseDetected;
      const label = d.raw_title.split("[")[0]?.trim() ?? d.raw_title;
      toast.info(label, { description: m.toast_release_detected() });
    });

    return () => {
      unsubProgress();
      unsubStatus();
      unsubRelease();
    };
  });

  const currentDL = $derived(
    activeDownloads.find((d) => d.status === "downloading"),
  );
  const dlCount = $derived(
    activeDownloads.filter((d) => d.status === "downloading").length,
  );

  function fmtSpeed(bps: number | undefined): string | null {
    if (!bps || bps <= 0) return null;
    if (bps >= 1_000_000) return (bps / 1_000_000).toFixed(1) + " MB/s";
    if (bps >= 1_000) return (bps / 1_000).toFixed(0) + " KB/s";
    return bps + " B/s";
  }

  type NavGroup = "content" | "system";

  const NAV: Array<{ href: string; icon: Component; grp: NavGroup }> = [
    { href: "/", icon: Home, grp: "content" },
    { href: "/seasonal", icon: CalendarDays, grp: "content" },
    { href: "/library", icon: Library, grp: "content" },
    { href: "/downloads", icon: Download, grp: "system" },
    { href: "/settings", icon: Settings, grp: "system" },
  ];

  const groups: NavGroup[] = ["content", "system"];

  function navLabel(href: string): string {
    if (href === "/") return m.nav_dashboard();
    if (href === "/seasonal") return m.nav_seasonal();
    if (href === "/library") return m.nav_library();
    if (href === "/downloads") return m.nav_downloads();
    if (href === "/settings") return m.nav_settings();
    return href;
  }

  function groupLabel(grp: NavGroup): string {
    return grp === "content" ? m.nav_group_content() : m.nav_group_system();
  }

  function isActive(href: string): boolean {
    if (href === "/") return page.url.pathname === "/";
    return page.url.pathname.startsWith(href);
  }
</script>

<div class="flex h-full overflow-hidden bg-background text-foreground">
  <!-- ── Sidebar (desktop) ── -->
  <aside
    class="hidden lg:flex w-[204px] shrink-0 flex-col border-r border-border bg-sidebar"
  >
    <!-- Logo -->
    <div
      class="flex h-12 shrink-0 items-center gap-2.5 border-b border-border px-4"
    >
      <svg
        width="18"
        height="18"
        viewBox="0 0 16 16"
        class="shrink-0 text-primary"
        fill="none"
        stroke="currentColor"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path
          d="M1.5 8C3 4.5 5.5 2.5 8 2.5S13 4.5 14.5 8C13 11.5 10.5 13.5 8 13.5S3 11.5 1.5 8z"
          stroke-width="1.25"
        ></path>
        <circle cx="8" cy="8" r="2.2" stroke-width="1.25"></circle>
      </svg>
      <span
        class="font-display text-base font-bold tracking-tight text-foreground"
        >guetteur</span
      >
    </div>

    <!-- Nav groups -->
    <nav class="flex-1 overflow-hidden px-2 py-2.5">
      {#each groups as grp}
        <div class="mb-4">
          <p
            class="mb-1 px-2.5 text-xs font-bold uppercase tracking-[1.1px]"
            style="color: var(--dim)"
          >
            {groupLabel(grp)}
          </p>

          {#each NAV.filter((n) => n.grp === grp) as link}
            {@const active = isActive(link.href)}
            <a
              href={link.href}
              class="flex h-[34px] items-center gap-2.5 rounded-[7px] px-2.5 text-sm no-underline transition-colors duration-[120ms]
								{active
                ? 'font-medium text-primary'
                : 'text-muted-foreground hover:text-primary'}"
              style={active
                ? "background: var(--violet-lo)"
                : "background: transparent"}
              onmouseenter={(e) => {
                if (!active)
                  e.currentTarget.style.background = "var(--violet-lo)";
              }}
              onmouseleave={(e) => {
                if (!active) e.currentTarget.style.background = "transparent";
              }}
            >
              <link.icon
                size={16}
                strokeWidth={1.5}
                class={active
                  ? "text-primary shrink-0"
                  : "text-muted-foreground shrink-0"}
              />
              {navLabel(link.href)}
            </a>
          {/each}
        </div>
      {/each}
    </nav>

    <!-- Live DL strip -->
    {#if currentDL}
      <div class="px-3 pb-2.5">
        <div
          class="rounded-lg border p-2.5"
          style="border-color: var(--violet-mid); background: var(--violet-lo)"
        >
          <div class="mb-1.5 flex items-center gap-1.5">
            <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary dot-pulse"
            ></span>
            <span class="flex-1 truncate text-xs font-medium text-primary"
              >{currentDL.raw_title?.split("[")[0]?.trim() ??
                m.dl_strip_default()}</span
            >
          </div>
          <div
            class="relative h-[2.5px] overflow-hidden rounded-full"
            style="background: oklch(1 0 0 / 7%)"
          >
            <div
              class="h-full rounded-full bg-primary bar-animated transition-all"
              style="width: {((currentDL.progress ?? 0) * 100).toFixed(0)}%"
            ></div>
          </div>
          <p class="mt-1 text-xs text-muted-foreground tabular-nums">
            {((currentDL.progress ?? 0) * 100).toFixed(0)}%{fmtSpeed(
              currentDL.speed_bps,
            )
              ? " · " + fmtSpeed(currentDL.speed_bps)
              : ""}
          </p>
        </div>
      </div>
    {/if}

    <!-- Footer: VPN + DL count + language selector -->
    <div
      class="shrink-0 border-t border-border px-3.5 py-2.5 flex flex-col gap-1.5"
    >
      <div class="flex items-center gap-1.5">
        {#if !vpnLoaded}
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-muted-foreground"
          ></span>
          <span class="text-xs text-muted-foreground">{m.vpn_loading()}</span>
        {:else if vpnIP}
          <span
            class="h-1.5 w-1.5 shrink-0 rounded-full dot-pulse"
            style="background: var(--green)"
          ></span>
          <span class="text-xs text-muted-foreground"
            >{m.vpn_active({ ip: vpnIP })}</span
          >
        {:else}
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-destructive"></span>
          <span class="text-xs text-destructive">{m.vpn_inactive()}</span>
        {/if}
      </div>

      {#if dlCount > 0}
        <div class="flex items-center gap-1.5">
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary"></span>
          <span class="text-xs text-muted-foreground"
            >{m.dl_active_count({
              count: dlCount,
              s: dlCount > 1 ? "s" : "",
            })}</span
          >
        </div>
      {/if}

      <div class="pt-0.5">
        <LanguageSelector />
      </div>
    </div>
  </aside>

  <!-- ── Main content ── -->
  <div class="flex flex-1 flex-col overflow-hidden">{@render children()}</div>
</div>

<Toaster position="bottom-right" richColors />

<!-- ── Bottom nav (mobile) ── -->
<nav
  class="fixed inset-x-0 bottom-0 z-40 flex border-t border-border bg-sidebar lg:hidden"
>
  {#each NAV as link}
    <a
      href={link.href}
      class="flex min-h-[52px] flex-1 flex-col items-center justify-center gap-0.5 no-underline text-xs font-medium transition-colors
				{isActive(link.href) ? 'text-primary' : 'text-muted-foreground'}"
    >
      <link.icon size={16} strokeWidth={1.5} />
      {navLabel(link.href).split(" ")[0]}
    </a>
  {/each}
</nav>
