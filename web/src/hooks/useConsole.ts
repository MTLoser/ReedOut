import { useEffect, useRef, useCallback } from "react";

interface UseConsoleOptions {
  serverId: string;
  enabled?: boolean;
}

export function useConsole({ serverId, enabled = true }: UseConsoleOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const onDataRef = useRef<((data: string) => void) | null>(null);

  const setOnData = useCallback((cb: (data: string) => void) => {
    onDataRef.current = cb;
  }, []);

  const sendData = useCallback((data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(data);
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;

    const token = localStorage.getItem("token");
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.hostname;
    const port = "8080";
    const url = `${protocol}//${host}:${port}/api/v1/servers/${serverId}/console?token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      onDataRef.current?.(event.data);
    };

    ws.onerror = (e) => {
      console.error("Console WebSocket error:", e);
    };

    ws.onclose = () => {
      onDataRef.current?.("\r\n\x1b[33m[Disconnected]\x1b[0m\r\n");
    };

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [serverId, enabled]);

  return { setOnData, sendData };
}
