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

export async function createSession(title?: string): Promise<string> {
  const res = await fetch("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title: title?.trim() ?? "" }),
  });
  if (!res.ok) {
    throw new Error(await readErrorMessage(res));
  }
  const data = (await res.json()) as { id?: string };
  if (!data.id) {
    throw new Error("session response missing id");
  }
  return data.id;
}

export async function abortSession(sessionId: string): Promise<void> {
  await fetch(`/api/sessions/${encodeURIComponent(sessionId)}/abort`, {
    method: "POST",
  });
}
