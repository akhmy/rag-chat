package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"rag-service/internal/http/client"
)

var motto = `                                  888               888    
                                  888               888    
                                  888               888    
888d888 8888b.   .d88b.   .d8888b 88888b.   8888b.  888888 
888P"      "88b d88P"88b d88P"    888 "88b     "88b 888    
888    .d888888 888  888 888      888  888 .d888888 888    
888    888  888 Y88b 888 Y88b.    888  888 888  888 Y88b.  
888    "Y888888  "Y88888  "Y8888P 888  888 "Y888888  "Y888 
                     888                                   
                Y8b d88P                                   
                 "Y88P"                                    `

func main() {
	os.Exit(func() int {
		fmt.Println(motto)

		c := client.New(http.DefaultClient, &url.URL{Scheme: "http", Host: "127.0.0.1:8080"})

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("\nYou: ")
			if !scanner.Scan() {
				return 0
			}

			query := scanner.Text()

			reply, err := c.Ask(context.Background(), query, 5)
			if err != nil {
				fmt.Println("Error: " + err.Error())
				continue
			}

			fmt.Println("Agent: " + reply)
		}
	}())
}
