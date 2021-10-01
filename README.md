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

## Getting Started
- Run `packwiz init` to create a modpack in the current folder
- Run `packwiz curseforge import [zip path]` to import from a CurseForge modpack
- Run `packwiz refresh` to update the index of mods
- Run `packwiz curseforge install [mod]` to install a mod from CurseForge
- Run `packwiz modrinth install [mod]` to install a mod from Modrinth
- Run `packwiz update [mod]` to update a mod
- Run `packwiz update --all` to update all the mods in the modpack
- Run `packwiz curseforge export` to export the modpack in the format supported by the CurseForge Launcher
- Run `packwiz serve` to start a local HTTP server running the pack - which packwiz-installer can install from
- Run `packwiz curseforge detect` to detect files that are available on CurseForge and make them downloaded from there
- Use the `--help` flag for more information about any command

### packwiz-installer for pack installation
[packwiz-installer](https://github.com/comp500/packwiz-installer) is a Java-based installer that allows for automatic installation and updates of packwiz packs! It can be used with MultiMC as a prelaunch task, or on servers as part of your start script, and supports side-only mods as well as optional mods with a fancy GUI.

To distribute a packwiz modpack, you'll first want to set up a web hosting service (such as Netlify, GitHub Pages, GitLab Pages) so that your pack files are accessible from a HTTP/HTTPS link.

Then to distribute the modpack as a MultiMC instance, do the following:

1. Create a barebones MultiMC instance, with the modloader and Minecraft version you want (memory allocation overrides are also a good idea)
2. Download packwiz-installer-bootstrap from https://github.com/comp500/packwiz-installer-bootstrap/releases and place it in the instance Minecraft folder
    - This is the same folder as options.txt - MultiMC will call it `.minecraft` or `minecraft` depending on your system
3. Go to Edit Instance -> Settings -> Custom commands, then check the Custom Commands box and paste the following command into the pre-launch command field:
    - `"$INST_JAVA" -jar packwiz-installer-bootstrap.jar https://[your-server]/pack.toml`
    - (where `https://[your-server]/pack.toml` is the HTTP URL your `pack.toml` file is hosted at)
4. Use the Export Instance function to export your pack as a `.zip` file
4. To install your pack, users just need to add it with Add instance -> Import from zip - then packwiz-installer does the rest, keeping it up to date every time the game is launched!

For use on servers, add the `-g` flag to disable the GUI and `-s server` to download only server-side mods.

### Usage with Git
- On Windows, line ending conversion causes the hashes to change when files are uploaded to Git, so you'll get a ton of errors when trying to install the pack
    - You'll want to add a `.gitattributes` file in the root folder with the content `* -text` - this disables line ending conversion
    - If you have existing files committed to Git, you'll need to run `git rm --cached -r .` then `git add .` to re-add them after adding .gitattributes
- You'll also want a `.packwizignore` file containing `.git/**` and `.gitattributes` (same format as `.gitignore`) so that Git metadata isn't included in the pack index

### Resources
- See https://suspicious-joliot-f51f5c.netlify.app/index.html for some documentation
    - I am in the process of rewriting the format, so there may be information there that is outdated
- See https://github.com/Fibercraft/Temporary-Modpack for an example of an existing modpack using packwiz
    - This repository can be published to a service like GitHub Pages or Netlify and installed using packwiz-installer
    - This repository also shows the use of `.gitattributes` and `.packwizignore` to disable line ending modification (so that the hashes are correct) and ignore git-specific files
- https://modfest.net/fallfest/1.16/server/ is also a good example of a MultiMC instance that uses packwiz-installer

### Tips
- There are some useful aliases, like `packwiz cf` => `packwiz curseforge` and `packwiz mr` => `packwiz modrinth`
- The `packwiz cf install` command supports multiple formats:
    - `packwiz cf install sodium` (by slug)
    - `packwiz cf install https://www.curseforge.com/minecraft/mc-mods/sodium` (by mod page URL)
    - `packwiz cf install https://www.curseforge.com/minecraft/mc-mods/sodium/files/3067101` (by file page URL)
    - `packwiz cf install Sodium` (by search)
    - `packwiz cf install --addon-id 394468 --file-id 3067101` (if all else fails)
- If files aren't being found, try running the `packwiz refresh` command to update the index!

## Options
- Additional options can be configured in the `[options]` section of `pack.toml`, as follows:
    - `mods-folder` The folder to save mod metadata files into, for the install commands (relative to the pack root)
    - `acceptable-game-versions` A list of additional Minecraft versions to accept when installing or updating mods
    - `no-internal-hashes` If this is set to true, packwiz will not generate hashes of local files, to prevent merge conflicts and inconsistent hashes when using git/etc.
        - `packwiz refresh --build` can be used in this mode to generate internal hashes for distributing the pack with packwiz-installer
