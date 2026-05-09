import type { DocumentRow } from "./types";

async function readErrorMessage(res: Response): Promise<string> {
  try {
    const j: unknown = await res.json();
    if (
      j &&
      typeof j === "object" &&
      "error" in j &&
      j.error &&
      typeof j.error === "object" &&
      "message" in j.error &&
      typeof (j.error as { message?: unknown }).message === "string"
    ) {
      return (j.error as { message: string }).message;
    }
  } catch {
    // ignore
  }
  return res.statusText;
}

export async function listDocuments(): Promise<DocumentRow[]> {
  const res = await fetch("/api/documents");
  if (!res.ok) {
    throw new Error(await readErrorMessage(res));
  }
  return (await res.json()) as DocumentRow[];
}

export async function uploadDocument(file: File): Promise<DocumentRow> {
  const body = new FormData();
  body.append("file", file);
  const res = await fetch("/api/documents", { method: "POST", body });
  if (!res.ok) {
    throw new Error(await readErrorMessage(res));
  }
  return (await res.json()) as DocumentRow;
}

export async function deleteDocument(id: string): Promise<void> {
  const res = await fetch(`/api/documents/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (res.status === 204) {
    return;
  }
  if (!res.ok) {
    throw new Error(await readErrorMessage(res));
  }
}
