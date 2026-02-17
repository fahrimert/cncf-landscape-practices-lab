package internal

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	"github.com/gorilla/mux"
)

const (
	StateStoreName = "statestore"
)

type InventoryController struct {
	Client dapr.Client
}

func NewInventoryController() *InventoryController {
	client, err := dapr.NewClient()
	if err != nil {
		log.Fatalf("❌ Dapr Client başlatılamadı: %v", err)
	}
	return &InventoryController{Client: client}
}

func (c *InventoryController) OrderCreatedHandler(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	log.Printf("🔔 EVENT: Sipariş alındı. ID: %s", e.ID)

	var order OrderEvent
	data, _ := e.Data.(string)
	if data == "" {
		byteData, _ := json.Marshal(e.Data)
		data = string(byteData)
	}
	_ = json.Unmarshal([]byte(data), &order)

	for _, item := range order.Items {
		itemKey := item.ProductID
		result, err := c.Client.GetState(ctx, StateStoreName, itemKey, nil)
		if err != nil {
			log.Printf("⚠️ Stok okuma hatası (%s): %v", itemKey, err)
			continue
		}

		currentQty := 0
		if result.Value != nil {
			var invItem InventoryItem
			_ = json.Unmarshal(result.Value, &invItem)
			currentQty = invItem.Quantity
		} else {
			currentQty = 100
			log.Printf("🆕 Yeni ürün varsayıldı: %s (100 Adet)", itemKey)
		}

		newQty := currentQty - item.Quantity
		if newQty < 0 {
			log.Printf("⛔ Yetersiz Stok! Ürün: %s, İstenen: %d, Eldeki: %d", itemKey, item.Quantity, currentQty)
			continue
		}

		newItem := InventoryItem{ProductID: itemKey, Quantity: newQty}
		jsonData, _ := json.Marshal(newItem)
		
		if err := c.Client.SaveState(ctx, StateStoreName, itemKey, jsonData, nil); err != nil {
			log.Printf("❌ Stok güncelleme hatası: %v", err)
		} else {
			log.Printf("✅ Stok Düştü: %s -> Kalan: %d", itemKey, newQty)
		}
	}

	return false, nil
}

func (c *InventoryController) GetInventory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	result, err := c.Client.GetState(r.Context(), StateStoreName, productID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.Value == nil {
		json.NewEncoder(w).Encode(InventoryItem{ProductID: productID, Quantity: 0})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result.Value)
}