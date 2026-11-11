function ColorTempIcon() {
  return (
    <div
      className="w-full aspect-square rounded-full"
      style={{
        background: `conic-gradient(from 0, #3b82f6, #ffffff, #facc15, #3b82f6)`,
        // This mask creates a hole from the center (0%) to 60%
        WebkitMaskImage: "radial-gradient(transparent 45%, black 46%)",
        maskImage: "radial-gradient(transparent 45%, black 46%)",
        filter: "blur(1px)",
      }}
    />
  );
}
export default ColorTempIcon;
