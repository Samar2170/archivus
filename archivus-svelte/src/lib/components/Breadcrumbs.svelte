<script lang="ts">
	import { ChevronRight, Home } from "lucide-svelte";
	import { goto } from "$app/navigation";

	export let path: string = "";

	$: segments = path
		? path
				.split("/")
				.filter(Boolean)
				.map((seg, i, arr) => ({
					label: seg,
					path: arr.slice(0, i + 1).join("/"),
				}))
		: [];

	function navigate(targetPath: string) {
		goto(`/?folder=${encodeURIComponent(targetPath)}`);
	}
</script>

<nav class="flex items-center gap-1 text-sm text-gray-600 flex-wrap">
	<button
		on:click={() => navigate("")}
		class="flex items-center gap-1 hover:text-orange-600 font-medium transition-colors"
	>
		<Home class="h-4 w-4" />
		Home
	</button>

	{#each segments as segment}
		<ChevronRight class="h-4 w-4 text-gray-400 flex-shrink-0" />
		<button
			on:click={() => navigate(segment.path)}
			class="hover:text-orange-600 font-medium transition-colors truncate max-w-[140px]"
			title={segment.label}
		>
			{segment.label}
		</button>
	{/each}
</nav>
