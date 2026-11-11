interface Props {
  className?: string;
  isOn: boolean;
}
function CeilingLightIcon({ className, isOn }: Props) {
  return (
    <svg
      viewBox="0 0 94.71 191.38"
      width="100%"
      height="100%"
      className={className}
      version="1.1"
      xmlns="http://www.w3.org/2000/svg"
      preserveAspectRatio="xMidYMid meet"
    >
      <path
        id="mount"
        fill="currentColor"
        d="m 68.055,0 v 12.46564 c 0,0 -1.058178,19.022711 -20.775,19.022711 L 47.295,0 Z M 26.52,0 v 12.46564 c 0,0 1.058178,19.022711 20.775,19.022711 L 47.28,0 Z"
      />
      <path
        fill="currentColor"
        d="M 45.32,114.01564 V 31.421973 h 3.95 v 82.593667 z"
        id="stem"
      />
      {/* <path
        id="glass"
        fill={isOn ? "#ffc101" : "currentColor"}
        className="transition-colors duration-300 ease-in-out"
        d="m 47.36,114.01564 c 0,0 41.535,-6.16302 41.535,45.54436 0,0 5.76,-0.0523 5.76,5.76 v 21.16 c 0,0 -0.50439,4.9 -6.557071,4.9 L 47.295,191.37 Z m -0.065,0 c 0,0 -41.535,-6.16302 -41.535,45.54436 0,0 -5.76,-0.0523 -5.76,5.76 v 21.16 c 0,0 0.50439006,4.9 6.5570708,4.9 L 47.36,191.37 Z"
      /> */}
      <path
        id="glass"
        fill={isOn ? "#ffc101" : "currentColor"}
        className="transition-colors duration-300 ease-in-out"
        d="m 47.205157,114.01564 c 0,0 41.445157,-6.16302 41.445157,45.54436 0,0 5.76,-0.0523 5.76,5.76 v 21.16 c 0,0 -0.50439,4.9 -6.557071,4.9 H 47.202658 Z m 0,0 c 0,0 -41.4451569,-6.16302 -41.4451569,45.54436 0,0 -5.7600001,-0.0523 -5.7600001,5.76 v 21.16 c 0,0 0.50439006,4.9 6.5570708,4.9 H 47.207656 Z"
      />
    </svg>
  );
}
export default CeilingLightIcon;
