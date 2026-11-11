import { Client, type StompSubscription } from "@stomp/stompjs";
import { convertProtbuf } from "@/services/protoService";

import {
  useConfigStore,
  useEntityPagesStore,
  useLightsStore,
  useRoomsStore,
  useScenePagesStore,
  useUIStore,
} from "@/stores";
import type { LightType } from "@/types";
import {
  NSPanelConfig,
  NSPanelMQTTManagerCommand,
  NSPanelMQTTManagerCommand_FirstPageTurnLightOn,
  NSPanelMQTTManagerCommand_LightCommand,
  NSPanelMQTTManagerCommand_ToggleEntityFromEntitiesPage,
  NSPanelRoomEntitiesPage,
  NSPanelRoomEntitiesPage_EntitySlot_EntityType,
  NSPanelRoomStatus,
  NSPanelWeatherUpdate,
} from "@/generated/src/proto/protobuf_nspanel";
import type { NSPanelEntityState } from "@/generated/src/proto/protobuf_nspanel_entity";
import { useWeatherStore } from "@/stores/useWeatherStore";

// Console output only in development builds: 26 unconditional console.log calls
// shipped to production kiosks (string formatting + object retention per message).
const devLog = import.meta.env.DEV ? console.log.bind(console) : ((): void => {});

//This variable gets an address when we receive register accept from manager
let MANAGER_ADDRESS = import.meta.env.DEV ? "192.168.32.201" : "";
// let MANAGER_ADDRESS = import.meta.env.DEV ? "192.168.60.3" : "";

// type CommandType = keyof Omit<NSPanelMQTTManagerCommand, "nspanelId">;
type ManagerCommandPayload = Omit<NSPanelMQTTManagerCommand, "nspanelId">;

type SubLevel =
  | "registerAccept"
  | "config"
  | "weather"
  | "globalRoom"
  | "rooms"
  | "entityPages"
  | "lights"
  | "scenePages";

const subscriptions: Record<SubLevel, Record<string, StompSubscription>> = {
  registerAccept: {},
  config: {},
  weather: {},
  globalRoom: {},
  rooms: {},
  entityPages: {},
  lights: {},
  scenePages: {},
};

export interface LightCommandOptions {
  brightness?: number;
  colorTemp?: number;
  rgb?: number;
  saturation?: number;
}

let client: Client | null = null;

let registerRequestTimerId: ReturnType<typeof setInterval> | null = null;
// let browserStatusOnlineTimerId: ReturnType<typeof setInterval> | null = null;

function startRegisterRequestTimer() {
  const interval = 5000;
  if (registerRequestTimerId !== null) return;
  devLog(
    `Starting to send register requests every ${interval / 1000} seconds`,
  );

  registerRequestTimerId = setInterval(() => {
    devLog("Sent register request");
    stompService.sendRegisterRequest();
  }, interval);
}

function stopRegisterRequestTimer() {
  if (registerRequestTimerId === null) return;

  clearInterval(registerRequestTimerId);
  registerRequestTimerId = null;
  devLog(
    "Received register accept. Stopping transmission of register requests.",
  );
}

// function startStatusOnlineTimer() {
//   const interval = 30000;
//   if (browserStatusOnlineTimerId !== null) return;
//   stompService.sendBrowserStatusOnline();
//   console.log(
//     `Starting to send browser status ONLINE every ${interval / 1000} seconds`,
//   );
//   browserStatusOnlineTimerId = setInterval(() => {
//     stompService.sendBrowserStatusOnline();
//     console.log("Sent browser status ONLINE");
//   }, interval);
// }

