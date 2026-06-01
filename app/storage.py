"""Storage adapters for durable frame variants and render artifacts."""
from __future__ import annotations

import hashlib
import hmac
import os
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Optional
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlsplit
from urllib.request import Request, urlopen
from xml.etree import ElementTree


class StorageError(RuntimeError):
    pass


@dataclass(frozen=True)
class StoredObject:
    key: str
    size: int
    capture_ts: Optional[str] = None
    variant: Optional[str] = None


def _segment(value: str, name: str) -> str:
    value = str(value or "").strip()
    if not value or value in (".", "..") or "/" in value or "\\" in value:
        raise ValueError(f"invalid {name}")
    return value


def _object_key(value: str) -> str:
    value = str(value or "").strip()
    if not value or value.startswith("/") or "\\" in value or any(part in ("", ".", "..") for part in value.split("/")):
        raise ValueError("invalid object key")
    return value


def normalize_capture_ts(value: datetime | str) -> str:
    if isinstance(value, datetime):
        dt = value
    else:
        raw = str(value or "").strip()
        try:
            dt = datetime.strptime(raw, "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)
        except ValueError:
            dt = datetime.fromisoformat(raw.replace("Z", "+00:00"))
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def frame_key(org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str, variant: str) -> str:
    ts = normalize_capture_ts(capture_ts)
    variant = _segment(variant, "variant")
    dt = datetime.strptime(ts, "%Y%m%dT%H%M%SZ")
    return (
        f"orgs/{_segment(org_id, 'org_id')}/sites/{_segment(site_id, 'site_id')}/"
        f"cameras/{_segment(camera_id, 'camera_id')}/frames/{dt:%Y/%m/%d}/{ts}/{variant}.jpg"
    )


def frame_prefix(org_id: str, site_id: str, camera_id: str) -> str:
    return (
        f"orgs/{_segment(org_id, 'org_id')}/sites/{_segment(site_id, 'site_id')}/"
        f"cameras/{_segment(camera_id, 'camera_id')}/frames/"
    )


def render_key(org_id: str, site_id: str, render_id: str, artifact_name: str) -> str:
    return (
        f"orgs/{_segment(org_id, 'org_id')}/sites/{_segment(site_id, 'site_id')}/"
        f"renders/{_segment(render_id, 'render_id')}/{_segment(artifact_name, 'artifact_name')}"
    )


def _stored_frame(key: str, size: int) -> StoredObject:
    parts = key.split("/")
    if len(parts) < 2:
        raise StorageError(f"invalid frame object key: {key}")
    return StoredObject(key=key, size=size, capture_ts=parts[-2], variant=Path(parts[-1]).stem)


class FilesystemStorage:
    def __init__(self, root: str | Path):
        self.root = Path(root)

    def _path(self, key: str) -> Path:
        return self.root / _object_key(key)

    def put_variant(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str, variant: str, data: bytes) -> StoredObject:
        key = frame_key(org_id, site_id, camera_id, capture_ts, variant)
        path = self._path(key)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return _stored_frame(key, len(data))

    def get_variant(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str, variant: str) -> Optional[bytes]:
        return self.get_object(frame_key(org_id, site_id, camera_id, capture_ts, variant))

    def get_object(self, key: str) -> Optional[bytes]:
        path = self._path(key)
        return path.read_bytes() if path.is_file() else None

    def _list(self, org_id: str, site_id: str, camera_id: str, variant: str) -> list[StoredObject]:
        prefix = frame_prefix(org_id, site_id, camera_id)
        suffix = f"/{_segment(variant, 'variant')}.jpg"
        base = self._path(prefix.rstrip("/"))
        if not base.exists():
            return []
        return sorted(
            (_stored_frame(str(path.relative_to(self.root)).replace(os.sep, "/"), path.stat().st_size)
             for path in base.rglob(f"{variant}.jpg")
             if str(path).replace(os.sep, "/").endswith(suffix)),
            key=lambda item: (item.capture_ts or "", item.key),
        )

    def latest_frame(self, org_id: str, site_id: str, camera_id: str, variant: str = "original") -> Optional[StoredObject]:
        objects = self._list(org_id, site_id, camera_id, variant)
        return objects[-1] if objects else None

    def list_range(self, org_id: str, site_id: str, camera_id: str, start_ts: datetime | str, end_ts: datetime | str, variant: str = "original") -> list[StoredObject]:
        start = normalize_capture_ts(start_ts)
        end = normalize_capture_ts(end_ts)
        return [item for item in self._list(org_id, site_id, camera_id, variant) if start <= (item.capture_ts or "") <= end]

    def delete_frame(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str) -> int:
        prefix = frame_key(org_id, site_id, camera_id, capture_ts, "placeholder").rsplit("/", 1)[0]
        folder = self._path(prefix)
        if not folder.exists():
            return 0
        deleted = 0
        for path in folder.iterdir():
            if path.is_file():
                path.unlink()
                deleted += 1
        try:
            folder.rmdir()
        except OSError:
            pass
        return deleted

    def create_render_artifact(self, org_id: str, site_id: str, render_id: str, artifact_name: str, data: bytes) -> StoredObject:
        key = render_key(org_id, site_id, render_id, artifact_name)
        path = self._path(key)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return StoredObject(key=key, size=len(data))


class S3Storage:
    def __init__(self, endpoint_url: str, bucket: str, access_key: str, secret_key: str, region: str = "us-east-1", timeout_seconds: float = 3.0):
        self.endpoint_url = endpoint_url.rstrip("/")
        self.bucket = _segment(bucket, "bucket")
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.timeout_seconds = float(timeout_seconds)
        parsed = urlsplit(self.endpoint_url)
        if parsed.scheme not in ("http", "https") or not parsed.netloc:
            raise ValueError("invalid S3 endpoint URL")

    @staticmethod
    def _sign(key: bytes, message: str) -> bytes:
        return hmac.new(key, message.encode("utf-8"), hashlib.sha256).digest()

    def _request(self, method: str, key: str = "", query: Optional[dict[str, str]] = None, data: bytes = b"", content_type: Optional[str] = None, allow_missing: bool = False) -> Optional[bytes]:
        parsed = urlsplit(self.endpoint_url)
        path = f"/{self.bucket}"
        if key:
            key = _object_key(key)
            path += f"/{key}"
        canonical_uri = quote(path, safe="/-_.~")
        canonical_query = urlencode(sorted((query or {}).items()), safe="~")
        url = f"{self.endpoint_url}{canonical_uri}"
        if canonical_query:
            url += f"?{canonical_query}"

        now = datetime.now(timezone.utc)
        amz_date = now.strftime("%Y%m%dT%H%M%SZ")
        date_stamp = now.strftime("%Y%m%d")
        payload_hash = hashlib.sha256(data).hexdigest()
        headers = {
            "host": parsed.netloc,
            "x-amz-content-sha256": payload_hash,
            "x-amz-date": amz_date,
        }
        if content_type:
            headers["content-type"] = content_type
        signed_headers = ";".join(sorted(headers))
        canonical_headers = "".join(f"{name}:{headers[name]}\n" for name in sorted(headers))
        canonical_request = "\n".join([method, canonical_uri, canonical_query, canonical_headers, signed_headers, payload_hash])
        scope = f"{date_stamp}/{self.region}/s3/aws4_request"
        string_to_sign = "\n".join(["AWS4-HMAC-SHA256", amz_date, scope, hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()])
        date_key = self._sign(("AWS4" + self.secret_key).encode("utf-8"), date_stamp)
        region_key = self._sign(date_key, self.region)
        service_key = self._sign(region_key, "s3")
        signing_key = self._sign(service_key, "aws4_request")
        signature = hmac.new(signing_key, string_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()
        headers["authorization"] = f"AWS4-HMAC-SHA256 Credential={self.access_key}/{scope}, SignedHeaders={signed_headers}, Signature={signature}"

        request = Request(url, data=data if method in ("PUT", "POST") else None, headers=headers, method=method)
        try:
            with urlopen(request, timeout=self.timeout_seconds) as response:
                return response.read()
        except HTTPError as exc:
            exc.close()
            if allow_missing and exc.code == 404:
                return None
            raise StorageError(f"S3 {method} {key or self.bucket} failed: HTTP {exc.code}") from exc
        except (OSError, URLError) as exc:
            raise StorageError(f"S3 {method} {key or self.bucket} failed: {exc}") from exc

    def put_variant(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str, variant: str, data: bytes) -> StoredObject:
        key = frame_key(org_id, site_id, camera_id, capture_ts, variant)
        self._request("PUT", key=key, data=data, content_type="image/jpeg")
        return _stored_frame(key, len(data))

    def get_variant(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str, variant: str) -> Optional[bytes]:
        return self.get_object(frame_key(org_id, site_id, camera_id, capture_ts, variant))

    def get_object(self, key: str) -> Optional[bytes]:
        return self._request("GET", key=key, allow_missing=True)

    def _list_keys(self, prefix: str) -> Iterable[StoredObject]:
        continuation = ""
        while True:
            query = {"list-type": "2", "prefix": prefix}
            if continuation:
                query["continuation-token"] = continuation
            body = self._request("GET", query=query) or b""
            root = ElementTree.fromstring(body)
            for content in root.findall("{*}Contents"):
                key = content.findtext("{*}Key") or ""
                size = int(content.findtext("{*}Size") or 0)
                yield StoredObject(key=key, size=size)
            if (root.findtext("{*}IsTruncated") or "").lower() != "true":
                return
            continuation = root.findtext("{*}NextContinuationToken") or ""
            if not continuation:
                raise StorageError("S3 list response omitted continuation token")

    def _list(self, org_id: str, site_id: str, camera_id: str, variant: str) -> list[StoredObject]:
        suffix = f"/{_segment(variant, 'variant')}.jpg"
        return sorted(
            (_stored_frame(item.key, item.size) for item in self._list_keys(frame_prefix(org_id, site_id, camera_id)) if item.key.endswith(suffix)),
            key=lambda item: (item.capture_ts or "", item.key),
        )

    def latest_frame(self, org_id: str, site_id: str, camera_id: str, variant: str = "original") -> Optional[StoredObject]:
        objects = self._list(org_id, site_id, camera_id, variant)
        return objects[-1] if objects else None

    def list_range(self, org_id: str, site_id: str, camera_id: str, start_ts: datetime | str, end_ts: datetime | str, variant: str = "original") -> list[StoredObject]:
        start = normalize_capture_ts(start_ts)
        end = normalize_capture_ts(end_ts)
        return [item for item in self._list(org_id, site_id, camera_id, variant) if start <= (item.capture_ts or "") <= end]

    def delete_frame(self, org_id: str, site_id: str, camera_id: str, capture_ts: datetime | str) -> int:
        prefix = frame_key(org_id, site_id, camera_id, capture_ts, "placeholder").rsplit("/", 1)[0] + "/"
        objects = list(self._list_keys(prefix))
        for item in objects:
            self._request("DELETE", key=item.key)
        return len(objects)

    def create_render_artifact(self, org_id: str, site_id: str, render_id: str, artifact_name: str, data: bytes) -> StoredObject:
        key = render_key(org_id, site_id, render_id, artifact_name)
        self._request("PUT", key=key, data=data, content_type="application/octet-stream")
        return StoredObject(key=key, size=len(data))


def storage_from_env(backend: Optional[str] = None):
    selected = (backend or os.getenv("STORAGE_BACKEND", "filesystem")).strip().lower()
    if selected == "filesystem":
        return FilesystemStorage(os.getenv("STORAGE_FILESYSTEM_ROOT", "/data/_storage"))
    if selected == "s3":
        required = {
            "STORAGE_S3_ENDPOINT_URL": os.getenv("STORAGE_S3_ENDPOINT_URL", ""),
            "STORAGE_S3_BUCKET": os.getenv("STORAGE_S3_BUCKET", ""),
            "STORAGE_S3_ACCESS_KEY": os.getenv("STORAGE_S3_ACCESS_KEY", ""),
            "STORAGE_S3_SECRET_KEY": os.getenv("STORAGE_S3_SECRET_KEY", ""),
        }
        missing = [name for name, value in required.items() if not value]
        if missing:
            raise StorageError(f"missing S3 storage configuration: {', '.join(missing)}")
        return S3Storage(
            endpoint_url=required["STORAGE_S3_ENDPOINT_URL"],
            bucket=required["STORAGE_S3_BUCKET"],
            access_key=required["STORAGE_S3_ACCESS_KEY"],
            secret_key=required["STORAGE_S3_SECRET_KEY"],
            region=os.getenv("STORAGE_S3_REGION", "us-east-1"),
            timeout_seconds=float(os.getenv("STORAGE_S3_TIMEOUT_SECONDS", "3")),
        )
    raise StorageError(f"unsupported storage backend: {selected}")
