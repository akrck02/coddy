package sdk

import "encoding/json"

// ModeFrameMethod asks a plugin that contributes a mode for a frame of its screen; the screen
// package says what goes in and out. Coddy asks only when there was input over the screen or the
// last frame asked for another, and shows the last frame meanwhile.
const ModeFrameMethod = "mode/frame"

// DrawMode answers mode/frame, given its params as they came: give it screen.Screen.Answer, so
// only the plugins that draw a screen depend on go-ui.
type DrawMode func(params json.RawMessage) (any, error)
