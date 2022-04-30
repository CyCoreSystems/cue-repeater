# cue-repeater

The `cue-repeater` listens on an arbitrary number of ports, combines all
messages from them into a common stream.
Anything in that common stream is then sent on to an arbitrary number of target
UDP ports.

This is a quick modification of the `osc-repeater` for general-purpose UDP
reflection.
It is called the "cue" repeater because it is intended to receive cues from QLab
over UDP and redistribute them to a number of other receivers.
However, nothing inherently related to QLab cues constrains this project.
