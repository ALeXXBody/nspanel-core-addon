export const LightType = {
  ALL: 0,
  TABLE: 1,
  CEILING: 2,
} as const;

// This line allows you to use 'LightType' as a type in your function arguments
export type LightType = (typeof LightType)[keyof typeof LightType];

export const SliderType = {
  BRIGHTNESS: 1,
  COLORTEMP: 2,
  RGB: 3,
  SATURATION: 4,
};

export type SliderType = (typeof SliderType)[keyof typeof SliderType];
