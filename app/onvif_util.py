from typing import Dict, Any, Optional, Tuple
from onvif import ONVIFCamera

def probe_onvif(host: str, username: str, password: str, port: int = 80) -> Dict[str, Any]:
    cam = ONVIFCamera(host, port, username, password)
    dev = cam.create_devicemgmt_service()
    info = dev.GetDeviceInformation()
    net = cam.create_devicemgmt_service()
    mac = None
    try:
        ifaces = net.GetNetworkInterfaces()
        for iface in ifaces:
            hw = getattr(iface, "Info", None)
            if hw and getattr(hw, "HwAddress", None):
                mac = hw.HwAddress
                break
    except Exception:
        mac = None

    media = cam.create_media_service()
    profiles = media.GetProfiles()

    # Pick the highest-resolution profile (max pixels)
    best = None
    best_px = -1
    best_res = (None, None)

    for p in profiles:
        v = getattr(p, "VideoEncoderConfiguration", None)
        if not v or not getattr(v, "Resolution", None):
            continue
        w = int(v.Resolution.Width)
        h = int(v.Resolution.Height)
        px = w * h
        if px > best_px:
            best_px = px
            best = p
            best_res = (w, h)

    if not best:
        raise RuntimeError("No usable ONVIF media profiles found")

    stream_setup = {"Stream": "RTP-Unicast", "Transport": {"Protocol": "RTSP"}}
    rtsp = media.GetStreamUri({"StreamSetup": stream_setup, "ProfileToken": best.token}).Uri

    # Snapshot URI can exist but may be low-res on some cameras; still useful for thumbnails.
    snapshot_uri: Optional[str] = None
    try:
        snapshot_uri = media.GetSnapshotUri({"ProfileToken": best.token}).Uri
    except Exception:
        snapshot_uri = None

    return {
        "make": info.Manufacturer,
        "model": info.Model,
        "firmware": info.FirmwareVersion,
        "serial": info.SerialNumber,
        "rtsp_uri": rtsp,
        "snapshot_uri": snapshot_uri,
        "width": best_res[0],
        "height": best_res[1],
        "profile_token": best.token,
        "mac": mac,
    }
