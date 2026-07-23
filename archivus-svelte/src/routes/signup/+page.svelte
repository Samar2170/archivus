<script lang="ts">
	import { goto } from "$app/navigation";
	import { onMount } from "svelte";
	import { authStore } from "$lib/stores/auth";
	import { signup } from "$lib/api/auth";

	let username = "";
	let email = "";
	let password = "";
	let pin = "";
	let userType: "personal" | "business" = "personal";
	// Business users either join a drive via invite code, or create their own as admin.
	let businessMode: "invite" | "admin" = "invite";
	let inviteCode = "";
	let driveName = "";
	let loading = false;
	let error = "";
	let success = "";

	onMount(() => {
		if ($authStore.isAuthenticated) goto("/");
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = "";
		success = "";

		if (userType === "business" && businessMode === "invite" && !inviteCode.trim()) {
			error = "An invite code is required to join an existing drive.";
			return;
		}

		loading = true;
		try {
			await signup({
				username,
				password,
				pin,
				email,
				user_type: userType,
				...(userType === "business" && businessMode === "invite"
					? { invite_code: inviteCode.trim() }
					: {}),
				...(userType === "business" && businessMode === "admin"
					? { is_admin: true, ...(driveName.trim() ? { drive_name: driveName.trim() } : {}) }
					: {}),
			});
			success = "Account created! Redirecting to login…";
			setTimeout(() => goto("/login"), 1200);
		} catch (err) {
			error = (err as Error).message || "Signup failed. Please try again.";
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Sign up — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4 py-10">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-orange-600">Archivus</h1>
			<p class="mt-2 text-sm text-gray-500">Create your account</p>
		</div>

		<div class="rounded-2xl bg-white p-8 shadow-lg ring-1 ring-gray-200">
			<form on:submit={handleSubmit} class="space-y-5">
				<div>
					<label for="username" class="block text-sm font-medium text-gray-700 mb-1"
						>Username</label
					>
					<input
						id="username"
						type="text"
						bind:value={username}
						required
						autocomplete="username"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				<div>
					<label for="email" class="block text-sm font-medium text-gray-700 mb-1"
						>Email</label
					>
					<input
						id="email"
						type="email"
						bind:value={email}
						required
						autocomplete="email"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				<div>
					<label for="password" class="block text-sm font-medium text-gray-700 mb-1"
						>Password</label
					>
					<input
						id="password"
						type="password"
						bind:value={password}
						required
						autocomplete="new-password"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				<div>
					<label for="pin" class="block text-sm font-medium text-gray-700 mb-1"
						>PIN</label
					>
					<input
						id="pin"
						type="password"
						bind:value={pin}
						required
						inputmode="numeric"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				<div>
					<span class="block text-sm font-medium text-gray-700 mb-1">Account type</span>
					<div class="grid grid-cols-2 gap-2">
						<button
							type="button"
							on:click={() => (userType = "personal")}
							class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors
								{userType === 'personal'
								? 'border-orange-500 bg-orange-50 text-orange-700'
								: 'border-gray-300 text-gray-600 hover:bg-gray-50'}"
						>
							Personal
						</button>
						<button
							type="button"
							on:click={() => (userType = "business")}
							class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors
								{userType === 'business'
								? 'border-orange-500 bg-orange-50 text-orange-700'
								: 'border-gray-300 text-gray-600 hover:bg-gray-50'}"
						>
							Business
						</button>
					</div>
				</div>

				{#if userType === "business"}
					<div>
						<span class="block text-sm font-medium text-gray-700 mb-1">Joining as</span>
						<div class="grid grid-cols-2 gap-2">
							<button
								type="button"
								on:click={() => (businessMode = "invite")}
								class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors
									{businessMode === 'invite'
									? 'border-orange-500 bg-orange-50 text-orange-700'
									: 'border-gray-300 text-gray-600 hover:bg-gray-50'}"
							>
								With invite
							</button>
							<button
								type="button"
								on:click={() => (businessMode = "admin")}
								class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors
									{businessMode === 'admin'
									? 'border-orange-500 bg-orange-50 text-orange-700'
									: 'border-gray-300 text-gray-600 hover:bg-gray-50'}"
							>
								Admin
							</button>
						</div>
					</div>

					{#if businessMode === "invite"}
						<div>
							<label
								for="inviteCode"
								class="block text-sm font-medium text-gray-700 mb-1"
								>Invite code</label
							>
							<input
								id="inviteCode"
								type="text"
								bind:value={inviteCode}
								placeholder="Code shared by your drive admin"
								class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
									focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
							/>
						</div>
					{:else}
						<div>
							<label
								for="driveName"
								class="block text-sm font-medium text-gray-700 mb-1"
								>Drive name <span class="text-gray-400">(optional)</span></label
							>
							<input
								id="driveName"
								type="text"
								bind:value={driveName}
								placeholder="Defaults to your username"
								class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
									focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
							/>
						</div>
					{/if}
				{/if}

				{#if error}
					<p class="text-sm text-red-600">{error}</p>
				{/if}
				{#if success}
					<p class="text-sm text-green-600">{success}</p>
				{/if}

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-lg bg-orange-600 py-2.5 text-sm font-semibold text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
				>
					{loading ? "Creating account..." : "Sign up"}
				</button>
			</form>

			<p class="mt-5 text-center text-sm text-gray-500">
				Already have an account?
				<a href="/login" class="font-medium text-orange-600 hover:text-orange-700">Sign in</a>
			</p>
		</div>
	</div>
</div>
