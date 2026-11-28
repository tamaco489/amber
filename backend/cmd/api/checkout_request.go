package main

import (
	"fmt"
)

// 1 品目あたりの数量の上限
const maxQuantity = 99

// checkoutRequest は、注文の確定の本文
//   - 項目があるかを見分けるため、ポインタで受ける (encoding/json には「必須」の仕組みがない)
type checkoutRequest struct {
	Items             []checkoutItem `json:"items"`               // 注文する品目
	ShippingAddressID *string        `json:"shipping_address_id"` // 届け先の ID
}

// checkoutItem は、注文する品目 1 つ
type checkoutItem struct {
	SKU       *string `json:"sku"`        // 商品の ID (prices のキー)
	Quantity  *int    `json:"quantity"`   // 数量
	UnitPrice *int    `json:"unit_price"` // クライアントが送る単価。合計には使わない
}

// requiredField は、必須の項目 1 つ
type requiredField[T any] struct {
	name    string        // 本文での項目の名前 (エラーの文言に使う)
	present func(*T) bool // 項目が入っていれば true
}

// 本文の必須の項目
//   - 並びの順に確かめ、最初に足りない項目をエラーにする
var requiredRequestFields = []requiredField[checkoutRequest]{
	{name: "items", present: func(r *checkoutRequest) bool { return len(r.Items) > 0 }},
	{name: "shipping_address_id", present: func(r *checkoutRequest) bool {
		return r.ShippingAddressID != nil && *r.ShippingAddressID != ""
	}},
}

// 品目ごとの必須の項目
//   - 並びの順に確かめ、最初に足りない項目をエラーにする
//   - 必須の項目を変えるときは、この一覧だけを直す
var requiredItemFields = []requiredField[checkoutItem]{
	{name: "sku", present: func(it *checkoutItem) bool { return it.SKU != nil }},
	{name: "quantity", present: func(it *checkoutItem) bool { return it.Quantity != nil }},
	{name: "unit_price", present: func(it *checkoutItem) bool { return it.UnitPrice != nil }},
}

// validate は、必須の項目がそろっているかと、値が使えるかを確かめる
//   - 必須の項目をすべて確かめてから、値を確かめる (足りない項目のエラーを先に返すため)
func (r *checkoutRequest) validate() error {
	if err := r.validateRequired(); err != nil {
		return err
	}
	return r.validateValues()
}

func (r *checkoutRequest) validateRequired() error {
	for _, f := range requiredRequestFields {
		if !f.present(r) {
			return fmt.Errorf("%s is required", f.name)
		}
	}
	for i := range r.Items {
		for _, f := range requiredItemFields {
			if !f.present(&r.Items[i]) {
				return fmt.Errorf("items[%d].%s is required", i, f.name)
			}
		}
	}
	return nil
}

func (r *checkoutRequest) validateValues() error {
	for i, it := range r.Items {
		if _, ok := prices[*it.SKU]; !ok {
			return fmt.Errorf("items[%d].sku is unknown: %q", i, *it.SKU)
		}
		if *it.Quantity < 1 || *it.Quantity > maxQuantity {
			return fmt.Errorf("items[%d].quantity must be between 1 and %d", i, maxQuantity)
		}
	}
	return nil
}
