import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
} from "react";
import { deleteDocument, listDocuments, uploadDocument } from "../api/documents";
import type { DocumentRow } from "../api/types";

export function DocumentPanel() {
  const [docs, setDocs] = useState<DocumentRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setErr(null);
    try {
      setDocs(await listDocuments());
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const onPickFile = () => {
    fileRef.current?.click();
  };

  const onFileChange = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) {
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await uploadDocument(file);
      await refresh();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex));
    } finally {
      setBusy(false);
    }
  };

  const onDelete = async (id: string) => {
    setBusy(true);
    setErr(null);
    try {
      await deleteDocument(id);
      await refresh();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="doc-panel" aria-labelledby="documents-heading">
      <div className="panel-head">
        <h2 id="documents-heading">Documents</h2>
        <div className="panel-actions">
          <input
            ref={fileRef}
            type="file"
            className="visually-hidden"
            onChange={onFileChange}
            disabled={busy}
          />
          <button type="button" onClick={onPickFile} disabled={busy}>
            Upload
          </button>
          <button type="button" onClick={() => void refresh()} disabled={busy || loading}>
            Refresh
          </button>
        </div>
      </div>
      {err ? (
        <p className="panel-error" role="alert">
          {err}
        </p>
      ) : null}
      {loading ? (
        <p className="muted">Loading…</p>
      ) : (
        <ul className="doc-list">
          {docs.length === 0 ? (
            <li className="muted">No documents yet.</li>
          ) : (
            docs.map((d) => (
              <li key={d.id} className="doc-row">
                <div className="doc-main">
                  <div className="doc-name">{d.filename}</div>
                  <div className="doc-meta">
                    <span className={`status status-${d.status}`}>{d.status}</span>
                    {d.error_message ? (
                      <span className="doc-err" title={d.error_message}>
                        {d.error_message}
                      </span>
                    ) : null}
                  </div>
                </div>
                <button type="button" onClick={() => void onDelete(d.id)} disabled={busy}>
                  Delete
                </button>
              </li>
            ))
          )}
        </ul>
      )}
    </section>
  );
}
