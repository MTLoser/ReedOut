import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Play, Square, RotateCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useServer, useServerAction, useDeleteServer } from "@/hooks/useServers";
import { ConsoleTerminal } from "@/components/ConsoleTerminal";
import { StatsCharts } from "@/components/StatsCharts";
import { BackupList } from "@/components/BackupList";
import { ScheduleList } from "@/components/ScheduleList";
import { formatBytes, cn } from "@/lib/utils";

const gameLabels: Record<string, string> = {
  minecraft: "Minecraft",
  vintagestory: "Vintage Story",
  valheim: "Valheim",
  terraria: "Terraria",
};

function statusBadgeVariant(status: string) {
  switch (status) {
    case "running": return "success" as const;
    case "exited": case "dead": return "destructive" as const;
    case "created": case "paused": return "warning" as const;
    default: return "secondary" as const;
  }
}

type Tab = "overview" | "console" | "stats" | "backups" | "schedules";

export function ServerDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: server, isLoading, error } = useServer(id!);
  const action = useServerAction();
  const deleteServer = useDeleteServer();
  const [activeTab, setActiveTab] = useState<Tab>("overview");

  if (isLoading) {
    return (
      <div className="flex justify-center py-20">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  if (error || !server) {
    return (
      <div>
        <Button variant="ghost" onClick={() => navigate("/")} className="mb-4">
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-destructive">
          Server not found
        </div>
      </div>
    );
  }

  const isRunning = server.status === "running";
  const isBusy = action.isPending || deleteServer.isPending;

  const tabs: { key: Tab; label: string; runningOnly?: boolean }[] = [
    { key: "overview", label: "Overview" },
    { key: "console", label: "Console", runningOnly: true },
    { key: "stats", label: "Stats", runningOnly: true },
    { key: "backups", label: "Backups" },
    { key: "schedules", label: "Schedules" },
  ];

  return (
    <div>
      <Button variant="ghost" onClick={() => navigate("/")} className="mb-4">
        <ArrowLeft className="h-4 w-4" /> Back
      </Button>

      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{server.name}</h1>
            <Badge variant={statusBadgeVariant(server.status)}>{server.status}</Badge>
          </div>
          <p className="text-muted-foreground mt-1">{gameLabels[server.game] ?? server.game}</p>
        </div>
        <div className="flex gap-2">
          {!isRunning ? (
            <Button onClick={() => action.mutate({ id: server.id, action: "start" })} disabled={isBusy}>
              <Play className="h-4 w-4" /> Start
            </Button>
          ) : (
            <>
              <Button variant="secondary" onClick={() => action.mutate({ id: server.id, action: "stop" })} disabled={isBusy}>
                <Square className="h-4 w-4" /> Stop
              </Button>
              <Button variant="secondary" onClick={() => action.mutate({ id: server.id, action: "restart" })} disabled={isBusy}>
                <RotateCw className="h-4 w-4" /> Restart
              </Button>
            </>
          )}
          <Button
            variant="destructive"
            onClick={() => {
              if (confirm(`Delete "${server.name}"?`)) {
                deleteServer.mutate(server.id, { onSuccess: () => navigate("/") });
              }
            }}
            disabled={isBusy}
          >
            <Trash2 className="h-4 w-4" /> Delete
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border mb-6">
        {tabs.map((tab) => {
          const disabled = tab.runningOnly && !isRunning;
          return (
            <button
              key={tab.key}
              onClick={() => !disabled && setActiveTab(tab.key)}
              disabled={disabled}
              className={cn(
                "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
                activeTab === tab.key
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground hover:text-foreground",
                disabled && "opacity-40 cursor-not-allowed"
              )}
            >
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Tab content */}
      {activeTab === "overview" && (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Connection</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {server.ports.map((p, i) => (
                <div key={i} className="flex justify-between">
                  <span className="text-muted-foreground">Port ({p.protocol})</span>
                  <span className="font-mono">{p.host}:{p.container}</span>
                </div>
              ))}
              <div className="flex justify-between">
                <span className="text-muted-foreground">Image</span>
                <span className="font-mono text-xs">{server.image}</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Resources</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Memory Limit</span>
                <span>{server.memory_limit ? formatBytes(server.memory_limit) : "Unlimited"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">CPU Limit</span>
                <span>{server.cpu_limit ? `${server.cpu_limit} cores` : "Unlimited"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Container ID</span>
                <span className="font-mono text-xs">{server.container_id?.slice(0, 12) ?? "—"}</span>
              </div>
            </CardContent>
          </Card>

          <Card className="md:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">Environment Variables</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-1 text-sm font-mono">
                {Object.entries(server.env).map(([k, v]) => (
                  <div key={k} className="flex gap-2">
                    <span className="text-muted-foreground">{k}=</span>
                    <span>{v}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {activeTab === "console" && isRunning && (
        <ConsoleTerminal serverId={server.id} enabled={isRunning} />
      )}

      {activeTab === "stats" && isRunning && (
        <StatsCharts serverId={server.id} enabled={isRunning} />
      )}

      {activeTab === "backups" && (
        <BackupList serverId={server.id} serverStatus={server.status} />
      )}

      {activeTab === "schedules" && (
        <ScheduleList serverId={server.id} />
      )}
    </div>
  );
}
