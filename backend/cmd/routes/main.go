// Export the actual registry without workers, listeners, or external requests.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/router"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir, err := os.MkdirTemp("", "ovh-route-registry-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	database, err := db.Open(dir)
	if err != nil {
		return err
	}
	defer database.Close()
	state := &app.State{DB: database, Config: config.New(database)}
	routes := router.New(router.Config{State: state, EnableAuth: true}, nil).Routes()
	out := []map[string]string{}
	for _, r := range routes {
		out = append(out, map[string]string{"method": r.Method, "path": r.Path, "handler": r.Handler})
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
