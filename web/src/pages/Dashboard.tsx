import { Link } from "react-router-dom";
import { Plus, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ServerCard } from "@/components/ServerCard";
import { useServers } from "@/hooks/useServers";

export function Dashboard() {
  const { data: servers, isLoading, error } = useServers();

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Servers</h1>
          <p className="text-muted-foreground mt-1">Manage your game servers</p>
        </div>
        <Link to="/create">
          <Button>
            <Plus className="h-4 w-4" /> New Server
          </Button>
        </Link>
      </div>

      {isLoading && (
        <div className="flex justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-destructive">
          Failed to load servers: {(error as Error).message}
        </div>
      )}

      {servers && servers.length === 0 && (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <Server className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-lg font-semibold mb-1">No servers yet</h2>
          <p className="text-muted-foreground mb-4">Create your first game server to get started.</p>
          <Link to="/create">
            <Button><Plus className="h-4 w-4" /> Create Server</Button>
          </Link>
        </div>
      )}

      {servers && servers.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {servers.map((s) => (
            <ServerCard key={s.id} server={s} />
          ))}
        </div>
      )}
    </div>
  );
}
