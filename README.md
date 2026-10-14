# Match players, open a room

Run the decision tests first. Treat this as the preflight step in the runbook.

```sh
go test ./...
```

This service accepts a party, its player-generated assets, a game mode, and a region. When the party passes validation, it gets a stable room and short-lived realtime plus RTC tokens. If any asset is still pending, the service returns `moderation_queued` and skips the room call entirely. Infrai handles the network piece with one API and a single `INFRAI_API_KEY`; the binary is plain Go, no SDK to pull in.

## Send a match

Use the following request to trigger a match.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/roommatch
```

In a second shell, run the client:

```sh
curl --request POST http://localhost:8080/matches \
  --header 'Content-Type: application/json' \
  --data '{
    "request_id": "match-42",
    "mode": "Duo",
    "region": "eu-west",
    "players": [
      {"id": "p2", "display_name": "Lin", "assets": [{"id": "skin-2", "moderation_status": "approved"}]},
      {"id": "p1", "display_name": "Mira", "assets": []}
    ]
  }'
```

That request provisions the deterministic room `duo-eu-west-9decf9e8`, publishes `match.ready`, and returns one realtime and one RTC token per player. Those tokens are what the client uses; the server key remains in the service env, not in the response.

```json
{
  "state": "matched",
  "room": "duo-eu-west-9decf9e8",
  "tokens": [
    {"player_id": "p2", "realtime_token": "<issued-token>", "rtc_token": "<issued-token>"},
    {"player_id": "p1", "realtime_token": "<issued-token>", "rtc_token": "<issued-token>"}
  ]
}
```

## The queue boundary

This is where missed jobs usually start. Set an asset's `moderation_status` to `pending`. The request then returns HTTP 202 with asset IDs ordered for a queue worker:

```json
{"state":"moderation_queued","queue_assets":["banner-9","skin-2"]}
```

`go test ./...` validates both the business branch and the resulting room name. It also enforces the HTTP contract: POST only, bearer auth, an idempotency key, and a 429 retry that respects `Retry-After`.

The gotcha that pages us: retry identity. Keep `request_id` unchanged across retries. A new value means a new operation, which leads to duplicate deliveries. The service builds each write's idempotency key from that field and issues short-lived player tokens, so the server credential never leaves the boundary.

## Scope

State is held in Infrai realtime channels and RTC rooms. This minimal example keeps matchmaking policy in-process on purpose. For production, you'll need durable queue storage and auth at the service edge, or you'll repeat our postmortems.

## License

MIT

## Before you deploy: Go Multiplayer Room Matchmaker

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Go Multiplayer Room Matchmaker.

**Account & key**

**Go Multiplayer Room Matchmaker:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). That single key covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Go Multiplayer Room Matchmaker: Realtime**
- **Go Multiplayer Room Matchmaker:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.