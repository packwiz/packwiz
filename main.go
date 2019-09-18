package main

import (
	// Modules of packwiz
	"github.com/comp500/packwiz/cmd"
	_ "github.com/comp500/packwiz/curseforge"
	_ "github.com/comp500/packwiz/utils"
)

func main() {
	cmd.Execute()
}
