import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { CreateServerRequest } from "@/types/server";

export function useServers() {
  return useQuery({
    queryKey: ["servers"],
    queryFn: api.listServers,
    refetchInterval: 5000,
  });
}

export function useServer(id: string) {
  return useQuery({
    queryKey: ["servers", id],
    queryFn: () => api.getServer(id),
    refetchInterval: 5000,
  });
}

export function useCreateServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateServerRequest) => api.createServer(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["servers"] }),
  });
}

export function useDeleteServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteServer(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["servers"] }),
  });
}

export function useServerAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: "start" | "stop" | "restart" }) => {
      switch (action) {
        case "start": return api.startServer(id);
        case "stop": return api.stopServer(id);
        case "restart": return api.restartServer(id);
      }
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["servers"] }),
  });
}

export function useTemplates() {
  return useQuery({
    queryKey: ["templates"],
    queryFn: api.listTemplates,
  });
}
