import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2, Plus, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";

interface ScheduleListProps {
  serverId: string;
}

const actionLabels: Record<string, string> = {
  start: "Start",
  stop: "Stop",
  restart: "Restart",
  backup: "Backup",
};

const actionColors: Record<string, string> = {
  start: "success",
  stop: "destructive",
  restart: "warning",
  backup: "secondary",
};

export function ScheduleList({ serverId }: ScheduleListProps) {
  const qc = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({ name: "", cron_expr: "", action: "restart" });

  const { data: schedules, isLoading } = useQuery({
    queryKey: ["schedules", serverId],
    queryFn: () => api.listSchedules(serverId),
  });

  const createSchedule = useMutation({
    mutationFn: () => api.createSchedule(serverId, formData),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["schedules", serverId] });
      setShowForm(false);
      setFormData({ name: "", cron_expr: "", action: "restart" });
    },
  });

  const toggleSchedule = useMutation({
    mutationFn: ({ scheduleId, enabled }: { scheduleId: string; enabled: boolean }) =>
      api.updateSchedule(serverId, scheduleId, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules", serverId] }),
  });

  const deleteSchedule = useMutation({
    mutationFn: (scheduleId: string) => api.deleteSchedule(serverId, scheduleId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules", serverId] }),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Schedules</CardTitle>
        <Button size="sm" onClick={() => setShowForm(!showForm)}>
          <Plus className="h-4 w-4" />
          Add Schedule
        </Button>
      </CardHeader>
      <CardContent>
        {showForm && (
          <div className="mb-4 rounded-lg border p-4 space-y-3">
            <div>
              <label className="text-sm font-medium">Name</label>
              <input
                type="text"
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm"
                placeholder="Daily restart"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              />
            </div>
            <div>
              <label className="text-sm font-medium">Cron Expression</label>
              <input
                type="text"
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
                placeholder="0 4 * * *"
                value={formData.cron_expr}
                onChange={(e) => setFormData({ ...formData, cron_expr: e.target.value })}
              />
              <p className="text-xs text-muted-foreground mt-1">
                Format: minute hour day-of-month month day-of-week (e.g. "0 4 * * *" = daily at 4:00 AM)
              </p>
            </div>
            <div>
              <label className="text-sm font-medium">Action</label>
              <select
                className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm"
                value={formData.action}
                onChange={(e) => setFormData({ ...formData, action: e.target.value })}
              >
                <option value="restart">Restart</option>
                <option value="start">Start</option>
                <option value="stop">Stop</option>
                <option value="backup">Backup</option>
              </select>
            </div>
            <div className="flex gap-2">
              <Button
                size="sm"
                onClick={() => createSchedule.mutate()}
                disabled={createSchedule.isPending || !formData.name || !formData.cron_expr}
              >
                {createSchedule.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Create"}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setShowForm(false)}>
                Cancel
              </Button>
            </div>
            {createSchedule.isError && (
              <p className="text-sm text-destructive">
                {(createSchedule.error as Error).message}
              </p>
            )}
          </div>
        )}

        {isLoading ? (
          <div className="flex justify-center py-4">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
        ) : !schedules?.length ? (
          <p className="text-sm text-muted-foreground py-4 text-center">
            No schedules. Add one to automate server tasks.
          </p>
        ) : (
          <div className="space-y-2">
            {schedules.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{s.name}</span>
                    <Badge variant={actionColors[s.action] as "success" | "destructive" | "warning" | "secondary"}>
                      {actionLabels[s.action] ?? s.action}
                    </Badge>
                  </div>
                  <div className="text-xs text-muted-foreground font-mono">
                    {s.cron_expr}
                    {s.last_run && (
                      <span> &middot; Last run: {new Date(s.last_run).toLocaleString()}</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => toggleSchedule.mutate({ scheduleId: s.id, enabled: !s.enabled })}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      s.enabled ? "bg-primary" : "bg-muted"
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                        s.enabled ? "translate-x-6" : "translate-x-1"
                      }`}
                    />
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (confirm(`Delete schedule "${s.name}"?`)) {
                        deleteSchedule.mutate(s.id);
                      }
                    }}
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
