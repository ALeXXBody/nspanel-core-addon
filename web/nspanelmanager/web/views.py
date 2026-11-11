import os
import re
import signal

import environ
import psutil
from django.core.files.storage import FileSystemStorage
from django.forms.models import model_to_dict
from django.http import FileResponse
from django.shortcuts import Http404, HttpResponse, redirect, render
from django.views.decorators.csrf import csrf_exempt
from django.core.exceptions import SuspiciousFileOperation
from django.db import transaction

from web.settings_helper import (
    delete_nspanel_setting,
    get_nspanel_setting_with_default,
    get_setting_with_default,
    set_nspanel_setting_value,
    set_setting_value,
)

from .apps import send_mqttmanager_reload_command, start_mqtt_manager
from .models import (
    Entity,
    Message,
    NSPanel,
    RelayGroup,
    RelayGroupBinding,
    Room,
    RoomEntitiesPage,
    Scene,
    Settings,
)


def _serve_file_range(path_on_storage, request):
    """
    Serve a storage file, honoring HTTP Range requests by seeking into the
    file instead of reading the whole multi-MB file into memory per request
    (panel OTA downloads the firmware in small 4-16 KB chunks — the old code
    loaded the entire binary and sliced it for every single chunk request).
    """
    fs = FileSystemStorage()
    try:
        f = fs.open(path_on_storage)
    except (FileNotFoundError, IOError, SuspiciousFileOperation):
        return HttpResponse("ERROR! File not found!", status=404)

    file_size = os.path.getsize(fs.path(path_on_storage))

    if "Range" in request.headers:
        match = _RANGE_PATTERN.match(request.headers["Range"].strip())
        if not match:
            f.close()
            return HttpResponse("ERROR! Unsupported range header!", status=400)
        start_s, end_s = match.groups()
        range_start = int(start_s)
        if end_s == "":
            range_end = file_size - 1
        else:
            range_end = min(int(end_s), file_size - 1)
        if range_start > range_end or range_start >= file_size:
            f.close()
            response = HttpResponse("ERROR! Requested range not satisfiable!", status=416)
            response["Content-Range"] = f"bytes */{file_size}"
            return response

        f.seek(range_start)
        response = HttpResponse(f.read(range_end - range_start + 1), content_type="application/octet-stream")
        response.status_code = 206
        response["Content-Range"] = f"bytes {range_start}-{range_end}/{file_size}"
        response["Content-Length"] = str(range_end - range_start + 1)
        f.close()
        return response

    # No Range header — stream the file instead of loading it fully into memory.
    response = FileResponse(f, content_type="application/octet-stream")
    response["Content-Length"] = file_size
    response["Accept-Ranges"] = "bytes"
    return response


from web.file_utils import _RANGE_PATTERN, get_cached_file_md5sum


def get_file_md5sum(filename):
    return get_cached_file_md5sum(filename)


def get_base_data(request):
    """Get data that is used on ALL rendered views."""
    data = {
        "ingress_path": request.headers["X-Ingress-Path"] if "X-Ingress-Path" in request.headers else "",
        "enabled_themes": {
            "default": "Default",
            "light": "Light",
            "dark": "Dark",
            "cupcake": "Cupcake",
            "bumblebee": "Bumblebee",
            "emerald": "Emerald",
            "corporate": "Corporate",
            "synthwave": "Synthwave",
            "retro": "Retro",
            "cyberpunk": "Cyberpunk",
            "valentine": "Valentine",
            "halloween": "Halloween",
            "garden": "Garden",
            "forest": "Forest",
            "aqua": "Aqua",
            "lofi": "Lo-Fi",
            "pastel": "Pastel",
            "fantasy": "Fantasy",
            "wireframe": "Wireframe",
            "black": "Black",
            "luxury": "Luxury",
            "dracula": "Dracula",
            "cmyk": "CMYK",
            "autumn": "Autumn",
            "business": "Business",
            "acid": "Acid",
            "lemonade": "Lemonade",
            "night": "Night",
            "coffee": "Coffee",
            "winter": "Winter",
            "dim": "Dim",
            "nord": "Nord",
            "sunset": "Sunset",
            "nspmwip": "NSPanel Core (dark, WIP)",
        },
        "theme": get_setting_with_default("theme"),
        "version": "%version%",  # This value get's replaced during build
        "manager_address": get_setting_with_default("manager_address"),
        "has_unread_messages": Message.objects.filter(read=False).exists(),
        "icons": [],
    }

    if data["manager_address"] == "":
        environment = environ.Env()
        data["is_home_assistant_addon"] = "IS_HOME_ASSISTANT_ADDON" in environment and environment("IS_HOME_ASSISTANT_ADDON") == "true"

    return data


