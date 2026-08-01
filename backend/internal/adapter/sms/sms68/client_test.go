package sms68

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSIMTypeFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata json.RawMessage
		want     string
	}{
		{name: "missing defaults to virtual", want: simVirtual},
		{name: "physical", metadata: json.RawMessage(`{"simType":"2"}`), want: simPhysical},
		{name: "invalid defaults to virtual", metadata: json.RawMessage(`{"simType":"3"}`), want: simVirtual},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := simTypeFromMetadata(test.metadata); got != test.want {
				t.Fatalf("simTypeFromMetadata() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidityTypesPrioritizesConfiguredValue(t *testing.T) {
	metadata := json.RawMessage(`{"validityType":"2","simType":"2"}`)
	want := []string{"2", "4", "3", "1"}
	if got := validityTypes(metadata); !reflect.DeepEqual(got, want) {
		t.Fatalf("validityTypes() = %v, want %v", got, want)
	}
}

func TestMetadataScopes(t *testing.T) {
	client := &Client{}
	got := client.MetadataScopes()
	if len(got) != 2 || got[0].SIMType != simVirtual || got[1].SIMType != simPhysical {
		t.Fatalf("MetadataScopes() = %#v", got)
	}
}
