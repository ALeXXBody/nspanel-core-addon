gui1 = water
gui2 = simpleblue
gui3 = sunrise
gui4 = lightbulb

# HMI design sources
The gui.HMI / .xcf sources and the automation scripts that build the
output_tft*/gui.tft files live OUTSIDE this repository in
NSPanelManager-HMI-archive (they were removed to keep the repo small).
The compiled gui.tft files stay here because panels download them at
runtime via the manager web interface.
