import { type FormEvent, useState } from "react";
import { useChatStream } from "../hooks/useChatStream";

type Props = {
  sessionId: string | null;
};

export function ChatPanel({ sessionId }: Props) {
  const { messages, send, stop, busy, lastError } = useChatStream(sessionId);
  const [draft, setDraft] = useState("");

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    void send(draft);
    setDraft("");
  };

  return (
    <section className="chat-panel" aria-labelledby="chat-heading">
      <div className="panel-head">
        <h2 id="chat-heading">Chat</h2>
      </div>
      {lastError ? (
        <p className="panel-error" role="alert">
          {lastError}
        </p>
      ) : null}
      {!sessionId ? <p className="muted">Starting session…</p> : null}
      <div className="chat-log" aria-live="polite">
        {messages.map((m, i) => (
          <article key={i} className={`bubble bubble-${m.role}`}>
            <div className="bubble-label">{m.role}</div>
            <div className="bubble-text">{m.content}</div>
            {m.citations && m.citations.length > 0 ? (
              <details className="citations">
                <summary>Citations ({m.citations.length})</summary>
                <ol>
                  {m.citations.map((c) => (
                    <li key={c.chunk_id}>
                      <div className="cite-doc">{c.document_name}</div>
                      <blockquote>{c.quote}</blockquote>
                    </li>
                  ))}
                </ol>
              </details>
            ) : null}
          </article>
        ))}
      </div>
      <form className="chat-form" onSubmit={onSubmit}>
        <label className="chat-label" htmlFor="chat-input">
          Message
        </label>
        <textarea
          id="chat-input"
          name="message"
          rows={3}
          placeholder="Ask a question about your documents…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          disabled={!sessionId || busy}
        />
        <div className="chat-actions">
          <button type="submit" disabled={!sessionId || busy}>
            Send
          </button>
          <button type="button" onClick={stop} disabled={!busy}>
            Stop
          </button>
        </div>
      </form>
    </section>
  );
}
