package checkout

import (
	"testing"
)

func TestTotalPrice(t *testing.T) {
	tests := map[string]struct {
		items []item
		want  int
	}{
		"単価 × 数量を足し合わせる": {
			items: []item{
				{SKU: new("TSHIRT-BLK-M"), Quantity: new(2)},
				{SKU: new("MUG-RED"), Quantity: new(1)},
			},
			want: 2980*2 + 1200,
		},
		"クライアントが送る unit_price は使わない": {
			items: []item{{SKU: new("TSHIRT-BLK-M"), Quantity: new(2), UnitPrice: new(1)}},
			want:  5960,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := totalPrice(tt.items); got != tt.want {
				t.Errorf("totalPrice() = %d, want %d", got, tt.want)
			}
		})
	}
}
