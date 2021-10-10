# packwiz
A command-line tool for creating Minecraft modpacks. Join my Discord server if you need help [here](https://discord.gg/Csh8zbbhCt)!

## Features
- Git-friendly TOML-based metadata format
- MultiMC pack installer/updater, with support for optional mods and fast automatic updates - perfect for servers!
- Pack distribution with HTTP servers, with a built in local server for testing
- Easy installation and updating of multiple mods at once from CurseForge and Modrinth
- Exporting and importing to/from CurseForge packs
- Server-only and Client-only mod handling
- Creation of remote file metadata from JAR files for CurseForge mods

## Installation
Prebuilt binaries are available from [GitHub Actions](https://github.com/comp500/packwiz/actions) - the UI is a bit terrible, but essentially select the top build, then download the artifact ZIP for your system at the bottom of the page. To run the executable, add the folder where you downloaded it to your PATH environment variable ([see tutorial for Windows here](https://www.howtogeek.com/118594/how-to-edit-your-system-path-for-easy-command-line-access/)) or move it to where you want to use it.

In future I will have a lot more installation options, but you can also compile from source:

1. Install Go (1.17 or newer) from https://golang.org/dl/
2. Run `go install github.com/comp500/packwiz@latest`. Be patient, it has to download and compile dependencies as well!

## Documentation
See https://packwiz.infra.link/ for the full packwiz documentation!