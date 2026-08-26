# Files

- [LLM Drivers](drivers.md) - The internal/llm provider-agnostic streaming chat interface: the Driver/PartStream contract, the OpenAI- and Anthropic-compatible HTTP/SSE drivers, the shared SSE pump, and the scripted fake driver (YOLO_LLM=fake) that keeps unit tests network-free.
- [Provider Registry and Catalogs](providers.md) - internal/provider builds the model/provider catalog (kido live probe with static fallback, the cached opencode zen catalog, and config-defined providers), resolves provider/model refs to a driver, and internal/auth resolves API keys (env, auth.json, config).
