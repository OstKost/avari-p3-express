export interface ChecklistItem { id: string; text: string; done: boolean }
export interface ArtifactLink { id: string; title: string; url: string; description?: string | null }
export interface Comment { id: string; text: string; created_at?: string; author?: string }
export interface Blocker { id: string; project_id?: string; step_id?: string | null; sdlc_stage_id?: string | null; title: string; description?: string | null; resolved: boolean; created_at?: string }
export interface Activity { id: string; text?: string; description?: string; type?: string; created_at?: string; step_id?: string; phase_code?: string }
export interface Step { id: string; code: string; name: string; description?: string; status: string; due_date?: string | null; progress: number; checklist: ChecklistItem[]; links: ArtifactLink[]; comments: Comment[]; blockers: Blocker[] }
export interface Phase { id: string; code: string; name: string; description?: string; progress: number; due_date?: string | null; status: string; steps: Step[] }
export interface SdlcStage { id: string; code?: string; name: string; description?: string; status: string; progress: number; start_date?: string | null; due_date?: string | null }
export interface Action { id: string; project_id: string; step_id?: string | null; title: string; due_date?: string | null; priority?: string | null; done: boolean }
export interface Article { id: string; title: string; category: string; summary?: string | null; body: string; updated_at?: string }
export interface Cycle { id: string; phase_code: string; number: number; created_at: string }
export interface ProjectSummary { id: string; name: string; description?: string | null; due_date?: string | null; archived: boolean; rag_status: string; progress: number; current_phase?: string | null; current_sdlc_stage?: string | null; open_blockers?: number }
export interface Project extends ProjectSummary { phases: Phase[]; sdlc_stages: SdlcStage[]; blockers: Blocker[]; actions: Action[]; activity: Activity[] }
