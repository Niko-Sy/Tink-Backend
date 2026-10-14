package main

import (
	"chatroombackend/internal/testenv"
	"fmt"
	"log"
)

func main() {
	e, err := testenv.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()
	if err = testenv.Migrate(e.DB, 7); err != nil {
		log.Fatal(err)
	}
	if _, err = testenv.Seed(e.DB); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Isolated PostgreSQL: migrations 1-7 and fixtures passed")
}