def index(request):
    if get_setting_with_default("use_fahrenheit") == "True":
        temperature_unit = "°F"
    else:
        temperature_unit = "°C"

    notifications = []
    if get_setting_with_default("manager_address") == "":
        notifications.append({"text": "No manager address configured in settings.", "class": "error"})

    nspanels = []
    for nspanel in NSPanel.objects.filter(denied=False):
        panel_info = {}
        panel_info["data"] = nspanel
        # TODO: Load warnings from MQTTManager.
        nspanels.append(panel_info)

    data = {
        "nspanels": nspanels,
        "notifications": notifications,
        "temperature_unit": temperature_unit,
        "manager_address": get_setting_with_default("manager_address"),
    }
    data = data | get_base_data(request)

    if data["manager_address"] == "":
        environment = environ.Env()
        data = {
            **data,
            **{
                "manager_port": get_setting_with_default("manager_port"),
                "mqtt_server": get_setting_with_default("mqtt_server"),
                "mqtt_port": get_setting_with_default("mqtt_port"),
                "mqtt_username": get_setting_with_default("mqtt_username"),
                "mqtt_password": get_setting_with_default("mqtt_password"),
                "home_assistant_address": get_setting_with_default("home_assistant_address"),
                "home_assistant_token": get_setting_with_default("home_assistant_token"),
            },
        }

    return render(request, "index_htmx.html", data)


def rooms(request):
    data = get_base_data(request)
    data["rooms"] = []
    # Build dict of online and accepted NSPanels for each room
    for room in Room.objects.all().order_by("displayOrder"):
        data["rooms"].append(
            {
                "room": room,
                "nspanels": room.nspanel_set.filter(accepted=True, denied=False),
            }
        )

    return render(request, "rooms.html", data)


def edit_room(request, room_id: int):
    room = Room.objects.filter(id=room_id).first()
    data = get_base_data(request)
    data.update(
        {
            "total_num_rooms": Room.objects.count(),
            "room": room,
            "nspanels": room.nspanel_set.filter(accepted=True, denied=False),
            "entity_pages": RoomEntitiesPage.objects.filter(room=room),
        }
    )

    return render(request, "edit_room.html", data)


def save_new_room(request):
    new_room = Room()
    new_room.friendly_name = request.POST["friendly_name"]
    new_room.save()

    # Create RoomEntitiesPage for scenes
    scenes_page = RoomEntitiesPage()
    scenes_page.room = new_room
    scenes_page.page_type = 4
    scenes_page.is_scenes_page = True
    scenes_page.display_order = 0
    scenes_page.save()

    # Create RoomEntitiesPage for entities
    entities_page = RoomEntitiesPage()
    entities_page.room = new_room
    entities_page.page_type = 4
    entities_page.is_scenes_page = False
    entities_page.display_order = 0
    entities_page.save()

    send_mqttmanager_reload_command()
    return redirect("edit_room", room_id=new_room.id)


def delete_room(request, room_id: int):
    num_rooms = Room.objects.all().count()
    if num_rooms > 1:
        room = Room.objects.filter(id=room_id).first()
        if num_rooms >= 2:
            nspanels = NSPanel.objects.filter(room=room)
            new_room = Room.objects.all().exclude(id=room.id).first()
            nspanels.update(room=new_room)
        room.delete()

        send_mqttmanager_reload_command()
    return redirect("rooms")


def update_room_form(request, room_id: int):
    room = Room.objects.filter(id=room_id).first()
    room.friendly_name = request.POST["friendly_name"]
    room.room_temp_provider = request.POST["room_temp_provider"]
    room.room_temp_sensor = request.POST["room_temp_sensor"]
    room.save()
    send_mqttmanager_reload_command()
    return redirect("edit_room", room_id=room_id)


