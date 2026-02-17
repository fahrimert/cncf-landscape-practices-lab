package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"inventory-service/internal"

	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/http"
)

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "6000"
	}

	controller := internal.NewInventoryController()

	s := daprd.NewService(":" + port)

	sub := &common.Subscription{
		PubsubName: "order-pubsub",
		Topic:      "order_created",
		Route:      "/events/order_created",
	}
	
	if err := s.AddTopicEventHandler(sub, controller.OrderCreatedHandler); err != nil {
		log.Fatalf("❌ Abonelik hatası: %v", err)
	}

	if err := s.AddServiceInvocationHandler("check-stock", func(ctx context.Context, in *common.InvocationEvent) (out *common.Content, err error) {
		productID := string(in.Data)
		log.Printf("🔍 Stok Sorgusu Geldi: %s", productID)
		
		result, err := controller.Client.GetState(ctx, "statestore", productID, nil)
		if err != nil {
			log.Printf("DB Hatası: %v", err)
			return nil, err
		}
		
		data := result.Value
		if data == nil {
			data = []byte(`{"quantity": 0}`)
		}
		
		return &common.Content{
			ContentType: "application/json",
			Data:        data,
		}, nil
	}); err != nil {
		log.Fatalf("❌ Handler eklenemedi: %v", err)
	}

	log.Printf("🚀 Inventory Service (Go) %s portunda hazır...", port)
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("💀 Sunucu çöktü: %v", err)
	}
}