# Casbin Gateway LLM API Gateway

Casbin Gateway works as a unified LLM API gateway. Instead of giving every
client the credentials of your upstream AI providers, you register the
providers once as **channels**, issue **tokens** to your clients, and they
call one stable endpoint: `POST /v1/chat/completions`.

```
client ──(Bearer sk-...)──▶ gateway ──▶ channel (OpenAI / OpenAI-compatible upstream)
                             │  checks: token validity, model allowlist,
                             │          rate limit, expiration
```

## 1. Configure a Channel

A channel is one upstream AI provider. Go to **Channels** in the console and
create one:

| Field      | Description                                                                              |
|------------|------------------------------------------------------------------------------------------|
| Display name | A human-friendly name                                                                   |
| Type       | `openai` for OpenAI or any OpenAI-compatible upstream (e.g. DeepSeek, Moonshot, a local vLLM) |
| Base URL   | The provider's API root, **without the `/v1` path** — the gateway appends `/v1/chat/completions` itself |
| API Key    | The provider's API key. Stored as plaintext (the project is locally deployed; encryption is planned for later) |
| Models     | The model names this channel serves, e.g. `gpt-4o`. The gateway routes a request to a channel only when the requested model is listed here |
| Priority   | Smaller number = higher priority. When several channels serve the same model, the gateway tries them in this order |
| Status     | `enabled` or `disabled`                                                                  |

> ⚠️ **Base URL pitfall**: for OpenAI the Base URL is `https://api.openai.com`,
> **not** `https://api.openai.com/v1`. If you include `/v1`, the gateway would
> call `.../v1/v1/chat/completions` and every request fails.

Use **Test Connectivity** to probe the channel before saving. Only `openai`
and `custom` channel types can be used by the proxy at this stage.

## 2. Issue a Token

Go to **Tokens** in the console and create one:

| Field        | Description                                                                   |
|--------------|-------------------------------------------------------------------------------|
| Name         | Unique identifier of the token                                                |
| Display name | A human-friendly name                                                         |
| Allowed models | The models this token may request. Empty = all models are allowed           |
| Rate limit   | Maximum requests per minute per token. `0` = unlimited                        |
| Expire time  | Optional expiration (RFC3339). Empty = never expires                          |
| Status       | `enabled` or `disabled`                                                       |

The full secret key (`sk-...`) is shown **only once**, right after creation —
copy it immediately. The list view only shows a masked prefix (`sk-abcde****`),
and the stored key is never sent back to the browser.

## 3. Call the Endpoint

The endpoint is OpenAI-compatible: `POST /v1/chat/completions` with
`Authorization: Bearer sk-...`.

### curl

```bash
# Non-streaming
curl https://<gateway-host>/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Streaming (SSE)
curl -N https://<gateway-host>/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "stream": true,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### OpenAI SDK

Point the SDK's `base_url` at the gateway — the rest of your code stays the
same:

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-...",  # the gateway token, not the upstream provider's key
    base_url="https://<gateway-host>/v1",
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(resp.choices[0].message.content)
```

## 4. Access Control Semantics

- **Authentication**: the token's secret key is compared against the stored
  key. Invalid or missing `Authorization` → `401 authentication_error`.
- **Disabled token** → `403 permission_error`.
- **Expiration**: a non-empty expire time is parsed as RFC3339. An expired
  token → `403`; a malformed value is rejected both at save time and at
  request time (fail closed), so it can never silently act as "never expires".
- **Model allowlist**: when the token lists allowed models, requesting a model
  that is not listed → `403 permission_error`. An empty allowlist means no
  restriction.
- **Rate limit**: the counter is a per-minute sliding window per token.
  Exceeding it → `429 rate_limit_error`.

All errors use the OpenAI error shape:

```json
{"error": {"message": "...", "type": "..."}}
```

## 5. Current Limitations

- Channel API keys and token secret keys are stored as plaintext. The project
  is locally deployed; encryption is planned for later.
- Only OpenAI-compatible upstreams (`openai` / `custom` channel types) are
  supported by the proxy. `claude` and `gemini` channel types need request
  translation, which is not implemented yet.
- Token authentication protects the chat completions endpoint only; other
  gateway endpoints keep their existing Casdoor-based access control.
