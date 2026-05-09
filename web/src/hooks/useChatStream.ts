import { useCallback, useRef, useState } from "react";
import type { ChatMessage, Citation } from "../api/types";
import { abortSession } from "../api/chat";

type SseEvent = { event: string; data: string };

function takeSseEvents(buffer: string): { events: SseEvent[]; rest: string } {
  const events: SseEvent[] = [];
  let rest = buffer;
  for (;;) {
    const sep = rest.indexOf("\n\n");
    if (sep === -1) {
      break;
    }
    const raw = rest.slice(0, sep);
    rest = rest.slice(sep + 2);
    let eventName = "message";
    const dataLines: string[] = [];
    for (const line of raw.split("\n")) {
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim();
      } else if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).replace(/^\s/, ""));
      }
    }
    events.push({ event: eventName, data: dataLines.join("\n") });
  }
  return { events, rest };
}

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

export function useChatStream(sessionId: string | null) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [busy, setBusy] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);
  const ctrlRef = useRef<AbortController | null>(null);

  const stop = useCallback(() => {
    ctrlRef.current?.abort();
    ctrlRef.current = null;
    if (sessionId) {
      void abortSession(sessionId);
    }
  }, [sessionId]);

  const send = useCallback(
    async (content: string) => {
      if (!sessionId) {
        return;
      }
      const text = content.trim();
      if (!text) {
        return;
      }

      setLastError(null);
      setBusy(true);
      const ctrl = new AbortController();
      ctrlRef.current = ctrl;

      setMessages((m) => [
        ...m,
        { role: "user", content: text },
        { role: "assistant", content: "", citations: [] },
      ]);

      let assistantText = "";
      let assistantCites: Citation[] = [];

      const patchAssistant = () => {
        setMessages((m) => {
          if (m.length === 0) {
            return m;
          }
          const copy = m.slice();
          const last = copy[copy.length - 1];
          if (!last || last.role !== "assistant") {
            return m;
          }
          copy[copy.length - 1] = {
            ...last,
            content: assistantText,
            citations: assistantCites.length ? assistantCites : last.citations,
          };
          return copy;
        });
      };

      try {
        const res = await fetch(
          `/api/sessions/${encodeURIComponent(sessionId)}/messages`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ content: text }),
            signal: ctrl.signal,
          },
        );

        if (!res.ok) {
          throw new Error(await readErrorMessage(res));
        }
        if (!res.body) {
          throw new Error("empty response body");
        }

        const reader = res.body.getReader();
        const dec = new TextDecoder();
        let buf = "";

        const applyEvents = (list: SseEvent[]) => {
          for (const ev of list) {
            if (ev.event === "citations") {
              const parsed = JSON.parse(ev.data) as Citation[];
              assistantCites = Array.isArray(parsed) ? parsed : [];
              patchAssistant();
            } else if (ev.event === "token") {
              const payload = JSON.parse(ev.data) as { t?: string };
              if (typeof payload.t === "string" && payload.t.length > 0) {
                assistantText += payload.t;
                patchAssistant();
              }
            } else if (ev.event === "error") {
              const payload = JSON.parse(ev.data) as {
                message?: string;
                code?: string;
              };
              const msg =
                typeof payload.message === "string"
                  ? payload.message
                  : "stream error";
              throw new Error(msg);
            } else if (ev.event === "done") {
              patchAssistant();
            }
          }
        };

        for (;;) {
          const { done, value } = await reader.read();
          if (done) {
            break;
          }
          buf += dec.decode(value, { stream: true });
          const parsed = takeSseEvents(buf);
          buf = parsed.rest;
          applyEvents(parsed.events);
        }
        const tail = dec.decode();
        if (tail) {
          buf += tail;
        }
        const parsed = takeSseEvents(buf);
        applyEvents(parsed.events);
      } catch (e) {
        if (e instanceof DOMException && e.name === "AbortError") {
          setMessages((m) => {
            if (m.length === 0) {
              return m;
            }
            const copy = m.slice();
            const last = copy[copy.length - 1];
            if (last?.role === "assistant") {
              copy[copy.length - 1] = {
                ...last,
                content: assistantText || last.content || "(stopped)",
              };
            }
            return copy;
          });
        } else {
          const msg = e instanceof Error ? e.message : String(e);
          setLastError(msg);
          setMessages((m) => {
            if (m.length === 0) {
              return m;
            }
            const copy = m.slice();
            const last = copy[copy.length - 1];
            if (last?.role === "assistant") {
              copy[copy.length - 1] = {
                ...last,
                content: last.content || msg,
              };
            }
            return copy;
          });
        }
      } finally {
        setBusy(false);
        ctrlRef.current = null;
      }
    },
    [sessionId],
  );

  return { messages, send, stop, busy, lastError };
}