def edit_nspanel(request, panel_id: int):
    if get_setting_with_default("use_fahrenheit") == "True":
        temperature_unit = "°F"
    else:
        temperature_unit = "°C"
    settings = {
        "lock_to_default_room": get_nspanel_setting_with_default(panel_id, "lock_to_default_room", "False"),
        "screen_dim_level": get_nspanel_setting_with_default(panel_id, "screen_dim_level", ""),
        "screensaver_dim_level": get_nspanel_setting_with_default(panel_id, "screensaver_dim_level", ""),
        "is_us_panel": get_nspanel_setting_with_default(panel_id, "is_us_panel", "False"),
        "us_panel_orientation": get_nspanel_setting_with_default(panel_id, "us_panel_orientation", "vertical"),
        "selected_tft": get_nspanel_setting_with_default(panel_id, "selected_tft", "tft1"),
        "show_screensaver_inside_temperature": get_nspanel_setting_with_default(panel_id, "show_screensaver_inside_temperature", "global"),
        "show_screensaver_outside_temperature": get_nspanel_setting_with_default(panel_id, "show_screensaver_outside_temperature", "global"),
        "screensaver_activation_timeout": get_nspanel_setting_with_default(panel_id, "screensaver_activation_timeout", ""),
        "screensaver_mode": get_nspanel_setting_with_default(panel_id, "screensaver_mode", "global"),
        "reverse_relays": get_nspanel_setting_with_default(panel_id, "reverse_relays", "False"),
        "relay1_default_mode": get_nspanel_setting_with_default(panel_id, "relay1_default_mode", "False"),
        "relay1_is_light": get_nspanel_setting_with_default(panel_id, "relay1_is_light", "False"),
        "relay2_default_mode": get_nspanel_setting_with_default(panel_id, "relay2_default_mode", "False"),
        "relay2_is_light": get_nspanel_setting_with_default(panel_id, "relay2_is_light", "False"),
        "temperature_calibration": get_nspanel_setting_with_default(panel_id, "temperature_calibration", 0),
        "button1_custom_mqtt_topic": get_nspanel_setting_with_default(panel_id, "button1_mqtt_topic", ""),
        "button1_custom_mqtt_payload": get_nspanel_setting_with_default(panel_id, "button1_mqtt_payload", ""),
        "button2_custom_mqtt_topic": get_nspanel_setting_with_default(panel_id, "button2_mqtt_topic", ""),
        "button2_custom_mqtt_payload": get_nspanel_setting_with_default(panel_id, "button2_mqtt_payload", ""),
        "button1_relay_lower_temperature": get_nspanel_setting_with_default(panel_id, "button1_relay_lower_temperature", ""),
        "button1_relay_upper_temperature": get_nspanel_setting_with_default(panel_id, "button1_relay_upper_temperature", ""),
        "button2_relay_lower_temperature": get_nspanel_setting_with_default(panel_id, "button2_relay_lower_temperature", ""),
        "button2_relay_upper_temperature": get_nspanel_setting_with_default(panel_id, "button2_relay_upper_temperature", ""),
        "default_page": get_nspanel_setting_with_default(panel_id, "default_page", "0"),
    }

    nspanel = NSPanel.objects.get(id=panel_id)
    panel_info = {}
    panel_info["data"] = nspanel

    data = get_base_data(request)
    data = data | {
        "panel_info": panel_info,
        "rooms": Room.objects.all(),
        "settings": settings,
        "temperature_unit": temperature_unit,
        "multiple": [1, 2, 3, 4],
        "max_live_log_messages": get_setting_with_default("max_live_log_messages"),
        "screensaver_mode_global": get_setting_with_default("screensaver_mode"),
    }

    return render(request, "edit_nspanel.html", data)


