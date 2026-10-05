package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	loggly "github.com/jamespearly/loggly"
)

func fetchLeaderboard(c http.Client) (League, error) {
	var league League
	resp, err := c.Get("https://na1.api.riotgames.com/lol/league/v4/challengerleagues/by-queue/RANKED_SOLO_5x5?api_key=" + os.Getenv("RIOT_API_KEY"))
	if err != nil {
		return league, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return league, fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(&league)
	return league, err
}

func printLeaderboard(league League) {
	fmt.Println("Tier:", league.Tier)
	fmt.Println("Queue:", league.Queue)
	fmt.Println("Total players:", len(league.Entries))
	for i, e := range league.Entries {
		if i >= 10 {
			break
		}
		fmt.Println("PUUID:", e.PUUID, "LP:", e.LeaguePoints, "Wins:", e.Wins, "Losses:", e.Losses)
	}
}

func main() {
	var tag string
	tag = "LoLPoller"

	// Instantiate the client
	client := loggly.New(tag)

	// Instantiate HTTP client 
	c := http.Client{}

	// Valid EchoSend (message echoed to console and no error returned)
	err := client.EchoSend("info", "Good morning!")
	fmt.Println("err:", err)

	// Valid Send (no error returned)
	err = client.Send("info", "Good morning! No echo.")
	fmt.Println("err:", err)

	// Invalid EchoSend -- message level error
	err = client.EchoSend("blah", "blah")
	fmt.Println("err:", err)

	// Poll for Riot API 
	for {
		league, fetchErr := fetchLeaderboard(c)
		if fetchErr != nil {
			message := "request failed: " + fetchErr.Error()
			client.EchoSend("error", message)
		} else {
			printLeaderboard(league)
			count := len(league.Entries)
			message := fmt.Sprintf("request good: collected %d entries: ", count)
			client.EchoSend("info", message)
		}
		//need it to loop a certain amount of times per minute
		time.Sleep(20 * time.Second)
	}
}

// my pull from postman for the riot API
type Entry struct {
	PUUID	string `json:"puuid"`
	LeaguePoints	int    `json:"leaguePoints"`
	Rank	string `json:"rank"`
	Wins	int    `json:"wins"`
	Losses	int    `json:"losses"`
	Veteran	bool   `json:"veteran"`
	Inactive	bool   `json:"inactive"`
	FreshBlood	bool   `json:"freshBlood"`
	HotStreak	bool   `json:"hotStreak"`
}

type League struct {
	Tier	string  `json:"tier"`
	Queue	string  `json:"queue"`
	Entries	[]Entry `json:"entries"`
}
