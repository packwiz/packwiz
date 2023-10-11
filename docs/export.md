## packwiz modrinth export

Export the current modpack into a .mrpack for Modrinth

```
packwiz export [flags]
```

### Options

```
  -h, --help              help for export
  -o, --output string     The file to export the modpack to
      --restrictDomains   Restricts domains to those allowed by modrinth.com (default true)
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

