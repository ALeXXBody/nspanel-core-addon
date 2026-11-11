function ColorIcon() {
  return (
    <div
      className="w-full aspect-square rounded-full"
      style={{
        background: `conic-gradient(
          from 0deg,
          hsl(0, 100%,70%), hsl(60, 100%,70%), hsl(120, 100%,70%), hsl(180, 100%,70%), hsl(240, 100%,70%), hsl(300, 100%,70%), hsl(360, 100%,70%)
        )`,
        WebkitMaskImage: "radial-gradient(transparent 45%, black 46%)",
        maskImage: "radial-gradient(transparent 45%, black 46%)",
      }}
    />
  );
}

export default ColorIcon;
