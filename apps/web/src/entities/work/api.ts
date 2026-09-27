import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { apiClient } from "@/shared/api/client";
import type { Project } from "@/entities/p3/types";
import type {
  Command,
  WorkSpec,
  WorkTask,
  TaskResult,
  Token,
  Review,
  VerificationReport,
} from "./types";
const root = "/api/v1";
export const workApi = {
  list: async (project_id: string) => {
    const out: WorkTask[] = [];
    let total = 1;
    while (out.length < total) {
      const { data } = await apiClient.get<{ data: WorkTask[]; total: number }>(
        `${root}/work-tasks`,
        { params: { project_id, limit: 100, offset: out.length } },
      );
      out.push(...data.data);
      total = data.total;
      if (!data.data.length) break;
    }
    return out;
  },
  get: async (id: string) =>
    (
      await apiClient.get<{
        task: WorkTask;
        project: Project;
        dependencies: WorkTask[];
        linked_steps: {
          id: string;
          code: string;
          name: string;
          cycle_number: number;
          cycle_id: string;
        }[];
      }>(`${root}/work-tasks/${id}`)
    ).data,
  create: async (project_id: string, spec: WorkSpec) =>
    (
      await apiClient.post<WorkTask>(`${root}/work-tasks`, {
        project_id,
        ...spec,
      })
    ).data,
  command: async (id: string, op: string, body: Command) =>
    (await apiClient.post<WorkTask>(`${root}/work-tasks/${id}/${op}`, body))
      .data,
  results: async (project_id: string) => {
    const out: {
      result: TaskResult;
      verification_reports: VerificationReport[];
      submission_digest: string;
      state: string;
      actor: string;
      actor_name: string;
      review: Review | null;
      step_ids: string[];
      stage_id: string;
    }[] = [];
    let total = 1;
    while (out.length < total) {
      const { data } = await apiClient.get<{ data: typeof out; total: number }>(
        `${root}/task-results`,
        { params: { project_id, limit: 100, offset: out.length } },
      );
      out.push(...data.data);
      total = data.total;
      if (!data.data.length) break;
    }
    return out;
  },
  tokens: async () =>
    (await apiClient.get<{ data: Token[] }>(`${root}/agent-tokens`)).data.data,
  createToken: async (
    name: string,
    projects: string[],
    role: "executor" | "reviewer" = "executor",
  ) =>
    (
      await apiClient.post<{ token: Token; secret: string }>(
        `${root}/agent-tokens`,
        { name, projects, role },
      )
    ).data,
  revokeToken: async (id: string) =>
    apiClient.delete(`${root}/agent-tokens/${id}`),
};
export function useWorkTasks(id: string) {
  return useQuery({
    queryKey: ["work", "list", id],
    queryFn: () => workApi.list(id),
    enabled: !!id,
    refetchInterval: 5000,
  });
}
export function useWorkTask(id: string) {
  return useQuery({
    queryKey: ["work", "task", id],
    queryFn: () => workApi.get(id),
    enabled: !!id,
    refetchInterval: 5000,
  });
}
export function useWorkMutation<T, V>(fn: (v: V) => Promise<T>) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["work"] });
      client.invalidateQueries({ queryKey: ["p3"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });
}
