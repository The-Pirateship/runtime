/*
Copyright © 2025 PirateShip
*/

package main

import (
	"log"

	"github.com/The-Pirateship/runtime/cmd"
)

func init() {
	log.Printf("init function ran")
}

func main() {
	cmd.Execute()
}
