import { Link } from "react-router-dom";
import { Play, Square, RotateCw, Trash2 } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useServerAction, useDeleteServer } from "@/hooks/useServers";
import { statusColor } from "@/lib/utils";
import type { Server } from "@/types/server";

function statusBadgeVariant(status: string) {
  switch (status) {
    case "running": return "success" as const;
    case "exited": case "dead": return "destructive" as const;
    case "created": case "paused": return "warning" as const;
    default: return "secondary" as const;
  }
}

const gameLabels: Record<string, string> = {
  minecraft: "Minecraft",
  vintagestory: "Vintage Story",
  valheim: "Valheim",
  terraria: "Terraria",
};

export function ServerCard({ server }: { server: Server }) {
  const action = useServerAction();
  const deleteServer = useDeleteServer();
  const isRunning = server.status === "running";
  const isBusy = action.isPending || deleteServer.isPending;

  return (
    <Card className="hover:border-primary/40 transition-colors">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <Link to={`/servers/${server.id}`} className="flex-1">
            <CardTitle className="text-lg hover:text-primary transition-colors">
              {server.name}
            </CardTitle>
            <p className="text-sm text-muted-foreground mt-1">
              {gameLabels[server.game] ?? server.game}
            </p>
          </Link>
          <Badge variant={statusBadgeVariant(server.status)}>
            <span className={`mr-1.5 h-2 w-2 rounded-full inline-block ${isRunning ? "bg-success animate-pulse" : statusColor(server.status).replace("text-", "bg-")}`} />
            {server.status}
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-xs text-muted-foreground mb-3">
          {server.ports.length > 0 && (
            <span>Port {server.ports[0].host}</span>
          )}
        </div>
        <div className="flex gap-2">
          {!isRunning ? (
            <Button
              size="sm"
              onClick={() => action.mutate({ id: server.id, action: "start" })}
              disabled={isBusy}
            >
              <Play className="h-3.5 w-3.5" /> Start
            </Button>
          ) : (
            <>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => action.mutate({ id: server.id, action: "stop" })}
                disabled={isBusy}
              >
                <Square className="h-3.5 w-3.5" /> Stop
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => action.mutate({ id: server.id, action: "restart" })}
                disabled={isBusy}
              >
                <RotateCw className="h-3.5 w-3.5" /> Restart
              </Button>
            </>
          )}
          <Button
            size="sm"
            variant="ghost"
            className="ml-auto text-muted-foreground hover:text-destructive"
            onClick={() => {
              if (confirm(`Delete "${server.name}"? This removes the container and all data.`))
                deleteServer.mutate(server.id);
            }}
            disabled={isBusy}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
