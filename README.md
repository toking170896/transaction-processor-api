# Transaction Processor

A small Go service that processes incoming transaction requests from external providers.
It updates user balances, prevents the same transaction from being applied twice, and works correctly under concurrent requests.

The service uses PostgreSQL for storage and exposes a REST API (Gin) with Swagger docs.

---

## What it does

* Accepts **win / lost** transactions
* Updates user balance inside a database transaction
* Ensures the same `transaction_id` is not applied twice
* Returns clear errors for invalid requests
* Runs a background job that cancels the latest matching transactions and adjusts balances

---

## Project structure

```
cmd/server            App entry point
internal/handler      HTTP handlers and routing
internal/service      Business logic
internal/repository   DB access (Postgres)
internal/worker       Background cancellation job
internal/model        Models, types, errors
internal/test         E2E tests
migrations            Database schema changes
```

---

## Requirements

* Go (project uses modules)
* Docker + Docker Compose (recommended for running locally)
* Optional: `make` (just a helper wrapper, but recommended)

---

## Configuration

The project uses environment variables for configuration.

An example configuration is provided in:

```

.env.example

````

To customize values, create your own `.env` file in the project root:

```bash
cp .env.example .env
````

Docker Compose automatically loads variables from `.env` if the file exists.

## Quick start (Docker)

Start the app + database:

```bash
make docker-up
```
> **Note:** Database migrations are applied automatically as part of the Docker startup (see `docker-compose.yml`).


Stop everything:

```bash
make docker-down
```

Swagger UI:

```
http://localhost:8080/swagger/index.html
```

Health endpoint:

```
http://localhost:8080/health
```

---

## Run locally (without Docker)

Run the API locally:

```bash
make run
```

This assumes you already have a PostgreSQL database running and your env vars are set correctly (see `internal/config`).

---

## Tests

### Unit + handler tests (no E2E)

Runs the full test suite **except** end-to-end tests:

```bash
make test
```

(Internally it sets `SKIP_E2E=1`.)

### End-to-end tests (requires DB)

Runs only the E2E suite under `internal/test` (needs PostgreSQL available):

```bash
make test-e2e
```

---

## Notes

* Balance updates happen inside database transactions
* Duplicate transaction requests are safely handled
* Concurrency is tested with E2E tests that send multiple requests at the same time

---

Here’s a clean, simple **last section** you can copy-paste directly into your README.
It keeps the language straightforward and avoids heavy or “academic” terms.

---

## Possible extensions

This project focuses on the core transaction flow required by the task.
In a real production system, the following improvements could be added:

* **API authorization**

  The API currently has no auth layer. In a real system, requests would be authenticated and authorized (for example, API keys or JWTr).

* **More flexible balance handling**

  Balances are handled using `decimal.Decimal`, which works well for fiat currencies.
  If support e.g. for crypto was required, the balance model
  could be extended to support different precision rules per currency.

* **Provider separation**

  Requests from different external providers could be isolated by provider ID, with separate rate limits, quotas, or validation rules.

* **Improved monitoring**

  Metrics e.g request counts, failures, processing time and structured alerts
  could be added to make system behavior easier to observe under load.

---
