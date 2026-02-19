import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { useTemplates, useCreateServer } from "@/hooks/useServers";
import type { GameTemplate } from "@/types/server";

export function CreateServer() {
  const navigate = useNavigate();
  const { data: templates, isLoading } = useTemplates();
  const createServer = useCreateServer();
  const [selected, setSelected] = useState<GameTemplate | null>(null);
  const [name, setName] = useState("");
  const [env, setEnv] = useState<Record<string, string>>({});

  const selectTemplate = (id: string) => {
    const t = templates?.find((t) => t.id === id) ?? null;
    setSelected(t);
    if (t) {
      const defaults: Record<string, string> = {};
      for (const f of t.config_fields) {
        defaults[f.env_var] = f.default;
      }
      setEnv(defaults);
      if (!name) setName(`My ${t.name} Server`);
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    await createServer.mutateAsync({
      name,
      template_id: selected.id,
      env,
      memory: selected.memory,
      cpu: selected.cpu,
    });
    navigate("/");
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-20">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div>
      <Button variant="ghost" onClick={() => navigate("/")} className="mb-4">
        <ArrowLeft className="h-4 w-4" /> Back
      </Button>

      <h1 className="text-2xl font-bold mb-6">Create Server</h1>

      {/* Template picker */}
      {!selected && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {templates?.map((t) => (
            <Card
              key={t.id}
              className="cursor-pointer hover:border-primary/60 transition-colors"
              onClick={() => selectTemplate(t.id)}
            >
              <CardHeader>
                <CardTitle>{t.name}</CardTitle>
                <CardDescription>{t.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="text-xs text-muted-foreground space-y-1">
                  <p>Image: {t.image}</p>
                  <p>Memory: {t.memory} | CPU: {t.cpu} cores</p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Config form */}
      {selected && (
        <Card>
          <CardHeader>
            <CardTitle>{selected.name}</CardTitle>
            <CardDescription>Configure your server settings</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-medium">Server Name</label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>

              {selected.config_fields.map((field) => (
                <div key={field.key} className="space-y-2">
                  <label className="text-sm font-medium">{field.label}</label>
                  <p className="text-xs text-muted-foreground">{field.description}</p>
                  {field.type === "select" && field.options ? (
                    <Select
                      value={env[field.env_var] ?? field.default}
                      onChange={(e) => setEnv({ ...env, [field.env_var]: e.target.value })}
                    >
                      {field.options.map((opt) => (
                        <option key={opt} value={opt}>{opt}</option>
                      ))}
                    </Select>
                  ) : (
                    <Input
                      type={field.type === "number" ? "number" : "text"}
                      value={env[field.env_var] ?? field.default}
                      onChange={(e) => setEnv({ ...env, [field.env_var]: e.target.value })}
                    />
                  )}
                </div>
              ))}

              {createServer.error && (
                <div className="rounded-md bg-destructive/10 border border-destructive/30 px-3 py-2 text-sm text-destructive">
                  {(createServer.error as Error).message}
                </div>
              )}

              <div className="flex gap-3 pt-2">
                <Button type="submit" disabled={createServer.isPending}>
                  {createServer.isPending ? "Creating..." : "Create Server"}
                </Button>
                <Button type="button" variant="outline" onClick={() => setSelected(null)}>
                  Change Template
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
