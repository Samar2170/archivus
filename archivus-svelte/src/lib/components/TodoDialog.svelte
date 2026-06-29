<script lang="ts">
	import { Plus, X } from "lucide-svelte";
	import { createEventDispatcher } from "svelte";
	import { createProject, createTodo } from "$lib/api/todo";
	import type { Project } from "$lib/api/todo";

	export let projects: Project[] = [];

	const dispatch = createEventDispatcher<{ refresh: void }>();

	let fabOpen = false;
	let projectOpen = false;
	let todoOpen = false;

	let projectTitle = "";
	let projectDesc = "";
	let savingProject = false;

	let todoTitle = "";
	let todoDesc = "";
	let todoProjectId: number | undefined = undefined;
	let savingTodo = false;

	async function handleCreateProject() {
		if (!projectTitle.trim()) return;
		savingProject = true;
		try {
			await createProject(projectTitle.trim(), projectDesc.trim());
			projectOpen = false;
			projectTitle = "";
			projectDesc = "";
			dispatch("refresh");
		} catch (err) {
			alert("Failed: " + (err as Error).message);
		} finally {
			savingProject = false;
		}
	}

	async function handleCreateTodo() {
		if (!todoTitle.trim()) return;
		savingTodo = true;
		try {
			await createTodo(todoTitle.trim(), todoDesc.trim(), todoProjectId);
			todoOpen = false;
			todoTitle = "";
			todoDesc = "";
			todoProjectId = undefined;
			dispatch("refresh");
		} catch (err) {
			alert("Failed: " + (err as Error).message);
		} finally {
			savingTodo = false;
		}
	}
</script>

<!-- FAB -->
<div class="fixed bottom-6 right-6 z-40 flex flex-col items-end gap-3">
	{#if fabOpen}
		<button
			on:click={() => {
				projectOpen = true;
				fabOpen = false;
			}}
			class="flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-medium text-gray-700
				shadow-lg ring-1 ring-gray-200 hover:bg-gray-50 transition-colors"
		>
			New project
		</button>
		<button
			on:click={() => {
				todoOpen = true;
				fabOpen = false;
			}}
			class="flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-medium text-gray-700
				shadow-lg ring-1 ring-gray-200 hover:bg-gray-50 transition-colors"
		>
			New todo
		</button>
	{/if}

	<button
		on:click={() => (fabOpen = !fabOpen)}
		class="flex h-14 w-14 items-center justify-center rounded-full bg-orange-600 text-white
			shadow-lg hover:bg-orange-700 transition-all duration-200
			{fabOpen ? 'rotate-45' : 'rotate-0'}"
		aria-label="Add"
	>
		<Plus class="h-7 w-7" />
	</button>
</div>

{#if fabOpen}
	<button
		class="fixed inset-0 z-30"
		aria-label="Close"
		on:click={() => (fabOpen = false)}
	></button>
{/if}

<!-- Project Modal -->
{#if projectOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
	>
		<div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold">New Project</h2>
				<button
					on:click={() => (projectOpen = false)}
					class="text-gray-400 hover:text-gray-600"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			<div class="space-y-3 mb-4">
				<input
					type="text"
					bind:value={projectTitle}
					placeholder="Project title"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
				/>
				<textarea
					bind:value={projectDesc}
					placeholder="Description (optional)"
					rows="3"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500 resize-none"
				/>
			</div>

			<div class="flex justify-end gap-2">
				<button
					on:click={() => (projectOpen = false)}
					class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					on:click={handleCreateProject}
					disabled={!projectTitle.trim() || savingProject}
					class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingProject ? "Creating..." : "Create"}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Todo Modal -->
{#if todoOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
	>
		<div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold">New Todo</h2>
				<button
					on:click={() => (todoOpen = false)}
					class="text-gray-400 hover:text-gray-600"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			<div class="space-y-3 mb-4">
				<input
					type="text"
					bind:value={todoTitle}
					placeholder="Todo title"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
				/>
				<textarea
					bind:value={todoDesc}
					placeholder="Description (optional)"
					rows="2"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500 resize-none"
				/>
				<select
					bind:value={todoProjectId}
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
						focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500 bg-white"
				>
					<option value={undefined}>No project</option>
					{#each projects as project (project.id)}
						<option value={project.id}>{project.title}</option>
					{/each}
				</select>
			</div>

			<div class="flex justify-end gap-2">
				<button
					on:click={() => (todoOpen = false)}
					class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					on:click={handleCreateTodo}
					disabled={!todoTitle.trim() || savingTodo}
					class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{savingTodo ? "Creating..." : "Create"}
				</button>
			</div>
		</div>
	</div>
{/if}
