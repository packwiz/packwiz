## packwiz modrinth add

Add a project from a Modrinth URL, slug/project ID or search

```
packwiz modrinth add [URL|slug|search] [flags]
```

### Options

```
  -h, --help                      help for add
      --project-id string         The Modrinth project ID to use
      --version-filename string   The Modrinth version filename to use
      --version-id string         The Modrinth version ID to use
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

* [packwiz modrinth](packwiz_modrinth.md)	 - Manage modrinth-based mods

