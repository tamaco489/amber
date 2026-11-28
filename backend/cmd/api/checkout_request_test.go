package main

import (
	"testing"
)

func TestCheckoutRequestValidate(t *testing.T) {
	// 必須の項目がすべてそろった品目
	fullItem := func() checkoutItem {
		return checkoutItem{SKU: new("TSHIRT-BLK-M"), Quantity: new(2), UnitPrice: new(2980)}
	}

	tests := map[string]struct {
		req     checkoutRequest
		wantErr string
	}{
		"必須の項目がそろっていればエラーにしない": {
			req: checkoutRequest{Items: []checkoutItem{fullItem()}, ShippingAddressID: new("addr_01")},
		},
		"items が無ければエラーにする": {
			req:     checkoutRequest{ShippingAddressID: new("addr_01")},
			wantErr: "items is required",
		},
		"items が空ならエラーにする": {
			req:     checkoutRequest{Items: []checkoutItem{}, ShippingAddressID: new("addr_01")},
			wantErr: "items is required",
		},
		"shipping_address_id が無ければエラーにする": {
			req:     checkoutRequest{Items: []checkoutItem{fullItem()}},
			wantErr: "shipping_address_id is required",
		},
		"shipping_address_id が空の文字列ならエラーにする": {
			req:     checkoutRequest{Items: []checkoutItem{fullItem()}, ShippingAddressID: new("")},
			wantErr: "shipping_address_id is required",
		},
		"sku が無ければ、その品目の番号でエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{fullItem(), {Quantity: new(1), UnitPrice: new(1200)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[1].sku is required",
		},
		"quantity が無ければエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("MUG-RED"), UnitPrice: new(1200)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[0].quantity is required",
		},
		"unit_price が無ければエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("TSHIRT-BLK-M"), Quantity: new(2)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[0].unit_price is required",
		},
		"足りない項目が複数あれば、一覧の先にある項目でエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("TSHIRT-BLK-M")}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[0].quantity is required",
		},
		"値の誤りより、足りない項目のエラーを先に返す": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("UNKNOWN"), Quantity: new(1), UnitPrice: new(1)}, {SKU: new("MUG-RED"), Quantity: new(1)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[1].unit_price is required",
		},
		"知らない sku ならエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("UNKNOWN"), Quantity: new(1), UnitPrice: new(1)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: `items[0].sku is unknown: "UNKNOWN"`,
		},
		"quantity が 0 ならエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("MUG-RED"), Quantity: new(0), UnitPrice: new(1200)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[0].quantity must be between 1 and 99",
		},
		"quantity が上限を超えればエラーにする": {
			req: checkoutRequest{
				Items:             []checkoutItem{{SKU: new("MUG-RED"), Quantity: new(100), UnitPrice: new(1200)}},
				ShippingAddressID: new("addr_01"),
			},
			wantErr: "items[0].quantity must be between 1 and 99",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.req.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("validate() = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
