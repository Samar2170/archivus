<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { authStore } from "$lib/stores/auth";
	import { getProjects, getTodos } from "$lib/api/todo";
	import type { Project, Todo } from "$lib/api/todo";
	import Navbar from "$lib/components/Navbar.svelte";
	import TodoTable from "$lib/components/TodoTable.svelte";
	import ProjectTable from "$lib/components/ProjectTable.svelte";
	import TodoDialog from "$lib/components/TodoDialog.svelte";

	let projects: Project[] = [];
	let todos: Todo[] = [];
	let selectedProjectId: number | undefined = undefined;
	let loading = false;
	let error = "";

	async function loadProjects() {
		try {
			projects = await getProjects();
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function loadTodos() {
		loading = true;
		try {
			if (selectedProjectId !== undefined && selectedProjectId !== null) {
				todos = await getTodos(selectedProjectId);
			} else {
				todos = await getTodos();
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function refresh() {
		await Promise.all([loadProjects(), loadTodos()]);
	}

	onMount(async () => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		await refresh();
	});

	$: if (
		($authStore.isAuthenticated && selectedProjectId !== undefined) ||
		selectedProjectId === undefined
	) {
		loadTodos();
	}

	function handleProjectSelect(e: CustomEvent<number | undefined>) {
		selectedProjectId = e.detail;
	}
</script>

<svelte:head>
	<title>Todo — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8 space-y-6">
		<h1 class="text-xl font-semibold text-gray-900">Projects</h1>

		{#if error}
			<div class="rounded-lg bg-red-50 p-4 text-sm text-red-700">
				{error}
			</div>
		{/if}

		<ProjectTable
			{projects}
			{selectedProjectId}
			on:select={handleProjectSelect}
		/>

		<div>
			<h2 class="mb-4 text-lg font-semibold text-gray-900">
				{selectedProjectId !== undefined
					? (projects.find((p) => p.id === selectedProjectId)
							?.title ?? "Todos")
					: "Todos"}
			</h2>

			{#if loading}
				<div class="flex items-center justify-center py-12">
					<div
						class="h-6 w-6 animate-spin rounded-full border-4 border-orange-200 border-t-orange-600"
					/>
				</div>
			{:else}
				<TodoTable {todos} on:refresh={refresh} />
			{/if}
		</div>
	</main>

	<TodoDialog {projects} on:refresh={refresh} />
</div>
