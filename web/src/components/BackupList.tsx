import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Download, Trash2, RotateCw, Plus, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { api } from "@/lib/api";
import { formatBytes } from "@/lib/utils";

interface BackupListProps {
  serverId: string;
  serverStatus: string;
}

export function BackupList({ serverId, serverStatus }: BackupListProps) {
  const qc = useQueryClient();
  const [restoring, setRestoring] = useState<string | null>(null);

  const { data: backups, isLoading } = useQuery({
    queryKey: ["backups", serverId],
    queryFn: () => api.listBackups(serverId),
  });

  const createBackup = useMutation({
    mutationFn: () => api.createBackup(serverId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["backups", serverId] }),
  });

  const deleteBackup = useMutation({
    mutationFn: (backupId: string) => api.deleteBackup(serverId, backupId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["backups", serverId] }),
  });

  const restoreBackup = useMutation({
    mutationFn: (backupId: string) => api.restoreBackup(serverId, backupId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["backups", serverId] });
      setRestoring(null);
    },
    onError: () => setRestoring(null),
  });

  const isRunning = serverStatus === "running";

  const handleDownload = (backupId: string) => {
    const token = localStorage.getItem("token");
    const url = api.backupDownloadUrl(serverId, backupId);
    // Open in new tab with auth header via fetch + blob
    fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      .then((res) => res.blob())
      .then((blob) => {
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = "";
        a.click();
        URL.revokeObjectURL(a.href);
      });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Backups</CardTitle>
        <Button
          size="sm"
          onClick={() => createBackup.mutate()}
          disabled={createBackup.isPending}
        >
          {createBackup.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Plus className="h-4 w-4" />
          )}
          Create Backup
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="flex justify-center py-4">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
        ) : !backups?.length ? (
          <p className="text-sm text-muted-foreground py-4 text-center">
            No backups yet. Create one to get started.
          </p>
        ) : (
          <div className="space-y-2">
            {backups.map((b) => (
              <div
                key={b.id}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div>
                  <div className="text-sm font-mono">{b.filename}</div>
                  <div className="text-xs text-muted-foreground">
                    {formatBytes(b.size_bytes)} &middot;{" "}
                    {new Date(b.created_at).toLocaleString()}
                  </div>
                </div>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleDownload(b.id)}
                    title="Download"
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (confirm("Restore this backup? This will replace all current server data.")) {
                        setRestoring(b.id);
                        restoreBackup.mutate(b.id);
                      }
                    }}
                    disabled={isRunning || restoring === b.id}
                    title={isRunning ? "Stop server before restoring" : "Restore"}
                  >
                    {restoring === b.id ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <RotateCw className="h-4 w-4" />
                    )}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (confirm("Delete this backup?")) {
                        deleteBackup.mutate(b.id);
                      }
                    }}
                    disabled={deleteBackup.isPending}
                    title="Delete"
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
