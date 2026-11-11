import logging
import time

from .models import NSPanel, NSPanelSettings, Settings

# Short-TTL process caches for settings. Every global settings read used to
# run 2 queries (count() + first()) and every panel-specific read 4 (panel
# existence + fetch + settings count + fetch) with no caching at all — the
# panel editor rendered ~105 queries per request and every panel boot's
# config fetch ~90. This cache loads all global settings in ONE query and
# all panel settings in one query per panel, refreshed at most once per TTL.
# The short TTL keeps cross-process writes (other uWSGI workers, MQTTManager
# REST calls, Django admin) visible without any single points of staleness.
_SETTINGS_CACHE_TTL_SECONDS = 2.0
_settings_cache = None
_settings_cache_time = 0.0

_nspanel_settings_cache = {}  # str(nspanel_id) -> {name: value}
_nspanel_settings_cache_time = {}  # str(nspanel_id) -> timestamp


def _load_global_settings_cache():
    global _settings_cache, _settings_cache_time
    _settings_cache = {s.name: s.value for s in Settings.objects.all().only("name", "value")}
    _settings_cache_time = time.time()


def invalidate_settings_cache():
    """Force a reload of the global settings cache on the next read."""
    global _settings_cache
    _settings_cache = None


def _ensure_global_settings_cache():
    global _settings_cache
    if _settings_cache is None or time.time() - _settings_cache_time > _SETTINGS_CACHE_TTL_SECONDS:
        _load_global_settings_cache()


def get_setting_with_default(name) -> str:
    _ensure_global_settings_cache()
    value = _settings_cache.get(name)
    if value is None:
        logging.debug(f"No value for setting '{name}'. Will return default value.")
        return ""
    return str(value)


def does_setting_exist(name):
    _ensure_global_settings_cache()
    return name in _settings_cache


def set_setting_value(name, value):
    Settings.objects.update_or_create(name=name, defaults={"value": value})
    # Keep the (possibly existing) cache coherent in this process; other
    # processes pick the change up on their next TTL refresh.
    global _settings_cache
    if _settings_cache is not None:
        _settings_cache[name] = value


def get_nspanel_setting_with_default(nspanel_id, name, default):
    key = str(nspanel_id)
    now = time.time()
    if key not in _nspanel_settings_cache or now - _nspanel_settings_cache_time.get(key, 0) > _SETTINGS_CACHE_TTL_SECONDS:
        if not NSPanel.objects.filter(id=nspanel_id).exists():
            return default
        _nspanel_settings_cache[key] = {s.name: s.value for s in NSPanelSettings.objects.filter(nspanel_id=nspanel_id).only("name", "value")}
        _nspanel_settings_cache_time[key] = now
    return _nspanel_settings_cache[key].get(name, default)


def delete_nspanel_setting(nspanel_id, name):
    panel_setting = NSPanelSettings.objects.filter(nspanel_id=nspanel_id, name=name).first()
    if panel_setting is not None:
        panel_setting.delete()
        _nspanel_settings_cache.pop(str(nspanel_id), None)  # force reload on next read


def set_nspanel_setting_value(nspanel_id, name, value):
    if NSPanel.objects.filter(id=nspanel_id).exists():
        NSPanelSettings.objects.update_or_create(nspanel_id=nspanel_id, name=name, defaults={"value": value})
        key = str(nspanel_id)
        cached = _nspanel_settings_cache.get(key)
        if cached is not None:
            cached[name] = value
            _nspanel_settings_cache_time[key] = time.time()
