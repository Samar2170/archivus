<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { authStore } from "$lib/stores/auth";
	import type { User } from "$lib/api/auth";
	import {
		getDriveInfo,
		getDriveUsers,
		inviteUser,
		addUserToDrive,
		removeUserFromDrive,
		type AccessLevel,
		type DriveInfoResponse,
		type UsersInDrive,
	} from "$lib/api/drive";
	import Navbar from "$lib/components/Navbar.svelte";
	import {
		Loader2,
		Copy,
		Check,
		UserPlus,
		UserMinus,
		Ticket,
		HardDrive,
	} from "lucide-svelte";

	let drives: UsersInDrive[] = [];
	let selectedDriveId: string | null = null;
	let driveInfo: DriveInfoResponse | null = null;

	let loading = false;
	let infoLoading = false;
	let error = "";

	// invite
	let inviteAccess: AccessLevel = "read";
	let inviteLoading = false;
	let inviteCode = "";
	let inviteError = "";
	let copied = false;

	// add user
	let addUserId = "";
	let addAccess: AccessLevel = "read";
	let addLoading = false;
	let addError = "";
	let addSuccess = "";

	// remove
	let removing = new Set<string>();

	$: selectedDrive = drives.find((d) => d.drive.ID === selectedDriveId) ?? null;

	async function load() {
		loading = true;
		error = "";
		try {
			drives = await getDriveUsers();
			if (drives.length > 0) {
				// Keep current selection if still present, else pick the first drive.
				if (!drives.some((d) => d.drive.ID === selectedDriveId)) {
					selectedDriveId = drives[0].drive.ID;
				}
				await loadDriveInfo();
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadDriveInfo() {
		if (!selectedDriveId) return;
		infoLoading = true;
		try {
			driveInfo = await getDriveInfo(selectedDriveId);
		} catch {
			driveInfo = null;
		} finally {
			infoLoading = false;
		}
	}

	async function selectDrive(id: string) {
		if (selectedDriveId === id) return;
		selectedDriveId = id;
		inviteCode = "";
		inviteError = "";
		addError = "";
		addSuccess = "";
		await loadDriveInfo();
	}

	async function handleInvite() {
		if (!selectedDriveId) return;
		inviteLoading = true;
		inviteError = "";
		inviteCode = "";
		copied = false;
		try {
			inviteCode = await inviteUser(selectedDriveId, inviteAccess);
		} catch (err) {
			inviteError = (err as Error).message;
		} finally {
			inviteLoading = false;
		}
	}

	async function copyInvite() {
		try {
			await navigator.clipboard.writeText(inviteCode);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// clipboard unavailable — ignore
		}
	}

	async function handleAddUser(e: Event) {
		e.preventDefault();
		if (!selectedDriveId || !addUserId.trim()) return;
		addLoading = true;
		addError = "";
		addSuccess = "";
		try {
			await addUserToDrive(addUserId.trim(), selectedDriveId, addAccess);
			addSuccess = "User added to drive.";
			addUserId = "";
			await load();
		} catch (err) {
			addError = (err as Error).message;
		} finally {
			addLoading = false;
		}
	}

	async function handleRemove(user: User) {
		if (!selectedDriveId) return;
		if (!confirm(`Remove ${user.Username} from this drive?`)) return;
		removing = new Set(removing).add(user.ID);
		try {
			await removeUserFromDrive(user.ID, selectedDriveId);
			await load();
		} catch (err) {
			alert("Remove failed: " + (err as Error).message);
		} finally {
			const next = new Set(removing);
			next.delete(user.ID);
			removing = next;
		}
	}

	// Flatten the selected drive's users into one list with their access level.
	$: userRows = selectedDrive
		? ([
				...(selectedDrive.readUsers ?? []).map((u) => ({ user: u, access: "read" })),
				...(selectedDrive.writeUsers ?? []).map((u) => ({ user: u, access: "write" })),
				...(selectedDrive.managerUsers ?? []).map((u) => ({ user: u, access: "manager" })),
			] as { user: User; access: string }[])
		: [];

	const accessBadge: Record<string, string> = {
		read: "bg-gray-100 text-gray-700",
		write: "bg-blue-100 text-blue-700",
		manager: "bg-purple-100 text-purple-700",
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
	<title>Drive Management — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
		<div class="mb-6">
			<h1 class="text-2xl font-bold text-gray-900">Drive Management</h1>
			<p class="mt-1 text-sm text-gray-500">
				Manage users and invites for drives you own.
			</p>
		</div>

		{#if loading}
			<div class="flex items-center justify-center py-20 text-gray-400">
				<Loader2 class="h-6 w-6 animate-spin" />
			</div>
		{:else if error}
			<div class="rounded-lg bg-red-50 border border-red-200 p-4 text-sm text-red-700">
				{error}
			</div>
		{:else if drives.length === 0}
			<div class="rounded-lg bg-white ring-1 ring-gray-200 p-10 text-center text-sm text-gray-500">
				No drives owned by this account.
			</div>
		{:else}
			<!-- Drive selector -->
			{#if drives.length > 1}
				<div class="mb-6 flex flex-wrap gap-2">
					{#each drives as d}
						<button
							on:click={() => selectDrive(d.drive.ID)}
							class="flex items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition-colors
								{selectedDriveId === d.drive.ID
								? 'border-orange-500 bg-orange-50 text-orange-700'
								: 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'}"
						>
							<HardDrive class="h-4 w-4" />
							{d.drive.Name}
						</button>
					{/each}
				</div>
			{/if}

			<div class="grid gap-6 lg:grid-cols-3">
				<!-- Users list -->
				<div class="lg:col-span-2">
					<div class="rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
						<div class="border-b border-gray-100 px-5 py-4 flex items-center justify-between">
							<div>
								<h2 class="text-base font-semibold text-gray-900">
									{selectedDrive?.drive.Name ?? "Drive"} — Members
								</h2>
								{#if driveInfo && !infoLoading}
									<p class="text-xs text-gray-400 mt-0.5">
										Slug: {driveInfo.slug} · Owner: {driveInfo.ownerName}
									</p>
								{/if}
							</div>
							<span class="text-xs text-gray-400">{userRows.length} user{userRows.length === 1 ? "" : "s"}</span>
						</div>

						{#if userRows.length === 0}
							<div class="px-5 py-10 text-center text-sm text-gray-500">
								No members yet. Invite someone or add them directly.
							</div>
						{:else}
							<ul class="divide-y divide-gray-100">
								{#each userRows as row (row.user.ID + row.access)}
									<li class="flex items-center justify-between px-5 py-3">
										<div class="min-w-0">
											<p class="text-sm font-medium text-gray-900 truncate">
												{row.user.Username}
											</p>
											<p class="text-xs text-gray-400 truncate">{row.user.Email}</p>
										</div>
										<div class="flex items-center gap-3">
											<span
												class="rounded-full px-2.5 py-0.5 text-xs font-medium {accessBadge[row.access] ?? 'bg-gray-100 text-gray-700'}"
											>
												{row.access}
											</span>
											<button
												on:click={() => handleRemove(row.user)}
												disabled={removing.has(row.user.ID)}
												class="text-gray-400 hover:text-red-600 disabled:opacity-50 transition-colors"
												title="Remove from drive"
											>
												{#if removing.has(row.user.ID)}
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
					</div>
				</div>

				<!-- Actions column -->
				<div class="space-y-6">
					<!-- Invite -->
					<div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-200">
						<h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 mb-3">
							<Ticket class="h-4 w-4 text-orange-600" />
							Invite a user
						</h2>
						<p class="text-xs text-gray-500 mb-3">
							Generate a one-time code. New users enter it at signup to join this drive.
						</p>
						<div class="flex gap-2">
							<select
								bind:value={inviteAccess}
								class="rounded-lg border border-gray-300 px-2 py-2 text-sm
									focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
							>
								<option value="read">Read</option>
								<option value="write">Write</option>
								<option value="manager">Manager</option>
							</select>
							<button
								on:click={handleInvite}
								disabled={inviteLoading}
								class="flex-1 rounded-lg bg-orange-600 px-3 py-2 text-sm font-semibold text-white
									hover:bg-orange-700 disabled:opacity-50 transition-colors"
							>
								{inviteLoading ? "Generating..." : "Generate code"}
							</button>
						</div>

						{#if inviteError}
							<p class="mt-3 text-sm text-red-600">{inviteError}</p>
						{/if}
						{#if inviteCode}
							<div class="mt-3 flex items-center gap-2 rounded-lg bg-orange-50 border border-orange-200 px-3 py-2">
								<code class="flex-1 text-sm font-mono text-orange-800 break-all">{inviteCode}</code>
								<button
									on:click={copyInvite}
									class="text-orange-600 hover:text-orange-800 shrink-0"
									title="Copy invite code"
								>
									{#if copied}
										<Check class="h-4 w-4" />
									{:else}
										<Copy class="h-4 w-4" />
									{/if}
								</button>
							</div>
						{/if}
					</div>

					<!-- Add existing user -->
					<div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-200">
						<h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 mb-3">
							<UserPlus class="h-4 w-4 text-orange-600" />
							Add existing user
						</h2>
						<p class="text-xs text-gray-500 mb-3">
							Directly add a registered user by their user ID.
						</p>
						<form on:submit={handleAddUser} class="space-y-3">
							<input
								type="text"
								bind:value={addUserId}
								required
								placeholder="User ID"
								class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
									focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
							/>
							<div class="flex gap-2">
								<select
									bind:value={addAccess}
									class="rounded-lg border border-gray-300 px-2 py-2 text-sm
										focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
								>
									<option value="read">Read</option>
									<option value="write">Write</option>
									<option value="manager">Manager</option>
								</select>
								<button
									type="submit"
									disabled={addLoading}
									class="flex-1 rounded-lg bg-orange-600 px-3 py-2 text-sm font-semibold text-white
										hover:bg-orange-700 disabled:opacity-50 transition-colors"
								>
									{addLoading ? "Adding..." : "Add user"}
								</button>
							</div>
						</form>
						{#if addError}
							<p class="mt-3 text-sm text-red-600">{addError}</p>
						{/if}
						{#if addSuccess}
							<p class="mt-3 text-sm text-green-600">{addSuccess}</p>
						{/if}
					</div>
				</div>
			</div>
		{/if}
	</main>
</div>
