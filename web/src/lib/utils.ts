import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export function gameIcon(game: string): string {
  const icons: Record<string, string> = {
    minecraft: "/games/minecraft.svg",
    vintagestory: "/games/vintagestory.svg",
    valheim: "/games/valheim.svg",
    terraria: "/games/terraria.svg",
  };
  return icons[game] ?? "/games/generic.svg";
}

export function statusColor(status: string): string {
  switch (status) {
    case "running":
      return "text-success";
    case "exited":
    case "dead":
      return "text-destructive";
    case "created":
    case "paused":
      return "text-warning";
    default:
      return "text-muted-foreground";
  }
}
