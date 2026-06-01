import os
import threading
import uuid
import statistics
import subprocess
import shutil
import json
from pathlib import Path
from datetime import datetime, timezone
from typing import Dict, Any, List, Optional, Tuple

from PIL import Image, ImageDraw, ImageFont
from app import db, metadata
from app.storage import storage_from_env

DATA_DIR = Path(os.getenv("DATA_DIR", "/data"))
RENDER_DIR = DATA_DIR / "_renders"
TMP_DIR = RENDER_DIR / "_tmp"

FONTFILE = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"

JOBS: Dict[str, Dict[str, Any]] = {}
LOCK = threading.Lock()


def _bool(val: Any) -> bool:
    try:
        if isinstance(val, bool):
            return val
        return bool(int(val))
    except Exception:
        return bool(val)


def _hydrate_job(data: Dict[str, Any]) -> Dict[str, Any]:
    job = dict(data) if data else {}
    req = job.get("request_json")
    if req and isinstance(req, str):
        try:
            parsed = json.loads(req)
            for k, v in parsed.items():
                job.setdefault(k, v)
        except Exception:
            pass

    # Normalize booleans and defaults
    job["filter_bad"] = _bool(job.get("filter_bad", False))
    job["overlay_name"] = _bool(job.get("overlay_name", False))
    job["overlay_timestamp"] = _bool(job.get("overlay_timestamp", False))
    job.setdefault("start_ts", "")
    job.setdefault("end_ts", "")
    job.setdefault("status", "unknown")
    job.setdefault("output_path", None)
    job.setdefault("artifact_key", None)
    job.setdefault("error", None)
    return job


