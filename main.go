package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	loggly "github.com/jamespearly/loggly"
)

func fetchPrices(c http.Client) ([]Coin, error) {
	var coins []Coin

	symbols := []string{"BTC", "ETH", "SOL", "XRP", "ADA", "DOGE", "AVAX", "LINK", "LTC", "BCH"}

	for _, symbol := range symbols {
		var price PriceResponse

		url := "https://api.exchange.coinbase.com/products/" + symbol + "-USD/ticker"

		resp, err := c.Get(url)
		if err != nil {
			return coins, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return coins, fmt.Errorf("unexpected response status for %s: %s", symbol, resp.Status)
		}

		err = json.NewDecoder(resp.Body).Decode(&price)
		resp.Body.Close()

		if err != nil {
			return coins, err
		}

		value, err := strconv.ParseFloat(price.Price, 64)
		if err != nil {
			return coins, err
		}

		coins = append(coins, Coin{
			Symbol: symbol,
			Price: value,
		})

		// space out API requests
		time.Sleep(400 * time.Millisecond)
	}

	return coins, nil
}

func printPrices(coins []Coin) {
	fmt.Println("Coinbase Price Tracker")
	fmt.Println("Total coins:", len(coins))

	for _, coin := range coins {
		fmt.Printf("%s: $%.6f  Change: %+.6f (%.4f%%)\n",
			coin.Symbol, coin.Price, coin.Change, coin.PercentChange)
	}
}

func main() {
	// DynamoDB
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println("AWS error:", err)
		return
	}
	db := dynamodb.NewFromConfig(cfg)

	var tag string
	tag = "CoinbasePoller"

	// Instantiate the client
	client := loggly.New(tag)

	// Instantiate HTTP client
	c := http.Client{Timeout: 10 * time.Second}

	// Valid EchoSend (message echoed to console and no error returned)
	err = client.EchoSend("info", "Good morning!")
	fmt.Println("err:", err)

	// Valid Send (no error returned)
	err = client.Send("info", "Good morning! No echo.")
	fmt.Println("err:", err)

	// Invalid EchoSend -- message level error
	err = client.EchoSend("blah", "blah")
	fmt.Println("err:", err)

	// Previous prices
	previous := make(map[string]float64)

	// Poll for Coinbase API
	for {
		coins, fetchErr := fetchPrices(c)

		if fetchErr != nil {
			message := "request failed: " + fetchErr.Error()
			client.EchoSend("error", message)
		} else {
			// compare prices to previous poll
			for i := range coins {
				oldPrice, found := previous[coins[i].Symbol]

				if found && oldPrice != 0 {
					coins[i].Change = coins[i].Price - oldPrice
					coins[i].PercentChange = (coins[i].Change / oldPrice) * 100
				}

				previous[coins[i].Symbol] = coins[i].Price
			}

			printPrices(coins)

			count := len(coins)
			message := fmt.Sprintf("request good: collected %d coin prices", count)
			client.EchoSend("info", message)

			// log each coin and its change
			for _, coin := range coins {
				message = fmt.Sprintf("%s: $%.4f Change: %+.4f (%.4f%%)",
					coin.Symbol, coin.Price, coin.Change, coin.PercentChange)
				client.Send("info", message)
			}

			// save data to DynamoDB
			err = saveData(db, coins)
			if err != nil {
				fmt.Println("DynamoDB error:", err)
			} else {
				fmt.Println("Saved to DynamoDB")
			}
		}

		// wait before polling again
		time.Sleep(30 * time.Second)
	}
}

// DynamoDB
func saveData(db *dynamodb.Client, coins []Coin) error {
	now := time.Now()

	data := Snapshot{
		PollID: strconv.FormatInt(now.UnixNano(), 10),
		Timestamp: now.Format(time.RFC3339),
		CoinCount: len(coins),
		Coins: coins,
	}

	item, err := attributevalue.MarshalMap(data)
	if err != nil {
		return err
	}

	_, err = db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("bchauder_CoinbasePrices"),
		Item: item,
	})

	return err
}

// my pull from the Coinbase API
type PriceResponse struct {
	Price string `json:"price"`
}

type Coin struct {
	Symbol string `json:"symbol" dynamodbav:"symbol"`
	Price float64 `json:"price" dynamodbav:"price"`
	Change float64 `json:"change" dynamodbav:"change"`
	PercentChange float64 `json:"percentChange" dynamodbav:"percentChange"`
}

type Snapshot struct {
	PollID string `dynamodbav:"pollId"`
	Timestamp string `dynamodbav:"timestamp"`
	CoinCount int `dynamodbav:"coinCount"`
	Coins []Coin `dynamodbav:"coins"`
}
