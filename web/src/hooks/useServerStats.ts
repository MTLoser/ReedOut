import { useState, useEffect, useRef } from "react";
import { api } from "@/lib/api";
import type { ServerStats } from "@/types/server";

interface UseServerStatsOptions {
  serverId: string;
  enabled?: boolean;
  period?: string;
}

export function useServerStats({ serverId, enabled = true, period = "1h" }: UseServerStatsOptions) {
  const [history, setHistory] = useState<ServerStats[]>([]);
  const [current, setCurrent] = useState<ServerStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const wsRef = useRef<WebSocket | null>(null);

  // Fetch history on mount
  useEffect(() => {
    if (!enabled) {
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    api.getStatsHistory(serverId, period)
      .then((data) => {
        setHistory(data);
        if (data.length > 0) {
          setCurrent(data[data.length - 1]);
        }
      })
      .catch((err) => {
        console.error("Failed to fetch stats history:", err);
      })
      .finally(() => setIsLoading(false));
  }, [serverId, enabled, period]);

  // Subscribe to live updates
  useEffect(() => {
    if (!enabled) return;

    const token = localStorage.getItem("token");
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.hostname;
    const port = "8080";
    const url = `${protocol}//${host}:${port}/api/v1/servers/${serverId}/stats/live?token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const stats: ServerStats = JSON.parse(event.data);
        setCurrent(stats);
        setHistory((prev) => {
          const next = [...prev, stats];
          // Keep only last hour of data (360 entries at 10s intervals)
          if (next.length > 360) {
            return next.slice(next.length - 360);
          }
          return next;
        });
      } catch (err) {
        console.error("Failed to parse stats:", err);
      }
    };

    ws.onerror = (e) => {
      console.error("Stats WebSocket error:", e);
    };

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [serverId, enabled]);

  return { history, current, isLoading };
}
