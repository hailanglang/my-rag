export type DocumentRow = {
  id: string;
  filename: string;
  mime: string;
  size: number;
  status: string;
  error_message?: string | null;
  created_at: number;
  updated_at: number;
};

export type Citation = {
  chunk_id: string;
  document_id: string;
  document_name: string;
  quote: string;
};

export type ChatMessage = {
  role: "user" | "assistant";
  content: string;
  citations?: Citation[];
};
