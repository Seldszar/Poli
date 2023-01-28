package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/tidwall/gjson"
)

type AdSlot struct {
	Date     time.Time `json:"date"`
	Duration int64     `json:"duration"`
}

type H = map[string]any

var (
	clientID     = os.Getenv("CLIENT_ID")
	channelLogin = os.Getenv("CHANNEL_LOGIN")
	accessToken  = os.Getenv("ACCESS_TOKEN")

	client = http.Client{
		Timeout: 5 * time.Second,
	}

	currentSlots = make([]AdSlot, 0)
)

func fetchAdSchedule(channelLogin string) ([]AdSlot, error) {
	s := fmt.Sprintf(
		`{"variables":{"login":"%s","shouldSkipCIP":false},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"42d19f285b491d9449fb08edcd68d7f22f2490178263aae3a60ecf8d6563d294"}}}`,
		channelLogin,
	)

	req, err := http.NewRequest("POST", "https://gql.twitch.tv/gql", strings.NewReader(s))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", clientID)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", fmt.Sprintf("OAuth %s", accessToken))

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	slots := make([]AdSlot, 0)

	gjson.
		GetBytes(body, "data.user.adProperties.density.adSchedule").
		ForEach(func(key, value gjson.Result) bool {
			slots = append(slots, AdSlot{
				Duration: value.Get("durationSeconds").Int(),
				Date:     value.Get("runAtTime").Time(),
			})

			return true
		})

	return slots, nil
}

func startWebServer() error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().
			Set("Access-Control-Allow-Origin", "*")

		w.Header().
			Set("Content-Type", "application/json")

		json.NewEncoder(w).
			Encode(currentSlots)
	})

	return http.ListenAndServe(":3000", handler)
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stdout,
	})

	go startWebServer()

	for {
		log.Debug().
			Msg("Fetching ad schedule...")

		slots, err := fetchAdSchedule(channelLogin)

		if err != nil {
			log.Error().
				Err(err).
				Msg("An error occured while fetching ad schedule")
		}

		if slots != nil {
			log.Debug().
				Interface("slots", slots).
				Msg("Fetched ad schedule")

			currentSlots = slots
		}

		time.Sleep(time.Minute)
	}
}
