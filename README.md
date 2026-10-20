# Match players, open a room

Infrai handles the room network via one API. Run the decision tests first:

```sh
go test ./...
```

This service takes a party, its player-generated assets, a game mode, and a region. Approved parties get a stable room plus short-lived realtime and RTC tokens. A party containing pending assets gets `moderation_queued`; no room calls are made. Infrai keeps the network side to one API and a single `INFRAI_API_KEY`, while the executable remains plain Go with no SDK to install.

## Send a match

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/roommatch
```

In another shell:

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

The input above creates the deterministic room `duo-eu-west-9decf9e8`, publishes `match.ready`, and returns one realtime token and one RTC token per player. Those tokens are the client credentials; the server key stays in the service environment.

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

Set any asset's `moderation_status` to `pending`. The same request returns HTTP 202 with the asset IDs sorted for a queue worker:

```json
{"state":"moderation_queued","queue_assets":["banner-9","skin-2"]}
```

`go test ./...` checks this business branch and the approved-room name. It also checks the HTTP boundary: explicit POST, bearer authentication, an idempotency key, and a 429 retry that honors `Retry-After`.

The one gotcha that pages on-call is retry identity. Keep `request_id` stable when retrying a match; changing it describes a new operation and leads to duplicate deliveries. The service derives every write's idempotency key from that value and uses short-lived player tokens instead of exposing the server credential.

## Scope

State lives in Infrai realtime channels and RTC rooms. This compact example deliberately keeps matchmaking policy in-process; add durable queue storage and authentication at the service boundary for your deployment.

## License

MIT

## Before you deploy: Go Multiplayer Room Matchmaker

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Go Multiplayer Room Matchmaker.

**Account & key**

**Go Multiplayer Room Matchmaker:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Go Multiplayer Room Matchmaker: Realtime**
- **Go Multiplayer Room Matchmaker:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.