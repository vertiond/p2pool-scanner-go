package handler

import (
	"encoding/json"
	"net/http"

	"github.com/KforG/p2pool-scanner-go/config"
	"github.com/KforG/p2pool-scanner-go/logging"
	"github.com/KforG/p2pool-scanner-go/scanner"

	"github.com/go-chi/chi"
)

func Router(l *scanner.NodeList) {
	logging.Infof("Setting up router..\n")

	r := chi.NewRouter()
	//r.Use(middleware.Logger) // Useful for debugging purposes. Doesn't need to clog log in production.

	r.Get("/nodes", getNodes(l))

	logging.Infof("Listening on port %s", config.Active.WebPort)
	http.ListenAndServe(":"+config.Active.WebPort, r)
}

func getNodes(l *scanner.NodeList) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(l.Snapshot())
	}
}
