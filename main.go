// A tiny server for Traefik ForwardAuth implementing ip blocking/allowing based on MaxMind data.
// Copyright (C) 2023 Enrico Bassetti
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// main is the program entry point. The only purpose of this function is to call run() and set the exit code if there is
// any error
func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	var dbpath = flag.String("db", "GeoLite2-Country.mmdb", "Database path")
	var countriesFlag = flag.String("countries", "IT", "Comma separated ISO country codes to allow or block (see action flag)")
	var action = flag.String("action", "allow", "Action on countries. If \"allow\", only those countries are allowed, others are blocked. If \"block\", only countries are blocked, others are allowed.")
	var httpTimeouts = flag.Duration("web-timeout", 30*time.Second, "Timeout when reading/writing HTTP")
	var httpListenAddr = flag.String("web-listen", ":8080", "HTTP Listener IP address and port")
	var allowEmptyCountries = flag.Bool("allow-empty-countries", false, "Whether to allow the request on empty results in country field (default block)")
	var dbRefreshPeriod = flag.Duration("db-refresh-every", 1*time.Hour, "After this period of time, the database file is re-read.")
	var debug = flag.Bool("debug", false, "Debug mode (log verbose)")

	flag.Parse()

	// Initialize logger
	var zlogger *zap.Logger
	if *debug {
		zlogger, _ = zap.NewDevelopment()
	} else {
		zlogger, _ = zap.NewProduction()
	}
	logger := zlogger.Sugar()

	if *action != "allow" && *action != "block" {
		logger.Fatal("Invalid action specified. Supported values are: allow, block")
		return errors.New("invalid action flag value")
	}

	// Create a map for country codes for fast lookup
	var countries = make(map[string]bool)
	for _, c := range strings.Split(*countriesFlag, ",") {
		countries[c] = true
	}

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// Open MaxMind GeoIP database
	mmdbfp, err := geoip2.Open(*dbpath)
	if err != nil {
		logger.Errorw("can't open MaxMind database", "err", err)
		return err
	}
	var mmdb = &mmdbfp
	defer func() { _ = (*mmdb).Close() }()

	// Refresh periodically MaxMind database by closing it and reopening it (as the downloader might have updated it)
	go func() {
		var t = time.NewTicker(*dbRefreshPeriod)
		for range t.C {
			logger.Debug("Trying to re-read the database from disk")
			mmdbfp2, err := geoip2.Open(*dbpath)
			if err != nil {
				logger.Errorw("can't re-read MaxMind database", "err", err)
				continue
			}
			// Swap the databases (mmdbfp1 is the old database, mmdbfp2 is the new one) and close the old one
			mmdbfp1 := *mmdb
			*mmdb = mmdbfp2

			// Wait for in-flight HTTP requests to finish
			time.Sleep(10 * time.Second)
			_ = mmdbfp1.Close()
			logger.Debug("MaxMind database reloaded successfully")
		}
	}()

	// Create the API server
	httpserver := http.Server{
		Addr:              *httpListenAddr,
		Handler:           handleRequest(logger, mmdb, countries, *action == "allow", *allowEmptyCountries),
		ReadTimeout:       *httpTimeouts,
		ReadHeaderTimeout: *httpTimeouts,
		WriteTimeout:      *httpTimeouts,
	}

	// Start the service listening for requests
	go func() {
		logger.Infof("http server listening on %s", httpserver.Addr)
		serverErrors <- httpserver.ListenAndServe()
		logger.Info("stopping API server")
	}()

	select {
	case err := <-serverErrors:
		// Non-recoverable server error
		logger.Errorw("http server error", "err", err)
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		logger.Infof("signal %v received, start shutdown", sig)
		_ = httpserver.Close()
	}

	return nil
}

func handleRequest(logger *zap.SugaredLogger, mmdb **geoip2.Reader, countries map[string]bool, allowListMode bool, allowEmpty bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the source IP address from Traefik
		sourceIP := r.Header.Get("X-Forwarded-For")
		if sourceIP == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse the IP
		ip := net.ParseIP(sourceIP)
		if ip == nil {
			logger.Errorw("can't parse IP address", "source-ip", sourceIP)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Lookup for the country from MaxMind
		record, err := (*mmdb).Country(ip)
		if err != nil {
			logger.Errorw("MaxMind database lookup failed", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if record.Country.IsoCode == "" && allowEmpty {
			logger.Debugw("Access granted in allow empty country mode", "ip", ip)
			w.WriteHeader(http.StatusOK)
			return
		} else if record.Country.IsoCode == "" && !allowEmpty {
			logger.Debugw("Access blocked in !allow empty country mode", "ip", ip)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Check if the country is in the list of countries
		if _, ok := countries[record.Country.IsoCode]; ok {
			// If found, reply depending on the mode
			if allowListMode {
				logger.Debugw("Access granted in allowlist mode (found in country list)", "ip", ip, "country", record.Country.IsoCode)
				w.WriteHeader(http.StatusOK)
			} else {
				logger.Debugw("Access blocked in blocklist mode (found in country list)", "ip", ip, "country", record.Country.IsoCode)
				w.WriteHeader(http.StatusForbidden)
			}
		} else {
			// If NOT found, reply depending on the mode
			if allowListMode {
				logger.Debugw("Access blocked in allowlist mode (NOT found in country list)", "ip", ip, "country", record.Country.IsoCode)
				w.WriteHeader(http.StatusForbidden)
			} else {
				logger.Debugw("Access allowed in blocklist mode (NOT found in country list)", "ip", ip, "country", record.Country.IsoCode)
				w.WriteHeader(http.StatusOK)
			}
		}
	}
}
