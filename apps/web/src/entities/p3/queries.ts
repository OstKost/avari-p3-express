import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { p3Api } from './api';

export const p3Keys = { projects: ['p3', 'projects'] as const, project: (id: string) => ['p3', 'project', id] as const, actions: ['p3', 'actions'] as const, articles: ['p3', 'articles'] as const, cycles: (id: string) => ['p3', 'cycles', id] as const };
export function useProjects() { return useQuery({ queryKey: p3Keys.projects, queryFn: p3Api.projects }) }
export function useProject(id: string) { return useQuery({ queryKey: p3Keys.project(id), queryFn: () => p3Api.project(id), enabled: !!id }) }
export function useActions() { return useQuery({ queryKey: p3Keys.actions, queryFn: p3Api.actions }) }
export function useArticles() { return useQuery({ queryKey: p3Keys.articles, queryFn: p3Api.articles }) }
export function useCycles(id: string) { return useQuery({ queryKey: p3Keys.cycles(id), queryFn: () => p3Api.cycles(id), enabled: !!id }) }
export function useP3Mutation<T, V>(fn: (variables: V) => Promise<T>, success?: string) {
  const client = useQueryClient();
  return useMutation({ mutationFn: fn, onSuccess: () => { client.invalidateQueries({ queryKey: ['p3'] }); if (success) toast.success(success) }, onError: (error: Error) => toast.error(error.message) });
}
