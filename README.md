# Barge - Tiny PaaS

A single‑binary, Docker‑based PaaS that lets you deploy containerised apps from a Git repo with just one command.

## Current features

- **deploy** - clone, build, run (assigns a free port, persists state)
- **logs** - follow container logs
- **stop/start** - pause & resume running apps
- **delete** - remove container, image, and local clone
- **list** - show all managed apps

## How it works

Barge stores a JSON state file in `~/.barge/state.json`.
It uses the `docker` and `git` CLIs directly.
Each app gets a unique host port and a Docker container named `barge-<appname>`.

## Prerequisites

- Golang
- Docker (running, accessible from your user)
- Git

## Build & run

```bash
go build -o barge
./barge deploy myapp https://github.com/heroku/node-js-sample
# access at http://localhost:<port>
```

## CLI reference

```
barge deploy <app-name> [git-url]   # deploy or update an app
barge logs <app-name>               # stream logs
barge stop <app-name>               # stop a running app
barge start <app-name>              # start a stopped app
barge delete <app-name>             # remove everything
barge list                          # show all apps
```

## Planned extensions

- **Reverse proxy & custom domains** - integrate Caddy/Traefik for automatic routing (e.g. `app.lvh.me`)
- **Git push deployment** - add an SSH endpoint to receive `git push` and trigger deploys
- **Build‑stage caching** - speed up rebuilds with Docker layer caching
- **Health‑checks & zero‑downtime rollouts** - replace containers gracefully
- **TLS / Let’s Encrypt** - automatic HTTPS for all apps
- **Web dashboard** - simple UI for managing apps
- **Config & secrets management** - env‑file or CLI injection
