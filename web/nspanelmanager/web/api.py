import hashlib
import json
import logging
import os
import signal
import subprocess
import traceback
from datetime import datetime

import environ
import psutil
import requests
from django.core.files.storage import FileSystemStorage
from django.http import HttpResponse, JsonResponse
from django.shortcuts import redirect, render
from django.views.decorators.csrf import csrf_exempt

from web.settings_helper import (
    get_nspanel_setting_with_default,
    get_setting_with_default,
    set_setting_value,
)

from .apps import restart_mqtt_manager_process
from .models import LightState, NSPanel, RelayGroup, Room, Scene


# TODO: Rework how available entities are gathered
def get_all_available_entities(request):
    # TODO: Implement manually entered entities
    home_assistant_type_filter = []
    if "filter" in request.GET:
        filter_data = json.loads(request.GET["filter"])
        if "home_assistant_type_filter" in filter_data:
            home_assistant_type_filter = filter_data["home_assistant_type_filter"]

    return_json = {"entities": [], "errors": []}

    # Home Assistant
    if get_setting_with_default("home_assistant_token") != "" and get_setting_with_default("home_assistant_address") != "":
        home_assistant_request_headers = {
            "Authorization": "Bearer " + get_setting_with_default("home_assistant_token"),
            "content-type": "application/json",
        }
        try:
            environment = environ.Env()
            if "IS_HOME_ASSISTANT_ADDON" in environment and environment("IS_HOME_ASSISTANT_ADDON") == "true":
                home_assistant_api_address = get_setting_with_default("home_assistant_address") + "/core/api/states"
            else:
                home_assistant_api_address = get_setting_with_default("home_assistant_address") + "/api/states"
            logging.debug("Trying to get Home Assistant entities via api address: " + home_assistant_api_address)
            home_assistant_response = requests.get(
                home_assistant_api_address,
                headers=home_assistant_request_headers,
                timeout=(3.05, 10),
                verify=False,
            )
            if home_assistant_response.status_code == 200:
                for entity in home_assistant_response.json():
                    entity_type = entity["entity_id"].split(".")[0]
                    if (len(home_assistant_type_filter) > 0 and entity_type in home_assistant_type_filter) or len(home_assistant_type_filter) == 0:
                        data = {
                            "type": "home_assistant",
                            "label": entity["entity_id"],
                            "entity_id": entity["entity_id"],
                            "raw_data": entity,
                        }
                        if "friendly_name" in entity["attributes"]:
                            data["label"] = entity["attributes"]["friendly_name"]

                        if data["entity_id"].startswith("light."):
                            data["entity_type"] = "light"
                        elif data["entity_id"].startswith("switch."):
                            data["entity_type"] = "switch"
                        else:
                            logging.warn("Unknown entity type for entity: " + data["entity_id"])

                        return_json["entities"].append(data)
            else:
                return_json["errors"].append("Failed to get Home Assistant lights, got return code: " + str(home_assistant_response.status_code))
                print("ERROR! Got status code other than 200. Got code: " + str(home_assistant_response.status_code))
        except Exception as e:
            return_json["errors"].append("Failed to get Home Assistant lights: " + str(traceback.format_exc()))
            logging.exception("Failed to get Home Assistant lights!")
    else:
        print("No home assistant configuration values. Will not gather Home Assistant entities.")

    # Was `return return_json` — a plain dict, which crashes middleware that
    # expects an HttpResponse object (and broke this endpoint's callers).
    return JsonResponse(return_json)


