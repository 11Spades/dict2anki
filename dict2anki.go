package main

import (
	"encoding/json"
	"errors"
	"github.com/atselvan/ankiconnect"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"io"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	APIKey   string `json:"apiKey"`
	DeckName string `json:"deckName"`
}

type Card struct {
	Word         string
	PartOfSpeech string   `json:"fl"`
	Definitions  []string `json:"shortdef"`
}

func printHelp() {
	println(`dict2anki is a tool for quickly creating Anki cards from words.

Usage:

        dict2anki <word>

Note:

		This tool requires a valid Merriam-Webster API key and a deck name specified in the config file located at "~/.config/dict2anki/config.json"
		Also required is a running instance of Anki with AnkiConnect.`)
}

func LoadConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	f, err := os.Open(home + "/.config/dict2anki/config.json")
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var newConfig Config

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&newConfig)
	if err != nil {
		return Config{}, err
	}

	return newConfig, nil
}

func parseResponse(responseBody io.ReadCloser) (Card, error) {
	cardJson, err := io.ReadAll(responseBody)
	if err != nil {
		println("Error: Failed to read response body.")
		return Card{}, err
	}

	var cards []Card

	err = json.Unmarshal(cardJson, &cards)
	if err != nil {
		println("Error: Failed to parse response body JSON.")
		return Card{}, err
	}

	return cards[0], nil
}

func requestDefinition(word string, key string) (Card, error) {
	response, err := http.Get("https://www.dictionaryapi.com/api/v3/references/collegiate/json/" + word + "?key=" + key)
	if err != nil {
		println("Error: Failed to contact Merriam-Webster.")
		return Card{}, err
	}

	card, err := parseResponse(response.Body)
	if err != nil {
		println("Error: Failed to parse response")
		return Card{}, err
	}

	card.Word = word

	return card, nil
}

func checkDeckForDuplicate (client *ankiconnect.Client, word string, deck string) (bool, error) {
	cards, restErr := client.Cards.Get(`"deck:` + deck +`" "front:` + word + `"` )
	if restErr != nil {
		return false, errors.New("AnkiConnect error.")
	}

	if len(*cards) != 0 {
		return true, nil
	}

	return false, nil
}

func addCardToDeck(client *ankiconnect.Client, card Card, deck string) error {
	note := ankiconnect.Note{
		DeckName:  deck,
		ModelName: "Basic",
		Fields: ankiconnect.Fields{
			"Front": cases.Title(language.AmericanEnglish).String(card.Word),
			"Back":  card.PartOfSpeech + "<br><br>" + strings.Join(card.Definitions, "<br>"),
		},
	}

	restErr := client.Notes.Add(note)
	if restErr != nil {
		return errors.New("Ankiconnect error")
	}

	return nil
}

func main() {
	// Validate arguments
	if len(os.Args) == 0 {
		printHelp()
		return
	}

	// Load config
	config, err := LoadConfig()
	if err != nil {
		println("Fatal: Failed to open config file.")
		return
	}

	// Connect to Anki
	client := ankiconnect.NewClient()
	restErr := client.Ping()
	if restErr != nil {
		println("Fatal: Failed to connect to Anki. Is it running? Does it have AnkiConnect?")
		return
	}

	// Make our request
	card, err := requestDefinition(os.Args[1], config.APIKey)
	if err != nil {
		println("Fatal: Failed to connect to Merriam-Webster and Wiktionary, or failed to parse response.")
		return
	}

	// Print card
	println(card.Word)
	println(card.PartOfSpeech)
	println(strings.Join(card.Definitions, "\n"))

	// Check if the card is already in the Anki deck
	duplicateExists, err := checkDeckForDuplicate(client, card.Word, config.DeckName)
	if err != nil {
		println("Fatal: Failed to query deck for duplicates.")
		return
	}

	if duplicateExists {
		println("Duplicate detected, omitting.")
		return
	}

	// Write to Anki deck
	err = addCardToDeck(client, card, config.DeckName)
	if err != nil {
		println("Fatal: Failed to add card to deck.")
		return
	}

	println("Done.")
}
