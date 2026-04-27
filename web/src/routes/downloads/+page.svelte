<script lang="ts">
	import { onMount } from 'svelte';
	import { onEvent } from '$lib/sse.js';
	import type { DownloadState, DownloadProgress } from '$lib/types.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { Tabs, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import * as m from '$lib/paraglide/messages.js';
	import { Play, ArrowDown, Check, LoaderCircle, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	interface DownloadItem extends DownloadState {
		cover_url?: string;
		size_bytes?: number;
		stream_url?: string;
	}

	let downloads = $state<DownloadItem[]>([]);
	let loading   = $state(true);
	let filter    = $state('active');

	async function reload(): Promise<void> {
		loading = true;
		const res = await fetch(`/api/downloads?status=${filter}`);
		downloads = res.ok ? await res.json() : [];
		loading = false;
	}

	onMount(() => {
		reload();
		const unsubProgress = onEvent('download_progress', (d) => {
			const p = d as DownloadProgress;
			downloads = downloads.map((item) =>
				item.info_hash === p.info_hash ? { ...item, progress: p.progress, speed_bps: p.speed_bps } : item
			);
		});
		const unsubStatus  = onEvent('download_status_changed', reload);
		const unsubRelease = onEvent('release_detected', reload);
		return () => { unsubProgress(); unsubStatus(); unsubRelease(); };
	});

	$effect(() => { filter; reload(); });

	const activeCount   = $derived(downloads.filter(d => d.status === 'downloading').length);
	const queuedCount   = $derived(downloads.filter(d => d.status === 'queued').length);
	const totalSpeedBps = $derived(
		downloads.filter(d => d.status === 'downloading').reduce((sum, d) => sum + (d.speed_bps ?? 0), 0)
	);

	function fmtSpeed(bps: number | undefined): string | null {
		if (!bps || bps <= 0) return null;
		if (bps >= 1_000_000) return (bps / 1_000_000).toFixed(1) + ' MB/s';
		if (bps >= 1_000)     return (bps / 1_000).toFixed(0) + ' KB/s';
		return bps + ' B/s';
	}

	function fmtBytes(b: number | null | undefined): string | null {
		if (!b) return null;
		if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB';
		if (b >= 1e6) return (b / 1e6).toFixed(0) + ' MB';
		return (b / 1e3).toFixed(0) + ' KB';
	}

	function copyStream(dl: DownloadItem): void {
		navigator.clipboard.writeText(window.location.origin + (dl.stream_url ?? `/stream/${dl.info_hash}`));
		toast.success(m.toast_url_copied());
	}
</script>

<div class="flex flex-col h-full page-enter">

	<!-- Header -->
	<div class="shrink-0 border-b border-border px-5 py-3.5 space-y-2.5">
		<div class="flex items-center gap-3">
			<p class="font-display text-lg font-bold leading-none flex-1">{m.nav_downloads()}</p>
			{#if activeCount > 0}
				<div class="flex items-center gap-2.5 text-xs text-muted-foreground">
					{#if fmtSpeed(totalSpeedBps)}
						<span class="inline-flex items-center gap-0.5 font-semibold tabular-nums" style="color: var(--color-primary)">
							<ArrowDown size={12} />{fmtSpeed(totalSpeedBps)}
						</span>
						<span class="h-3 w-px bg-border"></span>
					{/if}
					<span>{m.dl_active_badge({ count: activeCount, s: activeCount > 1 ? 's' : '' })}</span>
					{#if queuedCount > 0}
						<span>{m.dl_queued_badge({ count: queuedCount })}</span>
					{/if}
				</div>
			{/if}
		</div>
		<Tabs bind:value={filter}>
			<TabsList variant="line">
				<TabsTrigger value="active">{m.tab_active()}</TabsTrigger>
				<TabsTrigger value="completed">{m.tab_completed()}</TabsTrigger>
				<TabsTrigger value="failed">{m.tab_failed()}</TabsTrigger>
				<TabsTrigger value="all">{m.lbl_all()}</TabsTrigger>
			</TabsList>
		</Tabs>
	</div>

	<!-- Body -->
	<div class="flex-1 overflow-y-auto px-5 py-4 space-y-2">

		{#if loading}
			{#each { length: 4 } as _}
				<Skeleton class="h-16 w-full rounded-xl" />
			{/each}
		{:else if downloads.length === 0}
			<div class="flex flex-1 items-center justify-center py-20">
				<p class="text-sm text-muted-foreground">{m.dl_empty()}</p>
			</div>
		{:else}
			{#each downloads as dl}
				<div class="rounded-xl border px-4 py-3 flex items-center gap-3 transition-colors"
					style="{dl.status === 'completed'
						? 'background: var(--green-lo); border-color: var(--green-lo)'
						: 'background: var(--card); border-color: var(--border)'}">

					<!-- Cover -->
					{#if dl.cover_url}
						<img src={dl.cover_url} alt="" class="h-10 w-7 shrink-0 rounded object-cover" />
					{:else}
						<div class="h-10 w-7 shrink-0 rounded bg-muted"></div>
					{/if}

					<!-- Info -->
					<div class="min-w-0 flex-1">
						{#if dl.series_title}
							<p class="text-xs font-medium" style="color: var(--color-muted-foreground)">{dl.series_title}</p>
						{/if}
						<p class="truncate text-xs font-medium">{dl.raw_title}</p>
						{#if dl.status === 'downloading'}
							<Progress value={(dl.progress ?? 0) * 100} class="mt-1.5 h-1" />
							<p class="mt-0.5 text-xs text-muted-foreground tabular-nums">
								{((dl.progress ?? 0) * 100).toFixed(1)}%{fmtSpeed(dl.speed_bps) ? ' · ' + fmtSpeed(dl.speed_bps) : ''}{fmtBytes(dl.size_bytes) ? ' · ' + fmtBytes(dl.size_bytes) : ''}
							</p>
						{:else if dl.status === 'completed'}
							<p class="text-xs" style="color: var(--green)">
								{m.dl_status_completed()}{fmtBytes(dl.size_bytes) ? ' · ' + fmtBytes(dl.size_bytes) : ''}
							</p>
						{:else if dl.status === 'queued'}
							<p class="text-xs text-muted-foreground">{m.dl_status_queued_text()}{fmtBytes(dl.size_bytes) ? ' · ' + fmtBytes(dl.size_bytes) : ''}</p>
						{:else if dl.status === 'failed'}
							<p class="text-xs text-destructive">{m.dl_status_failed_text()}</p>
						{/if}
					</div>

					<!-- Actions + status indicator -->
					<div class="flex shrink-0 items-center gap-2">
						{#if dl.status === 'completed' && dl.stream_url}
							<a href="vlc:{dl.stream_url}" target="_blank"
								class="inline-flex items-center gap-1 h-7 px-2.5 rounded-md text-xs font-medium no-underline transition-opacity hover:opacity-80"
								style="background: var(--green-lo); border: 1px solid var(--green-lo); color: var(--green)">
								<Play size={10} fill="currentColor" strokeWidth={0} /> VLC
							</a>
							<button onclick={() => copyStream(dl)}
								class="inline-flex items-center h-7 px-2.5 rounded-md text-xs font-medium transition-opacity hover:opacity-80"
								style="background: var(--card2); border: 1px solid var(--border); color: var(--color-muted-foreground)">
								{m.btn_copy_url()}
							</button>
						{:else if dl.status === 'downloading' && dl.stream_url}
							<button onclick={() => copyStream(dl)}
								class="inline-flex items-center gap-1 h-7 px-2.5 rounded-md text-xs font-medium transition-opacity hover:opacity-80"
								style="background: var(--violet-lo); border: 1px solid var(--violet-mid); color: var(--color-primary)">
								<Play size={10} fill="currentColor" strokeWidth={0} /> Stream
							</button>
						{/if}
						<!-- Status dot -->
						<div class="h-6 w-6 rounded-full flex items-center justify-center shrink-0"
							style="{dl.status === 'completed'  ? 'background: oklch(0.72 0.14 155 / 18%); color: var(--green)'
							      : dl.status === 'downloading' ? 'background: var(--violet-lo); color: var(--color-primary)'
							      : dl.status === 'queued'      ? 'background: oklch(1 0 0 / 7%); color: var(--dim)'
							      : dl.status === 'failed'      ? 'background: var(--red-lo); color: var(--red)'
							      : 'background: oklch(1 0 0 / 7%); color: var(--dim)'}">
							{#if dl.status === 'completed'}
								<Check size={13} strokeWidth={2.5} />
							{:else if dl.status === 'downloading'}
								<ArrowDown size={13} strokeWidth={2} />
							{:else if dl.status === 'queued'}
								<LoaderCircle size={13} strokeWidth={2} class="animate-spin" />
							{:else if dl.status === 'failed'}
								<X size={13} strokeWidth={2.5} />
							{:else}
								<span class="text-xs font-bold">?</span>
							{/if}
						</div>
					</div>
				</div>
			{/each}
		{/if}
	</div>
</div>
