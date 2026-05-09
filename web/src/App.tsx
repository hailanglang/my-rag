import { useEffect, useState } from "react";
import { createSession } from "./api/chat";
import { ChatPanel } from "./components/ChatPanel";
import { DocumentPanel } from "./components/DocumentPanel";
import "./App.css";

export default function App() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [sessionErr, setSessionErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const id = await createSession();
        if (!cancelled) {
          setSessionId(id);
        }
      } catch (e) {
        if (!cancelled) {
          setSessionErr(e instanceof Error ? e.message : String(e));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="app-shell">
      <header className="app-banner">
        <h1>my-rag</h1>
      </header>
      <main className="app-main">
        <DocumentPanel />
        {sessionErr ? (
          <section className="chat-panel" aria-live="polite">
            <h2>Chat</h2>
            <p className="panel-error" role="alert">
              {sessionErr}
            </p>
          </section>
        ) : (
          <ChatPanel sessionId={sessionId} />
        )}
      </main>
    </div>
  );
}