export const stompService = {
  //Initialize the connection
  init: () => {
    if (client) return;

    client = new Client({
      webSocketFactory: () => {
        return new WebSocket(`ws://${window.location.host}/websocket/stomp`);
      },
      reconnectDelay: 5000,
      onConnect: () => {
        devLog("STOMP Connected");
        stompService.subscribeToRegisterAccept();
        stompService.sendRegisterRequest();
        startRegisterRequestTimer();
        // startStatusOnlineTimer();
      },
      onWebSocketClose: () => {
        stompService.cleanup();
        stompService.cleanUpTimers();
        //TODO feels like this can be done better. Quick solution to make it work for release.
        useConfigStore.setState({ config: null, isLoaded: false });
        useUIStore.setState({ redirectTo: "/webapp/" });
      },
      onStompError: (frame) => {
        console.error("Broker reported error: " + frame.headers["message"]);
      },
    });

    const virtualMac = useConfigStore.getState().virtualMac;
    if (!virtualMac) return;

    const lastWillPayload = {
      mac: virtualMac,
      state: "offline",
    };

    client.connectHeaders = {
      last_will_topic: `mqtt/nspanel/${virtualMac}/status`,
      last_will_message: JSON.stringify(lastWillPayload),
      last_will_retained: "true",
    };

    client.activate();
  },

  sendRegisterRequest() {
    const virtualMac = useConfigStore.getState().virtualMac;
    const friendlyName = useConfigStore.getState().friendlyName;
    if (!virtualMac && !friendlyName) return;

    const registerCommand = {
      command: "register_request",
      mac_origin: virtualMac,
      friendly_name: friendlyName,
      version: "1.0.1",
      md5_firmware: "",
      md5_data_file: "",
      md5_tft_file: "",
      address: "",
      model: "web",
    };

    if (!client?.connected) return;

    client.publish({
      destination: "mqtt/nspanel/mqttmanager/command",
      body: JSON.stringify(registerCommand), // Sends as a text string
      headers: { "content-type": "text/plain" },
    });
  },

  sendBrowserStatusOnline() {
    const virtualMac = useConfigStore.getState().virtualMac;
    if (!virtualMac) return;
    const payload = {
      mac: virtualMac,
      state: "online",
    };

    // Convert the object to a JSON string
    const jsonBody = JSON.stringify(payload);

    if (!client?.connected) return;

    client.publish({
      headers: { retain: "true" },
      destination: `mqtt/nspanel/${virtualMac}/status`,
      body: jsonBody, // Use 'body' for strings/JSON instead of 'binaryBody'
    });
  },

  newConfigCleanUp: () => {
    //Every time new config arrives remove all dependant subscriptions and reset stores
    stompService.cleanup("weather");
    stompService.cleanup("globalRoom");
    stompService.cleanup("rooms");
    stompService.cleanup("entityPages");
    stompService.cleanup("lights");
    stompService.cleanup("scenePages");

    useWeatherStore.getState().resetWeather();
    useRoomsStore.getState().resetRooms();
    useEntityPagesStore.getState().resetEntityPages();
    useLightsStore.getState().resetLights();
    useScenePagesStore.getState().resetScenePages();
  },

  subscribeToRegisterAccept: () => {
    //When we receive an accept from the manager on this topic we stop sending requests and
    //we start the config subscription
    if (!client?.connected || subscriptions.registerAccept["main"]) return;
    const virtualMac = useConfigStore.getState().virtualMac;
    if (!virtualMac) return;

    devLog("Subscribing to register accept topic");
    const sub = client.subscribe(
      `mqtt/nspanel/${virtualMac}/command`,
      (message) => {
        if (message.body) {
          stopRegisterRequestTimer();
          stompService.sendBrowserStatusOnline();
          try {
            //Decode Base64 string back to plain text JSON string
            const decodedString = atob(message.body);

            //Parse the plain text into a JS object
            const registerAccept = JSON.parse(decodedString);
            devLog("Register accept message:", registerAccept);
            devLog(
              `Setting manager address to: ${registerAccept.address}`,
            );
            MANAGER_ADDRESS = registerAccept.address;
            stompService.subscribeToConfig(registerAccept.config_topic);
          } catch (error) {
            console.error("Failed to decode or parse message:", error);
          }
        }
      },
    );

    subscriptions.registerAccept["main"] = sub;
  },

  subscribeToConfig: (configTopic: string) => {
    if (!client?.connected || subscriptions.config["main"]) return;
    configTopic = "mqtt/" + configTopic;
    devLog("Subscribing to config", configTopic);
    const sub = client.subscribe(configTopic, (message) => {
      const configData = convertProtbuf<NSPanelConfig>(
        message,
        "NSPanelConfig",
      );
      devLog("Received New Config");

      if (configData && configData.roomInfos) {
        stompService.newConfigCleanUp();
        useConfigStore.getState().setConfig(configData);

        devLog(
          "Starting subscriptions to weather, global room, rooms, entityPages and ScenePages",
        );
        stompService.subscribeToWeather();
        stompService.subscribeToGlobalRoom();
        //Loop through roomInfo object containing entityPages, scenesPages and room ids to start subscriptions
        for (const room of configData.roomInfos) {
          const { roomId, entityPageIds, scenePageIds } = room;

          if (roomId) {
            stompService.subscribeToRoom(roomId);
          }
          if (entityPageIds && entityPageIds.length > 0) {
            for (const id of entityPageIds) {
              stompService.subscribeToEntityPage(id);
            }
          }
          if (scenePageIds && scenePageIds.length > 0)
            for (const id of scenePageIds) {
              stompService.subscribeToScenesPage(id);
            }
        }
      }
    });
    subscriptions.config["main"] = sub;
  },

  subscribeToWeather: () => {
    if (!client?.connected || subscriptions.weather["main"])
      return devLog(
        "Client not connected or weather subscription exists since before.",
      );

    const sub = client.subscribe(
      `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/status/weather`,
      (message) => {
        const weather = convertProtbuf<NSPanelWeatherUpdate>(
          message,
          "NSPanelWeatherUpdate",
        );
        if (weather) {
          useWeatherStore.getState().setWeather(weather);
          // useRoomsStore.getState().setGlobalRoom(weather);
        }
      },
    );
    subscriptions.weather["main"] = sub;
  },

  subscribeToGlobalRoom: () => {
    if (!client?.connected || subscriptions.globalRoom["main"])
      return devLog(
        "Client not connected or room subscription exists since before.",
      );

    const sub = client.subscribe(
      `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/all_rooms_status`,
      (message) => {
        const allRoomState = convertProtbuf<NSPanelRoomStatus>(
          message,
          "NSPanelRoomStatus",
        );
        if (allRoomState) {
          useRoomsStore.getState().setGlobalRoom(allRoomState);
        }
      },
    );
    subscriptions.globalRoom["main"] = sub;
  },

  subscribeToRoom: (id: number) => {
    if (!client?.connected || subscriptions.rooms[id])
      return devLog(
        "Client not connected or room subscription exists since before.",
      );

    const sub = client.subscribe(
      `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/room/${id}/state`,
      (message) => {
        const roomData = convertProtbuf<NSPanelRoomStatus>(
          message,
          "NSPanelRoomStatus",
        );
        if (roomData) {
          useRoomsStore.getState().updateRoom(roomData);
        }
      },
    );
    // console.log(`Starting subscriptions to room id: ${id}`);
    subscriptions.rooms[id] = sub;
  },

  subscribeToEntityPage: (id: number) => {
    if (!client?.connected || subscriptions.entityPages[id])
      return devLog(
        "Client not connected or entitypage subscription exists since before.",
      );

    const sub = client.subscribe(
      `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/entity_pages/${id}/state`,
      (message) => {
        const entityPageData = convertProtbuf<NSPanelRoomEntitiesPage>(
          message,
          "NSPanelRoomEntitiesPage",
        );
        if (entityPageData) {
          useEntityPagesStore.getState().updateEntityPage(entityPageData);
          devLog("Subscribing to Entities");

          for (const entity of entityPageData.entities) {
            if (
              entity.type ===
                NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_LIGHT &&
              entity.mqttStateTopic
            ) {
              stompService.subscribeToEntity(entity.mqttStateTopic);
            }
          }
        }
      },
    );
    // console.log(`Starting subscription to entitypage id: ${id}`);
    subscriptions.entityPages[id] = sub;
  },

  subscribeToScenesPage: (id: number) => {
    if (!client?.connected || subscriptions.scenePages[id])
      return devLog(
        "Client not connected or scene page subscription exist since before.",
      );

    const sub = client.subscribe(
      `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/entity_pages/${id}/state`,
      (message) => {
        const scenePageData = convertProtbuf<NSPanelRoomEntitiesPage>(
          message,
          "NSPanelRoomEntitiesPage",
        );
        if (scenePageData) {
          useScenePagesStore.getState().updateScenePage(scenePageData);
        }
      },
    );
    // console.log(`Starting subscription to scene page id: ${id}`);
    subscriptions.scenePages[id] = sub;
  },

  subscribeToEntity: (mqttTopic: string) => {
    if (!client?.connected || subscriptions.lights[mqttTopic])
      return devLog(
        "Client not connected or light subscription exist since before.",
      );

    const sub = client.subscribe(`mqtt/${mqttTopic}`, (message) => {
      const entityData = convertProtbuf<NSPanelEntityState>(
        message,
        "NSPanelEntityState",
      );
      if (entityData?.light?.lightId) {
        useLightsStore.getState().updateLight(entityData);
      }
    });
    subscriptions.lights[mqttTopic] = sub;
  },

  //Function used to remove subsciptions
  cleanup: (level?: SubLevel) => {
    if (level) {
      const category = subscriptions[level];
      Object.keys(category).forEach((key) => {
        devLog(`Unsubscribing from ${level}: ${key}`);
        if (client?.connected) {
          category[key].unsubscribe();
        }
        delete category[key];
      });
    } else {
      // Total Cleanup

      (Object.keys(subscriptions) as SubLevel[]).forEach((key) => {
        stompService.cleanup(key);
      });
      devLog("Reached total cleanup");
    }
  },
  cleanUpTimers: () => {
    if (registerRequestTimerId) {
      clearInterval(registerRequestTimerId);
      registerRequestTimerId = null;
    }
    // if (browserStatusOnlineTimerId) {
    //   clearInterval(browserStatusOnlineTimerId);
    //   browserStatusOnlineTimerId = null;
    // }
    devLog("Timers stopped and removed");
  },

  sendMainPageLightCommand(
    type: LightType,
    options: LightCommandOptions,
    roomId: number,
  ) {
    const { brightness, colorTemp } = options;
    const command: NSPanelMQTTManagerCommand_FirstPageTurnLightOn = {
      affectLights: type, // 1 for Table, 2 for Ceiling
      selectedRoom: roomId,
      global: useUIStore.getState().mainPageMode === "allLights",
      hasBrightnessValue: brightness !== undefined,
      hasKelvinValue: colorTemp !== undefined,
      brightnessSliderValue: brightness ?? 0,
      kelvinSliderValue: colorTemp ?? 0,
    };
    stompService.sendNspanelMqttManagerCommand({ firstPageTurnOn: command });
  },

  sendLightCommand(options: LightCommandOptions, lightId: number) {
    const { brightness, colorTemp, rgb, saturation } = options;
    const command: NSPanelMQTTManagerCommand_LightCommand = {
      lightIds: [lightId],
      brightness: brightness ?? 0,
      colorTemperature: colorTemp ?? 0,
      hue: rgb ?? 0,
      saturation: saturation ?? 0,
      hasBrightness: brightness !== undefined,
      hasColorTemperature: colorTemp !== undefined,
      hasHue: rgb !== undefined,
      hasSaturation: saturation !== undefined,
    };
    stompService.sendNspanelMqttManagerCommand({ lightCommand: command });
  },

  //ToggleEntityFromEntitiesPage
  sendToggleEntityFromEntitiesPage(entityPageId: number, entitySlot: number) {
    const command: NSPanelMQTTManagerCommand_ToggleEntityFromEntitiesPage = {
      entityPageId,
      entitySlot,
    };
    stompService.sendNspanelMqttManagerCommand({
      toggleEntityFromEntitiesPage: command,
    });
  },

  sendNspanelMqttManagerCommand(commandPayload: ManagerCommandPayload) {
    const nspanelId = useConfigStore.getState().config?.nspanelId;

    if (!nspanelId) return;

    const payload: NSPanelMQTTManagerCommand = {
      nspanelId: nspanelId,
      ...commandPayload,
    };

    const buffer = NSPanelMQTTManagerCommand.encode(payload).finish();

    // 3. Publish
    if (!client?.connected) return;
    devLog("Sending to manager: ", payload);
    client.publish({
      destination: `mqtt/nspanel/mqttmanager_${MANAGER_ADDRESS}/command`,
      binaryBody: buffer,
    });
  },
};
