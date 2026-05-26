//go:build operational_scripts
// +build operational_scripts

package main

import (
	"log"
	"os"
)

func mustOperationalDSN() string {
	dsn := os.Getenv("CONTRACT_MANAGE_DSN")
	if dsn == "" {
		log.Fatal("CONTRACT_MANAGE_DSN is required")
	}
	return dsn
}