def save_panel_settings(request, panel_id: int):
    panel = NSPanel.objects.get(id=panel_id)
    panel.room = Room.objects.get(id=request.POST["room_id"])
    panel.friendly_name = request.POST["name"]
    panel.button1_mode = request.POST["button1_mode"]
    if request.POST["button1_mode"] == "1":  # Detached mode
        panel.button1_detached_mode_entity = Entity.objects.get(id=request.POST["button1_detached_mode_light"])
    else:
        panel.button1_detached_mode_entity = None

    if request.POST["button1_mode"] == "2":  # Custom MQTT Mode
        set_nspanel_setting_value(panel_id, "button1_mqtt_topic", request.POST["button1_custom_mqtt_topic"])
        set_nspanel_setting_value(
            panel_id,
            "button1_mqtt_payload",
            request.POST["button1_custom_mqtt_payload"],
        )
    else:
        delete_nspanel_setting(panel_id, "button1_mqtt_topic")
        delete_nspanel_setting(panel_id, "button1_mqtt_payload")

    panel.button2_mode = request.POST["button2_mode"]
    if request.POST["button2_mode"] == "1":  # Detached mode
        panel.button2_detached_mode_entity = Entity.objects.get(id=request.POST["button2_detached_mode_light"])
    else:
        panel.button2_detached_mode_entity = None

    if request.POST["button2_mode"] == "2":  # Custom MQTT Mode
        set_nspanel_setting_value(panel_id, "button2_mqtt_topic", request.POST["button2_custom_mqtt_topic"])
        set_nspanel_setting_value(
            panel_id,
            "button2_mqtt_payload",
            request.POST["button2_custom_mqtt_payload"],
        )
    else:
        delete_nspanel_setting(panel_id, "button2_mqtt_topic")
        delete_nspanel_setting(panel_id, "button2_mqtt_payload")

    if request.POST["screen_dim_level"].strip():
        set_nspanel_setting_value(panel_id, "screen_dim_level", request.POST["screen_dim_level"])
    else:
        delete_nspanel_setting(panel_id, "screen_dim_level")

    if request.POST["screensaver_dim_level"].strip():
        set_nspanel_setting_value(panel_id, "screensaver_dim_level", request.POST["screensaver_dim_level"])
    else:
        delete_nspanel_setting(panel_id, "screensaver_dim_level")

    if request.POST["screensaver_activation_timeout"].strip():
        set_nspanel_setting_value(
            panel_id,
            "screensaver_activation_timeout",
            request.POST["screensaver_activation_timeout"],
        )
    else:
        delete_nspanel_setting(panel_id, "screensaver_activation_timeout")

    if request.POST["screensaver_mode"] == "global":
        delete_nspanel_setting(panel_id, "screensaver_mode")
    else:
        set_nspanel_setting_value(panel_id, "screensaver_mode", request.POST["screensaver_mode"])

    if request.POST["show_screensaver_inside_temperature"] == "global":
        delete_nspanel_setting(panel_id, "show_screensaver_inside_temperature")
    else:
        set_nspanel_setting_value(
            panel_id,
            "show_screensaver_inside_temperature",
            request.POST["show_screensaver_inside_temperature"],
        )

    if request.POST["show_screensaver_outside_temperature"] == "global":
        delete_nspanel_setting(panel_id, "show_screensaver_outside_temperature")
    else:
        set_nspanel_setting_value(
            panel_id,
            "show_screensaver_outside_temperature",
            request.POST["show_screensaver_outside_temperature"],
        )

    if request.POST["temperature_calibration"] == "":
        set_nspanel_setting_value(panel_id, "temperature_calibration", float(0))
    else:
        set_nspanel_setting_value(
            panel_id,
            "temperature_calibration",
            float(request.POST.get("temperature_calibration", 0)),
        )

    set_nspanel_setting_value(panel_id, "relay1_default_mode", request.POST["relay1_default_mode"])
    set_nspanel_setting_value(panel_id, "relay2_default_mode", request.POST["relay2_default_mode"])
    set_nspanel_setting_value(panel_id, "default_page", request.POST["default_page"])
    set_nspanel_setting_value(panel_id, "lock_to_default_room", request.POST["lock_to_default_room"])
    set_nspanel_setting_value(panel_id, "reverse_relays", request.POST["reverse_relays"])

    if request.POST["button1_relay_lower_temperature"].strip():
        set_nspanel_setting_value(
            panel_id,
            "button1_relay_lower_temperature",
            request.POST["button1_relay_lower_temperature"],
        )

    if request.POST["button1_relay_upper_temperature"].strip():
        set_nspanel_setting_value(
            panel_id,
            "button1_relay_upper_temperature",
            request.POST["button1_relay_upper_temperature"],
        )

    if request.POST["button2_relay_lower_temperature"].strip():
        set_nspanel_setting_value(
            panel_id,
            "button2_relay_lower_temperature",
            request.POST["button2_relay_lower_temperature"],
        )

    if request.POST["button2_relay_upper_temperature"].strip():
        set_nspanel_setting_value(
            panel_id,
            "button2_relay_upper_temperature",
            request.POST["button2_relay_upper_temperature"],
        )

    if request.POST["panel_type"] == "eu":
        set_nspanel_setting_value(panel_id, "is_us_panel", "False")
    else:
        if request.POST["panel_type"] == "us_horizontal":
            set_nspanel_setting_value(panel_id, "is_us_panel", "True")
            set_nspanel_setting_value(panel_id, "us_panel_orientation", "horizontal")
        elif request.POST["panel_type"] == "us_horizontal_mirrored":
            set_nspanel_setting_value(panel_id, "is_us_panel", "True")
            set_nspanel_setting_value(panel_id, "us_panel_orientation", "horizontal_mirrored")
        elif request.POST["panel_type"] == "us_vertical":
            set_nspanel_setting_value(panel_id, "is_us_panel", "True")
            set_nspanel_setting_value(panel_id, "us_panel_orientation", "vertical")

    set_nspanel_setting_value(panel_id, "selected_tft", request.POST.get("selected_tft"))

    set_nspanel_setting_value(panel_id, "relay1_is_light", request.POST["relay1_is_light"])
    set_nspanel_setting_value(panel_id, "relay2_is_light", request.POST["relay2_is_light"])

    panel.save()
    send_mqttmanager_reload_command()
    return redirect("edit_nspanel", panel_id)


