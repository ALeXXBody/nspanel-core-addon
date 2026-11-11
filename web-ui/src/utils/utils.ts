export function generateRandomMac(): string {
  const bytes = new Uint8Array(6);

  bytes[0] = 0xaa;
  bytes[1] = 0xaa;
  bytes[2] = 0xaa;

  for (let i = 3; i < 6; i++) {
    bytes[i] = Math.floor(Math.random() * 256);
  }

  return Array.from(bytes)
    .map((byte) => byte.toString(16).padStart(2, "0").toUpperCase())
    .join(":");
}

export function generateFriendlyName(mac: string): string {
  const suffix = mac.replace(/:/g, "").slice(-4).toUpperCase();
  return `Browser-${suffix}`;
}
