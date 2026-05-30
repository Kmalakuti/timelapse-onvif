# Security setup (MVP hardening)

This version adds:
- Login + signed session cookies
- CSRF protection on POST actions
- Camera credentials encrypted at rest in SQLite
- Safer `ffmpeg` launches (no shell script command construction)
- Basic security headers (CSP, X-Frame-Options, etc.)

## 1) Set required environment variables

Copy `.env.example` to `.env` and fill in:

- `ADMIN_PASSWORD_HASH` (PBKDF2 hash, not plaintext)
- `SESSION_SECRET` (random string)
- `CRED_ENC_KEY` (Fernet key for encrypting camera creds)

Generate values:

```bash
# ADMIN_PASSWORD_HASH
python -c "from app.auth import hash_password; print(hash_password('change-me'))"

# SESSION_SECRET
python -c "import secrets; print(secrets.token_urlsafe(48))"

# CRED_ENC_KEY (32 bytes, base64)
python -c "import base64, os; print(base64.urlsafe_b64encode(os.urandom(32)).decode())"
```

## 2) Cookies over HTTP vs HTTPS

`SESSION_HTTPS_ONLY` controls whether the browser will only send the session cookie over HTTPS.

- If you are **testing locally over plain http://**, set `SESSION_HTTPS_ONLY=0`
- If you put the app **behind HTTPS**, set `SESSION_HTTPS_ONLY=1` (recommended)

## 3) Encrypting “in flight” (HTTPS)

The app itself does not terminate TLS. Put it behind a reverse proxy (Caddy/Nginx/Traefik) and serve HTTPS.

Minimum viable example (Caddy, self-signed for LAN):
- Caddyfile:
  ```
  :443 {
    tls internal
    reverse_proxy timelapse:8080
  }
  ```
- Expose 443 from the proxy container and set `SESSION_HTTPS_ONLY=1`.

## 4) Rotating the credential encryption key (future)

Right now, `CRED_ENC_KEY` is required and used to encrypt/decrypt camera usernames/passwords.
If you change it, existing encrypted rows will not decrypt.
Key rotation can be added by supporting multiple keys (decrypt with any, encrypt with primary).

## 5) What still isn’t perfect (but much better)

- RTSP creds still have to be passed to ffmpeg (most cameras require it in the URL).
  We avoid logging it, but a privileged attacker on the host could still inspect process args.
- For “real” hardening: lock down host access, use a VPN, isolate the container, and run as non-root.
