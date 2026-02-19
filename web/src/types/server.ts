export interface PortMapping {
  host: string;
  container: string;
  protocol: string;
}

export interface Server {
  id: string;
  name: string;
  game: string;
  container_id: string;
  image: string;
  ports: PortMapping[];
  env: Record<string, string>;
  volumes: Record<string, string>;
  memory_limit: number;
  cpu_limit: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface ConfigField {
  key: string;
  label: string;
  type: "text" | "number" | "select" | "toggle";
  default: string;
  description: string;
  options?: string[];
  env_var: string;
}

export interface GameTemplate {
  id: string;
  name: string;
  game: string;
  description: string;
  image: string;
  ports: string[];
  env: Record<string, string>;
  volumes: Record<string, string>;
  memory: string;
  cpu: number;
  config_fields: ConfigField[];
}

export interface CreateServerRequest {
  name: string;
  template_id: string;
  env: Record<string, string>;
  memory: string;
  cpu: number;
}

export type ServerStatus = "running" | "exited" | "created" | "paused" | "restarting" | "dead" | "unknown";

export interface ServerBackup {
  id: string;
  server_id: string;
  filename: string;
  size_bytes: number;
  created_at: string;
}

export interface ServerSchedule {
  id: string;
  server_id: string;
  name: string;
  cron_expr: string;
  action: "start" | "stop" | "restart" | "backup";
  enabled: boolean;
  last_run: string;
  created_at: string;
}

export interface CreateScheduleRequest {
  name: string;
  cron_expr: string;
  action: string;
}

export interface ServerStats {
  id: number;
  server_id: string;
  cpu_percent: number;
  memory_bytes: number;
  memory_limit: number;
  disk_bytes: number;
  network_rx: number;
  network_tx: number;
  recorded_at: string;
}
