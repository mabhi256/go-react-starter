import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../lib/api";

export const Route = createFileRoute("/items")({
  component: ItemsPage,
});

interface Item {
  id: string;
  org_id: string;
  name: string;
  description?: string;
}

function ItemsPage() {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const { data, isLoading, error } = useQuery({
    queryKey: ["items"],
    queryFn: async () => {
      const res = await api.get<{ items: Item[] }>("/items");
      return res.data.items ?? [];
    },
  });

  const create = useMutation({
    mutationFn: (body: { name: string; description?: string }) =>
      api.post("/items", body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["items"] });
      setName("");
      setDescription("");
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    create.mutate({ name: name.trim(), description: description.trim() || undefined });
  }

  return (
    <div className="mx-auto max-w-2xl p-8 space-y-8">
      <h1 className="text-2xl font-semibold">Items</h1>

      <form onSubmit={handleSubmit} className="space-y-3">
        <input
          type="text"
          placeholder="Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="block w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
        />
        <input
          type="text"
          placeholder="Description (optional)"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          className="block w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
        />
        <button
          type="submit"
          disabled={create.isPending}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          {create.isPending ? "Creating..." : "Create item"}
        </button>
        {create.isError && (
          <p className="text-sm text-destructive">Failed to create item.</p>
        )}
      </form>

      {isLoading && <p className="text-sm text-muted-foreground">Loading...</p>}
      {error && <p className="text-sm text-destructive">Failed to load items.</p>}

      {data && data.length === 0 && (
        <p className="text-sm text-muted-foreground">No items yet.</p>
      )}

      <ul className="divide-y divide-border rounded-md border border-border">
        {data?.map((item) => (
          <li key={item.id} className="flex items-start gap-3 px-4 py-3">
            <div>
              <p className="text-sm font-medium">{item.name}</p>
              {item.description && (
                <p className="text-xs text-muted-foreground">{item.description}</p>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
