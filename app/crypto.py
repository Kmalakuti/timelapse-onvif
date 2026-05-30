import os
from typing import Optional

ENC_PREFIX = "enc:"

def _require_key() -> bytes:
    """
    Key must be a urlsafe-base64-encoded 32-byte key (Fernet).
    Generate: python -c "import base64, os; print(base64.urlsafe_b64encode(os.urandom(32)).decode())"
    """
    key = os.getenv("CRED_ENC_KEY", "").strip()
    if not key:
        raise RuntimeError("Missing CRED_ENC_KEY env var (required to encrypt camera credentials at rest).")
    return key.encode("ascii")

def _fernet():
    from cryptography.fernet import Fernet
    return Fernet(_require_key())

def is_encrypted(value: Optional[str]) -> bool:
    return bool(value) and value.startswith(ENC_PREFIX)

def encrypt_str(value: Optional[str]) -> Optional[str]:
    if value is None:
        return None
    if is_encrypted(value):
        return value
    f = _fernet()
    token = f.encrypt(value.encode("utf-8")).decode("ascii")
    return ENC_PREFIX + token

def decrypt_str(value: Optional[str]) -> Optional[str]:
    if value is None:
        return None
    if not is_encrypted(value):
        return value
    f = _fernet()
    token = value[len(ENC_PREFIX):].encode("ascii")
    return f.decrypt(token).decode("utf-8")
