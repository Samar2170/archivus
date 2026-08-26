<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { authStore } from "$lib/stores/auth";
	import {
		getApiKeys,
		createApiKey,
		revokeApiKey,
		type ApiKeyInfo,
		type ApiKeyAccessLevel,
	} from "$lib/api/apikeys";
	import Navbar from "$lib/components/Navbar.svelte";
	import {
		Loader2,
		Copy,
		Check,
		KeyRound,
		Plus,
		Trash2,
		AlertTriangle,
	} from "lucide-svelte";

	let keys: ApiKeyInfo[] = [];
	let loading = false;
	let error = "";

	// create
	let newName = "";
	let newAccess: ApiKeyAccessLevel = "read";
	let createLoading = false;
	let createError = "";
	let createdKey: { name: string; key: string } | null = null;
	let copied = false;

	// revoke
	let revoking = new Set<string>();

	async function load() {
		loading = true;
		error = "";
		try {
			keys = await getApiKeys();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleCreate(e: Event) {
		e.preventDefault();
		if (createLoading) return;
		createLoading = true;
		createError = "";
		createdKey = null;
		copied = false;
		try {
			const created = await createApiKey(newName.trim(), newAccess);
			createdKey = { name: created.name, key: created.api_key };
			newName = "";
			await load();
		} catch (err) {
			createError = (err as Error).message;
		} finally {
			createLoading = false;
		}
	}

	async function copyKey() {
		if (!createdKey) return;
		try {
			await navigator.clipboard.writeText(createdKey.key);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// clipboard unavailable — ignore
		}
	}

	async function handleRevoke(key: ApiKeyInfo) {
		if (!confirm(`Revoke "${key.name}"? Clients using this key will immediately lose access.`))
			return;
		revoking = new Set(revoking).add(key.id);
		try {
			await revokeApiKey(key.id);
			// Keep the one-time secret visible even if it belonged to this key.
			await load();
		} catch (err) {
			alert("Revoke failed: " + (err as Error).message);
		} finally {
			const next = new Set(revoking);
			next.delete(key.id);
			revoking = next;
		}
	}

	function formatDate(iso: string): string {
		const d = new Date(iso);
		if (isNaN(d.getTime())) return iso;
		return d.toLocaleDateString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
		});
	}

	function isExpired(iso: string): boolean {
		return new Date(iso).getTime() < Date.now();
	}

	const accessBadge: Record<string, string> = {
		read: "bg-gray-100 text-gray-700",
		write: "bg-blue-100 text-blue-700",
	};

	onMount(() => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		load();
	});
</script>

<svelte:head>
	<title>API Keys — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
		<div class="mb-6">
			<h1 class="text-2xl font-bold text-gray-900">API Keys</h1>
			<p class="mt-1 text-sm text-gray-500">
				Create keys for scripts and apps to access your account. Keys are valid for 90 days.
			</p>
		</div>

		<div class="grid gap-6 lg:grid-cols-3">
			<!-- Keys list -->
			<div class="lg:col-span-2">
				<div class="rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
					<div class="border-b border-gray-100 px-5 py-4 flex items-center justify-between">
						<h2 class="text-base font-semibold text-gray-900">Your keys</h2>
						<span class="text-xs text-gray-400"
							>{keys.length} key{keys.length === 1 ? "" : "s"}</span
						>
					</div>

					{#if loading}
						<div class="flex items-center justify-center py-20 text-gray-400">
							<Loader2 class="h-6 w-6 animate-spin" />
						</div>
					{:else if error}
						<div class="m-5 rounded-lg bg-red-50 border border-red-200 p-4 text-sm text-red-700">
							{error}
						</div>
					{:else if keys.length === 0}
						<div class="px-5 py-10 text-center text-sm text-gray-500">
							No API keys yet. Create one to get started.
						</div>
					{:else}
						<ul class="divide-y divide-gray-100">
							{#each keys as key (key.id)}
								<li class="flex items-center justify-between px-5 py-3">
									<div class="min-w-0">
										<div class="flex items-center gap-2">
											<p class="text-sm font-medium text-gray-900 truncate">{key.name}</p>
											<span
												class="rounded-full px-2.5 py-0.5 text-xs font-medium {accessBadge[key.access_level] ??
													'bg-gray-100 text-gray-700'}"
											>
												{key.access_level}
											</span>
											{#if isExpired(key.expires_at)}
												<span
													class="rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-700"
													>expired</span
												>
											{/if}
										</div>
										<p class="mt-0.5 text-xs text-gray-400">
											Created {formatDate(key.created_at)} · Expires {formatDate(key.expires_at)}
										</p>
									</div>
									<div class="flex items-center gap-3">
										{#if revoking.has(key.id)}
											<Loader2 class="h-4 w-4 animate-spin text-gray-400" />
										{:else if !isExpired(key.expires_at)}
											<button
												on:click={() => handleRevoke(key)}
												class="text-gray-400 hover:text-red-600 transition-colors"
												title="Revoke key"
											>
												<Trash2 class="h-4 w-4" />
											</button>
										{/if}
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			</div>

			<!-- Create -->
			<div class="space-y-6">
				<div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-200">
					<h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 mb-3">
						<KeyRound class="h-4 w-4 text-orange-600" />
						Create a key
					</h2>
					<p class="text-xs text-gray-500 mb-3">
						Read keys can only fetch data; write keys can also upload, move, and delete.
					</p>
					<form on:submit={handleCreate} class="space-y-3">
						<input
							type="text"
							bind:value={newName}
							maxlength={64}
							placeholder="Key name (e.g. backup-script)"
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
								focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
						/>
						<div class="flex gap-2">
							<select
								bind:value={newAccess}
								class="rounded-lg border border-gray-300 px-2 py-2 text-sm
									focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
							>
								<option value="read">Read</option>
								<option value="write">Write</option>
							</select>
							<button
								type="submit"
								disabled={createLoading}
								class="flex-1 flex items-center justify-center gap-1.5 rounded-lg bg-orange-600 px-3 py-2 text-sm font-semibold text-white
									hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								<Plus class="h-4 w-4" />
								{createLoading ? "Creating..." : "Create key"}
							</button>
						</div>
					</form>
					{#if createError}
						<p class="mt-3 text-sm text-red-600">{createError}</p>
					{/if}
				</div>

				{#if createdKey}
					<div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-orange-200">
						<h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 mb-1">
							<AlertTriangle class="h-4 w-4 text-orange-600" />
							"{createdKey.name}" created
						</h2>
						<p class="text-xs text-gray-500 mb-3">
							Copy this key now — it is shown only once and cannot be recovered.
						</p>
						<div
							class="flex items-center gap-2 rounded-lg bg-orange-50 border border-orange-200 px-3 py-2"
						>
							<code class="flex-1 text-sm font-mono text-orange-800 break-all"
								>{createdKey.key}</code
							>
							<button
								on:click={copyKey}
								class="text-orange-600 hover:text-orange-800 shrink-0"
								title="Copy API key"
							>
								{#if copied}
									<Check class="h-4 w-4" />
								{:else}
									<Copy class="h-4 w-4" />
								{/if}
							</button>
						</div>
						<p class="mt-2 text-xs text-gray-400">
							Use it as the <code class="font-mono">X-API-Key</code> header on API requests.
						</p>
					</div>
				{/if}
			</div>
		</div>
	</main>
</div>
