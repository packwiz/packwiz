# packwiz-modded
This is a slightly altered version of packwiz modified to fit my workflow. This version removes support for Curseforge and adds more support for the Modrinth format, like using override folders.

## Command Reference
To see how a specific command works see the `docs` folder.

## Shell Completion
For autocompletion instructions see [completion.md](docs/completion.md).

## Basic Usage
Initialize the modpack by running:
```
packwiz init
```

You can then add content from modrinth using:
```
packwiz mr add [url]
```

Once ready you can export to the mrpack format using:
```
packwiz export
```

To add configs and other such files simply add these into the overrides folder.