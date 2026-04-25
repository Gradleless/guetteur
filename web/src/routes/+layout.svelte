<script lang="ts">
  import { page } from "$app/stores";
  import { onEvent, startSSE } from "$lib/sse.js";
  import type {
    DownloadProgress,
    DownloadState,
    HealthResponse,
  } from "$lib/types.js";
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
    const unsubStatus = onEvent("download_status_changed", async () => {
      const r = await fetch("/api/downloads?status=active");
      if (r.ok) activeDownloads = await r.json();
    });
    return () => {
      unsubProgress();
      unsubStatus();
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

  const NAV: Array<{
    href: string;
    label: string;
    icon: Component;
    grp: "Contenu" | "Système";
  }> = [
    { href: "/", label: "Tableau de bord", icon: Home, grp: "Contenu" },
    {
      href: "/seasonal",
      label: "Saison en cours",
      icon: CalendarDays,
      grp: "Contenu",
    },
    { href: "/library", label: "Bibliothèque", icon: Library, grp: "Contenu" },
    {
      href: "/downloads",
      label: "Téléchargements",
      icon: Download,
      grp: "Système",
    },
    { href: "/settings", label: "Réglages", icon: Settings, grp: "Système" },
  ];

  const groups = ["Contenu", "Système"] as const;

  function isActive(href: string): boolean {
    if (href === "/") return $page.url.pathname === "/";
    return $page.url.pathname.startsWith(href);
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
        />
        <circle cx="8" cy="8" r="2.2" stroke-width="1.25" />
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
            {grp}
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
              {link.label}
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
            <span class="flex-1 truncate text-xs font-medium text-primary">
              {currentDL.raw_title?.split("[")[0]?.trim() ?? "Téléchargement…"}
            </span>
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

    <!-- Footer: VPN + DL count -->
    <div
      class="shrink-0 border-t border-border px-3.5 py-2.5 flex flex-col gap-1.5"
    >
      <div class="flex items-center gap-1.5">
        {#if !vpnLoaded}
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-muted-foreground"
          ></span>
          <span class="text-xs text-muted-foreground">VPN…</span>
        {:else if vpnIP}
          <span
            class="h-1.5 w-1.5 shrink-0 rounded-full dot-pulse"
            style="background: var(--green)"
          ></span>
          <span class="text-xs text-muted-foreground"
            >VPN actif · {vpnIP}</span
          >
        {:else}
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-destructive"></span>
          <span class="text-xs text-destructive">Tunnel VPN inactif</span>
        {/if}
      </div>
      {#if dlCount > 0}
        <div class="flex items-center gap-1.5">
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary"></span>
          <span class="text-xs text-muted-foreground">
            {dlCount} téléchargement{dlCount > 1 ? "s" : ""} actif{dlCount > 1
              ? "s"
              : ""}
          </span>
        </div>
      {/if}
    </div>
  </aside>

  <!-- ── Main content ── -->
  <div class="flex flex-1 flex-col overflow-hidden">
    {@render children()}
  </div>
</div>

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
      {link.label.split(" ")[0]}
    </a>
  {/each}
</nav>
