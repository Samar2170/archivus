<script lang="ts">
	import type { Todo } from "$lib/api/todo";
	import { updateTodoStatus } from "$lib/api/todo";
	import {
		CheckSquare,
		Square,
		AlertCircle,
		Minus,
		ArrowUp,
	} from "lucide-svelte";
	import { createEventDispatcher } from "svelte";

	export let todos: Todo[] = [];

	const dispatch = createEventDispatcher<{ refresh: void }>();

	const STATUS_LABELS: Record<number, string> = {
		0: "Not Done",
		1: "In Progress",
		2: "Done",
	};

	const PRIORITY_LABELS: Record<number, { label: string; color: string }> = {
		0: { label: "Low", color: "text-green-600 bg-green-50" },
		1: { label: "Medium", color: "text-yellow-600 bg-yellow-50" },
		2: { label: "High", color: "text-red-600 bg-red-50" },
	};

	async function toggleStatus(todo: Todo) {
		const nextStatus = todo.status === 2 ? 0 : todo.status + 1;
		try {
			await updateTodoStatus(todo.id, nextStatus);
			dispatch("refresh");
		} catch (err) {
			alert("Failed to update: " + (err as Error).message);
		}
	}
</script>

{#if todos.length === 0}
	<p class="py-12 text-center text-sm text-gray-400">No todos yet.</p>
{:else}
	<div class="overflow-x-auto rounded-xl border border-gray-200">
		<table class="min-w-full divide-y divide-gray-200">
			<thead class="bg-gray-50">
				<tr>
					<th
						class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 w-10"
					></th>
					<th
						class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
						>Title</th
					>
					<th
						class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
						>Priority</th
					>
					<th
						class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
						>Status</th
					>
				</tr>
			</thead>
			<tbody class="divide-y divide-gray-100 bg-white">
				{#each todos as todo (todo.id)}
					<tr class="hover:bg-gray-50 transition-colors">
						<td class="px-4 py-3">
							<button
								on:click={() => toggleStatus(todo)}
								class="text-gray-400 hover:text-orange-600 transition-colors"
								title="Toggle status"
							>
								{#if todo.status === 2}
									<CheckSquare
										class="h-5 w-5 text-orange-500"
									/>
								{:else}
									<Square class="h-5 w-5" />
								{/if}
							</button>
						</td>
						<td
							class="px-4 py-3 text-sm font-medium text-gray-800 {todo.status ===
							2
								? 'line-through text-gray-400'
								: ''}"
						>
							{todo.title}
						</td>
						<td class="px-4 py-3">
							{#if todo.priority !== undefined}
								{@const p = PRIORITY_LABELS[todo.priority] ?? {
									label: "Unknown",
									color: "text-gray-500 bg-gray-50",
								}}
								<span
									class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium {p.color}"
								>
									{p.label}
								</span>
							{/if}
						</td>
						<td class="px-4 py-3 text-sm text-gray-500">
							{STATUS_LABELS[todo.status] ?? ""}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
