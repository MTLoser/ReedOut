import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useServerStats } from "@/hooks/useServerStats";
import { formatBytes } from "@/lib/utils";

interface StatsChartsProps {
  serverId: string;
  enabled?: boolean;
}

function formatTime(timestamp: string) {
  const date = new Date(timestamp);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function StatsCharts({ serverId, enabled = true }: StatsChartsProps) {
  const { history, current, isLoading } = useServerStats({ serverId, enabled });

  if (isLoading) {
    return (
      <div className="flex justify-center py-10">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  const chartData = history.map((s) => ({
    time: formatTime(s.recorded_at),
    cpu: Math.round(s.cpu_percent * 100) / 100,
    memory: s.memory_bytes,
    memoryMB: Math.round(s.memory_bytes / 1024 / 1024),
    networkRx: s.network_rx,
    networkTx: s.network_tx,
  }));

  return (
    <div className="space-y-4">
      {/* Current values */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardContent className="pt-4">
            <div className="text-sm text-muted-foreground">CPU Usage</div>
            <div className="text-2xl font-bold">
              {current ? `${current.cpu_percent.toFixed(1)}%` : "—"}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="text-sm text-muted-foreground">Memory Usage</div>
            <div className="text-2xl font-bold">
              {current ? formatBytes(current.memory_bytes) : "—"}
            </div>
            {current && current.memory_limit > 0 && (
              <div className="text-xs text-muted-foreground">
                of {formatBytes(current.memory_limit)}
              </div>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="text-sm text-muted-foreground">Network RX</div>
            <div className="text-2xl font-bold">
              {current ? formatBytes(current.network_rx) : "—"}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="text-sm text-muted-foreground">Network TX</div>
            <div className="text-2xl font-bold">
              {current ? formatBytes(current.network_tx) : "—"}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* CPU Chart */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">CPU Usage (%)</CardTitle>
        </CardHeader>
        <CardContent>
          {chartData.length === 0 ? (
            <div className="flex h-[200px] items-center justify-center text-muted-foreground">
              No data yet — stats update every 10 seconds
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis dataKey="time" stroke="#71717a" fontSize={12} />
                <YAxis stroke="#71717a" fontSize={12} domain={[0, "auto"]} />
                <Tooltip
                  contentStyle={{ backgroundColor: "#18181b", border: "1px solid #27272a", borderRadius: "8px" }}
                  labelStyle={{ color: "#a1a1aa" }}
                />
                <Line type="monotone" dataKey="cpu" stroke="#3b82f6" strokeWidth={2} dot={false} name="CPU %" />
              </LineChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      {/* Memory Chart */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Memory Usage (MB)</CardTitle>
        </CardHeader>
        <CardContent>
          {chartData.length === 0 ? (
            <div className="flex h-[200px] items-center justify-center text-muted-foreground">
              No data yet — stats update every 10 seconds
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis dataKey="time" stroke="#71717a" fontSize={12} />
                <YAxis stroke="#71717a" fontSize={12} domain={[0, "auto"]} />
                <Tooltip
                  contentStyle={{ backgroundColor: "#18181b", border: "1px solid #27272a", borderRadius: "8px" }}
                  labelStyle={{ color: "#a1a1aa" }}
                  formatter={(value: number) => [`${value} MB`, "Memory"]}
                />
                <Line type="monotone" dataKey="memoryMB" stroke="#22c55e" strokeWidth={2} dot={false} name="Memory MB" />
              </LineChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
