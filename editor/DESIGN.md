# Design

What every screen of Coddy keeps to. Read it before drawing a new one, or changing one.

## ⚠️ Mind the overflows

Text that runs past its box cannot be read, and it makes a screen look broken at once. A
window can be narrow, a name long, a translation longer: **every text has to fit the room it
is given, at any width.**

Before calling a screen done, choose for each text what it does when it is too long:

| The text                              | What it does                                   | Example in Coddy                         |
|---------------------------------------|------------------------------------------------|------------------------------------------|
| A name, a title, a path               | Ends in an ellipsis (`…`) at its box's edge    | An icon theme's name on its card         |
| A short description, in a grid        | Takes two lines at most, the second with `…`   | A plugin's description on its card       |
| An explanation someone has to read    | Wraps onto as many lines as it needs           | A setting's description in its sheet     |
| A button's word                       | Is short enough never to need either           | "Done", "Restart", "Open folder"         |

Go UI draws a text on one line and does not wrap it, so a long one has to be split on purpose:
a box with `Wrap: true` holding its words, or the line measured and cut. Split the words when
the text changes, not every frame.

Then check it: a test with a text far longer than its box (see
`TestASettingsLongDescriptionWrapsInsideItsSheet` and `TestANameTooLongForItsCardEndsInAnEllipsis`),
and a capture of the screen at its narrowest, in light and dark.
