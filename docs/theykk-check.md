# Theykk connection check

Date: 2026-09-19.

- Endpoint: `https://ml.theykk.net/v1`.
- OwnCode model ID: `theykk/qwen38`.
- API model ID: `qwen38`.
- Context limit: 262144 tokens, from the supplied configuration.
- Output limit: 32768 tokens, from the supplied configuration.
- Sampling: temperature 0.6, top_p 0.95, top_k 20.
- Thinking: enable_thinking and preserve_thinking enabled.

The authenticated model list returned HTTP 200 and included `qwen38`.
A non-interactive OwnCode request returned `OWN_CODE_CONNECTION_OK` with exit code 0.
The request ran in a temporary directory with no project documents.

Local HTTP tests verified authentication, sampling fields, streamed reasoning, tool-call parsing, and reasoning preservation across messages.
The tests also verified a clear error for an empty response.
The live check did not exercise image input or the full context limit.

The MCP executable completed initialization and returned eight tools.
The list included `search_graph`, `trace_path`, and `get_code_snippet`.

The private configuration uses `.owncode.local.json`, with file mode 0600.
Git ignores that file. The example contains only a placeholder key.
