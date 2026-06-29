<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import {
		Menu,
		X,
		Search,
		Home,
		List,
		CheckSquare,
		LogOut,
	} from "lucide-svelte";

	let mobileOpen = false;
	let searchQuery = "";

	const navItems = [
		{ label: "Home", href: "/", icon: Home },
		{ label: "List Files", href: "/list", icon: List },
		{ label: "ToDo", href: "/todo", icon: CheckSquare },
	];

	function handleSearch(e: Event) {
		e.preventDefault();
		if (searchQuery.trim()) {
			goto(`/list?search=${encodeURIComponent(searchQuery.trim())}`);
		}
	}

	function signout() {
		authStore.signout();
		goto("/login");
	}

	$: currentPath = $page.url.pathname;
</script>

<nav class="bg-orange-600 shadow">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<div class="flex h-16 items-center justify-between">
			<!-- Logo + Nav links -->
			<div class="flex items-center gap-8">
				<span class="text-white font-bold text-lg tracking-tight"
					>Archivus</span
				>
				<div class="hidden sm:flex gap-1">
					{#each navItems as item}
						<a
							href={item.href}
							class="flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors
								{currentPath === item.href
								? 'bg-orange-700 text-white'
								: 'text-orange-100 hover:bg-orange-500 hover:text-white'}"
						>
							<svelte:component
								this={item.icon}
								class="h-4 w-4"
							/>
							{item.label}
						</a>
					{/each}
				</div>
			</div>

			<!-- Search + user -->
			<div class="flex items-center gap-3">
				<form on:submit={handleSearch} class="hidden sm:flex">
					<div class="relative">
						<Search
							class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-orange-300"
						/>
						<input
							type="search"
							bind:value={searchQuery}
							placeholder="Search files..."
							class="pl-9 pr-3 py-1.5 text-sm rounded-md bg-orange-500 text-white placeholder-orange-300
								focus:outline-none focus:ring-2 focus:ring-white/40 w-52"
						/>
					</div>
				</form>

				<button
					on:click={signout}
					class="hidden sm:flex items-center gap-1.5 text-orange-100 hover:text-white text-sm font-medium"
					title="Sign out"
				>
					<LogOut class="h-4 w-4" />
					Sign out
				</button>

				<!-- Mobile hamburger -->
				<button
					class="sm:hidden text-white p-1"
					on:click={() => (mobileOpen = !mobileOpen)}
					aria-label="Toggle menu"
				>
					{#if mobileOpen}
						<X class="h-6 w-6" />
					{:else}
						<Menu class="h-6 w-6" />
					{/if}
				</button>
			</div>
		</div>
	</div>

	<!-- Mobile menu -->
	{#if mobileOpen}
		<div class="sm:hidden bg-orange-700 px-4 pt-2 pb-3 space-y-1">
			{#each navItems as item}
				<a
					href={item.href}
					on:click={() => (mobileOpen = false)}
					class="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-orange-100
						hover:bg-orange-600 hover:text-white"
				>
					<svelte:component this={item.icon} class="h-4 w-4" />
					{item.label}
				</a>
			{/each}

			<form on:submit={handleSearch} class="pt-2">
				<div class="relative">
					<Search
						class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-orange-300"
					/>
					<input
						type="search"
						bind:value={searchQuery}
						placeholder="Search files..."
						class="w-full pl-9 pr-3 py-1.5 text-sm rounded-md bg-orange-500 text-white
							placeholder-orange-300 focus:outline-none focus:ring-2 focus:ring-white/40"
					/>
				</div>
			</form>

			<button
				on:click={signout}
				class="flex items-center gap-2 w-full rounded-md px-3 py-2 text-sm font-medium
					text-orange-100 hover:bg-orange-600 hover:text-white"
			>
				<LogOut class="h-4 w-4" />
				Sign out
			</button>
		</div>
	{/if}
</nav>