def _norm_ts(s: str) -> str:
    s = (s or "").strip()
    if not s:
        return ""
    if s.endswith("Z") and "T" in s and "-" in s:
        dt = datetime.fromisoformat(s.replace("Z", "+00:00"))
        return dt.astimezone(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return s


def _parse_ts_from_stem(stem: str) -> Optional[datetime]:
    try:
        return datetime.strptime(stem, "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)
    except Exception:
        return None


def _escape_concat_path(p: Path) -> str:
    return str(p).replace("'", r"'\''")


def _list_frames(folder: Path) -> List[Path]:
    if not folder.exists():
        return []
    frames = list(folder.rglob("*.jpg"))
    frames.sort(key=lambda p: (p.stem, str(p)))
    return frames


def frame_range(camera_name: str) -> Tuple[Optional[str], Optional[str], int]:
    folder = DATA_DIR / camera_name
    frames = _list_frames(folder)
    if not frames:
        return None, None, 0
    return frames[0].stem, frames[-1].stem, len(frames)


def select_frames(camera_name: str, start_ts: str, end_ts: str) -> List[Path]:
    folder = DATA_DIR / camera_name
    frames = _list_frames(folder)
    if not frames:
        return []

    start_ts = _norm_ts(start_ts)
    end_ts = _norm_ts(end_ts)

    if not start_ts:
        start_ts = frames[0].stem
    if not end_ts:
        end_ts = frames[-1].stem

    return [p for p in frames if start_ts <= p.stem <= end_ts]



def select_storage_frames(app_camera_id: int, start_ts: str, end_ts: str, workspace: Path, adapter=None) -> List[Path]:
    adapter = adapter or storage_from_env()
    rows = metadata.list_range(app_camera_id, _norm_ts(start_ts), _norm_ts(end_ts), variant="original")
    workspace.mkdir(parents=True, exist_ok=True)
    frames: List[Path] = []
    for row in rows:
        key = row["variants"]["original"]["object_key"]
        data = adapter.get_object(key)
        if data is None:
            raise RuntimeError(f"Uploaded original is missing: {key}")
        path = workspace / f"{row['capture_ts']}.jpg"
        path.write_bytes(data)
        frames.append(path)
    return frames


def filter_bad_frames_by_size(frames: List[Path], min_ratio: float = 0.60) -> List[Path]:
    if len(frames) < 10:
        return frames
    sizes = [p.stat().st_size for p in frames]
    med = statistics.median(sizes)
    cutoff = med * float(min_ratio)
    return [p for p in frames if p.stat().st_size >= cutoff]


def write_concat_list(frames: List[Path], fps: int, list_path: Path) -> None:
    if not frames:
        raise RuntimeError("No frames to write concat list")

    dt = 1.0 / float(fps)
    lines: List[str] = []
    for p in frames:
        lines.append(f"file '{_escape_concat_path(p)}'")
        lines.append(f"duration {dt:.10f}")
    lines.append(f"file '{_escape_concat_path(frames[-1])}'")  # repeat last
    list_path.parent.mkdir(parents=True, exist_ok=True)
    list_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _load_font(size: int = 32) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(FONTFILE, size)


def stamp_frames_exact(
    frames: List[Path],
    out_dir: Path,
    camera_name: str,
    overlay_name: bool,
    overlay_timestamp: bool,
) -> List[Path]:
    """
    Stamp overlays onto copies of frames.
    Timestamp is EXACT, parsed from filename stem (YYYYMMDDTHHMMSSZ).
    """
    out_dir.mkdir(parents=True, exist_ok=True)
    font = _load_font(32)

    stamped: List[Path] = []
    pad = 14

    for src in frames:
        dst = out_dir / src.name

        img = Image.open(src).convert("RGB")
        draw = ImageDraw.Draw(img)

        y = pad

        if overlay_name:
            draw.text(
                (pad, y),
                camera_name,
                font=font,
                fill=(255, 255, 255),
                stroke_width=3,
                stroke_fill=(0, 0, 0),
            )
            # advance y by approx font height + padding
            bbox = draw.textbbox((0, 0), camera_name, font=font, stroke_width=3)
            y += (bbox[3] - bbox[1]) + pad

        if overlay_timestamp:
            dt = _parse_ts_from_stem(src.stem)
            ts = dt.strftime("%Y-%m-%d %H:%M:%SZ") if dt else src.stem
            draw.text(
                (pad, y),
                ts,
                font=font,
                fill=(255, 255, 255),
                stroke_width=3,
                stroke_fill=(0, 0, 0),
            )

        img.save(dst, quality=92)
        stamped.append(dst)

    stamped.sort(key=lambda p: (p.stem, str(p)))
    return stamped


def _escape_drawtext_text(s: str) -> str:
    s = s.replace("\\", "\\\\").replace("'", "\\'").replace(":", "\\:")
    return s


def build_vf_name_only(camera_name: str, overlay_name: bool) -> str:
    filters = ["format=yuv420p"]
    if overlay_name:
        txt = _escape_drawtext_text(camera_name)
        filters.append(
            f"drawtext=fontfile={FONTFILE}:text='{txt}':x=10:y=10:"
            "fontsize=24:fontcolor=white:box=1:boxcolor=0x00000099"
        )
    return ",".join(filters)


def _run_render(
    job_id: str,
    camera_name: str,
    app_camera_id: Optional[int],
    start_ts: str,
    end_ts: str,
    fps: int,
    filter_bad: bool,
    overlay_name: bool,
    overlay_timestamp: bool,
) -> None:
    list_path = None
    stamped_dir = None
    workspace = None
    adapter = None
    started_at = datetime.now(timezone.utc).isoformat()

    with LOCK:
        JOBS[job_id]["status"] = "running"
        JOBS[job_id]["started_at"] = started_at

    try:
        db.update_render_job(job_id, {"status": "running", "started_at": started_at})
    except Exception as exc:
        print(f"render state: failed to mark running {job_id}: {exc}", flush=True)

    try:
        RENDER_DIR.mkdir(parents=True, exist_ok=True)
        TMP_DIR.mkdir(parents=True, exist_ok=True)

        storage_render = os.getenv("RENDER_SOURCE", "filesystem").strip().lower() == "storage"
        if storage_render:
            if app_camera_id is None:
                raise RuntimeError("Storage render requires an app camera ID")
            workspace = TMP_DIR / f"{job_id}_objects"
            adapter = storage_from_env()
            frames = select_storage_frames(app_camera_id, start_ts, end_ts, workspace, adapter=adapter)
        else:
            frames = select_frames(camera_name, start_ts, end_ts)
        if not frames:
            raise RuntimeError("No frames found in selected range")

        eff_start = frames[0].stem
        eff_end = frames[-1].stem

        with LOCK:
            JOBS[job_id]["start_ts"] = eff_start
            JOBS[job_id]["end_ts"] = eff_end
        try:
            db.update_render_job(job_id, {"start_ts": eff_start, "end_ts": eff_end})
        except Exception:
            pass

        if filter_bad:
            frames = filter_bad_frames_by_size(frames, min_ratio=0.60)
            if not frames:
                raise RuntimeError("All frames filtered out (disable filter_bad to test)")

        # If timestamp overlay is requested, stamp exact timestamp from filenames.
        # This is the only “accurate” approach.
        if overlay_timestamp:
            stamped_dir = TMP_DIR / f"{job_id}_stamped"
            frames = stamp_frames_exact(
                frames,
                out_dir=stamped_dir,
                camera_name=camera_name,
                overlay_name=overlay_name,
                overlay_timestamp=True,
            )
            vf = "format=yuv420p"
        else:
            vf = build_vf_name_only(camera_name, overlay_name=overlay_name)

        out_folder = (workspace if storage_render else RENDER_DIR / camera_name)
        out_folder.mkdir(parents=True, exist_ok=True)
        out_path = out_folder / f"render_{eff_start}_{eff_end}_{fps}fps.mp4"

        list_path = TMP_DIR / f"{job_id}.txt"
        write_concat_list(frames, fps=fps, list_path=list_path)

        cmd = [
            "ffmpeg",
            "-hide_banner",
            "-loglevel", "error",
            "-f", "concat",
            "-safe", "0",
            "-i", str(list_path),
            "-vf", vf,
            "-vsync", "cfr",
            "-r", str(fps),
            "-c:v", "libx264",
            "-preset", "veryfast",
            "-crf", "20",
            "-pix_fmt", "yuv420p",
            "-movflags", "+faststart",
            "-y",
            str(out_path),
        ]

        p = subprocess.run(cmd, capture_output=True, text=True)
        if p.returncode != 0:
            raise RuntimeError((p.stderr or p.stdout or "ffmpeg failed").strip())

        artifact_key = None
        if storage_render:
            artifact = adapter.create_render_artifact(
                os.getenv("PROTOTYPE_ORG_ID", "org_dev_001"),
                os.getenv("PROTOTYPE_SITE_ID", "site_dev_001"),
                job_id,
                "timelapse.mp4",
                out_path.read_bytes(),
            )
            artifact_key = artifact.key
        finished_at = datetime.now(timezone.utc).isoformat()
        with LOCK:
            JOBS[job_id]["status"] = "done"
            JOBS[job_id]["output_path"] = None if storage_render else str(out_path)
            JOBS[job_id]["artifact_key"] = artifact_key
            JOBS[job_id]["finished_at"] = finished_at

        try:
            db.update_render_job(
                job_id,
                {
                    "status": "done",
                    "output_path": None if storage_render else str(out_path),
                    "artifact_key": artifact_key,
                    "finished_at": finished_at,
                    "frame_count": len(frames),
                },
            )
        except Exception as exc:
            print(f"render state: failed to persist completion {job_id}: {exc}", flush=True)
    except Exception as e:
        finished_at = datetime.now(timezone.utc).isoformat()
        with LOCK:
            JOBS[job_id]["status"] = "error"
            JOBS[job_id]["error"] = str(e)
            JOBS[job_id]["finished_at"] = finished_at

        try:
            db.update_render_job(
                job_id,
                {
                    "status": "error",
                    "error": str(e),
                    "finished_at": finished_at,
                },
            )
        except Exception as exc:
            print(f"render state: failed to persist error {job_id}: {exc}", flush=True)

    finally:
        try:
            if list_path and Path(list_path).exists():
                Path(list_path).unlink(missing_ok=True)
        except Exception:
            pass
        try:
            if stamped_dir and Path(stamped_dir).exists():
                shutil.rmtree(stamped_dir, ignore_errors=True)
        except Exception:
            pass
        try:
            if workspace and Path(workspace).exists():
                shutil.rmtree(workspace, ignore_errors=True)
        except Exception:
            pass


def start_render_job(
    camera_name: str,
    start_ts: str,
    end_ts: str,
    fps: int,
    filter_bad: bool,
    overlay_name: bool,
    overlay_timestamp: bool,
    app_camera_id: Optional[int] = None,
) -> str:
    job_id = uuid.uuid4().hex
    norm_start = _norm_ts(start_ts)
    norm_end = _norm_ts(end_ts)
    created_at = datetime.now(timezone.utc).isoformat()

    job_rec = {
        "job_id": job_id,
        "status": "queued",
        "camera_name": camera_name,
        "start_ts": norm_start,
        "end_ts": norm_end,
        "fps": int(fps),
        "filter_bad": bool(filter_bad),
        "overlay_name": bool(overlay_name),
        "overlay_timestamp": bool(overlay_timestamp),
        "output_path": None,
        "error": None,
        "created_at": created_at,
    }

    with LOCK:
        JOBS[job_id] = dict(job_rec)

    try:
        db.create_render_job(
            job_id=job_id,
            camera_name=camera_name,
            request_json=json.dumps(
                {
                    "start_ts": norm_start,
                    "end_ts": norm_end,
                    "fps": int(fps),
                    "filter_bad": bool(filter_bad),
                    "overlay_name": bool(overlay_name),
                    "overlay_timestamp": bool(overlay_timestamp),
                }
            ),
            status="queued",
            fps=int(fps),
            start_ts=norm_start,
            end_ts=norm_end,
            filter_bad=filter_bad,
            overlay_name=overlay_name,
            overlay_timestamp=overlay_timestamp,
        )
    except Exception as exc:
        print(f"render state: failed to persist job {job_id}: {exc}", flush=True)

    t = threading.Thread(
        target=_run_render,
        args=(job_id, camera_name, app_camera_id, start_ts, end_ts, int(fps),
              bool(filter_bad), bool(overlay_name), bool(overlay_timestamp)),
        daemon=True,
    )
    t.start()
    return job_id


def get_job(job_id: str) -> Optional[Dict[str, Any]]:
    live = None
    with LOCK:
        if job_id in JOBS:
            live = dict(JOBS[job_id])
    row = db.get_render_job(job_id)
    if not row and live:
        return live
    if row:
        job = _hydrate_job(row)
        if live:
            job.update(live)
        return job
    return None


def list_jobs() -> List[Dict[str, Any]]:
    rows = db.list_render_jobs(limit=200)
    jobs = []
    live_snapshot = {}
    with LOCK:
        live_snapshot = {k: dict(v) for k, v in JOBS.items()}
    for r in rows:
        job_id = r.get("job_id")
        job = _hydrate_job(r)
        if job_id in live_snapshot:
            job.update(live_snapshot[job_id])
        jobs.append(job)

    # Include any live-only jobs not yet persisted (should be rare)
    for job_id, live in live_snapshot.items():
        if not any(j.get("job_id") == job_id for j in jobs):
            jobs.append(dict(live))
    return jobs


def reset_running_jobs_to_interrupted() -> None:
    """Mark any in-flight renders as interrupted after an app restart."""
    try:
        db.mark_running_render_jobs_interrupted()
    except Exception as exc:
        print(f"render state: failed to mark running jobs as interrupted: {exc}", flush=True)
