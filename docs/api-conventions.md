# API Conventions

Source of truth for HTTP responses. Code: `internal/delivery/http/response`, `internal/delivery/http/middleware/errors.go`, `internal/pkg/apperr`.

## Envelope

Success (`response.Success`):

```json
{"data": {"token": "...", "user": {"id": "...", "email": "a@example.com"}}, "meta": {"request_id": "0198..."}}
```

Error (`response.Error`):

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "validation failed",
    "fields": [{"field": "email", "description": "email is required"}]
  },
  "meta": {"request_id": "0198..."}
}
```

- `data`, `error`, `meta` and `fields` use `omitzero`; absent means not applicable. A 204 status writes no body.
- `code` is a stable machine-readable string; `message` is client-safe. Clients branch on `code`, not `message`.
- Handlers return errors; `middleware.HandleErrors` renders them. Echo's own errors (404, 405, 413, 415, 408, bad JSON) go through the same path, keeping echo's HTTP status while the body comes from the mapped sentinel.
- `GET /health` returns `{"data": {"status": "ok"}, ...}`.

## Kind to HTTP status

`response.StatusFor` is the only mapping (unknown kinds fall back to 500).

| apperr.Kind | HTTP |
| --- | --- |
| InvalidArgument | 400 |
| Unauthenticated | 401 |
| PermissionDenied | 403 |
| NotFound | 404 |
| Conflict | 409 |
| FailedPrecondition | 422 (not 412, which is for conditional headers) |
| TooManyRequests | 429 |
| Unavailable | 503 |
| Internal | 500 |

## Kind to gRPC code

`kindToCode` in `internal/delivery/grpc/errors.go`. The status carries `errdetails.ErrorInfo{Reason: <code>, Domain: "server-template"}`; the RPC client (`internal/infrastructure/rpc/errors.go`) maps it back to `*apperr.Error`, preserving kind and code across the hop.

| apperr.Kind | gRPC code |
| --- | --- |
| InvalidArgument | InvalidArgument |
| NotFound | NotFound |
| Conflict | AlreadyExists |
| Unauthenticated | Unauthenticated |
| PermissionDenied | PermissionDenied |
| FailedPrecondition | FailedPrecondition |
| TooManyRequests | ResourceExhausted |
| Unavailable | Unavailable |
| Internal | Internal |

Inbound extras mapped to a kind by the client: OutOfRange -> InvalidArgument, Aborted -> Conflict, DeadlineExceeded / Canceled -> Unavailable.

## Redaction

- `Internal` and `Unavailable`: body is always `INTERNAL_ERROR` / `SERVICE_UNAVAILABLE` with the generic message and no `fields`, whatever the original error said. gRPC does the same for the status message.
- `Unauthenticated` and `PermissionDenied`: `code` and `message` only; `fields` dropped.
- Other kinds: `code`, `message`, and `fields`.
- Causes and stacks never reach clients; they go to logs (and span events, without `err.Error()`).

## Field violations

`apperr.FieldViolation{Field, Description}`, attached with `err.WithFields(...)`. `request.Bind*` produces `VALIDATION_FAILED` with one violation per failed validator rule; malformed input yields `INVALID_INPUT` (400). Password bounds are in `RegisterRequest` tags and `entity`; strength is checked in the usecase (`WEAK_PASSWORD`).

## Headers

- `X-Request-Id`: an inbound value is reused if 1-128 chars of `[A-Za-z0-9-_.:]`, otherwise a UUIDv7 is generated. It is echoed in the response header, in `meta.request_id`, and in every log line for the request.
- `traceparent` (and `tracestate`): written on the response by `middleware.Traceparent` when OTel is enabled and a span is active, so a client or support can find the trace.

## JSON serializer

`response.JSONSerializer` backs echo with `encoding/json/v2` (not v1 semantics):

- member names match case-sensitively; duplicate keys and invalid UTF-8 are rejected (request body failures become 400 `INVALID_INPUT`);
- nil slices/maps encode as `[]` / `{}`, not `null`;
- use `omitzero` rather than `omitempty` for zero values (as the envelope does);
- no trailing newline after the encoded value.
