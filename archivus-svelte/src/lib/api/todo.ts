import { apiFetch } from '$lib/utils/fetcher';

export interface Project {
	id: number;
	title: string;
	description: string;
	projectId: number;
	userId: number;
}

export interface Todo {
	id: number;
	title: string;
	description: string;
	status: number;
	priority: number;
	projectId: number;
	userId: number;
}

export async function getProjects(): Promise<Project[]> {
	return apiFetch<Project[]>('tempora/projects');
}

export async function createProject(title: string, description: string): Promise<Project> {
	return apiFetch<Project>('tempora/projects', {
		method: 'POST',
		body: JSON.stringify({ title, description })
	});
}

export async function getTodos(projectId?: number): Promise<Todo[]> {
	const query = projectId !== undefined ? `?projectId=${projectId}` : '';
	return apiFetch<Todo[]>(`tempora/todos${query}`);
}

export async function createTodo(
	title: string,
	description: string,
	projectId?: number
): Promise<Todo> {
	return apiFetch<Todo>('tempora/todos', {
		method: 'POST',
		body: JSON.stringify([{ title, description, projectId }])
	});
}

export async function updateTodoStatus(id: number, status: number): Promise<Todo> {
	return apiFetch<Todo>('tempora/todos/update', {
		method: 'POST',
		body: JSON.stringify([{ id, status }])
	});
}
