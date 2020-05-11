# packwiz
A command line tool for creating minecraft modpacks.

## Installation
In future I will have a lot more installation options, but for now the easiest way is to compile from source.

1. Install Go
2. Clone or download the repository (`git clone https://github.com/comp500/packwiz`), and open the folder in a terminal
3. Run `go install .` to put it on your path, or `go build` to just make a binary. Be patient, it has to download and compile dependencies as well!

## Getting Started
- Run `packwiz init` to create a modpack in the current folder
- Run `packwiz curseforge import [zip path]` to import from a Twitch modpack
- Run `packwiz refresh` to update the index of mods
- Run `packwiz curseforge install [mod]` to install a mod from CurseForge
- Run `packwiz update [mod]` to update a mod
- Run `packwiz update --all` to update all the mods in the modpack
- Run `packwiz curseforge export` to export the modpack in the format supported by the Twitch Launcher
- Run `packwiz serve` to start a local HTTP server running the pack - which packwiz-installer can install from
- Use the `--help` flag for more information about any command
- Use [packwiz-installer](https://github.com/comp500/packwiz-installer) as a MultiMC prelaunch task for auto updating, optional mods and side-only mods!