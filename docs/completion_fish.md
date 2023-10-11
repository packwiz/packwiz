## packwiz completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	packwiz completion fish | source

To load completions for every new session, execute once:

	packwiz completion fish > ~/.config/fish/completions/packwiz.fish

You will need to start a new shell for this setup to take effect.


```
packwiz completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
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

* [packwiz completion](packwiz_completion.md)	 - Generate the autocompletion script for the specified shell

