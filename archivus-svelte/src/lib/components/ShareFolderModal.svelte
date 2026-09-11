<script lang="ts">
	import { createEventDispatcher, onMount } from "svelte";
	import { X, Loader2, UserPlus, UserMinus, Share2 } from "lucide-svelte";
	import {
		listFolderShares,
		grantFolderShare,
		revokeFolderShare,
		type SharedFolderUser,
		type ShareAccessLevel,
	} from "$lib/api/shared";

	// The folder being managed, drive-relative to the drive it lives in.
	export let driveId: string;
	export let folderPath: string;

	const dispatch = createEventDispatcher<{ close: void }>();

	let users: SharedFolderUser[] = [];
	let loading = false;
	let error = "";

	// Grant form
	let username = "";
	let access: ShareAccessLevel = "read";
	let grantLoading = false;
	let grantError = "";
	let grantSuccess = "";

	// Revoke in flight, keyed by user id.
	let removing = new Set<string>();

	async function load() {
		loading = true;
		error = "";
		try {
			users = await listFolderShares(driveId, folderPath);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleGrant(e: Event) {
		e.preventDefault();
		if (!username.trim()) return;
		grantLoading = true;
		grantError = "";
		grantSuccess = "";
		try {
			await grantFolderShare(driveId, folderPath, { username: username.trim() }, access);
			grantSuccess = `Shared with ${username.trim()}.`;
			username = "";
			await load();
		} catch (err) {
			grantError = (err as Error).message;
		} finally {
			grantLoading = false;
		}
	}

	async function handleRevoke(user: SharedFolderUser) {
		removing = new Set(removing).add(user.userId);
		try {
			await revokeFolderShare(driveId, folderPath, { userId: user.userId });
			await load();
		} catch (err) {
			alert("Revoke failed: " + (err as Error).message);
		} finally {
			const next = new Set(removing);
			next.delete(user.userId);
			removing = next;
		}
	}

	const accessBadge: Record<string, string> = {
		read: "bg-gray-100 text-gray-700",
		write: "bg-blue-100 text-blue-700",
	};

	onMount(() => {
		load();
	});
</script>

<div
	class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
	role="presentation"
>
	<button
		class="absolute inset-0 cursor-default"
		aria-label="Close"
		on:click={() => dispatch("close")}
	></button>

	<div
		role="dialog"
		aria-modal="true"
		aria-label="Share folder"
		class="relative flex max-h-[85vh] w-full max-w-md flex-col overflow-hidden rounded-2xl bg-white shadow-xl"
	>
		<!-- Header -->
		<div class="flex items-start justify-between gap-3 border-b border-gray-100 px-5 py-4">
			<div class="min-w-0">
				<h2 class="flex items-center gap-2 text-base font-semibold text-gray-900">
					<Share2 class="h-4 w-4 text-orange-600" />
					Share folder
				</h2>
				<p class="mt-0.5 truncate text-xs text-gray-400" title={folderPath}>
					{folderPath}
				</p>
			</div>
			<button
				on:click={() => dispatch("close")}
				aria-label="Close"
				class="shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
			>
				<X class="h-5 w-5" />
			</button>
		</div>

		<div class="min-h-0 flex-1 overflow-auto px-5 py-4">
			<!-- Current shares -->
			<h3 class="mb-2 text-sm font-medium text-gray-700">
				Shared with ({users.length})
			</h3>
			{#if loading}
				<div class="flex justify-center py-6 text-gray-400">
					<Loader2 class="h-5 w-5 animate-spin" />
				</div>
			{:else if error}
				<p class="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>
			{:else if users.length === 0}
				<p class="py-3 text-sm text-gray-400">Not shared with anyone yet.</p>
			{:else}
				<ul class="divide-y divide-gray-100">
					{#each users as user (user.userId)}
						<li class="flex items-center justify-between gap-3 py-2.5">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-gray-900">{user.username}</p>
								<p class="truncate text-xs text-gray-400">{user.email}</p>
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<span
									class="rounded-full px-2.5 py-0.5 text-xs font-medium {accessBadge[user.accessLevel] ?? 'bg-gray-100 text-gray-700'}"
								>
									{user.accessLevel}
								</span>
								<button
									on:click={() => handleRevoke(user)}
									disabled={removing.has(user.userId)}
									class="text-gray-400 transition-colors hover:text-red-600 disabled:opacity-50"
									title="Revoke access"
								>
									{#if removing.has(user.userId)}
										<Loader2 class="h-4 w-4 animate-spin" />
									{:else}
										<UserMinus class="h-4 w-4" />
									{/if}
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}

			<!-- Grant form -->
			<form on:submit={handleGrant} class="mt-5 border-t border-gray-100 pt-4">
				<h3 class="mb-2 flex items-center gap-1.5 text-sm font-medium text-gray-700">
					<UserPlus class="h-4 w-4 text-orange-600" />
					Share with a user
				</h3>
				<input
					type="text"
					bind:value={username}
					required
					placeholder="Username"
					class="mb-2 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
				/>
				<div class="flex gap-2">
					<select
						bind:value={access}
						class="rounded-lg border border-gray-300 px-2 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					>
						<option value="read">Read</option>
						<option value="write">Write (soon)</option>
					</select>
					<button
						type="submit"
						disabled={grantLoading}
						class="flex-1 rounded-lg bg-orange-600 px-3 py-2 text-sm font-semibold text-white
							hover:bg-orange-700 disabled:cursor-not-allowed disabled:opacity-50 transition-colors"
					>
						{grantLoading ? "Sharing…" : "Share folder"}
					</button>
				</div>
				{#if grantError}
					<p class="mt-2 text-sm text-red-600">{grantError}</p>
				{/if}
				{#if grantSuccess}
					<p class="mt-2 text-sm text-green-600">{grantSuccess}</p>
				{/if}
			</form>
		</div>
	</div>
</div>
