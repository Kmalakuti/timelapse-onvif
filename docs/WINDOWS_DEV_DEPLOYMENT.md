# Windows Side-by-Side Dev Deployment

Use this checklist to run this clean tree beside the current Windows production stack without writing to production data.

## Validated run: 2026-05-30

This path was validated against Windows host `10.20.30.141`:

- Production remained on the base ports and data path.
- Dev stack ran on `18080`, `18081`, `18082`, and `15051`.
- Dev data path was `C:/timelapse-data-dev`.
- User logged into `http://10.20.30.141:18080`.
- Existing cameras were visible from the copied state.
- Capture started from the dev UI and wrote new files under the dev data path,
  verified through RDP.

The first successful transfer used an RDP session on the Windows host to download
`timelapse-onvif-windows-dev-deploy.zip` from a temporary Ubuntu HTTP server.
That server is not part of the deployment; recreate the package and transfer path
when needed.

## 1) Back up production state first

Before copying camera records into dev, back up:

- Current production `.env`.
- `C:/timelapse-data/_state/timelapse.sqlite`.
- `C:/timelapse-data/_state/worker.sqlite`.

Keep the production `CRED_ENC_KEY`; copied camera credentials cannot decrypt without it.

## 2) Create dev data directory

In PowerShell on the Windows host:

```powershell
New-Item -ItemType Directory -Force C:/timelapse-data-dev/_state
```

If testing with copied camera records, copy the backed-up SQLite files into `C:/timelapse-data-dev/_state/` only after the backup exists.

## 3) Create dev env file

Copy `env/windows-dev.env.example` to `.env.dev-windows` and fill in secrets.

Keep these safety values unless intentionally changing the dev stack:

```dotenv
TIMELAPSE_DATA_DIR=C:/timelapse-data-dev
WEB_PORT=18080
WORKER_REST_PORT=18081
WORKER_GATEWAY_PORT=18082
WORKER_GRPC_PORT=15051
```

Use the production `CRED_ENC_KEY` only when the dev data directory contains a copied production database.

## 4) Start the dev stack

Preferred scripted path from the repo root on the Windows host:

```powershell
.\scripts\windows-dev-deploy.ps1 -CopyProductionState
```

That script creates or validates `.env.dev-windows`, backs up production `.env` and SQLite state under `C:/timelapse-backups/`, copies SQLite state into `C:/timelapse-data-dev/_state/`, validates Compose config, starts the dev stack, and waits for web, worker REST, gateway, and gRPC readiness.

Manual equivalent:

```powershell
docker compose --env-file .env.dev-windows -f docker-compose.yml -f docker-compose.dev.yml up -d --build timelapse worker worker-grpc worker-gateway
```

Verify the web UI from another LAN machine at:

```text
http://10.20.30.141:18080
```

## 5) Stop the dev stack

```powershell
docker compose --env-file .env.dev-windows -f docker-compose.yml -f docker-compose.dev.yml down
```

Production remains on the base defaults: `C:/timelapse-data`, web `8080`, worker REST `8081`, gateway `8082`, and gRPC `50051`.
