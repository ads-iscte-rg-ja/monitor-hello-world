# Monitor hello-world

A small example application using the [Go SDK](../monitor-go-sdk/README.md).
It writes sample logs and a recovered panic to the local SQLite buffer;
[remote](../monitor-remote/README.md) delivers them to central.

## Setup

- Use **Go 1.25+**, a **C compiler**, and **`CGO_ENABLED=1`** for SQLite.
- The SDK is the published module `github.com/ads-iscte-rg-ja/monitor-go-sdk`
  (pinned in `go.mod`). To try unreleased SDK changes, create a local `go.work`
  with `../monitor-platform/scripts/go-work.sh`.
- Initialize the database through remote first. Start the agent to deliver
  records to central, or leave it stopped to try local buffering.

## Run

From `monitor-hello-world/`, point the example at remote's exact SQLite file:

```sh
export MONITOR_DB_PATH=../monitor-remote/remote-monitor.db
CGO_ENABLED=1 go run . -scenario logs
```

Repeat the log set or try a recovered panic:

```sh
CGO_ENABLED=1 go run . -scenario logs -count 10
CGO_ENABLED=1 go run . -scenario panic
```

| Option | Meaning |
| --- | --- |
| `-db` | SQLite path; overrides `MONITOR_DB_PATH` |
| `-scenario logs` | Write one record at each level: INFO, WARN, ERROR, CRITICAL, PANIC |
| `-scenario panic` | Recover an intentional panic and queue its stack trace |
| `-count` | Repeat the log set, from 1 to 1000; default 1 |

The program reads environment variables and flags; it **does not load `.env`**.
Use an absolute path if you configured a different buffer location. No central
URL or API key is required.

A successful run means records were queued locally. With remote running, check
central's `/errors` page for application **hello-world**, environment **testing**.