def settings_page(request):
    environment = environ.Env()

    data = get_base_data(request)
    data["mqttmanager_log_level"] = get_setting_with_default("mqttmanager_log_level")
    data["default_nspanel_type"] = get_setting_with_default("default_nspanel_type")
    data["color_temp_min"] = get_setting_with_default("color_temp_min")
    data["color_temp_max"] = get_setting_with_default("color_temp_max")
    data["reverse_color_temp"] = get_setting_with_default("reverse_color_temp")
    data["mqtt_server"] = get_setting_with_default("mqtt_server")
    data["mqtt_port"] = get_setting_with_default("mqtt_port")
    data["mqtt_username"] = get_setting_with_default("mqtt_username")
    data["mqtt_password"] = get_setting_with_default("mqtt_password")
    data["home_assistant_address"] = get_setting_with_default("home_assistant_address")
    data["home_assistant_token"] = get_setting_with_default("home_assistant_token")
    data["raise_to_100_light_level"] = get_setting_with_default("raise_to_100_light_level")
    data["min_button_push_time"] = get_setting_with_default("min_button_push_time")
    data["button_long_press_time"] = get_setting_with_default("button_long_press_time")
    data["special_mode_trigger_time"] = get_setting_with_default("special_mode_trigger_time")
    data["special_mode_release_time"] = get_setting_with_default("special_mode_release_time")
    data["screensaver_activation_timeout"] = get_setting_with_default("screensaver_activation_timeout")
    data["screen_dim_level"] = get_setting_with_default("screen_dim_level")
    data["screensaver_dim_level"] = get_setting_with_default("screensaver_dim_level")
    data["screensaver_mode"] = get_setting_with_default("screensaver_mode")
    data["show_screensaver_inside_temperature"] = get_setting_with_default("show_screensaver_inside_temperature")
    data["show_screensaver_outside_temperature"] = get_setting_with_default("show_screensaver_outside_temperature")
    data["turn_on_behavior"] = get_setting_with_default("turn_on_behavior")
    data["max_live_log_messages"] = get_setting_with_default("max_live_log_messages")
    data["max_log_buffer_size"] = get_setting_with_default("max_log_buffer_size")
    data["is_home_assistant_addon"] = "IS_HOME_ASSISTANT_ADDON" in environment and environment("IS_HOME_ASSISTANT_ADDON") == "true"
    data["manager_address"] = get_setting_with_default("manager_address")
    data["manager_port"] = get_setting_with_default("manager_port")
    data["optimistic_mode"] = get_setting_with_default("optimistic_mode")
    data["light_turn_on_brightness"] = get_setting_with_default("light_turn_on_brightness")
    data["room_status_backoff_time"] = get_setting_with_default("room_status_backoff_time")
    data["all_rooms_status_backoff_time"] = get_setting_with_default("all_rooms_status_backoff_time")
    return render(request, "settings.html", data)


def save_settings(request):
    # All 40+ settings writes in one transaction: one SQLite fsync instead
    # of one per setting.
    with transaction.atomic():
        set_setting_value(name="mqtt_server", value=request.POST["mqtt_server"])
        set_setting_value(name="mqtt_port", value=request.POST["mqtt_port"])
        set_setting_value(name="mqtt_username", value=request.POST["mqtt_username"])

        if "clear_mqtt_password" in request.POST:
            set_setting_value(name="mqtt_password", value="")
        elif request.POST["mqtt_password"] != "":
            set_setting_value(name="mqtt_password", value=request.POST["mqtt_password"])

        if "home_assistant_address" in request.POST:
            home_assistant_address = request.POST["home_assistant_address"]
            if home_assistant_address.endswith("/"):
                home_assistant_address = home_assistant_address[:-1]
            set_setting_value(name="home_assistant_address", value=home_assistant_address)

        if "clear_home_assistant_token" in request.POST:
            set_setting_value(name="home_assistant_token", value="")
        elif "home_assistant_token" in request.POST:
            if request.POST["home_assistant_token"] != "":
                set_setting_value(name="home_assistant_token", value=request.POST["home_assistant_token"])

        set_setting_value(name="raise_to_100_light_level", value=request.POST["raise_to_100_light_level"])
        set_setting_value(name="default_nspanel_type", value=request.POST["default_nspanel_type"])
        set_setting_value(name="color_temp_min", value=request.POST["color_temp_min"])
        set_setting_value(name="color_temp_max", value=request.POST["color_temp_max"])
        set_setting_value(name="reverse_color_temp", value=request.POST["reverse_color_temp"])
        set_setting_value(name="min_button_push_time", value=request.POST["min_button_push_time"])
        set_setting_value(name="button_long_press_time", value=request.POST["button_long_press_time"])
        set_setting_value(
            name="special_mode_trigger_time",
            value=request.POST["special_mode_trigger_time"],
        )
        set_setting_value(
            name="special_mode_release_time",
            value=request.POST["special_mode_release_time"],
        )

        set_setting_value(
            name="screensaver_activation_timeout",
            value=request.POST["screensaver_activation_timeout"],
        )
        set_setting_value(name="screen_dim_level", value=request.POST["screen_dim_level"])
        set_setting_value(name="screensaver_dim_level", value=request.POST["screensaver_dim_level"])
        set_setting_value(name="screensaver_mode", value=request.POST["screensaver_mode"])
        set_setting_value(
            name="show_screensaver_inside_temperature",
            value=request.POST.get("show_screensaver_inside_temperature", "True"),
        )
        set_setting_value(
            name="show_screensaver_outside_temperature",
            value=request.POST.get("show_screensaver_outside_temperature", "True"),
        )
        set_setting_value(
            name="turn_on_behavior",
            value=request.POST.get("turn_on_behavior", "color_temp"),
        )
        set_setting_value(name="max_live_log_messages", value=request.POST["max_live_log_messages"])
        set_setting_value(name="max_log_buffer_size", value=request.POST["max_log_buffer_size"])
        set_setting_value(name="mqttmanager_log_level", value=request.POST["mqttmanager_log_level"])
        set_setting_value(name="manager_address", value=request.POST["manager_address"])
        set_setting_value(name="manager_port", value=request.POST["manager_port"])
        if "optimistic_mode" in request.POST:
            set_setting_value(
                name="optimistic_mode",
                value=request.POST["optimistic_mode"] == "optimistic",
            )
        set_setting_value(
            name="all_rooms_status_backoff_time",
            value=request.POST["all_rooms_status_backoff_time"],
        )
        set_setting_value(
            name="room_status_backoff_time",
            value=request.POST["room_status_backoff_time"],
        )
        set_setting_value(name="light_turn_on_brightness", value=request.POST["light_turn_on_brightness"])
        # Settings saved, restart mqtt_manager
        send_mqttmanager_reload_command()
        return redirect("settings")

