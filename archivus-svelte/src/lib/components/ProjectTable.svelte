<script lang="ts">
	import type { Project } from "$lib/api/todo";
	import { createEventDispatcher } from "svelte";
	import { Folder } from "lucide-svelte";

	export let projects: Project[] = [];
	export let selectedProjectId: number | undefined = undefined;

	const dispatch = createEventDispatcher<{ select: number | undefined }>();

	function select(id: number | undefined) {
		dispatch("select", id);
	}
</script>

<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
	<!-- "Others" card -->
	<button
		on:click={() => select(undefined)}
		class="flex flex-col items-center gap-2 rounded-xl border p-4 text-sm font-medium
			transition-all hover:shadow-md
			{selectedProjectId === undefined
			? 'border-orange-500 bg-orange-50 text-orange-700'
			: 'border-gray-200 bg-white text-gray-600 hover:border-gray-300'}"
	>
		<Folder
			class="h-8 w-8 {selectedProjectId === undefined
				? 'text-orange-500'
				: 'text-gray-400'}"
		/>
		Others
	</button>

	{#each projects as project (project.id)}
		<button
			on:click={() => select(project.id)}
			class="flex flex-col items-center gap-2 rounded-xl border p-4 text-sm font-medium
				transition-all hover:shadow-md
				{selectedProjectId === project.id
				? 'border-orange-500 bg-orange-50 text-orange-700'
				: 'border-gray-200 bg-white text-gray-600 hover:border-gray-300'}"
		>
			<Folder
				class="h-8 w-8 {selectedProjectId === project.id
					? 'text-orange-500'
					: 'text-gray-400'}"
			/>
			<span class="truncate w-full text-center" title={project.title}
				>{project.title}</span
			>
		</button>
	{/each}
</div>
