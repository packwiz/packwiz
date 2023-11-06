## packwiz init

Initialise a packwiz modpack

```
packwiz init [flags]
```

### Options

```
      --author string               The author of the modpack (omit to define interactively)
      --fabric-latest               Automatically select the latest version of Fabric loader
      --fabric-version string       The Fabric loader version to use (omit to define interactively)
      --forge-latest                Automatically select the latest version of Forge
      --forge-version string        The Forge version to use (omit to define interactively)
  -h, --help                        help for init
      --index-file string           The index file to use (default "index.toml")
  -l, --latest                      Automatically select the latest version of Minecraft
      --liteloader-latest           Automatically select the latest version of LiteLoader
      --liteloader-version string   The LiteLoader version to use (omit to define interactively)
      --mc-version string           The Minecraft version to use (omit to define interactively)
      --modloader string            The mod loader to use (omit to define interactively)
      --name string                 The name of the modpack (omit to define interactively)
      --neoforge-latest             Automatically select the latest version of NeoForge
      --neoforge-version string     The NeoForge version to use (omit to define interactively)
      --quilt-latest                Automatically select the latest version of Quilt loader
      --quilt-version string        The Quilt loader version to use (omit to define interactively)
  -r, --reinit                      Recreate the pack file if it already exists, rather than exiting
  -s, --snapshot                    Use the latest snapshot version with --latest
      --version string              The version of the modpack (omit to define interactively)
```

### Options inherited from parent commands

```
      --cache string              The directory where packwiz will cache downloaded mods (default "/Users/filip/Library/Caches/packwiz/cache")
      --config string             The config file to use (default "/Users/filip/Library/Application Support/packwiz/.packwiz.toml")
      --meta-folder string        The folder in which new metadata files will be added, defaulting to a folder based on the category (mods, resourcepacks, etc; if the category is unknown the current directory is used)
      --meta-folder-base string   The base folder from which meta-folder will be resolved, defaulting to the current directory (so you can put all mods/etc in a subfolder while still using the default behaviour) (default ".")
      --pack-file string          The modpack metadata file to use (default "pack.toml")
  -y, --yes                       Accept all prompts with the default or "yes" option (non-interactive mode) - may pick unwanted options in search results
```

### SEE ALSO

* [packwiz](packwiz.md)	 - A command line tool for creating Minecraft modpacks

