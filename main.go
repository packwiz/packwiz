package main

import (
	// Modules of packwiz
	"packwiz/cmd"
	_ "packwiz/migrate"
	_ "packwiz/modrinth"
)

func main() {
	cmd.Execute()
}
