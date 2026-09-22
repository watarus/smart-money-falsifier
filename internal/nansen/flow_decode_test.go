package nansen

import (
	"encoding/json"
	"testing"
)

// exchange_avg_flow_usd is the only field that separates "no exchange touched
// this token" (null) from "exchanges saw flow netting to zero" (a number). A
// float64 would decode both to 0 and hide the difference.
func TestFlowIntelligence_ExchangeAvgNullIsDistinguishable(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		observed bool
	}{
		{"no exchange activity", `{"exchange_net_flow_usd": 0.0, "exchange_avg_flow_usd": null}`, false},
		{"flow that nets to zero", `{"exchange_net_flow_usd": 0.0, "exchange_avg_flow_usd": 211.8}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f FlowIntelligence
			if err := json.Unmarshal([]byte(tt.body), &f); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := f.ExchangeAvgFlowUSD != nil; got != tt.observed {
				t.Fatalf("exchange observed = %v, want %v", got, tt.observed)
			}
		})
	}
}
