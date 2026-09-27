import { apiClient } from '@/shared/api/client';
import type { Action, Article, ArtifactLink, Blocker, ChecklistItem, Comment, Cycle, Project, ProjectSummary, SdlcStage, Step } from './types';

const root = '/api/v1/p3';
async function data<T>(path: string): Promise<T> { return (await apiClient.get<T>(root + path)).data }
async function post<T>(path: string, body: unknown): Promise<T> { return (await apiClient.post<T>(root + path, body)).data }
async function patch<T>(path: string, body: unknown): Promise<T> { return (await apiClient.patch<T>(root + path, body)).data }
async function remove(path: string): Promise<void> { await apiClient.delete(root + path) }

export const p3Api = {
  projects: async () => (await data<{ data: ProjectSummary[] }>('/projects')).data,
  project: (id: string) => data<Project>(`/projects/${id}`),
  createProject: (body: { name: string; description?: string; due_date?: string | null }) => post<Project>('/projects', body),
  updateProject: (id: string, body: Partial<ProjectSummary>) => patch<Project>(`/projects/${id}`, body),
  updateStep: (id: string, body: Partial<Pick<Step, 'status' | 'due_date'>>) => patch<Step>(`/steps/${id}`, body),
  addChecklist: (id: string, text: string) => post<ChecklistItem>(`/steps/${id}/checklist`, { text }),
  updateChecklist: (id: string, body: Partial<ChecklistItem>) => patch<ChecklistItem>(`/checklist/${id}`, body),
  deleteChecklist: (id: string) => remove(`/checklist/${id}`),
  addLink: (id: string, body: { title: string; url: string; description?: string }) => post<ArtifactLink>(`/steps/${id}/links`, body),
  deleteLink: (id: string) => remove(`/links/${id}`),
  addComment: (id: string, text: string) => post<Comment>(`/steps/${id}/comments`, { text }),
  addBlocker: (id: string, body: { title: string; description?: string; step_id?: string; sdlc_stage_id?: string }) => post<Blocker>(`/projects/${id}/blockers`, body),
  updateBlocker: (id: string, resolved: boolean) => patch<Blocker>(`/blockers/${id}`, { resolved }),
  updateSdlc: (id: string, body: Partial<Pick<SdlcStage, 'status' | 'progress' | 'start_date' | 'due_date'>>) => patch<SdlcStage>(`/sdlc-stages/${id}`, body),
  actions: async () => (await data<{ data: Action[] }>('/actions')).data,
  addAction: (body: { project_id: string; title: string; due_date?: string | null; priority?: string; step_id?: string }) => post<Action>('/actions', body),
  updateAction: (id: string, body: Partial<Action>) => patch<Action>(`/actions/${id}`, body),
  articles: async () => (await data<{ data: Article[] }>('/articles')).data,
  addArticle: (body: { title: string; category: string; summary?: string; body: string }) => post<Article>('/articles', body),
  updateArticle: (id: string, body: Partial<Article>) => patch<Article>(`/articles/${id}`, body),
  cycles: async (id: string) => (await data<{ data: Cycle[] }>(`/projects/${id}/cycles`)).data,
  startCycle: (id: string, code: string) => post<Cycle>(`/projects/${id}/cycles/${code}`, {}),
};
