package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Alp4ka/sbpchecker"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	orders := []string{
		"BD10007BVLHP5U0I9BFQRF8T8FIAPJTU", // Оплачен 12 руб.
		"AD1000163781VN84804A2BJJMEBFVCS0", // Ин прогресс 50 руб.
	}

	client, err := sbpchecker.NewClient(sbpchecker.Options{Headless: true, EntityPoolSize: 8})
	if err != nil {
		log.Fatal(err)
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			timeStart := time.Now()
			res, err := client.FetchPaymentStatus(ctx, orders[i])
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return
				}
				fmt.Printf("Ошибка: %v\n", err)
				return
			}

			val, err := json.Marshal(res)
			if err != nil {
				fmt.Printf("Ошибка: %v\n", err)
				return
			}

			fmt.Printf("Order: %s\tВремя: %s\tСтатус: %s\n",
				orders[i],
				time.Since(timeStart).String(),
				string(val),
			)
		}((i + 1) % len(orders))
	}
	wg.Wait()
}