@csrf_exempt
def save_new_firmware(request):
    if request.method == "POST":
        if "model" not in request.POST:
            print("Missing 'model' parameter")
            return redirect("/")

        fs = FileSystemStorage()
        # Delete old firmware file:
        fs.delete(f"firmware/{request.POST['model']}/firmware.bin")

        uploaded_file = request.FILES["firmware"]
        fs.save(f"firmware/{request.POST['model']}/firmware.bin", uploaded_file)
        send_mqttmanager_reload_command()
    return redirect("/")


# TODO: Make exempt only when Debug = true
@csrf_exempt
def save_new_data_file(request):
    if request.method == "POST":
        if "model" not in request.POST:
            print("Missing 'model' parameter")
            return redirect("/")

        fs = FileSystemStorage()
        # Delete old firmware file:
        fs.delete(f"firmware/{request.POST['model']}/data_file.bin")

        uploaded_file = request.FILES["data_file"]
        fs.save(f"firmware/{request.POST['model']}/data_file.bin", uploaded_file)
        send_mqttmanager_reload_command()
    return redirect("/")


@csrf_exempt
def save_new_merged_flash(request):
    if request.method == "POST":
        if "model" not in request.POST:
            print("Missing 'model' parameter")
            return redirect("/")

        fs = FileSystemStorage()
        # Delete old firmware file:
        fs.delete(f"firmware/{request.POST['model']}/merged_flash.bin")

        uploaded_file = request.FILES["bin"]
        fs.save(f"firmware/{request.POST['model']}/merged_flash.bin", uploaded_file)
        # send_mqttmanager_reload_command() # Manager does not keep track of checksum for merged_flash.bin. Do not send reload command.
    return redirect("/")


# TODO: Make exempt only when Debug = true


@csrf_exempt
def save_new_tft_file(request):
    if request.method == "POST":
        tft_file_type = request.POST["tft_file_type"]
        file_path = ""

        if tft_file_type == "eu1":
            file_path = "HMI_files/tft_automation/eu/output_tft1/gui.tft"
        elif tft_file_type == "eu2":
            file_path = "HMI_files/tft_automation/eu/output_tft2/gui.tft"
        elif tft_file_type == "eu3":
            file_path = "HMI_files/tft_automation/eu/output_tft3/gui.tft"
        elif tft_file_type == "eu4":
            file_path = "HMI_files/tft_automation/eu/output_tft4/gui.tft"
        elif tft_file_type == "us1":
            file_path = "HMI_files/tft_automation/us/output_tft1/gui.tft"
        elif tft_file_type == "us2":
            file_path = "HMI_files/tft_automation/us/output_tft2/gui.tft"
        elif tft_file_type == "us3":
            file_path = "HMI_files/tft_automation/us/output_tft3/gui.tft"
        elif tft_file_type == "us4":
            file_path = "HMI_files/tft_automation/us/output_tft4/gui.tft"
        elif tft_file_type == "us1_horizontal_mirrored":
            file_path = "HMI_files/tft_automation/us_horizontal_mirrored/output_tft1/gui.tft"
        elif tft_file_type == "us2_horizontal_mirrored":
            file_path = "HMI_files/tft_automation/us_horizontal_mirrored/output_tft2/gui.tft"
        elif tft_file_type == "us3_horizontal_mirrored":
            file_path = "HMI_files/tft_automation/us_horizontal_mirrored/output_tft3/gui.tft"
        elif tft_file_type == "us4_horizontal_mirrored":
            file_path = "HMI_files/tft_automation/us_horizontal_mirrored/output_tft4/gui.tft"
        else:
            print("ERROR! Unknown TFT file type!")
            return HttpResponse("ERROR! Unknown TFT file type!", status=500)

        print("Saving new TFT file ", file_path)

        uploaded_file = request.FILES["tft_file"]
        fs = FileSystemStorage()
        fs.delete(file_path)
        fs.save(file_path, uploaded_file)
        send_mqttmanager_reload_command()
    return redirect("/")


