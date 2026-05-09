import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

function mockDefaultFetch() {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = typeof input === "string" ? input : (input as Request).url;
    const method = init?.method ?? "GET";

    if (url.endsWith("/api/sessions") && method === "POST") {
      return new Response(JSON.stringify({ id: "sid-1" }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      });
    }

    if (url.endsWith("/api/documents") && method === "GET") {
      return new Response("[]", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    return new Response(`unmocked: ${method} ${url}`, { status: 500 });
  });
}

describe("App", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders chat and documents", async () => {
    mockDefaultFetch();
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^send$/i })).toBeInTheDocument();
    });
    expect(
      screen.getByRole("heading", { level: 2, name: /^documents$/i }),
    ).toBeInTheDocument();
  });

  it("accumulates streamed assistant text from SSE", async () => {
    const user = userEvent.setup();
    const enc = new TextEncoder();

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      const method = init?.method ?? "GET";

      if (url.endsWith("/api/sessions") && method === "POST") {
        return new Response(JSON.stringify({ id: "sid-1" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (url.endsWith("/api/documents") && method === "GET") {
        return new Response("[]", {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (url.includes("/api/sessions/") && url.endsWith("/messages")) {
        const chunks = [
          "event: citations\ndata: []\n\n",
          `event: token\ndata: ${JSON.stringify({ t: "Hello" })}\n\n`,
          `event: token\ndata: ${JSON.stringify({ t: " world" })}\n\n`,
          "event: done\ndata: {}\n\n",
        ];
        const stream = new ReadableStream({
          start(controller) {
            for (const c of chunks) {
              controller.enqueue(enc.encode(c));
            }
            controller.close();
          },
        });
        return new Response(stream, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" },
        });
      }

      return new Response(`unmocked: ${method} ${url}`, { status: 500 });
    });

    render(<App />);
    await waitFor(() =>
      screen.getByRole("button", { name: /^send$/i }),
    );

    await user.type(
      screen.getByPlaceholderText(/ask a question/i),
      "hello",
    );
    await user.click(screen.getByRole("button", { name: /^send$/i }));

    await waitFor(() => {
      expect(screen.getByText("Hello world")).toBeInTheDocument();
    });
  });

  it("calls abort when Stop is clicked during an open stream", async () => {
    const user = userEvent.setup();
    const enc = new TextEncoder();
    const abortSpy = vi.fn();

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      const method = init?.method ?? "GET";

      if (url.endsWith("/api/sessions") && method === "POST") {
        return new Response(JSON.stringify({ id: "sid-1" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (url.endsWith("/api/documents") && method === "GET") {
        return new Response("[]", {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (url.includes("/abort")) {
        abortSpy(url);
        return new Response(null, { status: 204 });
      }

      if (url.includes("/api/sessions/") && url.endsWith("/messages")) {
        const stream = new ReadableStream({
          start(controller) {
            controller.enqueue(
              enc.encode("event: citations\ndata: []\n\n"),
            );
          },
        });
        return new Response(stream, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" },
        });
      }

      return new Response(`unmocked: ${method} ${url}`, { status: 500 });
    });

    render(<App />);
    await waitFor(() =>
      screen.getByRole("button", { name: /^send$/i }),
    );

    await user.type(screen.getByPlaceholderText(/ask a question/i), "wait");
    await user.click(screen.getByRole("button", { name: /^send$/i }));

    const stop = await waitFor(() => screen.getByRole("button", { name: /stop/i }));
    await waitFor(() => expect(stop).not.toBeDisabled());
    await user.click(stop);

    await waitFor(() => {
      expect(abortSpy).toHaveBeenCalled();
    });
    const calledUrl = String(abortSpy.mock.calls[0][0]);
    expect(calledUrl).toContain("/api/sessions/");
    expect(calledUrl).toContain("/abort");
  });
});
