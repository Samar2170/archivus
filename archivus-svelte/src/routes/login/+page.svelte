<script lang="ts">
	import { goto } from "$app/navigation";
	import { onMount } from "svelte";
	import { authStore } from "$lib/stores/auth";
	import { signin } from "$lib/api/auth";

	let username = "";
	let password = "";
	let pin = "";
	let loading = false;
	let error = "";

	onMount(() => {
		if ($authStore.isAuthenticated) goto("/");
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = "";
		loading = true;
		try {
			if (password === "" && pin === "") {
				error = "Please fill either PIN or password.";
				loading = false;
				return;
			}
			await signin(username, password, pin);
			goto("/");
		} catch (err) {
			error = "Invalid credentials. Please try again.";
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Login — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-bold text-orange-600">Archivus</h1>
			<p class="mt-2 text-sm text-gray-500">Sign in to your account</p>
		</div>

		<div class="rounded-2xl bg-white p-8 shadow-lg ring-1 ring-gray-200">
			<form on:submit={handleSubmit} class="space-y-5">
				<div>
					<label
						for="username"
						class="block text-sm font-medium text-gray-700 mb-1"
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
					<label
						for="pin"
						class="block text-sm font-medium text-gray-700 mb-1"
						>PIN</label
					>
					<input
						id="pin"
						type="password"
						bind:value={pin}
						inputmode="numeric"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				<div>
					<label
						for="password"
						class="block text-sm font-medium text-gray-700 mb-1"
						>Password</label
					>
					<input
						id="password"
						type="password"
						bind:value={password}
						autocomplete="current-password"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					/>
				</div>

				{#if error}
					<p class="text-sm text-red-600">{error}</p>
				{/if}

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-lg bg-orange-600 py-2.5 text-sm font-semibold text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
				>
					{loading ? "Signing in..." : "Sign in"}
				</button>
			</form>
		</div>
	</div>
</div>
