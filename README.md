# packwiz
A command line tool for creating minecraft modpacks.

## Installation
In future I will have a lot more installation options, but for now the easiest way is to compile from source.

1. Install Go
2. Run `go install github.com/comp500/packwiz`

## Getting Started
- Run `packwiz init` to create a modpack in the current folder
- Run `packwiz refresh` to update the index of mods
- Run `packwiz curseforge install [mod]` to install a mod from CurseForge
- Run `packwiz update *` to update all the mods in the modpack, or replace * with a mod to update it
- Run `packwiz curseforge export` to export the modpack in the format supported by the Twitch Launcher
- Use the `--help` flag for more information about any command
