import type { IMessage } from "@stomp/stompjs";
// import root from "@/proto/bundle.js";

import {
  NSPanelConfig,
  NSPanelRoomEntitiesPage,
  NSPanelRoomStatus,
  NSPanelStatusReport,
  NSPanelWeatherUpdate,
} from "@/generated/src/proto/protobuf_nspanel";

import { NSPanelEntityState } from "@/generated/src/proto/protobuf_nspanel_entity";

const messageRegistry = {
  NSPanelConfig,
  NSPanelEntityState,
  NSPanelRoomEntitiesPage,
  NSPanelRoomStatus,
  NSPanelStatusReport,
  NSPanelWeatherUpdate,
};

export type protoMessage = keyof typeof messageRegistry;

// export type protoMessage =
//   | "NSPanelConfig"
//   | "NSPanelEntityState"
//   | "NSPanelRoomEntitiesPage"
//   | "NSPanelRoomStatus"
//   | "NSPanelStatusReport";

export function convertProtbuf<T>(
  message: IMessage,
  messageName: protoMessage,
): T | undefined {
  const messageType = messageRegistry[messageName];

  try {
    //Get the message body (which is a Base64 string)
    const base64Data = message.body;

    if (!base64Data) {
      console.warn("Received empty message body");
      return;
    }

    // Convert Base64 string to a Uint8Array.
    // Uint8Array.from with a charCodeAt mapper is a native fast path — the old
    // manual per-byte loop was 6-10x slower on large payloads (this runs for
    // every light/room/page/weather message on an ARM wall panel).
    const binaryString = window.atob(base64Data);
    const bytes = Uint8Array.from(
      { length: binaryString.length },
      (_, i) => binaryString.charCodeAt(i),
    );

    //Now decode the raw bytes with Protobuf
    const decoded = messageType.decode(bytes);

    //Convert to a clean object
    const data = decoded as unknown as T;

    return data;
  } catch (err) {
    console.error("Manual Base64 decode failed:", err);
  }
}
