package main

import (
	// Modules of packwiz
	"github.com/packwiz/packwiz/cmd"
	_ "github.com/packwiz/packwiz/curseforge"
	_ "github.com/packwiz/packwiz/github"
	_ "github.com/packwiz/packwiz/modrinth"
	_ "github.com/packwiz/packwiz/utils"
)

func main() {
	cmd.Execute()
}
