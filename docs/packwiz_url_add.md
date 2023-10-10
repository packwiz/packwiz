## packwiz url add

Add an external file from a direct download link, for sites that are not directly supported by packwiz

```
packwiz url add [name] [url] [flags]
```

### Options

```
      --force              Add a file even if the download URL is supported by packwiz in an alternative command (which may support dependencies and updates)
  -h, --help               help for add
      --meta-name string   Filename to use for the created metadata file (defaults to a name generated from the name you supply)
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

* [packwiz url](packwiz_url.md)	 - Add external files from a direct download link, for sites that are not directly supported by packwiz