def get_nspanel_config(request):
    try:
        logging.info("Trying to load config for NSPanel with MAC " + request.GET["mac"])
        nspanel = NSPanel.objects.get(mac_address=request.GET["mac"])
        base = {}
        base["name"] = nspanel.friendly_name
        base["home"] = nspanel.room.id
        base["default_page"] = get_nspanel_setting_with_default(nspanel.id, "default_page", "0")
        base["raise_to_100_light_level"] = get_setting_with_default("raise_to_100_light_level")
        base["color_temp_min"] = get_setting_with_default("color_temp_min")
        base["color_temp_max"] = get_setting_with_default("color_temp_max")
        base["reverse_color_temp"] = get_setting_with_default("reverse_color_temp")
        base["min_button_push_time"] = get_setting_with_default("min_button_push_time")
        base["button_long_press_time"] = get_setting_with_default("button_long_press_time")
        base["special_mode_trigger_time"] = get_setting_with_default("special_mode_trigger_time")
        base["special_mode_release_time"] = get_setting_with_default("special_mode_release_time")
        base["screen_dim_level"] = get_nspanel_setting_with_default(nspanel.id, "screen_dim_level", get_setting_with_default("screen_dim_level"))
        base["screensaver_dim_level"] = get_nspanel_setting_with_default(
            nspanel.id,
            "screensaver_dim_level",
            get_setting_with_default("screensaver_dim_level"),
        )
        base["screensaver_activation_timeout"] = get_nspanel_setting_with_default(
            nspanel.id,
            "screensaver_activation_timeout",
            get_setting_with_default("screensaver_activation_timeout"),
        )
        base["screensaver_mode"] = get_nspanel_setting_with_default(nspanel.id, "screensaver_mode", get_setting_with_default("screensaver_mode"))
        base["clock_us_style"] = get_setting_with_default("clock_us_style")
        base["use_fahrenheit"] = get_setting_with_default("use_fahrenheit")
        base["is_us_panel"] = get_nspanel_setting_with_default(nspanel.id, "is_us_panel", "False")
        base["lock_to_default_room"] = get_nspanel_setting_with_default(nspanel.id, "lock_to_default_room", "False")
        base["reverse_relays"] = get_nspanel_setting_with_default(nspanel.id, "reverse_relays", False)
        base["relay1_default_mode"] = get_nspanel_setting_with_default(nspanel.id, "relay1_default_mode", "False")
        base["relay2_default_mode"] = get_nspanel_setting_with_default(nspanel.id, "relay2_default_mode", "False")
        base["temperature_calibration"] = float(get_nspanel_setting_with_default(nspanel.id, "temperature_calibration", 0))
        base["button1_mode"] = nspanel.button1_mode
        base["button1_mqtt_topic"] = get_nspanel_setting_with_default(nspanel.id, "button1_mqtt_topic", "")
        base["button1_mqtt_payload"] = get_nspanel_setting_with_default(nspanel.id, "button1_mqtt_payload", "")
        base["button2_mode"] = nspanel.button2_mode
        base["button2_mqtt_topic"] = get_nspanel_setting_with_default(nspanel.id, "button2_mqtt_topic", "")
        base["button2_mqtt_payload"] = get_nspanel_setting_with_default(nspanel.id, "button2_mqtt_payload", "")

        if nspanel.button1_detached_mode_entity:
            base["button1_detached_light"] = nspanel.button1_detached_mode_entity.id
        else:
            base["button1_detached_light"] = -1

        if nspanel.button2_detached_mode_entity:
            base["button2_detached_light"] = nspanel.button2_detached_mode_entity.id
        else:
            base["button2_detached_light"] = -1
        base["rooms"] = []
        for room in Room.objects.all().order_by("displayOrder"):
            base["rooms"].append(room.id)
        base["scenes"] = {}
        for scene in Scene.objects.filter(room__isnull=True):
            base["scenes"][scene.id] = {}
            base["scenes"][scene.id]["name"] = scene.friendly_name
            if scene.scene_type == "nspm_scene":
                base["scenes"][scene.id]["can_save"] = True
            else:
                base["scenes"][scene.id]["can_save"] = False
        return JsonResponse(base)
    except Exception as ex:
        logging.exception(ex)
        print("Tried to get NSPanel config for panel that was not registered.")
        return HttpResponse("", status=500)




@csrf_exempt


@csrf_exempt
def restart_mqtt_manager(request):
    restart_mqtt_manager_process()
    return JsonResponse({"result": "OK"})


@csrf_exempt
def save_theme(request):
    set_setting_value("dark_theme", request.POST["dark"])
    return HttpResponse("OK", status=200)


# @csrf_exempt
# def get_album_cover(request):
#     from wand.image import Image
#     combined_pixel_data = []
#     with Image(filename="/usr/src/app/nspanelmanager/output_resized.tiff") as img:
#         pixel_data = img.export_pixels(channel_map="RGB", storage="char")
#         for i in range(0, len(pixel_data), 3):
#             channel_r = pixel_data[i] >> 3
#             channel_g = pixel_data[i + 1] >> 2
#             channel_b = pixel_data[i + 2] >> 3
#             combined = (
#                 channel_r << 11 | channel_g << 5 | channel_b
#             )  # Combine all values into a single 16-bit color value (ie. 565 colors)
#             combined_pixel_data.append(combined)
#     return JsonResponse({"pixels": combined_pixel_data}, status=200)
