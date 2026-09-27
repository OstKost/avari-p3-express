import { WorkTasksPage, WorkTaskPage, WorkResultsPage } from '@/features/work/WorkPages';
import { Navigate, Route, Routes } from 'react-router-dom';
import { Shell } from '@/app/p3/Shell';
import { DashboardPage, ProjectsPage } from '@/features/p3/ProjectsPages';
import { ProjectPage, PhasePage } from '@/features/p3/ProjectPages';
import { StepPage } from '@/features/p3/StepPage';
import { KnowledgePage, ReferencePage, TasksPage } from '@/features/p3/OtherPages';

export function App() { return <Shell><Routes><Route path="/" element={<DashboardPage />} /><Route path="/projects" element={<ProjectsPage />} /><Route path="/projects/:projectId" element={<ProjectPage tab="overview" />} /><Route path="/projects/:projectId/p3" element={<ProjectPage tab="p3" />} /><Route path="/projects/:projectId/sdlc" element={<ProjectPage tab="sdlc" />} /><Route path="/projects/:projectId/tasks" element={<WorkTasksPage />} /><Route path="/projects/:projectId/tasks/:taskId" element={<WorkTaskPage />} /><Route path="/projects/:projectId/results" element={<WorkResultsPage />} /><Route path="/projects/:projectId/p3/:phaseCode" element={<PhasePage />} /><Route path="/projects/:projectId/p3/:phaseCode/:stepCode" element={<StepPage />} /><Route path="/p3" element={<ReferencePage kind="p3" />} /><Route path="/sdlc" element={<ReferencePage kind="sdlc" />} /><Route path="/knowledge" element={<KnowledgePage />} /><Route path="/tasks" element={<TasksPage />} /><Route path="*" element={<Navigate to="/" replace />} /></Routes></Shell> }

export default App;
