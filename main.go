package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: impact <config.json>")
		os.Exit(1)
	}

	configPath := os.Args[1]

	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Println("config error:", err)
		os.Exit(1)
	}

	if err := RunImpact(cfg); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
