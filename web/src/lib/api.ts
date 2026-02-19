import type { Server, GameTemplate, CreateServerRequest, ServerStats, ServerBackup, ServerSchedule, CreateScheduleRequest } from "@/types/server";

const BASE = "/api/v1";

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem("token");
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const res = await fetch(`${BASE}${path}`, { ...options, headers });

  if (res.status === 401) {
    localStorage.removeItem("token");
    window.location.href = "/login";
    throw new ApiError(401, "Unauthorized");
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error || res.statusText);
  }

  return res.json();
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ token: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request("/auth/logout", { method: "POST" }),

  me: () => request<{ id: number; username: string }>("/auth/me"),

  // Servers
  listServers: () => request<Server[]>("/servers"),

  getServer: (id: string) => request<Server>(`/servers/${id}`),

  createServer: (data: CreateServerRequest) =>
    request<Server>("/servers", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  updateServer: (id: string, data: { name: string }) =>
    request<Server>(`/servers/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  deleteServer: (id: string) =>
    request(`/servers/${id}`, { method: "DELETE" }),

  startServer: (id: string) =>
    request(`/servers/${id}/start`, { method: "POST" }),

  stopServer: (id: string) =>
    request(`/servers/${id}/stop`, { method: "POST" }),

  restartServer: (id: string) =>
    request(`/servers/${id}/restart`, { method: "POST" }),

  // Templates
  listTemplates: () => request<GameTemplate[]>("/templates"),

  // Stats
  getStats: (id: string) => request<ServerStats>(`/servers/${id}/stats`),

  getStatsHistory: (id: string, period = "1h") =>
    request<ServerStats[]>(`/servers/${id}/stats/history?period=${period}`),

  // Backups
  listBackups: (id: string) => request<ServerBackup[]>(`/servers/${id}/backups`),

  createBackup: (id: string) =>
    request<ServerBackup>(`/servers/${id}/backups`, { method: "POST" }),

  deleteBackup: (serverId: string, backupId: string) =>
    request(`/servers/${serverId}/backups/${backupId}`, { method: "DELETE" }),

  restoreBackup: (serverId: string, backupId: string) =>
    request(`/servers/${serverId}/backups/${backupId}/restore`, { method: "POST" }),

  backupDownloadUrl: (serverId: string, backupId: string) =>
    `${BASE}/servers/${serverId}/backups/${backupId}/download`,

  // Schedules
  listSchedules: (id: string) => request<ServerSchedule[]>(`/servers/${id}/schedules`),

  createSchedule: (id: string, data: CreateScheduleRequest) =>
    request<ServerSchedule>(`/servers/${id}/schedules`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  updateSchedule: (serverId: string, scheduleId: string, data: Partial<ServerSchedule>) =>
    request<ServerSchedule>(`/servers/${serverId}/schedules/${scheduleId}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  deleteSchedule: (serverId: string, scheduleId: string) =>
    request(`/servers/${serverId}/schedules/${scheduleId}`, { method: "DELETE" }),
};
