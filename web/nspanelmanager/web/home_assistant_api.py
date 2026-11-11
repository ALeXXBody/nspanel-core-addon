import json
import logging
import traceback

import environ
import requests

from web.settings_helper import get_setting_with_default

# Shared session: HTTP keep-alive so repeated wizard/api calls reuse the
# connection instead of paying a fresh TCP+TLS handshake every time.
_home_assistant_session = requests.Session()

# Mutable default args (`filter={}`) are shared across calls — harmless here
# because the dict is only read, but flagged by linters anyway.
def get_all_home_assistant_items(filter=None):
    if filter is None:
        filter = {}
    return_json = {"items": [], "errors": []}

    if get_setting_with_default("home_assistant_token") == "" or get_setting_with_default("home_assistant_address") == "":
        return_json["errors"].append("Home Assistant address and/or access token not configured")
        return return_json

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
        home_assistant_response = _home_assistant_session.get(home_assistant_api_address, headers=home_assistant_request_headers, timeout=(3.05, 10), verify=False)
        if home_assistant_response.status_code == 200:
            for entity in home_assistant_response.json():
                add_entity = False
                if len(filter) == 0:
                    add_entity = True
                elif "type" in filter:
                    entity_type = entity["entity_id"].split(".")[0]
                    add_entity = entity_type in filter["type"]

                if add_entity:
                    data = {
                        "type": "home_assistant",
                        "label": entity["entity_id"],
                        "item_id": entity["entity_id"],
                        "item": entity,
                    }
                    if "friendly_name" in entity["attributes"]:
                        data["label"] = entity["attributes"]["friendly_name"]
                    return_json["items"].append(data)
        else:
            return_json["errors"].append("Failed to get Home Assistant lights, got return code: " + str(home_assistant_response.status_code))
            print("ERROR! Got status code other than 200. Got code: " + str(home_assistant_response.status_code))
    except Exception as e:
        return_json["errors"].append("Failed to get Home Assistant lights: " + str(traceback.format_exc()))
        logging.exception("Failed to get Home Assistant lights!")
    return return_json