def download_firmware(request):
    if "model" in request.GET:
        model = request.GET["model"]
        if model != "sonoff" and model != "custom":
            return HttpResponse("ERROR! Unknown model!", status=400)
    else:
        # As model was not specified, default to sonoff as that was the original model
        model = "sonoff"

    return _serve_file_range(f"firmware/{model}/firmware.bin", request)


def download_data_file(request):
    if "model" in request.GET:
        model = request.GET["model"]
        if model != "sonoff" and model != "custom":
            return HttpResponse("ERROR! Unknown model!", status=400)
    else:
        # As model was not specified, default to sonoff as that was the original model
        model = "sonoff"

    return _serve_file_range(f"firmware/{model}/data_file.bin", request)


def download_tft(request, panel_id):
    selected_tft = get_nspanel_setting_with_default(panel_id, "selected_tft", "tft1")
    is_us_panel = get_nspanel_setting_with_default(panel_id, "is_us_panel", "False")
    us_panel_orientation = get_nspanel_setting_with_default(panel_id, "us_panel_orientation", "vertical")

    tft_file = ""
    if is_us_panel == "False":
        tft_file = "HMI_files/tft_automation/eu/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "horizontal":
        # We use EU tft file for horizontal US panel with buttons on left as it's the same screen and orientation
        tft_file = "HMI_files/tft_automation/eu/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "horizontal_mirrored":
        # We use EU tft file for horizontal US panel with buttons on left as it's the same screen and orientation
        tft_file = "HMI_files/tft_automation/us_horizontal_mirrored/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "vertical":
        tft_file = "HMI_files/tft_automation/us/output_" + selected_tft + "/gui.tft"
    else:
        print(f"ERROR! Could not determine TFT file for NSPanel with ID {panel_id}")

    return _serve_file_range(tft_file, request)


def checksum_firmware(request):
    if "model" in request.GET:
        model = request.GET["model"]
        if model != "sonoff" and model != "custom":
            return HttpResponse("ERROR! Unknown model!", status=400)
    else:
        # As model was not specified, default to sonoff as that was the original model
        model = "sonoff"

    return HttpResponse(get_file_md5sum(f"firmware/{model}/firmware.bin"))


def checksum_data_file(request):
    if "model" in request.GET:
        model = request.GET["model"]
        if model != "sonoff" and model != "custom":
            return HttpResponse("ERROR! Unknown model!", status=400)
    else:
        # As model was not specified, default to sonoff as that was the original model
        model = "sonoff"

    return HttpResponse(get_file_md5sum(f"firmware/{model}/data_file.bin"))


def checksum_tft_file(request, panel_id):
    selected_tft = get_nspanel_setting_with_default(panel_id, "selected_tft", "tft1")
    is_us_panel = get_nspanel_setting_with_default(panel_id, "is_us_panel", "False")
    us_panel_orientation = get_nspanel_setting_with_default(panel_id, "us_panel_orientation", "vertical")
    tft_file = ""
    if is_us_panel == "False":
        tft_file = "HMI_files/tft_automation/eu/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "horizontal":
        # We use EU tft file for horizontal US panel with buttons on left as it's the same screen and orientation
        tft_file = "HMI_files/tft_automation/eu/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "horizontal_mirrored":
        # We use EU tft file for horizontal US panel with buttons on left as it's the same screen and orientation
        tft_file = "HMI_files/tft_automation/us_horizontal_mirrored/output_" + selected_tft + "/gui.tft"
    elif is_us_panel == "True" and us_panel_orientation == "vertical":
        tft_file = "HMI_files/tft_automation/us/output_" + selected_tft + "/gui.tft"
    else:
        print(f"ERROR! Could not determine TFT file for NSPanel with ID {panel_id}")

    return HttpResponse(get_file_md5sum(tft_file))


