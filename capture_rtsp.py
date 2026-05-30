import os, time, subprocess
from datetime import datetime, timezone
from pathlib import Path

CAMERA_NAME = os.getenv("CAMERA_NAME", "camera1")
RTSP_URL = os.environ["RTSP_URL"]
INTERVAL = float(os.getenv("INTERVAL_SECONDS", "5"))
OUT_DIR = Path(os.getenv("OUT_DIR", "/data"))

MAX_ATTEMPTS = int(os.getenv("MAX_ATTEMPTS", "5"))
BACKOFF_SECONDS = float(os.getenv("BACKOFF_SECONDS", "2"))

def target_path(ts: datetime) -> Path:
    base = OUT_DIR / CAMERA_NAME / ts.strftime("%Y/%m/%d/%H")
    base.mkdir(parents=True, exist_ok=True)
    fname = ts.strftime("%Y%m%dT%H%M%SZ.jpg")
    return base / fname

while True:
    loop_start = time.time()
    ts = datetime.now(timezone.utc)
    out = target_path(ts)
    tmp = out.with_suffix(".tmp.jpg")

    cmd = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel", "error",

        # Network / RTSP behavior
        "-rtsp_transport", "tcp",
        "-fflags", "+discardcorrupt",
        "-err_detect", "ignore_err",
        "-skip_frame", "nokey",             # only decode keyframes -> avoids “torn” JPEGs

        "-i", RTSP_URL,

        # Output
        "-frames:v", "1",
        "-q:v", "2",
        "-f", "image2",                     # force image muxer (extra safety)
        "-an", "-sn",
        "-y",
        str(tmp),
    ]

    ok = False
    for attempt in range(1, MAX_ATTEMPTS + 1):
        try:
            t0 = time.time()
            subprocess.run(cmd, check=True, timeout=25)
            tmp.replace(out)
            dt = time.time() - t0
            size_kb = out.stat().st_size / 1024
            print(f"[{ts.isoformat()}] Saved {out} ({size_kb:.0f} KB) in {dt:.2f}s")
            ok = True
            break
        except Exception as e:
            delay = BACKOFF_SECONDS * attempt
            print(f"[{ts.isoformat()}] WARN attempt {attempt}/{MAX_ATTEMPTS}: {e} (retry in {delay:.1f}s)")
            time.sleep(delay)
            try:
                if tmp.exists():
                    tmp.unlink()
            except:
                pass

    if not ok:
        print(f"[{ts.isoformat()}] ERROR: failed to capture after {MAX_ATTEMPTS} attempts")

    # Best-effort pacing: keep “roughly every INTERVAL seconds”
    elapsed = time.time() - loop_start
    time.sleep(max(0, INTERVAL - elapsed))
