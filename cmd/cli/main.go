package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"rag-chat/internal/http/client"
	"strconv"

	_ "embed"
)

//go:embed motto
var motto string

func main() {
	os.Exit(run())
}

var usage = "usage: ingest {userID} {sessionID}"

func run() int {
	if len(os.Args) != 3 {
		fmt.Println(usage)
		return 1
	}

	c := client.New(http.DefaultClient, &url.URL{Scheme: "http", Host: "127.0.0.1:8080"})

	raw := os.Args[1]
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fmt.Println(usage)
		return 1
	}

	raw = os.Args[2]
	sessionID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fmt.Println(usage)
		return 1
	}

	fmt.Println(motto)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			return 0
		}

		query := scanner.Text()

		reply, err := c.Ask(context.Background(), userID, sessionID, query, 5)
		if err != nil {
			fmt.Println("Error: " + err.Error())
			continue
		}

		fmt.Println("Agent: " + reply)
	}
}
