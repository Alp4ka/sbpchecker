package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Alp4ka/sbpchecker"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orders := []string{
		"BD10007BVLHP5U0I9BFQRF8T8FIAPJTU", // Оплачен 12 руб.
		"AD1000163781VN84804A2BJJMEBFVCS0", // Ин прогресс 50 руб.
	}

	client, err := sbpchecker.NewClient(sbpchecker.Options{Headless: true})
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; ; {
		i = (i + 1) % len(orders)
		fmt.Println()
		fmt.Println(orders[i])

		timeStart := time.Now()
		res, err := client.FetchPaymentStatus(ctx, orders[i])
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			continue
		}
		fmt.Println("Время потрачено:", time.Since(timeStart).String())

		val, err := json.Marshal(res)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			continue
		}

		fmt.Printf("Статус: %s\n", string(val))
	}
}
