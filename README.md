# Timelapse OAI

FastAPI-based LAN tool for managing IP cameras, capturing timelapse JPEGs with ffmpeg, and rendering MP4 timelapses. The app uses Docker Compose, SQLite, Jinja templates, and a worker/gateway split for capture operations.

## Quick Start

1. Install Docker Desktop.
2. Clone the repository.
3. Copy `.env.example` to `.env`.
4. Fill in `ADMIN_PASSWORD_HASH`, `SESSION_SECRET`, and `CRED_ENC_KEY` using the commands in `.env.example`.
5. Start the stack:

```powershell
docker compose up -d --build
```

6. Open http://localhost:8080 and log in with the admin username/password configured in `.env`.

Runtime camera frames, renders, and SQLite state are stored outside the repo at `C:/timelapse-data` by the default Compose file.

## Important Files

- `PROJECT_OVERVIEW.md`: architecture and component summary.
- `SECURITY_SETUP.md`: required secrets, HTTPS notes, and security caveats.
- `docs/NEW_INSTALL_CHECKLIST.md`: first-time deployment checklist.
- `docker-compose.yml`: local multi-service stack.

## Security Notes

Do not commit `.env`, screenshots, handoff notes, runtime databases, camera frame data, or temporary login helpers. The included `.gitignore` and `.dockerignore` exclude those local artifacts.
