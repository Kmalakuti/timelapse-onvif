import os, base64, hmac, hashlib
from typing import Tuple

# Stored format:
# pbkdf2_sha256$<iterations>$<salt_b64>$<hash_b64>

def hash_password(password: str, iterations: int = 210_000) -> str:
    if not isinstance(password, str) or password == "":
        raise ValueError("password must be non-empty string")
    salt = os.urandom(16)
    dk = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, iterations, dklen=32)
    return "pbkdf2_sha256$%d$%s$%s" % (
        iterations,
        base64.urlsafe_b64encode(salt).decode("ascii").rstrip("="),
        base64.urlsafe_b64encode(dk).decode("ascii").rstrip("="),
    )

def _b64d(s: str) -> bytes:
    pad = "=" * ((4 - len(s) % 4) % 4)
    return base64.urlsafe_b64decode(s + pad)

def verify_password(password: str, stored: str) -> bool:
    try:
        alg, iters_s, salt_b64, hash_b64 = stored.split("$", 3)
        if alg != "pbkdf2_sha256":
            return False
        iterations = int(iters_s)
        salt = _b64d(salt_b64)
        want = _b64d(hash_b64)
        got = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, iterations, dklen=len(want))
        return hmac.compare_digest(got, want)
    except Exception:
        return False

def load_admin_credentials() -> Tuple[str, str]:
    """
    Returns (username, password_hash).
    Env:
      ADMIN_USERNAME (default: admin)
      ADMIN_PASSWORD_HASH (required)
    """
    username = os.getenv("ADMIN_USERNAME", "admin")
    pw_hash = os.getenv("ADMIN_PASSWORD_HASH", "")
    if not pw_hash:
        raise RuntimeError(
            "Missing ADMIN_PASSWORD_HASH env var. Generate one with: "
            "python -c \"from app.auth import hash_password; print(hash_password('change-me'))\""
        )
    return username, pw_hash
