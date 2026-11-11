# NSPanel Core — legacy add-on

This repository holds the **legacy Home Assistant add-on** (Docker container)
for NSPanel Core: the Django manager, C++/prebuilt MQTT bridge, kiosk web UI
and panel TFT files.

> **This add-on is superseded by the native HA integration** in
> [ALeXXBody/NSPanelCore](https://github.com/ALeXXBody/NSPanelCore)
> (installed via HA Marketplace). New development happens there.

- Last verified-installable add-on release: **v1.0.2**
  (`ghcr.io/alexxbody/nspanel-core-amd64:v1.0.2`)
- `docker/mqttmanager-build/nspm_mqttmanager` holds the proven v1.0.2 bridge
  binary, reused by the release workflow here
- The `release.yml` workflow publishes amd64 images using the prebuilt binary
  (no C++ compilation)

The add-on was moved out of the main repository so that HA Marketplace can
serve the integration (Marketplace does not install apps).