def get_manual(request):
    fs = FileSystemStorage()
    response = HttpResponse(fs.open("manual.pdf").read(), content_type="application/pdf")
    response["Content-Disposition"] = 'attachment; filename="manual.pdf"'
    return response


def global_scenes(request):
    data = get_base_data(request)
    data["global_scenes"] = Scene.objects.filter(room__isnull=True)
    return render(request, "global_scenes.html", data)


def relay_groups(request):
    data = get_base_data(request)
    data["nspanels"] = NSPanel.objects.all()
    data["relay_groups"] = RelayGroup.objects.all()
    return render(request, "relay_groups.html", data)


def create_or_update_relay_group(request):
    if request.POST["relay_group_id"]:
        new_rg = RelayGroup.objects.get(id=request.POST["relay_group_id"])
    else:
        new_rg = RelayGroup()
    new_rg.friendly_name = request.POST["relay_group_name"]
    new_rg.save()
    send_mqttmanager_reload_command()
    return redirect("relay_groups")


def delete_relay_group(request, relay_group_id):
    rg = RelayGroup.objects.get(id=relay_group_id)
    if rg:
        rg.delete()
    send_mqttmanager_reload_command()
    return redirect("relay_groups")


def add_nspanel_relay_to_group(request):
    rg = RelayGroup.objects.get(id=request.POST["relay_group_id"])
    nspanel = NSPanel.objects.get(id=request.POST["nspanel_id"])
    relay_num = request.POST["relay_num"]

    exists = RelayGroupBinding.objects.filter(nspanel=nspanel, relay_num=relay_num, relay_group=rg).count() > 0
    if not exists:
        binding = RelayGroupBinding()
        binding.relay_group = rg
        binding.nspanel = nspanel
        binding.relay_num = relay_num
        binding.save()
        send_mqttmanager_reload_command()
    return redirect("relay_groups")


def delete_relay_group_binding(request, relay_binding_id):
    binding = RelayGroupBinding.objects.get(id=relay_binding_id)
    if binding:
        binding.delete()
    send_mqttmanager_reload_command()
    return redirect("relay_groups")


def weather_and_time(request):
    if request.method == "POST":
        set_setting_value("location_latitude", request.POST["location_latitude"])
        set_setting_value("location_longitude", request.POST["location_longitude"])
        set_setting_value("weather_wind_speed_format", request.POST["wind_speed_format"])
        set_setting_value("weather_precipitation_format", request.POST["precipitation_format"])
        set_setting_value("outside_temp_sensor_provider", request.POST["outside_temp_provider"])
        set_setting_value("outside_temp_sensor_entity_id", request.POST["outside_temp_sensor"])
        set_setting_value("weather_update_interval", request.POST["weather_update_interval"])
        set_setting_value("date_format", request.POST["date_format"])
        set_setting_value("clock_us_style", request.POST["clock_us_style"])
        set_setting_value("use_fahrenheit", request.POST["use_fahrenheit"])
        send_mqttmanager_reload_command()
        return redirect("weather_and_time")
    else:
        data = get_base_data(request)
        data = data | {
            "date_format": get_setting_with_default("date_format"),
            "clock_us_style": get_setting_with_default("clock_us_style"),
            "use_fahrenheit": get_setting_with_default("use_fahrenheit"),
            "outside_temp_provider": get_setting_with_default("outside_temp_sensor_provider"),
            "outside_temp_sensor": get_setting_with_default("outside_temp_sensor_entity_id"),
            "location_latitude": get_setting_with_default("location_latitude"),
            "location_longitude": get_setting_with_default("location_longitude"),
            "wind_speed_format": get_setting_with_default("weather_wind_speed_format"),
            "precipitation_format": get_setting_with_default("weather_precipitation_format"),
            "weather_update_interval": get_setting_with_default("weather_update_interval"),
        }

        return render(request, "weather_and_time.html", data)


def denied_nspanels(request):
    data = get_base_data(request)
    data["nspanels"] = NSPanel.objects.all()
    return render(request, "denied_nspanels.html", data)


def unblock_nspanel(request, nspanel_id):
    panel = NSPanel.objects.filter(id=nspanel_id).first()
    if panel:
        panel.delete()
        send_mqttmanager_reload_command()
    return redirect("denied_nspanels")


def download_mqttmanager_log(request):
    if os.path.exists("/dev/shm/mqttmanager.log"):
        # Stream instead of loading the (potentially large) log into memory.
        response = FileResponse(open("/dev/shm/mqttmanager.log", "rb"), content_type="text/plain")
        response["Content-Disposition"] = "inline; filename=mqttmanager_log.txt"
        return response
    raise Http404
