import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { Button } from "@/shared/components/Button";
import { Card } from "@/shared/components/Card";
import { Field, fieldClass } from "@/features/p3/ui";
export function ManagerSession({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<"loading" | "login" | "ready">("loading");
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const client = useQueryClient();
  useEffect(() => {
    let alive = true;
    apiClient
      .get("/api/session")
      .then(() => {
        if (alive) setState("ready");
      })
      .catch(() => {
        if (alive) setState("login");
      });
    const expire = () => {
      client.clear();
      setState("login");
      setError("Сессия завершилась. Войдите снова.");
    };
    window.addEventListener("avari-session-expired", expire);
    return () => {
      alive = false;
      window.removeEventListener("avari-session-expired", expire);
    };
  }, [client]);
  if (state === "ready") return <>{children}</>;
  if (state === "loading")
    return (
      <p role="status" className="p-8">
        Проверяем сессию…
      </p>
    );
  return (
    <main className="mx-auto max-w-lg p-6 mt-12">
      <Card>
        <h1 className="text-2xl mb-3">Вход менеджера</h1>
        <p className="avari-secondary mb-4">
          Введите ключ менеджера, полученный у администратора сервера.
          Ключ агента предназначен для внешнего подключения.
        </p>
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault();
            setPending(true);
            setError("");
            try {
              await apiClient.post(
                "/api/session",
                { key },
                { headers: { "X-Avari-Local": "manager" } },
              );
              setKey("");
              client.clear();
              setState("ready");
            } catch (e) {
              setError(e instanceof Error ? e.message : "Не удалось войти");
            } finally {
              setPending(false);
            }
          }}
        >
          <Field label="Ключ менеджера">
            <input
              autoFocus
              autoComplete="off"
              type="password"
              required
              className={fieldClass}
              value={key}
              onChange={(e) => setKey(e.target.value)}
            />
          </Field>
          {error && (
            <p role="alert" className="text-[var(--av-danger)]">
              {error}
            </p>
          )}
          <Button type="submit" isLoading={pending}>
            Войти
          </Button>
        </form>
      </Card>
    </main>
  );
}
